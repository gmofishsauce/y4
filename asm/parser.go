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
)

var ParserDebug = false

const (				// parser states index parserStateMap
	StError = iota	// error seen, seeking newline
	StStartLine		// at start of line
	StHaveLabel		// have a label, must see an op
	StHaveKey		// have a key, need 0 or more operands
	StNeedNewline	// have everything, must see newline
)

var stateToString []string = []string{
	"StError", "StStartLine", "StHaveLabel", "StHaveKey", "StNeedNewline",
}

type stateHandler func(ctx *parserContext, t *Token)

// We have one handler function for each parser state. The
// table is index by the parser states, above.
var parserFunctionMap []stateHandler = []stateHandler {
	doErrorState,
	doStartLineState,
	doHaveLabelState,
	doHaveOpState,
	doNeedLineEndState,
}

type parserContext struct { // bag o' context
	srcPath string
	srcLine int
	errorCount int
	instructions []MachineInstruction
	state int
	key string
	operands []string
	opindex int
	syms *SymbolTable
	dot uint16
	signature uint16
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

func parse(srcPath string) (*[]MachineInstruction, error) {
	lx, err := MakeFileLexer(srcPath)
	if err != nil {
		return &[]MachineInstruction{}, err
	}
	defer lx.Close()

	ctx := &parserContext{
		srcPath: srcPath, srcLine: 1,
		dot: 0, errorCount: 0,
		instructions: make([]MachineInstruction, 0, 32),
		state: StStartLine,
		syms: MakeSymbolTable(),
	}

	// Process one token per iteration. If we see an error,  enter
	// the error state and move on. Otherwise hand off to one of
	// a few state-specific handlers.
	for t := lx.GetToken(); t.Kind() != TkEOF; t = lx.GetToken() {
		if ParserDebug {
			dbg("parser state %s", stateToString[ctx.state])
		}
		if t.Kind() == TkError {
			report(ctx, t.Text())
			ctx.state = StError
			continue
		}

		// Handle one token in the current state
		parserFunctionMap[ctx.state](ctx, t)
	}

	// EOF seen - end of file processing
	if ctx.state != StStartLine {
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
	return &ctx.instructions, err
}

/* FIXME remove
var TkError TokenKindType = TokenKindType{0}
var TkNewline TokenKindType = TokenKindType{1}
var TkSymbol TokenKindType = TokenKindType{2}
var TkLabel TokenKindType = TokenKindType{3}
var TkString TokenKindType = TokenKindType{4}
var TkNumber TokenKindType = TokenKindType{5}
var TkOperator TokenKindType = TokenKindType{6}
var TkEOF TokenKindType = TokenKindType{7}
*/

// In error state. Ignore everything until newline.
func doErrorState(ctx *parserContext, t *Token) {
	if t.Kind() == TkNewline {
		ctx.state = StStartLine
	}
}

// Line start state. Handle labels and operation symbols. All TkError
// tokens are handled in the caller and don't make it here.
func doStartLineState(ctx *parserContext, t *Token) {
	switch t.Kind() {
	case TkNewline:
		ctx.srcLine++
	case TkLabel:
		if _, err := ctx.syms.DefineSymbol(t.Text(), ctx.dot); err != nil {
			report(ctx, err.Error())
		}
		ctx.state = StHaveLabel
	case TkSymbol:
		ctx.state = StHaveLabel
		doHaveLabelState(ctx, t)
	default:
		report(ctx, "unexpected: %s", t.String())	
	}
}

// Get a key symbol or issue an error
func doHaveLabelState(ctx *parserContext, t *Token) {
	switch t.Kind() {
	case TkSymbol:
		symValue, err := ctx.syms.Get(t.Text())
		if err != nil {
			report(ctx, "unexpected: %s", t.Text())
		} else {
			ctx.key = t.Text()
			ctx.state = StHaveKey
			ctx.opindex = 0
			ctx.operands = []string{}
			ctx.signature = symValue
		}
	case TkNewline:
		// I think in this case we'll enter the error state,
		// which will cause the entire following line to be
		// skipped in order to resynchronize. Since the parse
		// has failed and we're just making a best effort to
		// report additional errors from here on out, this
		// is not worth fixing.
		report(ctx, "short line")
	default:
		report(ctx, "unexpected: %s", t.Text())
	}
}

func doHaveOpState(ctx *parserContext, t *Token) {
	if ctx.opindex >= numOperands(ctx.signature) {
		ctx.state = StNeedNewline
		doNeedLineEndState(ctx, t)
	}

	switch t.Kind() {
	case TkSymbol, TkLabel:
		ctx.operands[ctx.opindex] = t.Text()
	case TkNumber:
	case TkOperator:
	default:
	}
}

func doNeedLineEndState(ctx *parserContext, t *Token) {
	ctx.state = StNeedNewline
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
