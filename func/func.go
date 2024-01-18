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
const limit = 10 // instruction execution limit
const iosize = 32 // 32 words of i/o space
const lr = 0 // link register is spr[0]
const user = 0
const kern = 1

type word uint16

type y4mem struct {
	imem []word
	dmem []byte
}

type y4machine struct {
	mem []y4mem // [0] is user space, [1] is kernel
	reg []word  // registers (r0 must be zero)
	spr []word  // special registers
	io  []word	// i/o space
	pc word

	// Non-architectural persisted state
	running bool // run/stop flag (fake?)
	mode byte    // current mode, user = 0, kernel = 1

	// Non-architectual semi-persistent state
	// This is the longer term "fused instructions" bit
	hc byte      // experimental hidden carry bit

	// Non-architectural state set anywhere, even fetch
	ex word		 // contents TBD

	// Non-architectural per-cycle state set at decode
	ir word      // instruction register
	op, i7, i10 uint16
	xop, yop, zop, vop uint16
	isx, isy, isz, isv bool
	ra, rb, rc uint16

	// Non-architural state set at execute or memory
	alu word   // temporary alu result register
	mr word    // memory data source or dest register
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
	running: false,
	mode: 0,
	hc: 0,
}

func (y4 *y4machine) reset() {
	y4.pc = 0
	y4.mode = kern
	y4.running = true
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

	if err = y4.run(); err != nil {
		fatal(fmt.Sprintf("running %s: %s", binPath, err.Error()))
	}
}

// Run the simulator. There must already be something loaded in imem.

func (y4 *y4machine) run() error {
	var instCount int
	y4.reset()
	
	// The simulator is written as a rigid set of parameterless functions
	// that act on shared machine state. This will make it simpler to
	// simulate pipelining later.
	// XXX The instruction limit is just for getting this running.
	for instCount = 0; y4.running && instCount < limit; instCount++ {
		y4.fetch()
		y4.decode()
		y4.execute()
		y4.memory()
		y4.writeback()
	}

	pr(fmt.Sprintf("Stopped after %d instruction pc = 0x%04X", instCount, y4.pc))
	return nil
}

// Get the bits from hi:lo inclusive as a small uint16
// Example: w := 0xFDFF ; w.bits(10,8) == uint16(5)
func (w word) bits(hi int, lo int) uint16 {
	return uint16(w>>lo) & uint16(1<<(hi-lo+1)-1)
}

// Fetch next instruction into ir. Future: MMU page faults.
func (y4 *y4machine) fetch() {
	y4.ex = 0
	y4.mr = 0
	y4.alu = 0

	mem := &y4.mem[y4.mode]
	y4.ir = mem.imem[y4.pc]
	y4.pc++
}

// Pull out all the possible distinct field types into uint16s. The targets
// (op, i7, yop, etc.) are all non-architectural per-cycle and mostly mean
// e.g. multiplexer outputs in hardware. The remaining stages can act on the
// decoded values. Plausible additional decoding (which instructions have
// targets? Which target special registers?) is left to the execution code.
func (y4 *y4machine) decode() {
	y4.op = y4.ir.bits(15,13)	// base opcode
	y4.i7 = y4.ir.bits(12,6)	// 7-bit immediate, when present
	y4.i10 = y4.ir.bits(12,3)	// 10-bit immediate, when present

	y4.isx = y4.ir.bits(15,12) == 0x000F
	y4.xop = y4.ir.bits(11,9)
	y4.isy = y4.ir.bits(15,9)  == 0x007F
	y4.yop = y4.ir.bits(8,6)
	y4.isz = y4.ir.bits(15,6)  == 0x03FF
	y4.zop = y4.ir.bits(5,3)
	y4.isv = y4.ir.bits(15,3)  == 0x1FFF
	y4.vop = y4.ir.bits(2,0)

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
// The computed address is in the alu (alu result) register. For
// word loads and stores, the value has already been scaled; how
// this would work in hardare is TBD.
func (y4 *y4machine) memory() {
	if y4.ex != 0 { // exception pending
		return
	}
	mem := &y4.mem[y4.mode]
	if y4.op < 4 { // general register load or store
		switch y4.op { // no default
		case 0:  // ldw
			y4.mr = word(mem.dmem[y4.alu])
			y4.mr |= word(mem.dmem[y4.alu+1]) << 8
		case 1:  // ldb
			y4.mr = word(mem.dmem[y4.alu]) & 0x00FF
		case 2:  // stw
			mem.dmem[y4.alu] = byte(y4.mr & 0x00FF)
			mem.dmem[y4.alu+1] = byte(y4.mr >> 8)
		case 3:  // stb
			mem.dmem[y4.alu] = byte(y4.mr & 0x00FF)
		}
	} else if y4.isy { // special load or store, io
		switch y4.yop { // no default
		case 2: // lds (load special)
			y4.mr = word(mem.dmem[y4.alu])
			y4.mr |= word(mem.dmem[y4.alu+1]) << 8
		case 3: // sts (store special)
			mem.dmem[y4.alu] = byte(y4.mr & 0x00FF)
			mem.dmem[y4.alu+1] = byte(y4.mr >> 8)
		case 5: // ior 
			y4.mr = y4.io[y4.alu&(iosize-1)]
		case 6: // iow
			y4.io[y4.alu] = y4.mr
		}
	}
}

// Write the result (including possible memory result) to a register.
func (y4 *y4machine) writeback() {
	if y4.ex != 0 { // exception pending
		return
	}
	if y4.op < 4 { // general register load or store
		switch y4.op { // no default
		case 0, 1:  // ldw, ldb
			y4.reg[y4.ra] = y4.mr
		}
	} else if y4.isy { // special load or store, io
		switch y4.yop { // no default
		case 2: // lds (load special)
			// This is the only case where the target
			// is not in the register A (rA) field.
			y4.reg[y4.rb] = y4.mr
		case 5: // ior 
			y4.reg[y4.ra] = y4.mr
		}
	}
}

func usage() {
	pr("Usage: func [options] y4-binary\nOptions:")
	flag.PrintDefaults()
	os.Exit(1)
}

