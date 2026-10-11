package backup

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/axvilab/quipu/internal/fsx"
)

// Planned is a file restore would write.
type Planned struct {
	Path  string // where on this machine
	From  string // path in the backup layout
	State string // "new", "changed" or "same"
	Data  []byte
	Exec  bool
}

// PlannedLink is a symlink restore would create.
type PlannedLink struct {
	Path   string
	Target string
	State  string // "new", "same", or "taken" (something else is there; left alone)
}

// Plan is what restoring a backup would do on this machine. Nothing is written by planning.
type Plan struct {
	Files   []Planned
	Links   []PlannedLink
	Skipped []string // paths in the backup that are not part of quipu's layout
}

// PlanRestore compares a backup with this machine. Paths are moved to the current home when the
// backup came from another one.
func PlanRestore(m Manifest, entries []Entry, home, profilesFile string) Plan {
	var p Plan
	for _, e := range entries {
		to := target(e.Path, home, profilesFile, m.Home)
		if to == "" {
			p.Skipped = append(p.Skipped, e.Path)
			continue
		}
		state := "new"
		if cur, err := os.ReadFile(to); err == nil {
			state = "changed"
			if bytes.Equal(cur, e.Data) {
				state = "same"
			}
		}
		p.Files = append(p.Files, Planned{Path: to, From: e.Path, State: state, Data: e.Data, Exec: e.Exec})
	}
	for _, l := range m.Links {
		to := target(l.Path, home, profilesFile, m.Home)
		if to == "" {
			p.Skipped = append(p.Skipped, l.Path)
			continue
		}
		dest := l.Target
		if rest, ok := strings.CutPrefix(dest, m.Home+string(filepath.Separator)); ok && m.Home != "" {
			dest = filepath.Join(home, rest)
		}
		state := "new"
		if cur, err := os.Readlink(to); err == nil && cur == dest {
			state = "same"
		} else if _, err := os.Lstat(to); err == nil {
			state = "taken"
		}
		p.Links = append(p.Links, PlannedLink{Path: to, Target: dest, State: state})
	}
	sort.Slice(p.Files, func(i, j int) bool { return p.Files[i].Path < p.Files[j].Path })
	return p
}

// Count returns how many files restore would create and change.
func (p Plan) Count() (created, changed int) {
	for _, f := range p.Files {
		switch f.State {
		case "new":
			created++
		case "changed":
			changed++
		}
	}
	return created, changed
}

// Apply writes the new and changed files of the plan through the journal as one batch, so the
// whole restore is undone at once, and creates missing symlinks. It returns the id of a change
// of the batch ("" if nothing was written).
func (p Plan) Apply(j *fsx.Journal, version string) (string, error) {
	batch, err := fsx.NewBatch()
	if err != nil {
		return "", err
	}
	meta := fsx.Meta{Rule: "restore", Title: "Restored a backup", Params: map[string]string{"version": version}, Batch: batch}
	var id string
	for _, f := range p.Files {
		if f.State == "same" {
			continue
		}
		c, err := j.Write(f.Path, f.Data, meta)
		if err != nil {
			return id, err
		}
		id = c.ID
		if f.Exec {
			os.Chmod(f.Path, 0o755)
		}
	}
	for _, l := range p.Links {
		if l.State != "new" {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(l.Path), 0o755); err != nil {
			return id, err
		}
		if err := os.Symlink(l.Target, l.Path); err != nil {
			return id, err
		}
	}
	return id, nil
}
