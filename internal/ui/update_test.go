package ui

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/axvilab/quipu/internal/update"
)

func TestUpdateAPI(t *testing.T) {
	// A fake GitHub with release v0.9.0 for this system.
	newQuipu := "#!/bin/sh\necho quipu v0.9.0\n"
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	tw.WriteHeader(&tar.Header{Name: "quipu", Mode: 0o755, Size: int64(len(newQuipu)), Typeflag: tar.TypeReg})
	tw.Write([]byte(newQuipu))
	tw.Close()
	gz.Close()
	sum := sha256.Sum256(buf.Bytes())
	mux := http.NewServeMux()
	mux.HandleFunc("/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/releases/tag/v0.9.0", http.StatusFound)
	})
	mux.HandleFunc("/releases/download/v0.9.0/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(hex.EncodeToString(sum[:]) + "  " + update.Asset() + "\n"))
	})
	mux.HandleFunc("/releases/download/v0.9.0/"+update.Asset(), func(w http.ResponseWriter, r *http.Request) {
		w.Write(buf.Bytes())
	})
	gh := httptest.NewServer(mux)
	defer gh.Close()

	s := testServer(t)
	s.version = "v0.8.5"
	s.UpdateBase = gh.URL
	s.SelfBinary = filepath.Join(s.home, "bin", "quipu")
	os.MkdirAll(filepath.Dir(s.SelfBinary), 0o755)
	os.WriteFile(s.SelfBinary, []byte("old"), 0o755)
	restarted := make(chan bool, 1)
	s.Restart = func() { restarted <- true }
	h := s.Handler()
	cookie := &http.Cookie{Name: cookieName, Value: s.token}

	// Nothing is asked before the user checks.
	var info UpdateInfo
	getJSON(t, h, "/api/update", cookie, &info)
	if info.Available || info.Checked != "" || info.Auto || !info.CanInstall || !info.Restarts {
		t.Fatalf("before a check: %+v", info)
	}
	rec := postJSON(h, "/api/update/check", `{}`, cookie)
	json.Unmarshal(rec.Body.Bytes(), &info)
	if !info.Available || info.Latest != "v0.9.0" || !strings.HasSuffix(info.Notes, "/releases/tag/v0.9.0") {
		t.Fatalf("after a check: %s", rec.Body.String())
	}

	rec = postJSON(h, "/api/update/install", `{}`, cookie)
	if !strings.Contains(rec.Body.String(), `"update-installed"`) {
		t.Fatalf("install: %s", rec.Body.String())
	}
	if b, _ := os.ReadFile(s.SelfBinary); string(b) != newQuipu {
		t.Errorf("binary = %q", b)
	}
	select {
	case <-restarted:
	case <-time.After(5 * time.Second):
		t.Error("no restart after installing")
	}

	if rec := postJSON(h, "/api/update/auto", `{"on":true}`, cookie); !strings.Contains(rec.Body.String(), "update-auto-on") {
		t.Errorf("auto: %s", rec.Body.String())
	}
	getJSON(t, h, "/api/update", cookie, &info)
	if !info.Auto {
		t.Errorf("auto not saved: %+v", info)
	}

	// A dev build never offers to install.
	s.version = "dev"
	getJSON(t, h, "/api/update", cookie, &info)
	if info.Available || info.CanInstall {
		t.Errorf("dev build: %+v", info)
	}
}
