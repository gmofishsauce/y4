# Customasm based assembler

This is an assembler for the y4 ISA, a 16-bit RISC designed for pipelining.

## Install

On Windows you can (download a binary)[https://github.com/hlorenzi/customasm/releases].
For Mac, you need to install the Rust compiler and `cargo`. Then `cargo install customasm`
which will compile the source code and install the binary in `~/.cargo/bin`.

Then you'll need to add `~/.cargo/bin` to your PATH and you may need e.g. `hash -r`.

The binary is called `customasm`, but you should not need to run it directly except to
verify that it's present.

## Usage

The script `asm` in this directory is the assembler. It embeds the rules and runs customasm.
so you should never need to run customasm directly, nor should you need to #include any
rules as described in the documentation for customasm.

## Output

The binary result file is written to y4.bin.
