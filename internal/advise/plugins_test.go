package advise

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/axvilab/quipu/internal/claudecode"
)

// pluginHome has one plugin installed and enabled in the user's settings, and two projects.
func pluginHome(t *testing.T) (Input, string, string) {
	t.Helper()
	home := t.TempDir()
	cfg := filepath.Join(home, ".claude")
	pluginDir := filepath.Join(cfg, "plugins", "cache", "official", "browser", "1.0.0")
	files := map[string]string{
		filepath.Join(pluginDir, "skills", "browse", "SKILL.md"): "---\nname: browse\ndescription: " + strings.Repeat("Drive a browser to test pages. ", 10) + "\n---\nBody.\n",
		filepath.Join(cfg, "settings.json"):                      `{"theme": "dark", "enabledPlugins": {"browser@official": true}}`,
	}
	installed, _ := json.Marshal(map[string]any{"plugins": map[string]any{"browser@official": []map[string]string{{"installPath": pluginDir}}}})
	files[filepath.Join(cfg, "plugins", "installed_plugins.json")] = string(installed)
	for p, text := range files {
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(text), 0o644)
	}
	api, web := filepath.Join(home, "src", "api"), filepath.Join(home, "src", "web")
	os.MkdirAll(api, 0o755)
	os.MkdirAll(web, 0o755)
	all := map[string][]claudecode.Item{api: claudecode.Instructions(home, cfg, api), web: claudecode.Instructions(home, cfg, web)}
	return Input{Home: home, ConfigDir: cfg, Project: api, Items: all[api], Projects: all, Sessions: 5}, api, web
}

func TestUnusedPlugin(t *testing.T) {
	in, api, _ := pluginHome(t)
	in.PluginUse = map[string][]string{}
	got := unusedPlugin(in)
	if len(got) != 1 || got[0].Params["name"] != "browser" || got[0].Saves == 0 || len(got[0].Edits) != 1 {
		t.Fatalf("findings = %+v", got)
	}
	settings := filepath.Join(in.ConfigDir, "settings.json")
	before, _ := os.ReadFile(settings)
	after, err := got[0].Edits[0].Apply(before)
	if err != nil || !strings.Contains(string(after), `"browser@official": false`) || !strings.Contains(string(after), `"theme": "dark"`) {
		t.Errorf("settings after the fix:\n%s (%v)", after, err)
	}

	in.PluginUse = map[string][]string{"browser": {api}}
	if got := unusedPlugin(in); len(got) != 0 {
		t.Errorf("used plugin flagged: %+v", got)
	}
	in.PluginUse, in.Sessions = map[string][]string{}, 0
	if got := unusedPlugin(in); len(got) != 0 {
		t.Errorf("flagged without any sessions: %+v", got)
	}
	in.PluginUse, in.Sessions = nil, 5
	if got := unusedPlugin(in); len(got) != 0 {
		t.Errorf("flagged without usage data: %+v", got)
	}
}

func TestPluginOneProject(t *testing.T) {
	in, api, web := pluginHome(t)
	in.PluginUse = map[string][]string{"browser": {web}}
	got := pluginOneProject(in)
	if len(got) != 1 || got[0].Params["project"] != "web" || len(got[0].Edits) != 2 {
		t.Fatalf("findings = %+v", got)
	}
	if e := got[0].Edits[1]; e.Path != claudecode.LocalSettings(web) || e.JSONSet["browser@official"] != true {
		t.Errorf("enable edit = %+v", e)
	}
	// The finding is the same from every project, so the UI shows it once.
	in.Project, in.Items = web, in.Projects[web]
	if other := pluginOneProject(in); len(other) != 1 || other[0].ID() != got[0].ID() {
		t.Errorf("web reports %+v", other)
	}
	in.PluginUse = map[string][]string{"browser": {api, web}}
	if got := pluginOneProject(in); len(got) != 0 {
		t.Errorf("plugin used in two projects flagged: %+v", got)
	}
}
