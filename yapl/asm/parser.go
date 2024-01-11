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
	"fmt"
	"os"
	"strconv"
)

var ParserDebug = false

const (              // parser states index parserStateMap
	StError = iota   // error seen, seeking newline
	StBetweenLines   // at start of line
	StNeedKey        // have a label if any, need a key
	StNeedExpression // have a key, need operand
	StNeedNewline    // have everything, must see newline
)

var stateToString []string = []string{
	"StError", "StBetweenLines", "StNeedKey", "StNeedExpression", "StNeedNewline",
}

type stateHandler func(ctx *parserContext, t *Token)(consumed bool)

// We have one handler function for each parser state. The
// table is index by the parser states, above.
var parserFunctionMap []stateHandler = []stateHandler {
	doError,
	doBetweenLines,
	doNeedKey,
	doNeedExpression,
	doNeedNewline,
}

type parserContext struct { // bag o' context
	srcPath string
	instructions []MachineInstruction
	symtab *SymbolTable
	state int
	srcLine int
	errorCount int
	dot uint16
	signature uint16
	opindex uint16
	numoperands uint16
	positive bool
}

// Parser
//
// I think the assembly language is a regular language. There's nothing
// that needs to balance. The "expression parser" only needs to handle
// negation as a single unary operator. If want to change this, hand off
// to a Pratt parser for expressions.
//
// If an error occurs, don't create a struct for the error line or for
// any future lines on this run, but continue processing to detect
// additional errors. FIXME TODO

func Parse(srcPath string) (*SymbolTable, *[]MachineInstruction, error) {
	lx, err := MakeFileLexer(srcPath)
	if err != nil {
		return nil, nil, err
	}
	defer lx.Close()

	ctx := &parserContext{
		srcPath: srcPath, srcLine: 1,
		dot: 0, errorCount: 0,
		instructions: make([]MachineInstruction, 0, 32),
		state: StBetweenLines,
		symtab: MakeSymbolTable(),
	}

	// Process one token per iteration. If we see an error token from
	// the lexer, enter the error state and move on. Otherwise hand off
	// to one of a few state-specific handlers.
	for t := lx.GetToken(); t.Kind() != TkEOF; t = lx.GetToken() {
		if ParserDebug {
			dbg("parser state %s token %s", stateToString[ctx.state], t)
		}
		if t.Kind() == TkError {
			report(ctx, t.Text())
			ctx.state = StError
			continue
		}

		consume := parserFunctionMap[ctx.state](ctx, t)
		if !consume {
			lx.Unget(t)
		}

		if t.Kind() == TkNewline {
			ctx.srcLine++
		}
	}

	// EOF seen - end of file processing
	if ctx.state != StBetweenLines {
		// trailing newline triggers processing,
		// so any source file that ends mid-line
		// is guaranteed to have problems.
		report(ctx, "unexpected EOF")
	}
	err = nil
	if ctx.errorCount != 0 {
		s := "s"
		if ctx.errorCount == 1 {
			s = ""
		}
		err = fmt.Errorf("%d error%s", ctx.errorCount, s)
	}

	if ParserDebug {
		ctx.symtab.dump()
	}
	return ctx.symtab, &ctx.instructions, err
}

// Parser state machine functions. These functions are never passed error
// tokens as their argument; these are handled by the caller. These functions
// don't have to count source lines; this is handled by the caller.

// In error state. Ignore everything until newline.
func doError(ctx *parserContext, t *Token) bool {
	if t.Kind() == TkNewline {
		ctx.state = StBetweenLines
	}
	return true
}

// Line start state. Handle labels and operation symbols. All TkError
// tokens are handled in the caller and don't make it here.
func doBetweenLines(ctx *parserContext, t *Token) bool {
	consume := true
	switch t.Kind() {
	case TkLabel:
		if _, err := ctx.symtab.Define(t.Text(), ctx.dot); err != nil {
			report(ctx, err.Error())
		}
		ctx.state = StNeedKey
	case TkSymbol:
		ctx.state = StNeedKey
		consume = false
	case TkNewline:
		// nothing happens...
	default:
		report(ctx, "unexpected: %s", t.String())	
	}
	return consume
}

