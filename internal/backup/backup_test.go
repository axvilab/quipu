package backup

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, path, text string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), mode); err != nil {
		t.Fatal(err)
	}
}

// fakeHome builds a home with two profiles; work shares CLAUDE.md through a symlink.
func fakeHome(t *testing.T) string {
	t.Helper()
	home := filepath.Join(t.TempDir(), "alice")
	cfg := filepath.Join(home, ".claude")
	write(t, filepath.Join(cfg, "CLAUDE.md"), "# Me\n", 0o644)
	// Built at run time, so the source holds no string that looks like a real key.
	key := "sk-ant" + "-api03-" + strings.Repeat("x1", 14)
	write(t, filepath.Join(cfg, "settings.json"), `{"env": {"ANTHROPIC_API_KEY": "`+key+`"}}`, 0o644)
	write(t, filepath.Join(cfg, "rules", "go.md"), "Go rules\n", 0o644)
	write(t, filepath.Join(cfg, "skills", "release", "SKILL.md"), "---\nname: release\n---\n", 0o644)
	write(t, filepath.Join(cfg, "skills", "release", "scripts", "tag.sh"), "git tag\n", 0o755)
	write(t, filepath.Join(cfg, "skills", "synced", "acc", "pptx", "SKILL.md"), "synced\n", 0o644)
	mem := filepath.Join(cfg, "projects", "-alice-src-api", "memory", "MEMORY.md")
	write(t, strings.Replace(mem, "-alice-src-api", projectDir(home, "src/api"), 1), "- note\n", 0o644)
	write(t, filepath.Join(cfg, "projects", projectDir(home, "src/api"), "session.jsonl"), "{}\n", 0o644)
	write(t, filepath.Join(cfg, ".credentials.json"), `{"token":"secret"}`, 0o600)
	work := filepath.Join(home, ".claude-work")
	os.MkdirAll(filepath.Join(work, "projects"), 0o755)
	if err := os.Symlink(filepath.Join(cfg, "CLAUDE.md"), filepath.Join(work, "CLAUDE.md")); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(home, ".config", "quipu", "profiles.json"), `{"profiles":["work"]}`, 0o644)
	return home
}

func projectDir(home, rel string) string {
	var b strings.Builder
	for _, r := range filepath.Join(home, rel) {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	return b.String()
}

func TestCollectAndScan(t *testing.T) {
	home := fakeHome(t)
	m, entries, err := Collect(home, filepath.Join(home, ".config", "quipu", "profiles.json"), "test", nil)
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, e := range entries {
		paths = append(paths, e.Path)
	}
	got := strings.Join(paths, "\n")
	for _, want := range []string{"claude/.claude/CLAUDE.md", "claude/.claude/settings.json", "claude/.claude/rules/go.md",
		"claude/.claude/skills/release/scripts/tag.sh", "memory/MEMORY.md", "quipu/profiles.json"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s in:\n%s", want, got)
		}
	}
	for _, never := range []string{"synced", "session.jsonl", "credentials"} {
		if strings.Contains(got, never) {
			t.Errorf("%s must not be backed up:\n%s", never, got)
		}
	}
	if len(m.Links) != 1 || m.Links[0].Path != "claude/.claude-work/CLAUDE.md" {
		t.Errorf("links = %+v", m.Links)
	}
	secrets := Scan(entries)
	if len(secrets) != 1 || secrets[0].Path != "claude/.claude/settings.json" || secrets[0].Kind != "anthropic-key" {
		t.Errorf("secrets = %+v", secrets)
	}
}

func TestArchiveRestoreOnAnotherHome(t *testing.T) {
	home := fakeHome(t)
	m, entries, _ := Collect(home, filepath.Join(home, ".config", "quipu", "profiles.json"), "test", nil)
	var buf bytes.Buffer
	if err := WriteArchive(&buf, m, entries); err != nil {
		t.Fatal(err)
	}
	m2, entries2, err := ReadArchive(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries2) != len(entries) || m2.Home != home {
		t.Fatalf("round trip: %d entries (want %d), home %q", len(entries2), len(entries), m2.Home)
	}

	// A new machine where the user is called bob.
	newHome := filepath.Join(t.TempDir(), "bob")
	profiles := filepath.Join(newHome, ".config", "quipu", "profiles.json")
	plan := PlanRestore(m2, entries2, newHome, profiles)
	if created, changed := plan.Count(); created != len(entries) || changed != 0 {
		t.Errorf("created %d changed %d, want %d new", created, changed, len(entries))
	}
	wantMem := filepath.Join(newHome, ".claude", "projects", projectDir(newHome, "src/api"), "memory", "MEMORY.md")
	var found bool
	for _, f := range plan.Files {
		if !strings.HasPrefix(f.Path, newHome) {
			t.Errorf("file outside the new home: %s", f.Path)
		}
		found = found || f.Path == wantMem
		os.MkdirAll(filepath.Dir(f.Path), 0o755)
		os.WriteFile(f.Path, f.Data, 0o644)
	}
	if !found {
		t.Errorf("memory not moved to the new home's project folder %s", wantMem)
	}
	if len(plan.Links) != 1 || plan.Links[0].Target != filepath.Join(newHome, ".claude", "CLAUDE.md") || plan.Links[0].State != "new" {
		t.Errorf("links = %+v", plan.Links)
	}
	again := PlanRestore(m2, entries2, newHome, profiles)
	if created, changed := again.Count(); created != 0 || changed != 0 {
		t.Errorf("after restoring: created %d changed %d, want nothing", created, changed)
	}
}

