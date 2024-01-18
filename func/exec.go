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
	// TODO
}

var vops []xf = []xf {
	// TODO
}

// base operations

func (y4 *y4machine) ldw() {
}

func (y4 *y4machine) ldb() {
}

func (y4 *y4machine) stw() {
}

func (y4 *y4machine) stb() {
}

func (y4 *y4machine) beq() {
}

func (y4 *y4machine) adi() {
}

func (y4 *y4machine) lui() {
}

// 3-operand ALU operations all handled here

func (y4 *y4machine) alu3() {
}

// 1-operand ALU operations all handled here

func (y4 *y4machine) alu1() {
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
	y4.decodeFailure("miss")
}
