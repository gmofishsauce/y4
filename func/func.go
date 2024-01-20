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
const iosize = 32 // 32 words of i/o space
const lr = 0 // link register is spr[0]
const user = 0
const kern = 1

type word uint16

// Exception types. There can be only a few.

const ExIllegal word = 1 // illegal instruction
const ExMachine word = 2 // machine check

type y4mem struct {
	imem []word
	dmem []byte
}

type y4machine struct {
	cyc uint64   // cycle counter
	mem []y4mem  // [0] is user space, [1] is kernel
	reg []word   // registers (r0 must be zero)
	spr []word   // special registers
	io  []word	 // i/o space
	pc word

	// Non-architectural persisted state
	run bool     // run/stop flag (fake?)
	mode byte    // current mode, user = 0, kernel = 1

	// Non-architectual semi-persistent state
	// This is the longer term "fused instructions" bit
	hc uint16    // hidden carry bit, 1 or 0

	// Non-architectural state set anywhere
	ex word		 // 0 = no exception, nonzero values TBD

	// Non-architectural per-cycle state set at decode
	ir word      // instruction register
	op, imm uint16
	xop, yop, zop, vop uint16
	isx, isy, isz, isv bool
	ra, rb, rc uint16

	// Non-architural state set at execute or memory.
	//
	// The alu result is computed at execution time. If there
	// is a load or store, it is the address in all cases. If
	// it's a 16-bit write, the LS bits go at the byte addressed
	// by the alu value and the MS bits at byte (alu+1) in memory.
	// This is called "little endian".
	//
	// If there's a store, the source data is set at execution
	// time and stored in sd. For a load, the data is placed in
	// the writeback register (wb) at memory time.
	//
	// The instruction result if any if computed at execute time
	// or, if there's a load, at memory time, and placed in the wb
	// register. The wb register is written to either a general or
	// special register at writeback time as required by the opcode.
	alu uint16  // temporary alu result register
	sd word    // memory source data register
	wb word    // register writeback (instruction result)

	// Assists for the code that may not correspond directly
	// to anything you'd do in hardware (I think). These are
	// set at execute or memory time and used at writeback.
	hasStandardWriteback bool // wb => reg[rA] at writeback
	hasSpecialWriteback bool  // wb => spr[rB] at writeback
	isbase bool				  // base op (not xopy, yop, etc.)
}

var y4 y4machine = y4machine {
	mem: []y4mem{
		{imem: make([]word, 64*K, 64*K), dmem: make([]byte, 64*K, 64*K)}, // user
		{imem: make([]word, 64*K, 64*K), dmem: make([]byte, 64*K, 64*K)}, // kernel
	},
	reg: make([]word, 8, 8),
	spr: make([]word, 8, 8),
	io: make([]word, iosize, iosize),
	pc: 0,
	cyc: 0,
	run: false,
	mode: 0,
	hc: 0,
}

func (y4 *y4machine) reset() {
	y4.cyc = 0
	y4.pc = 0
	y4.mode = kern
	y4.run = true
}

