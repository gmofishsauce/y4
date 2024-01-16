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
const RHI uint16 = 8 // register, imm3, imm7 (jlr only)

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
	{"jlr", 4,  0xE000, RHI}, // reg, 3-bit imm, 7-bit positive imm

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
	{"y04", 10, 0xFF00, XXX},
	{"ior", 10, 0xFF40, RRX},
	{"iow", 10, 0xFF80, RRX},

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
	{"v02", 16, 0xFFFA, XXX},
	{"v03", 16, 0xFFFB, XXX},
	{"rtl", 16, 0xFFFC, XXX},
	{"hlt", 16, 0xFFFD, XXX},
	{"brk", 16, 0xFFFE, XXX},
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
// have no header. There can be up to 128kb of code, but programs are
// typically small because there's no compiler (yet). The opcode 0x0000
// is "load word from memory to register 0". This is meaningless because,
// like many RISC machines, r0 is a black hole. So the disassembler stops
// when it sees an aligned 0 word.

const MaxCodeSegLen = 64*1024 // uints

func disassemble(f *os.File) error {
	var b []byte = make([]byte, 2, 2) 
	var inst uint16
	var at int
	var err error

	for n, err := f.Read(b); n == 2 && err == nil && at < MaxCodeSegLen; n, err = f.Read(b) {
		if inst = binary.LittleEndian.Uint16(b[:]); inst == 0 {
			break
		}
		if (*qflag) {
			fmt.Println(decode(inst))
		} else {
			fmt.Printf("%5d: 0x%04X: %s\n", at, inst, decode(inst))
		}
		at++
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

func decode(op uint16) string {
	// So the key table has column "nbits". It specifies how many
	// upper bits of matching opcode are required to recognize the
	// instruction. If the nbits column holds 3, (op&0b111)<<13
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
		args = fmt.Sprintf("%s, %s, 0x%02X",
			RegNames[bits(op,2,0)], RegNames[bits(op,5,3)], bits(op,12,6))
	case RHI:
		args = fmt.Sprintf("%s, 0x%1X, 0x%02X",
			RegNames[bits(op,2,0)], bits(op,5,3), bits(op,12,6))
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

