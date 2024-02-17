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

// Fetch next instruction into ir.
func (y4 *y4machine) fetch() {
	if y4.ex != 0 {
		// double fault should have been handled in main loop.
		assert(y4.en, "double fault in fetch()")

		// an exception occurred during the previous cycle.
		y4.reg[Kern].spr[Irr] = y4.pc
		y4.reg[Kern].spr[Icr] = y4.ex
		y4.reg[Kern].spr[Imr] = word(y4.mode)

		y4.mode = Kern
		y4.pc = word(y4.ex)
		y4.en = false
		y4.ex = 0
	}

	mem := &y4.mem[y4.mode]
	y4.ir = mem.imem[y4.pc]

	// Control flow instructions will overwrite this in a later stage.
	// This implementation is sequential (does everything each clock cycle).
	y4.pc++
	if y4.pc == 0 {
		y4.ex = ExMachine // machine check - PC wrapped		
	}
}

// Pull out all the possible distinct field types into uint16s. The targets
// (op, i7, yop, etc.) are all non-architectural per-cycle and mostly mean
// e.g. multiplexer outputs in hardware. The remaining stages can act on the
// decoded values. Plausible additional decoding (which instructions have
// targets? Which target special registers?) is left to the execution code.
func (y4 *y4machine) decode() {
	y4.op = y4.ir.bits(15,13)	// base opcode
	y4.imm = y4.sxtImm()

	y4.xop = y4.ir.bits(11,9)
	y4.yop = y4.ir.bits(8,6)
	y4.zop = y4.ir.bits(5,3)
	y4.vop = y4.ir.bits(2,0)

	y4.isVop = y4.ir.bits(15,3) == 0x1FFF
	y4.isZop = !y4.isVop && y4.ir.bits(15,6) == 0x03FF
	y4.isYop = !y4.isVop && !y4.isZop && y4.ir.bits(15,9) == 0x007F
	y4.isXop = !y4.isVop && !y4.isZop && !y4.isYop && y4.ir.bits(15,12) == 0x000F
	y4.isBase = !y4.isVop && !y4.isZop && !y4.isYop && !y4.isXop

	y4.ra = y4.vop
	y4.rb = y4.zop
	y4.rc = y4.yop
}

// Set the ALU output and memory (for stores) data in the
// non-architectural per-cycle machine state. Again,
// somewhat like the eventual pipelined implementation.
func (y4 *y4machine) execute() {
	if y4.ex != 0 {
		// The program counter gets modified by the execution
		// stage, so we must not proceed if there has been any
		// exception caused by the fetch or decode activities.
		return
	}
	if y4.isBase {
		baseops[y4.op]()
	} else if y4.isXop {
		y4.alu3()
	} else if y4.isYop {
		yops[y4.yop]()
	} else if y4.isZop {
		y4.alu1()
	} else {
		if !y4.isVop {
			y4.decodeFailure("vop")
		}
		vops[y4.vop]()
	}
}

// For instructions that reference memory, special register space,
// or I/O space, do the operation. The computed address is in the alu
// (alu result) register and the execute phase must also have loaded
// the store data register.
func (y4 *y4machine) memory() {
	if y4.ex != 0 { // exception pending - don't modify memory
		return
	}

	// We always set the writeback register to the alu output. It
	// gets overwritten in the code below by memory, io, or spr
	// read, if any. In the writeback stage, it gets used, or it
	// just doesn't, depending on the instruction.
	y4.wb = word(y4.alu)

	if y4.op < 4 { // general register load or store
		mem := &y4.mem[y4.mode]
		switch y4.op {
		case 0:  // ldw
			y4.wb = word(mem.dmem[y4.alu])
			y4.wb |= word(mem.dmem[y4.alu+1]) << 8
		case 1:  // ldb
			y4.wb = word(mem.dmem[y4.alu])
		case 2:  // stw
			mem.dmem[y4.alu] = byte(y4.sd&0x00FF)
			mem.dmem[y4.alu+1] = byte(y4.sd>>8)
		case 3:  // stb
			mem.dmem[y4.alu] = byte(y4.sd)
		// no default
		}
	} else if y4.isYop { // special register or IO load or store
		switch y4.yop {
		case 0: // lsp (load special)
			y4.wb = y4.loadSpecial()
		case 1: // lio (load from io)
			y4.wb = y4.loadIO()
		case 2: // ssp (store special)
			y4.storeSpecial(y4.sd)
		case 3: // sio
			y4.storeIO(y4.sd)
		// no default
		}
	}
}