func TestReadArchiveRejectsEscapes(t *testing.T) {
	m := Manifest{Home: "/home/x"}
	entries := []Entry{{Path: "claude/.claude/../../.ssh/authorized_keys", Data: []byte("key")}, {Path: "claude/other/x", Data: []byte("x")}}
	var buf bytes.Buffer
	WriteArchive(&buf, m, entries)
	m2, e2, err := ReadArchive(&buf)
	if err != nil {
		t.Fatal(err)
	}
	plan := PlanRestore(m2, e2, t.TempDir(), "/tmp/profiles.json")
	if len(plan.Files) != 0 || len(plan.Skipped) != 2 {
		t.Errorf("unsafe paths planned: %+v", plan)
	}
}

func TestGitWithLocalRemote(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	ctx := context.Background()
	home := fakeHome(t)
	profiles := filepath.Join(home, ".config", "quipu", "profiles.json")
	remote := filepath.Join(t.TempDir(), "remote.git")
	if out, err := exec.Command("git", "init", "-q", "--bare", remote).CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	repo := filepath.Join(t.TempDir(), "backup")
	if out, err := exec.Command("git", "clone", "-q", remote, repo).CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}

	// Without quipu profiles there is no quipu/ folder; saving must still work.
	bare := filepath.Join(t.TempDir(), "no-profiles")
	m, entries, _ := Collect(home, filepath.Join(home, "missing.json"), "test", nil)
	if _, err := SaveToGit(ctx, bare, m, entries, false); err != nil {
		t.Fatalf("save without profiles: %v", err)
	}

	m, entries, _ = Collect(home, profiles, "test", nil)
	changed, err := SaveToGit(ctx, repo, m, entries, true)
	if err != nil || !changed {
		t.Fatalf("first save: %v, changed %v", err, changed)
	}
	// Nothing new: no commit, the manifest is left as it was.
	m, entries, _ = Collect(home, profiles, "test", nil)
	if changed, err := SaveToGit(ctx, repo, m, entries, true); err != nil || changed {
		t.Fatalf("second save: %v, changed %v", err, changed)
	}
	// A removed rule disappears from the repository on the next save.
	os.Remove(filepath.Join(home, ".claude", "rules", "go.md"))
	m, entries, _ = Collect(home, profiles, "test", nil)
	if changed, err := SaveToGit(ctx, repo, m, entries, true); err != nil || !changed {
		t.Fatalf("third save: %v, changed %v", err, changed)
	}

	// Restore on a new machine from a fresh clone of the remote.
	clone := filepath.Join(t.TempDir(), "restore")
	if out, err := exec.Command("git", "clone", "-q", remote, clone).CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	m2, e2, err := ReadDir(clone)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range e2 {
		if strings.HasSuffix(e.Path, "rules/go.md") {
			t.Errorf("removed rule still in the repository")
		}
	}
	if len(e2) != len(entries) || m2.Home != home {
		t.Errorf("clone has %d entries, want %d", len(e2), len(entries))
	}
	log, _ := exec.Command("git", "-C", clone, "log", "--oneline").Output()
	if n := strings.Count(string(log), "\n"); n != 2 {
		t.Errorf("%d commits, want 2:\n%s", n, log)
	}
}

func TestCollectExcludeAndProfilesWithoutSessions(t *testing.T) {
	home := t.TempDir()
	for _, p := range []string{"-srv-shop", "-srv-secret"} {
		mem := filepath.Join(home, ".claude", "projects", p, "memory")
		os.MkdirAll(mem, 0o755)
		os.WriteFile(filepath.Join(mem, "MEMORY.md"), []byte("note\n"), 0o644)
	}
	// A profile with no session yet: no projects folder, only its settings.
	work := filepath.Join(home, ".claude-work")
	os.MkdirAll(work, 0o755)
	os.WriteFile(filepath.Join(work, "settings.json"), []byte(`{"theme":"dark"}`), 0o644)
	profiles := filepath.Join(home, "profiles.json")
	os.WriteFile(profiles, []byte(`{"profiles":[{"name":"work","dir":"`+work+`"}]}`), 0o644)

	_, entries, err := Collect(home, profiles, "test", map[string]bool{"-srv-secret": true})
	if err != nil {
		t.Fatal(err)
	}
	paths := map[string]bool{}
	for _, e := range entries {
		paths[e.Path] = true
	}
	if !paths["claude/.claude/projects/-srv-shop/memory/MEMORY.md"] {
		t.Errorf("shop memory missing: %v", paths)
	}
	if paths["claude/.claude/projects/-srv-secret/memory/MEMORY.md"] {
		t.Error("an excluded project's memory was backed up")
	}
	if !paths["claude/.claude-work/settings.json"] {
		t.Errorf("a profile without sessions was left out: %v", paths)
	}

	memories, err := ProjectMemories(home, profiles)
	if err != nil || len(memories) != 2 || memories[1].Dir != "-srv-shop" || memories[1].Files != 1 || memories[1].Profiles[0] != "default" {
		t.Errorf("memories = %+v, %v", memories, err)
	}

	dir := t.TempDir()
	if err := (Settings{Exclude: []string{"-srv-secret"}}).Save(dir); err != nil {
		t.Fatal(err)
	}
	if !LoadSettings(dir).Excluded()["-srv-secret"] {
		t.Error("settings did not keep the exclusion")
	}
}
