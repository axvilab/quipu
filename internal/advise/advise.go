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
	Home      string
	ConfigDir string // the profile's Claude Code config dir, e.g. ~/.claude
	Project   string
	Items     []claudecode.Item
	// Projects holds the items of every known project of the profile (Project included), keyed by
	// project path, for rules that compare projects. Callers scan them once and share the map.
	Projects map[string][]claudecode.Item
	// PluginUse holds, per plugin name, the folders of the projects that used it in the last 30
	// days; Sessions counts the sessions of that window. Without data (nil) plugin rules stay quiet.
	PluginUse map[string][]string
	Sessions  int64
}

// Finding is one problem and what to do about it.
type Finding struct {
	Rule   string
	Title  string // one line: what is wrong
	Detail string // why it matters and what to do
	Path   string // file to change, if any
	// Diff is the proposed change in unified-diff style, if the fix is mechanical. A line
	// "@@ <path>:<line>" starts a hunk: the file and the line number its first line has.
	Diff  string
	Saves int64  // approximate tokens saved per session, 0 if unknown
	Edits []Edit // set when the fix can be applied automatically

	// Params fill the rule's translated title and detail in the UI ("rule.<id>.title");
	// Title and Detail above are the English text for the CLI.
	Params map[string]string
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
	{"repeated-in-session", "a project file repeats lines your own CLAUDE.md already sends", repeatedInSession},
	{"shared-block", "the same CLAUDE.md block is copied into several projects", sharedBlock},
	{"copied-extension", "the same skill, rule or agent is copied into several projects", copiedExtension},
	{"unused-plugin", "a plugin was not used in any project in 30 days", unusedPlugin},
	{"plugin-one-project", "a plugin enabled for every project is used in only one", pluginOneProject},
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
