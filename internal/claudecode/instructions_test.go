package claudecode

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestInstructions(t *testing.T) {
	home := t.TempDir()
	cfg := filepath.Join(home, ".claude")
	proj := filepath.Join(home, "src", "demo")

	write(t, filepath.Join(cfg, "CLAUDE.md"), "Answer briefly.\n")
	write(t, filepath.Join(cfg, "skills", "notes", "SKILL.md"), "---\nname: notes\ndescription: Take notes\n---\nSteps.\n")
	write(t, filepath.Join(cfg, "skills", "synced", "acc_1", "pptx", "SKILL.md"), "---\nname: pptx\ndescription: Slides\n---\nBody.\n")
	write(t, filepath.Join(cfg, "settings.json"), `{"enabledPlugins":{"fmt@market":true,"off@market":false}}`)
	write(t, filepath.Join(cfg, "plugins", "installed_plugins.json"),
		`{"plugins":{"fmt@market":[{"installPath":"`+filepath.Join(home, "plug", "fmt")+`"}],"off@market":[{"installPath":"`+filepath.Join(home, "plug", "off")+`"}]}}`)
	write(t, filepath.Join(home, "plug", "fmt", "skills", "format", "SKILL.md"), "---\nname: format\ndescription: Format code\n---\nRun the formatter.\n")
	write(t, filepath.Join(home, "plug", "off", "skills", "hidden", "SKILL.md"), "---\nname: hidden\ndescription: x\n---\n")

	write(t, filepath.Join(home, "src", "CLAUDE.md"), "Parent rules.\n")
	write(t, filepath.Join(proj, "CLAUDE.md"), "Project.\nSee @docs/adr/README.md and `@not/an/import`.\n")
	write(t, filepath.Join(proj, "docs", "adr", "README.md"), "ADR index.\n")
	write(t, filepath.Join(proj, "CLAUDE.local.md"), "My machine.\n")
	write(t, filepath.Join(proj, ".claude", "rules", "style.md"), "Always style.\n")
	write(t, filepath.Join(proj, ".claude", "rules", "go.md"), "---\npaths:\n  - \"**/*.go\"\n---\nGo rules.\n")
	write(t, filepath.Join(proj, ".claude", "agents", "reviewer.md"), "---\nname: reviewer\ndescription: Reviews code\n---\nYou review.\n")
	write(t, filepath.Join(proj, "web", "CLAUDE.md"), "Frontend notes.\n")
	write(t, filepath.Join(proj, "node_modules", "pkg", "CLAUDE.md"), "must be skipped\n")

	mem := filepath.Join(cfg, "projects", ProjectDirName(proj), "memory")
	write(t, filepath.Join(mem, "MEMORY.md"), strings.Repeat("line\n", 300))
	write(t, filepath.Join(mem, "deploy.md"), "Deploy notes.\n")

	var got []string
	for _, it := range Instructions(home, cfg, proj) {
		rel, _ := filepath.Rel(home, it.Path)
		got = append(got, string(it.Load)+" "+it.Scope+" "+it.Kind+" "+rel)
		if it.Kind == "memory" && it.Name == "MEMORY.md" && strings.Count(it.Text, "\n") != memoryMaxLines {
			t.Errorf("MEMORY.md not truncated to %d lines", memoryMaxLines)
		}
		if it.Kind == "import" && (it.Ref != "docs/adr/README.md" || filepath.Base(it.From) != "CLAUDE.md") {
			t.Errorf("import from %q ref %q", it.From, it.Ref)
		}
		if it.Kind == "skill" && it.Load == Always && it.Name == "format" && it.Text != "format: Format code" {
			t.Errorf("skill listing text = %q", it.Text)
		}
	}
	sort.Strings(got)

	want := []string{
		"always local CLAUDE.md src/demo/CLAUDE.local.md",
		"always memory memory .claude/projects/" + ProjectDirName(proj) + "/memory/MEMORY.md",
		"always plugin fmt skill plug/fmt/skills/format/SKILL.md",
		"always project CLAUDE.md src/CLAUDE.md",
		"always project CLAUDE.md src/demo/CLAUDE.md",
		"always project agent src/demo/.claude/agents/reviewer.md",
		"always project import src/demo/docs/adr/README.md",
		"always project rule src/demo/.claude/rules/style.md",
		"always user CLAUDE.md .claude/CLAUDE.md",
		"always user skill .claude/skills/notes/SKILL.md",
		"always claude.ai skill .claude/skills/synced/acc_1/pptx/SKILL.md",
		"on-demand claude.ai skill .claude/skills/synced/acc_1/pptx/SKILL.md",
		"on-demand memory memory .claude/projects/" + ProjectDirName(proj) + "/memory/deploy.md",
		"on-demand plugin fmt skill plug/fmt/skills/format/SKILL.md",
		"on-demand project CLAUDE.md src/demo/web/CLAUDE.md",
		"on-demand project agent src/demo/.claude/agents/reviewer.md",
		"on-demand project rule src/demo/.claude/rules/go.md",
		"on-demand user skill .claude/skills/notes/SKILL.md",
	}
	sort.Strings(want)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("items:\n%s\n\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestExtensionAt(t *testing.T) {
	base := filepath.Join(t.TempDir(), "p", ".claude")
	write := func(rel string) string {
		p := filepath.Join(base, rel)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte("x"), 0o644)
		return p
	}
	skill := write("skills/release/SKILL.md")
	rule := write("rules/go/style.md")
	agent := write("agents/reviewer.md")
	other := write("notes.md")
	for _, c := range []struct{ path, kind, file string }{
		{filepath.Dir(skill), "skill", skill},
		{skill, "skill", skill},
		{rule, "rule", rule},
		{agent, "agent", agent},
		{other, "", ""},
		{filepath.Join(base, "missing.md"), "", ""},
	} {
		kind, file, ok := ExtensionAt(c.path)
		if kind != c.kind || file != c.file || ok != (c.kind != "") {
			t.Errorf("ExtensionAt(%s) = %q %q %v", c.path, kind, file, ok)
		}
	}
}
