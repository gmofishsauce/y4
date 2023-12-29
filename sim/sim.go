/*
Copyright © 2022 Jeff Berkowitz (pdxjjb@gmail.com)

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
	"encoding/binary"
	"fmt"
	"os"
	"time"
)

var startTime time.Time = time.Now()
var binLog *os.File

const (
	SevError = byte('E')
	SevWarn = byte('W')
	SevInfo = byte('I')
	SevDebug = byte('D')
	KindEval = byte('V')
	KindEdge = byte('E')
)

func main() {
	var err error
	if binLog, err = os.Create("./log.bin"); err != nil {
		fatal(fmt.Sprintf("open log.bin: %s\n", err.Error()))
	}
	defer binLog.Close()

	s, err := Build()
	if err != nil {
		fatal(err.Error())
	}
	if err := Check(s); err != nil {
		fatal(err.Error())
	}
	if err = Simulate(s, true, 5); err != nil {
		fatal(err.Error())
	}
	pr("success")
	os.Exit(0)
}

// Make all the System components and wire them together.
// In time, command line flags will offer a choice of implementations.
func Build() (*System, error) {
	dbg("building...")
	s, err := MakeSystem()
	if err != nil {
		return nil, err
	}
	if err = Sequential(s); err != nil {
		return nil, err
	}
	return s, nil
}

// Components can't check themselves during Build() because they
// can't know if another AddInput() call might be coming, etc.
// This is called after build returns and calls Check() on all
// the components that registered themselves during Build().
func Check(s *System) error {
	dbg("checking...")
	var nError int = 0
	for _, cl := range s.state {
		dbg("clockable: %s", cl.Name())
		if err := cl.Check(); err != nil {
			nError++
			pr(err.Error())
		}
	}
	for _, co := range s.logic {
		dbg("combinational: %s", co.Name())
		if err := co.Check(); err != nil {
			nError++
			pr(err.Error())
		}
	}
	if nError > 0 {
		msg := "s" // Oh for a ternary ...
		if nError == 1 {
			msg = ""
		}
		return fmt.Errorf("%d error%s found in circuit", nError, msg)
	}
	return nil
}

// We have to be extremely careful not to introduce any ordering dependencies.
// On each cycle, we first Prepare() all the components which clears the next
// state variable for Clockables and also clears optional caching for logic.
// Then we Evaluate() all the Clockables, which prepares their nextStates and
// their clock enables, typically by calling to Evaluate() on logic components.
// Finally, we call PositiveEdge() on all the Clockables which transfers next
// state to exposed state. It's critical that all computation is performed in
// Evaluate() only, after all components are prepared and before any are clocked.
// Any computations done in PositiveEdge() may accidentally read exposed state
// that has already been updated to its value for the following machine cycle.

var cycleCounter int

func Simulate(s *System, reset bool, nCycles int) error {
	dbg("simulating...")
	if (reset) {
		for _, cl := range s.state {
			cl.Reset()
		}
		cycleCounter = 0
	}
	for end := cycleCounter + nCycles ; cycleCounter < end ; cycleCounter++ {
		for _, co := range s.logic {
			co.Prepare()
		}
		for _, cl := range s.state {
			cl.Evaluate()
		}
		for _, cl := range s.state {
			cl.PositiveEdge()
		}
	}
	return nil
}

func fatal(s string) {
	pr(s)
	os.Exit(2)
}

func pr(s string) {
	fmt.Fprintf(os.Stderr, "%s\n", s)
}

const recordSize = 32
const recordsPerBuffer = 128
const bufLen = recordSize * recordsPerBuffer
var buf1 []byte = make([]byte, bufLen, bufLen)
var buf2 []byte = make([]byte, bufLen, bufLen)
var bufferPair [2][]byte = [2][]byte{buf1, buf2}
var bufferPairIndex = 0

var bufOffset int // 0..4k by bufLen, then back to 0
const srcLen = 12
const evtLen = 10
var zeroBytes []byte = make([]byte, recordSize, recordSize)

// Written to a packed binary buffer formatted:
// timestamp uint32 (details TBD; microseconds since execution start?)
// source [srcLen]byte (truncated, unterminated ASCII-only string)
// event  [evtLen]byte (truncated, unterminated ASCII-only string)
// oldval uint16
// newval uint16
// sev byte
// kind byte
func Report(src string, evt string, oldval uint16, newval uint16, sev byte, kind byte)  {
	logBuffer := bufferPair[bufferPairIndex]

	if bufOffset == bufLen {
		if _, err := binLog.Write(logBuffer); err != nil {
			fmt.Fprintf(os.Stderr, "log write error: %s\n", err.Error())
			os.Exit(2)
		}

		bufOffset = 0
		bufferPairIndex = 1 - bufferPairIndex
		logBuffer = bufferPair[bufferPairIndex]
	}

	copy(logBuffer[bufOffset:], zeroBytes)

	var runtimeMicroseconds uint32
	runtimeMicroseconds = uint32(time.Since(startTime).Nanoseconds() / 1000)
	binary.LittleEndian.PutUint32(logBuffer[bufOffset:], runtimeMicroseconds)
	bufOffset += 4

	copy(logBuffer[bufOffset:], src)
	bufOffset += srcLen
	copy(logBuffer[bufOffset:], evt)
	bufOffset += evtLen

	binary.LittleEndian.PutUint16(logBuffer[bufOffset:], oldval)
	bufOffset += 2
	binary.LittleEndian.PutUint16(logBuffer[bufOffset:], newval)
	bufOffset += 2

	logBuffer[bufOffset] = sev
	bufOffset += 1
	logBuffer[bufOffset] = kind
	bufOffset += 1

	if bufOffset&(recordSize-1) != 0 {
		panic(fmt.Sprintf("bufOffset %d", bufOffset))
	}
}

