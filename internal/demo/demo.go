// Package demo creates a fake home directory with a Claude Code setup, projects and transcripts.
// It lets anyone try quipu, take screenshots or test "Apply" without touching a real ~/.claude:
//
//	quipu demo /tmp/quipu-demo
//	HOME=/tmp/quipu-demo quipu ui
package demo

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/axvilab/quipu/internal/claudecode"
)

// Create writes the demo setup under home, which must be empty or missing.
func Create(home string, now time.Time) error {
	if entries, err := os.ReadDir(home); err == nil && len(entries) > 0 {
		return fmt.Errorf("%s is not empty", home)
	}
	cfg := filepath.Join(home, ".claude")
	shop := filepath.Join(home, "work", "shop-api")
	blog := filepath.Join(home, "pet", "blog")
	// More projects with only a short CLAUDE.md and sessions, so the overview looks like a real
	// machine with several projects of different weight.
	others := []string{
		filepath.Join(home, "work", "billing"),
		filepath.Join(home, "work", "infra"),
		filepath.Join(home, "work", "admin"),
		filepath.Join(home, "pet", "mobile"),
		filepath.Join(home, "pet", "notes"),
	}

	files := map[string]string{
		filepath.Join(cfg, "CLAUDE.md"):     "# About me\n\n- Answer briefly and to the point.\n- Write commit messages as Conventional Commits.\n",
		filepath.Join(cfg, "settings.json"): "{\n  \"theme\": \"dark\",\n  \"effortLevel\": \"high\",\n  \"enabledPlugins\": {\n    \"browser@demo-market\": true,\n    \"slides@demo-market\": true\n  }\n}\n",

		filepath.Join(shop, "CLAUDE.md"):                                shopClaudeMD(),
		filepath.Join(shop, "docs", "DESIGN.md"):                        "# shop-api design\n\n" + strings.Repeat("The order service talks to payments through a queue; retries are idempotent by order ID.\n", 60),
		filepath.Join(shop, ".claude", "rules", "db.md"):                "---\npaths:\n  - \"migrations/**\"\n---\nMigrations must be reversible.\n",
		filepath.Join(shop, ".claude", "skills", "release", "SKILL.md"): releaseSkill,

		filepath.Join(blog, "CLAUDE.md"): "# blog\n\nStatic site. Build with `make site`.\n",

		filepath.Join(cfg, "projects", claudecode.ProjectDirName(shop), "memory", "MEMORY.md"): strings.Repeat("- [Note](note.md): something learned about the shop\n", 190),
	}
	// The same testing block in several projects, and the same release skill in two of them.
	for _, o := range others {
		files[filepath.Join(o, "CLAUDE.md")] = "# " + filepath.Base(o) + "\n\n" + testingBlock
	}
	files[filepath.Join(others[0], ".claude", "skills", "release", "SKILL.md")] = releaseSkill
	// Two plugins enabled for every project: browser is used in shop-api only, slides nowhere.
	installed := map[string]any{}
	for _, name := range []string{"browser", "slides"} {
		dir := filepath.Join(cfg, "plugins", "cache", "demo-market", name, "1.0.0")
		files[filepath.Join(dir, "skills", name, "SKILL.md")] = fmt.Sprintf(
			"---\nname: %s\ndescription: Use this skill whenever the user asks to work with %s, from the first step to a finished result, including checks and fixes along the way.\n---\nInstructions.\n",
			name, name)
		installed[name+"@demo-market"] = []map[string]string{{"installPath": dir}}
	}
	b, _ := json.Marshal(map[string]any{"version": 2, "plugins": installed})
	files[filepath.Join(cfg, "plugins", "installed_plugins.json")] = string(b)
	for _, skill := range []string{"pptx", "docx", "xlsx", "pdf", "deep-research", "browser"} {
		files[filepath.Join(cfg, "skills", "synced", "demo-account", skill, "SKILL.md")] = fmt.Sprintf(
			"---\nname: %s\ndescription: Use this skill whenever the user wants to create, read or edit %s content, including converting, extracting, reorganizing and polishing it into a finished deliverable for sharing, printing or presenting.\n---\nInstructions for %s.\n",
			skill, skill, skill)
	}
	for path, text := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			return err
		}
	}

	rng := rand.New(rand.NewPCG(1, 2)) // fixed seed: the same demo every time
	projects := []struct {
		path     string
		sessions int
		start    int64 // tokens of the first request
	}{{shop, 6, 48000}, {blog, 3, 31000}, {others[0], 4, 39000}, {others[1], 5, 52000}, {others[2], 3, 35000}, {others[3], 2, 27000}, {others[4], 1, 22000}}
	for _, p := range projects {
		dir := filepath.Join(cfg, "projects", claudecode.ProjectDirName(p.path))
		for i := 0; i < p.sessions; i++ {
			started := now.AddDate(0, 0, -(i*4 + 1)).Add(-time.Duration(rng.IntN(8)) * time.Hour)
			id := fmt.Sprintf("demo-%s-%d", filepath.Base(p.path), i)
			tool := "" // shop-api drives the browser plugin now and then
			if p.path == shop {
				tool = "mcp__plugin_browser_browser__click"
			}
			if err := writeSession(filepath.Join(dir, id+".jsonl"), id, p.path, started, p.start, tool, rng); err != nil {
				return err
			}
		}
	}
	return nil
}

