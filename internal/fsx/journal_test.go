package fsx

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestJournal(t *testing.T) {
	dir := t.TempDir()
	j, err := OpenJournal(filepath.Join(dir, "backups"))
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "CLAUDE.md")
	os.WriteFile(file, []byte("before\n"), 0o640)

	c, err := j.Write(file, []byte("after\n"), "large-import", "title")
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(file); string(b) != "after\n" {
		t.Fatalf("file = %q", b)
	}
	if fi, _ := os.Stat(file); fi.Mode().Perm() != 0o640 {
		t.Errorf("mode = %v, want 0640 kept", fi.Mode().Perm())
	}

	list, _ := j.List()
	if len(list) != 1 || list[0].ID != c.ID || !list[0].Existed {
		t.Fatalf("list = %+v", list)
	}

	if err := j.Undo(c.ID); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(file); string(b) != "before\n" {
		t.Errorf("after undo file = %q", b)
	}
	if list, _ := j.List(); !list[0].Undone {
		t.Error("change not marked undone")
	}
}

func TestUndoRefusesWhenFileChanged(t *testing.T) {
	dir := t.TempDir()
	j, _ := OpenJournal(filepath.Join(dir, "backups"))
	file := filepath.Join(dir, "settings.json")
	os.WriteFile(file, []byte("{}"), 0o644)

	c, err := j.Write(file, []byte(`{"a":1}`), "r", "t")
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(file, []byte(`{"a":1,"user":"edit"}`), 0o644)
	if err := j.Undo(c.ID); !errors.Is(err, ErrChanged) {
		t.Fatalf("undo after a later edit: err = %v, want ErrChanged", err)
	}
	if b, _ := os.ReadFile(file); string(b) != `{"a":1,"user":"edit"}` {
		t.Errorf("the later edit was lost: %q", b)
	}
}

func TestUndoNewFile(t *testing.T) {
	dir := t.TempDir()
	j, _ := OpenJournal(filepath.Join(dir, "backups"))
	file := filepath.Join(dir, "new", "rules.md")

	c, err := j.Write(file, []byte("rule"), "r", "t")
	if err != nil {
		t.Fatal(err)
	}
	if err := j.Undo(c.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(file); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("created file still exists after undo: %v", err)
	}
	if err := j.Undo("../escape"); err == nil {
		t.Error("path traversal id accepted")
	}
}
