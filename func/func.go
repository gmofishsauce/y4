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
	"os/signal"
	"runtime/pprof"
	"time"
)

// The WUT-4 boots in kernel mode, so the kernel binary is mandatory.
// A user mode binary is optional.
var dflag = flag.Bool("d", false, "enable debugging")
var hflag = flag.Bool("h", false, "home cursor (don't scroll)")
var pflag = flag.Bool("p", false, "write profile to cpu.prof")
var qflag = flag.Bool("q", false, "quiet (no simulator output)")
var uflag = flag.String("u", "", "user binary")

// Functional simulator for y4 instruction set

const K = 1024
const IOSize = 64	// 64 words of I/O space
const SprSize = 64	// 64 special registers, per Mode
const PC = 0		// Special register 0 is PC, read-only
const Link = 1		// Special register 1 is Link, per Mode
const Irr = 2       // Kernel only interrupt return register SPR
const Icr = 3		// Kernel only interrupt cause register SPR
const Imr = 4		// Kernel only interrupt mode register SPR
const CCLS = 6		// Cycle counter, lower short
const CCMS = 7		// Cycle counter, most significant short

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

	if *pflag {
		if *dflag {
			fatal("cannot profile and debug at the same time")
		}
        f, err := os.Create("cpu.prof")
        if err != nil {
            fatal(fmt.Sprintf("could not create CPU profile: ", err))
        }
        defer f.Close()
        if err := pprof.StartCPUProfile(f); err != nil {
            fatal(fmt.Sprintf("could not start CPU profile: ", err))
        }
        defer pprof.StopCPUProfile()
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

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for sig := range c {
			fmt.Printf("caught %v ('x' to exit)\n", sig)
			dbEnabled = true
		}
	}()

	dbg("start")
	y4.reset()
	err = y4.simulate()
	if err != nil {
		// This represents some kind of internal error, not error in program
		fatal(fmt.Sprintf("error: running %s: %s", args[0], err.Error()))
	}
	dbg("done")
}

// If we block for input at any time during execution, we don't report
// the machine cycles per second at the end because it's not meaningful.
var blockedForInput bool

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

	tStart := time.Now()
	for y4.cyc++ ; y4.run ; y4.cyc++ {
		y4.fetch()
		y4.decode()
		y4.execute()
		y4.memory()
		y4.writeback()
		if y4.ex != 0 && !y4.en {
			break
		}
		if dbEnabled {
			y4.dump()
			y4.run = prompt()
		}
	}
	d := time.Since(tStart)

	if *qflag {
		return nil
	}

	// Dump the registers. Print a line about why the simulator halted,
	// and then a line about timing unless sim was interactive.
	y4.dump()
	msg := "halt"
	if y4.ex != 0 && !y4.en {
		msg += fmt.Sprintf(": double fault: exception %d", y4.ex)
	}
	fmt.Println(msg)

	msg = fmt.Sprintf("%d cycles executed", y4.cyc)
	if !blockedForInput { // noninteractive run: print time
		msg += fmt.Sprintf(" in %s (%1.3fMHz)",
					d.Round(time.Millisecond).String(),
					(float64(y4.cyc)/1e6) / d.Seconds())
	}
	fmt.Println(msg)
	return nil
}

// Prompt the user for input and return false if the sim should halt
func prompt() bool {
	blockedForInput = true

	var c []byte = make([]byte, 80, 80)
	loop: for {
		fmt.Printf("\n[h c s x] sim> ")
		os.Stdin.Read(c)
		switch c[0] {
		case 'h':
			fmt.Printf("h - help\nc - continue\ns - single step\nx - exit\n")
		case 'c':
			dbEnabled = false
			break loop
		case 's':
			dbEnabled = true
			break loop
		case 'x':
			return false
		}
	}
	return true
}

// Dump some machine state. This method can be invoked from inside a wut-4
// program by executing the dsp instruction or the brk instruction. The brk
// instruction also makes the simulator go interactive (prompt to continue).
func (y4 *y4machine) dump() {
	if *hflag {
		// Home cursor and clear screen
		// This erases the debug output
		fmt.Printf("\033[2J\033[0;0H")
	}

	modeName := "user"
	if y4.mode == Kern {
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

