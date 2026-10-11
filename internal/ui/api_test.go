package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/axvilab/quipu/internal/claudecode"
)

func getJSON(t *testing.T, h http.Handler, path string, cookie *http.Cookie, v any) int {
	t.Helper()
	rec := get(h, path, "127.0.0.1:4242", cookie)
	if v != nil && rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
			t.Fatalf("%s: %v: %s", path, err, rec.Body.String())
		}
	}
	return rec.Code
}

func postJSON(h http.Handler, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", path, strings.NewReader(body))
	req.Host = "127.0.0.1:4242"
	req.Header.Set("Origin", "http://127.0.0.1:4242")
	req.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestAPI(t *testing.T) {
	s := testServer(t)
	h := s.Handler()
	cookie := &http.Cookie{Name: cookieName, Value: s.token}
	proj := filepath.Join(s.home, "src", "demo")
	dir := claudecode.ProjectDirName(proj)
	os.WriteFile(filepath.Join(proj, "CLAUDE.md"), []byte("Rules.\nDesign: @docs/DESIGN.md\n"), 0o644)
	os.MkdirAll(filepath.Join(proj, "docs"), 0o755)
	os.WriteFile(filepath.Join(proj, "docs", "DESIGN.md"), []byte(strings.Repeat("design ", 600)), 0o644)

	if code := getJSON(t, h, "/api/overview", nil, nil); code != http.StatusUnauthorized {
		t.Errorf("no cookie: %d, want 401", code)
	}

	var ov Overview
	if code := getJSON(t, h, "/api/overview?days=30", cookie, &ov); code != 200 {
		t.Fatalf("overview: %d", code)
	}
	if len(ov.Projects) != 1 || ov.Projects[0].Name != "demo" || ov.Projects[0].Findings == 0 || len(ov.Attention) == 0 {
		t.Fatalf("overview = %+v", ov)
	}
	f := ov.Attention[0]
	if f.Rule != "large-import" || !f.CanApply || f.Params["ref"] != "docs/DESIGN.md" || len(f.Diff) != 3 || f.Diff[0].Kind != "file" || f.Diff[0].Line != 2 || f.Diff[1].Kind != "del" {
		t.Errorf("finding = %+v", f)
	}

	var p Project
	if code := getJSON(t, h, "/api/projects/"+dir, cookie, &p); code != 200 || p.Name != "demo" || len(p.Items) == 0 || p.AlwaysTotal == 0 {
		t.Errorf("project %d = %+v", code, p)
	}
	if code := getJSON(t, h, "/api/projects/nope", cookie, nil); code != http.StatusNotFound {
		t.Errorf("unknown project: %d", code)
	}

	var c CacheReport
	if code := getJSON(t, h, "/api/cache?days=7", cookie, &c); code != 200 || len(c.Misses) != 1 || c.Misses[0].Reason != "model_changed" {
		t.Errorf("cache %d = %+v", code, c)
	}

	// Apply needs JSON: a form post is refused before anything runs.
	form := httptest.NewRequest("POST", "/api/advice/apply", strings.NewReader("dir=x&id=y"))
	form.Host = "127.0.0.1:4242"
	form.Header.Set("Origin", "http://127.0.0.1:4242")
	form.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	form.AddCookie(cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, form)
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Errorf("form post: %d, want 415", rec.Code)
	}

	rec = postJSON(h, "/api/advice/apply", `{"dir":"`+dir+`","id":"`+f.ID+`"}`, cookie)
	var res Result
	json.Unmarshal(rec.Body.Bytes(), &res)
	if rec.Code != 200 || !res.OK || res.Code != "applied" {
		t.Fatalf("apply %d = %+v", rec.Code, res)
	}
	if b, _ := os.ReadFile(filepath.Join(proj, "CLAUDE.md")); !strings.Contains(string(b), "`docs/DESIGN.md`") {
		t.Errorf("CLAUDE.md after apply = %q", b)
	}

	var adv Advice
	getJSON(t, h, "/api/advice", cookie, &adv)
	if len(adv.Changes) != 1 {
		t.Fatalf("changes = %+v", adv.Changes)
	}
	rec = postJSON(h, "/api/advice/undo", `{"id":"`+adv.Changes[0].ID+`"}`, cookie)
	json.Unmarshal(rec.Body.Bytes(), &res)
	if !res.OK || res.Code != "undone" {
		t.Errorf("undo = %+v", res)
	}

	rec = postJSON(h, "/api/advice/apply", `{"dir":"`+dir+`","id":"nope"}`, cookie)
	json.Unmarshal(rec.Body.Bytes(), &res)
	if rec.Code != http.StatusBadRequest || res.Code != "finding-gone" {
		t.Errorf("stale apply %d = %+v", rec.Code, res)
	}
}

