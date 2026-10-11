package advise

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/axvilab/quipu/internal/analytics"
	"github.com/axvilab/quipu/internal/claudecode"
)

// pluginTokens is what a plugin lists in every session of the project: its skills, agents and
// commands. Its MCP tools cost more, but only Claude Code knows their size.
func pluginTokens(items []claudecode.Item, name string) int64 {
	var n int64
	for _, it := range items {
		if it.Scope == "plugin "+name && it.Load == claudecode.Always {
			n += analytics.EstimateTokens(it.Text)
		}
	}
	return n
}

// enablePluginEdit sets enabledPlugins.<id> in a settings file, with a diff line for it.
func enablePluginEdit(path, id string, on bool) (Edit, string) {
	key, _ := json.Marshal(id)
	return Edit{Path: path, JSONKey: "enabledPlugins", JSONSet: map[string]any{id: on}},
		fmt.Sprintf("%s+ \"enabledPlugins\": { %s: %v }\n", hunk(path, 0), key, on)
}

// unusedPlugin flags a plugin enabled for the project that no project used in the last 30 days,
// and turns it off where it is enabled.
func unusedPlugin(in Input) []Finding {
	if in.PluginUse == nil || in.Sessions == 0 {
		return nil
	}
	var out []Finding
	for _, p := range claudecode.Plugins(in.ConfigDir, in.Project) {
		if len(in.PluginUse[p.Name]) > 0 {
			continue
		}
		tokens := pluginTokens(in.Items, p.Name)
		edit, diff := enablePluginEdit(p.Settings, p.ID, false)
		out = append(out, Finding{
			Title: fmt.Sprintf("plugin %s was not used in 30 days", p.Name),
			Detail: "Its skills, agents, commands and MCP tools are listed in every session where it is enabled. " +
				"Turn it off; enable it again when you need it.",
			Path:   p.Settings,
			Saves:  tokens,
			Diff:   strings.TrimRight(diff, "\n"),
			Edits:  []Edit{edit},
			Params: map[string]string{"name": p.Name, "tokens": fmt.Sprint(tokens), "state": p.Scope},
		})
	}
	return out
}

// pluginOneProject flags a plugin enabled in the user's settings, so in every project, that only
// one project used in the last 30 days, and moves it to that project's local settings.
func pluginOneProject(in Input) []Finding {
	if in.PluginUse == nil || len(in.Projects) < 2 {
		return nil
	}
	var out []Finding
	for _, p := range claudecode.Plugins(in.ConfigDir, in.Project) {
		used := in.PluginUse[p.Name]
		if p.Scope != "user" || len(used) != 1 {
			continue
		}
		home := used[0]
		tokens := pluginTokens(in.Items, p.Name)
		off, d1 := enablePluginEdit(p.Settings, p.ID, false)
		on, d2 := enablePluginEdit(claudecode.LocalSettings(home), p.ID, true)
		out = append(out, Finding{
			Title: fmt.Sprintf("plugin %s is enabled for every project but used only in %s", p.Name, filepath.Base(home)),
			Detail: "It is listed in every session of every project. Enable it only where it is used; the other " +
				"projects stop paying for it.",
			Path:  p.Settings,
			Saves: tokens,
			Diff:  strings.TrimRight(d1+d2, "\n"),
			Edits: []Edit{off, on},
			Params: map[string]string{"name": p.Name, "project": filepath.Base(home), "tokens": fmt.Sprint(tokens),
				"others": fmt.Sprint(len(in.Projects) - 1)},
		})
	}
	return out
}
