/*
Copyright © 2024 Jeff Berkowitz (pdxjjb@gmail.com)

This program is free software: you can redistribute it and/or modify it
under the terms of the GNU Affero General Public License as published
by the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful, but
WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU
Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public
License along with this program. If not, see
<http://www.gnu.org/licenses/>.
*/
package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"flag"
	"io"
	"os"
)

var qflag = flag.Bool("q", false, "quiet offsets and opcodes")

// Table of mnemonics and their signatures

type KeyEntry struct {
	name string
	nbits uint16     // number of high bits required to recognize
	opcode uint16    // fixed opcode bits
	signature uint16 // see below
}

// Instruction argument shapes
const RRI uint16 = 1 // register, register, immediate7
const RJX uint16 = 2 // register, immediate10
const RRR uint16 = 3 // register, register, register
const RRX uint16 = 5 // register, register
const RXX uint16 = 6 // register
const XXX uint16 = 7 // no arguments

// Names of the registers, indexed by field content
var RegNames []string = []string {
	"r0", "r1", "r2", "r3", "r4", "r5", "r6", "r7",
}

// Names of special registers, indexed by field content
var SprNames []string = []string {
	"pc", "lnk", "err3", "err4", "err5", "err6", "err7",
}

// The allowed mnemonics and their signatures. This table is
// entered into the symbol table during initialization.
// Keep the entires in this table in the same order, with the
// same grouping, as the rules in ../asm/asm.
var KeyTable []KeyEntry = []KeyEntry {
	// Operations with two registers and a 7-bit immediate
	{"ldw", 3,  0x0000, RRI},
	{"ldb", 3,  0x2000, RRI},
	{"stw", 3,  0x4000, RRI},
	{"stb", 3,  0x6000, RRI},
	{"beq", 3,  0x8000, RRI},
	{"adi", 3,  0xA000, RRI}, // special case(s) in pass 2
	{"lui", 3,  0xC000, RJX}, // special case(s) in pass 2
	{"jlr", 4,  0xE000, RRI}, // special case(s) in pass 2

	// 3-operand XOPs
	{"add", 7,  0xF000, RRR},
	{"adc", 7,  0xF200, RRR},
	{"sub", 7,  0xF400, RRR},
	{"sbb", 7,  0xF600, RRR},
	{"bic", 7,  0xF800, RRR},
	{"bis", 7,  0xFA00, RRR},
	{"xor", 7,  0xFC00, RRR},

	// 2 operand YOPs
	{"lsp", 10, 0xFE00, RRX},
	{"lio", 10, 0xFE40, RRX},
	{"ssp", 10, 0xFE80, RRX},
	{"sio", 10, 0xFEC0, RRX},
	{"y04", 10, 0xFF00, RRX},
	{"y06", 10, 0xFF40, RRX},
	{"y06", 10, 0xFF80, XXX},

	// 1 operand ZOPs
	{"not", 13, 0xFFC0, RXX},
	{"neg", 13, 0xFFC8, RXX},
	{"sxt", 13, 0xFFD0, RXX},
	{"swb", 13, 0xFFD8, RXX},
	{"lsr", 13, 0xFFE0, RXX},
	{"lsl", 13, 0xFFE8, RXX},
	{"asr", 13, 0xFFF0, RXX},

	// 0 operand VOPs
	{"rti", 16, 0xFFF8, XXX},
	{"rtl", 16, 0xFFF9, XXX},
	{"di ", 16, 0xFFFA, XXX},
	{"ei ", 16, 0xFFFB, XXX},
	{"hlt", 16, 0xFFFC, XXX},
	{"brk", 16, 0xFFFD, XXX},
	{"v06", 16, 0xFFFE, XXX},
	{"die", 16, 0xFFFF, XXX},
}

// Y4 disassembler. A general theme with this tool is that it has
// only limited dependencies on libraries. The goal is to eventually
// rewrite this in a simple language with limited libraries and self-
// host on homemade Y4.

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) != 1 {
		usage()
	}
	f, err := os.Open(args[0])
	if err != nil {
		fatal(fmt.Sprintf("dis: opening \"%s\": %s", args[0], err.Error()))
	}
	defer f.Close()

	err = disassemble(f)
	if err != nil {
		fatal(fmt.Sprintf("dis: %s", err.Error()))
	}
	os.Exit(0)
}

