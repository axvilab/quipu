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
	"time"
)

// ErrChanged means the file was modified after quipu wrote it, so undoing would lose that edit.
var ErrChanged = errors.New("the file was changed after this edit")

// WriteFile replaces path with data atomically, keeping the file's mode (0644 for a new file).
func WriteFile(path string, data []byte) error {
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
	Existed  bool              `json:"existed"`   // false: quipu created the file
	AfterSHA string            `json:"after_sha"` // content hash right after the edit
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
	}
	if err := j.save(c); err != nil {
		return Change{}, err
	}
	if err := WriteFile(path, data); err != nil {
		return Change{}, fmt.Errorf("write %s: %w", path, err)
	}
	return c, nil
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
		if err != nil || hash(current) != g.AfterSHA {
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
	} else if err := os.Remove(c.Path); err != nil {
		return err
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
