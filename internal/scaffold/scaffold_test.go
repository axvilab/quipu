package scaffold

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/axvilab/quipu/internal/fsx"
)

func byRel(files []File) map[string]File {
	m := map[string]File{}
	for _, f := range files {
		m[f.Rel] = f
	}
	return m
}

func TestPlanMinimal(t *testing.T) {
	files, err := Plan(Options{Dir: filepath.Join(t.TempDir(), "shop"), Summary: "A shop."})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Content != "# shop\n\nA shop.\n" || files[0].Exists {
		t.Fatalf("files = %+v", files)
	}
}

func TestPlanExistingFolder(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# mine\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o644)
	files, err := Plan(Options{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || !files[0].Exists {
		t.Fatalf("an existing CLAUDE.md is not marked: %+v", files)
	}

	j, _ := fsx.OpenJournal(filepath.Join(t.TempDir(), "journal"))
	id, err := Apply(dir, files, j, fsx.Meta{Rule: "new-project"})
	if err != nil || id != "" {
		t.Fatalf("apply wrote something: %q %v", id, err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "CLAUDE.md")); string(b) != "# mine\n" {
		t.Errorf("existing CLAUDE.md changed: %q", b)
	}

	file := filepath.Join(dir, "main.go")
	if _, err := Plan(Options{Dir: file}); !errors.Is(err, ErrNotFolder) {
		t.Errorf("a file as the folder: %v", err)
	}
}

func TestPlanCopiesExtensions(t *testing.T) {
	other := filepath.Join(t.TempDir(), "other", ".claude")
	skill := filepath.Join(other, "skills", "release")
	os.MkdirAll(filepath.Join(skill, "scripts"), 0o755)
	os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte("---\nname: release\n---\nRelease it.\n"), 0o644)
	os.WriteFile(filepath.Join(skill, "scripts", "tag.sh"), []byte("git tag\n"), 0o644)
	os.MkdirAll(filepath.Join(other, "agents"), 0o755)
	os.WriteFile(filepath.Join(other, "agents", "reviewer.md"), []byte("Review.\n"), 0o644)
	os.MkdirAll(filepath.Join(other, "rules"), 0o755)
	os.WriteFile(filepath.Join(other, "rules", "go.md"), []byte("My Go rules.\n"), 0o644)

	files, err := Plan(Options{Dir: filepath.Join(t.TempDir(), "new"), Extensions: []Extension{
		{Kind: "skill", Path: filepath.Join(skill, "SKILL.md")},
		{Kind: "agent", Path: filepath.Join(other, "agents", "reviewer.md")},
		{Kind: "rule", Path: filepath.Join(other, "rules", "go.md")},
	}})
	if err != nil {
		t.Fatal(err)
	}
	m := byRel(files)
	for rel, want := range map[string]string{
		".claude/skills/release/SKILL.md":       "Release it.",
		".claude/skills/release/scripts/tag.sh": "git tag",
		".claude/agents/reviewer.md":            "Review.",
		".claude/rules/go.md":                   "My Go rules.",
	} {
		if !strings.Contains(m[rel].Content, want) || m[rel].From == "" || m[rel].About == "" {
			t.Errorf("%s = %+v", rel, m[rel])
		}
	}
	if _, err := Plan(Options{Dir: t.TempDir(), Extensions: []Extension{{Kind: "command", Path: "x"}}}); err == nil {
		t.Error("a command was copied")
	}
}

func TestApplyAndUndo(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "projects", "shop")
	j, err := fsx.OpenJournal(filepath.Join(base, "journal"))
	if err != nil {
		t.Fatal(err)
	}
	skill := filepath.Join(base, "other", ".claude", "skills", "ship", "SKILL.md")
	os.MkdirAll(filepath.Dir(skill), 0o755)
	os.WriteFile(skill, []byte("---\nname: ship\n---\nShip.\n"), 0o644)
	files, err := Plan(Options{Dir: dir, Extensions: []Extension{{Kind: "skill", Path: skill}}})
	if err != nil {
		t.Fatal(err)
	}
	batch, _ := fsx.NewBatch()
	id, err := Apply(dir, files, j, fsx.Meta{Rule: "new-project", Batch: batch})
	if err != nil || id == "" {
		t.Fatalf("apply: %q %v", id, err)
	}
	if b, err := os.ReadFile(filepath.Join(dir, ".claude", "skills", "ship", "SKILL.md")); err != nil || !strings.Contains(string(b), "Ship.") {
		t.Fatalf("skill: %v %q", err, b)
	}
	if err := j.Undo(id); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(base, "projects")); !os.IsNotExist(err) {
		t.Errorf("folders quipu made left after undo: %v", err)
	}
}

func TestAboutOf(t *testing.T) {
	for rel, want := range map[string]string{
		"CLAUDE.md": "claude-md", ".claude/rules/a.md": "rule", ".claude/skills/x/SKILL.md": "skill",
		".claude/agents/r.md": "agent", "README.md": "",
	} {
		if got := AboutOf(rel); got != want {
			t.Errorf("AboutOf(%s) = %q, want %q", rel, got, want)
		}
	}
}
