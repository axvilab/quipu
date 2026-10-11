package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/axvilab/quipu/internal/advise"
	"github.com/axvilab/quipu/internal/claudecode"
)

func runAdvise(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("advise", flag.ExitOnError)
	profile := fs.String("profile", "default", "Claude Code profile: default (~/.claude) or <name> (~/.claude-<name>)")
	disable := fs.String("disable", "", "comma-separated rule IDs to skip")
	list := fs.Bool("rules", false, "list the rules and exit")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: quipu advise [flags] [project path, default: current directory]")
		fs.PrintDefaults()
	}
	fs.Parse(args)

	if *list {
		for _, r := range advise.Rules {
			fmt.Printf("  %-18s %s\n", r.ID, r.Summary)
		}
		return nil
	}

	project := "."
	if fs.NArg() > 0 {
		project = fs.Arg(0)
	}
	project, err := filepath.Abs(project)
	if err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	configDir := filepath.Join(home, ".claude")
	if *profile != "default" {
		configDir += "-" + *profile
	}

	disabled := map[string]bool{}
	for _, id := range strings.Split(*disable, ",") {
		if id = strings.TrimSpace(id); id != "" {
			disabled[id] = true
		}
	}

	in := advise.Input{Home: home, Project: project, Items: claudecode.Instructions(home, configDir, project)}
	findings := advise.Run(in, disabled)
	fmt.Printf("%s  (profile %s)\n\n", shortPath(home, project, project), *profile)
	if len(findings) == 0 {
		fmt.Println("Nothing to improve by the current rules (see quipu advise --rules).")
		return nil
	}
	var saves int64
	for i, f := range findings {
		fmt.Printf("%d. %s  [%s]\n", i+1, f.Title, f.Rule)
		fmt.Printf("   %s\n", f.Detail)
		if f.Path != "" {
			fmt.Printf("   file: %s\n", shortPath(home, f.Path, f.Path))
		}
		if f.Diff != "" {
			for _, line := range strings.Split(f.Diff, "\n") {
				fmt.Printf("     %s\n", line)
			}
		}
		fmt.Println()
		saves += f.Saves
	}
	if saves > 0 {
		fmt.Printf("Fixing these saves ≈ %s tokens per session.\n", human(saves))
	}
	return nil
}
