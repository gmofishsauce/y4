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
	n, err := st.Get("r5")
	check(t, err, nil)
	check(t, n, uint16(5))

	n, err = st.Get("r42")
	if err == nil {
		t.Errorf("st.Get(\"r42\"): fail expected")
	}
}

func TestSym2(t *testing.T) {
	v := sigFor(4, 3, 2)
	check(t, v, uint16(0x0234))
}

func TestSym3(t *testing.T) {
	st := MakeSymbolTable()
	sig, err := st.Get("lui")
	check(t, err, nil)
	check(t, numOperands(sig), 2)
	check(t, getSig(sig, 0), SeReg)
	check(t, getSig(sig, 1), SeImm10)
}

func TestSym4(t *testing.T) {
	st := MakeSymbolTable()
	n, err := st.Get("pdp11")
	if err == nil {
		t.Errorf("st.Get(\"pdp11\"): fail expected")
	}

	n, err = st.UseSymbol("pdp11", 11)
	if err != nil {
		t.Errorf("st.Get(\"r42\"): success expected")
	}
	check(t, n, uint16(len(st.entries)-1))

	// This should fail because the symbol has only
	// been seen as a "use", not a "definition".
	n, err = st.Get("pdp11")
	if err == nil {
		t.Errorf("st.Get(\"pdp11\"): fail expected")
	}
}