const (
	testingBlock = "## Testing\n\n- Run `make test` before committing.\n- Keep each test next to the code it covers.\n"
	releaseSkill = "---\nname: release\ndescription: Cut a release\n---\n1. Bump the version.\n2. Tag and push.\n"
)

func shopClaudeMD() string {
	var b strings.Builder
	// The commit rule repeats the user's CLAUDE.md, so it is sent twice in every session.
	b.WriteString("# shop-api\n\nDesign: @docs/DESIGN.md\n\n- Write commit messages as Conventional Commits.\n\n" +
		testingBlock + "\n## Commands\n\n- `make test`\n- `make run`\n\n## Rules\n\n")
	for i := 1; i <= 240; i++ {
		fmt.Fprintf(&b, "- Rule %d: keep handlers thin and put logic into services.\n", i)
	}
	return b.String()
}

// writeSession writes a transcript in Claude Code's format: each response is split over two
// lines repeating the same usage, as the real client does.
// tool, if set, is called in every fifth response.
func writeSession(path, session, cwd string, started time.Time, startTokens int64, tool string, rng *rand.Rand) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)

	context := startTokens
	ts := started
	turns := 20 + rng.IntN(40)
	for i := 0; i < turns; i++ {
		read, written := context, int64(200+rng.IntN(1500))
		var diagnostics any
		switch {
		case i == 0:
			read, written = 0, startTokens
		case rng.IntN(12) == 0:
			reason := []string{"previous_message_not_found", "messages_changed", "tools_changed", "model_changed"}[rng.IntN(4)]
			diagnostics = map[string]any{"cache_miss_reason": map[string]any{"type": reason, "cache_missed_input_tokens": context}}
			read, written = 0, context
			ts = ts.Add(70 * time.Minute) // a long pause expires the cache
		}
		msg := map[string]any{
			"id": fmt.Sprintf("msg_%s_%d", session, i), "model": "claude-opus-5-5", "stop_reason": "end_turn",
			"usage": map[string]any{
				"input_tokens": 3, "cache_read_input_tokens": read, "cache_creation_input_tokens": written,
				"output_tokens":  100 + rng.IntN(900),
				"cache_creation": map[string]any{"ephemeral_5m_input_tokens": 0, "ephemeral_1h_input_tokens": written},
			},
			"diagnostics": diagnostics,
		}
		if tool != "" && i%5 == 4 {
			msg["content"] = []map[string]any{{"type": "tool_use", "name": tool, "input": map[string]any{}}}
		}
		line := map[string]any{
			"type": "assistant", "sessionId": session, "timestamp": ts.UTC().Format(time.RFC3339),
			"version": "2.1.296", "cwd": cwd, "gitBranch": "main", "isSidechain": false,
			"requestId": fmt.Sprintf("req_%s_%d", session, i), "message": msg,
		}
		for range 2 {
			if err := enc.Encode(line); err != nil {
				return err
			}
		}
		context += written
		ts = ts.Add(time.Duration(30+rng.IntN(300)) * time.Second)
	}
	return nil
}
