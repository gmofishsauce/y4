/*
Copyright © 2022 Jeff Berkowitz (pdxjjb@gmail.com)

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <http://www.gnu.org/licenses/>.
*/

package main

import (
	"testing"
)

func TestSym1(t *testing.T) {
	st := MakeSymbolTable()
	value, index, err := st.Get("r5")
	check(t, err, nil)
	check(t, index, uint16(5))
	check(t, value, uint16(5))

	value, index, err = st.Get("nop")
	check(t, err, nil)
	check(t, value, uint16(0)) // means "no operands"
	if index == NoSymbol || index > uint16(8 + len(KeyTable)) {
		t.Errorf("st.Get(\"nop\"): bad index")
	}

	value, index, err = st.Get("r42")
	check(t, value, NoValue)
	check(t, index, NoSymbol)
	if err == nil {
		t.Errorf("st.Get(\"r42\"): fail expected")
	}
}

func TestSym2(t *testing.T) {
	v := sigFor(4, 3, 2)
	check(t, v, uint16(0x02340))
	check(t, numOperands(v), uint16(3))
	check(t, getSig(v, Ra), SignatureElement(4))
	check(t, getSig(v, Rc), SignatureElement(2))
}

func TestSym3(t *testing.T) {
	st := MakeSymbolTable()
	value, _, err := st.Get("lui")
	check(t, err, nil)
	check(t, numOperands(value), uint16(2))
	check(t, getSig(value, Ra), SeReg)
	check(t, getSig(value, Rb), SeImm10)
}

func TestSym4(t *testing.T) {
	st := MakeSymbolTable()
	_, _, err := st.Get("pdp11")
	if err == nil {
		t.Errorf("st.Get(\"pdp11\"): fail expected")
	}

	index, err := st.Use("pdp11")
	if err != nil {
		t.Errorf("st.Get(\"r42\"): success expected")
	}
	check(t, index, uint16(len(st.entries)-1))

	// This should fail because the symbol has only
	// been seen as a "use", not a "definition".
	_, index, err = st.Get("pdp11")
	if err == nil {
		t.Errorf("st.Get(\"pdp11\"): fail expected")
	}
}

