package main

/*
Author: Jeff Berkowitz
Copyright (C) 2023 Jeff Berkowitz

This file is part of sim.

Sim is free software; you can redistribute it and/or
modify it under the terms of the GNU General Public License
as published by the Free Software Foundation, either version 3
of the License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program. If not, see http://www.gnu.org/licenses/.
*/

import (
	"fmt"
)

const (
	SymReserved uint16 = 0
	SymKey uint16 = 1
	SymLabel uint16 = 2
	SymValue uint16 = 3
)

// To save on space in the MachineInstruction structures,
// we need to allow symbols to be indexed by a 16-bit value.
// So we  allocate the symEntries sequentially from an array
// and hash the key to the array index. Space management is
// easy because nothing gets freed during a parse and everything
// can be freed after the parse (we currently don't bother).

type SymbolEntry struct {
	kind uint16
	value uint16
}

type SymbolIndex uint16

type SymbolTable struct {
	indexes map[string]SymbolIndex
	entries []SymbolEntry
}

func MakeSymbolTable() *SymbolTable {
	symTab := &SymbolTable{}
	symTab.indexes = make(map[string]SymbolIndex)
	symTab.entries = make([]SymbolEntry, 0, 0)

	// FIXME the value will be the types of all operands
	// FIXME there should be only one table of opeerations
	// and operand types in the assember, shared with the
	// parser and maybe elsewhere.
	// FIXME the list below is incomplete.

	symTab.defineSymbol("adi", SymKey, 0)
	symTab.defineSymbol("beq", SymKey, 0)
	symTab.defineSymbol("lb", SymKey, 0)
	symTab.defineSymbol("li", SymKey, 0)
	symTab.defineSymbol("lli", SymKey, 0)
	symTab.defineSymbol("lui", SymKey, 0)
	symTab.defineSymbol("sb", SymKey, 0)
	symTab.defineSymbol("sw", SymKey, 0)
	symTab.defineSymbol("nop", SymKey, 0)
	symTab.defineSymbol("jalr", SymKey, 0)
	symTab.defineSymbol(".fill", SymKey, 0)
	symTab.defineSymbol(".space", SymKey, 0)
	symTab.defineSymbol(".set", SymKey, 0)

	return symTab
}

func (st *SymbolTable) defineSymbol(name string, kind uint16, value uint16) error {
	if _, trouble := st.indexes[name]; trouble {
		return fmt.Errorf("%s: symbol exists", name)
	}
	if len(st.entries) > 0xFFFE {
		return fmt.Errorf("symbol table overflow")
	}

	var index SymbolIndex = SymbolIndex(len(st.entries))
	st.entries = append(st.entries, SymbolEntry{kind: kind, value: value})
	st.indexes[name] = index
	return nil
}

func (st *SymbolTable) isKeySymbol(name string) bool {
	if index, ok := st.indexes[name]; ok && st.entries[index].kind == SymKey {
		return true
	}
	return false
}
