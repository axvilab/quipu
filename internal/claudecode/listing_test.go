package claudecode

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLastSkillListing(t *testing.T) {
	cfg := t.TempDir()
	proj := "/src/shop"
	dir := filepath.Join(cfg, "projects", ProjectDirName(proj))
	os.MkdirAll(dir, 0o755)
	line := func(initial bool, names []string, content string) string {
		b, _ := json.Marshal(map[string]any{"type": "attachment", "timestamp": "2026-10-12T10:00:00Z",
			"attachment": map[string]any{"type": "skill_listing", "isInitial": initial, "names": names, "content": content}})
		return string(b)
	}
	listing := line(true, []string{"ship", "loop", "pr-tools:review", "anthropic-skills:pdf"},
		"- ship: Commit and push\n- loop: Run a prompt\n  on an interval\n- pr-tools:review: Review a PR\n- anthropic-skills:pdf: Work with PDF files")
	old := filepath.Join(dir, "old.jsonl")
	os.WriteFile(old, []byte(listing+"\n"), 0o600)
	// The newest session has no full listing (only a later delta): the older one answers.
	recent := filepath.Join(dir, "new.jsonl")
	os.WriteFile(recent, []byte(`{"type":"user","message":{"content":"hi"}}`+"\n"+line(false, []string{"x"}, "- x: y")+"\n"), 0o600)
	os.Chtimes(old, time.Now().Add(-time.Hour), time.Now().Add(-time.Hour))

	l, ok := LastSkillListing(cfg, proj)
	if !ok || len(l.Skills) != 4 || l.At.IsZero() {
		t.Fatalf("listing = %+v, %v", l, ok)
	}
	if got := l.Skills["loop"]; !strings.Contains(got, "on an interval") {
		t.Errorf("a wrapped description = %q", got)
	}
	if got := l.Skills["pr-tools:review"]; got != "pr-tools:review: Review a PR" {
		t.Errorf("a plugin skill = %q", got)
	}

	for _, tt := range []struct {
		it   Item
		want string
	}{
		{Item{Kind: "skill", Scope: "user", Name: "ship"}, "ship"},
		{Item{Kind: "skill", Scope: "claude.ai", Name: "pdf"}, "anthropic-skills:pdf"},
		{Item{Kind: "skill", Scope: "plugin pr-tools", Name: "review"}, "pr-tools:review"},
		{Item{Kind: "rule", Scope: "project", Name: "db"}, ""},
	} {
		if got := ListedName(tt.it); got != tt.want {
			t.Errorf("ListedName(%+v) = %q, want %q", tt.it, got, tt.want)
		}
	}
}
