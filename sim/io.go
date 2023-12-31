package main

/*
Author: Jeff Berkowitz
Copyright (C) 2023 Jeff Berkowitz

This file is part of sim.

Sim is free software; you can redistribute it and/or
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
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"time"
)

func fatal(s string) {
	pr(s)
	os.Exit(2)
}

func pr(s string) {
	fmt.Fprintf(os.Stderr, "%s\n", s)
}

// Binary logger. Writes fixed-length 64-byte records to bin.log
// The logger is not threadsafe. The simulator is single threaded.

const LogFileName = "log.bin"
var binLog *os.File
var startTime time.Time

func OpenLog() error {
	var err error
	if binLog, err = os.Create(LogFileName); err != nil {
		return err
	}
	startTime = time.Now()
	return nil
}

const recordSize = 64
const recordsPerBuffer = 128
const bufLen = recordSize * recordsPerBuffer
var buf1 []byte = make([]byte, bufLen, bufLen)
var buf2 []byte = make([]byte, bufLen, bufLen)
var bufferPair [2][]byte = [2][]byte{buf1, buf2}
var bufferPairIndex = 0

var bufOffset int // 0..8k by bufLen, then back to 0
const srcLen = 16
const evtLen = 16
var zeroBytes []byte = make([]byte, recordSize, recordSize)

// Written to a packed binary buffer formatted:
// timestamp uint64 (ns since execution start) (8 bytes)
// source [srcLen]byte (truncated unterminated ASCII-only string)
// event  [evtLen]byte (truncated unterminated ASCII-only string)
// b0 Bits (struct Bits value) (8 bytes)
// b1 Bits (struct Bits value) (8 bytes)
// sev byte
// kind byte
// 6 bytes unused = 64
func Report(src string, evt string, b0 Bits, b1 Bits, sev byte, kind byte)  {
	logBuffer := bufferPair[bufferPairIndex]

	if bufOffset == bufLen {
		// I experimented with handing off to a background writer but
		// found that it wasn't worth the trouble. I can write something
		// like 10 million records per second with this code.
		if _, err := binLog.Write(logBuffer); err != nil {
			fmt.Fprintf(os.Stderr, "log write error: %s\n", err.Error())
			os.Exit(2)
		}

		bufOffset = 0
		bufferPairIndex = 1 - bufferPairIndex
		logBuffer = bufferPair[bufferPairIndex]
	}

	copy(logBuffer[bufOffset:], zeroBytes)

	var runtimeMicroseconds uint64
	runtimeMicroseconds = uint64(time.Since(startTime).Nanoseconds())
	binary.LittleEndian.PutUint64(logBuffer[bufOffset:], runtimeMicroseconds)
	bufOffset += 8 // now 8

	copy(logBuffer[bufOffset:], src)
	bufOffset += srcLen // now 24
	copy(logBuffer[bufOffset:], evt)
	bufOffset += evtLen // now 40

	binary.LittleEndian.PutUint64(logBuffer[bufOffset:], b0.toUint64())
	bufOffset += 8 // now 48
	binary.LittleEndian.PutUint64(logBuffer[bufOffset:], b1.toUint64())
	bufOffset += 8 // now 56

	logBuffer[bufOffset] = sev
	bufOffset += 1 // now 57
	logBuffer[bufOffset] = kind
	bufOffset += 1 // now 58

	bufOffset += 6 // unused space at end; now 64
	if bufOffset&(recordSize-1) != 0 {
		panic(fmt.Sprintf("bufOffset %d", bufOffset))
	}
}

func CloseLog() {
	if bufOffset != 0 {
		logBuffer := bufferPair[bufferPairIndex]
		if _, err := binLog.Write(logBuffer[0:bufOffset]); err != nil {
			fmt.Fprintf(os.Stderr, "log write error during close: %s\n", err.Error())
		}
	}
	binLog.Close()
}

func trim(b []byte) string {
	var i int
	for i = 0; i < len(b); i++ {
		if b[i] == 0 {
			break
		}
	}
	return string(b[0:i])
}

func Dumplog() error {
	var f *os.File
	var err error
	if f, err = os.Open("./log.bin"); err != nil {
		fatal(fmt.Sprintf("open log.bin: %s\n", err.Error()))
	}
	defer binLog.Close()

	var n int
	var at int64 = 0
	buf := make([]byte, recordSize, recordSize)
	const billion = 1_000_000_000

	for n, err = f.ReadAt(buf, at) ; err == nil ; n, err = f.ReadAt(buf, at) {
		ts := binary.LittleEndian.Uint64(buf[0:8])
		b0 := fromUint64(binary.LittleEndian.Uint64(buf[40:48]))
		b1 := fromUint64(binary.LittleEndian.Uint64(buf[48:56]))

		fmt.Printf("%4d.%06d: %-16s %-16s {%4X %04X %04X %04X} {%4X %04X %04X %04X} %c %c\n",
			ts / billion, // timestamp uint64 seconds part
			ts % billion, // timestamp uint64 billionths of a second part
			trim(buf[8:24]), // source
			trim(buf[24:40]), // event
			b0.width, b0.undef, b0.highz, b0.value, // struct bits b0
			b1.width, b1.undef, b1.highz, b1.value, // struct bits b1
			rune(buf[56]), // sev byte
			rune(buf[57]), // kind byte
		)
		at += recordSize
	}
	if n == 0 {
		return nil
	}
	return err
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

