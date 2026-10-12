package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/axvilab/quipu/internal/advise"
	"github.com/axvilab/quipu/internal/analytics"
	"github.com/axvilab/quipu/internal/claudecode"
	"github.com/axvilab/quipu/internal/collect"
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

	in := advise.Input{Home: home, ConfigDir: configDir, Project: project, Items: claudecode.Instructions(home, configDir, project)}
	in.Projects, in.PluginUse, in.SkillUse, in.Sessions = knownProjects(ctx, home, configDir)
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
		fmt.Printf("Fixing these saves ≈ %s tokens per session.\n", analytics.Human(saves))
	}
	return nil
}

// knownProjects scans every project quipu has seen in the last year (collecting new transcripts
// first, as quipu cache does), so rules can compare the project with the others; it also reports
// which projects used each plugin and each skill in the last 30 days, and the sessions of that
// window.
func knownProjects(ctx context.Context, home, configDir string) (items map[string][]claudecode.Item, pluginUse, skillUse map[string][]string, sessions int64) {
	items = map[string][]claudecode.Item{}
	_, _, st, err := openData()
	if err != nil {
		return items, nil, nil, 0
	}
	defer st.Close()
	if _, err := collect.Run(ctx, home, st); err != nil {
		return items, nil, nil, 0
	}
	rows, err := analytics.ByProject(ctx, st, analytics.Filter{Since: time.Now().AddDate(0, 0, -365)})
	if err != nil {
		return items, nil, nil, 0
	}
	path := map[string]string{}
	for _, r := range rows {
		path[r.ProjectDir] = r.Path
		if fi, err := os.Stat(r.Path); err == nil && fi.IsDir() {
			items[r.Path] = claudecode.Instructions(home, configDir, r.Path)
		}
	}
	month := analytics.Filter{Since: time.Now().AddDate(0, 0, -30)}
	if t, err := analytics.Total(ctx, st, month); err == nil {
		sessions = t.Sessions
	}
	if uses, err := analytics.PluginUses(ctx, st, month); err == nil {
		pluginUse = analytics.Folders(uses, path)
	}
	if uses, err := analytics.SkillUses(ctx, st, month); err == nil {
		skillUse = analytics.Folders(uses, path)
	}
	return items, pluginUse, skillUse, sessions
}
