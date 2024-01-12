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

var dflag = flag.Bool("d", false, "enable debug")

// These values index the parts[] array. They are also multiplied by 4
// to product the shift into the Signature of the instruction, below.
const Key uint16 = 0
const Ra uint16 = 1
const Rb uint16 = 2
const Rc uint16 = 3

// Table of mnemonics and their signatures

type KeyEntry struct {
	name string
	nbits uint16     // number of high bits required to recognize
	opcode uint16    // fixed opcode bits
	signature uint16 // see below
}

const (
	X uint16 = 0 // this argument doesn't exist for this opcode
	I uint16 = 1 // argument is a 7-bit immediate in bits 12:6
	J uint16 = 2 // argument is a 10-bit immediate in bits 12:3
	R uint16 = 3 // argument is a general register in 8:6, 5:3, or 2:0
	S uint16 = 4 // argument is a special register always in 2:0
)

// The specifies run from least signficant bits to most
//                 rA   | rB, imm, or etc.
//                 ~~~    ~~~~~~~~~~~~~~~~
const RRI uint16 = R    | R<<4 | I<<8 // load, store, adi, jlr, beq
const RJX uint16 = R    | J<<4 | X    // lui
const RRR uint16 = R    | R<<4 | R<<8 // XOPs
const SRX uint16 = S    | R<<4 | X    // YOPs with special registers
const RRX uint16 = R    | R<<4 | X    // YOPs with general registers
const RXX uint16 = R    | X    | X    // ZOPs with one general register
const XXX uint16 = X    | X    | X    // VOPs with no arguments

// Names of the registers, indexed by field content
var RegNames []string = []string {
	"r0", "r1", "r2", "r3", "r4", "r5", "r6", "r7",
}

// Names of special registers, indexed by field content
var SprNames []string = []string {
	"psw", "lnk", "pc", "err3", "err4", "err5", "err6", "err7",
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
	{"jlr", 4,  0xE000, RRI},

	// 3-operand XOPs
	{"add", 7,  0xF000, RRR},
	{"adc", 7,  0xF200, RRR},
	{"sub", 7,  0xF400, RRR},
	{"sbb", 7,  0xF600, RRR},
	{"bic", 7,  0xF800, RRR},
	{"bis", 7,  0xFA00, RRR},
	{"xor", 7,  0xFC00, RRR},

	// 2 operand YOPs
	{"wrs", 10, 0xFE00, SRX},
	{"rds", 10, 0xFE40, SRX},
	{"lds", 10, 0xFE80, SRX},
	{"sts", 10, 0xFEC0, SRX},
	{"ior", 10, 0xFF00, RRX},
	{"iow", 10, 0xFF40, RRX},
	{"FF8", 10, 0xFF80, XXX},

	// 1 operand ZOPs
	{"not", 13, 0xFFC0, RXX},
	{"neg", 13, 0xFFC8, RXX},
	{"sxt", 13, 0xFFD0, RXX},
	{"swb", 13, 0xFFD8, RXX},
	{"lsr", 13, 0xFFE0, RXX},
	{"lsl", 13, 0xFFE8, RXX},
	{"asr", 13, 0xFFF0, RXX},

	// 0 operand VOPs
	{"sys", 16, 0xFFF8, XXX},
	{"srt", 16, 0xFFF9, XXX},
	{"FFA", 16, 0xFFFA, XXX},
	{"FFB", 16, 0xFFFB, XXX},
	{"rtl", 16, 0xFFFC, XXX},
	{"brk", 16, 0xFFFD, XXX},
	{"hlt", 16, 0xFFFE, XXX},
	{"die", 16, 0xFFFF, XXX},

	// aliases for other instructions
	// the disassembler has to special case these "by hand"
	// {"lli",     0xA000, sigFor(SeReg, SeImm6, SeNone)},  // adi rT, rS, imm&0x3F
	// {"ldi",     0xFFFF, sigFor(SeReg, SeVal16, SeNone)}, // lui ; adi
	// {"nop",     0xA000, sigFor(SeNone, SeNone, SeNone)}, // adi r0, r0, 0
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
// have no header. There can be up to 128kb of code, but programs are
// typically small because there's no compiler (yet). The opcode 0x0000
// is "load word from memory to register 0". This is meaningless because,
// like many RISC machines, r0 is a black hole that reads as 0. So the
// disassembler stops when it sees an aligned 0 word. The disassembler
// must wait before flushing out each instruction's disassembled form
// because some two-instruction pairs are disassembled as a single line
// with a single mnemonic.

const MaxCodeSegLen = 64*1024 // uints

func disassemble(f *os.File) error {
	var b []byte = make([]byte, 2, 2) 
	var thisInst uint16
	var prevInst uint16
	var thisLine string
	var prevLine string
	var at int
	var err error

	for n, err := f.Read(b); n == 2 && err == nil && at < MaxCodeSegLen; n, err = f.Read(b) {
		thisInst = binary.LittleEndian.Uint16(b[:])
		if thisInst == 0 {
			break
		}
		if isCombinable(prevInst, thisInst) {
			prevLine = ""
			thisLine = decodeCombined(prevInst, thisInst)
		} else {
			thisLine = decode(thisInst)
		}
		if len(prevLine) != 0 {
			fmt.Printf("0x%02X %s\n", prevInst, prevLine)
		}
		at++
		prevInst = thisInst
		prevLine = thisLine
	}
	if len(prevLine) != 0 {
		fmt.Printf("0x%02X %s\n", prevInst, prevLine)
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

func isCombinable(prevInst uint16, thisInst uint16) bool {
	TODO()
	return false
}

func decodeCombined(prevInst uint16, thisInst uint16) string {
	TODO()
	return "TODO: decode combined instruction patterns"
}

func decode(op uint16) string {
	// So the key table has column "nbits". It specifies how many
	// upper bits of matching opcode are required to recognize the
	// instruction. If the nbits column holds 3, then (op&7)<<13
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
		return "internal error: opcode not found"
	}

	var args string
	switch found.signature {
	case RRI:
		args = fmt.Sprintf("%s %s 0x%02X",
			RegNames[bits(op,2,0)], RegNames[bits(op,5,3)], bits(op,12,6))
	case RJX:
		args = fmt.Sprintf("%s 0x%03X", RegNames[bits(op,2,0)], bits(op,12,3))
	case RRR:
		args = fmt.Sprintf("%s %s %s",
			RegNames[bits(op,2,0)], RegNames[bits(op,5,3)], RegNames[bits(op,8,6)])
	case SRX:
		args = fmt.Sprintf("%s %s", SprNames[bits(op,2,0)], RegNames[bits(op,5,3)])
	case RRX:
		args = fmt.Sprintf("%s %s", RegNames[bits(op,2,0)], RegNames[bits(op,5,3)])
	case RXX:
		args = fmt.Sprintf("%s", RegNames[bits(op,2,0)])
	case XXX:
		args = ""
	default:
		args = fmt.Sprintf("internal error: unknown signature 0x%x", found.signature)
	}

	return fmt.Sprintf("%s %s", found.name, args)
}

// Hi and lo are inclusive bit numbers
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

