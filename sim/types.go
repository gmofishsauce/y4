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

const k64 = 64*1024

// This simulator is aimed at 16-bit computer designs. So simulated bits
// come in groups of 16, each called a Bits. Any group of 16 or fewer bits 
// can be represented by one Bits. Each bit in a Bits can be in one of four
// states: 0, 1, high impedence ("Z"), or undefined. These correspond exactly
// to the four bit states of Verilog. The additional "weak" states of VHDL
// STD_LOGIC are not represented.

type Bits struct {
	value uint16
	highz uint16
	undef uint16
	width uint16
}

const MaxWidth uint16 = 16
const AllBits uint16 = 0xFFFF

// Make a Bits from its four components, guarding against overflow
func MakeBits(width uint16, undef uint16, highz uint16, value uint16) Bits {
	return Bits{value, highz, undef, width}
}

func MakeZero(width uint16) Bits {
	return MakeBits(width, 0, 0, 0)
}
var ZeroBits = MakeZero(16)

func MakeOnes(width uint16) Bits {
	return MakeBits(width, 0, 0, AllBits)
}
var OneBits = MakeOnes(16)

func MakeHighz(width uint16) Bits {
	return MakeBits(width, 0, AllBits, 0)
}
var HighzBits = MakeHighz(16)

func MakeUndefined(width uint16) Bits {
	return MakeBits(width, AllBits, 0, 0)
}
var UndefinedBits = MakeUndefined(16)

// This is a convenience for the binary log writer
func (b Bits) toUint64() uint64 {
	return uint64(b.width)<<48 | uint64(b.undef)<<32 | uint64(b.highz)<<16 | uint64(b.value)
}

// A Component implements combinational logic.

type Component interface {
	Name() string
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

func RegisterClockable(s *System, c Clockable) {
	s.state = append(s.state, c)
}

func RegisterComponent(s *System, c Component) {
	s.logic = append(s.logic, c)
}

func MakeSystem() (*System, error) {
	result := &System{}
	result.imem = make([]uint16, k64, k64)
	result.dmem = make([]byte, k64, k64)
	return result, nil
}

func boolToBits(b bool) Bits {
	if b {
		return OneBits
	}
	return ZeroBits
}
