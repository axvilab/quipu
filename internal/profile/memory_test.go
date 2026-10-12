package profile

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/axvilab/quipu/internal/claudecode"
	"github.com/axvilab/quipu/internal/fsx"
)

func memorySetup(t *testing.T) (home, project string, c *Config, j *fsx.Journal) {
	t.Helper()
	home = t.TempDir()
	project = filepath.Join(home, "work", "shop")
	os.MkdirAll(project, 0o755)
	c, err := Load(home, filepath.Join(home, ".config", "quipu", "profiles.json"))
	if err != nil {
		t.Fatal(err)
	}
	c.Profiles = append(c.Profiles, Profile{Name: "work", Dir: filepath.Join(home, ".claude-work")})
	j, err = fsx.OpenJournal(filepath.Join(home, "journal"))
	if err != nil {
		t.Fatal(err)
	}
	return home, project, c, j
}

func writeMemory(t *testing.T, configDir, project string, files map[string]string) {
	t.Helper()
	dir := claudecode.OwnMemoryDir(configDir, project)
	for name, text := range files {
		os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755)
		os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644)
	}
}

func TestShareMemory(t *testing.T) {
	home, project, c, j := memorySetup(t)
	personal, work := filepath.Join(home, ".claude"), filepath.Join(home, ".claude-work")
	writeMemory(t, personal, project, map[string]string{"MEMORY.md": "- stage\n", "stage.md": "copied from dev\n"})
	writeMemory(t, work, project, map[string]string{"todo.md": "x\n", "stage.md": "copied from dev\n"})

	m := ProjectMemory(c, home, project, "work")
	if m.Shared || m.Source != "default" || m.SourceFiles != 2 || m.Own != 2 {
		t.Fatalf("before: %+v", m)
	}
	id, err := ShareMemory(c, home, project, "work", j)
	if err != nil || id == "" {
		t.Fatalf("share: %q %v", id, err)
	}
	shared := claudecode.OwnMemoryDir(personal, project)
	if got := claudecode.ProjectMemoryDir(home, project); got != shared {
		t.Errorf("autoMemoryDirectory = %q, want %q", got, shared)
	}
	b, _ := os.ReadFile(claudecode.LocalSettings(project))
	if !strings.Contains(string(b), `"~/.claude/projects/`) {
		t.Errorf("setting is not written with ~: %s", b)
	}
	if _, err := os.Stat(filepath.Join(shared, "todo.md")); err != nil {
		t.Error("work's own file was not moved into the shared folder")
	}
	if n := countFiles(claudecode.OwnMemoryDir(work, project)); n != 0 {
		t.Errorf("%d files left in work's own folder", n)
	}
	if m := ProjectMemory(c, home, project, "work"); !m.Shared || m.From != "default" {
		t.Errorf("after: %+v", m)
	}
	// Both profiles now read the same folder.
	if claudecode.MemoryDir(work, project) != claudecode.MemoryDir(personal, project) {
		t.Error("profiles read different memory folders")
	}

	// One undo reverts the move and the setting.
	if err := j.Undo(id); err != nil {
		t.Fatal(err)
	}
	if claudecode.ProjectMemoryDir(home, project) != "" || countFiles(claudecode.OwnMemoryDir(work, project)) != 2 {
		t.Error("undo did not bring back work's own memory")
	}
}

func TestShareMemoryConflictAndUnshare(t *testing.T) {
	home, project, c, j := memorySetup(t)
	personal, work := filepath.Join(home, ".claude"), filepath.Join(home, ".claude-work")
	if _, err := ShareMemory(c, home, project, "work", j); !errors.Is(err, ErrNoMemory) {
		t.Errorf("nothing to share: %v", err)
	}
	writeMemory(t, personal, project, map[string]string{"MEMORY.md": "personal\n"})
	writeMemory(t, work, project, map[string]string{"MEMORY.md": "work\n"})
	var conflict *ConflictError
	if _, err := ShareMemory(c, home, project, "work", j); !errors.As(err, &conflict) || conflict.Files[0] != "MEMORY.md" {
		t.Fatalf("conflict: %v", err)
	}
	if claudecode.ProjectMemoryDir(home, project) != "" {
		t.Error("a conflict still wrote the setting")
	}

	os.Remove(filepath.Join(claudecode.OwnMemoryDir(work, project), "MEMORY.md"))
	if _, err := ShareMemory(c, home, project, "work", j); err != nil {
		t.Fatal(err)
	}
	if _, err := UnshareMemory(c, home, project, "work", j); err != nil {
		t.Fatal(err)
	}
	if claudecode.ProjectMemoryDir(home, project) != "" {
		t.Error("setting left after unshare")
	}
	if b, _ := os.ReadFile(filepath.Join(claudecode.OwnMemoryDir(work, project), "MEMORY.md")); string(b) != "personal\n" {
		t.Errorf("work did not keep a copy: %q", b)
	}
}
