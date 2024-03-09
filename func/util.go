package main

/*
Author: Jeff Berkowitz
Copyright (C) 2024 Jeff Berkowitz

This file is part of func.

Func is free software; you can redistribute it and/or
modify it under the terms of the GNU General Public License
as published by the Free Software Foundation, either version 3
of the License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program. If not, see http://www.gnu.org/licenses/.
*/

import (
	"encoding/binary"
	"fmt" // fmt.Errorf only
	"io"
	"os"
)

// Get the bits from hi:lo inclusive as a small uint16
// Example: w := 0xFDFF ; w.bits(10,8) == uint16(5)
func (w word) bits(hi int, lo int) uint16 {
	return uint16(w>>lo) & uint16(1<<(hi-lo+1)-1)
}

// Virtual to physical address translation. There are two MMUs, one for
// kernel and one for user mode. Each MMU is at offset 32 in the respective
// arrays of 64 SPRs. The first 16 entries map 64k words (128k bytes) of
// code space. The second 16 SPRs map 64k bytes of data space. Physical
// addresses are 24 bits long, allowing 16Mib of physical data memory.
//
// The lower 12 bits of virtual address become part of the physical address.
// The upper 4 bits of virtual address are used to select one of the 16 MMU
// registers for that (mode, kind) pair. The lower 12 bits of the selected
// MMU register become the upper 12 bits of the 24-bit physical address.
//
// Since physical memory is implemented as an array of shortwords, data
// addresses are shifted right one to make up for the automatic address
// scaling that results from indexing the uint16 array. Byte accesses within
// this word must be handled by the caller.
//
// It's cheesy using a bool for the 2-element enum {code, data}. But adding
// to that enum would require a major change to the WUT-4 architecture, i.e.
// this would be the least of my worries.
func (y4 *y4machine) translate(isData bool, virtAddr word) (exception, physaddr) {
	sprOffset := 32
	if isData {
		sprOffset += 16
	}
	sprOffset += int(virtAddr >> 12)

	mmu := y4.reg[y4.mode].spr
	upper := physaddr(mmu[sprOffset] & 0xFFF)
	lower := physaddr(virtAddr & 0xFFF)
	result := (upper << 12) | lower
	if isData {
		result >>= 1
	}
	// Prevent the emulator from crashing if the emulated code accesses
	// past the end of physmem. TODO: memory protection architecture.
	if result > PhysMemSize {
		return ExMemory, result
	}
	return ExNone, result
}

// Reset the simulated hardware
func (y4 *y4machine) reset() {
	y4.cyc = 0
	y4.pc = 0
	y4.run = true
	y4.mode = Kern
	y4.ex = 0
	y4.en = false
	// After initialiation, we'll want to enter user mode at user address
	// 0 by executing an RTI instruction. This will restore the mode from
	// the kernel mode SPR "Imr". We need this register to contain "user
	// mode" when that RTI happens. I don't know what I'd do about this in
	// real hardware if I do that. Should the IMR be writable?
	y4.reg[Kern].spr[Imr] = User
}

// Decode a sign extended 10 or 7 bit immediate value from the current
// instruction. If the instruction doesn't have an immediate value, then
// the rest of the decode shouldn't try to use it so the return value is
// not important. In this case return the most harmless value, 0.
func (y4 *y4machine) sxtImm() uint16 {
	var result uint16
	ir := y4.ir
	op := ir.bits(15, 13)
	neg := ir.bits(12, 12) != 0
	if op < 6 { // ldw, ldb, stw, stb, beq, adi all have 7-bit immediates
		result = ir.bits(12, 6)
		if neg {
			result |= 0xFF80
		}
	} else if op == 6 { // lui has a 10-bit immediate, upper bits
		result = ir.bits(12, 3) << 6
	} else if op == 7 && !neg { // jlr - 7-bit immediate if positive
		result = ir.bits(12, 6)
	}
	// else bits(15,12) == 0xF and the instruction has no immediate value
	return result
}

// Load a binary into memory. This consumes binaries written directly
// by customasm. Each binary has exactly 1 code segment of up to 64k
// words (128k bytes) optionally followed by 1 64k byte data segement.
// If the data segment is present, the code segment is filled with
// zeroes to 128k. If the mode is 0 (kernel), the file is loaded at
// physical 0. If it is 1 (user), it's loaded at physical 3*64k byte.
func (y4 *y4machine) load(mode int, binPath string) error {
	f, err := os.Open(binPath)
	if err != nil {
		return err
	}
	defer f.Close()

	maxSizeBytes := 3 * 64 * K
	fi, err := f.Stat()
	if err != nil {
		return err
	}
	size := int(fi.Size())
	if size > maxSizeBytes {
		return fmt.Errorf("not a binary: %s", binPath)
	}

	off := 0
	if mode == User {
		off += maxSizeBytes / 2
	}

	var b []byte = []byte{0}
	var nRead int

	for {
		n, err := f.Read(b)
		if err != nil && err != io.EOF {
			break
		}
		if n == 0 {
			break
		}
		if nRead&1 == 0 {
			physmem[off] = word(b[0])
		} else {
			physmem[off] |= word(b[0]) << 8
			off++
		}
		nRead++
	}

	if err == io.EOF {
		err = nil
	}
	if err != nil {
		return err
	}
	if nRead != size {
		return fmt.Errorf("load didn't read the entire file")
	}
	return nil
}

func (y4 *y4machine) core(corePath string) error {
	f, err := os.Create(corePath)
	if err != nil {
		return err
	}
	defer f.Close()

	return binary.Write(f, binary.LittleEndian, physmem)
}
