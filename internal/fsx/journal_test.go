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

	c, err := j.Write(file, []byte("after\n"), Meta{Rule: "large-import", Title: "title"})
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

	c, err := j.Write(file, []byte(`{"a":1}`), Meta{Rule: "r", Title: "t"})
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

	c, err := j.Write(file, []byte("rule"), Meta{Rule: "r", Title: "t"})
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

func TestUndoBatch(t *testing.T) {
	dir := t.TempDir()
	j, err := OpenJournal(filepath.Join(dir, "backups"))
	if err != nil {
		t.Fatal(err)
	}
	a, b := filepath.Join(dir, "a.md"), filepath.Join(dir, "b.md")
	os.WriteFile(a, []byte("a\n"), 0o644)
	os.WriteFile(b, []byte("b\n"), 0o644)
	batch, _ := NewBatch()
	ca, _ := j.Write(a, []byte("a2\n"), Meta{Rule: "r", Batch: batch})
	if _, err := j.Write(b, []byte("b2\n"), Meta{Rule: "r", Batch: batch}); err != nil {
		t.Fatal(err)
	}

	// An edit to one file blocks undoing the whole batch, and nothing is restored.
	os.WriteFile(b, []byte("edited\n"), 0o644)
	if err := j.Undo(ca.ID); !errors.Is(err, ErrChanged) {
		t.Fatalf("err = %v, want ErrChanged", err)
	}
	if got, _ := os.ReadFile(a); string(got) != "a2\n" {
		t.Errorf("a was restored although the batch was refused: %q", got)
	}

	os.WriteFile(b, []byte("b2\n"), 0o644)
	if err := j.Undo(ca.ID); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{a: "a\n", b: "b\n"} {
		if got, _ := os.ReadFile(path); string(got) != want {
			t.Errorf("%s = %q, want %q", path, got, want)
		}
	}
}
