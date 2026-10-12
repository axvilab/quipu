// Package profile manages Claude Code profiles: one config dir per subscription, folder rules
// that pick a profile, and an optional library of skills and rules shared with ~/.claude.
package profile

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Default is the profile that uses plain ~/.claude.
const Default = "default"

// Shareable are the items of ~/.claude a new profile may link to. projects/ (memory and
// transcripts) is deliberately not here: it stays per profile.
var Shareable = []string{"CLAUDE.md", "skills", "agents", "rules", "commands"}

// Profile is one Claude Code config dir.
type Profile struct {
	Name  string   `json:"name"`
	Dir   string   `json:"dir"`
	Links []string `json:"links,omitempty"` // symlinks quipu created inside Dir
}

// Rule selects a profile for a folder and everything below it.
type Rule struct {
	Path    string `json:"path"`
	Profile string `json:"profile"`
}

// Config is quipu's profile state, stored as JSON.
type Config struct {
	Profiles []Profile `json:"profiles"`
	Rules    []Rule    `json:"rules"`

	home string
	file string
}

var nameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}$`)

// Load reads the config file (missing is fine) and makes sure the default profile is listed.
func Load(home, file string) (*Config, error) {
	c := &Config{home: home, file: file}
	b, err := os.ReadFile(file)
	switch {
	case err == nil:
		if err := json.Unmarshal(b, c); err != nil {
			return nil, fmt.Errorf("%s: %w", file, err)
		}
	case !errors.Is(err, fs.ErrNotExist):
		return nil, err
	}
	if c.Find(Default) == nil {
		c.Profiles = append([]Profile{{Name: Default, Dir: filepath.Join(home, ".claude")}}, c.Profiles...)
	}
	return c, nil
}

// Save writes the config atomically.
func (c *Config) Save() error {
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(c.file), 0o700); err != nil {
		return err
	}
	// A temp file of its own, so two terminals saving at once never mix their writes.
	tmp, err := os.CreateTemp(filepath.Dir(c.file), filepath.Base(c.file)+".*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(append(b, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), c.file)
}

// Find returns the named profile or nil.
func (c *Config) Find(name string) *Profile {
	for i := range c.Profiles {
		if c.Profiles[i].Name == name {
			return &c.Profiles[i]
		}
	}
	return nil
}

// Add creates profile name in ~/.claude-<name> and links the chosen shared items from ~/.claude.
// An existing dir (a profile made by hand earlier) is adopted as is; links are never placed over
// existing files.
func (c *Config) Add(name string, share []string) (*Profile, error) {
	if !nameRe.MatchString(name) || name == Default {
		return nil, fmt.Errorf("invalid profile name %q: use lowercase letters, digits and dashes", name)
	}
	if c.Find(name) != nil {
		return nil, fmt.Errorf("profile %q already exists", name)
	}
	for _, item := range share {
		if !contains(Shareable, item) {
			return nil, fmt.Errorf("cannot share %q; shareable: %s", item, strings.Join(Shareable, ", "))
		}
	}
	p := Profile{Name: name, Dir: filepath.Join(c.home, ".claude-"+name)}
	if err := os.MkdirAll(p.Dir, 0o700); err != nil {
		return nil, err
	}
	for _, item := range share {
		src := filepath.Join(c.home, ".claude", item)
		dst := filepath.Join(p.Dir, item)
		if _, err := os.Stat(src); err != nil {
			continue // nothing to share yet
		}
		if _, err := os.Lstat(dst); err == nil {
			continue // the profile already has its own
		}
		if err := os.Symlink(src, dst); err != nil {
			return nil, err
		}
		p.Links = append(p.Links, dst)
	}
	c.Profiles = append(c.Profiles, p)
	return &p, nil
}

// Remove forgets profile name: its rules, the symlinks quipu created and the config entry. The
// profile dir itself stays because it holds the login and sessions; its path is returned.
func (c *Config) Remove(name string) (string, error) {
	if name == Default {
		return "", errors.New("the default profile cannot be removed")
	}
	p := c.Find(name)
	if p == nil {
		return "", fmt.Errorf("no profile %q", name)
	}
	for _, link := range p.Links {
		if fi, err := os.Lstat(link); err == nil && fi.Mode()&fs.ModeSymlink != 0 {
			if err := os.Remove(link); err != nil {
				return "", err
			}
		}
	}
	dir := p.Dir
	var keep []Profile
	for _, q := range c.Profiles {
		if q.Name != name {
			keep = append(keep, q)
		}
	}
	c.Profiles = keep
	var rules []Rule
	for _, r := range c.Rules {
		if r.Profile != name {
			rules = append(rules, r)
		}
	}
	c.Rules = rules
	return dir, nil
}

// SetRule makes folder (and everything below) use profile; it replaces a rule for the same folder.
func (c *Config) SetRule(folder, profile string) error {
	if c.Find(profile) == nil {
		return fmt.Errorf("no profile %q", profile)
	}
	folder = filepath.Clean(folder)
	c.DeleteRule(folder)
	c.Rules = append(c.Rules, Rule{Path: folder, Profile: profile})
	sort.Slice(c.Rules, func(i, j int) bool { return c.Rules[i].Path < c.Rules[j].Path })
	return nil
}

// DeleteRule removes the rule for folder and reports whether there was one.
func (c *Config) DeleteRule(folder string) bool {
	folder = filepath.Clean(folder)
	for i, r := range c.Rules {
		if r.Path == folder {
			c.Rules = append(c.Rules[:i], c.Rules[i+1:]...)
			return true
		}
	}
	return false
}

// ForFolder returns the profile of the most specific rule covering dir, or "" if none does.
func (c *Config) ForFolder(dir string) string {
	r, _ := c.RuleFor(dir)
	return r.Profile
}

// RuleFor returns the rule that applies to dir: the one for the closest folder above it.
func (c *Config) RuleFor(dir string) (Rule, bool) {
	dir = filepath.Clean(dir)
	var best Rule
	found := false
	for _, r := range c.Rules {
		if (dir == r.Path || strings.HasPrefix(dir, r.Path+string(filepath.Separator))) && (!found || len(r.Path) > len(best.Path)) {
			best, found = r, true
		}
	}
	return best, found
}

// RememberFolder is the folder a profile chosen in dir should be remembered for: the git
// repository dir is in, or dir itself. It is "" where a rule would cover too much (home, a folder
// above it, the filesystem root) or is pointless (the temp folder).
func RememberFolder(dir, home string) string {
	dir, home = filepath.Clean(dir), filepath.Clean(home)
	folder := dir
	for d := dir; d != home && d != filepath.Dir(d); d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, ".git")); err == nil {
			folder = d
			break
		}
	}
	tmp := filepath.Clean(os.TempDir())
	switch {
	case folder == home, folder == filepath.Dir(folder), strings.HasPrefix(home, folder+string(filepath.Separator)):
		return ""
	case folder == tmp, strings.HasPrefix(folder, tmp+string(filepath.Separator)):
		return ""
	}
	return folder
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
