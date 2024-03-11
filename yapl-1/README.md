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
the function body. The identifier defines the function's name.

The function body consists of a block.

A block consists of an opening curly brace, a sequence of 0 or more
statements, and a closing curly brace. Variables may not be defined
within a block.

Statements consist of expressions, function calls, and conditional
statements.

An expression begins with an identifier and the character = It may
be followed by either a term and a semicolon or by a term, the character
+, another term, and a semicolon.

A term is a variable name or numeric constant.

A function call is the identifier of the known function followed by
a semicolon. A function becomes known when its declaration is seen,
so functions may be recursive.

A conditional statement consists of a conditional expression followed
by a block followed by the keyword E followed by another block. The
E and the second block are not optional.

A conditional expression the keyword I followed by two terms separated 
by whitespace with no ; or other punctuation.

Every program should define a function named m. Execution begins at
this function.

The builtin variables W, X, Y, and Z may be assigned. These values are
displayed by the emulator when the program exits.

Execution of the builtin Q causes the program to quit. In the emulator,
this results in a state dump which displays W, X, Y, and Z among
other values.

### Semantic structure of YAPL-1

All variables have type unsigned 8-bit value, the equivalent of uint8
in Golang. Variables not initialized by the program are automatically
initialized to 0.

All identifiers must be defined before use in source code textual order.

### Example of YAPL-1

```
    # Compute the first few Fibonacci numbers

    V a = 0 ;     # variable "a" is fib(0)
    V b = 1 ;     # fib(1)
    V r     ;     # variable "r" result
    V m = 8 ;     # variable "m" limit (fib(6))

    F m {              # function "m"
        r = a + b ;
        I r m {        # if r == m
            W = a ;    # write some values to display variables
            X = b ;
            Y = r ;
            Z = m ;
            Q          # quit to OS
        } E {          # else
            a = b      # shift down
            b = r
            m          # recursively call m
        }
    }
```
