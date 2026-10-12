package profile

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// SetLabel sets how a profile is shown. The name, and so its config dir, stays: moving the dir
// would break open sessions, folder rules, shared memory and the stored history.
func (c *Config) SetLabel(name, label string) error {
	label = strings.TrimSpace(label)
	if len(label) > 40 {
		return errors.New("a label is at most 40 characters")
	}
	for i := range c.Profiles {
		if c.Profiles[i].Name == name {
			if label == name {
				label = ""
			}
			c.Profiles[i].Label = label
			return nil
		}
	}
	return fmt.Errorf("no profile %q", name)
}

// Shared tells which shareable items a profile takes from the default profile.
func (c *Config) Shared(name string) []string {
	p := c.Find(name)
	if p == nil || name == Default {
		return nil
	}
	var out []string
	for _, item := range Shareable {
		if target, err := os.Readlink(filepath.Join(p.Dir, item)); err == nil &&
			filepath.Clean(target) == filepath.Join(c.home, ".claude", item) {
			out = append(out, item)
		}
	}
	return out
}

// OwnError means the profile has its own item of that name, which sharing would hide.
type OwnError struct {
	Item  string
	Files int
}

func (e *OwnError) Error() string {
	return fmt.Sprintf("the profile has its own %s (%d files); move or delete them to share", e.Item, e.Files)
}

// Share makes a profile use the default profile's item through a link. A profile that has its own
// item of that name keeps it: sharing stops with OwnError, unless what it has is empty.
func (c *Config) Share(name, item string) error {
	p, err := c.shareable(name, item)
	if err != nil {
		return err
	}
	src := filepath.Join(c.home, ".claude", item)
	dst := filepath.Join(p.Dir, item)
	if _, err := os.Stat(src); err != nil {
		// Nothing to share yet: start it empty in the default profile, so both use it from now on.
		if item == "CLAUDE.md" {
			err = os.WriteFile(src, nil, 0o644)
		} else {
			err = os.MkdirAll(src, 0o755)
		}
		if err != nil {
			return err
		}
	}
	if fi, err := os.Lstat(dst); err == nil {
		if fi.Mode()&fs.ModeSymlink != 0 {
			return nil // shared already
		}
		if n := countFiles(dst); n > 0 || (!fi.IsDir() && fi.Size() > 0) {
			if !fi.IsDir() {
				n = 1
			}
			return &OwnError{Item: item, Files: n}
		}
		if err := os.RemoveAll(dst); err != nil {
			return err
		}
	}
	if err := os.Symlink(src, dst); err != nil {
		return err
	}
	if !contains(p.Links, dst) {
		p.Links = append(p.Links, dst)
	}
	return nil
}

// Unshare gives a profile its own copy of a shared item, so it keeps what it had.
func (c *Config) Unshare(name, item string) error {
	p, err := c.shareable(name, item)
	if err != nil {
		return err
	}
	dst := filepath.Join(p.Dir, item)
	fi, err := os.Lstat(dst)
	if err != nil || fi.Mode()&fs.ModeSymlink == 0 {
		return nil // not shared
	}
	src, err := filepath.EvalSymlinks(dst)
	if err != nil {
		return os.Remove(dst) // a link to nothing: just drop it
	}
	if err := os.Remove(dst); err != nil {
		return err
	}
	if err := copyTree(src, dst); err != nil {
		return err
	}
	var links []string
	for _, l := range p.Links {
		if l != dst {
			links = append(links, l)
		}
	}
	p.Links = links
	return nil
}

func (c *Config) shareable(name, item string) (*Profile, error) {
	if name == Default {
		return nil, errors.New("the default profile is the one others share from")
	}
	if !contains(Shareable, item) {
		return nil, fmt.Errorf("cannot share %q; shareable: %s", item, strings.Join(Shareable, ", "))
	}
	for i := range c.Profiles {
		if c.Profiles[i].Name == name {
			return &c.Profiles[i], nil
		}
	}
	return nil, fmt.Errorf("no profile %q", name)
}

// copyTree copies a file or a folder with everything in it.
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, info.Mode().Perm())
	})
}
