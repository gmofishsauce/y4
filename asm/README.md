# Customasm based assembler

This is an assembler for the y4 instruction set. The y4 is a 16-bit RISC designed for
pipelining. This assembler uses an open-source tool called customasm that reads an assembly
"meta language" to create an assembler for any instruction set. The metalanguage for y4 is
contained in the script `asm` in this directory. It's prepended to the y4 assembler source
on every run.

## Install

It's necessary to install customasm.

On Windows you can [download a binary](https://github.com/hlorenzi/customasm/releases).
For Mac, you need to install the Rust compiler and `cargo`. Then `cargo install customasm`
which will compile the source code and install the binary in `~/.cargo/bin`.

Then you'll need to add `~/.cargo/bin` to your PATH and you may need e.g. `hash -r`.

The binary is called `customasm`, but you should not need to run it directly except to
verify that it's present.

## Usage

The script `asm` in this directory is the y4 assembler. It embeds the rules and runs
customasm. You should never need to run customasm directly, nor should you need to
\#include any "rules" as described in the documentation for customasm--the script
takes care of all that.

## Source

Source files are named *.y4a (YARC-4 assembler).

## Output

The binary result file is written to y4.out.

## TODO

Create a disassembler. Create a few tests that assemble source code, run the disassembler,
and verify that the results match the original source code.