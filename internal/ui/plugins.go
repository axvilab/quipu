package ui

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/axvilab/quipu/internal/analytics"
	"github.com/axvilab/quipu/internal/claudecode"
	"github.com/axvilab/quipu/internal/fsx"
)

// PluginRow is one installed plugin of one profile.
type PluginRow struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Marketplace string   `json:"marketplace"`
	Version     string   `json:"version"`
	Profile     string   `json:"profile"`
	State       string   `json:"state"`    // "everywhere" (the profile's settings enable it), "projects" or "off"
	Projects    []string `json:"projects"` // projects whose own settings enable it
	Tokens      int64    `json:"tokens"`   // what it lists in every session where it is enabled
	Uses        int64    `json:"uses"`     // responses that used it in 30 days, under this profile
	UsedIn      []string `json:"usedIn"`
	Last        string   `json:"last,omitempty"`
}

// PluginsReport lists the plugins installed in every profile.
func (s *Service) PluginsReport(ctx context.Context) ([]PluginRow, error) {
	dirs := s.profileDirs()
	projects := s.projectItems(ctx)
	out := []PluginRow{}
	for _, dir := range dirs {
		profileName := claudecode.ProfileName(dir)
		f := window(30)
		f.Profile = profileName
		uses, _ := analytics.PluginUses(ctx, s.st, f)
		for _, p := range claudecode.InstalledPlugins(dir) {
			row := PluginRow{ID: p.ID, Name: p.Name, Marketplace: p.Marketplace, Version: p.Version, Profile: profileName,
				State: "off", Projects: []string{}, UsedIn: []string{}}
			if p.Enabled != nil && *p.Enabled {
				row.State = "everywhere"
			}
			for _, path := range sortedProjectPaths(projects) {
				if _, ok := claudecode.PluginsEnabledIn(path)[p.ID]; ok {
					row.Projects = append(row.Projects, filepath.Base(path))
				}
			}
			if row.State == "off" && len(row.Projects) > 0 {
				row.State = "projects"
			}
			for _, it := range claudecode.PluginListing(p.Dir, p.Name) {
				row.Tokens += analytics.EstimateTokens(it.Text)
			}
			var last time.Time
			for _, u := range uses {
				if u.Plugin != p.Name {
					continue
				}
				row.Uses += u.Responses
				if name := filepath.Base(u.Path); !contains(row.UsedIn, name) {
					row.UsedIn = append(row.UsedIn, name)
				}
				if u.Last.After(last) {
					last = u.Last
				}
			}
			if !last.IsZero() {
				row.Last = last.Format(time.RFC3339)
			}
			sort.Strings(row.UsedIn)
			out = append(out, row)
		}
	}
	return out, nil
}

// profileDirs are the config dirs of every profile: quipu's profiles, the ones with sessions, and
// ~/.claude if it exists, even before a first session.
func (s *Service) profileDirs() []string {
	seen := map[string]bool{}
	var out []string
	add := func(dir string) {
		if info, err := os.Stat(dir); err == nil && info.IsDir() && !seen[dir] {
			seen[dir] = true
			out = append(out, dir)
		}
	}
	add(filepath.Join(s.home, ".claude"))
	if c, err := s.loadProfiles(); err == nil {
		for _, p := range c.Profiles {
			add(p.Dir)
		}
	}
	dirs, _ := claudecode.ConfigDirs(s.home)
	for _, dir := range dirs {
		add(dir)
	}
	return out
}

// profileDir finds the config dir of a profile by name.
func (s *Service) profileDir(name string) (string, bool) {
	for _, dir := range s.profileDirs() {
		if claudecode.ProfileName(dir) == name {
			return dir, true
		}
	}
	return "", false
}

// SetPluginEnabled turns a plugin on or off for every project of a profile, in that profile's
// settings through the journal.
func (s *Service) SetPluginEnabled(profileName, id string, on bool) (Result, error) {
	dir, found := s.profileDir(profileName)
	if !found {
		return Result{}, problem("profile-unknown")
	}
	path := claudecode.UserSettings(dir)
	current, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return Result{}, problem("read-failed", "path", s.shortPath(path), "error", err.Error())
	}
	enabled := map[string]json.RawMessage{}
	if raw, ok, err := claudecode.SettingsValue(current, "enabledPlugins"); err != nil {
		return Result{}, problem("settings-invalid", "path", s.shortPath(path), "error", err.Error())
	} else if ok {
		json.Unmarshal(raw, &enabled)
	}
	enabled[id], _ = json.Marshal(on)
	raw, _ := json.Marshal(enabled)
	next, err := claudecode.SetSettingsValue(current, "enabledPlugins", raw)
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
	name, _, _ := strings.Cut(id, "@")
	ch, err := s.journal.Write(path, next, fsx.Meta{Rule: "plugin-toggle", Title: "plugin " + name + " " + state + " for " + profileName,
		Params: map[string]string{"state": state, "name": name, "profile": profileName}, Batch: batch})
	if err != nil {
		return Result{}, problem("write-failed", "path", s.shortPath(path), "error", err.Error())
	}
	return ok("plugin-"+state, "name", name, "profile", profileName, "change", ch.ID), nil
}

// UninstallPlugin removes a plugin from a profile with Claude Code's own command. It cannot be
// undone from quipu; the plugin can be installed again with claude plugin install.
func (s *Service) UninstallPlugin(ctx context.Context, profileName, id string) (Result, error) {
	if s.ClaudeBinary == "" {
		return Result{}, problem("claude-missing")
	}
	dir, found := s.profileDir(profileName)
	if !found {
		return Result{}, problem("profile-unknown")
	}
	known := false
	for _, p := range claudecode.InstalledPlugins(dir) {
		known = known || p.ID == id
	}
	if !known {
		return Result{}, problem("plugin-unknown", "name", id)
	}
	cctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	cmd := exec.CommandContext(cctx, s.ClaudeBinary, "plugin", "uninstall", id)
	cmd.Env = claudecode.Env(s.home, dir)
	out, err := cmd.CombinedOutput()
	name, _, _ := strings.Cut(id, "@")
	if err != nil {
		return Result{}, problem("plugin-uninstall-failed", "name", name, "error", strings.TrimSpace(string(out)))
	}
	return ok("plugin-uninstalled", "name", name, "profile", profileName), nil
}
