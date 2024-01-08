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
	"flag"
	"fmt"
	"os"
)

var dflag = flag.Bool("d", false, "enable debug")

// An array (really slice) of MachineInstructions is returned by the parser
// for a successful parse and passed to the generator.
//
// MachineInstructions are created for any line that needs them - not for
// blank lines, comment-only lines, or pseudo instructions that only affect
// the symbol table. Make more than one of these for mnemonics that expand
// into multiple instructions.
//
// parts[0] is the key field. It always holds a symbol index. The ra, rb,
// and rc operands are in [1], [2], and [3] respectively. These hold either
// a symbol index (if MS bit clear) or the actual value, if the MS bit is
// set. This works because actual values are either 7-bit immediates, 10-bit
// immediates, or in the range 0..7 for register indexes. True pseudos, like
// .fill, can take 16-bit immediates as arguments; but these do not result
// in creation of MachineInstructions. Immediate values, when present, may
// be in the rB or rC field. 
//
// This arrangement enforces a limit of 32767 symbols on a compilation unit.
// In this first version of the assembler, at least, there is no linker; so
// every program must be a single compilation unit.

type MachineInstruction struct {
	parts [4]uint16
}

// These values index the parts[] array. They are also multiplied by 4
// to product the shift into the Signature of the instruction, below.
const Key uint16 = 0
const Ra uint16 = 1
const Rb uint16 = 2
const Rc uint16 = 3

// 
const IsSymbolIndex uint16 = 0 // bit not set if it's a symbol ref
const IsValue uint16 = 0x8000 // set in parts[n] if it's a value

// Table of mnemonics and their signatures

type KeyEntry struct {
	name string
	signature uint16
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
// There aren't any operations with 4 operands, but this isn't
// the place to check that.
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
var KeyTable []KeyEntry = []KeyEntry{
	// Operations with two registers and a 7-bit immediate
	{"adi",    sigFor(SeReg, SeReg, SeImm7)},
	{"beq",    sigFor(SeReg, SeReg, SeImm7)},
	{"lb",     sigFor(SeReg, SeReg, SeImm7)},
	{"lw",     sigFor(SeReg, SeReg, SeImm7)},
	{"sb",     sigFor(SeReg, SeReg, SeImm7)},
	{"sw",     sigFor(SeReg, SeReg, SeImm7)},
	{"lli",    sigFor(SeReg, SeReg, SeImm6)},

	// Special case - lui - one register, one 10-bit immed
	{"lui",   sigFor(SeReg, SeImm10, SeNone)},

	// 3-operand XOPs
	{"add",    sigFor(SeReg, SeReg, SeReg)},
	{"sub",    sigFor(SeReg, SeReg, SeReg)},
	{"addc",   sigFor(SeReg, SeReg, SeReg)},
	{"subb",   sigFor(SeReg, SeReg, SeReg)},
	{"nand",   sigFor(SeReg, SeReg, SeReg)},
	{"or",     sigFor(SeReg, SeReg, SeReg)},
	{"xor",    sigFor(SeReg, SeReg, SeReg)},

	// XOPs (or etc.) with fewer than 3 register arguments
	{"jalr",   sigFor(SeReg, SeReg, SeNone)},
	{"not",    sigFor(SeReg, SeNone, SeNone)},
	{"nop",    sigFor(SeNone, SeNone, SeNone)},
	{"hlt",    sigFor(SeNone, SeNone, SeNone)},
	{"neg",    sigFor(SeReg, SeNone, SeNone)},

	// Pseudo-ops that can accept 16-bit values
	{"li",     sigFor(SeReg, SeVal16, SeNone)},
	{".align", sigFor(SeVal16, SeNone, SeNone)},
	{".byte",  sigFor(SeVal16, SeNone, SeNone)},
	{".word",  sigFor(SeVal16, SeNone, SeNone)},
	{".space", sigFor(SeVal16, SeNone, SeNone)},
	{".str",   sigFor(SeString, SeNone, SeNone)},
	{".set",   sigFor(SeSym, SeVal16, SeNone)},
}

// Y4 assembler. A general theme with this assembler is that it has
// only limited dependencies on libraries. The goal is to eventually
// rewrite this in a simple language with limited libraries and self-
// host on homemade Y4.

func main() {
	flag.Parse()
	args := flag.Args()
	if *dflag {
		// LexerDebug = true
		ParserDebug = true
		GeneratorDebug = true
	}
	if len(args) != 1 {
		usage()
	}
	instructions, err := parse(args[0])
	if err != nil {
		fatal(fmt.Sprintf("%s: %s\n", args[0], err.Error()))
	}
	err = generate(instructions)
	if err != nil {
		fatal(fmt.Sprintf("%s: %s\n", args[0], err.Error()))
	}
}

func usage() {
	pr("Usage: asm [options] source-file\nOptions:")
	flag.PrintDefaults()
	os.Exit(1)
}

