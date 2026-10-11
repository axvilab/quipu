package advise

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/axvilab/quipu/internal/claudecode"
)

func TestLargeImport(t *testing.T) {
	dir := t.TempDir()
	claudeMD := filepath.Join(dir, "CLAUDE.md")
	os.WriteFile(claudeMD, []byte("# Demo\nDesign: @docs/DESIGN.md\n"), 0o644)

	items := []claudecode.Item{
		{Load: claudecode.Always, Kind: "import", Ref: "docs/DESIGN.md", From: claudeMD, Text: strings.Repeat("word ", 1000)},
		{Load: claudecode.Always, Kind: "import", Ref: "small.md", From: claudeMD, Text: "tiny"},
	}
	got := largeImport(Input{Items: items})
	if len(got) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(got), got)
	}
	f := got[0]
	wantDiff := "@@ " + claudeMD + ":2\n- Design: @docs/DESIGN.md\n+ Design: `docs/DESIGN.md` (read it when relevant)"
	if f.Diff != wantDiff || f.Path != claudeMD || f.Saves < largeImportTokens {
		t.Errorf("finding = %+v", f)
	}
}

func TestClaudeAISkills(t *testing.T) {
	var items []claudecode.Item
	for _, n := range []string{"pptx", "docx", "xlsx"} {
		items = append(items, claudecode.Item{Load: claudecode.Always, Scope: "claude.ai", Kind: "skill", Name: n, Text: strings.Repeat("x", 600)})
	}
	got := claudeAISkills(Input{Items: items})
	if len(got) != 1 || !strings.Contains(got[0].Diff, `"docx": "user-invocable-only"`) {
		t.Errorf("findings = %+v", got)
	}
	if got := claudeAISkills(Input{Items: items[:1]}); len(got) != 0 {
		t.Errorf("one small skill should not be flagged: %+v", got)
	}
}

func TestSizeLimits(t *testing.T) {
	dir := t.TempDir()
	mem := filepath.Join(dir, "MEMORY.md")
	os.WriteFile(mem, []byte(strings.Repeat("- entry\n", 250)), 0o644)

	in := Input{Items: []claudecode.Item{
		{Load: claudecode.Always, Kind: "CLAUDE.md", Name: "CLAUDE.md", Text: strings.Repeat("rule\n", 250)},
		{Load: claudecode.Always, Kind: "CLAUDE.md", Name: "CLAUDE.md", Text: "short\n"},
		{Load: claudecode.Always, Kind: "memory", Name: "MEMORY.md", Path: mem, Text: "truncated"},
	}}
	got := Run(in, map[string]bool{"large-import": true, "claude-ai-skills": true})
	if len(got) != 2 || got[0].Rule != "long-claude-md" || got[1].Rule != "memory-limit" {
		t.Fatalf("findings = %+v", got)
	}
	if !strings.Contains(got[1].Title, "250 lines") {
		t.Errorf("memory finding = %q", got[1].Title)
	}
	if got := Run(in, map[string]bool{"long-claude-md": true, "memory-limit": true}); len(got) != 0 {
		t.Errorf("disabled rules still ran: %+v", got)
	}
}

func TestEditReplace(t *testing.T) {
	e := Edit{Old: "Design: @docs/DESIGN.md", New: "Design: `docs/DESIGN.md` (read it when relevant)"}
	got, err := e.Apply([]byte("# Demo\nDesign: @docs/DESIGN.md\n"))
	if err != nil || string(got) != "# Demo\nDesign: `docs/DESIGN.md` (read it when relevant)\n" {
		t.Errorf("got %q, %v", got, err)
	}
	if _, err := e.Apply([]byte("# Demo\nsomeone already edited this\n")); !errors.Is(err, ErrStale) {
		t.Errorf("stale content: err = %v, want ErrStale", err)
	}
}

func TestEditJSON(t *testing.T) {
	e := Edit{JSONKey: "skillOverrides", JSONSet: map[string]any{"pptx": "user-invocable-only"}}

	in := `{"env": {"A": "1"}, "theme": "dark", "skillOverrides": {"docs": "off"}, "zzz": [1, 2]}`
	got, err := e.Apply([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	want := `{
  "env": {
    "A": "1"
  },
  "theme": "dark",
  "skillOverrides": {
    "docs": "off",
    "pptx": "user-invocable-only"
  },
  "zzz": [
    1,
    2
  ]
}
`
	if string(got) != want {
		t.Errorf("merge into existing key:\n%s\nwant\n%s", got, want)
	}

	got, err = e.Apply([]byte(`{"theme": "dark"}`))
	if err != nil || !strings.Contains(string(got), "\"theme\": \"dark\",\n  \"skillOverrides\": {\n    \"pptx\"") {
		t.Errorf("new key appended last:\n%s (%v)", got, err)
	}
	if got, err := e.Apply(nil); err != nil || !strings.Contains(string(got), "pptx") {
		t.Errorf("missing file: %q, %v", got, err)
	}
	if _, err := e.Apply([]byte("not json")); !errors.Is(err, ErrStale) {
		t.Errorf("broken JSON: err = %v, want ErrStale", err)
	}
}
