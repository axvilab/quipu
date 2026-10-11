package claudecode

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// SetupFile is a file of a profile's own setup, the part worth backing up: instructions,
// settings, extensions and project memory. Rel is relative to the config dir, with slashes.
type SetupFile struct {
	Rel  string
	Link string // set when the entry is a symlink (profiles share files this way): its target
}

// setupItems are the top-level entries of a config dir that hold the user's setup. Everything
// else (credentials, .claude.json, transcripts, caches, plugins) is never part of it.
var setupItems = []string{"CLAUDE.md", "settings.json", "rules", "skills", "agents", "commands"}

// Setup lists a profile's setup files. Skills synced from claude.ai are left out (claude.ai owns
// them); of projects/ only each project's memory folder is included.
func Setup(configDir string) ([]SetupFile, error) {
	var out []SetupFile
	add := func(path string) error {
		return walkSetup(configDir, path, func(f SetupFile) {
			if !strings.HasPrefix(f.Rel, "skills/synced/") && f.Rel != "skills/synced" {
				out = append(out, f)
			}
		})
	}
	for _, item := range setupItems {
		if err := add(filepath.Join(configDir, item)); err != nil {
			return nil, err
		}
	}
	projects, err := os.ReadDir(filepath.Join(configDir, "projects"))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	for _, p := range projects {
		if p.IsDir() {
			if err := add(filepath.Join(configDir, "projects", p.Name(), "memory")); err != nil {
				return nil, err
			}
		}
	}
	return out, nil
}

// walkSetup reports path and, for a folder, every file under it; symlinks are reported as links
// and not followed.
func walkSetup(configDir, path string, report func(SetupFile)) error {
	rel := func(p string) string {
		r, _ := filepath.Rel(configDir, p)
		return filepath.ToSlash(r)
	}
	fi, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(path)
		if err != nil {
			return err
		}
		report(SetupFile{Rel: rel(path), Link: target})
		return nil
	}
	if !fi.IsDir() {
		report(SetupFile{Rel: rel(path)})
		return nil
	}
	return filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		switch {
		case d.Type()&fs.ModeSymlink != 0:
			target, err := os.Readlink(p)
			if err != nil {
				return err
			}
			report(SetupFile{Rel: rel(p), Link: target})
		case d.Type().IsRegular():
			report(SetupFile{Rel: rel(p)})
		}
		return nil
	})
}

// RemapProjectDir rewrites the project folder in a setup path (projects/<name>/...) for a new
// home: project folders are named after the project's absolute path, which starts with the home.
func RemapProjectDir(rel, oldHome, newHome string) string {
	rest, ok := strings.CutPrefix(rel, "projects/")
	if !ok || oldHome == newHome {
		return rel
	}
	name, tail, _ := strings.Cut(rest, "/")
	from, to := ProjectDirName(oldHome), ProjectDirName(newHome)
	if after, ok := strings.CutPrefix(name, from); ok {
		name = to + after
	}
	return "projects/" + name + "/" + tail
}
