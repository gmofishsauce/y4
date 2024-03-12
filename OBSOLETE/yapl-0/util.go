/*
Copyright © 2024 Jeff Berkowitz (pdxjjb@gmail.com)

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
	"fmt"	// only for Fprintln of a single string
	"os"	// only for os.Exit
)

var debug bool

func assert(b bool, msg string) {
    if !b {
        panic("assertion failure: " + msg)
    }
}

func fatal(s string) {
    pr(s)
    os.Exit(2)
}

func pr(s string) {
    fmt.Fprintln(os.Stderr, s)
}

func dbg(s string) {
	if debug {
		pr(s)
	}
}
