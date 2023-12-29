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

func (z *ZeroGenerator) Prepare() {
}

func (z *ZeroGenerator) Evaluate() []Bit {
	// Report(z.name, "zero src", 0, 0, SevInfo, KindEval) TODO FIXME
	return z.zeroes
}

func (z *ZeroGenerator) Check() error {
	return nil
}

func MakeZeroGenerator(s *System, name string, width int) *ZeroGenerator {
	result := &ZeroGenerator{name, make([]Bit, width, width)}
	RegisterComponent(s, result)
	return result
}

type EnableFunc func() bool

type Register struct {
	name string
	input []Component
	exposedState []Bit
	nextState []Bit
	width int
	nextStateValid bool
	nextClockEnabled bool
	enableFunc EnableFunc
}

func MakeRegister(s *System, name string, width int, en EnableFunc) *Register {
	result := &Register{}
	result.name = name
	result.exposedState = make([]Bit, width, width)
	result.nextState = make([]Bit, width, width)
	result.width = width
	result.nextStateValid = false
	result.nextClockEnabled = false
	result.enableFunc = en
	RegisterClockable(s, result)
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
	if n != r.width {
		return fmt.Errorf("%s: %d inputs, %d outputs", r.Name(), n, r.width)
	}
	return nil
}

func (r *Register) Reset() {
	r.exposedState = undefined(r.width)
	r.nextStateValid = false
	r.nextClockEnabled = false
}

func (r *Register) Prepare() {
	r.nextStateValid = false
}

// We must generate and cache all our next-state information at Evaluate()
// time to avoid ordering dependencies at PositiveEdge() time. But we always
// return our last state ... we're a register.
func (r *Register) Evaluate() []Bit {
	if !r.nextStateValid {
		n := 0
		for _, c := range r.input {
			bits := c.Evaluate()
			for _, b := range(bits) {
				r.nextState[n] = b
				n++
			}
		}
		r.nextClockEnabled = r.enableFunc()
		r.nextStateValid = true
	}
	// Report(r.name, "reg", r.nextClockEnabled, r.exposedState, SevInfo, KindEval) TODO FIXME
	return r.exposedState
}

func (r *Register) PositiveEdge() {
	// old := r.exposedState TODO FIXME
	if r.nextClockEnabled {
		r.exposedState = r.nextState
	}
	// Report(r.name, "reg", old, r.exposedState, SevInfo, KindEdge) TODO FIXME
}

