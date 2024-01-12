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
	nbits uint16      // number of high bits required to recognize
	opcode uint16     // fixed opcode bits
	signature uint16  // see below
}

// Operations (key symbols) can have up to three operands. The operand
// types are represented as SignatureElements. Bits 3:0 of the signature
// are always 0. The ra signature element is in bits 7:4, rb in 11:8 and
// rc in 15:12. Note that operands don't necessarily fit in the fields of
// a MachineInstruction - .fill, for example, can take a 16-bit operand -
// but it doesn't result in the creation of any MachineInstructions; the
// largest value in a MachineInstruction is a 10-bit immediate.
// SignatureElements can be combined by shifts into a uint16, forming a
// signature. If the signature is 0, there are no operands. If it's less
// than 0x100, there is 1 operand, 0x1000 for 2, or larger for 3 operands.

type SignatureElement uint16

const (
	SeNone = SignatureElement(0)
	SeReg = SignatureElement(1)      // Field is a register
	SeImm6 = SignatureElement(2)     // Field is a 6-bit unsigned
	SeImm7 = SignatureElement(3)     // Field is a 7-bit signed
	SeImm10 = SignatureElement(4)    // Field is a 10-bit unsigned
	SeVal16 = SignatureElement(5)    // Field is a 16-bit value
	SeSym = SignatureElement(6)      // Field is a new symbol
	SeString = SignatureElement(7)   // Field is a quoted string
)

// Make a Signature from up to three SignatureElements.
func sigFor(ra SignatureElement, rb SignatureElement, rc SignatureElement) uint16 {
	return uint16( ((rc&0xF)<<(4*Rc)) | ((rb&0xF)<<(4*Rb)) | (ra&0xF)<<(4*Ra) )
}

// Extract the key, ra, rb, or rc signature element
func getSig(value uint16, whichElement uint16) SignatureElement {
	whichElement &= 0x3
	whichElement *= 4
	return SignatureElement((value>>whichElement)&0xF)
}

// Return the number of operands represented by this Signature.
func numOperands(signature uint16) uint16 {
	if signature == 0 {
		return 0
	}
	if signature < 0x100 {
		return 1
	}
	if signature < 0x1000 {
		return 2
	}
	return 3
}

// The allowed mnemonics and their signatures. This table is
// entered into the symbol table during initialization.
// Keep the entires in this table in the same order, with the
// same grouping, as the rules in ../asm/asm.
var KeyTable []KeyEntry = []KeyEntry {
	// Operations with two registers and a 7-bit immediate
	{"ldw", 3,  0x0000, sigFor(SeReg, SeReg, SeImm7)},
	{"ldb", 3,  0x2000, sigFor(SeReg, SeReg, SeImm7)},
	{"stw", 3,  0x4000, sigFor(SeReg, SeReg, SeImm7)},
	{"stb", 3,  0x6000, sigFor(SeReg, SeReg, SeImm7)},
	{"beq", 3,  0x8000, sigFor(SeReg, SeReg, SeImm7)},
	{"adi", 3,  0xA000, sigFor(SeReg, SeReg, SeImm7)},
	{"lui", 3,  0xC000, sigFor(SeReg, SeImm10, SeNone)},
	{"jlr", 4,  0xE000, sigFor(SeReg, SeReg, SeImm6)},

	// 3-operand XOPs
	{"add", 7,  0xF000, sigFor(SeReg, SeReg, SeReg)},
	{"adc", 7,  0xF200, sigFor(SeReg, SeReg, SeReg)},
	{"sub", 7,  0xF400, sigFor(SeReg, SeReg, SeReg)},
	{"sbb", 7,  0xF600, sigFor(SeReg, SeReg, SeReg)},
	{"bic", 7,  0xF800, sigFor(SeReg, SeReg, SeReg)}, // nand
	{"bis", 7,  0xFA00, sigFor(SeReg, SeReg, SeReg)}, // or
	{"xor", 7,  0xFC00, sigFor(SeReg, SeReg, SeReg)},

	// 2 operand YOPs
	{"wrs", 10, 0xFE00, sigFor(SeReg, SeReg, SeNone)},
	{"rds", 10, 0xFE40, sigFor(SeReg, SeReg, SeNone)},
	{"lds", 10, 0xFE80, sigFor(SeReg, SeReg, SeNone)},
	{"sts", 10, 0xFEC0, sigFor(SeReg, SeReg, SeNone)},
	{"ior", 10, 0xFF00, sigFor(SeReg, SeReg, SeNone)},
	{"sys", 10, 0xFF40, sigFor(SeReg, SeReg, SeNone)},
	{"FF8", 10, 0xFF80, sigFor(SeReg, SeReg, SeNone)}, // unassigned

	// 1 operand ZOPs
	{"not", 13, 0xFFC0, sigFor(SeReg, SeNone, SeNone)},
	{"neg", 13, 0xFFC8, sigFor(SeReg, SeNone, SeNone)},
	{"sxt", 13, 0xFFD0, sigFor(SeReg, SeNone, SeNone)},
	{"swb", 13, 0xFFD8, sigFor(SeReg, SeNone, SeNone)},
	{"lsr", 13, 0xFFE0, sigFor(SeReg, SeNone, SeNone)},
	{"lsl", 13, 0xFFE8, sigFor(SeReg, SeNone, SeNone)},
	{"asr", 13, 0xFFF0, sigFor(SeReg, SeNone, SeNone)},

	// 0 operand VOPs
	{"sys", 16, 0xFFF8, sigFor(SeNone, SeNone, SeNone)}, // syscall
	{"srt", 16, 0xFFF9, sigFor(SeNone, SeNone, SeNone)}, // sysreturn
	{"FFA", 16, 0xFFFA, sigFor(SeNone, SeNone, SeNone)}, // unassigned
	{"FFB", 16, 0xFFFB, sigFor(SeNone, SeNone, SeNone)}, // unassigned
	{"rtl", 16, 0xFFFC, sigFor(SeNone, SeNone, SeNone)}, // return link
	{"brk", 16, 0xFFFD, sigFor(SeNone, SeNone, SeNone)},
	{"hlt", 16, 0xFFFE, sigFor(SeNone, SeNone, SeNone)},
	{"die", 16, 0xFFFF, sigFor(SeNone, SeNone, SeNone)}, // illegal

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
	return "TODO: decode combined instruction patterns"
}

func decode(inst uint16) string {
	// So the key table has column "nbits". It specifies how many
	// upper bits of matching opcode are required to recognize the
	// instruction. If the nbits column holds 3, then (inst&7)<<13
	// must match the entry's opcode. If it does then we can get
	// the signature and decode the rest of the instruction. We
	// could build a hashtable for this, but the KeyEntry table
	// isn't large and performance is fine.

	var found KeyEntry
	for _, ke := range KeyTable {
		mask := uint16(1 << ke.nbits) - 1
		mask <<= (16 - ke.nbits)
		if inst&mask == ke.opcode&mask {
			found = ke
			break
		}
	}
	if found.nbits == 0 {
		return "internal error: opcode not found"
	}
	return found.name
}

func usage() {
	pr("Usage: dis [options] binary-file\nOptions:")
	flag.PrintDefaults()
	os.Exit(1)
}

