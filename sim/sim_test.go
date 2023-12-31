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
	"runtime"
	"testing"
)

func chop(s string) string {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == byte('/') {
			return s[i+1:]
		}
	}
	return s
}

func chk(t *testing.T, b bool) {
	if !b {
		pc, file, line, ok := runtime.Caller(1)
		details := runtime.FuncForPC(pc)
		where := "???"
		if ok && details != nil {
			where = details.Name()
		}
		t.Log("<= IGNORE. Failed at", where, chop(file), line)
		t.Fail()
	}
}

func TestBits1(t *testing.T) {
	chk(t, ZeroBits.toUint64() ==  0x10000000000000)
	chk(t, OneBits.toUint64() == 0x1000000000FFFF)
	chk(t, HighzBits.toUint64() == 0x10FFFF00000000)
	chk(t, UndefBits.toUint64() == 0x100000FFFF0000)
}
