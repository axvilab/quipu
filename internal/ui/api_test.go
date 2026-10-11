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
