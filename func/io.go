package main

/*
Author: Jeff Berkowitz
Copyright (C) 2023 Jeff Berkowitz

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
	"fmt"
	"io"
	"os"
	"runtime"
	"runtime/debug"
)

func assert(b bool, msg string) {
	if !b {
		panic("assertion failure: " + msg)
	}
}

func fatal(s string) {
	pr(s)
	os.Exit(2)
}

func pr(s string) {
	fmt.Fprintln(os.Stderr, "func: " + s)
}

func dbg(s string, args... any) {
	// dbgN(1, ...) is this function
	dbgN(2, s, args...)
}

func dbgN(n int, s string, args... any) {
    pc, _, _, ok := runtime.Caller(n)
    details := runtime.FuncForPC(pc)
	where := "???"
    if ok && details != nil {
		where = details.Name()
    }
	s = "[at " + where + "]: " + s + "\n"
	fmt.Fprintf(os.Stderr, s, args...)
}

func dbgST() {
	debug.PrintStack()
}

var todoDone = make(map[string]bool)

// This function prints the callers name and TODO once per
// execution of the calling program. Arguments are ignored
// and are provided to make reference to unreference variables
// in a partially completely implementation.
func TODO(args... any) error {
	pc, _, _, ok := runtime.Caller(1)
    details := runtime.FuncForPC(pc)
    if ok && details != nil && !todoDone[details.Name()] {
        dbg("TODO called from %s", details.Name())
		todoDone[details.Name()] = true
    }
	return nil
}

// For now, we accept the output of customasm directly. The bin file has
// no file header. There are between 1 and 4 sections in this order: user
// code, user data (if any), kernel code (if any) and kernel data (if any).
// Each section is full length if it's followed by other sections, else
// it's truncated to the length required to hold whatever is there. The
// full section lengths are 128kb for the code sections and 64kb for the
// data sections.
//
// So if the file length is ... then we need to load ...
//
//                       <= 128k                user code only
//                       <= 192k                user code and data
//                       <= 320k                plus kernel code
//                       <= 384k                plus kernel data
//
// If it's bigger than that, we report an error because we're confused.
// Note that there might not be any user code or data, for example, but
// by convention the file still has full-length sections.
//
// This is all temporary and will be replaced by something more reasonable.
func (y4 *y4machine) load(binPath string) (int, error) {
	f, err := os.Open(binPath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	// I looked at using encoding.binary.Read() directly on the binfile
	// but because it doesn't return a byte count, checking for the
	// partial read at the end of the file based on error types is messy. 

	var buf []byte = make([]byte, 64*K, 64*K)
	var nLoaded int
	var n int

	if n, err = readChunk(f, buf, 0, nil, y4.mem[0].imem[0:64*K/2]); err != nil {
		return 0, err
	}
	nLoaded += n
	if n < len(buf) {
		return nLoaded, nil
	}

	if n, err = readChunk(f, buf, 64*K, nil, y4.mem[0].imem[64*K/2:64*K]); err != nil {
		return 0, err
	}
	nLoaded += n
	if n < len(buf) {
		return nLoaded, nil
	}

	if n, err = readChunk(f, buf, 128*K, y4.mem[0].dmem[0:64*K], nil); err != nil {
		return 0, err
	}
	nLoaded += n
	if n < len(buf) {
		return nLoaded, nil
	}

	if n, err = readChunk(f, buf, 192*K, nil, y4.mem[1].imem[0:64*K/2]); err != nil {
		return 0, err
	}
	nLoaded += n
	if n < len(buf) {
		return nLoaded, nil
	}

	if n, err = readChunk(f, buf, 256*K, nil, y4.mem[1].imem[64*K/2:64*K]); err != nil {
		return 0, err
	}
	nLoaded += n
	if n < len(buf) {
		return nLoaded, nil
	}

	if n, err = readChunk(f, buf, 320*K, y4.mem[1].dmem[0:64*K], nil); err != nil {
		return 0, err
	}
	nLoaded += n
	return nLoaded, nil
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

