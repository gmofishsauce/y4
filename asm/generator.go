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

var GeneratorDebug = false

func Generate(symbols *SymbolTable, instructions *[]MachineInstruction) error {
	return nil
}

	/*
	if GeneratorDebug {
		dbg("generate(): not implemented")
	}

	if GeneratorDebug {
		// dump machine instructions
		for i := range *instructions {
			mi := (*instructions)[i]
			//key := KeyTable[mi.parts[0]].name
			//dbg("key %5s rA 0x%04X rB 0x%04X rC 0x%04X",
			//    key, mi.parts[1], mi.parts[2], mi.parts[3])
			//dbg("key 0x%04X rA 0x%04X rB 0x%04X rC 0x%04X",
			//    mi.parts[0], mi.parts[1], mi.parts[2], mi.parts[3])
		}
	}
	*/
