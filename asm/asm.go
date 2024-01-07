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
// The key field always holds a symbol index. The ra, rb, and rc fields
// hold either symbol indices (if MS bit clear) or the actual value, if
// the MS bit is set. This works because actual values are either 7-bit
// immediates, 10-bit immediates, or in the range 0..7 for register indexes.
// True pseudos, like .space, can take 16-bit immediates as arguments, but
// do not result in creation of MachineInstructions. Immediate values, when
// present, are held in the rC field. 
//
// This arrangement enforces a limit of 32767 symbols on a compilation
// unit. In this first version of the assembler, at least, there is no
// linker; so every program must be a single compilation unit.

type MachineInstruction struct {
	key uint16 // unconditionally a symbol index
	rc uint16 // rc field value or immediate value or symbol
	rb uint16 // rb field or symbol
	ra uint16 // ra field or symbol
}

// Table of mnemonics and their signatures

type KeyEntry struct {
	name string
	signature uint16
}

// Operations (key symbols) can have up to three operands. The operand
// types are represented as SignatureElements. The ra signature element
// is in bits 3:0, rb in 7:4, and rc in 11:8. Note that operands don't
// necessarily fit in the fields of a MachineInstruction - .space, for
// example, can take a 16-bit operand, but doesn't result in the creation
// of any MachineInstructions. SignatureElements can be combined by shifts
// into a uint16. If its signature is 0, there are no operands. If it's
// less than 0x10, there is 1 operand, etc.

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
	return uint16( ((rc&0xF)<<8) | ((rb&0xF)<<4) | (ra&0xF) )
}

// Extract the key, ra, rb, or rc signature element
func getSig(value uint16, elem SignatureElement) SignatureElement {
	elem &= 0x3
	elem *= 4
	return SignatureElement((value>>elem)&0xF)
}

// Return the number of operands represented by this Signature.
// There aren't any operations with 4 operands, but this isn't
// the place to check that.
func numOperands(sigHolder uint16) int {
	if sigHolder == 0 {
		return 0
	}
	if sigHolder < 0x10 {
		return 1
	}
	if sigHolder < 0x100 {
		return 2
	}
	if sigHolder < 0x1000 {
		return 3
	}
	return 4
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
		LexerDebug = true
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

