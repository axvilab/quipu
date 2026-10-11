// Package backup saves the Claude Code setup of every profile (instructions, settings, extensions,
// project memory) and quipu's profiles to an archive or a git working copy, and plans its restore.
// Credentials, transcripts and caches are never part of it; see docs/plans/2026-10-11-backup.md.
package backup

import (
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/axvilab/quipu/internal/claudecode"
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
func Collect(home, profilesFile, version string) (Manifest, []Entry, error) {
	m := Manifest{Version: version, Created: time.Now().UTC(), Home: home, Links: []Link{}}
	var entries []Entry
	dirs, err := claudecode.ConfigDirs(home)
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
