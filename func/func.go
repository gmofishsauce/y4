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

// The WUT-4 boots in kernel mode, so the kernel binary is mandatory.
// A user mode binary is optional.
var dflag = flag.Bool("d", false, "enable debugging")
var uflag = flag.String("u", "", "user binary")

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
// exceptional condition is detected. SYS 0 jumps to the system
// reset code; if called from user mode, the kernel can (and
// does) choose to treat this as an illegal instruction.

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
	en bool     // true if interrupts are enabled
	mode byte   // current mode, user = 0, kernel = 1

	// Non-architectural state used within an instruction
	alu uint16  // temporary alu result register; memory address
	sd word     // memory source data register set at execute
	wb word     // writeback register set at execute or memory
	ex word		// exception code
	ir word     // instruction register
	hc uint16   // hidden carry bit, 1 or 0

	// These variables are part of the combinational logic.
	// The are set at decode time and used at execute, memory,
	// or writeback time.
	op, imm uint16
	xop, yop, zop, vop uint16
	isXop, isYop, isZop, isVop, isBase bool
	ra, rb, rc uint16
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

func main() {
	var err error

	flag.Parse()
	args := flag.Args()
    if len(args) != 1 { // kernel mode binary file is mandatory
        usage()
    }

    dbEnabled = *dflag
	if err := y4.load(Kern, args[0]); err != nil {
		fatal(fmt.Sprintf("loading %s: %s", args[0], err.Error()))
	}
	if len(*uflag) != 0 {
		if err := y4.load(User, *uflag); err != nil {
			fatal(fmt.Sprintf("loading %s: %s", *uflag, err.Error()))
		}
	}

	dbg("start")
	y4.reset()
	err = y4.simulate()
	if err != nil {
		// This represents some kind of internal error, not error in program
		fatal(fmt.Sprintf("error: running %s: %s", args[0], err.Error()))
	}
	dbg("done")
}

// Run the simulator. There must already be something loaded in imem.

func (y4 *y4machine) simulate() error {
	if y4.ex != 0 {
		fatal("internal error: simulation started with an exception pending")
	}

	// The simulator is written as a rigid set of parameterless functions
	// that act on shared machine state. This will make it simpler to
	// simulate pipelining later.
	//
	// Sequential implementation: everything happens in each machine cycle.
	// It happens in the order of a pipelined machine, though, to make
	// converting this to a pipelined simulation easier in the future.

	for y4.cyc++ ; y4.run ; y4.cyc++ {
		y4.fetch()
		y4.decode()
		y4.execute()
		y4.memory()
		y4.writeback()
		if y4.ex != 0 && !y4.en {
			fmt.Printf("double fault: exception %d\n", y4.ex)
			break
		}
		if *dflag {
			y4.dump()
			var toss []byte = []byte{0}
			fmt.Printf("sim> ")
			os.Stdin.Read(toss)
		}
	}
	y4.dump()
	return nil
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
		fmt.Printf("%04X%s", reg.gen[i], spOrNL(i < len(reg.gen)-1))
	}

	// For now, just print both the first 8 user and kernel sprs
	fmt.Printf(headerFormat, "user spr")
	for i := 0; i < 8; i++ {
		fmt.Printf("%04X%s", y4.reg[0].spr[i], spOrNL(i < 7))
	}
	fmt.Printf(headerFormat, "kern spr")
	for i := 0; i < 8; i++ {
		fmt.Printf("%04X%s", y4.reg[1].spr[i], spOrNL(i < 7))
	}

	mem := &y4.mem[y4.mode] // user or kernel
	off := int(y4.pc & 0xFFF8)
	fmt.Printf(headerFormat, fmt.Sprintf("imem@0x%04X", off))
	for i := 0; i < 8; i++ {
		fmt.Printf("%04X%s", mem.imem[off+i], spOrNL(i < 7))
	}
	
	// For lack of a better answer, print the memory row at 0.
	// This at least gives 8 deterministic locations for putting
	// the results of tests
	off = 0 // was: int(y4.alu & 0xFFF8)
	fmt.Printf(headerFormat, fmt.Sprintf("dmem@0x%04X", off))
	for i := 0; i < 8; i++ {
		fmt.Printf("%04X%s", mem.dmem[off+i], spOrNL(i < 7))
	}
}

func spOrNL(sp bool) string {
	if sp {
		return " "
	}
	return "\n"
}

func usage() {
	pr("Usage: func [options] kernel-binary\nOptions:")
	flag.PrintDefaults()
	os.Exit(1)
}

