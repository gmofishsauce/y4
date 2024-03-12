/*
Copyright © 2022 Jeff Berkowitz (pdxjjb@gmail.com)

This program is free software: you can redistribute it and/or modify it
under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or (at your
option) any later version.

This program is distributed in the hope that it will be useful, but
WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU
Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public
License along with this program. If not, see <http://www.gnu.org/licenses/>.
*/
package main

import "strings"

const k64 = 64*1024

// This simulator is aimed at 16-bit computer designs. So simulated bits
// come in groups of 16, each called a Bits. Any group of 16 or fewer bits 
// can be represented by one Bits. Each bit in a Bits can be in one of four
// states: 0, 1, high impedence ("Z"), or undefined. These correspond exactly
// to the four bit states of Verilog. The additional "weak" states of VHDL
// STD_LOGIC are not representable. When viewed as a little endian 64-bit
// int, the width is in the MS bits and the value is in the LS bits. Width
// in the MS bits ensures that the 64-bit value will be positive.

type Bits struct {
	value uint16
	highz uint16
	undef uint16
	width uint16
}

const MaxWidth uint16 = 16
const AllBits uint16 = 0xFFFF

func MakeBits(w uint16, u uint16, z uint16, v uint16) Bits {
	return Bits{width: w, highz: z, undef: u, value: v}
}

func MakeZero(w uint16) Bits {
	return Bits{width: w, highz: 0, undef: 0, value: 0}
}
var ZeroBits = MakeZero(16)

func MakeOnes(w uint16) Bits {
	return Bits{width: w, highz: 0, undef: 0, value: AllBits}
}
var OneBits = MakeOnes(16)

func MakeHighz(w uint16) Bits {
	return Bits{width: w, highz: AllBits, undef: 0, value: 0}
}
var HighzBits = MakeHighz(16)

func MakeUndefined(w uint16) Bits {
	return Bits{width: w, highz: 0, undef: AllBits, value: 0}
}
var UndefBits = MakeUndefined(16)

func boolToBits(b bool) Bits {
	if b {
		return OneBits
	}
	return ZeroBits
}

// This is a convenience for the binary log writer.
func (b Bits) toUint64() uint64 {
	return uint64(b.width)<<48 | uint64(b.highz)<<32 | uint64(b.undef)<<16 | uint64(b.value)
}

// This is a convenience for the binary log reader.
func fromUint64(bits uint64) Bits {
	return Bits{value: uint16(bits)&AllBits, undef:uint16(bits>>16)&AllBits,
				highz:uint16(bits>>32)&AllBits, width:uint16(bits>>48)&0x1F }
}

// A Component implements combinational logic. Components must register
// themselves with the System when they are created.

type Component interface {
	Name() string
	Width() uint16
	Check() error
	Prepare()
	Evaluate() Bits
}

// A Clockable computes its internal state on a positive going clock edge
// and produces the last output when asked for it. Clockables must register
// themselves with the System when they are created.

type Clockable interface {
	Component
	Reset()
	PositiveEdge()
}

type System struct {
	logic []Component
	state []Clockable
	imem []uint16
	dmem []byte
}

// Register a Clockable (make it part of the System)
func RegisterClockable(s *System, c Clockable) {
	s.state = append(s.state, c)
}

// Register a Component (make it part of the System)
func RegisterComponent(s *System, c Component) {
	s.logic = append(s.logic, c)
}

func MakeSystem() (*System, error) {
	result := &System{}
	result.imem = make([]uint16, k64, k64)
	result.dmem = make([]byte, k64, k64)
	return result, nil
}

// Severities and kinds for the binary logger
const (
	SevError = byte('E')
	SevWarn = byte('W')
	SevInfo = byte('I')
	SevDebug = byte('D')
	KindEval = byte('E')
	KindReset = byte('R')
	KindEdge = byte('^')
)

// ErrorList collection class implements error

type ErrorList struct {
	errors []error
}

func (list *ErrorList) appendIfNotNil(err error) {
	if err == nil {
		return
	}
	list.errors = append(list.errors, err)
}

func (list ErrorList) Error() string {
	if list.Length() == 0 { // shouldn't happen
		return "(no errors)"
	}
	var sb strings.Builder
	for _, err := range(list.errors) {
		sb.WriteString(err.Error())
		sb.WriteString("\n")
	}
	return sb.String()
}

func (list *ErrorList) Length() int {
	return len(list.errors)
}

