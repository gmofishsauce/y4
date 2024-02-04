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
	"bytes"
	"encoding/binary"
	"io"
	"os"
)

// Get the bits from hi:lo inclusive as a small uint16
// Example: w := 0xFDFF ; w.bits(10,8) == uint16(5)
func (w word) bits(hi int, lo int) uint16 {
	return uint16(w>>lo) & uint16(1<<(hi-lo+1)-1)
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

