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

	s, err := New(home, filepath.Join(home, ".claude"), "default", st)
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
	if rec := get(h, "/", "localhost:4242", cookies[0]); rec.Code != http.StatusOK {
		t.Errorf("with cookie via localhost: status %d", rec.Code)
	}
}

func TestPages(t *testing.T) {
	s := testServer(t)
	h := s.Handler()
	cookie := &http.Cookie{Name: cookieName, Value: s.token}
	dir := claudecode.ProjectDirName(filepath.Join(s.home, "src", "demo"))

	for path, want := range map[string]string{
		"/":                   "~/src/demo",
		"/project?dir=" + dir: "long-claude-md",
		"/cache?days=7":       "model_changed",
		"/advice":             "CLAUDE.md has 251 lines",
		"/static/style.css":   "--accent",
	} {
		rec := get(h, path, "127.0.0.1:4242", cookie)
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status %d: %s", path, rec.Code, rec.Body.String())
			continue
		}
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("%s: body does not contain %q", path, want)
		}
	}
	if rec := get(h, "/project?dir=nope", "127.0.0.1:4242", cookie); rec.Code != http.StatusNotFound {
		t.Errorf("unknown project: status %d, want 404", rec.Code)
	}
}
