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
	"flag"
	"os"
)

var dflag = flag.Bool("d", false, "enable debugging")

// Functional simulator for y4 instruction set

const K = 1024
const IOSize = 64	// 64 words of I/O space
const SprSize = 64	// 64 special registers, per Mode
const PC = 0		// Special register 0 is PC
const Link = 1		// Special register 1 is Link, per Mode
const Irr = 2       // Kernel only interrupt return register SPR
const Icr = 3		// Kernel only interrupt cause register SPR

const User = 0		// Mode = User
const Kern = 1		// Mode = Kernel

type word uint16

// Exception types. These must be even numbers less than 64, so
// there are 32 distinct types. The first 16 types 0..30 are
// accessible as opcodes SYS 0 through SYS 30. The second 16,
// 32..62, are reserved for the hardware to inject when an
// exceptional condition is detected.

const ExIllegal word = 32 // illegal instruction
const ExMemory word = 48  // Page fault or unaligned access
const ExMachine word = 62 // machine check

type y4mem struct { // per mode
	imem []word // code space
	dmem []byte // data space
}

type y4reg struct { // per mode
	gen []word // general registers
	spr []word // special registers
}

type y4machine struct {
	cyc uint64  // cycle counter
	mem []y4mem // [0] is user space, [1] is kernel
	reg []y4reg // [0] is user space, [1] is kernel
	io  []word	// i/o space, accesible only in kernel mode
	pc word

	// Non-architectural state that persists beyond an instruction
	run bool    // run/stop flag
	mode byte   // current mode, user = 0, kernel = 1

	// Non-architectural state used within an instruction
	ex word		// exception code
	ir word     // instruction register
	hc uint16   // hidden carry bit, 1 or 0

	// These variables are a programming convenience
	// They hold the output of the instruction decoder
	// They are all set at fetch time.
	op, imm uint16
	xop, yop, zop, vop uint16
	isXop, isYop, isZop, isVop, isBase bool
	ra, rb, rc uint16

	// These are cleared at fetch time and set during execution as
	// a programming convenience. The save the effort to recompute
	// which instructions have writebacks.
	hasStandardWriteback bool // wb => reg[rA] at writeback
	hasSpecialWriteback bool  // wb => spr[rA] at writeback

	// Non-architectural state set at execute or memory. These
	// will evolve into pipeline registers in the future pipelined
	// simulation.
	//
	// The alu result is computed at execution time. If there
	// is a load or store, it is the address in all cases.
	//
	// If there's a store, the source data is set at execution
	// time and stored in sd. For a load, the data is placed in
	// the writeback register (wb) at memory time. If there's a
	// 16-bit write, the LS bits go at the byte addressed by the
	// alu value and the MS bits next byte i.e. "little endian".
	//
	// The instruction result, if any, is computed at execute time
	// or, if there's a load, at memory time, and placed in the wb
	// register. The wb register is written to either a general or
	// special register at writeback time as required by the opcode.
	alu uint16 // temporary alu result register
	sd word    // memory source data register
	wb word    // register writeback (instruction result)
}

var y4 y4machine = y4machine {
	mem: []y4mem{
		{imem: make([]word, 64*K, 64*K), dmem: make([]byte, 64*K, 64*K)}, // user
		{imem: make([]word, 64*K, 64*K), dmem: make([]byte, 64*K, 64*K)}, // kernel
	},
	reg: []y4reg{
		{gen: make([]word, 8, 8), spr: make([]word, SprSize, SprSize)}, // user
		{gen: make([]word, 8, 8), spr: make([]word, SprSize, SprSize)}, // kernel
	},
	io: make([]word, IOSize, IOSize),
}

func (y4 *y4machine) reset() {
	y4.cyc = 0
	y4.pc = 0
	y4.run = true
	y4.mode = Kern
	y4.ex = 0
}

func main() {
	var err error
	var n int

	flag.Parse()
	args := flag.Args()
	if len(args) != 1 {
		usage()
	}
	dbEnabled = *dflag
	binPath := args[0]

	if n, err = y4.load(binPath); err != nil {
		fatal(fmt.Sprintf("loading %s: %s", binPath, err.Error()))
	}
	dbg("loaded %d bytes", n)

	// reset here in main so that run() can act as "continue" (TBD)`
	dbg("start")
	y4.reset()
	err = y4.simulate()
	if err != nil {
		fatal(fmt.Sprintf("error: running %s: %s", binPath, err.Error()))
		os.Exit(2)
	}
	dbg("done")
}

// Run the simulator. There must already be something loaded in imem.

func (y4 *y4machine) simulate() error {
	// The simulator is written as a rigid set of parameterless functions
	// that act on shared machine state. This will make it simpler to
	// simulate pipelining later.
	//
	// Sequential implementation: everything happens in each machine cycle.
	// It happens in the order of a pipelined machine, though, to make
	// converting this to a pipelined simulation easier in the future.

	for ; y4.run ; y4.cyc++ {
		if y4.ex != 0 {
			// All exceptions cause aborts for now.
			fmt.Printf("exception %d\n", y4.ex)
			break
		}
		y4.fetch()
		y4.decode()
		y4.execute()
		y4.memory()
		y4.writeback()
		if *dflag {
			y4.dump()
			var toss []byte = []byte{0}
			os.Stdin.Read(toss)
		}
	}
	y4.dump()
	return nil
}

