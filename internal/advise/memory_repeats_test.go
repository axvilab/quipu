package advise

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/axvilab/quipu/internal/claudecode"
)

func TestMemoryRepeats(t *testing.T) {
	home := t.TempDir()
	cfg := filepath.Join(home, ".claude")
	proj := filepath.Join(home, "src", "shop")
	os.MkdirAll(proj, 0o755)
	rule := "Never run migrate:fresh without DB_DATABASE=shop_testing set."
	os.WriteFile(filepath.Join(proj, "CLAUDE.md"), []byte("# shop\n\n"+rule+"\nUse the repository layer for every query.\n"), 0o644)

	mem := claudecode.MemoryDir(cfg, proj)
	os.MkdirAll(mem, 0o755)
	index := "- [Fresh migrations](fresh-db.md) — wiped the dev database once\n" +
		"- [Deploys](deploys.md) — deploys go through the release pipeline only\n" +
		"Use the repository layer for every query.\n"
	os.WriteFile(filepath.Join(mem, "MEMORY.md"), []byte(index), 0o644)
	os.WriteFile(filepath.Join(mem, "fresh-db.md"), []byte("---\nname: fresh-db\ndescription: a long description line that is not part of the body\n---\n\n"+rule+"\n"), 0o644)
	os.WriteFile(filepath.Join(mem, "deploys.md"), []byte("Deploys go through the release pipeline, never by hand.\n"), 0o644)

	items := claudecode.Instructions(home, cfg, proj)
	got := memoryRepeats(Input{Home: home, ConfigDir: cfg, Project: proj, Items: items})
	if len(got) != 2 {
		t.Fatalf("findings = %+v", got)
	}
	note, idx := got[0], got[1]
	if note.Params["state"] != "note" || note.Params["name"] != "fresh-db.md" || len(note.Edits) != 2 || !note.Edits[0].Delete {
		t.Fatalf("note finding = %+v", note)
	}
	after, err := note.Edits[1].Apply([]byte(index))
	if err != nil || strings.Contains(string(after), "fresh-db") || !strings.Contains(string(after), "deploys.md") {
		t.Errorf("MEMORY.md after the note fix = %q, %v", after, err)
	}
	if idx.Params["state"] != "index" || idx.Params["lines"] != "1" || idx.Saves == 0 {
		t.Fatalf("index finding = %+v", idx)
	}
	after, err = idx.Edits[0].Apply([]byte(index))
	if err != nil || strings.Contains(string(after), "repository layer") {
		t.Errorf("MEMORY.md after the index fix = %q, %v", after, err)
	}
}
