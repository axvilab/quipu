package ui

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/axvilab/quipu/internal/claudecode"
	"github.com/axvilab/quipu/internal/profile"
)

// Check is one line of the doctor: what was checked, how it is, details for the text.
type Check struct {
	ID     string            `json:"id"`
	State  string            `json:"state"` // "ok", "info", "warn" or "fail"
	Params map[string]string `json:"params"`
}

// Doctor checks the setup quipu depends on and that quipu sets up.
func (s *Service) Doctor(ctx context.Context) []Check {
	var out []Check
	add := func(id, state string, kv ...string) {
		c := Check{ID: id, State: state, Params: map[string]string{}}
		for i := 0; i+1 < len(kv); i += 2 {
			c.Params[kv[i]] = kv[i+1]
		}
		out = append(out, c)
	}

	// Claude Code itself.
	if s.ClaudeBinary == "" {
		add("claude", "fail")
	} else {
		cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		v, _ := exec.CommandContext(cctx, s.ClaudeBinary, "--version").Output()
		cancel()
		add("claude", "ok", "version", strings.TrimSpace(string(v)), "path", s.shortPath(s.ClaudeBinary))
	}
	if installs := s.claudeInstalls(); len(installs) > 1 {
		add("installs", "warn", "list", strings.Join(installs, ", "))
	}

	// Profiles: login, shared links, folder rules.
	for _, p := range s.profileInfos(ctx) {
		if !p.SignedIn {
			add("login", "warn", "name", p.Name)
		}
	}
	dirs, _ := claudecode.ConfigDirs(s.home)
	for _, dir := range dirs {
		files, _ := claudecode.Setup(dir)
		for _, f := range files {
			if f.Link == "" {
				continue
			}
			target := f.Link
			if !filepath.IsAbs(target) {
				target = filepath.Join(dir, filepath.Dir(filepath.FromSlash(f.Rel)), target)
			}
			if _, err := os.Stat(target); err != nil {
				add("broken-link", "fail", "path", s.shortPath(filepath.Join(dir, filepath.FromSlash(f.Rel))), "target", f.Link)
			}
		}
	}
	if s.ProfilesFile != "" {
		if c, err := profile.Load(s.home, s.ProfilesFile); err == nil {
			for _, r := range c.Rules {
				switch {
				case c.Find(r.Profile) == nil:
					add("rule-profile", "fail", "folder", s.shortPath(r.Path), "name", r.Profile)
				default:
					if _, err := os.Stat(r.Path); err != nil {
						add("rule-folder", "warn", "folder", s.shortPath(r.Path), "name", r.Profile)
					}
				}
			}
		}
	}

	// The shim. Services do not see the shell's PATH, so "on" means: the link is in place and the
	// shell's startup file puts it first, which new terminals pick up.
	sh := s.Shim()
	switch {
	case sh.Installed && sh.InRC:
		add("shim", "ok", "rc", sh.RCFile)
	case sh.Installed:
		add("shim", "warn", "rc", sh.RCFile)
	default:
		add("shim", "info")
	}

	// quipu itself: the daemon, the window.
	if _, ok := Running(s.dataDir); ok {
		add("daemon", "ok")
	} else {
		add("daemon", "warn")
	}
	if app := windowApp(s.SelfBinary); app == "" {
		add("window", "info")
	} else if exec.CommandContext(ctx, app, "-version").Run() != nil {
		add("window", "warn")
	} else {
		add("window", "ok")
	}

	settings, _ := os.ReadFile(claudecode.UserSettings(s.configDir))
	if v, ok, _ := claudecode.SettingsValue(settings, "remoteControlAtStartup"); ok && bytes.Equal(bytes.TrimSpace(v), []byte("true")) {
		add("rc-autostart", "warn")
	}
	return out
}

// claudeInstalls lists the different claude programs on PATH (not the shim, not quipu).
func (s *Service) claudeInstalls() []string {
	self, _ := os.Stat(s.SelfBinary)
	seen := map[string]bool{}
	var out []string
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		path := filepath.Join(dir, "claude")
		info, err := os.Stat(path)
		if err != nil || info.IsDir() || info.Mode()&0o111 == 0 || (self != nil && os.SameFile(info, self)) {
			continue
		}
		real, err := filepath.EvalSymlinks(path)
		if err != nil {
			real = path
		}
		// The native installer keeps versions side by side; one install, whatever the version.
		key := filepath.Dir(real)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, s.shortPath(path))
	}
	sort.Strings(out)
	return out
}

// windowApp finds quipu-app as quipu ui does: in Quipu.app on macOS, else next to quipu.
func windowApp(self string) string {
	var candidates []string
	if runtime.GOOS == "darwin" {
		if home, err := os.UserHomeDir(); err == nil {
			candidates = append(candidates, filepath.Join(home, "Applications", "Quipu.app", "Contents", "MacOS", "quipu-app"))
		}
	}
	if self != "" {
		candidates = append(candidates, filepath.Join(filepath.Dir(self), "quipu-app"))
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

// ShimState is the shim as the settings page shows it.
type ShimState struct {
	Installed bool        `json:"installed"`
	InRC      bool        `json:"inRc"`
	RCFile    string      `json:"rcFile"`
	Line      string      `json:"line"`   // what goes into the startup file, for doing it by hand
	Shells    []ShellLine `json:"shells"` // every shell, for terminals that run another one
}

// ShellLine is the shim's PATH line for one shell.
type ShellLine struct {
	Name   string `json:"name"`
	RCFile string `json:"rcFile"`
	Line   string `json:"line"`
	InRC   bool   `json:"inRc"`
	Login  bool   `json:"login"` // the user's login shell
}

// Shim reports the shim's state.
func (s *Service) Shim() ShimState {
	dir := profile.ShimDir(s.dataDir)
	login := filepath.Base(profile.LoginShell())
	rcFile, block := profile.RCFile(s.home, login, dir)
	st := ShimState{Installed: s.SelfBinary != "" && profile.ShimInstalled(dir, s.SelfBinary), InRC: profile.HasRCBlock(rcFile),
		RCFile: s.shortPath(rcFile), Line: strings.TrimSpace(block)}
	for _, sh := range profile.Shells {
		f, b := profile.RCFile(s.home, sh, dir)
		lines := strings.Split(strings.TrimSpace(b), "\n")
		st.Shells = append(st.Shells, ShellLine{Name: sh, RCFile: s.shortPath(f), Line: lines[len(lines)-1],
			InRC: profile.HasRCBlock(f), Login: sh == login})
	}
	return st
}

// SetShimRC adds or removes the shim's PATH line for one shell.
func (s *Service) SetShimRC(shell string, on bool) (Result, error) {
	if !contains(profile.Shells, shell) {
		return Result{}, problem("shell-unknown", "shell", shell)
	}
	rc, err := profile.SetRC(on, shell, s.home, s.dataDir, s.journal)
	if err != nil {
		return Result{}, problem("shim-failed", "error", err.Error())
	}
	if on {
		return ok("shim-rc-on", "file", s.shortPath(rc)), nil
	}
	return ok("shim-rc-off", "file", s.shortPath(rc)), nil
}

// SetShim turns the shim on or off, editing the shell startup file through the journal.
func (s *Service) SetShim(on bool) (Result, error) {
	if s.SelfBinary == "" {
		return Result{}, problem("no-self-binary")
	}
	if _, err := profile.SetShim(on, true, s.home, s.dataDir, s.SelfBinary, s.journal); err != nil {
		return Result{}, problem("shim-failed", "error", err.Error())
	}
	if on {
		return ok("shim-on"), nil
	}
	return ok("shim-off"), nil
}
