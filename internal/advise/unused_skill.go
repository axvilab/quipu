package advise

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/axvilab/quipu/internal/analytics"
	"github.com/axvilab/quipu/internal/claudecode"
)

// unusedSkill flags the user's and the project's own skills that are listed in every session but
// were not invoked in 30 days, by Claude or as /name, and hides them from Claude with
// skillOverrides; /name keeps working. claude.ai and plugin skills have rules of their own.
func unusedSkill(in Input) []Finding {
	if in.SkillUse == nil || in.Sessions == 0 {
		return nil
	}
	var out []Finding
	for _, it := range in.Items {
		if it.Kind != "skill" || it.Load != claudecode.Always || (it.Scope != "user" && it.Scope != "project") {
			continue
		}
		used := in.SkillUse[it.Name]
		if it.Scope == "user" && len(used) > 0 || it.Scope == "project" && slices.Contains(used, in.Project) {
			continue
		}
		settings := claudecode.UserSettings(in.ConfigDir)
		if it.Scope == "project" {
			settings = claudecode.LocalSettings(in.Project)
		}
		key, _ := json.Marshal(it.Name)
		tokens := analytics.EstimateTokens(it.Text)
		out = append(out, Finding{
			Title: fmt.Sprintf("skill %s was not used in 30 days", it.Name),
			Detail: "Its name and description are listed in every session, yet neither Claude nor you invoked it. " +
				"Hide it from Claude with skillOverrides; /" + it.Name + " keeps working.",
			Path:   settings,
			Saves:  tokens,
			Diff:   fmt.Sprintf("%s+ \"skillOverrides\": { %s: \"user-invocable-only\" }", hunk(settings, 0), key),
			Edits:  []Edit{{Path: settings, JSONKey: "skillOverrides", JSONSet: map[string]any{it.Name: "user-invocable-only"}}},
			Params: map[string]string{"name": it.Name, "tokens": fmt.Sprint(tokens), "state": it.Scope, "project": filepath.Base(in.Project)},
		})
	}
	return out
}
