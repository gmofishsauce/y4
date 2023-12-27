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

import (
	"fmt"
)

// Zero generator component

type ZeroGenerator struct {
	name string
	zeroes []Bit
}

func (z *ZeroGenerator) Name() string {
	return z.name
}

func (z *ZeroGenerator) Evaluate() []Bit {
	return z.zeroes
}

func (z *ZeroGenerator) Check() error {
	return nil
}

func MakeZeroGenerator(name string, width int) *ZeroGenerator {
	result := &ZeroGenerator{name, make([]Bit, width, width)}
	RegisterComponent(result)
	return result
}

type EnableFunc func() bool

type Register struct {
	name string
	input []Component
	state []Bit
	clockEnable EnableFunc
}

func MakeRegister(name string, width int, en EnableFunc) *Register {
	result := &Register{name: name, state: make([]Bit, width, width), clockEnable: en}
	RegisterClockable(result)
	return result
}

func (r *Register) Name() string {
	return r.name
}

func (r *Register) AddInput(c Component) {
	r.input = append(r.input, c)
}

func (r *Register) Check() error {
	n := 0
	for _, c := range r.input {
		n += len(c.Evaluate())
	}
	if n != len(r.state) {
		return fmt.Errorf("%s: %d inputs, %d outputs", r.Name(), n, len(r.state))
	}
	return nil
}

func (r *Register) Evaluate() []Bit {
	return r.state
}

func (r *Register) PositiveEdge() {
	if r.clockEnable() {
		n := 0
		for _, c := range r.input {
			bits := c.Evaluate()
			for _, b := range(bits) {
				r.state[n] = b
				n++
			}
		}
	}
}

