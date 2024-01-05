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

// lexer.go - exported types: Token and Lexer.

import (
	"fmt"
	"io"
)

var lexer_debug = false // prints token stream to stdout

const SP = byte(' ')
const TAB = byte('\t')
const NL = byte('\n')

const COMMA = byte(',')
const COLON = byte(':')
const NEG = byte('-')

const DOT = byte('.')
const UNDERSCORE = byte('_')

const COMMENT = byte('#')

// N.B. The below is my preferred solution to Go's appalling lack of support
// for type-checked enumerations. Note that if e.g. lexerStateType is changed
// to be an int instead of a struct containing an int, then assignments to the
// lexerState are no longer type checked - the RHS can be any int.

// Lexer states. FYI: A label is a symbol followed by a colon. We recognize
// the trailing colon when we come to the end of the symbol characters and
// immediately transistion back to state stBetween, so no "stInLabel" state
// is required. Also, no white space need follow the colon.

type lexerStateType struct {
	s int
}

var stBetween lexerStateType = lexerStateType{0}
var stInError lexerStateType = lexerStateType{1}
var stInSymbol lexerStateType = lexerStateType{2}
var stInString lexerStateType = lexerStateType{3}
var stInNumber lexerStateType = lexerStateType{4}
var stInOperator lexerStateType = lexerStateType{5}
var stInComment lexerStateType = lexerStateType{6}
var stEnd lexerStateType = lexerStateType{7}

// Token kinds

type TokenKindType struct {
	k int
}

var TkError TokenKindType = TokenKindType{0}
var TkNewline TokenKindType = TokenKindType{1}
var TkSymbol TokenKindType = TokenKindType{2}
var TkLabel TokenKindType = TokenKindType{3}
var TkString TokenKindType = TokenKindType{4}
var TkNumber TokenKindType = TokenKindType{5}
var TkOperator TokenKindType = TokenKindType{6}
var TkEOF TokenKindType = TokenKindType{7}

var kindToString = []string{
	"TkError",
	"TkNewline",
	"TkSymbol",
	"TkLabel",
	"TkString",
	"TkNumber",
	"TkOperator",
	"TkEOF",
}

// =====
// Token
// =====

type Token struct {
	tokenText string
	tokenKind TokenKindType
}

func (t *Token) String() string {
	s := t.tokenText
	if s == "\n" {
		s = "\\n"
	}
	return fmt.Sprintf("{%s %s}", kindToString[t.tokenKind.k], s)
}

func (t *Token) Text() string {
	return t.tokenText
}

func (t *Token) Kind() TokenKindType {
	return t.tokenKind
}

var eofToken = Token{"EOF", TkEOF}   // const
var nlToken = Token{"\n", TkNewline} // const

// =====
// Lexer
// =====

type Lexer struct {
	reader PushbackByteReader
	lexerState lexerStateType
	path string 
	pbToken *Token
}

func MakeFileLexer(path string) (*Lexer, error) {
	pbr, err := NewFilePushbackByteReader(path)
	if err != nil {
		return nil, err
	}
	return &Lexer{reader: pbr, lexerState: stBetween, path: path}, nil
}

func MakeStringLexer(ident string, body string) (*Lexer, error) {
	pbr, err := NewStringPushbackByteReader(body)
	if err != nil {
		return nil, err
	}
	return &Lexer{reader: pbr, lexerState: stBetween, path: ident}, nil
}

func (lx *Lexer) Close() {
	lx.reader.Close()
}

// GetToken returns the next lexer token (or an EOF or error token).
//
// The language is all ASCII - no exceptions, not even in quoted strings. White space
// includes only space, tab, and newline. Newline is returned as a separate token so
// that the language may be at least partially line-oriented. The handling of control
// characters other than the defined whitespace characters is undefined.
//
// Tokens are:
//
// 1. Symbols. These are unquoted restricted character strings. The first character
// must be one of the "initial symbol characters" and the remaining characters must
// be "symbol characters" (neither set is a subset of the other). Symbols terminate
// at a "white space character" or at a "single character token" (see next).
//
// 2. Single-character tokens. The only single-character token is comma. It can act
// as delimiters for symbols. So foo,bar is accepted as is foo, bar and foo bar.
// Newlines are also returned as a separate token which the caller may choose to
// treat as whitespace or as a delimiter.
//
// 3. Quoted strings. These are surrounded by double quotes. Double quotes do not
// serve as single-character tokens for purposes of terminating a symbol, so a
// sequence like foo"bar" isn't legal. Newlines are never allowed in strings.
//
// 4. Numbers. These can be decimal numbers or hex numbers starting with 0x or 0X and
// containing the letters a-f in either case.
//
// EOF is not equivalent to whitespace; a token won't be recognized if it's terminated
// by end of file without a newline (or tab or space). The language doesn't even have
// constant expressions, so the small set of "operator" characters are more like
// punctuation than arithment operators. Comments ("# ...") are terminated by newlines
// and must be preceded by whitespace, which is usually desirable for readability
// anyway. When the lexer encounters an error, it is returned as token; the lexer then
// enters an error state and throws away characters until it sees a newline (or EOF).