// return the value of the special register addressed by the ALU result
// from the previous stage. May set an exception, in which case the result
// value doesn't matter because it won't be written back to a register.
func (y4 *y4machine) loadSpecial() word {
	r := y4.alu&(SprSize-1) // 0..63
	switch r { // no default
	case PC:
		return y4.pc
	case Link:
		return y4.reg[y4.mode].spr[Link]
	case Irr, Icr, Imr, 5:
		if y4.mode == Kern {
			return y4.reg[y4.mode].spr[r]
		}
		y4.ex = ExIllegal
		return 0
	case CCLS:
		return word(y4.cyc&0xFFFF)
	case CCMS:
		return word((y4.cyc&0xFFFF0000) >> 16)
	}
	if y4.mode == User {
		y4.ex = ExIllegal
		return 0
	}
	switch {
	case r >= 8 && r < 16: // unused SPRs
		return 0; 
	case r >= 16 && r < 24: // user general registers
		return y4.reg[User].gen[r-16]
	case r >= 24 && r < 31: // user special registers
		if r == 25 { // user link register
			// Could allow the kernel to access the PC
			// here, or CCLS/CCMS, but it's stupid.
			return y4.reg[User].spr[Link]
		}
	case r >= 32:	// MMU - details TBD
		TODO()
		return 0
	}
	// All the cases should have been handled,
	// so this should not be reachable.
	assert(false, "missing case in loadSpecial()")
	return 0
}

func (y4 *y4machine) loadIO() word {
	TODO()
	return 0
}

func (y4 *y4machine) storeSpecial(val word) {
	r := y4.alu&(SprSize-1) // 0..63
	if y4.mode == User {
		if r == Link { // usermode can write its link register
			y4.reg[User].spr[Link] = val
		} else {
			y4.ex = ExIllegal
		}
		return
	}
	switch {
	case r == Irr, r == Icr, r == Imr, r == 5:
		y4.reg[Kern].spr[r] = val
	case r >= 16 && r < 24: // set user general register
		y4.reg[User].gen[r-16] = val
	case r == 25:
		y4.reg[User].spr[Link] = val
	case r >= 32:	// MMU - details TBD
		TODO()
	default:
		y4.ex = ExIllegal // likely double fault
	}
}

func (y4 *y4machine) storeIO(val word) {
	TODO()
}

// Write the result (including possible memory result) to a register.
// Stores and io writes are handled at memory time.
func (y4 *y4machine) writeback() {
	if y4.ex != 0 { // exception pending - don't update registers
		return
	}

	reg := y4.reg[y4.mode]
	if y4.op == 0 ||   // ldw
		y4.op == 1 ||  // ldb
		y4.op == 5 ||  // adi
		y4.op == 6 ||  // lui
		y4.isXop ||    // 3-operand alu
		(y4.isYop && y4.yop < 2) ||  // lsp or lio
		y4.isZop {     // single operand alu

		if y4.ra != 0 {
			reg.gen[y4.ra] = y4.wb
		}
	}
}

// ================================================================
// === The rest of this file is the implementation of execute() ===
// ================================================================

// The opcodes basically spread out to the right, using more and
// more leading 1-bits. The bits come in groups of 3, with the
// special case that 1110... is jlr and 1111... requires decoding
// the next three (XOP) bits. After that, 1111 111... requires
// decoding the next three bits, then 1111 111 111..., etc.
//
// The decoder already figured this out and set isx, xop, isy,
// yop, and so on. We just need to switch on them and do all
// the things.

