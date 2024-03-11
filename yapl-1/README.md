# YAPL-1: Yet Another Programming Language, version 1

Note: YAPL-0, both the language design and implementation, were abandoned.
YAPL-0 was a Forth-like language intended as a platform for experimenting
with call stack design. In retrospect this does not seem like the most
important concern.

## Overview

YAPL-1 is an attempt to make a minimal programming language. No potential
simplification is too minimal for YAPL-1, because the goal is write an *entire*
compiler, from source code input to assembler source code for an executable.

The assembler is contained in this Github project. The target computer is the
WUT-4, which exists as a non-public design document and a functional emulator.
The emulator is also found in this Github project. The compiler has absolutely
minimal system dependencies: the ability to get a byte of input, to put a byte
of output, and exit with an exit code. Other dependencies will be introduced
grudgingly.

YAPL-1 is envisioned as the first step in an evolution. The overreaching
goal of the evolution is self-host the YAPL language, that is, to write the
YAPL compiler in YAPL and run the compiler on the WUT-4. The assembler will
not be self-hosted for now. The Go language project has shown that the design
of an assembler intended for machine use in a compiler pipeline may be quite
different than the design of a traditional assembler intended for programmers,
and keeping the assembler on the host computer retains design flexibility.

## Constraints

The WUT-4 is a 16-bit computer with both byte and word memory addressing.
Each running program has 64kb of data space plus 64kw (16-bit words) of
program space. The WUT-4 is a traditional RISC with low code density, but
the most important factor is data space. Every aspect of the implementation
must be oriented toward efficient use of space. Popular modern techniques,
like loading the entire source program into memory and using pointers or
indexes as references, are not candidates for this implementation. Everything
must stream, and anything to be stored in memory must be designed with
attention to space reduction.

The WUT-4 kernel will eventually offer memory sharing with very lightweight
switching between process-like entities in a predefined group of processes. 
The compiler will eventually be implemented as such a predefined group,
enabling a lightweight multiple-pass structure. I hope this will allow
moderately sophisticated modern compiler techniques like a control-flow
graph and register allocation by linear scan. My belief is that dataflow
techniques (like SSA, sea of nodes, etc.) are probably out of reach due
to memory constraints, but we'll see when we get there.

## Language Design

This first stage of YAPL evolution is designed to trivialize the lexical
analysis and parsing ("front end") phases of the compilation. These phases
dominate discussions and undergraduate compiler courses, but are actually
the easy parts of a real compiler. Trivializing them allows focus on the
hard parts.

---

YAPL-1 is a procedural language in the style of BCPL or C. It's just
absurdly simplified.

### Lexical structure

Identifiers consist of a single lower-case letter. Identifiers are used
to name functions and variables.

Keywords and builtin functions consist of a single upper-case letter.

Numeric constants consist of a single digit 0..9. No operators may be
applied to numeric constants. There is no support for other number bases.

Whitespace consists of spaces, tabs, and newlines. Whitespace is required
where other rules would be broken without it, e.g. between a keyword and
a variable name. Whitespace is optional elsewhere.

Comments may be introduced with the character # (hash). Comments are
terminated by a newline character. This is the only place in the language
where newline is distinguished from the other whitespace characters.

Semicolon ; is used as a statement separator. Its use is conventional.
The details are defined in the syntactic structure below.

The following additional characters are recognized and given meanings
described later: { } = +

### Syntactic structure

A program consists of a sequence of declarations. There are two types of
declarations: variables and functions.

Variable declarations consist of the keyword V, a variable name, and
an optional constant assignment.

A constant assignment consists of = followed by a numeric constant.

Function declarations consist of the keyword F, an identifier, and
the function body.

The function body consists of an opening curly brace, a sequence of
0 or more statements, and a closing curly brace.


serves as the function name, 
The characters { and } (curly braces) are used to create blocks of
simple statements.

### Semantic structure

All variables have type unsigned 8-bit value, the equivalent of uint8
in Golang.

