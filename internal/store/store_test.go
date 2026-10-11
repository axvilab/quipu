package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/axvilab/quipu/internal/claudecode"
)

func response(id string, read, creation int64, miss *claudecode.CacheMiss) claudecode.Response {
	r := claudecode.Response{
		MessageID: id,
		SessionID: "s-1",
		Timestamp: time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC),
		Model:     "claude-opus-5-5",
		CacheMiss: miss,
	}
	r.Usage.InputTokens = 2
	r.Usage.CacheReadTokens = read
	r.Usage.CacheCreationTokens = creation
	r.Usage.OutputTokens = 10
	return r
}

type totals struct {
	rows, read, creation, misses int64
}

func queryTotals(t *testing.T, s *Store) totals {
	t.Helper()
	rows, err := s.Query(context.Background(), `
		SELECT count(*), coalesce(sum(cache_read_tokens), 0), coalesce(sum(cache_creation_tokens), 0),
		       count(cache_miss_type)
		FROM responses`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got totals
	rows.Next()
	if err := rows.Scan(&got.rows, &got.read, &got.creation, &got.misses); err != nil {
		t.Fatal(err)
	}
	return got
}

func TestStore(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()
	s, err := Open(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if got := queryTotals(t, s); got != (totals{}) {
		t.Fatalf("empty store totals = %+v", got)
	}

	main := Source{Path: "/cfg/projects/p/s-1.jsonl", Profile: "default", ProjectDir: "p", SessionID: "s-1", Kind: claudecode.KindSession}
	agent := Source{Path: "/cfg/projects/p/s-1/subagents/agent-a.jsonl", Profile: "default", ProjectDir: "p", SessionID: "s-1", Kind: claudecode.KindSubagent}

	miss := &claudecode.CacheMiss{Type: "model_changed", MissedTokens: 500}
	if err := s.WriteResponses(ctx, main, []claudecode.Response{
		response("m1", 0, 1000, nil),
		response("m2", 1000, 100, miss),
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteResponses(ctx, agent, []claudecode.Response{response("a1", 300, 0, nil)}); err != nil {
		t.Fatal(err)
	}

	want := totals{rows: 3, read: 1300, creation: 1100, misses: 1}
	if got := queryTotals(t, s); got != want {
		t.Errorf("totals = %+v, want %+v", got, want)
	}

	// The transcript grew: rewriting the same source replaces its file instead of appending.
	if err := s.WriteResponses(ctx, main, []claudecode.Response{
		response("m1", 0, 1000, nil),
		response("m2", 1000, 100, miss),
		response("m3", 1100, 50, nil),
	}); err != nil {
		t.Fatal(err)
	}
	want = totals{rows: 4, read: 2400, creation: 1150, misses: 1}
	if got := queryTotals(t, s); got != want {
		t.Errorf("after rewrite totals = %+v, want %+v", got, want)
	}

	read := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	main.ModTime = read
	if err := s.WriteResponses(ctx, main, []claudecode.Response{response("m1", 0, 1000, nil)}); err != nil {
		t.Fatal(err)
	}
	if !s.UpToDate(main.Path, read) {
		t.Error("UpToDate = false for a transcript unchanged since it was read")
	}
	if s.UpToDate(main.Path, read.Add(time.Millisecond)) {
		t.Error("UpToDate = true for a transcript appended to after it was read")
	}
	if s.UpToDate("/cfg/projects/p/unknown.jsonl", time.Time{}) {
		t.Error("UpToDate = true for a transcript never stored")
	}

	files, _ := filepath.Glob(filepath.Join(dataDir, "responses", "*"))
	if len(files) != 2 {
		t.Fatalf("files = %v, want 2 parquet files and no leftovers", files)
	}
	for _, f := range files {
		fi, err := os.Stat(f)
		if err != nil {
			t.Fatal(err)
		}
		if fi.Mode().Perm() != 0o600 {
			t.Errorf("%s mode = %v, want 0600", f, fi.Mode().Perm())
		}
	}
}
