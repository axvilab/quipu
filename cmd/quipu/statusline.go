package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/axvilab/quipu/internal/claudecode"
)

// runStatusline prints the status line Claude Code shows under the prompt: the profile, context
// use, the 5-hour and 7-day limits with the next reset, and the cache hit ratio. Claude Code runs
// it after every reply with the session's numbers on stdin, so it needs no store, network or login.
func runStatusline(in io.Reader, out io.Writer) error {
	s, err := claudecode.ReadStatusInput(in)
	if err != nil {
		return err
	}
	profile := "default"
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		profile = claudecode.ProfileName(dir)
	}
	fmt.Fprintln(out, statusLine(profile, s, time.Now()))
	return nil
}

func statusLine(profile string, s claudecode.StatusInput, now time.Time) string {
	parts := []string{profile}
	if p := s.ContextWindow.UsedPercentage; p != nil {
		parts = append(parts, fmt.Sprintf("ctx %.0f%%", *p))
	}
	if w := s.RateLimits.FiveHour; w != nil {
		parts = append(parts, fmt.Sprintf("5h %.0f%%%s", w.UsedPercentage, resetsIn(w.ResetsAt, now)))
	}
	if w := s.RateLimits.SevenDay; w != nil {
		parts = append(parts, fmt.Sprintf("7d %.0f%%", w.UsedPercentage))
	}
	if c := s.PromptCache; c != nil && c.HitRatio != nil {
		parts = append(parts, fmt.Sprintf("cache %.0f%%", *c.HitRatio*100))
	}
	return strings.Join(parts, " · ")
}

// resetsIn says when a window resets, as a local time today or a duration beyond that.
func resetsIn(at int64, now time.Time) string {
	if at <= 0 {
		return ""
	}
	t := time.Unix(at, 0)
	if !t.After(now) {
		return ""
	}
	if t.Sub(now) < 24*time.Hour {
		return " ↻" + t.Format("15:04")
	}
	return " ↻" + t.Format("Mon 15:04")
}