type xf func()

// We need a function with a parameter for reporting decode
// failures (internal errors). Then we need wrappers of type
// xf for the tables.
func (y4 *y4machine) decodeFailure(msg string) {
	y4.dump()
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

var baseops []xf = []xf{
	y4.ldw,
	y4.ldb,
	y4.stw,
	y4.stb,
	y4.beq,
	y4.adi,
	y4.lui,
	y4.jlr,
}

var yops []xf = []xf {
	y4.lsp,
	y4.lio,
	y4.ssp,
	y4.sio,
	y4.y04,
	y4.y05,
	y4.y06,
	y4.yopFail,
}

var vops []xf = []xf {
	y4.rti,
	y4.rtl,
	y4.di,
	y4.ei,
	y4.hlt,
	y4.brk,
	y4.v06,
	y4.die,
}

// base operations

func (y4 *y4machine) ldw() {
	// We end up here for zero opcodes. These try
	// to load r0 which is the black hole register.
	// Instead of having them be noops, we call
	// them illegal instructions. This prevents
	// running uninitialized memory.
	if y4.ir == 0 {
		y4.ex = ExIllegal
		return
	}
	reg := y4.reg[y4.mode].gen
	y4.alu = uint16(reg[y4.rb]) + y4.imm
}

func (y4 *y4machine) ldb() {
	reg := y4.reg[y4.mode].gen
	y4.alu = uint16(reg[y4.rb]) + y4.imm
}

func (y4 *y4machine) stw() {
	reg := y4.reg[y4.mode].gen
	y4.alu = uint16(reg[y4.rb]) + y4.imm
	// no register writeback
	// memory operation handled in memory phase
}

func (y4 *y4machine) stb() {
	reg := y4.reg[y4.mode].gen
	y4.alu = uint16(reg[y4.rb]) + y4.imm
	// no register writeback
	// memory operation handled in memory phase
}

func (y4 *y4machine) beq() {
	reg := y4.reg[y4.mode].gen
	if reg[y4.rb] == reg[y4.ra] {
		y4.pc = word(uint16(y4.pc) + y4.imm)
	}
	// no standard register writeback
}

func (y4 *y4machine) adi() {
	reg := y4.reg[y4.mode].gen
	y4.alu = uint16(reg[y4.rb]) + y4.imm
}

func (y4 *y4machine) lui() {
	y4.alu = y4.imm
}

func (y4 *y4machine) jlr() {
	// the jlr opcode has bits [15..13] == 0b111, just like xops.
	// It's a jlr, not an xop, because bit 12, the MS bit of the
	// immediate value, has to be a 0. The decoder is supposed to
	// take care of this, but for sanity, we check here.
	if y4.ir.bits(15,12) != 0xE {
		y4.baseFail() // internal error
	}

	// There are three flavors, determined by the rA field, which
	// is overloaded as additional opcode bits here.
	switch y4.ra {
	case 0: // sys trap
		if y4.rb != 0 || y4.imm&1 == 1 || y4.imm == 0 || y4.imm > 30 {
			// 15 of the first 16 traps, represented by values 2..30,
			// are legal instructions. 32..62 are reserved for hardware.
			// Trap 0 is not legal because it resets the machine. The
			// kernel can do this by jmp 0.
			y4.ex = ExIllegal
			return
		}
		y4.ex = word(y4.imm)
	case 1: // jump and link
		y4.reg[y4.mode].spr[Link] = y4.pc
		y4.pc = word(uint16(y4.reg[y4.mode].gen[y4.rb]) + y4.imm)
	case 2: // jump
		y4.pc = word(uint16(y4.reg[y4.mode].gen[y4.rb]) + y4.imm)
	default:
		y4.ex = ExIllegal
	}
}

// xops - 3-operand ALU operations all handled here

func (y4 *y4machine) alu3() {
	reg := y4.reg[y4.mode].gen
	rs2 := uint16(reg[y4.rc])
	rs1 := uint16(reg[y4.rb])

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

func (y4 *y4machine) lsp() {
	reg := y4.reg[y4.mode].gen
	y4.alu = uint16(reg[y4.rb]) + y4.imm
}

func (y4 *y4machine) lio() {
	reg := y4.reg[y4.mode].gen
	y4.alu = uint16(reg[y4.rb]) + y4.imm
}

func (y4 *y4machine) ssp() {
	reg := y4.reg[y4.mode].gen
	y4.alu = uint16(reg[y4.rb]) + y4.imm
}

func (y4 *y4machine) sio() {
	reg := y4.reg[y4.mode].gen
	y4.alu = uint16(reg[y4.rb]) + y4.imm
}

func (y4 *y4machine) y04() {
	y4.ex = ExIllegal
}

func (y4 *y4machine) y05() {
	y4.ex = ExIllegal
}

func (y4 *y4machine) y06() {
	y4.ex = ExIllegal
}

// zops - 1-operand ALU operations all handled here

func (y4 *y4machine) alu1() {
	reg := y4.reg[y4.mode].gen
	rs1 := uint16(reg[y4.ra])

	switch y4.zop {
	case 0: //not
		y4.alu = ^rs1
		y4.hc = 0
	case 1: //neg
		y4.alu = 1 + ^rs1
		y4.hc = 0 // ???
	case 2: //swb
		y4.alu = rs1 >> 8 | rs1 << 8
		y4.hc = 0
	case 3: //sxt
		if rs1&0x80 != 0 {
			y4.alu = rs1 | 0xFF00
		} else {
			y4.alu = rs1 &^ 0xFF00
		}
		y4.hc = 0
	case 4: //lsr
		y4.hc = rs1&1
		y4.alu = rs1 >> 1
	case 5: //lsl
		if rs1&0x8000 == 0 {
			y4.hc = 0
		} else {
			y4.hc = 1
		}
		y4.alu = rs1 << 1
	case 6: //asr
		sign := rs1 & 0x8000
		y4.hc = rs1&1
		y4.alu = rs1 >> 1
		y4.alu |= sign
	case 7:
		y4.zopFail()
	}
}

// vops - 0 operand instructions

func (y4 *y4machine) rti() {
	if y4.mode == User {
		y4.ex = ExIllegal
		return
	}

	// This is acceptable because (1) the machine is not pipelined
	// and (2) the instruction doesn't do anything else but this.
	// In a pipelined implementation, this would be more complex.
	// Also note that we can enable interrupts when returning from
	// any interrupt or fault, because interrupts must have been
	// enabled for the interrupt or fault to have been taken.
	y4.ex = 0
	y4.en = true
	y4.pc = y4.reg[Kern].spr[Irr]
	y4.reg[Kern].spr[Irr] = 0
	y4.mode = byte(y4.reg[Kern].spr[Imr])
}

func (y4 *y4machine) rtl() {
	y4.pc = y4.reg[y4.mode].spr[Link]
}

func (y4 *y4machine) di() {
	if y4.mode == User {
		y4.ex = ExIllegal
		return
	}

	y4.en = false
}

func (y4 *y4machine) ei() {
	if y4.mode == User {
		y4.ex = ExIllegal
		return
	}

	y4.en = true
}

func (y4 *y4machine) hlt() {
	if y4.mode == User {
		y4.ex = ExIllegal
		return
	}

	y4.run = false
}

func (y4 *y4machine) brk() {
	if y4.mode == User {
		y4.ex = ExIllegal
		return
	}

	// for now
	y4.dump()
}

func (y4 *y4machine) v06() {
	y4.ex = ExIllegal
}

func (y4 *y4machine) die() {
	if y4.mode == User {
		y4.ex = ExIllegal
		return
	}

	y4.ex = ExIllegal
}
