package ui

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/axvilab/quipu/internal/advise"
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

func post(h http.Handler, path, origin string, form url.Values, cookie *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", path, strings.NewReader(form.Encode()))
	req.Host = "127.0.0.1:4242"
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", "http://127.0.0.1:4242/advice")
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestApplyAndUndo(t *testing.T) {
	s := testServer(t)
	h := s.Handler()
	cookie := &http.Cookie{Name: cookieName, Value: s.token}
	proj := filepath.Join(s.home, "src", "demo")
	claudeMD := filepath.Join(proj, "CLAUDE.md")
	original := "Project rules.\nDesign: @docs/DESIGN.md\n"
	os.WriteFile(claudeMD, []byte(original), 0o644)
	os.MkdirAll(filepath.Join(proj, "docs"), 0o755)
	os.WriteFile(filepath.Join(proj, "docs", "DESIGN.md"), []byte(strings.Repeat("design text ", 400)), 0o644)

	in := advise.Input{Home: s.home, ConfigDir: s.configDir, Project: proj, Items: claudecode.Instructions(s.home, s.configDir, proj)}
	var id string
	for _, f := range advise.Run(in, nil) {
		if f.Rule == "large-import" {
			id = f.ID()
		}
	}
	if id == "" {
		t.Fatal("large-import finding not produced")
	}
	form := url.Values{"dir": {claudecode.ProjectDirName(proj)}, "id": {id}}

	if rec := post(h, "/apply", "http://evil.example", form, cookie); rec.Code != http.StatusForbidden {
		t.Errorf("cross-origin apply: status %d, want 403", rec.Code)
	}
	if rec := post(h, "/apply", "", form, cookie); rec.Code != http.StatusForbidden {
		t.Errorf("apply without Origin: status %d, want 403", rec.Code)
	}
	if b, _ := os.ReadFile(claudeMD); string(b) != original {
		t.Fatalf("refused requests changed the file: %q", b)
	}

	rec := post(h, "/apply", "http://127.0.0.1:4242", form, cookie)
	if rec.Code != http.StatusSeeOther || !strings.Contains(rec.Header().Get("Location"), "Applied") {
		t.Fatalf("apply: status %d location %q", rec.Code, rec.Header().Get("Location"))
	}
	if b, _ := os.ReadFile(claudeMD); !strings.Contains(string(b), "`docs/DESIGN.md` (read it when relevant)") {
		t.Fatalf("file after apply = %q", b)
	}

	changes, _ := s.journal.List()
	if len(changes) != 1 {
		t.Fatalf("changes = %+v", changes)
	}
	page := get(h, "/advice", "127.0.0.1:4242", cookie).Body.String()
	if !strings.Contains(page, "Recent changes") || !strings.Contains(page, `action="/undo"`) {
		t.Error("advice page does not list the change with an undo button")
	}

	rec = post(h, "/undo", "http://127.0.0.1:4242", url.Values{"id": {changes[0].ID}}, cookie)
	if !strings.Contains(rec.Header().Get("Location"), "Undone") {
		t.Errorf("undo: location %q", rec.Header().Get("Location"))
	}
	if b, _ := os.ReadFile(claudeMD); string(b) != original {
		t.Errorf("file after undo = %q", b)
	}

	// Applying a finding that no longer exists is reported, not attempted.
	rec = post(h, "/apply", "http://127.0.0.1:4242", url.Values{"dir": form["dir"], "id": {"0000"}}, cookie)
	if !strings.Contains(rec.Header().Get("Location"), "gone") {
		t.Errorf("stale apply: location %q", rec.Header().Get("Location"))
	}
}
