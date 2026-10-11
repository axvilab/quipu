// Package live tells which Claude Code sessions are running now and which of them have Remote
// Control on: from the processes, and from records an opt-in hook writes on session events.
package live

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/axvilab/quipu/internal/claudecode"
	"github.com/axvilab/quipu/internal/fsx"
)

// HookMarker ends the hook's command and finds quipu's entries in settings.json; the binary's
// path before it may be quoted, so the marker does not include it.
const HookMarker = " hook claude"

// HookEvents are the session events the hook listens to: start, every prompt (also those sent
// through Remote Control), end of a reply, and end of the session.
var HookEvents = []string{"SessionStart", "UserPromptSubmit", "Stop", "SessionEnd"}

// Record is what the hook saw at a session event.
type Record struct {
	SessionID     string    `json:"session_id"`
	Cwd           string    `json:"cwd"`
	PID           int       `json:"pid"` // the claude process, 0 if not found
	RemoteControl bool      `json:"remote_control"`
	Event         string    `json:"event"`
	Time          time.Time `json:"time"`
}

// Session is a running Claude Code session.
type Session struct {
	PID     int       `json:"pid"`
	Cwd     string    `json:"cwd"`
	Started time.Time `json:"started"`
	// RemoteControl is "on", "off" or "unknown" (switched on inside the session and no hook record
	// yet, or the hook is not installed).
	RemoteControl string    `json:"remoteControl"`
	Source        string    `json:"source"` // "command" (flag on the command line) or "hook"
	Seen          time.Time `json:"seen"`   // time of the hook record behind RemoteControl
}

// Save writes the record of a session, or removes it when the session ended.
func Save(dir string, r Record) error {
	path := filepath.Join(dir, safeName(r.SessionID)+".json")
	if r.Event == "SessionEnd" {
		err := os.Remove(path)
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	return fsx.WriteFile(path, b)
}

// Sessions lists running sessions with what is known about Remote Control. Records of sessions
// that are no longer running are removed.
func Sessions(dir string) ([]Session, error) {
	procs, err := claudecode.RunningSessions()
	if err != nil {
		return nil, err
	}
	latest := map[int]Record{}
	running := map[int]bool{}
	for _, p := range procs {
		running[p.PID] = true
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		var r Record
		b, err := os.ReadFile(path)
		if err != nil || json.Unmarshal(b, &r) != nil || !running[r.PID] {
			os.Remove(path) // its session is gone
			continue
		}
		if r.Time.After(latest[r.PID].Time) {
			latest[r.PID] = r
		}
	}
	out := []Session{}
	for _, p := range procs {
		s := Session{PID: p.PID, Cwd: p.Cwd, Started: p.Started, RemoteControl: "unknown"}
		switch r, ok := latest[p.PID]; {
		case p.RemoteControl:
			s.RemoteControl, s.Source = "on", "command"
		case ok:
			s.RemoteControl, s.Source, s.Seen = "off", "hook", r.Time
			if r.RemoteControl {
				s.RemoteControl = "on"
			}
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Started.Before(out[j].Started) })
	return out, nil
}

// safeName keeps a session id usable as a file name.
func safeName(id string) string {
	id = strings.Map(func(r rune) rune {
		if r == '-' || r == '_' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' {
			return r
		}
		return '_'
	}, id)
	if id == "" {
		return "unknown"
	}
	return id
}