// Get the bits from hi:lo inclusive as a small uint16
// Example: w := 0xFDFF ; w.bits(10,8) == uint16(5)
func (w word) bits(hi int, lo int) uint16 {
	return uint16(w>>lo) & uint16(1<<(hi-lo+1)-1)
}

// Decode a sign extended 10 or 7 bit immediate value from the current
// instruction. If the instruction doesn't have an immediate value, then
// the rest of the decode shouldn't try to use it so the return value is
// not important. In this case return the most harmless value, 0.
func (y4 *y4machine) sxtImm() uint16 {
	var result uint16
	ir := y4.ir
	op := ir.bits(15,13)
	neg := ir.bits(12,12) != 0
	if op < 6 { // ldw, ldb, stw, stb, beq, adi all have 7-bit immediates
		result = ir.bits(12,6)
		if neg {
			result |= 0xFF80
		}
	} else if op == 6 { // lui has a 10-bit immediate, upper bits
		result = ir.bits(12, 3) << 6
	} else if op == 7 && !neg { // jlr - 7-bit immediate if positive
		result = ir.bits(12,6)
	}
	// else bits(15,12) == 0xF and the instruction has no immediate value
	return result
}

// Fetch next instruction into ir.
func (y4 *y4machine) fetch() {
	// Clear the state set during execution and used at writeback.
	// The state set during decode is taken care of there.
	y4.alu = 0
	y4.sd = 0
	y4.wb = 0
	y4.hasStandardWriteback = false
	y4.hasSpecialWriteback = false
	// This one is more complicated. May remove this when IFTEs are
	// fully implemented.
	y4.ex = 0

	mem := &y4.mem[y4.mode]
	y4.ir = mem.imem[y4.pc]

	// Control flow instructions will overwrite this at the writeback stage.
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
// The implementation is in exec.go
func (y4 *y4machine) execute() {
	y4.executeSequential()
}

// For instructions that reference memory, special register space,
// or I/O space, do the operation. The computed address is in the alu
// (alu result) register and the execute phase must also have loaded
// the store data register.
//
// Note that this modifies memory and it's not in the writeback phase.
// Fine for this sequential implementation, but would seem to be an
// error for pipelining. But I think it isn't: if it succeeds, then
// the instruction will, because no exceptions are thrown at writeback
// time. If it fails, the memory write fails, and the store instruction
// didn't do anything else because this is RISC. So we can just handle
// the exception at writeback time like every other exception.
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
		reg := &y4.reg[y4.mode]
		switch y4.yop {
		case 0: // lsp (load special)
			y4.wb = reg.spr[y4.alu&(SprSize-1)]
		case 1: // ssp (store special)
			reg.spr[y4.alu&(SprSize-1)] = y4.sd
		case 4: // ior 
			y4.wb = y4.io[y4.alu&(IOSize-1)]
		case 5: // iow
			y4.io[y4.alu&(IOSize-1)] = y4.sd
		// no default
		}
	}
}

// Write the result (including possible memory result) to a register.
// Stores and io writes are handled at memory time.
func (y4 *y4machine) writeback() {
	if y4.ex != 0 { // exception pending - don't update registers
		return
	}

	// This code will be replaced by hasStandardWriteback and
	// hasSpecialWriteback after the execution code is complete. 
	// It's retained for now and will also be used for testing.
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

// I don't know exactly what I'm going to do for output from the
// simulator. For now, I threw together this function, which dumps
// the machine state and some memory contents to the screen.
func (y4 *y4machine) dump() {
	modeName := "user"
	if y4.mode == 1 {
		modeName = "kern"
	}
	fmt.Printf("Run %t mode %s cycle %d alu = 0x%04X pc = %d exception = 0x%04X\n",
		y4.run, modeName, y4.cyc, y4.alu, y4.pc, y4.ex)

	reg := &y4.reg[y4.mode] // user or kernel
	headerFormat := "%12s: "
	fmt.Printf(headerFormat, "reg")
	for i := range reg.gen {
		fmt.Printf("%04X%s", reg.gen[i], spOrNL(i < len(y4.reg)-1))
	}

	// For now, just print both the first 8 user and kernel sprs
	fmt.Printf(headerFormat, "user spr")
	for i := 0; i < 8; i++ {
		fmt.Printf("%04X%s", y4.reg[0].spr[i], spOrNL(i < 8))
	}
	fmt.Printf(headerFormat, "kern spr")
	for i := 0; i < 8; i++ {
		fmt.Printf("%04X%s", y4.reg[1].spr[i], spOrNL(i < 8))
	}

	mem := &y4.mem[y4.mode] // user or kernel
	off := int(y4.pc & 0xFFF8)
	fmt.Printf(headerFormat, fmt.Sprintf("imem@0x%04X", off))
	for i := 0; i < 8; i++ {
		fmt.Printf("%04X%s", mem.imem[off+i], spOrNL(i < len(y4.reg)-1))
	}
	
	// The memory address, if there is one, always comes from the ALU
	// Print the memory at the ALU address even though it might not have
	// anything to do with current execution.
	off = int(y4.alu & 0xFFF8)
	fmt.Printf(headerFormat, fmt.Sprintf("dmem@0x%04X", off))
	for i := 0; i < 8; i++ {
		fmt.Printf("%04X%s", mem.dmem[off+i], spOrNL(i < len(y4.reg)-1))
	}
}

func spOrNL(sp bool) string {
	if sp {
		return " "
	}
	return "\n"
}

func usage() {
	pr("Usage: func [options] y4-binary\nOptions:")
	flag.PrintDefaults()
	os.Exit(1)
}

