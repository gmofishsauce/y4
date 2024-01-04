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
	ast, err := parse(args[0])
	if err != nil {
		fatal(fmt.Sprintf("%s: %d errors\n", args[0], ast.ErrorCount))
	}
	err = generate(ast)
	if err != nil {
		fatal(fmt.Sprintf("%s: %s errors\n", args[0], err.Error()))
	}
}

func usage() {
	pr("Usage: asm [options] source-file\nOptions:")
	flag.PrintDefaults()
	os.Exit(1)
}

