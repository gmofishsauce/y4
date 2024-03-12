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

func main() {
    vflag := flag.Bool("v", false, "dump binary log file")
	flag.Parse()

	if *vflag {
		if err := Dumplog(); err != nil {
			fatal(fmt.Sprintf("show %s: %s\n", LogFileName, err.Error()))
		}
		os.Exit(0)
	}

	if err := OpenLog(); err != nil {
		fatal(fmt.Sprintf("open log file %s: %s\n", LogFileName, err))
	}
	defer FlushAndCloseLog()

	s, err := Build()
	if err != nil {
		fatal(err.Error())
	}
	if err := Check(s); err != nil {
		fatal(err.Error())
	}
	if err = Simulate(s, true, 5); err != nil {
		fatal(err.Error())
	}

	pr("success")
}

// Make all the System components and wire them together.
// In time, command line flags will offer a choice of implementations.
func Build() (*System, error) {
	dbg("building...")
	s, err := MakeSystem()
	if err != nil {
		return nil, err
	}
	if err = Sequential(s); err != nil {
		return nil, err
	}
	return s, nil
}

// Components can't fully check themselves during Build() because they
// can't know if another AddInput() call might be coming, etc. This is
// called after build returns and calls Check() on all the components
// that registered themselves during Build().
func Check(s *System) error {
	dbg("checking...")
	var nError int = 0
	for _, cl := range s.state {
		dbg("clockable: %s", cl.Name())
		if err := cl.Check(); err != nil {
			nError++
			pr(err.Error())
		}
	}
	for _, co := range s.logic {
		dbg("combinational: %s", co.Name())
		if err := co.Check(); err != nil {
			nError++
			pr(err.Error())
		}
	}
	if nError > 0 {
		msg := "s" // Oh for a ternary ...
		if nError == 1 {
			msg = ""
		}
		return fmt.Errorf("%d error%s found in circuit", nError, msg)
	}
	return nil
}

// We have to be extremely careful not to introduce any ordering dependencies.
// On each cycle, we first Prepare() all the components which clears the next
// state variable for Clockables and also clears optional caching for logic.
// Then we Evaluate() all the Clockables, which prepares their nextStates and
// their clock enables, typically by calling to Evaluate() on logic components.
// Finally, we call PositiveEdge() on all the Clockables which transfers next
// state to visible state. It's critical that all computation is performed in
// Evaluate() only, after all components are prepared and before any are clocked.
// Any computations done in PositiveEdge() may accidentally read exposed state
// that has already been updated to its value for the following machine cycle.

var CycleCounter uint32

func Simulate(s *System, reset bool, nCycles uint32) error {
	dbg("simulating...")
	if (reset) {
		for _, cl := range s.state {
			cl.Reset()
		}
		CycleCounter = 0
	}
	for end := CycleCounter + nCycles ; CycleCounter < end ; CycleCounter++ {
		for _, co := range s.logic {
			co.Prepare()
		}
		for _, co := range s.state {
			co.Prepare()
		}
		for _, cl := range s.state {
			cl.Evaluate()
		}
		for _, cl := range s.state {
			cl.PositiveEdge()
		}
	}
	return nil
}
