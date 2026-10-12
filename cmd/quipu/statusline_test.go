package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestStatusline(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "shop")
	os.MkdirAll(filepath.Join(repo, ".git"), 0o755)
	os.WriteFile(filepath.Join(repo, ".git", "HEAD"), []byte("ref: refs/heads/feature-1\n"), 0o644)
	os.MkdirAll(filepath.Join(repo, "src"), 0o755)

	now := time.Now()
	reset := now.Add(2*time.Hour + 10*time.Minute).Unix()
	warm := now.Add(4*time.Minute + 30*time.Second).Unix()
	miss := now.Add(-time.Minute).Unix()
	input := `{"model":{"display_name":"Opus"},"effort":{"level":"xhigh"},"fast_mode":true,"pr":{"number":12,"review_state":"approved"},"workspace":{"current_dir":"` + filepath.Join(repo, "src") + `"},
		"cost":{"total_cost_usd":0.4213},
		"context_window":{"used_percentage":34.4,"context_window_size":200000,
			"current_usage":{"input_tokens":500,"cache_creation_input_tokens":2500,"cache_read_input_tokens":65000}},
		"rate_limits":{"five_hour":{"used_percentage":23.5,"resets_at":` + strconv.FormatInt(reset, 10) + `},"seven_day":{"used_percentage":41.2,"resets_at":1}},
		"prompt_cache":{"warm":true,"expires_at":` + strconv.FormatInt(warm, 10) + `,"hit_ratio":0.91,
			"last_miss_at":` + strconv.FormatInt(miss, 10) + `,"last_miss_cause":{"causes":["tools_changed"]}}}`

	t.Setenv("HOME", t.TempDir()) // no profiles file: the profile shows by its name
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("NO_COLOR", "1")
	t.Setenv("CLAUDE_CONFIG_DIR", "/home/u/.claude-work")
	var out strings.Builder
	if err := runStatusline(strings.NewReader(input), &out); err != nil {
		t.Fatal(err)
	}
	line := strings.TrimRight(out.String(), "\n")
	if strings.Contains(line, "\n") {
		t.Fatalf("more than one line: %q", line)
	}
	if want := "work · Opus xhigh fast · src ⎇ feature-1 · PR #12 ✓ · $0.42 · ctx"; !strings.HasPrefix(line, want) {
		t.Errorf("line = %q, want it to start with %q", line, want)
	}
	for _, want := range []string{"ctx ▰▰▰▱▱▱▱▱ 34% 68K/200K", "5h ▰▱▱▱▱ 24% ↻", "7d ▰▰▱▱▱ 41%", "cache 91% ⏳4m miss: tools changed"} {
		if !strings.Contains(line, want) {
			t.Errorf("line %q has no %q", line, want)
		}
	}

	// Colours are on unless NO_COLOR is set; a nearly full context is red.
	t.Setenv("NO_COLOR", "")
	out.Reset()
	runStatusline(strings.NewReader(`{"context_window":{"used_percentage":92}}`), &out)
	if !strings.Contains(out.String(), "\x1b[31m") {
		t.Errorf("no red for 92%%: %q", out.String())
	}

	// Early in a session, or on a plan without limits: only what is there.
	t.Setenv("NO_COLOR", "1")
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	out.Reset()
	runStatusline(strings.NewReader(`{"model":{}}`), &out)
	if got := strings.TrimSpace(out.String()); got != "default" {
		t.Errorf("empty input = %q", got)
	}
}

func TestGitBranchWorktree(t *testing.T) {
	base := t.TempDir()
	gitDir := filepath.Join(base, "main", ".git", "worktrees", "wt")
	os.MkdirAll(gitDir, 0o755)
	os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/wt-branch\n"), 0o644)
	wt := filepath.Join(base, "wt")
	os.MkdirAll(wt, 0o755)
	os.WriteFile(filepath.Join(wt, ".git"), []byte("gitdir: "+gitDir+"\n"), 0o644)
	if got := gitBranch(wt); got != "wt-branch" {
		t.Errorf("worktree branch = %q", got)
	}
	if got := gitBranch(t.TempDir()); got != "" {
		t.Errorf("outside a repository = %q", got)
	}
}