// Disassemble an object file. Files written by the assembler currently
// have no header. They consist of up to two sections: code (at 0, length
// 128kB) and data (at 128k in file, length 64kB). The disassembler does
// not care whether the file is intended as a kernel or user binary.
//
// The disassembler processes the code segment and ignores the data segment.
// It stops processing if it sees the opcode 0 (which causes an illegal
// instruction trap when executed). The assembler either writes physical
// zeroes for part of the segment containing no instructions or seeks over
// it leaving a *nix file "hole" that reads as zeroes. Since there are no
// 16-bit immediate values in the ISA and no instructions designed to allow
// data tables in the code section, zero is a reliable endmarker.
//
// Pass 1 produces a list of internal mnemonics. Each corresponds to
// exactly one two-byte instruction in the code section. Some entries in
// the list, e.g. "jlr", are not legal assembly mnemonics, rather more
// like descriptors. All offsets (e.g. branch offsets) are correct.
//
// Disassembler pass 2 replaces some mnemonics in the list (including all
// the ones that aren't legal in the assembler) with others. It also nulls
// out (sets to zero length) some mnemonics. This is "sufficient" because
// some mnemonics expand to 2 (or possibly more) machine instructions, but
// the opposite ("shrinkage") never occurs. Pass 2 also places labels at
// every target location detected in pass 1. Finally pass3 prints the list.

const Ki64 int = 64*1024

func disassemble(f *os.File) error {
	var instructions []string

	// Pass 1
	forEachInst(f, &instructions, func(at int, prevInst uint16, inst uint16, instructions *[]string) error {
		(*instructions) = append((*instructions), decode(inst, at))
		return nil
	})

	// Pass 2
	forEachInst(f, &instructions, func(at int, prevInst uint16, inst uint16, instructions *[]string) error {
		condense(at, prevInst, inst, instructions)
		return nil
	})

	// Pass 3
	// Print everything in quick format or long format.

	if *qflag {
		for _, str := range instructions {
			// don't print instructions
			// blanked out in pass2().
			if len(str) > 0 {
				fmt.Println(str)
			}
		}
		return nil
	}

	forEachInst(f, &instructions, func(at int, prevInst uint16, inst uint16, instructions *[]string) error {
		// For instructions like jsr and ldi that are condensed in pass2(),
		// this loop prints something like:
		// ...
		// 1: 0xDFF9:
		// 2: 0xAFC1: ldi r1, 0xFFFF
		// ...
		// where DFF9 is the opcode of the lui and AFC1 is the opcode of the lli (adi) that make
		// up the 16-bit load (ldi) alias.
		fmt.Printf("%5d: 0x%04X: %s\n", at, inst, (*instructions)[at])
		return nil
	})
	return nil
}

