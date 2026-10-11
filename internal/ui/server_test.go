package ui

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/axvilab/quipu/internal/claudecode"
	"github.com/axvilab/quipu/internal/store"
)

func testServer(t *testing.T) *Server {
	t.Helper()
	home := t.TempDir()
	proj := filepath.Join(home, "src", "demo")
	os.MkdirAll(proj, 0o755)
	os.WriteFile(filepath.Join(proj, "CLAUDE.md"), []byte("Project rules.\n"+strings.Repeat("line\n", 250)), 0o644)

	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	r := claudecode.Response{MessageID: "m1", SessionID: "s1", Cwd: proj, Timestamp: time.Now().Add(-time.Hour)}
	r.Usage.CacheCreationTokens = 1200
	r.CacheMiss = &claudecode.CacheMiss{Type: "model_changed"}
	src := store.Source{Path: "/t/s1.jsonl", Profile: "default", ProjectDir: claudecode.ProjectDirName(proj), SessionID: "s1", Kind: claudecode.KindSession}
	if err := st.WriteResponses(t.Context(), src, []claudecode.Response{r}); err != nil {
		t.Fatal(err)
	}

	s, err := New(home, filepath.Join(home, ".claude"), "default", filepath.Join(home, "data"), st)
	if err != nil {
		t.Fatal(err)
	}
	s.addr = "127.0.0.1:4242"
	return s
}

func get(h http.Handler, path, host string, cookie *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", path, nil)
	req.Host = host
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestGuard(t *testing.T) {
	s := testServer(t)
	h := s.Handler()

	if rec := get(h, "/", "127.0.0.1:4242", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("no token: status %d, want 401", rec.Code)
	}
	if rec := get(h, "/?t="+s.token, "evil.example:4242", nil); rec.Code != http.StatusForbidden {
		t.Errorf("foreign Host: status %d, want 403", rec.Code)
	}
	if rec := get(h, "/?t=wrong", "127.0.0.1:4242", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("wrong token: status %d, want 401", rec.Code)
	}

	rec := get(h, "/?t="+s.token, "127.0.0.1:4242", nil)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/" {
		t.Fatalf("token exchange: status %d location %q", rec.Code, rec.Header().Get("Location"))
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly {
		t.Fatalf("cookie = %+v", cookies)
	}
	for _, host := range []string{"localhost:4242", "quipu.localhost:4242"} {
		if rec := get(h, "/", host, cookies[0]); rec.Code != http.StatusOK {
			t.Errorf("with cookie via %s: status %d", host, rec.Code)
		}
	}
	if rec := get(h, "/", "quipu.localhost:9999", cookies[0]); rec.Code != http.StatusForbidden {
		t.Errorf("our name on another port: status %d, want 403", rec.Code)
	}
}

func TestTokenSurvivesRestart(t *testing.T) {
	s := testServer(t)
	again, err := NewServer(s.Service)
	if err != nil {
		t.Fatal(err)
	}
	if again.token != s.token || len(s.token) != 32 {
		t.Errorf("token changed across restarts: %q then %q", s.token, again.token)
	}
	if fi, err := os.Stat(filepath.Join(s.dataDir, "ui-token")); err != nil || fi.Mode().Perm() != 0o600 {
		t.Errorf("token file: %v %v", fi, err)
	}
}
