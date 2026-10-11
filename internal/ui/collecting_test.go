package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/axvilab/quipu/internal/store"
)

func TestCollection(t *testing.T) {
	home := t.TempDir()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	s, err := NewService(home, filepath.Join(home, ".claude"), "default", t.TempDir(), "test", st)
	if err != nil {
		t.Fatal(err)
	}
	if c := s.Collection(); !c.First || c.Running {
		t.Fatalf("before any run: %+v", c)
	}

	data, err := os.ReadFile("../claudecode/testdata/session.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	transcript := filepath.Join(home, ".claude", "projects", "-srv-demo", "s-1.jsonl")
	os.MkdirAll(filepath.Dir(transcript), 0o755)
	os.WriteFile(transcript, data, 0o644)

	// During the first run the page keeps waiting even after the first transcript is stored.
	s.coll.mu.Lock()
	s.coll.empty, s.coll.state.Running = true, true
	s.coll.mu.Unlock()
	if c := s.Collection(); !c.First {
		t.Fatalf("during the first run: %+v", c)
	}
	s.coll.mu.Lock()
	s.coll.state.Running = false
	s.coll.mu.Unlock()

	s.collectOnce(t.Context())
	if c := s.Collection(); c.First || c.Running || c.Generation != 1 || c.Done != 1 || c.Total != 1 {
		t.Fatalf("after a run: %+v", c)
	}
	// Nothing new: the generation stays, so pages do not reload for nothing.
	s.collectOnce(t.Context())
	if c := s.Collection(); c.Generation != 1 {
		t.Fatalf("after an empty run: %+v", c)
	}
}
