package advise

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/axvilab/quipu/internal/claudecode"
)

func TestUnusedSkill(t *testing.T) {
	home := t.TempDir()
	cfg := filepath.Join(home, ".claude")
	shop := filepath.Join(home, "src", "shop")
	blog := filepath.Join(home, "src", "blog")
	skill := func(dir, name string) {
		os.MkdirAll(filepath.Join(dir, "skills", name), 0o755)
		os.WriteFile(filepath.Join(dir, "skills", name, "SKILL.md"), []byte("---\nname: "+name+"\ndescription: Does "+name+" things\n---\nBody.\n"), 0o644)
	}
	skill(cfg, "ship")                            // used in blog
	skill(cfg, "old")                             // never used
	skill(cfg, "hidden")                          // never used, already hidden
	skill(filepath.Join(shop, ".claude"), "seed") // used only in blog, which has no such skill
	os.WriteFile(filepath.Join(cfg, "settings.json"), []byte(`{"skillOverrides": {"hidden": "user-invocable-only"}}`), 0o644)

	in := Input{Home: home, ConfigDir: cfg, Project: shop, Items: claudecode.Instructions(home, cfg, shop),
		SkillUse: map[string][]string{"ship": {blog}, "seed": {blog}}, Sessions: 5}
	got := map[string]Finding{}
	for _, f := range unusedSkill(in) {
		got[f.Params["name"]] = f
	}
	if len(got) != 2 || got["old"].Params["state"] != "user" || got["seed"].Params["state"] != "project" {
		t.Fatalf("findings = %+v", got)
	}
	if got["old"].Path != claudecode.UserSettings(cfg) || got["seed"].Path != claudecode.LocalSettings(shop) {
		t.Errorf("settings: %s, %s", got["old"].Path, got["seed"].Path)
	}
	after, err := got["old"].Edits[0].Apply([]byte(`{"theme": "dark", "skillOverrides": {"hidden": "user-invocable-only"}}`))
	if err != nil || !strings.Contains(string(after), `"old": "user-invocable-only"`) || !strings.Contains(string(after), `"hidden"`) {
		t.Errorf("settings after the fix = %s, %v", after, err)
	}

	// Without usage data the rule stays quiet.
	in.SkillUse = nil
	if f := unusedSkill(in); len(f) != 0 {
		t.Errorf("no usage data: %+v", f)
	}
}