func (lx *Lexer) GetToken() *Token {
	result := lx.internalGetToken()
	if lexer_debug {
		fmt.Printf("[ %s ]\n", result)
	}
	return result
}

func (lx *Lexer) internalGetToken() *Token {
	if lx.lexerState == stEnd {
		return &eofToken
	}
	if lx.pbToken != nil {
		result := lx.pbToken
		lx.pbToken = nil
		if lx.lexerState != stBetween {
			lx.lexerState = stInError
			result = &Token{"internal error: pbToken but not between tokens", TkError}
		}
		return result // leaving the state "between"
	}

	var accumulator []byte

	for b, err := lx.reader.ReadByte(); ; b, err = lx.reader.ReadByte() {
		// Preliminaries
		if err == io.EOF {
			lx.lexerState = stEnd
			return &eofToken
		}
		if err != nil {
			lx.lexerState = stInError
			return &Token{err.Error(), TkError}
		}
		if b >= 0x80 {
			lx.lexerState = stInError
			return &Token{fmt.Sprintf("non-ASCII character 0x%02x", b), TkError}
		}

		// Switch on lexer state. Within each case, handle all character types. The
		// "stBetween" state is the start state. It's similar to an "in white space"
		// state except for some subtleties: currently all operators (punctuation)
		// are single characters, so we can just return a token when we see one and
		// remain in the "stBetween" state for sequences like 7:4 that contain no
		// actual whitespace around the colon operator.

		switch lx.lexerState {
		case stInError, stInComment:
			if b == NL {
				lx.lexerState = stBetween
				return &nlToken
			}
		case stBetween:
			if len(accumulator) != 0 {
				panic(fmt.Sprintf("token accumulator not empty between tokens: %s\n", accumulator))
			}
			if b == NL {
				// Still between, but returned as a distinct token so that
				// caller may implement a line-oriented higher level syntax
				return &nlToken
			}
			if b == COMMENT {
				lx.lexerState = stInComment
			} else if isWhiteSpaceChar(b) {
				// move along, nothing to see here
			} else if isDigitChar(b) {
				accumulator = append(accumulator, b)
				lx.lexerState = stInNumber
			} else if isInitialSymbolChar(b) {
				accumulator = append(accumulator, b)
				lx.lexerState = stInSymbol
			} else if isQuoteChar(b) {
				// we do not capture the quotes in the result
				lx.lexerState = stInString
			} else if isOperatorChar(b) {
				lx.lexerState = stBetween
				// For now, at least, commas occurring between tokens are simply ignored -
				// they are white space. We might do something fancier later.
				if b != COMMA {
					return &Token{string(b), TkOperator}
				}
			} else {
				msg := fmt.Sprintf("character 0x%02x (%d) unexpected [1]", b, b)
				lx.lexerState = stInError
				return &Token{msg, TkError}
			}
		case stInSymbol:
			if len(accumulator) == 0 {
				panic("token accumulator empty in symbol")
			}
			if isWhiteSpaceChar(b) || isOperatorChar(b) {
				lx.lexerState = stBetween
				var result *Token
				if b == COLON {
					// Label definition, e.g. "myLabel:"
					// Again, here, we end in the BETWEEN state with
					// no intervening white space. This makes it ok
					// to write "myLabel:JMP myLabel" with no space
					// between the colon and the previously defined
					// assembler mnemonic.
					result = &Token{string(accumulator), TkLabel}
				} else {
					result = &Token{string(accumulator), TkSymbol}
					// Even for whitespace, we need to push it back
					// and process it next time we're called because
					// it might be a newline, which gets returned as
					// a separate token while still being white space.
					lx.reader.UnreadByte(b)
				}
				accumulator = nil
				return result
			} else if isSymbolChar(b) {
				accumulator = append(accumulator, b)
			} else {
				msg := fmt.Sprintf("character 0x%02x (%d) unexpected [2]", b, b)
				lx.lexerState = stInError
				return &Token{msg, TkError}
			}
		case stInString:
			if isQuoteChar(b) {
				// Changing directly to "between" here means a symbol or something
				// can come after a quoted string without any intervening white space.
				// Wrong/ugly, but not worth fixing. Also, the caller may separately
				// demand that e.g. builtin symbols be preceded by a newline and optional
				// whitespace, etc., so this may be reported as an error there.
				lx.lexerState = stBetween
				result := &Token{`"` + string(accumulator) + `"`, TkString}
				accumulator = nil
				return result
			} else if b == NL {
				// There is no escape convention
				lx.lexerState = stInError
				return &Token{"newline in string", TkError}
			} else {
				accumulator = append(accumulator, b)
			}
		case stInNumber:
			// We get into the number state when we see a digit 0-9. When in the number state,
			// we accumulate any digit, a-f, A-F, x, or X, i.e. we allow garbage sequences with
			// multiple x's, hex letters without a leading 0x, etc. Then at the end we apply the
			// validity tests and return error if the numeric string is garbage.
			if isDigitChar(b) || isHexLetter(b) || isX(b) {
				accumulator = append(accumulator, b)
			} else if isWhiteSpaceChar(b) || isOperatorChar(b) {
				var result *Token
				if !validNumber(accumulator) {
					result = &Token{fmt.Sprintf("invalid number %s", string(accumulator)), TkError}
					lx.lexerState = stInError
				} else {
					result = &Token{string(accumulator), TkNumber}
					lx.lexerState = stBetween
				}
				accumulator = nil
				lx.reader.UnreadByte(b)
				return result
			} else {
				msg := fmt.Sprintf("character 0x%02x (%d) unexpected in number", b, b)
				lx.lexerState = stInError
				return &Token{msg, TkError}
			}
			// That's it - no state called stInOperator since they are all single characters
		}
	}
}

