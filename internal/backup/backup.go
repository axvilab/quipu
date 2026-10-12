// Package backup saves the Claude Code setup of every profile (instructions, settings, extensions,
// project memory) and quipu's profiles to an archive or a git working copy, and plans its restore.
// Credentials, transcripts and caches are never part of it; see docs/plans/2026-10-11-backup.md.
package backup

import (
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/axvilab/quipu/internal/claudecode"
	"github.com/axvilab/quipu/internal/profile"
)

// Manifest describes a backup; it is stored as manifest.json at the top.
type Manifest struct {
	Version string    `json:"version"` // quipu version that wrote it
	Created time.Time `json:"created"`
	Home    string    `json:"home"`  // home folder of the machine it came from
	Links   []Link    `json:"links"` // symlinks, restored as links
}

// Link is a symlink in the setup: Path in the backup layout, Target as it was on disk.
type Link struct {
	Path   string `json:"path"`
	Target string `json:"target"`
}

// Entry is one file in the backup layout (slash-separated path) with its content.
type Entry struct {
	Path string
	Data []byte
	Exec bool // the file was executable (skill scripts)
}

// Collect reads the setup of every profile under home and quipu's profiles file.
func Collect(home, profilesFile, version string, exclude map[string]bool) (Manifest, []Entry, error) {
	m := Manifest{Version: version, Created: time.Now().UTC(), Home: home, Links: []Link{}}
	var entries []Entry
	dirs, err := ProfileDirs(home, profilesFile)
	if err != nil {
		return m, nil, err
	}
	for _, dir := range dirs {
		files, err := claudecode.Setup(dir)
		if err != nil {
			return m, nil, err
		}
		prefix := "claude/" + filepath.Base(dir) + "/"
		for _, f := range files {
			if project := memoryProject(f.Rel); project != "" && exclude[project] {
				continue
			}
			if f.Link != "" {
				m.Links = append(m.Links, Link{Path: prefix + f.Rel, Target: f.Link})
				continue
			}
			full := filepath.Join(dir, filepath.FromSlash(f.Rel))
			b, err := os.ReadFile(full)
			if err != nil {
				return m, nil, err
			}
			fi, _ := os.Stat(full)
			entries = append(entries, Entry{Path: prefix + f.Rel, Data: b, Exec: fi != nil && fi.Mode()&0o111 != 0})
		}
	}
	if b, err := os.ReadFile(profilesFile); err == nil {
		entries = append(entries, Entry{Path: "quipu/profiles.json", Data: b})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return m, entries, nil
}

// target maps a path of the backup layout to where it goes on this machine, or "" if the path
// is not part of the layout (a file someone added to the repository).
func target(p, home, profilesFile, oldHome string) string {
	if p == "quipu/profiles.json" {
		return profilesFile
	}
	rest, ok := strings.CutPrefix(p, "claude/")
	if !ok {
		return ""
	}
	profile, rel, ok := strings.Cut(rest, "/")
	if !ok || (profile != ".claude" && !strings.HasPrefix(profile, ".claude-")) || !safe(rel) {
		return ""
	}
	rel = claudecode.RemapProjectDir(rel, oldHome, home)
	return filepath.Join(home, profile, filepath.FromSlash(rel))
}

// safe rejects paths that could leave the profile folder.
func safe(rel string) bool {
	clean := path.Clean(rel)
	return clean == rel && rel != "" && !strings.HasPrefix(rel, "../") && rel != ".." && !path.IsAbs(rel)
}

// ProfileDirs are the config dirs to back up: those with sessions, and every profile quipu knows,
// even one without a session yet.
func ProfileDirs(home, profilesFile string) ([]string, error) {
	dirs, err := claudecode.ConfigDirs(home)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, d := range dirs {
		seen[d] = true
	}
	if c, err := profile.Load(home, profilesFile); err == nil {
		for _, p := range c.Profiles {
			if info, err := os.Stat(p.Dir); err == nil && info.IsDir() && !seen[p.Dir] {
				seen[p.Dir] = true
				dirs = append(dirs, p.Dir)
			}
		}
	}
	sort.Strings(dirs)
	return dirs, nil
}

// memoryProject is the project folder name of a file in a project's memory
// ("projects/<name>/memory/…"), or "" for any other file.
func memoryProject(rel string) string {
	rest, ok := strings.CutPrefix(rel, "projects/")
	if !ok {
		return ""
	}
	name, tail, ok := strings.Cut(rest, "/")
	if !ok || !strings.HasPrefix(tail, "memory") {
		return ""
	}
	return name
}

// ProjectMemory is a project whose memory goes into the backup.
type ProjectMemory struct {
	Dir      string   `json:"dir"`      // the project folder name under projects/
	Profiles []string `json:"profiles"` // profiles that keep memory for it
	Files    int      `json:"files"`
}

// ProjectMemories lists the projects whose memory the backup would take, by folder name.
func ProjectMemories(home, profilesFile string) ([]ProjectMemory, error) {
	dirs, err := ProfileDirs(home, profilesFile)
	if err != nil {
		return nil, err
	}
	byDir := map[string]*ProjectMemory{}
	for _, dir := range dirs {
		files, err := claudecode.Setup(dir)
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			project := memoryProject(f.Rel)
			if project == "" || f.Link != "" {
				continue
			}
			pm := byDir[project]
			if pm == nil {
				pm = &ProjectMemory{Dir: project}
				byDir[project] = pm
			}
			pm.Files++
			if name := claudecode.ProfileName(dir); len(pm.Profiles) == 0 || pm.Profiles[len(pm.Profiles)-1] != name {
				pm.Profiles = append(pm.Profiles, name)
			}
		}
	}
	out := make([]ProjectMemory, 0, len(byDir))
	for _, pm := range byDir {
		out = append(out, *pm)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Dir < out[j].Dir })
	return out, nil
}

// Settings are the backup choices quipu keeps in <data dir>/backup.json, shared by the window and
// the command line.
type Settings struct {
	GitDir  string   `json:"gitDir,omitempty"`
	Exclude []string `json:"exclude,omitempty"` // project folder names whose memory stays out
}

// SettingsFile is where the backup settings are kept.
func SettingsFile(dataDir string) string { return filepath.Join(dataDir, "backup.json") }

// LoadSettings reads the backup settings; missing or broken means none.
func LoadSettings(dataDir string) Settings {
	var s Settings
	if b, err := os.ReadFile(SettingsFile(dataDir)); err == nil {
		json.Unmarshal(b, &s)
	}
	return s
}

// Save writes the backup settings.
func (s Settings) Save(dataDir string) error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(SettingsFile(dataDir), append(b, '\n'), 0o600)
}

// Excluded is the set of excluded project folder names.
func (s Settings) Excluded() map[string]bool {
	out := map[string]bool{}
	for _, d := range s.Exclude {
		out[d] = true
	}
	return out
}
