package ui

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"strings"

	"github.com/axvilab/quipu/internal/claudecode"
	"github.com/axvilab/quipu/internal/fsx"
)

// statusLineOf tells which status line a profile uses: "quipu", "other" (with its command) or "".
func statusLineOf(configDir string) (state, command string) {
	b, err := os.ReadFile(claudecode.UserSettings(configDir))
	if err != nil {
		return "", ""
	}
	raw, ok, _ := claudecode.SettingsValue(b, "statusLine")
	if !ok {
		return "", ""
	}
	var v struct {
		Command string `json:"command"`
	}
	json.Unmarshal(raw, &v)
	if isQuipuStatusLine(v.Command) {
		return "quipu", v.Command
	}
	return "other", v.Command
}

func isQuipuStatusLine(command string) bool {
	return strings.HasSuffix(strings.TrimSpace(command), " statusline") && strings.Contains(command, "quipu")
}

// SetStatusLine makes profile name use quipu's status line, or removes it, in that profile's
// settings through the journal; undo brings back what was there before.
func (s *Service) SetStatusLine(name string, on bool) (Result, error) {
	if on && s.SelfBinary == "" {
		return Result{}, problem("no-self-binary")
	}
	c, err := s.loadProfiles()
	if err != nil {
		return Result{}, problem("profiles-unavailable")
	}
	p := c.Find(name)
	if p == nil {
		return Result{}, problem("profile-unknown")
	}
	path := claudecode.UserSettings(p.Dir)
	current, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return Result{}, problem("read-failed", "path", s.shortPath(path), "error", err.Error())
	}
	var value json.RawMessage // nil removes the key
	if on {
		value = claudecode.StatusLineCommand(shellQuote(s.SelfBinary) + " statusline")
	} else if state, _ := statusLineOf(p.Dir); state != "quipu" {
		return ok("unchanged"), nil // someone else's status line is not ours to remove
	}
	next, err := claudecode.SetSettingsValue(current, "statusLine", value)
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
	ch, err := s.journal.Write(path, next, fsx.Meta{Rule: "statusline", Title: "quipu status line " + state + " for " + name,
		Params: map[string]string{"state": state, "name": name}, Batch: batch})
	if err != nil {
		return Result{}, problem("write-failed", "path", s.shortPath(path), "error", err.Error())
	}
	return ok("statusline-"+state, "name", name, "change", ch.ID), nil
}
