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
	"fmt"
	"os"
)

func main() {
    Report("starting...")
	s, err := Build()
	if err != nil {
		Report(err.Error())
		os.Exit(2)
	}
	if err := Check(s); err != nil {
		Report(err.Error())
		os.Exit(3)
	}
	if err = Simulate(s); err != nil {
		Report(err.Error())
		os.Exit(4)
	}
	Report("success")
	os.Exit(0)
}

const k64 = 64*1024

// Make all the system components and wire them together.
// In time, command line flags will offer a choice of implementations.
func Build() (*system, error) {
	Report("building...")
	if err := Sequential(); err != nil {
		return nil, err
	}

	sys.imem = make([]uint16, k64, k64)
	sys.dmem = make([]byte, k64, k64)
	return &sys, nil
}

// Components can't check themselves during Build() because they
// can't know if another AddInput() call might be coming, etc.
// This is called after build returns and calls Check() on all
// the components that registered themselves during Build().
func Check(s *system) error {
	Report("checking...")
	var nError int = 0
	for _, cl := range s.state {
		dbg("clockable: %s", cl.Name())
		if err := cl.Check(); err != nil {
			nError++
			Report(err.Error())
		}
	}
	for _, co := range sys.logic {
		dbg("combinational: %s", co.Name())
		if err := co.Check(); err != nil {
			nError++
			Report(err.Error())
		}
	}
	if nError > 0 {
		s := "s" // Oh for a ternary ...
		if nError == 1 {
			s = ""
		}
		return fmt.Errorf("%d error%s found in circuit", nError, s)
	}
	return nil
}

func Simulate(s *system) error {
	Report("simulating...")
	return nil
}

func Report(s string) {
	fmt.Printf("%s\n", s)
}

type system struct {
	logic []Component
	state []Clockable
	imem []uint16
	dmem []byte
}

func RegisterClockable(c Clockable) {
	sys.state = append(sys.state, c)
}

func RegisterComponent(c Component) {
	sys.logic = append(sys.logic, c)
}

var sys system

