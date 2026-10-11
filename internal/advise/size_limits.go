package advise

import (
	"fmt"
	"os"
	"strings"

	"github.com/axvilab/quipu/internal/claudecode"
)

const (
	claudeMDMaxLines   = 200 // recommended maximum for an always-loaded CLAUDE.md
	memoryLoadLines    = 200 // Claude Code loads only this many lines of MEMORY.md
	memoryWarnFraction = 0.9
)

// longClaudeMD flags always-loaded CLAUDE.md files over the recommended length.
func longClaudeMD(in Input) []Finding {
	var out []Finding
	for _, it := range in.Items {
		if it.Kind != "CLAUDE.md" || it.Load != claudecode.Always {
			continue
		}
		if n := lineCount(it.Text); n > claudeMDMaxLines {
			out = append(out, Finding{
				Title: fmt.Sprintf("%s has %d lines (recommended ≤ %d)", it.Name, n, claudeMDMaxLines),
				Detail: "Everything in it is sent with every request. Move topic-specific rules to " +
					".claude/rules/*.md with paths:, and procedures to skills.",
				Path:   it.Path,
				Params: map[string]string{"name": it.Name, "lines": fmt.Sprint(n), "limit": fmt.Sprint(claudeMDMaxLines)},
			})
		}
	}
	return out
}

// memoryLimit flags MEMORY.md files close to or beyond the part Claude Code loads: lines past
// the limit are silently ignored.
func memoryLimit(in Input) []Finding {
	var out []Finding
	for _, it := range in.Items {
		if it.Kind != "memory" || it.Name != "MEMORY.md" || it.Load != claudecode.Always {
			continue
		}
		b, err := os.ReadFile(it.Path) // the item text is already truncated
		if err != nil {
			continue
		}
		n := lineCount(string(b))
		switch {
		case n > memoryLoadLines:
			out = append(out, Finding{
				Title:  fmt.Sprintf("MEMORY.md has %d lines; only the first %d are loaded", n, memoryLoadLines),
				Detail: "Entries past the limit are invisible to Claude. Merge or remove stale entries.",
				Path:   it.Path,
				Params: map[string]string{"lines": fmt.Sprint(n), "limit": fmt.Sprint(memoryLoadLines), "state": "over"},
			})
		case float64(n) > memoryWarnFraction*memoryLoadLines:
			out = append(out, Finding{
				Title:  fmt.Sprintf("MEMORY.md has %d of %d loadable lines", n, memoryLoadLines),
				Detail: "New entries will soon be cut off. Merge or remove stale entries.",
				Path:   it.Path,
				Params: map[string]string{"lines": fmt.Sprint(n), "limit": fmt.Sprint(memoryLoadLines), "state": "near"},
			})
		}
	}
	return out
}

func lineCount(s string) int {
	return strings.Count(strings.TrimRight(s, "\n"), "\n") + 1
}
