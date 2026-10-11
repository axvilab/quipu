package analytics

import (
	"context"
	"testing"
	"time"

	"github.com/axvilab/quipu/internal/claudecode"
	"github.com/axvilab/quipu/internal/store"
)

var day1 = time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)

func resp(id, session, cwd string, ts time.Time, read, creation int64, miss string) claudecode.Response {
	r := claudecode.Response{MessageID: id, SessionID: session, Cwd: cwd, Timestamp: ts}
	r.Usage.InputTokens = 0
	r.Usage.CacheReadTokens = read
	r.Usage.CacheCreationTokens = creation
	r.Usage.OutputTokens = 1
	if miss != "" {
		r.CacheMiss = &claudecode.CacheMiss{Type: miss, MissedTokens: creation}
	}
	return r
}

func testStore(t *testing.T) *store.Store {
	t.Helper()
	ctx := context.Background()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })

	write := func(path, project, session string, rs ...claudecode.Response) {
		src := store.Source{Path: path, Profile: "default", ProjectDir: project, SessionID: session, Kind: claudecode.KindSession}
		if err := st.WriteResponses(ctx, src, rs); err != nil {
			t.Fatal(err)
		}
	}
	// Project A: two sessions over two days; 900 of 1000 prompt tokens from cache.
	write("/a/s1.jsonl", "-srv-a", "s1",
		resp("a1", "s1", "/srv/a", day1, 0, 100, ""),
		resp("a2", "s1", "/srv/a/sub", day1.Add(time.Minute), 400, 0, ""))
	write("/a/s2.jsonl", "-srv-a", "s2",
		resp("a3", "s2", "/srv/a", day1.Add(24*time.Hour), 500, 0, ""))
	// Project B: one session, half the tokens missed the cache.
	write("/b/s3.jsonl", "-srv-b", "s3",
		resp("b1", "s3", "/srv/b", day1, 100, 100, "model_changed"),
		resp("b2", "s3", "/srv/b", day1.Add(time.Hour), 0, 50, "tools_changed"),
		resp("b3", "s3", "/srv/b", day1.Add(2*time.Hour), 0, 10, "model_changed"))
	// Old response outside the default window.
	write("/b/old.jsonl", "-srv-b", "old", resp("o1", "old", "/srv/b", day1.AddDate(0, -2, 0), 1, 1, ""))
	return st
}

func TestByProject(t *testing.T) {
	st := testStore(t)
	got, err := ByProject(context.Background(), st, Filter{Since: day1.AddDate(0, 0, -1)})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d projects, want 2: %+v", len(got), got)
	}
	a, b := got[0], got[1]
	if a.ProjectDir != "-srv-a" || a.Path != "/srv/a" || a.Sessions != 2 || a.Requests != 3 {
		t.Errorf("project a = %+v", a)
	}
	if a.Prompt() != 1000 || a.HitRate() != 0.9 {
		t.Errorf("project a prompt %d hit %.2f, want 1000 and 0.90", a.Prompt(), a.HitRate())
	}
	if b.ProjectDir != "-srv-b" || b.Misses != 3 || b.Prompt() != 260 {
		t.Errorf("project b = %+v (old session must be excluded)", b)
	}
}

func TestByDay(t *testing.T) {
	st := testStore(t)
	got, err := ByDay(context.Background(), st, Filter{Since: day1.AddDate(0, 0, -1), ProjectDir: "-srv-a"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d days, want 2: %+v", len(got), got)
	}
	if !got[0].Day.Equal(day1.Truncate(24*time.Hour)) || got[0].Requests != 2 || got[1].Requests != 1 {
		t.Errorf("days = %+v", got)
	}
}

func TestMisses(t *testing.T) {
	st := testStore(t)
	got, err := Misses(context.Background(), st, Filter{Since: day1.AddDate(0, 0, -1)})
	if err != nil {
		t.Fatal(err)
	}
	want := []MissRow{{"model_changed", 2, 110}, {"tools_changed", 1, 50}}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("misses = %+v, want %+v", got, want)
	}
}

func TestSessionStart(t *testing.T) {
	st := testStore(t)
	got, err := SessionStart(context.Background(), st, Filter{Since: day1.AddDate(0, 0, -1), ProjectDir: "-srv-a"})
	if err != nil {
		t.Fatal(err)
	}
	// Session s1 starts with 100 tokens, s2 (the latest) with 500.
	want := StartWeight{Sessions: 2, Median: 300, Min: 100, Max: 500, Latest: 500, LatestAt: day1.Add(24 * time.Hour)}
	if got != want {
		t.Errorf("start weight = %+v, want %+v", got, want)
	}
}

func TestEstimateTokens(t *testing.T) {
	if got := EstimateTokens("abcdefgh"); got != 2 {
		t.Errorf("ascii estimate = %d, want 2", got)
	}
	if got := EstimateTokens("привіт"); got != 4 {
		t.Errorf("cyrillic estimate = %d, want 4", got)
	}
}

func TestTotalAndPrevious(t *testing.T) {
	st := testStore(t)
	f := Filter{Since: day1.AddDate(0, 0, -1), Until: day1.AddDate(0, 0, 2)}
	got, err := Total(context.Background(), st, f)
	if err != nil {
		t.Fatal(err)
	}
	if got.Requests != 6 || got.Sessions != 3 {
		t.Errorf("total = %+v, want 6 requests in 3 sessions", got)
	}
	prev := f.Previous()
	if !prev.Until.Equal(f.Since) || !prev.Since.Equal(f.Since.AddDate(0, 0, -3)) {
		t.Errorf("previous = %+v", prev)
	}
	old, _ := Total(context.Background(), st, Filter{Since: day1.AddDate(0, -3, 0), Until: day1.AddDate(0, -1, 0)})
	if old.Requests != 1 {
		t.Errorf("old window = %+v, want the one old response", old)
	}
}

func TestEarliest(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	tests := []struct {
		project string
		want    time.Time
	}{
		{"", day1.AddDate(0, -2, 0)},
		{"-srv-a", day1},
		{"-srv-none", time.Time{}},
	}
	for _, tt := range tests {
		got, err := Earliest(ctx, st, Filter{ProjectDir: tt.project})
		if err != nil {
			t.Fatal(err)
		}
		if !got.Equal(tt.want) {
			t.Errorf("Earliest(%q) = %v, want %v", tt.project, got, tt.want)
		}
	}
}
