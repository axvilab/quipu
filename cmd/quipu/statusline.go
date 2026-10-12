package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/axvilab/quipu/internal/claudecode"
	"github.com/axvilab/quipu/internal/profile"
	"github.com/axvilab/quipu/internal/ui"
)

// runStatusline prints the status line Claude Code shows under the prompt, in one line: who and
// where (profile, model, folder and git branch, session cost), then what is used (context, the
// 5-hour and 7-day limits, the prompt cache). Claude Code runs it after every reply with the
// session's numbers on stdin, so it reads no store, network or login.
func runStatusline(in io.Reader, out io.Writer) error {
	s, err := claudecode.ReadStatusInput(in)
	if err != nil {
		return err
	}
	profile := "default"
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		profile = claudecode.ProfileName(dir)
	}
	plan := profilePlan(profile)
	profile = profileTitle(profile)
	st := style{color: os.Getenv("NO_COLOR") == ""}
	line := statusTop(profile, plan, s, st)
	if usage := statusBottom(s, time.Now(), st); usage != "" {
		line += st.dim(" · ") + usage
	}
	fmt.Fprintln(out, line)
	return nil
}

// style paints text with ANSI colours, or leaves it plain when NO_COLOR is set.
type style struct{ color bool }

func (st style) paint(code, text string) string {
	if !st.color || text == "" {
		return text
	}
	return "\x1b[" + code + "m" + text + "\x1b[0m"
}

func (st style) dim(text string) string  { return st.paint("2", text) }
func (st style) bold(text string) string { return st.paint("1", text) }

// level colours a share by how close it is to the limit: green, yellow, red.
func (st style) level(pct float64, text string) string {
	switch {
	case pct >= 85:
		return st.paint("31", text)
	case pct >= 60:
		return st.paint("33", text)
	}
	return st.paint("32", text)
}

func statusTop(profile, plan string, s claudecode.StatusInput, st style) string {
	who := st.bold(profile)
	if plan != "" {
		who += " " + st.dim(plan)
	}
	parts := []string{who}
	if m := s.Model.DisplayName; m != "" {
		parts = append(parts, m)
	}
	if dir := s.Workspace.CurrentDir; dir != "" {
		where := filepath.Base(dir)
		if b := gitBranch(dir); b != "" {
			where += " " + st.dim("⎇") + " " + b
		}
		parts = append(parts, where)
	}
	if c := s.Cost.TotalCostUSD; c != nil && *c > 0 {
		parts = append(parts, st.dim(fmt.Sprintf("$%.2f", *c)))
	}
	return strings.Join(parts, st.dim(" · "))
}

func statusBottom(s claudecode.StatusInput, now time.Time, st style) string {
	var parts []string
	if p := s.ContextWindow.UsedPercentage; p != nil {
		part := "ctx " + st.level(*p, bar(*p, 8)+fmt.Sprintf(" %.0f%%", *p))
		if used, size := s.ContextTokens(), s.ContextWindow.ContextWindowSize; used > 0 && size > 0 {
			part += st.dim(" " + shortTokens(used) + "/" + shortTokens(size))
		}
		parts = append(parts, part)
	}
	if w := s.RateLimits.FiveHour; w != nil {
		parts = append(parts, "5h "+st.level(w.UsedPercentage, bar(w.UsedPercentage, 5)+fmt.Sprintf(" %.0f%%", w.UsedPercentage))+
			st.dim(resetsIn(w.ResetsAt, now)))
	}
	if w := s.RateLimits.SevenDay; w != nil {
		parts = append(parts, "7d "+st.level(w.UsedPercentage, bar(w.UsedPercentage, 5)+fmt.Sprintf(" %.0f%%", w.UsedPercentage))+
			st.dim(resetsIn(w.ResetsAt, now)))
	}
	if c := s.PromptCache; c != nil {
		part := "cache"
		if c.HitRatio != nil {
			part += fmt.Sprintf(" %.0f%%", *c.HitRatio*100)
		}
		// How long the warm cache lasts: after that, the next message pays for the whole context again.
		if left := time.Unix(c.ExpiresAt, 0).Sub(now); c.Warm && c.ExpiresAt > 0 && left > 0 {
			part += " " + st.level(warmthPct(left), "⏳"+shortDuration(left))
		} else if c.ExpiresAt > 0 {
			part += " " + st.paint("31", "cold")
		}
		if part != "cache" {
			parts = append(parts, part)
		}
	}
	return strings.Join(parts, st.dim(" · "))
}

// bar draws pct (0..100) as cells filled from the left.
func bar(pct float64, cells int) string {
	filled := int(math.Round(math.Max(0, math.Min(100, pct)) / 100 * float64(cells)))
	return strings.Repeat("▰", filled) + strings.Repeat("▱", cells-filled)
}

// warmthPct turns the time a warm cache has left into a share used, so it is coloured like the
// limits: plenty of time is green, the last minutes red.
func warmthPct(left time.Duration) float64 {
	switch {
	case left < time.Minute:
		return 90
	case left < 2*time.Minute:
		return 70
	}
	return 0
}

func shortTokens(n int64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1000:
		return fmt.Sprintf("%dK", (n+500)/1000)
	}
	return fmt.Sprint(n)
}

func shortDuration(d time.Duration) string {
	if d >= time.Hour {
		return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
	}
	if d >= time.Minute {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%ds", int(d.Seconds()))
}

// resetsIn says when a window resets, as a local time today or a weekday and time beyond that.
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

// gitBranch reads the branch of the repository dir is in from .git/HEAD, without running git:
// the status line runs after every reply and must be quick. "" outside a repository or when HEAD
// is detached.
func gitBranch(dir string) string {
	for d := dir; ; d = filepath.Dir(d) {
		gitPath := filepath.Join(d, ".git")
		info, err := os.Stat(gitPath)
		if err == nil {
			head := filepath.Join(gitPath, "HEAD")
			if !info.IsDir() { // a worktree: .git is a file naming the real git dir
				b, err := os.ReadFile(gitPath)
				if err != nil {
					return ""
				}
				gitDir := strings.TrimSpace(strings.TrimPrefix(string(b), "gitdir:"))
				if !filepath.IsAbs(gitDir) {
					gitDir = filepath.Join(d, gitDir)
				}
				head = filepath.Join(gitDir, "HEAD")
			}
			b, err := os.ReadFile(head)
			if err != nil {
				return ""
			}
			ref, ok := strings.CutPrefix(strings.TrimSpace(string(b)), "ref: refs/heads/")
			if !ok {
				return ""
			}
			return ref
		}
		if d == filepath.Dir(d) {
			return ""
		}
	}
}

// profileTitle is the label the user gave the profile in quipu, or its name.
func profileTitle(name string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return name
	}
	c, err := profile.Load(home, profilesFile(home))
	if err != nil {
		return name
	}
	if p := c.Find(name); p != nil {
		return p.Title()
	}
	return name
}

// profilePlan is the subscription plan quipu last saw for the profile (max, pro, team…), "" if
// none is known yet.
func profilePlan(name string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	dir, err := dataDir(home)
	if err != nil {
		return ""
	}
	b, err := os.ReadFile(ui.PlansFile(dir))
	if err != nil {
		return ""
	}
	var plans map[string]string
	json.Unmarshal(b, &plans)
	return plans[name]
}
