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

// ========================
// Zero generator component
// ========================

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
	Report(z.name, "zero source", z.zeroes, z.zeroes, SevInfo, KindEval)
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

// ==================================================================
// Register component (with clock enable but without 3-state outputs)
// ==================================================================

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

func (r *Register) AddInput(c Component) error {
	if c == nil {
		panic(fmt.Sprintf("%s.AddInput(): nil arg", r.Name))
	}
	if r.width != c.Width() {
		return fmt.Errorf("%s.AddInput(%s): width mismatch", r.Name(), c.Name())
	}
	r.input = c
	return nil
}

func (r *Register) Name() string {
	return r.name
}

func (r *Register) Width() uint16 {
	return r.width
}

func (r *Register) Check() error {
	if r.input == nil || r.input.Width() != r.width {
		return fmt.Errorf("%s: invalid input: %[2]v (type %[2]T)", r.name, r.input)
	}
	return nil
}

func (r *Register) Reset() {
	r.visibleState = UndefBits
	r.cacheValid = false
	r.clockEnabled = false
	Report(r.name, "", ZeroBits, r.visibleState, SevInfo, KindReset)
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

// ======================================================
// N-input multiplexer component - N must be a power of 2
// ======================================================

// A multiplexer is a combinational component. It has a control input of
// N bits where N is small and a list of 2^N data inputs. Its output is
// its nth data input where n is the value of the N bit control. All inputs
// must be single components. The control input often requires a Combiner
// (wire net).
type Mux struct {
	name string
	control Component
	data []Component
	cachedState Bits
	dataWidth uint16
	cacheValid bool
}

func MakeMux(s *System, name string, width uint16) *Mux {
	result := &Mux{}
	result.name = name
	result.dataWidth = width
	result.cacheValid = false
	RegisterComponent(s, result)
	return result
}

func (m *Mux) Name() string {
	return m.name
}

func (m *Mux) Width() uint16 {
	return m.dataWidth
}

func (m *Mux) AddControl(c Component) error {
	if c == nil || c.Width() > 3 { // we don't allow more than 8 inputs
		return fmt.Errorf("%s.AddControl(%s): invalid argument", m, c)
	}
	if len(m.data) > 0 {
		return fmt.Errorf("%s.AddControl(): only one AddControl() per mux", m)
	}
	m.control = c
	nInput := 1<<c.Width()
	m.data = make([]Component, nInput, nInput)
	return nil
}

// Add component c to mux m on input in. The control component must already
// have been added to establish the maxiumum allowed value of in.
func (m *Mux) AddData(c Component, in uint16) error {
	if m.control == nil || c == nil || c.Width() != m.Width() || in >= (1 << m.control.Width()) {
		return fmt.Errorf("%s.AddData(): input %v invalid on %d", m, c, in)
	}
	m.data[in] = c
	return nil
}

func (m *Mux) Check() error {
	if m.control == nil {
		return fmt.Errorf("%s: no control", m.Name())
	}
	for i, in := range m.data {
		if in == nil {
			return fmt.Errorf("%s: no data source on input %d", m.Name(), i)
		}
	}
	return nil
}

func (m *Mux) Prepare() {
	m.cacheValid = false
}

func (m *Mux) Evaluate() Bits {
	old := m.cachedState
	if !m.cacheValid {
		selector := m.control.Evaluate().value
		m.cachedState = m.data[selector].Evaluate()
		m.cacheValid = true
	}
	Report(m.Name(), "mux", old, m.cachedState, SevInfo, KindEval)
	return m.cachedState 
}
