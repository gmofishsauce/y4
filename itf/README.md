# Instruction Test Framework (ITF)

This directory tries to test the assembler and disassembler by round-
tripping them. It contains some test programs with the y4a (YARC-4 assembler)
extension.

For each test program t1.y4a, t2, etc., the itf program (itf.go) first
assembles the test program, producing y4.out. It moves the generated binary
to a temporary working directory. The binary file is disassembled producing
a disassembly in the temporary directory, which is then assembled to produce
another y4.out file. The two assembler output binaries are then compared for
equality.

This approach removes all issues with minor textual differences such as number
bases between the original assembler source and the disassembled source.
