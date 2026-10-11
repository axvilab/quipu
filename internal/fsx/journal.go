// Package fsx writes user files safely: atomically, with a backup of what was there before, and
// with a way to undo the change as long as nobody has edited the file since.
package fsx

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ErrChanged means the file was modified after quipu wrote it, so undoing would lose that edit.
var ErrChanged = errors.New("the file was changed after this edit")

// WriteFile replaces path with data atomically, keeping the file's mode (0644 for a new file).
// A symlink is followed and its target replaced: profiles share files through symlinks, and
// renaming over the link would silently stop the sharing.
func WriteFile(path string, data []byte) error {
	if real, err := filepath.EvalSymlinks(path); err == nil {
		path = real
	}
	mode := fs.FileMode(0o644)
	if fi, err := os.Stat(path); err == nil {
		mode = fi.Mode().Perm()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".quipu-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op after a successful rename
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), mode); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// Meta says why quipu changes a file.
type Meta struct {
	Rule   string
	Title  string            // English, for the CLI and old entries
	Params map[string]string // lets the UI show the title in the user's language
	Batch  string            // changes made by one action share it and are undone together
}

// NewBatch returns an id that groups the changes of one action.
func NewBatch() (string, error) {
	return newID()
}

// Change is one file edit made by quipu.
type Change struct {
	ID       string            `json:"id"`
	Path     string            `json:"path"`
	Rule     string            `json:"rule"`
	Title    string            `json:"title"`
	Params   map[string]string `json:"params,omitempty"`
	Batch    string            `json:"batch,omitempty"`
	Time     time.Time         `json:"time"`
	Existed  bool              `json:"existed"`            // false: quipu created the file
	Deleted  bool              `json:"deleted"`            // quipu removed the file
	MadeDir  string            `json:"made_dir,omitempty"` // topmost folder quipu created for a new file
	AfterSHA string            `json:"after_sha"`          // content hash right after the edit
	Undone   bool              `json:"undone"`
}

// Journal keeps a backup of every file before quipu changes it.
// Each change is a directory <dir>/<id>/ with meta.json and, if the file existed, original.
type Journal struct {
	dir string
}

// OpenJournal uses dir (created if missing, private to the user) for backups.
func OpenJournal(dir string) (*Journal, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return &Journal{dir: dir}, nil
}

// Write backs up path, replaces it with data and records the change.
func (j *Journal) Write(path string, data []byte, m Meta) (Change, error) {
	id, err := newID()
	if err != nil {
		return Change{}, err
	}
	c := Change{ID: id, Path: path, Rule: m.Rule, Title: m.Title, Params: m.Params, Batch: m.Batch,
		Time: time.Now(), AfterSHA: hash(data)}
	dir := filepath.Join(j.dir, id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return Change{}, err
	}

	original, err := os.ReadFile(path)
	switch {
	case err == nil:
		c.Existed = true
		if err := os.WriteFile(filepath.Join(dir, "original"), original, 0o600); err != nil {
			return Change{}, err
		}
	case !errors.Is(err, fs.ErrNotExist):
		return Change{}, err
	default:
		c.MadeDir = topMissingDir(path)
	}
	if err := j.save(c); err != nil {
		return Change{}, err
	}
	if err := WriteFile(path, data); err != nil {
		return Change{}, fmt.Errorf("write %s: %w", path, err)
	}
	return c, nil
}

// Remove backs up path and deletes it, then removes the directories left empty by that, up to but
// not including stop. Undo puts the file back.
func (j *Journal) Remove(path, stop string, m Meta) (Change, error) {
	id, err := newID()
	if err != nil {
		return Change{}, err
	}
	original, err := os.ReadFile(path)
	if err != nil {
		return Change{}, err
	}
	c := Change{ID: id, Path: path, Rule: m.Rule, Title: m.Title, Params: m.Params, Batch: m.Batch,
		Time: time.Now(), Existed: true, Deleted: true}
	dir := filepath.Join(j.dir, id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return Change{}, err
	}
	if err := os.WriteFile(filepath.Join(dir, "original"), original, 0o600); err != nil {
		return Change{}, err
	}
	if err := j.save(c); err != nil {
		return Change{}, err
	}
	if err := os.Remove(path); err != nil {
		return Change{}, err
	}
	for d := filepath.Dir(path); d != stop && strings.HasPrefix(d, stop+string(filepath.Separator)); d = filepath.Dir(d) {
		if os.Remove(d) != nil { // not empty
			break
		}
	}
	return c, nil
}

