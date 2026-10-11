package advise

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/axvilab/quipu/internal/claudecode"
)

// dupHome writes a fake home: a user CLAUDE.md, three projects sharing a block, two of them
// with the same skill. It returns the input for the first project.
func dupHome(t *testing.T) (Input, map[string]string) {
	t.Helper()
	home := t.TempDir()
	cfg := filepath.Join(home, ".claude")
	write := func(path, text string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(cfg, "CLAUDE.md"), "# Me\n\n- Write commit messages as Conventional Commits.\n")

	shared := "## Testing\n\n- Run `make test` before every commit, no exceptions.\n- Keep each test next to the code it covers.\n"
	projects := map[string]string{}
	for _, name := range []string{"api", "web", "cli"} {
		p := filepath.Join(home, "src", name)
		projects[name] = p
		write(filepath.Join(p, "CLAUDE.md"), "# "+name+"\n\n"+shared)
	}
	// api also repeats a line of the user's CLAUDE.md, with different spacing.
	write(filepath.Join(projects["api"], "CLAUDE.md"),
		"# api\n\n-  Write commit messages as Conventional Commits.\n\n"+shared)
	skill := "---\nname: release\ndescription: Cut a release\n---\nTag and push.\n"
	write(filepath.Join(projects["api"], ".claude", "skills", "release", "SKILL.md"), skill)
	write(filepath.Join(projects["web"], ".claude", "skills", "release", "SKILL.md"), skill)

	all := map[string][]claudecode.Item{}
	for _, p := range projects {
		all[p] = claudecode.Instructions(home, cfg, p)
	}
	api := projects["api"]
	return Input{Home: home, ConfigDir: cfg, Project: api, Items: all[api], Projects: all}, projects
}

func TestRepeatedInSession(t *testing.T) {
	in, projects := dupHome(t)
	got := repeatedInSession(in)
	if len(got) != 1 {
		t.Fatalf("findings = %+v", got)
	}
	f := got[0]
	file := filepath.Join(projects["api"], "CLAUDE.md")
	if f.Path != file || f.Params["lines"] != "1" || f.Saves == 0 || len(f.Edits) != 1 {
		t.Errorf("finding = %+v", f)
	}
	before, _ := os.ReadFile(file)
	after, err := f.Edits[0].Apply(before)
	if err != nil || strings.Contains(string(after), "Conventional") || !strings.Contains(string(after), "make test") {
		t.Errorf("after fix: %q, %v", after, err)
	}
}

func TestSharedBlock(t *testing.T) {
	in, projects := dupHome(t)
	got := sharedBlock(in)
	if len(got) != 1 {
		t.Fatalf("findings = %+v", got)
	}
	f := got[0]
	if f.Path != claudecode.UserClaudeMD(in.ConfigDir) || f.Params["lines"] != "2" || f.Params["projects"] != "3" {
		t.Errorf("finding = %+v", f)
	}
	// One append to the user's CLAUDE.md, one removal per project; the section heading moves too.
	if len(f.Edits) != 4 || !strings.HasPrefix(f.Edits[0].Append, "## Testing\n") {
		t.Fatalf("edits = %+v", f.Edits)
	}
	file := filepath.Join(projects["web"], "CLAUDE.md")
	before, _ := os.ReadFile(file)
	for _, e := range f.Edits[1:] {
		if e.Path != file {
			continue
		}
		after, err := e.Apply(before)
		if err != nil || string(after) != "# web\n\n" {
			t.Errorf("web after the move = %q, %v", after, err)
		}
	}

	// Every project reports the same finding, so the UI shows it once.
	in.Project = projects["web"]
	if other := sharedBlock(in); len(other) != 1 || other[0].ID() != f.ID() {
		t.Errorf("web reports %+v, want the same finding", other)
	}

	// Two projects are not enough.
	delete(in.Projects, projects["cli"])
	if got := sharedBlock(in); len(got) != 0 {
		t.Errorf("two copies flagged: %+v", got)
	}
}

func TestCopiedExtension(t *testing.T) {
	in, projects := dupHome(t)
	// A script next to SKILL.md makes the skill a folder, moved as a whole.
	for _, p := range []string{projects["api"], projects["web"]} {
		os.MkdirAll(filepath.Join(p, ".claude", "skills", "release", "scripts"), 0o755)
		os.WriteFile(filepath.Join(p, ".claude", "skills", "release", "scripts", "tag.sh"), []byte("git tag\n"), 0o755)
	}
	got := copiedExtension(in)
	if len(got) != 1 || got[0].Params["name"] != "release" || got[0].Params["copies"] != "2" {
		t.Fatalf("findings = %+v", got)
	}
	f := got[0]
	target := filepath.Join(in.ConfigDir, "skills", "release")
	// Two files created under the user's skills, four deleted (two per copy).
	var creates, deletes int
	for _, e := range f.Edits {
		switch {
		case e.Create != "" && strings.HasPrefix(e.Path, target+string(filepath.Separator)):
			creates++
		case e.Delete && e.Stop == filepath.Dir(filepath.Dir(e.Path)) || e.Delete && strings.HasSuffix(e.Stop, "skills"):
			deletes++
		}
	}
	if f.Path != target || creates != 2 || deletes != 4 {
		t.Errorf("path %s, %d creates, %d deletes; edits = %+v", f.Path, creates, deletes, f.Edits)
	}

	in.Project = projects["cli"] // cli has no copy
	if got := copiedExtension(in); len(got) != 0 {
		t.Errorf("cli flagged: %+v", got)
	}

	// With a skill of that name in the user's settings, nothing is moved automatically.
	os.MkdirAll(target, 0o755)
	os.WriteFile(filepath.Join(target, "SKILL.md"), []byte("---\nname: release\n---\nOther.\n"), 0o644)
	in.Project = projects["api"]
	if got := copiedExtension(in); len(got) != 1 || len(got[0].Edits) != 0 || got[0].Params["state"] != "exists" {
		t.Errorf("with a user skill of that name: %+v", got)
	}
}

func TestEditCreateDelete(t *testing.T) {
	if got, err := (Edit{Create: "x\n"}).Apply(nil); err != nil || string(got) != "x\n" {
		t.Errorf("create = %q, %v", got, err)
	}
	if _, err := (Edit{Create: "x\n"}).Apply([]byte("already here\n")); !errors.Is(err, ErrStale) {
		t.Errorf("create over a file: %v, want ErrStale", err)
	}
	if _, err := (Edit{Delete: true, Old: "a\n"}).Apply([]byte("a\n")); err != nil {
		t.Errorf("delete: %v", err)
	}
	if _, err := (Edit{Delete: true, Old: "a\n"}).Apply([]byte("edited\n")); !errors.Is(err, ErrStale) {
		t.Errorf("delete of an edited file: %v, want ErrStale", err)
	}
}

func TestEditAppend(t *testing.T) {
	e := Edit{Append: "- one\n"}
	for _, tc := range []struct{ in, want string }{
		{"", "- one\n"},
		{"# Me\n", "# Me\n\n- one\n"},
		{"# Me\n\n\n", "# Me\n\n- one\n"},
	} {
		if got, err := e.Apply([]byte(tc.in)); err != nil || string(got) != tc.want {
			t.Errorf("Apply(%q) = %q, %v; want %q", tc.in, got, err, tc.want)
		}
	}
}