// Unget a token, allowing one-character look ahead
func (lx *Lexer) unget(tk *Token) error {
	if lx.pbToken != nil {
		lx.lexerState = stInError
		return fmt.Errorf("internal error: too many token pushbacks")
	}
	if lx.lexerState != stBetween {
		lx.lexerState = stInError
		return fmt.Errorf("internal error: invalid token pushback")
	}
	lx.pbToken = tk
	return nil
}

func validNumber(num []byte) bool {
	isHex := false
	digitOffset := 0
	if len(num) > 2 && num[0] == byte('0') && isX(num[1]) {
		isHex = true
		digitOffset = 2
	}
	for i := digitOffset; i < len(num); i++ {
		switch { // no fallthrough in Go
		case isDigitChar(num[i]): // OK
		case isHex && isHexLetter(num[i]): // OK
		default:
			return false
		}
	}
	return true
}

func isWhiteSpaceChar(b byte) bool {
	return b == SP || b == TAB || b == NL
}

func isDigitChar(b byte) bool {
	return b >= '0' && b <= '9'
}

func isHexLetter(b byte) bool {
	switch {
	case b >= 'A' && b <= 'F':
		return true
	case b >= 'a' && b <= 'f':
		return true
	}
	return false
}

func isX(b byte) bool {
	return b == 'x' || b == 'X'
}

func isQuoteChar(b byte) bool {
	return b == '"' // || b == '`' future multiline string
}

func isOperatorChar(b byte) bool {
	return b == COMMA || b == COLON || b == NEG
}

// Dot is allowed only as the initial character
// of a symbol, where it means "builtin"
func isInitialSymbolChar(b byte) bool {
	switch {
	case b >= 'a' && b <= 'z':
		return true
	case b == '.' || b == '_':
		return true
	case b >= 'A' && b <= 'Z':
		return true
	}
	return false
}

func isSymbolChar(b byte) bool {
	switch {
	case b >= 'a' && b <= 'z':
		return true
	case b >= '0' && b <= '9':
		return true
	case b == '_':
		return true
	case b >= 'A' && b <= 'Z':
		return true
	}
	return false
}
