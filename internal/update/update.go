// Package update finds a newer quipu release and installs it over the running binary the way
// install.sh does: the same archive from GitHub Releases, checked against checksums.txt (ADR 0008).
package update

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// DefaultBase is the repository releases come from.
const DefaultBase = "https://github.com/axvilab/quipu"

// Latest returns the tag of the newest release. It reads where GitHub redirects /releases/latest
// rather than calling the API, whose limit without a login is easy to hit.
func Latest(ctx context.Context, base string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, base+"/releases/latest", nil)
	if err != nil {
		return "", err
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	resp.Body.Close()
	loc := resp.Header.Get("Location")
	_, tag, ok := strings.Cut(loc, "/releases/tag/")
	if !ok || tag == "" || strings.Contains(tag, "/") {
		return "", fmt.Errorf("no release found at %s (status %d)", base, resp.StatusCode)
	}
	return tag, nil
}

// Newer reports whether tag is a later version than current, both like v0.8.5. A build without a
// version (dev) is never behind.
func Newer(current, tag string) bool {
	c, ok1 := parse(current)
	t, ok2 := parse(tag)
	if !ok1 || !ok2 {
		return false
	}
	for i := range c {
		if t[i] != c[i] {
			return t[i] > c[i]
		}
	}
	return false
}

func parse(v string) ([3]int, bool) {
	var out [3]int
	v, _, _ = strings.Cut(strings.TrimPrefix(v, "v"), "-")
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return out, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return out, false
		}
		out[i] = n
	}
	return out, true
}

// Asset is the archive name for this system, as the release workflow names it.
func Asset() string { return fmt.Sprintf("quipu_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH) }

// Install downloads release tag, checks it against the release's checksums.txt and puts quipu in
// place of exe, with quipu-app next to it and, on macOS, Quipu.app in appsDir when the archive has
// them. The new quipu must start before it replaces the old one.
func Install(ctx context.Context, base, tag, exe, appsDir string) error {
	exe, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return err
	}
	dir := filepath.Dir(exe)
	stage, err := os.MkdirTemp(dir, ".quipu-update-")
	if err != nil {
		return fmt.Errorf("cannot write to %s: %w", dir, err)
	}
	defer os.RemoveAll(stage)

	download := base + "/releases/download/" + tag + "/"
	sums, err := fetch(ctx, download+"checksums.txt")
	if err != nil {
		return err
	}
	want := checksum(string(sums), Asset())
	if want == "" {
		return fmt.Errorf("no checksum for %s in checksums.txt", Asset())
	}
	archive := filepath.Join(stage, Asset())
	got, err := fetchFile(ctx, download+Asset(), archive)
	if err != nil {
		return err
	}
	if got != want {
		return fmt.Errorf("checksum mismatch for %s: the download is corrupt or was tampered with", Asset())
	}
	if err := extract(archive, stage); err != nil {
		return err
	}

	newExe := filepath.Join(stage, "quipu")
	check, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if out, err := exec.CommandContext(check, newExe, "version").CombinedOutput(); err != nil {
		return fmt.Errorf("the new quipu does not start here: %v %s", err, strings.TrimSpace(string(out)))
	}
	// quipu goes last: if anything before it fails, the running version stays whole.
	if app := filepath.Join(stage, "quipu-app"); exists(app) {
		if err := os.Rename(app, filepath.Join(dir, "quipu-app")); err != nil {
			return err
		}
	}
	if bundle := filepath.Join(stage, "Quipu.app"); exists(bundle) && appsDir != "" {
		if err := replaceDir(bundle, filepath.Join(appsDir, "Quipu.app")); err != nil {
			return err
		}
	}
	return os.Rename(newExe, exe)
}

func fetch(ctx context.Context, url string) ([]byte, error) {
	resp, err := get(ctx, url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}

// fetchFile saves url to file and returns its SHA-256.
func fetchFile(ctx context.Context, url, file string) (string, error) {
	resp, err := get(ctx, url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	f, err := os.Create(file)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(f, h), resp.Body); err != nil {
		return "", fmt.Errorf("download %s: %w", url, err)
	}
	return hex.EncodeToString(h.Sum(nil)), f.Close()
}

func get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("download %s: %s", url, resp.Status)
	}
	return resp, nil
}

// checksum finds asset's line in a sha256sum listing.
func checksum(sums, asset string) string {
	for _, line := range strings.Split(sums, "\n") {
		if sum, name, ok := strings.Cut(strings.TrimSpace(line), "  "); ok && strings.TrimPrefix(name, "*") == asset {
			return sum
		}
	}
	return ""
}

// extract unpacks quipu, quipu-app and Quipu.app from the archive into dir; anything else in it,
// and any path that would land outside dir, is skipped.
func extract(archive, dir string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		name := path.Clean(strings.TrimPrefix(h.Name, "./"))
		if name != "quipu" && name != "quipu-app" && name != "Quipu.app" && !strings.HasPrefix(name, "Quipu.app/") {
			continue
		}
		target := filepath.Join(dir, filepath.FromSlash(name))
		switch h.Typeflag {
		case tar.TypeDir:
			err = os.MkdirAll(target, 0o755)
		case tar.TypeSymlink:
			if filepath.IsAbs(h.Linkname) || strings.HasPrefix(path.Clean(path.Join(path.Dir(name), h.Linkname)), "..") {
				continue
			}
			os.MkdirAll(filepath.Dir(target), 0o755)
			err = os.Symlink(h.Linkname, target)
		case tar.TypeReg:
			err = writeFile(target, tr, os.FileMode(h.Mode)&0o755|0o600)
		}
		if err != nil {
			return err
		}
	}
}

func writeFile(file string, r io.Reader, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(file, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, r); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// replaceDir moves src to dst, putting the old dst aside until the move succeeds.
func replaceDir(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	old := dst + ".old"
	os.RemoveAll(old)
	hadOld := os.Rename(dst, old) == nil
	if err := os.Rename(src, dst); err != nil {
		if hadOld {
			os.Rename(old, dst)
		}
		return err
	}
	return os.RemoveAll(old)
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
