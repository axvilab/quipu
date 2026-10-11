package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/axvilab/quipu/internal/claudecode"
	"github.com/axvilab/quipu/internal/live"
)

// runHook handles `quipu hook claude`, the hook quipu adds to Claude Code's settings when the user
// asks it to watch Remote Control. It records the session and whether Remote Control is connected,
// prints nothing (a hook's output can enter the session's context) and ignores every error.
func runHook(args []string) {
	if len(args) == 0 || args[0] != "claude" {
		return
	}
	var in struct {
		SessionID string `json:"session_id"`
		Cwd       string `json:"cwd"`
		Event     string `json:"hook_event_name"`
	}
	if err := json.NewDecoder(io.LimitReader(os.Stdin, 1<<20)).Decode(&in); err != nil || in.SessionID == "" {
		return
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	dir, err := dataDir(home)
	if err != nil {
		return
	}
	live.Save(filepath.Join(dir, "live"), live.Record{
		SessionID:     in.SessionID,
		Cwd:           in.Cwd,
		PID:           claudecode.SessionOf(os.Getppid()),
		RemoteControl: os.Getenv("CLAUDE_CODE_BRIDGE_SESSION_ID") != "",
		Event:         in.Event,
		Time:          time.Now(),
	})
}
