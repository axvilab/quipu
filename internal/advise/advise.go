// Package advise checks a project's Claude Code setup against a list of rules and proposes
// concrete fixes.
//
// To add a rule: write a function with the Check signature in its own file, give it a short
// kebab-case ID and append it to Rules. A rule only reads its Input; it never writes files.
package advise

import (
	"sort"

	"github.com/axvilab/quipu/internal/claudecode"
)

// Input is everything a rule may look at.
type Input struct {
	Home    string
	Project string
	Items   []claudecode.Item
}

// Finding is one problem and what to do about it.
type Finding struct {
	Rule   string
	Title  string // one line: what is wrong
	Detail string // why it matters and what to do
	Path   string // file to change, if any
	Diff   string // proposed change in unified-diff style, if the fix is mechanical
	Saves  int64  // approximate tokens saved per session, 0 if unknown
}

// Rule is one check.
type Rule struct {
	ID      string
	Summary string
	Check   func(Input) []Finding
}

// Rules are all known checks, in the order they run.
var Rules = []Rule{
	{"large-import", "a large file @imported into CLAUDE.md is paid in every session", largeImport},
	{"claude-ai-skills", "skills synced from claude.ai are listed in every code project", claudeAISkills},
	{"long-claude-md", "CLAUDE.md longer than 200 lines", longClaudeMD},
	{"memory-limit", "MEMORY.md near or over the 200-line load limit", memoryLimit},
}

// Run applies every rule not in disabled and returns findings, biggest savings first.
func Run(in Input, disabled map[string]bool) []Finding {
	var out []Finding
	for _, r := range Rules {
		if disabled[r.ID] {
			continue
		}
		for _, f := range r.Check(in) {
			f.Rule = r.ID
			out = append(out, f)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Saves > out[j].Saves })
	return out
}
