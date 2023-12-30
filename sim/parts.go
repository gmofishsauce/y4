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

func (z *ZeroGenerator) Prepare() {
}

func (z *ZeroGenerator) Evaluate() Bits {
	Report(z.name, "zero src", ZeroBits, ZeroBits, SevInfo, KindEval)
	return z.zeroes
}

func (z *ZeroGenerator) Check() error {
	return nil
}

func MakeZeroGenerator(s *System, name string, width int) *ZeroGenerator {
	result := &ZeroGenerator{name, MakeBits(16, 0, 0, 0)}
	RegisterComponent(s, result)
	return result
}

type EnableFunc func() bool

type Input struct {
	src Component
	mask uint16
	shift uint16
}

type Register struct {
	name string
	visibleState Bits
	cachedState Bits
	width uint16
	inputs []*Input
	cacheValid bool
	clockEnabled bool
	enableFunc EnableFunc
}

func MakeRegister(s *System, name string, width uint16, en EnableFunc) *Register {
	result := &Register{}
	result.name = name
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

// Add an input to register. In general, connections can be arbitrarily
// scrambled. Here we allow a width and a shift from output to input.
// This means e.g. two 4-bit registers can be sensibly connected to the
// input of an 8-bit register. We do not allow other bit reorderings.
func (r *Register) AddInput(c Component, width uint16, shift uint16) {
	if width + shift > MaxWidth {
		panic(fmt.Sprintf("%s.AddInput(%s, %d, %d): bad size\n",
			r.Name(), c.Name(), width, shift))
	}
	var proposedMask uint16 = ((1<<width)-1)<<shift

	// The shifted value is required to fit in this register
	allowed := MakeOnes(r.width)
	proposed := MakeBits(width, 0, 0, proposedMask)
	if allowed.value | proposed.value != allowed.value {
		panic(fmt.Sprintf("%s.AddInput(%s, %d, %d): bad fit\n",
			r.Name(), c.Name(), width, shift))
	}

	// Don't allow the new input to overlap an existing input
	var existingMask uint16
	for _, in := range r.inputs {
		existingMask |= (in.mask << in.shift)
	}
	if existingMask&proposedMask != 0 {
		panic(fmt.Sprintf("%s.AddInput(): inputs from %s overlap\n",
			r.Name(), c.Name()))
	}
	r.inputs = append(r.inputs, &Input{c, proposedMask, shift})
}

func (r *Register) Check() error {
	var thisMask uint16 = (1<<r.width) - 1
	var inputMask uint16
	for _, in := range r.inputs {
		inputMask |= in.mask << in.shift
	}
	if thisMask != inputMask {
		return fmt.Errorf("%s: this 0x%x, inputs 0x%x", r.Name(), thisMask, inputMask)
	}
	return nil
}

func (r *Register) Reset() {
	r.visibleState = UndefinedBits
	r.cacheValid = false
	r.clockEnabled = false
}

func (r *Register) Prepare() {
	r.cacheValid = false
}

// We must generate and cache all our next-state information at Evaluate()
// time to avoid ordering dependencies at PositiveEdge() time. But we always
// return our state from the previous clock edge... we're a register.
func (r *Register) Evaluate() Bits {
	if !r.cacheValid {
		r.clockEnabled = r.enableFunc()
		if r.clockEnabled {
			r.cachedState = ZeroBits
			//foreach input {
			//	input.undefined |= input.highz
			//}
			//for _, in := range r.inputs {
			//  FIXME
			//}
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

