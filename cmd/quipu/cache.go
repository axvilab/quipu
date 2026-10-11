package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/axvilab/quipu/internal/analytics"
	"github.com/axvilab/quipu/internal/claudecode"
	"github.com/axvilab/quipu/internal/collect"
)

func runCache(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("cache", flag.ExitOnError)
	days := fs.Int("days", 30, "report the last N days")
	daily := fs.Bool("daily", false, "one row per day instead of per project")
	project := fs.String("project", "", "only this project: a path, or the name of its folder")
	fs.Parse(args)

	home, st, err := openStore()
	if err != nil {
		return err
	}
	defer st.Close()

	// Collect first so the report includes the sessions of the last minutes.
	if _, err := collect.Run(ctx, home, st); err != nil {
		fmt.Fprintln(os.Stderr, "quipu: some transcripts could not be collected:", err)
	}

	f := analytics.Filter{Since: time.Now().AddDate(0, 0, -*days)}
	projects, err := analytics.ByProject(ctx, st, f)
	if err != nil {
		return err
	}
	if *project != "" {
		dir, ok := resolveProject(*project, projects)
		if !ok {
			return fmt.Errorf("no data for project %q in the last %d days", *project, *days)
		}
		f.ProjectDir = dir
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.AlignRight)
	if *daily || f.ProjectDir != "" {
		rows, err := analytics.ByDay(ctx, st, f)
		if err != nil {
			return err
		}
		fmt.Fprintln(w, "DAY\tSESSIONS\tREQUESTS\tPROMPT\tCACHE HIT\tCACHE WRITE\tOUTPUT\tMISSES\t")
		for _, r := range rows {
			fmt.Fprintf(w, "%s\t%s\n", r.Day.Format(time.DateOnly), totalsCells(r.Totals))
		}
	} else {
		fmt.Fprintln(w, "PROJECT\tSESSIONS\tREQUESTS\tPROMPT\tCACHE HIT\tCACHE WRITE\tOUTPUT\tMISSES\t")
		var all analytics.Totals
		for _, r := range projects {
			fmt.Fprintf(w, "%s\t%s\n", shortPath(home, r.Path, r.ProjectDir), totalsCells(r.Totals))
			add(&all, r.Totals)
		}
		fmt.Fprintf(w, "TOTAL\t%s\n", totalsCells(all))
	}
	if err := w.Flush(); err != nil {
		return err
	}

	misses, err := analytics.Misses(ctx, st, f)
	if err != nil {
		return err
	}
	if len(misses) > 0 {
		fmt.Println("\nCache misses reported by the API:")
		mw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		for _, m := range misses {
			fmt.Fprintf(mw, "  %s\t%d\t%s\n", m.Reason, m.Count, missHint[m.Reason])
		}
		mw.Flush()
	}
	return nil
}

var missHint = map[string]string{
	"previous_message_not_found": "cache expired: a long pause inside a session",
	"messages_changed":           "earlier conversation content changed",
	"system_changed":             "system prompt changed: CLAUDE.md, settings, output style",
	"tools_changed":              "tool list changed: MCP server or plugin toggled",
	"model_changed":              "model switched mid-session",
}

func totalsCells(t analytics.Totals) string {
	return fmt.Sprintf("%d\t%d\t%s\t%.1f%%\t%s\t%s\t%d\t",
		t.Sessions, t.Requests, human(t.Prompt()), 100*t.HitRate(),
		human(t.CacheCreation), human(t.Output), t.Misses)
}

func add(sum *analytics.Totals, t analytics.Totals) {
	sum.Sessions += t.Sessions
	sum.Requests += t.Requests
	sum.Input += t.Input
	sum.CacheRead += t.CacheRead
	sum.CacheCreation += t.CacheCreation
	sum.Output += t.Output
	sum.Misses += t.Misses
}

// resolveProject finds a project by path (encoded the way Claude Code names its folder) or by
// the base name of its working directory.
func resolveProject(arg string, projects []analytics.ProjectRow) (string, bool) {
	if strings.ContainsAny(arg, "/.") {
		if abs, err := filepath.Abs(arg); err == nil {
			arg = claudecode.ProjectDirName(abs)
		}
	}
	for _, p := range projects {
		if p.ProjectDir == arg || filepath.Base(p.Path) == arg {
			return p.ProjectDir, true
		}
	}
	return "", false
}

// shortPath shows a project's directory with the home prefix replaced by ~.
func shortPath(home, path, fallback string) string {
	if path == "" {
		return fallback
	}
	if rel, ok := strings.CutPrefix(path, home); ok {
		return "~" + rel
	}
	return path
}

// human formats a token count as 950, 12.3K, 4.5M or 1.2B.
func human(n int64) string {
	switch {
	case n >= 1e9:
		return fmt.Sprintf("%.1fB", float64(n)/1e9)
	case n >= 1e6:
		return fmt.Sprintf("%.1fM", float64(n)/1e6)
	case n >= 1e3:
		return fmt.Sprintf("%.1fK", float64(n)/1e3)
	}
	return fmt.Sprint(n)
}
