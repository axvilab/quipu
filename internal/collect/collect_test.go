package collect

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/axvilab/quipu/internal/store"
)

func copyFixture(t *testing.T, dst string) {
	t.Helper()
	data, err := os.ReadFile("../claudecode/testdata/session.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRun(t *testing.T) {
	ctx := context.Background()
	home := t.TempDir()
	personal := filepath.Join(home, ".claude", "projects", "-srv-pet-demo", "s-1.jsonl")
	work := filepath.Join(home, ".claude-work", "projects", "-srv-work-api", "s-2.jsonl")
	copyFixture(t, personal)
	copyFixture(t, work)

	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	stats, err := Run(ctx, home, st)
	if err != nil {
		t.Fatal(err)
	}
	// The fixture has 3 responses and one truncated line.
	want := Stats{Profiles: 2, Seen: 2, Collected: 2, Responses: 6, Malformed: 2}
	if stats != want {
		t.Errorf("first run = %+v, want %+v", stats, want)
	}

	stats, err = Run(ctx, home, st)
	if err != nil {
		t.Fatal(err)
	}
	want = Stats{Profiles: 2, Seen: 2}
	if stats != want {
		t.Errorf("second run = %+v, want %+v (nothing changed)", stats, want)
	}

	// The work session gets new lines: only it is collected again.
	later := time.Now().Add(time.Minute)
	if err := os.Chtimes(work, later, later); err != nil {
		t.Fatal(err)
	}
	stats, err = Run(ctx, home, st)
	if err != nil {
		t.Fatal(err)
	}
	want = Stats{Profiles: 2, Seen: 2, Collected: 1, Responses: 3, Malformed: 1}
	if stats != want {
		t.Errorf("third run = %+v, want %+v", stats, want)
	}

	rows, err := st.Query(ctx, "SELECT profile, count(*) FROM responses GROUP BY profile ORDER BY profile")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got := map[string]int{}
	for rows.Next() {
		var p string
		var n int
		if err := rows.Scan(&p, &n); err != nil {
			t.Fatal(err)
		}
		got[p] = n
	}
	if got["default"] != 3 || got["work"] != 3 || len(got) != 2 {
		t.Errorf("responses by profile = %v, want default:3 work:3", got)
	}
}
