# YAPL - Yet Another Programming Language

This is the home of the eventual YAPL compiler.

## YAPL-0

“What’s the simplest thing that could possibly work?” - Ward Cunningham

YAPL-0 has three keywords: `func`, `def`, and `asm`. Each defines a block,
which is enclosed in curly braces. `Func` and `def` blocks may contain `asm`
blocks. No other containment is allowed: `asm` blocks may not contain
blocks, and neither `func` nor `def` blocks may contain nested `func` or
`def` blocks. Both `func` and `def` may contain sequential `asm` blocks,
but the result is exactly the same as a single larger block containing
their combined content with order preserved.

The `asm` block(s) within a `def` block may contain customasm pseudos (any
customasm statement starting with a # sign), even #include. Executable
code is not allowed in a `def` block. The asm(s) within a `func` block may
contain executable code (WUT-4 assembly language) but may not contain
pseudos. The effect is that defs are used to create defined symbols and
top-level declarations while funcs are used to create code.

Defs themselves cause nothing to be generated in the resulting binary;
they simply enclose an `asm` holding customasm pseudos. Funcs, however,
cause a very simple call and return sequence to occur. First, funcs have
*names*. A *name* is an identifier in the C/C++/Golang style. The name
follows the keyword: `func name { … }`. Second, functions may be invoked
from within the block of a function (but not from within an `asm`) by
mentioning their name. This is the intent:

```
func firstThing {
	asm {
		… stuff …
	}
}

func secondThing {
	asm {
		… stuff …
	}
	asm {
		; call the function firstThing
	}
	firstThing
	asm {
		… more stuff …
	}
	asm {
		; call it again
	}
	firstThing
}
```

The func statement for a function must precede its first use. So
recursion is allowed, but forward references are not.

At the closing curly brace of a function block, the compiler emits a
single RTL instruction. This loads the link register into the program
counter. The assembler code in the function is responsible for ensuring
this will behave as intended, e.g. by saving the link register on a
stack and restoring it before the closing curly brace. At the call site,
the compiler generates a single JSR instruction. This jumps to the label
representing the named function and stores the return address in the
link register. The result is that this code “works”:

```
func callee { }

func caller {
	callee
}
```

In YAPL-0, there are no statement terminators or separators, but
newlines are significant. Every assembler statement must be on a
separate line. Every function call must be on a separate line. Every
keyword must be on a separate line. Keywords must be together with their
opening curly brace, optionally preceded and followed by white space.
Curly braces must be the last non-whitespace characters on their
respective lines. Closing curly braces must be alone on a line.

That is the entire language. There is no comment syntax aside creating
a `def` or `asm` block containing a semicolon comment that will be
accepted by the assembler.

## Implementation notes

In the initial implementation, the following rules apply:

1. The curly brace that begins a block must appear on the same
line with its keyword.

1. The curly brace that ends a block must appear alone on a line,
preceded only by white space (spaces or tabs).

1. Curly braces may not appear as the first character of a line
within an `asm` block, even if allowed by the assembler syntax.

These rules make it unnecessary for the lexical analyzer to read
embedded assembler, aside from the scan for closing curly braces.