func TestEditorAPI(t *testing.T) {
	s := testServer(t)
	h := s.Handler()
	cookie := &http.Cookie{Name: cookieName, Value: s.token}
	proj := filepath.Join(s.home, "src", "demo")
	file := filepath.Join(proj, "CLAUDE.md")
	os.WriteFile(file, []byte("# demo\n\nRules.\n"), 0o644)

	var f File
	if code := getJSON(t, h, "/api/file?path=~/src/demo/CLAUDE.md", cookie, &f); code != 200 {
		t.Fatalf("read: %d", code)
	}
	if f.Content != "# demo\n\nRules.\n" || !f.Always || f.Tokens == 0 || f.Version == "" {
		t.Fatalf("file = %+v", f)
	}

	// Files Claude Code does not load for a known project are off limits.
	secret := filepath.Join(s.home, ".ssh", "config")
	os.MkdirAll(filepath.Dir(secret), 0o700)
	os.WriteFile(secret, []byte("Host x\n"), 0o600)
	if code := getJSON(t, h, "/api/file?path=~/.ssh/config", cookie, nil); code != http.StatusForbidden {
		t.Errorf("read outside instructions: %d, want 403", code)
	}
	if rec := postJSON(h, "/api/file", `{"path":"~/.ssh/config","content":"x","version":""}`, cookie); rec.Code != http.StatusForbidden {
		t.Errorf("write outside instructions: %d, want 403", rec.Code)
	}

	save := func(content, version string) *httptest.ResponseRecorder {
		b, _ := json.Marshal(map[string]string{"path": f.Path, "content": content, "version": version})
		return postJSON(h, "/api/file", string(b), cookie)
	}
	if rec := save("# demo\n\nShorter.\n", f.Version); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"saved"`) {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}
	if got, _ := os.ReadFile(file); string(got) != "# demo\n\nShorter.\n" {
		t.Errorf("file = %q", got)
	}
	// Saving again from the old version would overwrite the change just made.
	if rec := save("# demo\n\nStale.\n", f.Version); rec.Code == 200 || !strings.Contains(rec.Body.String(), "file-changed") {
		t.Errorf("stale save: %d %s", rec.Code, rec.Body.String())
	}

	// The edit is in the journal and can be undone.
	changes, _ := s.journal.List()
	if len(changes) != 1 || changes[0].Rule != "edit" {
		t.Fatalf("journal = %+v", changes)
	}
	if err := s.journal.Undo(changes[0].ID); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(file); string(got) != "# demo\n\nRules.\n" {
		t.Errorf("after undo = %q", got)
	}

	if rec := postJSON(h, "/api/tokens", `{"text":"four words in here"}`, cookie); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"tokens":5`) {
		t.Errorf("tokens: %d %s", rec.Code, rec.Body.String())
	}
}

func TestLiveAPI(t *testing.T) {
	s := testServer(t)
	s.SelfBinary = "/opt/my tools/quipu"
	h := s.Handler()
	cookie := &http.Cookie{Name: cookieName, Value: s.token}
	settings := filepath.Join(s.configDir, "settings.json")
	os.MkdirAll(s.configDir, 0o755)
	os.WriteFile(settings, []byte(`{"theme": "dark", "remoteControlAtStartup": true}`), 0o644)

	var lv Live
	if code := getJSON(t, h, "/api/live", cookie, &lv); code != 200 || lv.Watching || !lv.AutoStart || lv.Sessions == nil {
		t.Fatalf("live = %+v (%d)", lv, code)
	}

	if rec := postJSON(h, "/api/live/watch", `{"on":true}`, cookie); rec.Code != 200 || !strings.Contains(rec.Body.String(), "watching-on") {
		t.Fatalf("watch on: %d %s", rec.Code, rec.Body.String())
	}
	b, _ := os.ReadFile(settings)
	if !strings.Contains(string(b), `"'/opt/my tools/quipu' hook claude"`) || !strings.Contains(string(b), `"theme": "dark"`) {
		t.Errorf("settings after watch on:\n%s", b)
	}
	if code := getJSON(t, h, "/api/live", cookie, &lv); code != 200 || !lv.Watching {
		t.Errorf("not watching after watch on: %+v", lv)
	}

	if rec := postJSON(h, "/api/live/watch", `{"on":false}`, cookie); rec.Code != 200 || !strings.Contains(rec.Body.String(), "watching-off") {
		t.Fatalf("watch off: %d %s", rec.Code, rec.Body.String())
	}
	b, _ = os.ReadFile(settings)
	if strings.Contains(string(b), "hook claude") || strings.Contains(string(b), `"hooks"`) {
		t.Errorf("hook left after watch off:\n%s", b)
	}
	if changes, _ := s.journal.List(); len(changes) != 2 {
		t.Errorf("journal has %d changes, want 2", len(changes))
	}
}