func main() {
	var err error
	var n int

	flag.Parse()
	args := flag.Args()
	if len(args) != 1 {
		usage()
	}
	binPath := args[0]

	if n, err = y4.load(binPath); err != nil {
		fatal(fmt.Sprintf("loading %s: %s", binPath, err.Error()))
	}
	pr(fmt.Sprintf("loaded %d bytes", n))

	// reset here in main so that run() can act as "continue" (TBD)`
	fmt.Println("start")
	y4.reset()
	if err = y4.simulate(); err != nil {
		fatal(fmt.Sprintf("running %s: %s", binPath, err.Error()))
	}
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

	fmt.Printf("stopped\n")
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

// Fetch next instruction into ir. Future: MMU page faults.
func (y4 *y4machine) fetch() {
	y4.ex = 0	// I don't know if we should clear these ... it hides bugs.
	y4.alu = 0
	y4.sd = 0
	y4.wb = 0

	mem := &y4.mem[y4.mode]
	y4.ir = mem.imem[y4.pc]

	// Control flow instructions will overwrite this at the writeback stage.
	// This implementation is completely sequential so it's fine.
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

	y4.isv = y4.ir.bits(15,3) == 0x1FFF
	y4.isz = !y4.isv && y4.ir.bits(15,6) == 0x03FF
	y4.isy = !y4.isv && !y4.isz && y4.ir.bits(15,9) == 0x007F
	y4.isx = !y4.isv && !y4.isz && !y4.isy && y4.ir.bits(15,12) == 0x000F
	y4.isbase = !y4.isv && !y4.isz && !y4.isy && !y4.isv

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

// For instructions that reference memory, do the memory operation.
// The computed address is in the alu (alu result) register and the
// execute phase must also have loaded the store data register.
//
// Note that this modifies memory and it's not in the writeback phase.
// Fine for this sequential implementation, but would seem to be an
// error for pipelining. But I think it isn't: if it succeeds, then
// the instruction will, because no exceptions are thrown at writeback
// time. If it fails, the memory write fails, and the store instruction
// didn't do anything else because this is RISC. So we can just handle
// the exception at writeback time like every other exception.
func (y4 *y4machine) memory() {
	if y4.ex != 0 { // exception pending
		return
	}

	mem := &y4.mem[y4.mode] // user or kernel
	if y4.op < 4 { // general register load or store
		switch y4.op { // no default
		case 0:  // ldw
			y4.wb = word(mem.dmem[y4.alu])
			y4.wb |= word(mem.dmem[y4.alu+1]) << 8
		case 1:  // ldb
			y4.wb = word(mem.dmem[y4.alu]) & 0x00FF
		case 2:  // stw
			mem.dmem[y4.alu] = byte(y4.sd & 0x00FF)
			mem.dmem[y4.alu+1] = byte(y4.sd >> 8)
		case 3:  // stb
			mem.dmem[y4.alu] = byte(y4.sd & 0x00FF)
		}
	} else if y4.isy { // special load or store, io
		// FIXME the yop's will be renumbered XXX
		switch y4.yop { // no default
		case 2: // lds (load special)
			y4.wb = word(mem.dmem[y4.alu])
			y4.wb |= word(mem.dmem[y4.alu+1]) << 8
		case 3: // sts (store special)
			mem.dmem[y4.alu] = byte(y4.sd & 0x00FF)
			mem.dmem[y4.alu+1] = byte(y4.sd >> 8)
		case 5: // ior 
			y4.wb = y4.io[y4.alu&(iosize-1)]
		case 6: // iow
			y4.io[y4.alu] = y4.sd
		}
	} else {
		// the remaining instructions may or may not
		// have a result. But if they do, it comes 
		// from the alu. So put the alu output in the
		// writeback register; it will be used, or not.
		dbg("set wb to 0x%04X", y4.alu)
		y4.wb = word(y4.alu)
	}
}

// Write the result (including possible memory result) to a register.
// Stores and io writes are handled at memory time.
func (y4 *y4machine) writeback() {
	if y4.ex != 0 { // exception pending
		return
	}

	// This code will be replaced by hasStandardWriteback and
	// hasSpecialWriteback after the execution code is complete. 
	// It's retained for now and will also be used for testing.
	if y4.op == 0 ||   // ldw
		y4.op == 1 ||  // ldb
		y4.op == 5 ||  // adi
		y4.op == 6 ||  // lui
		y4.isx ||      // 3-operand alu
		(y4.isy && y4.yop == 5) ||  // ior
		y4.isz {       // single operand alu

		if y4.ra != 0 {
			dbg("set reg %d to 0x%04X", y4.ra, y4.wb)
			y4.reg[y4.ra] = y4.wb
		}
	} else if y4.isy && (y4.yop == 1 || y4.yop == 2) {
		// lds, rds
		y4.spr[y4.rb] = y4.wb
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

	headerFormat := "%12s: "
	fmt.Printf(headerFormat, "reg")
	for i := range y4.reg {
		fmt.Printf("%04X%s", y4.reg[i], spOrNL(i < len(y4.reg)-1))
	}

	fmt.Printf(headerFormat, "spr")
	for i := range y4.spr {
		fmt.Printf("%04X%s", y4.spr[i], spOrNL(i < len(y4.reg)-1))
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

