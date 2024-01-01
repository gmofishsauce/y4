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
	"bufio"
)

type AST struct {
	ErrorCount int
}

func parse(in *bufio.Reader) (*AST, error) {
	ast := &AST{}
	ast.ErrorCount = 0
	return ast, nil
}
