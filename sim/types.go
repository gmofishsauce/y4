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

// In a digital simulation, bits have a minimum of three possible
// values: 1, 0, and at least one kind of undefined state. Since this
// simulation isn't intended for systems with very large numbers of
// bits, we don't attempt to pack them.

type Bit byte
const bUndef = Bit('U')
const bHighz = Bit('Z')

var undef64 []Bit = []Bit{
	bUndef, bUndef, bUndef, bUndef, bUndef, bUndef, bUndef, bUndef,
	bUndef, bUndef, bUndef, bUndef, bUndef, bUndef, bUndef, bUndef,
	bUndef, bUndef, bUndef, bUndef, bUndef, bUndef, bUndef, bUndef,
	bUndef, bUndef, bUndef, bUndef, bUndef, bUndef, bUndef, bUndef,
	bUndef, bUndef, bUndef, bUndef, bUndef, bUndef, bUndef, bUndef,
	bUndef, bUndef, bUndef, bUndef, bUndef, bUndef, bUndef, bUndef,
	bUndef, bUndef, bUndef, bUndef, bUndef, bUndef, bUndef, bUndef,
	bUndef, bUndef, bUndef, bUndef, bUndef, bUndef, bUndef, bUndef,
}

var highz64 []Bit = []Bit{
	bHighz, bHighz, bHighz, bHighz, bHighz, bHighz, bHighz, bHighz,
	bHighz, bHighz, bHighz, bHighz, bHighz, bHighz, bHighz, bHighz,
	bHighz, bHighz, bHighz, bHighz, bHighz, bHighz, bHighz, bHighz,
	bHighz, bHighz, bHighz, bHighz, bHighz, bHighz, bHighz, bHighz,
	bHighz, bHighz, bHighz, bHighz, bHighz, bHighz, bHighz, bHighz,
	bHighz, bHighz, bHighz, bHighz, bHighz, bHighz, bHighz, bHighz,
	bHighz, bHighz, bHighz, bHighz, bHighz, bHighz, bHighz, bHighz,
	bHighz, bHighz, bHighz, bHighz, bHighz, bHighz, bHighz, bHighz,
}

func undefined(width int) []Bit {
	if width > 64 {
		panic("undefined > 64")
	}
	return undef64[0:width]
}

func highz(width int) []Bit {
	if width > 64 {
		panic("highz > 64")
	}
	return highz64[0:width]
}

// A Component implements combinational logic.

type Component interface {
	Name() string
	Check() error
	Prepare()
	Evaluate() []Bit
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
