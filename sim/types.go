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

// In a digital simulation, bits have a minimum of three possible
// values: 1, 0, and at least one kind of undefined state. Since this
// simulation isn't intended for systems with very large numbers of
// bits, we don't attempt to pack them.

type Bit byte

// A Component implements combinational logic.

type Component interface {
	Name() string
	Check() error
	Evaluate() []Bit
}

// A Clockable computes its internal state on a positive going clock edge
// and produces the last output when asked for it. Clockables must register
// themselves with the System when they are created.

type Clockable interface {
	Component
	PositiveEdge()
}

