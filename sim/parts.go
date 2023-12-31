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
	zeroes Bits
}

func (z *ZeroGenerator) Name() string {
	return z.name
}

func (z *ZeroGenerator) Width() uint16 {
	return z.zeroes.width
}

func (z *ZeroGenerator) Prepare() {
}

func (z *ZeroGenerator) Evaluate() Bits {
	Report(z.name, "zero src", ZeroBits, ZeroBits, SevInfo, KindEval)
	return z.zeroes
}

func (z *ZeroGenerator) Check() error {
	return nil
}

func MakeZeroGenerator(s *System, name string, width uint16) *ZeroGenerator {
	result := &ZeroGenerator{name, MakeBits(width, 0, 0, 0)}
	RegisterComponent(s, result)
	return result
}

// A Register has a clock enable function. This function is combinational and
// must be called only at Evaluate() time. If it returns true, the Register
// will be updated at the following PositiveEdge().
type EnableFunc func() bool

// A Register has a single properly-aligned full-width input Component. In
// the simple case, it's the output of another aligned, full-width Component.
// In the harder cases, a wiring net Component like a Bus or a Combiner must
// be used; these Components have more complicated input structures.
// A Register has an Enable function that is called at Evaluate() time. If
// it returns true, a next value is computed and cached. The visible value
// is then updated from the next value at PositiveEdge() time. The cached
// value is cleared at Prepare() time and the cycle repeats.
type Register struct {
	name string
	input Component
	enableFunc EnableFunc
	visibleState Bits
	cachedState Bits
	width uint16
	cacheValid bool
	clockEnabled bool
}

func MakeRegister(s *System, name string, width uint16, input Component, en EnableFunc) *Register {
	result := &Register{}
	result.name = name
	result.input = input
	result.visibleState = MakeUndefined(width)
	result.cachedState = MakeUndefined(width)
	result.width = width
	result.cacheValid = false
	result.clockEnabled = false
	result.enableFunc = en
	RegisterClockable(s, result)
	return result
}

func (r *Register) Name() string {
	return r.name
}

func (r *Register) Width() uint16 {
	return r.width
}

func (r *Register) Check() error {
	if r.input == nil || r.input.Width() != r.width {
		return fmt.Errorf("%s: invalid input: %[1]v %[1]T", r.input)
	}
	return nil
}

func (r *Register) Reset() {
	r.visibleState = UndefBits
	r.cacheValid = false
	r.clockEnabled = false
	Report(r.name, "", ZeroBits, r.visibleState, SevInfo, KindEval)
}

func (r *Register) Prepare() {
	r.cacheValid = false
}

// We must generate and cache all our next-state information at Evaluate()
// time to avoid ordering dependencies at PositiveEdge() time. But we always
// return our visible state from the previous clock edge: we're a Register.
func (r *Register) Evaluate() Bits {
	if !r.cacheValid {
		r.clockEnabled = r.enableFunc()
		if r.clockEnabled {
			r.cachedState = r.input.Evaluate()
		}
		r.cacheValid = true
	}
	Report(r.name, "", boolToBits(r.clockEnabled), r.cachedState, SevInfo, KindEval)
	return r.visibleState
}

func (r *Register) PositiveEdge() {
	old := r.visibleState
	if r.clockEnabled {
		r.visibleState = r.cachedState
	}
	Report(r.name, "reg", old, r.visibleState, SevInfo, KindEdge)
}

