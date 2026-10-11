package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/axvilab/quipu/internal/analytics"
	"github.com/axvilab/quipu/internal/claudecode"
	"github.com/axvilab/quipu/internal/collect"
)

func runMap(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("map", flag.ExitOnError)
	profile := fs.String("profile", "default", "Claude Code profile: default (~/.claude) or <name> (~/.claude-<name>)")
	all := fs.Bool("all", false, "list every on-demand item, not only the 10 heaviest")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: quipu map [flags] [project path, default: current directory]")
		fs.PrintDefaults()
	}
	fs.Parse(args)

	project := "."
	if fs.NArg() > 0 {
		project = fs.Arg(0)
	}
	project, err := filepath.Abs(project)
	if err != nil {
		return err
	}

	home, st, err := openStore()
	if err != nil {
		return err
	}
	defer st.Close()
	configDir := filepath.Join(home, ".claude")
	if *profile != "default" {
		configDir += "-" + *profile
	}

	items := claudecode.Instructions(home, configDir, project)
	var always, onDemand []row
	for _, it := range items {
		r := row{item: it, tokens: analytics.EstimateTokens(it.Text)}
		if it.Load == claudecode.Always {
			always = append(always, r)
		} else {
			onDemand = append(onDemand, r)
		}
	}

	fmt.Printf("%s  (profile %s)\n\n", shortPath(home, project, project), *profile)
	alwaysTotal := printGroup("Loaded in every session", always, home, project, 0)
	limit := 10
	if *all {
		limit = 0
	}
	printGroup("Loaded on demand", onDemand, home, project, limit)

	if _, err := collect.Run(ctx, home, st); err != nil {
		fmt.Fprintln(os.Stderr, "quipu: some transcripts could not be collected:", err)
	}
	w, err := analytics.SessionStart(ctx, st, analytics.Filter{
		Since:      time.Now().AddDate(0, 0, -30),
		ProjectDir: claudecode.ProjectDirName(project),
	})
	if err != nil {
		return err
	}
	fmt.Println("Measured first request of a session (last 30 days)")
	if w.Sessions == 0 {
		fmt.Println("  no sessions recorded for this project yet")
		return nil
	}
	fmt.Printf("  latest %s (%s), median %s over %d sessions, range %s–%s\n",
		analytics.Human(w.Latest), w.LatestAt.Local().Format(time.DateOnly), analytics.Human(w.Median), w.Sessions, analytics.Human(w.Min), analytics.Human(w.Max))
	fmt.Printf("  files above ≈ %s of it; the rest is Claude Code's system prompt, tools, MCP and the first message\n",
		analytics.Human(alwaysTotal))
	return nil
}

type row struct {
	item   claudecode.Item
	tokens int64
}

// printGroup prints items heaviest first, at most limit rows (0 = all), and returns the total.
func printGroup(title string, rows []row, home, project string, limit int) int64 {
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].tokens > rows[j].tokens })
	var total int64
	for _, r := range rows {
		total += r.tokens
	}
	fmt.Printf("%s: ≈ %s tokens, %d items\n", title, analytics.Human(total), len(rows))

	shown := rows
	if limit > 0 && len(rows) > limit {
		shown = rows[:limit]
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, r := range shown {
		fmt.Fprintf(w, "  %6s\t%s\t%s\t%s\n", analytics.Human(r.tokens), r.item.Scope, r.item.Kind, itemLabel(r.item, home, project))
	}
	w.Flush()
	if len(shown) < len(rows) {
		fmt.Printf("  … %d more, see --all\n", len(rows)-len(shown))
	}
	fmt.Println()
	return total
}

// itemLabel names an item: skills, agents and commands by name, files by a short path.
func itemLabel(it claudecode.Item, home, project string) string {
	switch it.Kind {
	case "skill", "agent", "command":
		return it.Name
	}
	if rel, err := filepath.Rel(project, it.Path); err == nil && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return shortPath(home, it.Path, it.Path)
}
