package main

/*
Author: Jeff Berkowitz
Copyright (C) 2023 Jeff Berkowitz

This file is part of asm.

Asm is free software; you can redistribute it and/or
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

// Maximum number of symbols in symbol table. We enforce a limit of 2^15-2
// symbols because it's adequate and it's convenient for other parts of the
// implementation (symbol indexes are always positive).
const MaxSymbols = 0x7FFE
const NoValue uint16 = 0x7FFF   // Returned when cannot get the value of a symbol
const NoSymbol uint16 = 0x7FFF	// Returned when cannot get the index of a symbol

// Undefined symbols can later become defined. The value of a defined
// symbol may not be changed. Symbols can be negated before definition.
const symDefined uint16 = 0x8000
const symNegated uint16 = 0x4000

// To save on space in the MachineInstruction structures, we allow
// symbols to be indexed by a 16-bit value. We  allocate symbolEntry
// structs sequentially from an array and hash the key to the array
// index. Space management is easy because nothing gets freed during
// a parse and everything can be freed after the parse if desired.

type symbolEntry struct {
	flags uint16 // symDefined, symNegated
	value uint16
}

type SymbolTable struct {
	indexes map[string]uint16
	entries []symbolEntry
}

// Initialize the symtab by creating all the reserved entries. The first
// 8 entries in the symbol table are the registers r0..r7. This creates an
// identity mapping so that e.g. SymbolTable.indexes["r5"] == 5. Then add
// all the key symbols.
func MakeSymbolTable() *SymbolTable {
	symTab := &SymbolTable{}
	symTab.indexes = make(map[string]uint16)
	symTab.entries = make([]symbolEntry, 0, 64)

	for i := 0; i < 8; i++ {
		symTab.internalCreateSymbol("r" + string(rune('0'+i)), symDefined, uint16(i))
	}
	for _, keyEntry := range KeyTable {
		symTab.internalCreateSymbol(keyEntry.name, symDefined, keyEntry.signature)
	}

	return symTab
}

// Define a symbol. The symbol may not exist or may exist in the undefined state
// Return the symbol's index, a uint16 <= MaxSymbols.
func (st *SymbolTable) Define(name string, value uint16) (uint16, error) {
	index, exists := st.indexes[name]
	if exists {
		entry := st.entries[index]
		if entry.flags&symDefined != 0 {
			return NoSymbol, fmt.Errorf("%s redefined", name)
		}
		entry.flags |= symDefined
		return index, nil
	}
	return st.internalCreateSymbol(name, symDefined, value)
}

// A symbol use has been seen. The symbol may or may not be exist; if not, we
// enter it as an undefined symbol (forward reference).
func (st *SymbolTable) Use(name string) (uint16, error) {
	index, exists := st.indexes[name]
	if exists {
		return index, nil
	}
	return st.internalCreateSymbol(name, 0, NoValue)
}

// Add a symbol to the symbol table. The added symbol may be "defined" or simply "used".
// If the symbol was used before definition, the value will be NoValue.
// Return the index of the symbol entry or an error indicating symbol table overflow.
func (st *SymbolTable) internalCreateSymbol(name string, flags uint16, value uint16) (uint16, error) {
	if len(st.entries) == MaxSymbols {
		return NoSymbol, fmt.Errorf("symbol table overflow")
	}
	var index uint16 = uint16(len(st.entries))
	st.entries = append(st.entries, symbolEntry{flags: flags, value: value})
	st.indexes[name] = index
	return index, nil
}

// Get the value and symbol index of a defined symbol.
// XXX - it's easy to misinterpret the order of the first two return value - ugly
// XXX - to get the index of a symbol that's used by not defined, Use()
func (st *SymbolTable) Get(name string) (value uint16, index uint16, err error) {
	index, ok := st.indexes[name]
	if !ok {
		return NoValue, NoSymbol, fmt.Errorf("undefined: %s", name)
	}
	entry := st.entries[index]
	if entry.flags&symDefined == 0 {
		return NoValue, index, fmt.Errorf("used by not defined: %s", name)
	}
	return entry.value, index, nil
}

// Negate the value of a symbol. The symbol need not be defined yet, because
// the language allows e.g. adi r1, r2, -foo and then later .set foo 19. This
// is a hack around not having a real expression parser.
func (st *SymbolTable) Negate(index uint16) error {
	st.entries[index].flags |= symNegated
	return nil
}

