package backup

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const manifestName = "manifest.json"

// maxEntry and maxTotal bound what restore reads from an archive or folder: a setup is small.
const (
	maxEntry = 4 << 20
	maxTotal = 64 << 20
)

// WriteArchive writes the backup as a .tar.gz: manifest first, then the files.
func WriteArchive(w io.Writer, m Manifest, entries []Entry) error {
	gz := gzip.NewWriter(w)
	tw := tar.NewWriter(gz)
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	if err := writeFile(tw, manifestName, b, 0o600, m); err != nil {
		return err
	}
	for _, e := range entries {
		mode := int64(0o600)
		if e.Exec {
			mode = 0o700
		}
		if err := writeFile(tw, e.Path, e.Data, mode, m); err != nil {
			return err
		}
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return gz.Close()
}

func writeFile(tw *tar.Writer, name string, data []byte, mode int64, m Manifest) error {
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: mode, Size: int64(len(data)), ModTime: m.Created, Typeflag: tar.TypeReg}); err != nil {
		return err
	}
	_, err := tw.Write(data)
	return err
}

// ReadArchive reads a backup written by WriteArchive. Anything but regular files is ignored.
func ReadArchive(r io.Reader) (Manifest, []Entry, error) {
	var m Manifest
	gz, err := gzip.NewReader(r)
	if err != nil {
		return m, nil, fmt.Errorf("not a quipu backup: %w", err)
	}
	tr := tar.NewReader(gz)
	var entries []Entry
	total := 0
	found := false
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return m, nil, err
		}
		if h.Typeflag != tar.TypeReg {
			continue
		}
		if h.Size > maxEntry || total+int(h.Size) > maxTotal {
			return m, nil, errors.New("backup too big")
		}
		b, err := io.ReadAll(io.LimitReader(tr, maxEntry))
		if err != nil {
			return m, nil, err
		}
		total += len(b)
		if h.Name == manifestName {
			if err := json.Unmarshal(b, &m); err != nil {
				return m, nil, fmt.Errorf("manifest: %w", err)
			}
			found = true
			continue
		}
		entries = append(entries, Entry{Path: h.Name, Data: b, Exec: h.Mode&0o111 != 0})
	}
	if !found {
		return m, nil, errors.New("not a quipu backup: no manifest")
	}
	return m, entries, nil
}

// ReadDir reads a backup laid out in a folder, such as a git working copy.
func ReadDir(dir string) (Manifest, []Entry, error) {
	var m Manifest
	b, err := os.ReadFile(filepath.Join(dir, manifestName))
	if err != nil {
		return m, nil, fmt.Errorf("not a quipu backup: %w", err)
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return m, nil, fmt.Errorf("manifest: %w", err)
	}
	var entries []Entry
	total := 0
	for _, top := range []string{"claude", "quipu"} {
		err := filepath.WalkDir(filepath.Join(dir, top), func(p string, d os.DirEntry, err error) error {
			if errors.Is(err, os.ErrNotExist) {
				return filepath.SkipDir
			}
			if err != nil || !d.Type().IsRegular() {
				return err
			}
			fi, err := d.Info()
			if err != nil {
				return err
			}
			if fi.Size() > maxEntry || total+int(fi.Size()) > maxTotal {
				return errors.New("backup too big")
			}
			data, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			total += len(data)
			rel, _ := filepath.Rel(dir, p)
			entries = append(entries, Entry{Path: filepath.ToSlash(rel), Data: data, Exec: fi.Mode()&0o111 != 0})
			return nil
		})
		if err != nil {
			return m, nil, err
		}
	}
	return m, entries, nil
}

// WriteDir mirrors the backup into dir: writes every file and the manifest, and removes files of
// an earlier backup that are gone now. Other top-level files (README, .git) are left alone.
func WriteDir(dir string, m Manifest, entries []Entry) error {
	keep := map[string]bool{}
	for _, e := range entries {
		keep[e.Path] = true
		full := filepath.Join(dir, filepath.FromSlash(e.Path))
		if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
			return err
		}
		mode := os.FileMode(0o600)
		if e.Exec {
			mode = 0o700
		}
		if err := os.WriteFile(full, e.Data, mode); err != nil {
			return err
		}
		if err := os.Chmod(full, mode); err != nil {
			return err
		}
	}
	for _, top := range []string{"claude", "quipu"} {
		root := filepath.Join(dir, top)
		filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(dir, p)
			if !keep[filepath.ToSlash(rel)] {
				os.Remove(p)
			}
			return nil
		})
		removeEmptyDirs(root)
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, manifestName), append(b, '\n'), 0o600)
}

// removeEmptyDirs removes empty folders under root, deepest first.
func removeEmptyDirs(root string) {
	var dirs []string
	filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err == nil && d.IsDir() {
			dirs = append(dirs, p)
		}
		return nil
	})
	for i := len(dirs) - 1; i >= 0; i-- {
		if !strings.EqualFold(dirs[i], root) {
			os.Remove(dirs[i]) // fails when not empty, which is fine
		}
	}
}
