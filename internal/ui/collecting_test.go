package ui

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

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

func TestCollectSavesPlans(t *testing.T) {
	home := t.TempDir()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	data := t.TempDir()
	s, err := NewService(home, filepath.Join(home, ".claude"), "default", data, "test", st)
	if err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Join(home, ".claude"), 0o755)
	os.WriteFile(filepath.Join(home, ".claude.json"), []byte(`{"oauthAccount":{"organizationType":"claude_max","organizationRateLimitTier":"default_claude_max_20x"}}`), 0o600)
	s.ProfilesFile = filepath.Join(home, ".config", "quipu", "profiles.json")
	s.ClaudeBinary = filepath.Join(home, "bin", "claude")
	os.MkdirAll(filepath.Dir(s.ClaudeBinary), 0o755)
	os.WriteFile(s.ClaudeBinary, []byte("#!/bin/sh\necho '{\"loggedIn\":true,\"subscriptionType\":\"max\"}'\n"), 0o755)

	// One pass, without the UI ever asking for profiles: the status line still gets the plan.
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.Collect(ctx, time.Hour); close(done) }()
	defer func() { cancel(); <-done }()
	var b []byte
	for deadline := time.Now().Add(5 * time.Second); len(b) == 0 && time.Now().Before(deadline); {
		time.Sleep(20 * time.Millisecond)
		b, _ = os.ReadFile(PlansFile(data))
	}
	if want := `{"default":"max 20×"}`; string(b) != want {
		t.Errorf("plans.json = %s, want %s", b, want)
	}
}
