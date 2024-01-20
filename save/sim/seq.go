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

func Sequential(s *System) error {
	var errors ErrorList

	m := MakeMux(s, "pc-input", 16)
	z := MakeZeroGenerator(s, "temp-ctl", 1)
	errors.appendIfNotNil(m.AddControl(z))

	z = MakeZeroGenerator(s, "reset-vec", 16)
	errors.appendIfNotNil(m.AddData(z, 0))
	errors.appendIfNotNil(m.AddData(z, 1))

	r := MakeRegister(s, "pc", 16, func() bool {return true})
	errors.appendIfNotNil(r.AddInput(m))
	if errors.Length() > 0 {
		return errors
	}
	return nil
}
