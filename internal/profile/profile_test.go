package profile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProfiles(t *testing.T) {
	home := t.TempDir()
	os.MkdirAll(filepath.Join(home, ".claude", "skills", "notes"), 0o755)
	os.WriteFile(filepath.Join(home, ".claude", "CLAUDE.md"), []byte("me"), 0o644)
	file := filepath.Join(home, ".config", "quipu", "profiles.json")

	c, err := Load(home, file)
	if err != nil {
		t.Fatal(err)
	}
	if d := c.Find(Default); d == nil || d.Dir != filepath.Join(home, ".claude") {
		t.Fatalf("default profile = %+v", d)
	}

	p, err := c.Add("work", []string{"skills", "CLAUDE.md", "agents"})
	if err != nil {
		t.Fatal(err)
	}
	if p.Dir != filepath.Join(home, ".claude-work") || len(p.Links) != 2 {
		t.Errorf("profile = %+v (agents does not exist, so 2 links)", p)
	}
	if target, _ := os.Readlink(filepath.Join(p.Dir, "skills")); target != filepath.Join(home, ".claude", "skills") {
		t.Errorf("skills link -> %q", target)
	}
	if _, err := c.Add("work", nil); err == nil {
		t.Error("duplicate profile accepted")
	}
	if _, err := c.Add("Bad Name", nil); err == nil {
		t.Error("invalid name accepted")
	}
	if _, err := c.Add("x", []string{"projects"}); err == nil {
		t.Error("sharing projects/ must be refused")
	}
	if _, err := os.Stat(filepath.Join(home, ".claude-x")); !os.IsNotExist(err) {
		t.Error("a refused profile left its dir behind")
	}

	if err := c.SetRule(filepath.Join(home, "work"), "work"); err != nil {
		t.Fatal(err)
	}
	c.SetRule(filepath.Join(home, "work", "oss"), Default)
	for dir, want := range map[string]string{
		filepath.Join(home, "work", "api"):        "work",
		filepath.Join(home, "work"):               "work",
		filepath.Join(home, "work", "oss", "lib"): Default,
		filepath.Join(home, "workshop"):           "",
		filepath.Join(home, "pet"):                "",
	} {
		if got := c.ForFolder(dir); got != want {
			t.Errorf("ForFolder(%s) = %q, want %q", dir, got, want)
		}
	}

	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	c2, err := Load(home, file)
	if err != nil || c2.Find("work") == nil || len(c2.Rules) != 2 {
		t.Fatalf("reloaded = %+v, %v", c2, err)
	}

	dir, err := c2.Remove("work")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(dir, "skills")); !os.IsNotExist(err) {
		t.Error("shared link not removed")
	}
	if _, err := os.Stat(dir); err != nil {
		t.Error("profile dir must stay: it holds the login")
	}
	if len(c2.Rules) != 1 || c2.Find("work") != nil {
		t.Errorf("after remove: %+v", c2)
	}
	if _, err := c2.Remove(Default); err == nil {
		t.Error("default profile removed")
	}
}
