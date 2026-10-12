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

// PluginsMatrix shows every plugin against every profile.
type PluginsMatrix struct {
	Profiles []string       `json:"profiles"`
	Plugins  []PluginMatrix `json:"plugins"`
}

// PluginMatrix is one plugin, with its state in each profile.
type PluginMatrix struct {
	ID          string                `json:"id"`
	Name        string                `json:"name"`
	Marketplace string                `json:"marketplace"`
	Tokens      int64                 `json:"tokens"` // what it lists in every session where it is on
	Cells       map[string]PluginCell `json:"cells"`  // by profile
}

// PluginCell is a plugin in one profile.
type PluginCell struct {
	Installed bool     `json:"installed"`
	Version   string   `json:"version,omitempty"`
	State     string   `json:"state"`    // "everywhere", "projects", "off" or "" when not installed
	Projects  []string `json:"projects"` // projects whose own settings enable it
	Uses      int64    `json:"uses"`     // responses that used it in 30 days, under this profile
	UsedIn    []string `json:"usedIn"`
	Last      string   `json:"last,omitempty"`
	// Installable: not installed here, and its marketplace can be added from another profile.
	Installable bool `json:"installable"`
}

// PluginsReport puts every plugin installed in any profile against every profile.
func (s *Service) PluginsReport(ctx context.Context) (PluginsMatrix, error) {
	out := PluginsMatrix{Profiles: []string{}, Plugins: []PluginMatrix{}}
	projects := s.projectItems(ctx)
	byID := map[string]*PluginMatrix{}
	sources := map[string]string{} // marketplace → source, from any profile that knows it
	dirs := s.profileDirs()
	for _, dir := range dirs {
		for name, src := range claudecode.MarketplaceSources(dir) {
			if src != "" {
				sources[name] = src
			}
		}
	}
	for _, dir := range dirs {
		profileName := claudecode.ProfileName(dir)
		out.Profiles = append(out.Profiles, profileName)
		f := window(30)
		f.Profile = profileName
		uses, _ := analytics.PluginUses(ctx, s.st, f)
		for _, p := range claudecode.InstalledPlugins(dir) {
			row := byID[p.ID]
			if row == nil {
				row = &PluginMatrix{ID: p.ID, Name: p.Name, Marketplace: p.Marketplace, Cells: map[string]PluginCell{}}
				byID[p.ID] = row
			}
			var tokens int64
			for _, it := range claudecode.PluginListing(p.Dir, p.Name) {
				tokens += analytics.EstimateTokens(it.Text)
			}
			row.Tokens = max(row.Tokens, tokens)
			cell := PluginCell{Installed: true, Version: p.Version, State: "off", Projects: []string{}, UsedIn: []string{}}
			if p.Enabled != nil && *p.Enabled {
				cell.State = "everywhere"
			}
			for _, path := range sortedProjectPaths(projects) {
				if _, ok := claudecode.PluginsEnabledIn(path)[p.ID]; ok {
					cell.Projects = append(cell.Projects, filepath.Base(path))
				}
			}
			if cell.State == "off" && len(cell.Projects) > 0 {
				cell.State = "projects"
			}
			var last time.Time
			for _, u := range uses {
				if u.Plugin != p.Name {
					continue
				}
				cell.Uses += u.Responses
				if name := filepath.Base(u.Path); !contains(cell.UsedIn, name) {
					cell.UsedIn = append(cell.UsedIn, name)
				}
				if u.Last.After(last) {
					last = u.Last
				}
			}
			if !last.IsZero() {
				cell.Last = last.Format(time.RFC3339)
			}
			sort.Strings(cell.UsedIn)
			row.Cells[profileName] = cell
		}
	}
	for _, row := range byID {
		for _, profileName := range out.Profiles {
			if _, ok := row.Cells[profileName]; !ok {
				row.Cells[profileName] = PluginCell{Projects: []string{}, UsedIn: []string{},
					Installable: sources[row.Marketplace] != ""}
			}
		}
		out.Plugins = append(out.Plugins, *row)
	}
	sort.Slice(out.Plugins, func(i, j int) bool { return out.Plugins[i].ID < out.Plugins[j].ID })
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

// InstallPlugin installs a plugin another profile has into profile, with Claude Code's own
// commands under that profile's config dir: the marketplace is added first, from the source the
// other profile knows it by, when this profile does not know it yet.
func (s *Service) InstallPlugin(ctx context.Context, profileName, id string) (Result, error) {
	if s.ClaudeBinary == "" {
		return Result{}, problem("claude-missing")
	}
	dir, found := s.profileDir(profileName)
	if !found {
		return Result{}, problem("profile-unknown")
	}
	name, market, _ := strings.Cut(id, "@")
	run := func(args ...string) error {
		cctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(cctx, s.ClaudeBinary, args...)
		cmd.Env = claudecode.Env(s.home, dir)
		cmd.Stdin = nil // never wait for an answer; a question fails the command instead
		if out, err := cmd.CombinedOutput(); err != nil {
			return problem("plugin-install-failed", "name", name, "error", strings.TrimSpace(string(out)),
				"command", "CLAUDE_CONFIG_DIR="+s.shortPath(dir)+" claude "+strings.Join(args, " "))
		}
		return nil
	}
	if _, known := claudecode.MarketplaceSources(dir)[market]; !known {
		source := ""
		for _, other := range s.profileDirs() {
			if src := claudecode.MarketplaceSources(other)[market]; src != "" {
				source = src
			}
		}
		if source == "" {
			return Result{}, problem("marketplace-unknown", "name", market)
		}
		if err := run("plugin", "marketplace", "add", source); err != nil {
			return Result{}, err
		}
	}
	if err := run("plugin", "install", id); err != nil {
		return Result{}, err
	}
	return ok("plugin-installed", "name", name, "profile", profileName), nil
}
