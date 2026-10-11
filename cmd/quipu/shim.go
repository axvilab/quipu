package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/axvilab/quipu/internal/fsx"
	"github.com/axvilab/quipu/internal/profile"
)

func runShim(home string, args []string) error {
	if len(args) == 0 {
		args = []string{"status"}
	}
	fs := flag.NewFlagSet("profile shim", flag.ExitOnError)
	rc := fs.Bool("rc", false, "also add or remove the PATH line in your shell's startup file")
	cmd := args[0]
	fs.Parse(args[1:])

	data, err := dataDir(home)
	if err != nil {
		return err
	}
	dir := profile.ShimDir(data)
	self, err := os.Executable()
	if err != nil {
		return err
	}
	rcFile, block := profile.RCFile(home, profile.LoginShell(), dir)

	switch cmd {
	case "status":
		switch {
		case profile.ShimActive(dir):
			fmt.Println("On: plain claude picks the profile by folder.")
		case profile.ShimInstalled(dir, self):
			fmt.Printf("Installed but not first on PATH. Add to %s:\n\n%s\n", rcFile, block)
		default:
			fmt.Println("Off. Turn it on with: quipu profile shim on --rc")
		}
		return nil
	case "on", "off":
		journal, err := fsx.OpenJournal(filepath.Join(data, "backups"))
		if err != nil {
			return err
		}
		if _, err := profile.SetShim(cmd == "on", *rc, home, data, self, journal); err != nil {
			return err
		}
		switch {
		case cmd == "off":
			fmt.Println("Off: claude runs as before.")
		case *rc:
			fmt.Printf("On. Open a new terminal (or run: source %s); plain claude then picks the profile by folder.\n", rcFile)
		default:
			fmt.Printf("Shim installed in %s. Put it first on PATH by adding to %s:\n\n%s\nor run again with --rc.\n", dir, rcFile, block)
		}
		return nil
	}
	return fmt.Errorf("unknown shim command %q (on, off or status)", cmd)
}
