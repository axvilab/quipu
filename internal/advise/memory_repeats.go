package advise

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/axvilab/quipu/internal/analytics"
	"github.com/axvilab/quipu/internal/claudecode"
)

// instructionLines are the normalized lines every session of the project already gets from
// CLAUDE.md files, rules and imports, the user's and the project's.
func instructionLines(items []claudecode.Item) map[string]bool {
	seen := map[string]bool{}
	for _, it := range items {
		if it.Load != claudecode.Always || it.Scope == "memory" {
			continue
		}
		if it.Kind != "CLAUDE.md" && it.Kind != "rule" && it.Kind != "import" {
			continue
		}
		for _, line := range strings.Split(it.Text, "\n") {
			if n := norm(line); n != "" {
				seen[n] = true
			}
		}
	}
	return seen
}

// memoryRepeats flags memory that repeats what CLAUDE.md or rules already say: lines of MEMORY.md
// (sent in every session, so they are paid twice) and notes whose text is mostly such lines
// (Claude has the rule anyway; the note and its MEMORY.md line only add noise and drift).
func memoryRepeats(in Input) []Finding {
	seen := instructionLines(in.Items)
	if len(seen) == 0 {
		return nil
	}
	var index *claudecode.Item
	for i, it := range in.Items {
		if it.Scope == "memory" && it.Name == "MEMORY.md" {
			index = &in.Items[i]
		}
	}
	var out []Finding
	for _, it := range in.Items {
		if it.Scope != "memory" || it.Name == "MEMORY.md" {
			continue
		}
		if f, ok := repeatedNote(it, index, seen); ok {
			out = append(out, f)
		}
	}
	if index != nil {
		if f, ok := repeatedIndexLines(*index, seen); ok {
			out = append(out, f)
		}
	}
	return out
}

// repeatedIndexLines removes lines of MEMORY.md that CLAUDE.md or rules already send.
func repeatedIndexLines(it claudecode.Item, seen map[string]bool) (Finding, bool) {
	b, err := os.ReadFile(it.Path)
	if err != nil {
		return Finding{}, false
	}
	text := string(b)
	f := Finding{Path: it.Path}
	var removed strings.Builder
	lines := 0
	for _, r := range runs(text, func(n string) bool { return seen[n] }) {
		if strings.Count(text, r.raw) != 1 {
			continue
		}
		f.Edits = append(f.Edits, Edit{Path: it.Path, Old: r.raw, New: ""})
		f.Diff += hunk(it.Path, lineOf(text, r.raw)) + diffLines("-", r.raw)
		removed.WriteString(r.raw)
		lines += len(r.norms)
	}
	if lines == 0 {
		return Finding{}, false
	}
	f.Saves = analytics.EstimateTokens(removed.String())
	f.Diff = strings.TrimRight(f.Diff, "\n")
	f.Title = fmt.Sprintf("MEMORY.md repeats %d lines CLAUDE.md or rules already send", lines)
	f.Detail = "MEMORY.md is sent in every session next to CLAUDE.md, so these lines are paid twice. " +
		"Remove them from memory; CLAUDE.md keeps them."
	f.Params = map[string]string{"name": it.Name, "lines": fmt.Sprint(lines), "count": fmt.Sprint(lines), "tokens": fmt.Sprint(f.Saves), "state": "index"}
	return f, true
}

// repeatedNote deletes a memory note whose text is mostly lines CLAUDE.md or rules already send,
// with its line in MEMORY.md.
func repeatedNote(it claudecode.Item, index *claudecode.Item, seen map[string]bool) (Finding, bool) {
	b, err := os.ReadFile(it.Path)
	if err != nil {
		return Finding{}, false
	}
	text := string(b)
	total, repeated := 0, 0
	for _, line := range strings.Split(noteBody(text), "\n") {
		if n := norm(line); n != "" {
			total++
			if seen[n] {
				repeated++
			}
		}
	}
	if repeated == 0 || repeated*2 < total {
		return Finding{}, false
	}
	f := Finding{Path: it.Path}
	f.Edits = []Edit{{Path: it.Path, Delete: true, Old: text, Stop: filepath.Dir(it.Path)}}
	f.Diff = hunk(it.Path, 1) + diffPreview("-", text)
	if index != nil {
		if idx, err := os.ReadFile(index.Path); err == nil {
			if line, ok := linkLine(string(idx), it.Name); ok {
				f.Edits = append(f.Edits, Edit{Path: index.Path, Old: line, New: ""})
				f.Diff += "\n" + hunk(index.Path, lineOf(string(idx), line)) + diffLines("-", line)
				f.Saves = analytics.EstimateTokens(line)
			}
		}
	}
	f.Diff = strings.TrimRight(f.Diff, "\n")
	f.Title = fmt.Sprintf("Memory note %s repeats what CLAUDE.md already says", it.Name)
	f.Detail = "Claude gets these lines from CLAUDE.md or rules in every session anyway. The note and " +
		"its line in MEMORY.md only add noise, and the two copies drift apart over time."
	f.Params = map[string]string{"name": it.Name, "lines": fmt.Sprint(repeated), "count": fmt.Sprint(repeated), "tokens": fmt.Sprint(f.Saves), "state": "note"}
	return f, true
}

// noteBody is a memory note without its frontmatter.
func noteBody(text string) string {
	if !strings.HasPrefix(text, "---\n") {
		return text
	}
	if i := strings.Index(text[4:], "\n---\n"); i >= 0 {
		return text[4+i+5:]
	}
	return text
}

// linkLine finds the line of MEMORY.md that links to the note file name, with its line break.
func linkLine(index, name string) (string, bool) {
	var found string
	for _, line := range strings.SplitAfter(index, "\n") {
		if strings.Contains(line, "("+name+")") {
			if found != "" {
				return "", false // more than one: the edit must point at exactly one place
			}
			found = line
		}
	}
	return found, found != ""
}
