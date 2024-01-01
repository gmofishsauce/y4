/*
Copyright © 2022 Jeff Berkowitz (pdxjjb@gmail.com)

This program is free software: you can redistribute it and/or modify it
under the terms of the GNU Affero General Public License as published
by the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful, but
WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU
Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public
License along with this program. If not, see
<http://www.gnu.org/licenses/>.
*/
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
)

var dflag = flag.Bool("d", false, "enable debug")

func main() {
	flag.Parse()

	args := flag.Args()
	if len(args) != 1 {
		usage()
	}
	name := args[0]
	src, err := os.Open(name)
	if err != nil {
		fatal(fmt.Sprintf("open source file %s: %s\n", name, err))
	}
	defer src.Close()

	ast, err := parse(bufio.NewReader(src))
	if err != nil {
		fatal(fmt.Sprintf("%s: %d errors\n", name, ast.ErrorCount))
	}
	code, err := generate(ast)
	if err != nil {
		fatal(fmt.Sprintf("%s: %d errors\n", name, code.ErrorCount))
	}
}

func usage() {
	pr("Usage: asm [options] source-file\nOptions:")
	flag.PrintDefaults()
	os.Exit(1)
}

