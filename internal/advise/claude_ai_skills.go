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

	var overrides []string
	for _, n := range names {
		overrides = append(overrides, fmt.Sprintf("%q: \"user-invocable-only\"", n))
	}
	return []Finding{{
		Title: fmt.Sprintf("%d skills synced from claude.ai add ≈ %d tokens to every session", len(names), tokens),
		Detail: "Skills: " + strings.Join(names, ", ") + ". " +
			"Hide the ones code projects never use from the model with skillOverrides in " +
			"~/.claude/settings.json (they stay available as /name), or stop syncing them with " +
			"\"syncClaudeAiSkills\": false.",
		Diff:  "+ \"skillOverrides\": {" + strings.Join(overrides, ", ") + "}",
		Saves: tokens,
	}}
}
