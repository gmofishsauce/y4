/*
Copyright © 2022 Jeff Berkowitz (pdxjjb@gmail.com)

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
	"strings"
	"testing"
)

func check(t *testing.T, a1 any, a2 any) {
	if a1 != a2 {
		t.Errorf("%[1]v (a %[1]T) != %[2]v (a %[2]T)", a1, a2)
	}
}

func TestLexer1(t *testing.T) {
	data := ".symbol\n"
	lx, err := MakeStringLexer(t.Name(), data)
	check(t, err, nil)
	tk := lx.GetToken()
	check(t, TkSymbol, tk.Kind())
	check(t, data[:len(data)-1], tk.Text())
}

func TestLexer2(t *testing.T) {
	data := ".sym\"bol\n"
	lx, err := MakeStringLexer(t.Name(), data)
	check(t, err, nil)
	tk := lx.GetToken()
	check(t, TkError, tk.Kind())
	check(t, "character 0x22 (34) unexpected [2]", tk.Text())
}

func TestLexer3(t *testing.T) {
	data := ".aSymbol \"and a string\"\n"
	lx, err := MakeStringLexer(t.Name(), data)
	check(t, err, nil)
	tk := lx.GetToken()
	check(t, TkSymbol, tk.Kind())
	check(t, ".aSymbol", tk.Text())
	tk = lx.GetToken()
	check(t, TkString, tk.Kind())
	check(t, `"and a string"`, tk.Text())
}

func TestLexer4(t *testing.T) {
	data := "# .symbol\n"
	lx, err := MakeStringLexer(t.Name(), data)
	check(t, err, nil)
	tk := lx.GetToken()
	check(t, TkNewline, tk.Kind())
}

func TestLexer5(t *testing.T) {
	data := "10\n0x10\n0X3F\n"
	lx, err := MakeStringLexer(t.Name(), data)
	check(t, err, nil)
	tk := lx.GetToken()
	check(t, TkNumber, tk.Kind())
	check(t, "10", tk.Text())
	tk = lx.GetToken()
	check(t, TkNewline, tk.Kind())

	tk = lx.GetToken()
	check(t, TkNumber, tk.Kind())
	check(t, "0x10", tk.Text())
	tk = lx.GetToken()
	check(t, TkNewline, tk.Kind())

	tk = lx.GetToken()
	check(t, TkNumber, tk.Kind())
	check(t, "0X3F", tk.Text())
	tk = lx.GetToken()
	check(t, TkNewline, tk.Kind())
}

func TestLexer6(t *testing.T) {
	data := "1x0\n0xxxx10\n3F\n"
	lx, err := MakeStringLexer(t.Name(), data)
	check(t, err, nil)
	tk := lx.GetToken()
	check(t, TkError, tk.Kind())
	tk = lx.GetToken()
	check(t, TkNewline, tk.Kind())

	tk = lx.GetToken()
	check(t, TkError, tk.Kind())
	tk = lx.GetToken()
	check(t, TkNewline, tk.Kind())

	tk = lx.GetToken()
	check(t, TkError, tk.Kind())
	tk = lx.GetToken()
	check(t, TkNewline, tk.Kind())
}

var t7data string = `
		lw 1,0,count	# load reg1 with 5 (uses symbolic address)
		lw 2,1,2		# load reg2 with -1 (uses numeric address)
start:	add 1,1,2		# decrement reg1 -- could have been addi 1,1,-1
		beq 0,1,1		# goto end of program when reg1==0
		beq 0,0,start	# go back to the beginning of the loop
		done: halt		# end of program
		count: .fill 5
		neg1: .fill -1
		startAddr: .fill start # will contain the address of start (2)
`

var t7dataAsString []string = []string{
"{TkNewline \\n}",
"{TkSymbol lw}",
"{TkNumber 1}",
"{TkNumber 0}",
"{TkSymbol count}",
"{TkNewline \\n}",
"{TkSymbol lw}",
"{TkNumber 2}",
"{TkNumber 1}",
"{TkNumber 2}",
"{TkNewline \\n}",
"{TkLabel start}",
"{TkSymbol add}",
"{TkNumber 1}",
"{TkNumber 1}",
"{TkNumber 2}",
"{TkNewline \\n}",
"{TkSymbol beq}",
"{TkNumber 0}",
"{TkNumber 1}",
"{TkNumber 1}",
"{TkNewline \\n}",
"{TkSymbol beq}",
"{TkNumber 0}",
"{TkNumber 0}",
"{TkSymbol start}",
"{TkNewline \\n}",
"{TkLabel done}",
"{TkSymbol halt}",
"{TkNewline \\n}",
"{TkLabel count}",
"{TkSymbol .fill}",
"{TkNumber 5}",
"{TkNewline \\n}",
"{TkLabel neg1}",
"{TkSymbol .fill}",
"{TkOperator -}",
"{TkNumber 1}",
"{TkNewline \\n}",
"{TkLabel startAddr}",
"{TkSymbol .fill}",
"{TkSymbol start}",
"{TkNewline \\n}",
"{TkEOF EOF}",
}

func TestLexer7(t *testing.T) {
    lx, err := MakeStringLexer(t.Name(), t7data)
    check(t, err, nil)
	var i int
	for token := lx.GetToken(); token.Kind() != TkEOF; token = lx.GetToken() {
		s := strings.ReplaceAll(token.String(), "\n", "\\n")
		check(t, s, t7dataAsString[i])
		i++
	}
}