func forEachInst(f *os.File, instructions *[]string, op func(int, uint16, uint16, *[]string) error) error {
	var b []byte = make([]byte, 2, 2) 
	var inst uint16
	var prevInst uint16
	var at int // instruction index, 0..64k-1
	var pos int64 // file position, 0..128k-1
	var err error
	var n int

	for n, err = f.ReadAt(b, pos); n == 2 && err == nil && at < int(Ki64); n, err = f.ReadAt(b, pos) {
		if inst = binary.LittleEndian.Uint16(b[:]); inst == 0 {
			break
		}
		if err := op(at, prevInst, inst, instructions); err != nil {
			return err
		}
		at++      // instruction index
		pos += 2  // file position
		prevInst = inst
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

func condense(at int, prevInst uint16, inst uint16, pInstr *[]string) error {
	instructions := *pInstr
	luiSeen := bits(prevInst,15,13) == 6

	// This first bit handles the messy case of trying to turn adi opcodes in lli or ldi.
	// We don't try to reconstruct the lsi (load small immediate) and instead disassemble
	// it as adi rT, r0, 6BitValue. We could have an "lsiSeen" kind of variable and disassemble
	// the ior, iow, srr, srw aliases, but they're not required (like with jlr) and I'm really
	// sick of this trixieness.
	if bits(inst,15,13) == 5 && bits(inst,5,3) == bits(inst,2,0) && bits(inst,2,0) != 0 {
	   if bits(inst,12,12) == 0 {
			// It's an lli, adding a positive 7-bit immediate += to rA.
			// If previous was lui, condense to ldi. Otherwise, write as lli.
			if luiSeen {
				instructions[at-1] = "" // hide the lui
				instructions[at] = fmt.Sprintf("ldi %s, 0x%04X",
					RegNames[bits(inst,2,0)],
					(bits(prevInst,12,3)<<6) | bits(inst,12,6))
			} else {
				// it's an lli, but without a leading lui. This could be
				// written as either lli or adi. We write it as lli.
				instructions[at] = fmt.Sprintf("lli %s, 0x%02X",
					RegNames[bits(inst,2,0)], bits(inst,12,6))
			}
		} else {
			// it's an adi with a negative immediate. This can't be an lli
			instructions[at] = fmt.Sprintf("adi %s, 0x%02X",
				RegNames[bits(inst,2,0)], bits(inst,12,6))
		}
	} else if bits(inst,15,12) == 0xE { // jlr
		// Replace with sys, jmp, or jsr, depending on the j2:j0 (rA) bits.
		//
		// The disassembler assumes the code was written by the assembler.
		// So if the code builds the upper part of rB without using an lui,
		// and then does a jmp or jsr with a nonzero immediate, the disassembler
		// writes out something that cannot be reassembled. The disassembler
		// emits the 0xE... opcode as "jlr" in this case, but the assembler
		// does not accept jlr.

		rb := bits(inst,5,3)
		imm := bits(inst,12,6)  // but we know bit 12 is 0
		switch bits(inst,2,0) { // the j2:j0 bits in the rA field
		case 0: // sys
			if rb != 0 || imm&1 != 0 {
				instructions[at] = fmt.Sprintf("die ; ILLEGAL OPCODE 0x%04X", inst)
			} else {
				instructions[at] = fmt.Sprintf("sys %d", imm)
			}
		case 1: // jsr
			if luiSeen && rb == bits(prevInst,2,0) {
				// lui+jlr with j's == 1 becomes jsr rB, target
				instructions[at-1] = "" // hide the lui
				instructions[at] = fmt.Sprintf("jsr %s, 0x%04X",
					RegNames[bits(inst,5,3)],
					(bits(prevInst,12,3)<<6) | bits(inst,12,6))
			} else if imm == 0 && rb != 0 { // becomes computed jsr rB
				instructions[at] = fmt.Sprintf("jsr %s", RegNames[bits(inst,5,3)])
			}
			// else do nothing - nonstandard sequence to be emitted as jlr
		case 2: // jmp
			if luiSeen && rb == bits(prevInst,2,0) {
				// lui+jlr with j's == 2 becomes jmp rB, target
				instructions[at-1] = "" // hide the lui
				instructions[at] = fmt.Sprintf("jmp %s, %d",
					RegNames[bits(inst,5,3)],
					(bits(prevInst,12,3)<<6) | bits(inst,12,6))
			} else if imm == 0 && rb != 0 { // becomes computed jmp rB
				instructions[at] = fmt.Sprintf("jmp %s", RegNames[bits(inst,5,3)])
			}
			// else do nothing - nonstandard sequence to be emitted as jlr
		default: // illegal
			instructions[at] = fmt.Sprintf("die ; ILLEGAL OPCODE 0x%04X", inst)
		}
	} else if inst == 0xFFC8 {
		// rewrite NEG r0 as nop
		instructions[at] = "nop"
	}
	return nil
}

func decode(op uint16, at int) string {
	// The key table has column "nbits". It specifies how many
	// upper bits of matching opcode are required to recognize the
	// instruction. If the nbits column holds 3, op&(0b111<<13)
	// must match the entry's opcode masked with the same mask. If
	// it does then we can get the signature and decode the rest of
	// the instruction. We could build a hashmap for this, but the
	// KeyEntry table isn't large and performance is fine.

	var found KeyEntry
	for _, ke := range KeyTable {
		mask := uint16(1 << ke.nbits) - 1
		mask <<= (16 - ke.nbits)
		if op&mask == ke.opcode&mask {
			found = ke
			break
		}
	}
	if found.nbits == 0 {
		// All the opcodes are taken, so this is probably a bug,
		// not e.g. an illegal instruction, etc.
		return "internal error: opcode not found"
	}

	var args string
	var format string
	switch found.signature {
	case RRI:
		// Special case for computing the branch target. We don't want to emit
		// the branch *offset* into the disassembly, we want the *target*.
		imm := bits(op,12,6)
		if bits(op,15,13) == 4 { // beq
			imm = uint16((int(imm)+at+1)&0x7F)
			format = "%s, %s, %d"
		} else {
			format = "%s, %s, 0x%02X"
		}
		args = fmt.Sprintf(format, RegNames[bits(op,2,0)], RegNames[bits(op,5,3)], imm)
	case RJX:
		args = fmt.Sprintf("%s, 0x%03X", RegNames[bits(op,2,0)], bits(op,12,3))
	case RRR:
		args = fmt.Sprintf("%s, %s, %s",
			RegNames[bits(op,2,0)], RegNames[bits(op,5,3)], RegNames[bits(op,8,6)])
	case RRX:
		args = fmt.Sprintf("%s, %s", RegNames[bits(op,2,0)], RegNames[bits(op,5,3)])
	case RXX:
		args = fmt.Sprintf("%s", RegNames[bits(op,2,0)])
	case XXX:
		args = ""
	default:
		args = fmt.Sprintf("internal error: unknown signature 0x%x", found.signature)
	}

	return fmt.Sprintf("%s %s", found.name, args)
}

// Hi and lo are inclusive bit numbers - "15,13" is the 3 MS bits of a uint16
func bits(op uint16, hi uint16, lo uint16) uint16 {
	b := hi - lo + 1 // if hi, low == 5,3 then b == 3
	var mask uint16 = 1<<b - 1 // 1<<3 == 8 so mask == 7 == 0b111
	return (op>>lo)&mask
}

func usage() {
	pr("Usage: dis [options] binary-file\nOptions:")
	flag.PrintDefaults()
	os.Exit(1)
}

