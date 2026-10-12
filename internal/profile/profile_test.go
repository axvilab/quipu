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

func TestRememberFolder(t *testing.T) {
	home := t.TempDir()
	// t.TempDir is under the temp folder, which RememberFolder skips; point TMPDIR elsewhere.
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "tmp"))
	repo := filepath.Join(home, "work", "shop")
	os.MkdirAll(filepath.Join(repo, ".git"), 0o755)
	os.MkdirAll(filepath.Join(repo, "src", "api"), 0o755)
	plain := filepath.Join(home, "notes")
	os.MkdirAll(plain, 0o755)

	for _, c := range []struct{ dir, want string }{
		{filepath.Join(repo, "src", "api"), repo}, // the repository, not the subfolder
		{repo, repo},
		{plain, plain},
		{home, ""},
		{filepath.Dir(home), ""},
		{"/", ""},
		{os.Getenv("TMPDIR"), ""},
		{filepath.Join(os.Getenv("TMPDIR"), "x"), ""},
	} {
		if got := RememberFolder(c.dir, home); got != c.want {
			t.Errorf("RememberFolder(%s) = %q, want %q", c.dir, got, c.want)
		}
	}
}

func TestRuleFor(t *testing.T) {
	c := &Config{Profiles: []Profile{{Name: "default"}, {Name: "work"}}}
	c.Rules = []Rule{{Path: "/w", Profile: "default"}, {Path: "/w/shop", Profile: "work"}}
	if r, ok := c.RuleFor("/w/shop/src"); !ok || r.Path != "/w/shop" || r.Profile != "work" {
		t.Errorf("RuleFor = %+v %v", r, ok)
	}
	if _, ok := c.RuleFor("/elsewhere"); ok {
		t.Error("a rule for an unrelated folder")
	}
}