// Prune deletes the backups of changes older than age; those changes can no longer be undone.
func (j *Journal) Prune(age time.Duration) error {
	all, err := j.List()
	if err != nil {
		return err
	}
	for _, c := range all {
		if time.Since(c.Time) > age {
			if err := os.RemoveAll(filepath.Join(j.dir, c.ID)); err != nil {
				return err
			}
		}
	}
	return nil
}

// Undo restores the file as it was before change id, together with every other file of the
// same batch. It refuses with ErrChanged, before restoring anything, when any of those files no
// longer has the content quipu wrote.
func (j *Journal) Undo(id string) error {
	c, err := j.load(id)
	if err != nil {
		return err
	}
	group := []Change{c}
	if c.Batch != "" {
		all, err := j.List()
		if err != nil {
			return err
		}
		group = group[:0]
		for _, o := range all {
			if o.Batch == c.Batch {
				group = append(group, o)
			}
		}
	}
	var todo []Change
	for _, g := range group {
		if g.Undone {
			continue
		}
		current, err := os.ReadFile(g.Path)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		switch {
		case g.Deleted && err == nil: // someone created the file again
			return ErrChanged
		case !g.Deleted && (err != nil || hash(current) != g.AfterSHA):
			return ErrChanged
		}
		todo = append(todo, g)
	}
	for _, g := range todo {
		if err := j.restore(g); err != nil {
			return err
		}
	}
	return nil
}

func (j *Journal) restore(c Change) error {
	if c.Existed {
		original, err := os.ReadFile(filepath.Join(j.dir, c.ID, "original"))
		if err != nil {
			return err
		}
		if err := WriteFile(c.Path, original); err != nil {
			return err
		}
	} else {
		if err := os.Remove(c.Path); err != nil {
			return err
		}
		// Remove the folders quipu made for the file, if nothing else was put there since.
		for d := filepath.Dir(c.Path); c.MadeDir != "" && strings.HasPrefix(d+string(filepath.Separator), c.MadeDir+string(filepath.Separator)); d = filepath.Dir(d) {
			if os.Remove(d) != nil {
				break
			}
		}
	}
	c.Undone = true
	return j.save(c)
}

// List returns all recorded changes, newest first.
func (j *Journal) List() ([]Change, error) {
	entries, err := os.ReadDir(j.dir)
	if err != nil {
		return nil, err
	}
	var out []Change
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if c, err := j.load(e.Name()); err == nil {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Time.After(out[b].Time) })
	return out, nil
}

func (j *Journal) save(c Change) error {
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(j.dir, c.ID, "meta.json"), b, 0o600)
}

func (j *Journal) load(id string) (Change, error) {
	var c Change
	if filepath.Base(id) != id || id == "" || id == "." || id == ".." {
		return c, fmt.Errorf("bad change id %q", id)
	}
	b, err := os.ReadFile(filepath.Join(j.dir, id, "meta.json"))
	if err != nil {
		return c, err
	}
	return c, json.Unmarshal(b, &c)
}

// topMissingDir is the outermost folder that writing path would create, or "" if its folder exists.
func topMissingDir(path string) string {
	top := ""
	for d := filepath.Dir(path); ; d = filepath.Dir(d) {
		if _, err := os.Stat(d); err == nil || d == filepath.Dir(d) {
			return top
		}
		top = d
	}
}

// newID is sortable by time and unique: a timestamp plus random bytes.
func newID() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return time.Now().UTC().Format("20060102T150405") + "-" + hex.EncodeToString(b), nil
}

func hash(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
