package profile

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/axvilab/quipu/internal/claudecode"
	"github.com/axvilab/quipu/internal/fsx"
)

// Memory is a project's memory as one profile sees it: its own, or shared with another profile
// through the project's autoMemoryDirectory setting.
type Memory struct {
	Shared      bool   `json:"shared"`
	From        string `json:"from,omitempty"`   // the profile whose memory folder is shared
	Own         int    `json:"own"`              // files in this profile's own memory folder
	Source      string `json:"source,omitempty"` // another profile with memory for the project
	SourceFiles int    `json:"sourceFiles"`
}

// ProjectMemory reports the memory of project for profile to.
func ProjectMemory(c *Config, home, project, to string) Memory {
	var m Memory
	p := c.Find(to)
	if p == nil {
		return m
	}
	m.Own = countFiles(claudecode.OwnMemoryDir(p.Dir, project))
	if shared := claudecode.ProjectMemoryDir(home, project); shared != "" {
		m.Shared = true
		for _, o := range c.Profiles {
			if filepath.Clean(claudecode.OwnMemoryDir(o.Dir, project)) == filepath.Clean(shared) {
				m.From = o.Name
			}
		}
	}
	for _, o := range c.Profiles {
		if o.Name == to {
			continue
		}
		if n := countFiles(claudecode.OwnMemoryDir(o.Dir, project)); n > m.SourceFiles {
			m.Source, m.SourceFiles = o.Name, n
		}
	}
	return m
}

func countFiles(dir string) int {
	n := 0
	filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			n++
		}
		return nil
	})
	return n
}

// ErrNoMemory means no other profile has memory for the project.
var ErrNoMemory = errors.New("no other profile has memory for this project")

// ConflictError lists memory files both profiles have with different content.
type ConflictError struct{ Files []string }

func (e *ConflictError) Error() string {
	return fmt.Sprintf("both profiles have different %v; keep one and try again", e.Files)
}

// ShareMemory makes profile to use the project memory of the profile that has the most of it:
// the files only to has move there, then the project's settings.local.json points
// autoMemoryDirectory at that folder for every profile. One journal batch, undone at once. It
// returns the id of one of the changes.
func ShareMemory(c *Config, home, project, to string, j *fsx.Journal) (string, error) {
	m := ProjectMemory(c, home, project, to)
	if m.Source == "" {
		return "", ErrNoMemory
	}
	target := claudecode.OwnMemoryDir(c.Find(m.Source).Dir, project)
	own := claudecode.OwnMemoryDir(c.Find(to).Dir, project)

	type move struct{ from, to string }
	var moves []move
	var drop []string // same file on both sides: only the copy in own goes
	var conflicts []string
	err := filepath.WalkDir(own, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		rel, _ := filepath.Rel(own, path)
		dest := filepath.Join(target, rel)
		mine, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		switch theirs, err := os.ReadFile(dest); {
		case err == nil && bytes.Equal(mine, theirs):
			drop = append(drop, path)
		case err == nil:
			conflicts = append(conflicts, rel)
		default:
			moves = append(moves, move{path, dest})
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if len(conflicts) > 0 {
		sort.Strings(conflicts)
		return "", &ConflictError{conflicts}
	}

	batch, err := fsx.NewBatch()
	if err != nil {
		return "", err
	}
	meta := fsx.Meta{Rule: "share-memory", Title: "share project memory with " + to,
		Params: map[string]string{"name": to, "from": m.Source, "project": filepath.Base(project)}, Batch: batch}
	var id string
	for _, mv := range moves {
		b, err := os.ReadFile(mv.from)
		if err != nil {
			return id, err
		}
		if ch, err := j.Write(mv.to, b, meta); err != nil {
			return id, err
		} else {
			id = ch.ID
		}
		drop = append(drop, mv.from)
	}
	for _, path := range drop {
		if ch, err := j.Remove(path, filepath.Dir(own), meta); err != nil {
			return id, err
		} else {
			id = ch.ID
		}
	}
	ch, err := setMemorySetting(project, claudecode.MemorySettingValue(home, target), j, meta)
	if err != nil {
		return id, err
	}
	if ch != "" {
		id = ch
	}
	return id, nil
}

// UnshareMemory gives profile to its own memory again: a copy of the shared files goes into its
// own folder, and the autoMemoryDirectory setting is removed. One journal batch.
func UnshareMemory(c *Config, home, project, to string, j *fsx.Journal) (string, error) {
	shared := claudecode.ProjectMemoryDir(home, project)
	p := c.Find(to)
	if shared == "" || p == nil {
		return "", nil
	}
	own := claudecode.OwnMemoryDir(p.Dir, project)
	batch, err := fsx.NewBatch()
	if err != nil {
		return "", err
	}
	meta := fsx.Meta{Rule: "unshare-memory", Title: "stop sharing project memory with " + to,
		Params: map[string]string{"name": to, "project": filepath.Base(project)}, Batch: batch}
	var id string
	if filepath.Clean(shared) != filepath.Clean(own) {
		err = filepath.WalkDir(shared, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(shared, path)
			dest := filepath.Join(own, rel)
			if _, err := os.Stat(dest); err == nil {
				return nil // keep what the profile already has
			}
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			ch, err := j.Write(dest, b, meta)
			if err == nil {
				id = ch.ID
			}
			return err
		})
		if err != nil {
			return id, err
		}
	}
	ch, err := setMemorySetting(project, "", j, meta)
	if ch != "" {
		id = ch
	}
	return id, err
}

// setMemorySetting sets (or with "" removes) autoMemoryDirectory in the project's
// settings.local.json through the journal.
func setMemorySetting(project, value string, j *fsx.Journal, meta fsx.Meta) (string, error) {
	path := claudecode.LocalSettings(project)
	current, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	var raw json.RawMessage
	if value != "" {
		raw, _ = json.Marshal(value)
	}
	next, err := claudecode.SetSettingsValue(current, "autoMemoryDirectory", raw)
	if err != nil {
		return "", fmt.Errorf("%s: %w", path, err)
	}
	if bytes.Equal(next, current) {
		return "", nil
	}
	ch, err := j.Write(path, next, meta)
	return ch.ID, err
}
