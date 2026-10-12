package profile

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLabelAndSharing(t *testing.T) {
	home := t.TempDir()
	os.MkdirAll(filepath.Join(home, ".claude", "skills", "release"), 0o755)
	os.WriteFile(filepath.Join(home, ".claude", "skills", "release", "SKILL.md"), []byte("release"), 0o644)
	os.WriteFile(filepath.Join(home, ".claude", "CLAUDE.md"), []byte("me"), 0o644)
	c, err := Load(home, filepath.Join(home, "profiles.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Add("work", []string{"CLAUDE.md"}); err != nil {
		t.Fatal(err)
	}

	if err := c.SetLabel("work", " Office "); err != nil || c.Find("work").Title() != "Office" {
		t.Fatalf("label: %v %q", err, c.Find("work").Title())
	}
	c.SetLabel("work", "work")
	if c.Find("work").Label != "" {
		t.Error("a label equal to the name is kept")
	}

	if got := c.Shared("work"); len(got) != 1 || got[0] != "CLAUDE.md" {
		t.Fatalf("shared = %v", got)
	}
	if err := c.Share("work", "skills"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude-work", "skills", "release", "SKILL.md")); err != nil {
		t.Error("shared skills are not visible in work")
	}
	// Agents do not exist yet: sharing starts them in the default profile.
	if err := c.Share("work", "agents"); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Stat(filepath.Join(home, ".claude", "agents")); err != nil || !fi.IsDir() {
		t.Error("agents folder not started in the default profile")
	}

	// Unsharing leaves a copy, so nothing is lost.
	if err := c.Unshare("work", "skills"); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Lstat(filepath.Join(home, ".claude-work", "skills"))
	if err != nil || fi.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("skills still a link: %v", err)
	}
	if b, _ := os.ReadFile(filepath.Join(home, ".claude-work", "skills", "release", "SKILL.md")); string(b) != "release" {
		t.Errorf("copy = %q", b)
	}
	// Now work has its own skills: sharing would hide them, so it stops.
	var own *OwnError
	if err := c.Share("work", "skills"); !errors.As(err, &own) || own.Files != 1 {
		t.Errorf("share over own skills: %v", err)
	}
	if err := c.Share(Default, "skills"); err == nil {
		t.Error("the default profile shared with itself")
	}
}
