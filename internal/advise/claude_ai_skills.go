package advise

import (
	"fmt"
	"sort"
	"strings"

	"github.com/axvilab/quipu/internal/analytics"
	"github.com/axvilab/quipu/internal/claudecode"
)

// claudeAISkillsTokens is the listing size of synced skills worth mentioning.
const claudeAISkillsTokens = 300

// claudeAISkills flags skills enabled on claude.ai (documents, slides, spreadsheets…): Claude
// Code syncs them and lists them in every session of every project.
func claudeAISkills(in Input) []Finding {
	var names []string
	var tokens int64
	for _, it := range in.Items {
		if it.Scope == "claude.ai" && it.Kind == "skill" && it.Load == claudecode.Always {
			names = append(names, it.Name)
			tokens += analytics.EstimateTokens(it.Text)
		}
	}
	if tokens < claudeAISkillsTokens {
		return nil
	}
	sort.Strings(names)

	diff := []string{`+ "skillOverrides": {`}
	for i, n := range names {
		comma := ","
		if i == len(names)-1 {
			comma = ""
		}
		diff = append(diff, fmt.Sprintf(`+   %q: "user-invocable-only"%s`, n, comma))
	}
	diff = append(diff, "+ }")
	return []Finding{{
		Title: fmt.Sprintf("%d skills synced from claude.ai add ≈ %d tokens to every session", len(names), tokens),
		Detail: "Skills: " + strings.Join(names, ", ") + ". " +
			"Hide the ones code projects never use from the model with skillOverrides in " +
			"~/.claude/settings.json (they stay available as /name), or stop syncing them with " +
			"\"syncClaudeAiSkills\": false.",
		Diff:  strings.Join(diff, "\n"),
		Saves: tokens,
	}}
}
