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
	"bytes"
	"encoding/binary"
	//"errors"
	"fmt"
	"flag"
	"io"
	"os"
)

var dflag = flag.Bool("d", false, "enable debugging")

// Functional simulator for y4 instruction set

const K = 1024
type word uint16

type y4mem struct {
	imem []word
	dmem []byte
}

type y4machine struct {
	user y4mem // user space
	kern y4mem // kernel space
	reg []word // registers (r0 must be zero)
	spr []word // special registers
	hc byte // experimental hidden carry bit
}

var y4 y4machine = y4machine {
	user: y4mem{imem: make([]word, 64*K, 64*K), dmem: make([]byte, 64*K, 64*K)},
	kern: y4mem{imem: make([]word, 64*K, 64*K), dmem: make([]byte, 64*K, 64*K)},
	reg: make([]word, 8, 8),
	spr: make([]word, 8, 8),
	hc: 0,
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
	pr(fmt.Sprintf("%s: exit 0", binPath))
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

	if n, err = readChunk(f, buf, 0, nil, y4.user.imem[0:64*K/2]); err != nil {
		return 0, err
	}
	nLoaded += n
	if n < len(buf) {
		return nLoaded, nil
	}

	if n, err = readChunk(f, buf, 0, nil, y4.user.imem[64*K/2:64*K]); err != nil {
		return 0, err
	}
	nLoaded += n
	if n < len(buf) {
		return nLoaded, nil
	}

	if n, err = readChunk(f, buf, 0, y4.user.dmem[0:64*K], nil); err != nil {
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

func (y4 *y4machine) run() error {
	TODO()
	return nil
}

func usage() {
	pr("Usage: func [options] y4-binary\nOptions:")
	flag.PrintDefaults()
	os.Exit(1)
}

