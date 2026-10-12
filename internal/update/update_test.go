package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestNewer(t *testing.T) {
	tests := []struct {
		current, tag string
		want         bool
	}{
		{"v0.8.4", "v0.8.5", true},
		{"v0.8.5", "v0.8.5", false},
		{"v0.9.0", "v0.8.12", false},
		{"v0.8.9", "v0.8.10", true},
		{"v0.8.5", "v1.0.0-rc1", true},
		{"dev", "v0.8.5", false},
		{"v0.8.5", "nightly", false},
	}
	for _, tt := range tests {
		if got := Newer(tt.current, tt.tag); got != tt.want {
			t.Errorf("Newer(%q, %q) = %v", tt.current, tt.tag, got)
		}
	}
}

// release serves a fake GitHub release: the latest redirect, the archive and its checksums.
func release(t *testing.T, tag string, files map[string]string, sum string) *httptest.Server {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range files {
		tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg})
		tw.Write([]byte(body))
	}
	tw.Close()
	gz.Close()
	archive := buf.Bytes()
	if sum == "" {
		h := sha256.Sum256(archive)
		sum = hex.EncodeToString(h[:])
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/releases/tag/"+tag, http.StatusFound)
	})
	mux.HandleFunc("/releases/download/"+tag+"/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(sum + "  " + Asset() + "\nffff  quipu_other_os.tar.gz\n"))
	})
	mux.HandleFunc("/releases/download/"+tag+"/"+Asset(), func(w http.ResponseWriter, r *http.Request) {
		w.Write(archive)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestLatestAndInstall(t *testing.T) {
	newQuipu := "#!/bin/sh\necho quipu v0.9.0\n"
	srv := release(t, "v0.9.0", map[string]string{"quipu": newQuipu, "quipu-app": "app", "LICENSE": "x", "../evil": "x"}, "")
	ctx := context.Background()

	tag, err := Latest(ctx, srv.URL)
	if err != nil || tag != "v0.9.0" {
		t.Fatalf("Latest = %q, %v", tag, err)
	}

	dir := t.TempDir()
	exe := filepath.Join(dir, "quipu")
	os.WriteFile(exe, []byte("old"), 0o755)
	if err := Install(ctx, srv.URL, tag, exe, ""); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(exe); string(b) != newQuipu {
		t.Errorf("quipu = %q", b)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "quipu-app")); string(b) != "app" {
		t.Errorf("quipu-app = %q", b)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 2 {
		t.Errorf("left in %s: %v", dir, entries)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dir), "evil")); err == nil {
		t.Error("a path outside the folder was written")
	}
}

func TestInstallRefusesBadDownloads(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name string
		srv  *httptest.Server
	}{
		{"checksum mismatch", release(t, "v0.9.0", map[string]string{"quipu": "#!/bin/sh\n"}, "0000")},
		{"does not start", release(t, "v0.9.0", map[string]string{"quipu": "#!/bin/sh\nexit 3\n"}, "")},
		{"no quipu in archive", release(t, "v0.9.0", map[string]string{"README.md": "x"}, "")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exe := filepath.Join(t.TempDir(), "quipu")
			os.WriteFile(exe, []byte("old"), 0o755)
			if err := Install(ctx, tt.srv.URL, "v0.9.0", exe, ""); err == nil {
				t.Fatal("installed")
			}
			if b, _ := os.ReadFile(exe); string(b) != "old" {
				t.Errorf("quipu replaced with %q", b)
			}
		})
	}
}
