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

func TestSchemaUpgrade(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	src := Source{Path: "/t/a.jsonl", Profile: "default", ProjectDir: "-p", SessionID: "s-1", Kind: claudecode.KindSession}

	// A file written by schema 1: no plugins column, old name.
	old := filepath.Join(dir, "responses", oldFileName(src.Path))
	if _, err := s.db.ExecContext(ctx, "COPY (SELECT 'default' AS profile, '-p' AS project_dir, TIMESTAMP '2026-10-01 10:00:00' AS ts, 5::BIGINT AS cache_read_tokens) TO "+quote(old)+" (FORMAT parquet)"); err != nil {
		t.Fatal(err)
	}
	if s.UpToDate(src.Path, time.Time{}) {
		t.Error("a schema 1 file counts as up to date")
	}
	rows, err := s.Query(ctx, "SELECT count(*), count(plugins) FROM responses")
	if err != nil {
		t.Fatalf("query over schema 1 files only: %v", err)
	}
	var n, withPlugins int
	rows.Next()
	rows.Scan(&n, &withPlugins)
	rows.Close()
	if n != 1 || withPlugins != 0 {
		t.Errorf("rows %d, with plugins %d", n, withPlugins)
	}

	r := response("m1", 5, 0, nil)
	r.Plugins = []string{"frontend-design", "playwright"}
	if err := s.WriteResponses(ctx, src, []claudecode.Response{r}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Errorf("schema 1 file kept next to the new one: %v", err)
	}
	rows, _ = s.Query(ctx, "SELECT plugins FROM responses")
	var plugins string
	rows.Next()
	rows.Scan(&plugins)
	rows.Close()
	if plugins != "frontend-design,playwright" {
		t.Errorf("plugins = %q", plugins)
	}
}

func TestQueryConcurrent(t *testing.T) {
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	src := Source{Path: "/t/a.jsonl", Profile: "default", ProjectDir: "-p", SessionID: "s", Kind: claudecode.KindSession}
	if err := st.WriteResponses(t.Context(), src, []claudecode.Response{{MessageID: "m", SessionID: "s", Timestamp: time.Now()}}); err != nil {
		t.Fatal(err)
	}
	errs := make(chan error, 16)
	for range 16 {
		go func() {
			rows, err := st.Query(t.Context(), "SELECT count(*) FROM responses")
			if err == nil {
				rows.Close()
			}
			errs <- err
		}()
	}
	for range 16 {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
}
