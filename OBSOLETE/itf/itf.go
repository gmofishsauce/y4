/*
Copyright © 2022 Jeff Berkowitz (pdxjjb@gmail.com)

This program is free software: you can redistribute it and/or modify it
under the terms of the GNU Affero General Public License as published
by the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful, but
WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU
Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public
License along with this program. If not, see
<http://www.gnu.org/licenses/>.
*/
package main

import (
	"fmt"
	"flag"
	"io"
	"os"
	"os/exec"
	"path"
	"strings"
)

var dflag = flag.Bool("d", false, "enable debugging")

// Round trip test program for assembler and disassembler

func main() {
	var err error

	flag.Parse()
	args := flag.Args()
	if len(args) != 1 {
		usage()
	}
	asmPath := args[0]

	// TODO Should check that asmPath is a readable plain file
	workDir := makeTmpDir(asmPath)
	if err = os.RemoveAll(workDir) ; err != nil {
		fatal("removing working directory: " + err.Error())
	}
	if err = os.Mkdir(workDir, 0750) ; err != nil {
		fatal("creating working directory: " + err.Error())
	}
	pr(fmt.Sprintf("testing %s in %s...", asmPath, workDir))

	binPath := path.Join(workDir, "y4.out")
	if err = runAssembler(asmPath, binPath); err != nil {
		fatal(fmt.Sprintf("asm: %s: %s", asmPath, err.Error()))
	}
	pr(fmt.Sprintf("assembled %s to %s", asmPath, binPath))

	disassembledSourcePath := path.Join(workDir, "y4.dis")
	err = runDisassembler(binPath, disassembledSourcePath)
	if err != nil {
		fatal(fmt.Sprintf("dis: %s: %s", binPath, err.Error()))
	}
	pr(fmt.Sprintf("disassembled %s to %s", binPath, disassembledSourcePath))

	reassembledBinPath := path.Join(workDir, "y4.out2")
	err = runAssembler(disassembledSourcePath, reassembledBinPath)
	if err != nil {
		fatal(fmt.Sprintf("reassemble: %s", err.Error()))
	}
	pr(fmt.Sprintf("reassembled %s to %s", disassembledSourcePath, reassembledBinPath))

	err = runCompare(binPath, reassembledBinPath)
	if err != nil {
		fatal(fmt.Sprintf("compare: %s", err.Error()))
	}

	pr("passed")
}

const Assembler string = "../asm/asm"

// The assembler writes a binary file, so it accepts a -o output command
// line option that must precede the source file path(s).
func runAssembler(sourcePath string, targetPath string) error { 
	cmd := exec.Command(Assembler, "-o", targetPath, sourcePath)
	pr("running: " + cmd.String())
	output, err := cmd.CombinedOutput()
	pr(string(output))
	return err
}

const Disassembler string = "../dis/dis"

// The disassembler reads a binary and writes text, so it always writes
// to standard output. Here we have to redirect the output.
func runDisassembler(sourcePath string, targetPath string) error {
	cmd := exec.Command(Disassembler, "-q", sourcePath)
	outfile, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = outfile
	stderr, err := cmd.StderrPipe()
	pr("running: " + cmd.String() + " > " + targetPath)
	if err := cmd.Start(); err != nil {
		return err
	}
	slurp, _ := io.ReadAll(stderr)
	pr(string(slurp))
	return cmd.Wait()
}

const Comparer string = "cmp"

func runCompare(origBinPath string, reassembledBinPath string) error {
    cmd := exec.Command(Comparer, origBinPath, reassembledBinPath)
    pr("running: " + cmd.String())
    output, err := cmd.CombinedOutput()
    pr(string(output))
    return err
}

func makeTmpDir(asmPath string) string {
    base := path.Base(asmPath)
    ext := path.Ext(asmPath)
    name := strings.ReplaceAll(base, ext, "")
	return "./_Test_" + name
}

func usage() {
	pr("Usage: itf [options] assembler-source\nOptions:")
	flag.PrintDefaults()
	os.Exit(1)
}

