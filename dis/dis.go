/*
Copyright © 2022 Jeff Berkowitz (pdxjjb@gmail.com)

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
const SRX uint16 = 4 // special, register
const RRX uint16 = 5 // register, register
const RXX uint16 = 6 // register
const XXX uint16 = 7 // no arguments
const RHI uint16 = 8 // register, imm3, imm7 (jlr only/special case)

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
	{"adi", 3,  0xA000, RRI},
	{"lui", 3,  0xC000, RJX},
	{"BUG", 4,  0xE000, RHI}, // special case(s)

	// 3-operand XOPs
	{"add", 7,  0xF000, RRR},
	{"adc", 7,  0xF200, RRR},
	{"sub", 7,  0xF400, RRR},
	{"sbb", 7,  0xF600, RRR},
	{"bic", 7,  0xF800, RRR},
	{"bis", 7,  0xFA00, RRR},
	{"xor", 7,  0xFC00, RRR},

	// 2 operand YOPs
	{"lds", 10, 0xFE00, SRX},
	{"sts", 10, 0xFE40, SRX},
	{"rds", 10, 0xFE80, SRX},
	{"wrs", 10, 0xFEC0, SRX},
	{"ior", 10, 0xFF00, RRX},
	{"iow", 10, 0xFF40, RRX},
	{"y07", 10, 0xFF80, XXX},

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
// have no header. They consist of up to four sections: user code (at 0,
// length 128kB), user data (at 128k in file, length 64kB), kernel code
// (at 192k in file, length 128kB), and kernel data (at 320k, length 64kB).
// The disassembler ignores the data segments. It disassembles both code
// segments with the appropriate pseudo instructions to align them (space
// them at 0 and 192k). It stops processing the current segment if it sees
// the illegal opcode 0 (which causes an illegal instruction trap). Since
// there are no 16-bit immediate values in the ISA, this is reliable.

const K64 int64 = 64*1024

func disassemble(f *os.File) error {
	if err := disassembleSection(f, 0); err != nil {
		return fmt.Errorf("user code section: %s", err.Error())
	}
	if err := disassembleSection(f, 3*K64); err != nil {
		return fmt.Errorf("kernel code section: %s", err.Error())
	}
	return nil
}

func disassembleSection(f *os.File, pos int64) error {
	var b []byte = make([]byte, 2, 2) 
	var inst uint16
	var at int // 16-bit instruction index in section
	var err error

	// As we disassemble, we must watch for jmp and jsr. These mnemonics assemble
	// to an LUI followed by an opcode, called "jlr" in the architecture document,
	// that doesn't have a separate definition in the assembler; it subsumes the
	// lui. In this case we don't emit the preceding lui, which we necessarily have
	// already disassembled. So we have to hold each line, even lui, to see if it's
	// followed by a jlr (0xEnnn) opcode that subsumes it.
	var linebuf string

	for n, err := f.ReadAt(b, pos); n == 2 && err == nil && at < int(2*K64); n, err = f.ReadAt(b, pos) {
		if inst = binary.LittleEndian.Uint16(b[:]); inst == 0 {
			break
		}
		if len(linebuf) != 0 && bits(inst,15,12) != 0xE {
			fmt.Println(linebuf)
			// else it will be consumed below
		}
		if (*qflag) {
			linebuf = decode(inst, at)
		} else {
			linebuf = fmt.Sprintf("%5d: 0x%04X: %s\n", at, inst, decode(inst, at))
		}
		at++      // instruction index
		pos += 2  // file position
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	if len(linebuf) != 0 {
		fmt.Println(linebuf)
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
	switch found.signature {
	case RRI:
		// Special case for computing the branch target. We don't want to emit
		// the branch *offset* into the disassembly, we want the *target*.
		imm := bits(op,12,6)
		if bits(op,15,13) == 4 { // beq
			imm = uint16((int(imm)+at+1)&0x7F)
			dbg("### imm = 0x%x at = %d", imm, at)
		}
		args = fmt.Sprintf("%s, %s, 0x%02X",
			RegNames[bits(op,2,0)], RegNames[bits(op,5,3)], imm)
	case RHI:
		// A bit ugly but in this case we have to return the whole instruction
		// here, rather than just the args.
		return decodeJLR(op)
	case RJX:
		args = fmt.Sprintf("%s, 0x%03X", RegNames[bits(op,2,0)], bits(op,12,3))
	case RRR:
		args = fmt.Sprintf("%s, %s, %s",
			RegNames[bits(op,2,0)], RegNames[bits(op,5,3)], RegNames[bits(op,8,6)])
	case SRX:
		args = fmt.Sprintf("%s, %s", SprNames[bits(op,2,0)], RegNames[bits(op,5,3)])
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

func decodeJLR(op uint16) string {
	if bits(op,12,6) < 64 && (bits(op,12,6)&1) == 0 && bits(op,5,3) == 0 && bits(op,2,0) == 0 {
		// The immediate is an even number < 64; ra and rb fields are both 0: system call trap
		return fmt.Sprintf("sys %d", bits(op,12,6))
	} else if bits(op,2,0) != 0 && bits(op,5,3) == 0 {
		// The ra field is nonzero and the rb field is 0: conventional jump and link, called jsr
		if bits(op,12,6) == 0 {
			return fmt.Sprintf("jsr %s", RegNames[bits(op,2,0)])
		} else {
			return fmt.Sprintf("jsr %s, 0x%04X", RegNames[bits(op,2,0)], bits(op,12,6))
		}
	} else if bits(op,2,0) != 0 && bits(op,5,3) == 1 {
		// The ra field is nonzero and the rb field is 1: conventional jump, called jmp
		if bits(op,12,6) == 0 {
			return fmt.Sprintf("jmp %s", RegNames[bits(op,2,0)])
		} else {
			return fmt.Sprintf("jmp %s, 0x%04X", RegNames[bits(op,2,0)], bits(op,12,6))
		}
	}
	// It's not one of those cases - maybe it's a sys instruction with an odd offset
	// or the rb field is > 1 - disassemble as die with a comment containing the opcode
	return fmt.Sprintf("die ; ILLEGAL OPCODE 0x%04X", op)
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

