package ui

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/axvilab/quipu/internal/claudecode"
	"github.com/axvilab/quipu/internal/fsx"
	"github.com/axvilab/quipu/internal/live"
)

// LiveSession is a running Claude Code session.
type LiveSession struct {
	PID           int    `json:"pid"`
	Path          string `json:"path"`
	Name          string `json:"name"`
	Started       string `json:"started"`
	RemoteControl string `json:"remoteControl"` // "on", "off" or "unknown"
	Source        string `json:"source"`        // "command" or "hook"
	Seen          string `json:"seen,omitempty"`
}

// Live is what runs now: sessions, and how much quipu can tell about Remote Control.
type Live struct {
	Sessions  []LiveSession `json:"sessions"`
	Watching  bool          `json:"watching"`  // quipu's hook is in the profile's settings
	AutoStart bool          `json:"autoStart"` // remoteControlAtStartup turns it on in every session
}

func (s *Service) liveDir() string { return filepath.Join(s.dataDir, "live") }

// LiveSessions lists the running Claude Code sessions.
func (s *Service) LiveSessions(ctx context.Context) (Live, error) {
	out := Live{Sessions: []LiveSession{}}
	settings, _ := os.ReadFile(claudecode.UserSettings(s.configDir))
	out.Watching = claudecode.HasHook(settings, live.HookMarker)
	if v, ok, _ := claudecode.SettingsValue(settings, "remoteControlAtStartup"); ok && bytes.Equal(bytes.TrimSpace(v), []byte("true")) {
		out.AutoStart = true
	}
	sessions, err := live.Sessions(s.liveDir())
	if err != nil {
		return out, err
	}
	for _, x := range sessions {
		ls := LiveSession{PID: x.PID, Path: s.shortPath(x.Cwd), Name: filepath.Base(x.Cwd), RemoteControl: x.RemoteControl, Source: x.Source}
		if !x.Started.IsZero() {
			ls.Started = x.Started.Format("2006-01-02T15:04:05Z07:00")
		}
		if !x.Seen.IsZero() {
			ls.Seen = x.Seen.Format("2006-01-02T15:04:05Z07:00")
		}
		out.Sessions = append(out.Sessions, ls)
	}
	return out, nil
}

// WatchRemoteControl adds or removes quipu's hook in the profile's settings.json, through the
// journal, so it can be undone like any other change.
func (s *Service) WatchRemoteControl(on bool) (Result, error) {
	if on && s.SelfBinary == "" {
		return Result{}, problem("no-self-binary")
	}
	path := claudecode.UserSettings(s.configDir)
	current, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return Result{}, problem("read-failed", "path", s.shortPath(path), "error", err.Error())
	}
	var next []byte
	if on {
		next, err = claudecode.WithHook(current, live.HookEvents, shellQuote(s.SelfBinary)+" hook claude", live.HookMarker)
	} else {
		next, err = claudecode.WithoutHook(current, live.HookMarker)
	}
	if err != nil {
		return Result{}, problem("settings-invalid", "path", s.shortPath(path), "error", err.Error())
	}
	if bytes.Equal(next, current) {
		return ok("unchanged"), nil
	}
	batch, err := fsx.NewBatch()
	if err != nil {
		return Result{}, err
	}
	state := "off"
	if on {
		state = "on"
	}
	c, err := s.journal.Write(path, next, fsx.Meta{Rule: "watch-remote-control", Title: "Remote Control watching " + state,
		Params: map[string]string{"state": state}, Batch: batch})
	if err != nil {
		return Result{}, problem("write-failed", "path", s.shortPath(path), "error", err.Error())
	}
	return ok("watching-"+state, "change", c.ID), nil
}

// shellQuote quotes a path for the hook command, which Claude Code runs through a shell.
func shellQuote(p string) string {
	if !strings.ContainsAny(p, " '\"$`\\!*?[]{}()&;|<>~#") {
		return p
	}
	return "'" + strings.ReplaceAll(p, "'", `'\''`) + "'"
}
