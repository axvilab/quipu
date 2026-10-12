package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/axvilab/quipu/internal/update"
)

// TestUpdateRestartsInPlace runs a real quipu daemon, offers it a newer release from a fake
// GitHub, installs it through the API and checks that the same process (same PID) now answers
// as the new version, with the window app replaced too. It builds quipu twice, so it runs only
// when QUIPU_E2E is set (CI does, on Linux and macOS).
func TestUpdateRestartsInPlace(t *testing.T) {
	if os.Getenv("QUIPU_E2E") == "" {
		t.Skip("set QUIPU_E2E=1 to run the self-update test")
	}
	tmp := t.TempDir()
	build := func(version, out string) {
		cmd := exec.Command("go", "build", "-ldflags", "-X main.version="+version, "-o", out, ".")
		if b, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("build %s: %v\n%s", version, err, b)
		}
	}
	home := filepath.Join(tmp, "home")
	bin := filepath.Join(home, ".local", "bin")
	os.MkdirAll(bin, 0o755)
	build("v0.0.1", filepath.Join(bin, "quipu"))
	build("v0.0.2", filepath.Join(tmp, "new-quipu"))

	// The release archive, laid out like the release workflow's.
	files := map[string]string{}
	b, _ := os.ReadFile(filepath.Join(tmp, "new-quipu"))
	files["quipu"] = string(b)
	if runtime.GOOS == "darwin" {
		files["Quipu.app/Contents/version"] = "new"
		os.MkdirAll(filepath.Join(home, "Applications", "Quipu.app", "Contents"), 0o755)
		os.WriteFile(filepath.Join(home, "Applications", "Quipu.app", "Contents", "version"), []byte("old"), 0o644)
	} else {
		files["quipu-app"] = "new window"
	}
	archive := tarGz(t, files)
	sum := sha256.Sum256(archive)
	mux := http.NewServeMux()
	mux.HandleFunc("/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/releases/tag/v0.0.2", http.StatusFound)
	})
	mux.HandleFunc("/releases/download/v0.0.2/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s  %s\n", hex.EncodeToString(sum[:]), update.Asset())
	})
	mux.HandleFunc("/releases/download/v0.0.2/"+update.Asset(), func(w http.ResponseWriter, r *http.Request) {
		w.Write(archive)
	})
	gh := httptest.NewServer(mux)
	defer gh.Close()

	port := freePort(t)
	daemon := exec.Command(filepath.Join(bin, "quipu"), "daemon", "--ui-port", fmt.Sprint(port))
	daemon.Env = append(os.Environ(), "HOME="+home, "XDG_DATA_HOME="+filepath.Join(home, ".local", "share"),
		"XDG_CONFIG_HOME="+filepath.Join(home, ".config"), "QUIPU_UPDATE_BASE="+gh.URL)
	var logs bytes.Buffer
	daemon.Stdout, daemon.Stderr = &logs, &logs
	if err := daemon.Start(); err != nil {
		t.Fatal(err)
	}
	exited := make(chan error, 1)
	go func() { exited <- daemon.Wait() }()
	defer func() {
		daemon.Process.Kill()
		<-exited
		if t.Failed() {
			t.Logf("daemon output:\n%s", logs.String())
		}
	}()

	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 5 * time.Minute}
	token := waitFor(t, 60*time.Second, func() (string, bool) {
		var found string
		filepath.WalkDir(home, func(p string, d os.DirEntry, err error) error {
			if err == nil && d.Name() == "ui-token" {
				b, _ := os.ReadFile(p)
				found = strings.TrimSpace(string(b))
			}
			return nil
		})
		if found == "" {
			return "", false
		}
		resp, err := client.Get(base + "/?t=" + found)
		if err != nil {
			return "", false
		}
		resp.Body.Close()
		return found, resp.StatusCode == http.StatusOK
	})
	_ = token

	post := func(path string) string {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, base+path, strings.NewReader("{}"))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", base)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("POST %s: %v", path, err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return string(b)
	}
	if got := post("/api/update/check"); !strings.Contains(got, `"available":true`) {
		t.Fatalf("check: %s", got)
	}
	if got := post("/api/update/install"); !strings.Contains(got, `"update-installed"`) {
		t.Fatalf("install: %s", got)
	}

	// The same process comes back as the new version.
	waitFor(t, 60*time.Second, func() (string, bool) {
		select {
		case err := <-exited:
			exited <- err
			t.Fatalf("the daemon exited instead of restarting in place: %v", err)
		default:
		}
		resp, err := client.Get(base + "/api/session")
		if err != nil {
			return "", false
		}
		defer resp.Body.Close()
		var s struct{ Version string }
		json.NewDecoder(resp.Body).Decode(&s)
		return s.Version, s.Version == "v0.0.2"
	})
	if out, err := exec.Command(filepath.Join(bin, "quipu"), "version").Output(); err != nil || !strings.Contains(string(out), "v0.0.2") {
		t.Errorf("quipu on disk: %q, %v", out, err)
	}
	if runtime.GOOS == "darwin" {
		if b, _ := os.ReadFile(filepath.Join(home, "Applications", "Quipu.app", "Contents", "version")); string(b) != "new" {
			t.Errorf("Quipu.app not replaced: %q", b)
		}
	} else if b, _ := os.ReadFile(filepath.Join(bin, "quipu-app")); string(b) != "new window" {
		t.Errorf("quipu-app not replaced: %q", b)
	}
}

func tarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range files {
		tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg})
		tw.Write([]byte(body))
	}
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

// waitFor polls f until it reports done or the time runs out.
func waitFor(t *testing.T, limit time.Duration, f func() (string, bool)) string {
	t.Helper()
	deadline := time.Now().Add(limit)
	for time.Now().Before(deadline) {
		if v, ok := f(); ok {
			return v
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("timed out after %s", limit)
	return ""
}
