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

func (y4 *y4machine) yopFail() {
	y4.decodeFailure("yop")
}

func (y4 *y4machine) zopFail() {
	y4.decodeFailure("zop")
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
	y4.yopFail,
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
	// We end up here for zero opcodes. These try
	// to load r0 which is the black hole register.
	// Instead of having them be noops, we call
	// them illegal instructions.
	if y4.ir == 0 {
		y4.ex = ExIllegal
		return
	}
	y4.alu = uint16(y4.reg[y4.rb]) + y4.imm
	y4.hasStandardWriteback = true
	// memory operation handled in memory phase
}

func (y4 *y4machine) ldb() {
	y4.alu = uint16(y4.reg[y4.rb]) + y4.imm
	y4.hasStandardWriteback = true
	// memory operation handled in memory phase
}

func (y4 *y4machine) stw() {
	y4.alu = uint16(y4.reg[y4.rb]) + y4.imm
	// no register writeback
	// memory operation handled in memory phase
}

func (y4 *y4machine) stb() {
	y4.alu = uint16(y4.reg[y4.rb]) + y4.imm
	// no register writeback
	// memory operation handled in memory phase
}

func (y4 *y4machine) beq() {
	if y4.reg[y4.rb] == y4.reg[y4.ra] {
		dbg("beq y4.imm == %d", y4.imm)
		y4.pc = word(uint16(y4.pc) + y4.imm)
	}
	// no standard register writeback
}

func (y4 *y4machine) adi() {
	y4.alu = uint16(y4.reg[y4.rb]) + y4.imm
	y4.hasStandardWriteback = true
}

func (y4 *y4machine) lui() {
	y4.alu = y4.imm
	y4.hasStandardWriteback = true
}

// xops - 3-operand ALU operations all handled here

func (y4 *y4machine) alu3() {
	rs2 := uint16(y4.reg[y4.rc])
	rs1 := uint16(y4.reg[y4.rb])
	y4.hasStandardWriteback = true

	switch (y4.xop) {
	case 0: // add
		full := uint32(rs2 + rs1)
		y4.alu = uint16(full&0xFFFF)
		y4.hc = uint16((full & 0x10000) >> 16)
	case 1: // adc
		full := uint32(rs2 + rs1 + y4.hc)
		y4.alu = uint16(full&0xFFFF)
		y4.hc = uint16((full & 0x10000) >> 16)
	case 2: // sub
		full := uint32(rs2 - rs1)
		y4.alu = uint16(full&0xFFFF)
		y4.hc = uint16((full & 0x10000) >> 16)
	case 3: // sbc
		full := uint32(rs2 - rs1 - y4.hc)
		y4.alu = uint16(full&0xFFFF)
		y4.hc = uint16((full & 0x10000) >> 16)
	case 4: // bic (nand)
		full := uint32(rs2 &^ rs1)
		y4.alu = uint16(full&0xFFFF)
		y4.hc = 0
	case 5: // bis (or)
		full := uint32(rs2 | rs1)
		y4.alu = uint16(full&0xFFFF)
		y4.hc = 0
	case 6: // xor
		full := uint32(rs2 ^ rs1)
		y4.alu = uint16(full&0xFFFF)
		y4.hc = 0
	case 7:
		y4.decodeFailure("alu3 op == 7")	
	}
}

// yops

func (y4 *y4machine) wrs() {
	TODO()
}

func (y4 *y4machine) rds() {
	TODO()
}

func (y4 *y4machine) lds() {
	TODO()
}

func (y4 *y4machine) sts() {
	TODO()
}

func (y4 *y4machine) y04() {
	TODO()
}

func (y4 *y4machine) ior() {
	TODO()
}

func (y4 *y4machine) iow() {
	TODO()
}

// zops - 1-operand ALU operations all handled here

func (y4 *y4machine) alu1() {
	rs1 := uint16(y4.reg[y4.ra])
	y4.hasStandardWriteback = true

	switch y4.zop {
	case 0: //not
		y4.alu = ^rs1
	case 1: //neg
		y4.alu = 1 + ^rs1
	case 2: //swb
		y4.alu = rs1 >> 8 | rs1 << 8
	case 3: //sxt
		if rs1&0x80 != 0 {
			y4.alu = rs1 | 0xFF00
		} else {
			y4.alu = rs1 &^ 0xFF00
		}
	case 4: //lsr
		y4.alu = rs1 >> 1
	case 5: //lsl
		y4.alu = rs1 << 1
	case 6: //asr
		sign := rs1 & 0x8000
		y4.alu = rs1 >> 1
		if sign != 0 {
			y4.alu |= sign
		}
	case 7:
		y4.zopFail()
	}
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
