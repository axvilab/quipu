// Command quipu manages Claude Code profiles and the knowledge Claude accumulates.
//
// The same binary has two roles, chosen by the name it was invoked under:
//   - "quipu": the management CLI;
//   - "claude": the shim that asks which profile to use and then runs the real claude.
package main

import (
	"fmt"
	"os"
	"path/filepath"
)

var version = "dev"

func main() {
	if filepath.Base(os.Args[0]) == "claude" {
		fmt.Fprintln(os.Stderr, "quipu: shim mode is not implemented yet")
		os.Exit(1)
	}

	if len(os.Args) > 1 && (os.Args[1] == "version" || os.Args[1] == "--version") {
		fmt.Println("quipu", version)
		return
	}

	fmt.Println("quipu", version, "— nothing here yet, see docs/DESIGN.md")
}
