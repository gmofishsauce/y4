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
	"fmt"
)

// The opcodes basically spread out to the right, using more and
// more leading 1-bits. The bits come in groups of 3, with the
// special case that 1110... is jlr and 1111... requires decoding
// the next three (XOP) bits. After that, 1111 111... requires
// decoding the next three bits, then 1111 111 111..., etc.
//
// The decoder already figured this out and set isx, xop, isy,
// yop, and so on. We just need to switch on them (or not, if
// a bunch of ops are very similar, like the xops).

type xf func()

// We need a function with a parameter for reporting decode
// failures. Then we need wrappers of type xf for tables.
func (y4 *y4machine) decodeFailure(msg string) {
	pr(fmt.Sprintf("opcode 0x%04X\n", y4.op))
	panic("executeSequential(): decode failure: " + msg)
}

func (y4 *y4machine) baseFail() {
	y4.decodeFailure("base")
}

// Important: any execution function that expects to have its
// result written back to the register file needs to set either
// hasStandardWriteback or hasSpecialWriteback. These are used
// at the writeback stage of the instruction to gate writing
// to either the special or general register array.

var baseops []xf = []xf{
	y4.ldw,
	y4.ldb,
	y4.stw,
	y4.stb,
	y4.beq,
	y4.adi,
	y4.lui,
	y4.baseFail,
}

var yops []xf = []xf {
	y4.wrs,
	y4.rds,
	y4.lds,
	y4.sts,
	y4.y04, // unused opcode
	y4.ior,
	y4.iow,
}

var vops []xf = []xf {
	y4.sys,
	y4.srt,
	y4.v02, // unused opcode
	y4.v03, // unused opcode
	y4.rtl,
	y4.hlt,
	y4.brk,
	y4.die,
}

// base operations

func (y4 *y4machine) ldw() {
	y4.alu = uint16(y4.reg[y4.rb]) + y4.i7
	y4.hasStandardWriteback = true
}

func (y4 *y4machine) ldb() {
	y4.alu = uint16(y4.reg[y4.rb]) + y4.i7
	y4.hasStandardWriteback = true
}

func (y4 *y4machine) stw() {
	y4.alu = uint16(y4.reg[y4.rb]) + y4.i7
	// no register writeback
	// memory operation handled in memory phase
}

func (y4 *y4machine) stb() {
	y4.alu = uint16(y4.reg[y4.rb]) + y4.i7
	// no register writeback
	// memory operation handled in memory phase
}

func (y4 *y4machine) beq() {
	if y4.reg[y4.rb] == y4.reg[y4.ra] {
		y4.pc = word(uint16(y4.pc) + y4.i7)
	}
}

func (y4 *y4machine) adi() {
	y4.alu = uint16(y4.reg[y4.rb]) + y4.i7
}

func (y4 *y4machine) lui() {
	y4.alu = y4.i10
}

// xops - 3-operand ALU operations all handled here

func (y4 *y4machine) alu3() {
}

// yops

func (y4 *y4machine) wrs() {
}

func (y4 *y4machine) rds() {
}

func (y4 *y4machine) lds() {
}

func (y4 *y4machine) sts() {
}

func (y4 *y4machine) y04() {
}

func (y4 *y4machine) ior() {
}

func (y4 *y4machine) iow() {
}

// zops - 1-operand ALU operations all handled here

func (y4 *y4machine) alu1() {
}

// vops - 0 operand instructions

func (y4 *y4machine) sys() {
}

func (y4 *y4machine) srt() {
}

func (y4 *y4machine) v02() {
}

func (y4 *y4machine) v03() {
}

func (y4 *y4machine) rtl() {
}

func (y4 *y4machine) hlt() {
}

func (y4 *y4machine) brk() {
}

func (y4 *y4machine) die() {
}

func (y4 *y4machine) executeSequential() {
	if y4.isbase {
		baseops[y4.op]()
	} else if y4.isx {
		y4.alu3()
	} else if y4.isy {
		yops[y4.yop]()
	} else if y4.isz {
		y4.alu1()
	} else {
		if !y4.isv {
			y4.decodeFailure("vop")
		}
		vops[y4.vop]()
	}
}