// Get a key symbol or issue an error. If the key symbol has operands,
// enter the NeedExpression state; otherwise, the NeedNewline state.
func doNeedKey(ctx *parserContext, t *Token) bool {
	switch t.Kind() {
	case TkSymbol:
		sig, index, err := ctx.symtab.Get(t.Text())
		if err != nil {
			report(ctx, "key unknown: %s", t.Text())
			break
		}
		ctx.instructions = append(ctx.instructions, MachineInstruction{})
		ctx.instructions[ctx.dot].parts[Key] = index
		ctx.signature = sig
		ctx.numoperands = numOperands(ctx.signature)
		// These must be updated per operand:
		ctx.opindex = Ra
		ctx.positive = true
		if ctx.numoperands > 0 {
			ctx.state = StNeedExpression
		} else {
			ctx.state = StNeedNewline
		}
	default:
		report(ctx, "unexpected: %s", t.Text())
	}
	return true
}

// This is our somewhat silly expression parser. Numbers and labels are
// Values. An expression is zero or more optional minus signs followed
// by a Value. There's also a string type, which has a signature code and
// is used with exactly one purpose-built keyword.
func doNeedExpression(ctx *parserContext, t *Token) bool {
	switch t.Kind() {
	case TkSymbol:
		index, err := ctx.symtab.Use(t.Text())
		if err != nil {
			report(ctx, "internal error: %s", err.Error())
			break
		}
		if !ctx.positive {
			ctx.symtab.Negate(index)
		}
		ctx.instructions[ctx.dot].parts[ctx.opindex] = index
		// These must be updated per operand:
		ctx.positive = true
		ctx.opindex++
		if ctx.opindex > ctx.numoperands {
			ctx.state = StNeedNewline
		}
	case TkNumber:
		value, err := strconv.ParseInt(t.Text(), 0, 0)
		if err != nil {
			report(ctx, "internal error: ParseInt(): %s", err.Error())
			break
		}
		if !ctx.positive {
			value = -value
		}
		// The largest value of a number that can appear in a machine
		// instruction is 10 bits (0x3FF). So whether "signed" or
		// "unsigned", it fits in the uint16 with space for the upper
		// flag bit that distinguishes symbol table entries from values.
		var immed uint16 = uint16(value&0x3FF)
		ctx.instructions[ctx.dot].parts[ctx.opindex] = IsValue | immed
		// These must be updated per operand:
		ctx.positive = true
		ctx.opindex++
		if ctx.opindex > ctx.numoperands {
			ctx.state = StNeedNewline
		}
	case TkOperator:
		if t.Text() == "-" {
			ctx.positive = !ctx.positive
		} else {
			report(ctx, "unexpected: %s", t.Text())
		}
	default:
		report(ctx, "unexpected: %s", t.Text())
	}
	return true
}

func doNeedNewline(ctx *parserContext, t *Token) bool {
	if t.Kind() != TkNewline {
		report(ctx, "unexpected at end of line: %s", t.Text())
	}

    ctx.state = StBetweenLines
    ctx.dot++
    ctx.signature = 0
    ctx.opindex = 0
    ctx.numoperands = 0
    ctx.positive = true
	return true
}

// This function prints an error, counts the error and then changes
// the state machine to the error state. It needs a better name.
func report(ctx *parserContext, msg string, args ...any) {
	actuals := []any{ctx.srcPath, ctx.srcLine}
	for _, a := range args {
		actuals = append(actuals, a)
	}
	fmt.Fprintf(os.Stderr, "error: %s, line %d: "+msg+"\n", actuals...)
	ctx.state = StError
	ctx.errorCount++
}
