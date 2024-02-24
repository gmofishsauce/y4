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

// Virtual to physical address translation. There are two MMUs,
// one for kernel and one for user mode. Each MMU is at offset
// 32 in the respective arrays of 64 SPRs. The first 16 entries
// map 64k words of code space. The second 16 SPRs map 64k bytes
// of data space. Physical addresses are 24 bits long, allowing
// 16Mib of physical data memory and 16MiWords of code memory.
// The lower 12 bits of virtual address become part of the physical
// address. The upper 4 bits of virtual address are used to select
// one of the 16 MMU registers for that (mode, kind) pair. The
// lower 12 bits of the selected MMU register become the upper 12
// bits of the 24-bit physical address.
//
// It's cheesy using a bool for the 2-element enum {code, data}.
// But adding to that enum would require a major change to the
// WUT-4 architecture, i.e. this would be the least of my worries.
func (y4 *y4machine) translate(isData bool, virtAddr word) physaddr {
	sprOffset := 32
	if isData {
		sprOffset += 16
	}
	sprOffset += int(virtAddr>>12)

	mmu := y4.reg[y4.mode].spr
	upper := physaddr(mmu[sprOffset]&0xFFF)
	lower := physaddr(virtAddr&0xFFF)
	return (upper<<12)|lower
}

// Reset the simulated hardware
func (y4 *y4machine) reset() {
	y4.cyc = 0
	y4.pc = 0
	y4.run = true
	y4.mode = Kern
	y4.ex = 0
	y4.en = false
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

func (y4 *y4machine) load(mode int, binPath string) error {
    f, err := os.Open(binPath)
    if err != nil {
        return err
    }
    defer f.Close()

	maxSizeBytes := 3*64*K
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
			physmem[off] |= word(b[0])<<8
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

/* MEM
// For now, we accept the output of customasm directly. The bin file has
// no file header. There are 1 or 2 sections in the file. Code is at file
// offset 0 for a maximum length of 64k 2-byte words. Initialized Data,
// if present, is at offset 128 kiB for a maximum length of 64kibB. Since
// the machine initializes in kernel mode, kernel code is mandatory; this
// is handled in main(). If a user mode binary is present for this simulation
// run, it results in a second call to this function.
func (y4 *y4machine) load(mode int, binPath string) error {
	f, err := os.Open(binPath)
	if err != nil {
		return err
	}
	defer f.Close()

	// I looked at using encoding.binary.Read() directly on the binfile
	// but because it doesn't return a byte count, checking for the
	// partial read at the end of the file based on error types is messy. 

	var buf []byte = make([]byte, 64*K, 64*K)
	var n int

	if n, err = readChunk(f, buf, 0, nil, y4.mem[mode].imem[0:64*K/2]); err != nil {
		return err
	}
	if n < len(buf) {
		return nil
	}

	if n, err = readChunk(f, buf, 64*K, nil, y4.mem[mode].imem[64*K/2:64*K]); err != nil {
		return err
	}
	if n < len(buf) {
		return nil
	}

	if n, err = readChunk(f, buf, 128*K, y4.mem[mode].dmem[0:64*K], nil); err != nil {
		return err
	}
	return nil
}

// Read a chunk of f at offset pos and decode it into either b or w. One of
// b or w must be nil on each call and the other is the decoding target (so
// just a nil check is required instead of RTTI).
//
// This function either returns a positive count or an error, but not both.
// If the count is short, the read hit EOF and was successfully decoded into
// the buffer. No error is returned.
func readChunk(f *os.File, buf []byte, pos int64, b []byte, w []word) (int, error) {
    n, err := f.ReadAt(buf, pos)
	// "ReadAt always returns a non-nil error when n < len(b)." (Docs)
    if n == 0 || (err != nil && err != io.EOF) {
        return 0, err
    }

	// Now if n < len(buf), err is io.EOF but we don't care.
    r := bytes.NewReader(buf)
	if b != nil {
		err = binary.Read(r, binary.LittleEndian, b[0:n])
	} else {
		err = binary.Read(r, binary.LittleEndian, w[0:n/2])
	}
	if err != nil {
		// The decoder shouldn't fail. Say we got no data.
		return 0, err
	}
	return n, nil
}

MEM */
