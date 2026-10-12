package main

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestStatusline(t *testing.T) {
	now := time.Date(2026, 10, 12, 10, 0, 0, 0, time.Local)
	reset := now.Add(2*time.Hour + 10*time.Minute).Unix()
	input := `{"model":{"display_name":"Opus"},"context_window":{"used_percentage":34.4},
		"rate_limits":{"five_hour":{"used_percentage":23.5,"resets_at":` + itoa(reset) + `},"seven_day":{"used_percentage":41.2,"resets_at":1}},
		"prompt_cache":{"hit_ratio":0.91}}`
	var out strings.Builder
	t.Setenv("CLAUDE_CONFIG_DIR", "/home/u/.claude-work")
	if err := runStatusline(strings.NewReader(input), &out); err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(out.String())
	for _, want := range []string{"work", "ctx 34%", "5h 24%", "7d 41%", "cache 91%"} {
		if !strings.Contains(got, want) {
			t.Errorf("%q has no %q", got, want)
		}
	}

	// Early in a session, or on a plan without limits: only what is there.
	out.Reset()
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	runStatusline(strings.NewReader(`{"model":{}}`), &out)
	if got := strings.TrimSpace(out.String()); got != "default" {
		t.Errorf("empty input = %q", got)
	}
	if got := resetsIn(reset, now); got != " ↻12:10" {
		t.Errorf("resetsIn = %q", got)
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
