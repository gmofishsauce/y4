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
	pc uint16
	mode byte   // current mode, user = 0, kernel = 1
	hc byte     // experimental hidden carry bit
	running bool    // run/stop flag
}

var y4 y4machine = y4machine {
	mem: []y4mem{
		{imem: make([]word, 64*K, 64*K), dmem: make([]byte, 64*K, 64*K)}, // user
		{imem: make([]word, 64*K, 64*K), dmem: make([]byte, 64*K, 64*K)}, // kernel
	},
	reg: make([]word, 8, 8),
	spr: make([]word, 8, 8),
	pc: 0,
	mode: 0,
	hc: 0,
	running: false,
}

func (y4 *y4machine) reset() {
	y4.pc = 0
	y4.mode = user
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

type runfunc func(inst word) bool

func (y4 *y4machine) run() error {
	y4.reset()
	mem := &y4.mem[y4.mode]
	var instCount int
	
	for instCount = 0; y4.running && instCount < limit; instCount++ {
		inst := mem.imem[y4.pc]
		dbg(fmt.Sprintf("pc = 0x%04X inst = 0x%04X", y4.pc, inst))
		y4.pc++
		runFunc := decode(inst)
		y4.running = runFunc(inst)
	}

	pr(fmt.Sprintf("Stopped after %d instruction pc = 0x%04X", instCount, y4.pc))
	return nil
}

func (w word) bits(hi int, lo int) uint16 {
	mask :=  uint16(1<<(hi-lo+1)-1)
	//dbg(fmt.Sprintf("bits w = 0x%04X hi %d lo %d mask 0x%04X", w, hi, lo, mask))
	return uint16(w>>lo) & mask
}

func decode(inst word) runfunc {
	TODO()
	op := inst.bits(15, 13)
	//dbg(fmt.Sprintf("  op %d", op))
	if op < 4 {
		return ldst
	}
	return func(inst word) bool {
		dbg("runfunc default")
		return true
	}
}

// runfuncs

func ldst(inst word) bool {
	TODO()
	dbg("runfunc ldst")
	return true
}

func usage() {
	pr("Usage: func [options] y4-binary\nOptions:")
	flag.PrintDefaults()
	os.Exit(1)
}

