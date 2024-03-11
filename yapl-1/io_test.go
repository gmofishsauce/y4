/* Copyright © 2024 Jeff Berkowitz (pdxjjb@gmail.com) - Affero GPL v3 */

package main

import (
    "os"
	"testing"
)

func check(t *testing.T, header string, e error) {
    if e != nil {
		t.Fatalf("%s: error: %s\n", header, e.Error())
    }
}

func TestIO1(t *testing.T) {
	var testdata string = "This\nis a string\n    with multiple lines.\n"

	// Create a temporary file
    f, err := os.CreateTemp("", "TestIO1")
    check(t, "creating temp file", err)
	name := f.Name()
	defer os.Remove(name)
	fd := word(f.Fd())

	// Write the test data to the temp file using putb
	for _, c := range testdata {
		if putb(fd, byte(c)) != 1 {
			t.Fatalf("putb failed")
		}
	}
	f.Close()

	// Reopen the temp file
	f, err = os.Open(name)
	check(t, "reopen temp file", err)

	// Read the data using getb
	var text []byte
	var w word
	for w = getb(fd); w < 0x80; w = getb(fd) {
		text = append(text, byte(w))
	}
	if w != E_EOF {
		text = append(text, byte('?'))
		text = append(text, byte('?'))
		text = append(text, byte('?'))
		text = append(text, byte('\n'))
	}

	// Compare
	result := string(text)
	if result != testdata {
		t.Fatalf("compare failed")
	}
}
