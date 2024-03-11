/* Copyright © 2024 Jeff Berkowitz (pdxjjb@gmail.com) - Affero GPL v3 */

package main

import (
	"syscall"
	"fmt" // only for debugging
	"os"  // only for debugging
)

func dbg(s string, args ...any) {
	fmt.Fprintf(os.Stderr, s, args...)
}

// There should be no further direct use of fmt or os.

// Input/output compatible with the tiny kernel on the WUT-4. The
// WUT-4 has no native idea of signedness or signs, so the I/O
// functions are coded somewhat clumsily using unsigned values.

const E_EOF word = 0xFFFF
const E_IOERR word = 0xFFFE
const E_UNKNOWN word = 0xFFFD

// Get a byte from "channel" fd. If the MS byte of the return value
// is nonzero, an error has occurred. Otherwise, the input byte is
// in the LS byte of the return value.
func getb(fd word) word {
	b := []byte{0x00}
	n, err := syscall.Read(int(fd), b)
	if err != nil {
		return E_IOERR
	}
	if n == 0 {
		return E_EOF
	}
	return word(b[0]) // success
}

// Put a byte on "channel" fd. If the MS byte of the return value
// is nonzero, an error has occurred. Otherwise, the return value
// is 1 indicating that the byte was written.
func putb(fd word, val byte) word {
	b := []byte{val}	
	n, err := syscall.Write(int(fd), b)
	if err != nil {
		return E_IOERR
	}
	if n != 1 {
		return E_EOF
	}
	return 1
}
