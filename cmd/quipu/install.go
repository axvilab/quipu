package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/axvilab/quipu/internal/autostart"
	"github.com/axvilab/quipu/internal/ui"
)

func runInstall(args []string, uninstall bool) error {
	name := "install"
	if uninstall {
		name = "uninstall"
	}
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	dryRun := fs.Bool("dry-run", false, "show what would be done and change nothing")
	noStart := fs.Bool("no-start", false, "write the files but do not run systemctl/launchctl")
	fs.Parse(args)

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	binary, err := os.Executable()
	if err != nil {
		return err
	}
	if resolved, err := filepath.EvalSymlinks(binary); err == nil {
		binary = resolved
	}
	dir, err := dataDir(home)
	if err != nil {
		return err
	}
	plan, err := autostart.For(runtime.GOOS, home, binary, xdgDir("XDG_CONFIG_HOME", home, ".config"), xdgDir("XDG_DATA_HOME", home, ".local/share"), ui.Icon())
	if err != nil {
		return err
	}
	manifest := filepath.Join(dir, "autostart.json")

	if *dryRun {
		var paths []string
		for p := range plan.Files {
			paths = append(paths, p)
		}
		sort.Strings(paths)
		verb, cmds := "write", plan.Start
		if uninstall {
			verb, cmds = "remove", plan.Stop
		}
		for _, p := range paths {
			fmt.Printf("%s %s\n", verb, p)
		}
		for _, c := range cmds {
			fmt.Printf("run   %s\n", strings.Join(c, " "))
		}
		return nil
	}

	if uninstall {
		if err := autostart.Uninstall(plan, manifest, *noStart); err != nil {
			return err
		}
		fmt.Println("quipu no longer starts at login. Collected data stays in", dir)
		return nil
	}
	if err := autostart.Install(plan, manifest, *noStart); err != nil {
		return err
	}
	fmt.Printf("quipu starts at login and collects transcripts in the background (%s daemon).\n", binary)
	if *noStart {
		fmt.Println("Not started now; to start it:", plan.ManualRun)
	}
	fmt.Println("Open it from the application menu or with `quipu ui`. If you move the binary, run `quipu install` again.")
	return nil
}

// profilesFile is quipu's own config: profiles and folder rules.
func profilesFile(home string) string {
	return filepath.Join(xdgDir("XDG_CONFIG_HOME", home, ".config"), "quipu", "profiles.json")
}

func xdgDir(env, home, fallback string) string {
	if v := os.Getenv(env); v != "" {
		return v
	}
	return filepath.Join(home, fallback)
}
