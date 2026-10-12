package ui

import (
	"encoding/json"
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

func TestBackupAPI(t *testing.T) {
	s := testServer(t)
	h := s.Handler()
	cookie := &http.Cookie{Name: cookieName, Value: s.token}
	os.MkdirAll(filepath.Join(s.configDir, "projects"), 0o755)
	claudeMD := filepath.Join(s.configDir, "CLAUDE.md")
	os.WriteFile(claudeMD, []byte("# Me\n"), 0o644)
	settings := filepath.Join(s.configDir, "settings.json")
	// Built at run time, so the source holds no string that looks like a real token.
	token := "ghp" + "_" + strings.Repeat("x1", 18)
	os.WriteFile(settings, []byte(`{"env": {"GITHUB_TOKEN": "`+token+`"}}`), 0o644)

	// A credential stops the backup and is reported.
	rec := postJSON(h, "/api/backup", `{}`, cookie)
	var res BackupResult
	json.Unmarshal(rec.Body.Bytes(), &res)
	if rec.Code != 200 || res.OK || len(res.Secrets) != 1 {
		t.Fatalf("backup with a token: %d %s", rec.Code, rec.Body.String())
	}
	os.WriteFile(settings, []byte(`{"theme": "dark"}`), 0o644)
	rec = postJSON(h, "/api/backup", `{}`, cookie)
	json.Unmarshal(rec.Body.Bytes(), &res)
	if !res.OK || res.Path == "" {
		t.Fatalf("backup: %s", rec.Body.String())
	}
	var info BackupInfo
	if code := getJSON(t, h, "/api/backup", cookie, &info); code != 200 || len(info.Archives) != 1 {
		t.Fatalf("info = %+v", info)
	}

	// Lose CLAUDE.md, preview the restore, apply it, undo it.
	os.Remove(claudeMD)
	body, _ := json.Marshal(map[string]string{"source": info.Archives[0].Path})
	rec = postJSON(h, "/api/restore/plan", string(body), cookie)
	var plan RestorePlan
	json.Unmarshal(rec.Body.Bytes(), &plan)
	if plan.New != 1 || len(plan.Files) != 1 || !strings.HasSuffix(plan.Files[0].Path, "CLAUDE.md") {
		t.Fatalf("plan: %s", rec.Body.String())
	}
	if rec := postJSON(h, "/api/restore", string(body), cookie); !strings.Contains(rec.Body.String(), `"restored"`) {
		t.Fatalf("restore: %s", rec.Body.String())
	}
	if b, _ := os.ReadFile(claudeMD); string(b) != "# Me\n" {
		t.Errorf("CLAUDE.md after restore = %q", b)
	}
	changes, _ := s.journal.List()
	if len(changes) != 1 || changes[0].Rule != "restore" {
		t.Fatalf("journal = %+v", changes)
	}
	if err := s.journal.Undo(changes[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(claudeMD); !os.IsNotExist(err) {
		t.Errorf("undo did not remove the restored file: %v", err)
	}
}

func TestDoctorAndShimAPI(t *testing.T) {
	s := testServer(t)
	s.SelfBinary = filepath.Join(s.home, "bin", "quipu")
	os.MkdirAll(filepath.Dir(s.SelfBinary), 0o755)
	os.WriteFile(s.SelfBinary, []byte("#!/bin/sh\n"), 0o755)
	t.Setenv("SHELL", "/bin/bash")
	h := s.Handler()
	cookie := &http.Cookie{Name: cookieName, Value: s.token}

	var d struct {
		Checks []Check   `json:"checks"`
		Shim   ShimState `json:"shim"`
	}
	if code := getJSON(t, h, "/api/doctor", cookie, &d); code != 200 || len(d.Checks) == 0 || d.Shim.Installed {
		t.Fatalf("doctor = %+v (%d)", d, code)
	}
	if rec := postJSON(h, "/api/shim", `{"on":true}`, cookie); !strings.Contains(rec.Body.String(), "shim-on") {
		t.Fatalf("shim on: %s", rec.Body.String())
	}
	getJSON(t, h, "/api/doctor", cookie, &d)
	var shim Check
	for _, c := range d.Checks {
		if c.ID == "shim" {
			shim = c
		}
	}
	if !d.Shim.Installed || !d.Shim.InRC || shim.State != "ok" {
		t.Errorf("after shim on: %+v, check %+v", d.Shim, shim)
	}
	rc, _ := os.ReadFile(filepath.Join(s.home, ".bashrc"))
	if !strings.Contains(string(rc), "/shim:$PATH") {
		t.Errorf(".bashrc = %q", rc)
	}
	// Another shell, as an IDE's terminal may run: zsh is listed and can be connected on its own.
	if len(d.Shim.Shells) != 3 || !d.Shim.Shells[0].Login || d.Shim.Shells[1].InRC {
		t.Errorf("shells = %+v", d.Shim.Shells)
	}
	if rec := postJSON(h, "/api/shim/rc", `{"shell":"zsh","on":true}`, cookie); !strings.Contains(rec.Body.String(), "shim-rc-on") {
		t.Fatalf("zsh on: %s", rec.Body.String())
	}
	if zshrc, _ := os.ReadFile(filepath.Join(s.home, ".zshrc")); !strings.Contains(string(zshrc), "/shim:$PATH") {
		t.Errorf(".zshrc = %q", zshrc)
	}
	if rec := postJSON(h, "/api/shim/rc", `{"shell":"tcsh","on":true}`, cookie); !strings.Contains(rec.Body.String(), "shell-unknown") {
		t.Errorf("unknown shell: %s", rec.Body.String())
	}
	if rec := postJSON(h, "/api/shim", `{"on":false}`, cookie); !strings.Contains(rec.Body.String(), "shim-off") {
		t.Fatalf("shim off: %s", rec.Body.String())
	}
	rc, _ = os.ReadFile(filepath.Join(s.home, ".bashrc"))
	if strings.Contains(string(rc), "quipu") {
		t.Errorf(".bashrc after off = %q", rc)
	}
	if zshrc, _ := os.ReadFile(filepath.Join(s.home, ".zshrc")); strings.Contains(string(zshrc), "quipu") {
		t.Errorf(".zshrc after off = %q", zshrc)
	}
}

func TestNewProjectAPI(t *testing.T) {
	s := testServer(t)
	h := s.Handler()
	cookie := &http.Cookie{Name: cookieName, Value: s.token}
	// A second project, so the same skill can be in two projects.
	other := filepath.Join(s.home, "src", "api")
	os.MkdirAll(other, 0o755)
	r := claudecode.Response{MessageID: "m2", SessionID: "s2", Cwd: other, Timestamp: time.Now().Add(-time.Hour)}
	src := store.Source{Path: "/t/s2.jsonl", Profile: "default", ProjectDir: claudecode.ProjectDirName(other), SessionID: "s2", Kind: claudecode.KindSession}
	if err := s.st.WriteResponses(t.Context(), src, []claudecode.Response{r}); err != nil {
		t.Fatal(err)
	}
	skill := "---\nname: release\ndescription: Cuts a release.\n---\nTag and push.\n"
	for _, p := range []string{filepath.Join(s.home, "src", "demo"), other} {
		os.MkdirAll(filepath.Join(p, ".claude", "skills", "release"), 0o755)
		os.WriteFile(filepath.Join(p, ".claude", "skills", "release", "SKILL.md"), []byte(skill), 0o644)
	}
	os.MkdirAll(filepath.Join(other, ".claude", "agents"), 0o755)
	os.WriteFile(filepath.Join(other, ".claude", "agents", "reviewer.md"), []byte("---\nname: reviewer\ndescription: Reviews.\n---\nReview.\n"), 0o644)

	var info NewInfo
	if code := getJSON(t, h, "/api/new", cookie, &info); code != 200 || info.Parent != "~/src" || info.Average == 0 {
		t.Fatalf("info %d = %+v", code, info)
	}
	ext := map[string]NewExtension{}
	for _, e := range info.Extensions {
		ext[e.Name] = e
	}
	if e := ext["release"]; !e.Global || e.Variants != 1 || len(e.Projects) != 2 || e.Tokens == 0 {
		t.Fatalf("release = %+v", e)
	}
	if e := ext["reviewer"]; e.Global || len(e.Projects) != 1 {
		t.Fatalf("reviewer = %+v", e)
	}

	body := `{"folder":"~/src/shop","summary":"A shop.","extensions":["` + ext["reviewer"].Path + `"],"global":["` + ext["release"].Path + `"]}`
	rec := postJSON(h, "/api/new/plan", body, cookie)
	var files []NewFile
	json.Unmarshal(rec.Body.Bytes(), &files)
	byRel := map[string]NewFile{}
	for _, f := range files {
		byRel[f.Rel] = f
	}
	if f := byRel["~/.claude/skills/release/SKILL.md"]; !f.Global || f.Always == 0 {
		t.Errorf("global release = %+v (files %+v)", f, files)
	}
	if f := byRel[".claude/agents/reviewer.md"]; f.From == "" || f.Always == 0 {
		t.Errorf("reviewer = %+v", f)
	}
	if f := byRel["CLAUDE.md"]; f.Always == 0 || f.Existing {
		t.Errorf("CLAUDE.md = %+v", f)
	}
	if _, err := os.Stat(filepath.Join(s.home, "src", "shop")); !os.IsNotExist(err) {
		t.Fatal("plan wrote files")
	}

	rec = postJSON(h, "/api/new", body, cookie)
	var res Result
	json.Unmarshal(rec.Body.Bytes(), &res)
	if rec.Code != 200 || res.Code != "project-created" || res.Params["change"] == "" {
		t.Fatalf("create %d: %s", rec.Code, rec.Body.String())
	}
	user := filepath.Join(s.home, ".claude", "skills", "release", "SKILL.md")
	if _, err := os.Stat(user); err != nil {
		t.Errorf("release not moved to the user's skills: %v", err)
	}
	if _, err := os.Stat(filepath.Join(other, ".claude", "skills", "release")); !os.IsNotExist(err) {
		t.Errorf("project copy left after moving: %v", err)
	}
	if _, err := os.Stat(filepath.Join(s.home, "src", "shop", ".claude", "skills", "release")); !os.IsNotExist(err) {
		t.Errorf("a global skill was also copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(s.home, "src", "shop", ".claude", "agents", "reviewer.md")); err != nil {
		t.Error(err)
	}

	// The folder now exists: its CLAUDE.md is kept and shown as existing.
	rec = postJSON(h, "/api/new/plan", `{"folder":"~/src/shop"}`, cookie)
	json.Unmarshal(rec.Body.Bytes(), &files)
	for _, f := range files {
		if f.Rel == "CLAUDE.md" && !f.Existing {
			t.Errorf("existing CLAUDE.md not marked: %+v", f)
		}
	}
	if rec := postJSON(h, "/api/new", `{"folder":"~/src/shop"}`, cookie); !strings.Contains(rec.Body.String(), "nothing-new") {
		t.Errorf("nothing to add: %s", rec.Body.String())
	}

	// One undo reverts the project and the move.
	if rec := postJSON(h, "/api/advice/undo", `{"id":"`+res.Params["change"]+`"}`, cookie); rec.Code != 200 {
		t.Fatalf("undo %d: %s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(filepath.Join(s.home, "src", "shop")); !os.IsNotExist(err) {
		t.Errorf("project left after undo: %v", err)
	}
	if _, err := os.Stat(filepath.Join(other, ".claude", "skills", "release", "SKILL.md")); err != nil {
		t.Errorf("project copy not restored: %v", err)
	}
	if _, err := os.Stat(user); !os.IsNotExist(err) {
		t.Errorf("user skill left after undo: %v", err)
	}
	if rec := postJSON(h, "/api/new/plan", `{"folder":"shop"}`, cookie); !strings.Contains(rec.Body.String(), "folder-not-absolute") {
		t.Errorf("relative folder: %s", rec.Body.String())
	}
}

func TestStatusLineAPI(t *testing.T) {
	s := testServer(t)
	s.ProfilesFile = filepath.Join(s.home, ".config", "quipu", "profiles.json")
	s.SelfBinary = filepath.Join(s.home, "bin", "quipu")
	h := s.Handler()
	cookie := &http.Cookie{Name: cookieName, Value: s.token}
	if rec := postJSON(h, "/api/profiles", `{"name":"work","share":[]}`, cookie); !strings.Contains(rec.Body.String(), "profile-added") {
		t.Fatalf("add: %s", rec.Body.String())
	}
	work := filepath.Join(s.home, ".claude-work", "settings.json")
	// Another status line was there before; quipu's replaces it and undo brings it back.
	os.WriteFile(work, []byte(`{"theme":"dark","statusLine":{"type":"command","command":"node dash.js"}}`), 0o644)

	var p Profiles
	getJSON(t, h, "/api/profiles", cookie, &p)
	if st := statusOf(p, "work"); st != "other" {
		t.Fatalf("before: %q", st)
	}
	rec := postJSON(h, "/api/profiles/work/statusline", `{"on":true}`, cookie)
	var r Result
	json.Unmarshal(rec.Body.Bytes(), &r)
	if r.Code != "statusline-on" || r.Params["change"] == "" {
		t.Fatalf("on: %s", rec.Body.String())
	}
	b, _ := os.ReadFile(work)
	if !strings.Contains(string(b), "quipu statusline") || !strings.Contains(string(b), `"theme": "dark"`) {
		t.Errorf("settings = %s", b)
	}
	getJSON(t, h, "/api/profiles", cookie, &p)
	if st := statusOf(p, "work"); st != "quipu" {
		t.Errorf("after on: %q", st)
	}
	if st := statusOf(p, "default"); st != "" {
		t.Errorf("default changed: %q", st)
	}
	postJSON(h, "/api/advice/undo", `{"id":"`+r.Params["change"]+`"}`, cookie)
	if b, _ := os.ReadFile(work); !strings.Contains(string(b), "dash.js") {
		t.Errorf("undo did not bring back the old status line: %s", b)
	}
	// Off removes only quipu's own status line.
	if rec := postJSON(h, "/api/profiles/work/statusline", `{"on":false}`, cookie); !strings.Contains(rec.Body.String(), "unchanged") {
		t.Errorf("off over another status line: %s", rec.Body.String())
	}
	// On, then off: the status line that was there before comes back.
	postJSON(h, "/api/profiles/work/statusline", `{"on":true}`, cookie)
	postJSON(h, "/api/profiles/work/statusline", `{"on":false}`, cookie)
	if b, _ := os.ReadFile(work); !strings.Contains(string(b), "dash.js") || strings.Contains(string(b), "quipu statusline") {
		t.Errorf("off did not bring back the old status line: %s", b)
	}
}

func statusOf(p Profiles, name string) string {
	for _, pr := range p.Profiles {
		if pr.Name == name {
			return pr.StatusLine
		}
	}
	return "missing"
}

func TestShareMemoryAPI(t *testing.T) {
	s := testServer(t)
	s.ProfilesFile = filepath.Join(s.home, ".config", "quipu", "profiles.json")
	h := s.Handler()
	cookie := &http.Cookie{Name: cookieName, Value: s.token}
	postJSON(h, "/api/profiles", `{"name":"work","share":[]}`, cookie)
	project := filepath.Join(s.home, "src", "demo")
	if rec := postJSON(h, "/api/rules", `{"folder":"~/src/demo","profile":"work"}`, cookie); rec.Code != 200 {
		t.Fatalf("rule: %s", rec.Body.String())
	}
	mem := claudecode.OwnMemoryDir(filepath.Join(s.home, ".claude"), project)
	os.MkdirAll(mem, 0o755)
	os.WriteFile(filepath.Join(mem, "MEMORY.md"), []byte("- stage copied from dev\n"), 0o644)

	var p Profiles
	getJSON(t, h, "/api/profiles", cookie, &p)
	if len(p.Rules) != 1 || p.Rules[0].Memory.Source != "default" || p.Rules[0].Memory.SourceFiles != 1 || p.Rules[0].Memory.Shared {
		t.Fatalf("rules = %+v", p.Rules)
	}
	rec := postJSON(h, "/api/rules/memory", `{"folder":"~/src/demo","on":true}`, cookie)
	if !strings.Contains(rec.Body.String(), "memory-shared") {
		t.Fatalf("share: %s", rec.Body.String())
	}
	getJSON(t, h, "/api/profiles", cookie, &p)
	if !p.Rules[0].Memory.Shared || p.Rules[0].Memory.From != "default" {
		t.Errorf("after share: %+v", p.Rules[0].Memory)
	}
	if rec := postJSON(h, "/api/rules/memory", `{"folder":"~/src/demo","on":false}`, cookie); !strings.Contains(rec.Body.String(), "memory-unshared") {
		t.Errorf("unshare: %s", rec.Body.String())
	}
	if rec := postJSON(h, "/api/rules/memory", `{"folder":"~/elsewhere","on":true}`, cookie); !strings.Contains(rec.Body.String(), "rule-missing") {
		t.Errorf("no rule: %s", rec.Body.String())
	}
}

func TestPluginsAPI(t *testing.T) {
	s := testServer(t)
	h := s.Handler()
	cookie := &http.Cookie{Name: cookieName, Value: s.token}
	cfg := filepath.Join(s.home, ".claude")
	work := filepath.Join(s.home, ".claude-work")
	os.MkdirAll(filepath.Join(work, "projects"), 0o755)
	plugDir := filepath.Join(cfg, "plugins", "cache", "market", "dash", "1.0.0")
	os.MkdirAll(filepath.Join(plugDir, "skills", "check-usage"), 0o755)
	os.WriteFile(filepath.Join(plugDir, "skills", "check-usage", "SKILL.md"),
		[]byte("---\nname: check-usage\ndescription: Check usage limits of every AI CLI and recommend one.\n---\nRun it.\n"), 0o644)
	os.WriteFile(filepath.Join(cfg, "plugins", "installed_plugins.json"),
		[]byte(`{"version":2,"plugins":{"dash@market":[{"scope":"user","version":"1.0.0","installPath":"`+plugDir+`"}]}}`), 0o644)
	os.WriteFile(filepath.Join(cfg, "plugins", "known_marketplaces.json"),
		[]byte(`{"market":{"source":{"source":"github","repo":"someone/market"}}}`), 0o644)
	os.WriteFile(filepath.Join(cfg, "settings.json"), []byte(`{"enabledPlugins":{"dash@market":true}}`), 0o644)

	var m PluginsMatrix
	if code := getJSON(t, h, "/api/plugins", cookie, &m); code != 200 || len(m.Plugins) != 1 || len(m.Profiles) != 2 {
		t.Fatalf("plugins %d = %+v", code, m)
	}
	row := m.Plugins[0]
	if row.Name != "dash" || row.Tokens == 0 {
		t.Errorf("row = %+v", row)
	}
	if c := row.Cells["default"]; !c.Installed || c.State != "everywhere" || c.Version != "1.0.0" {
		t.Errorf("default cell = %+v", c)
	}
	if c := row.Cells["work"]; c.Installed || !c.Installable {
		t.Errorf("work cell = %+v", c)
	}

	rec := postJSON(h, "/api/plugins/enable", `{"profile":"default","id":"dash@market","on":false}`, cookie)
	var res Result
	json.Unmarshal(rec.Body.Bytes(), &res)
	if res.Code != "plugin-off" {
		t.Fatalf("off: %s", rec.Body.String())
	}
	getJSON(t, h, "/api/plugins", cookie, &m)
	if m.Plugins[0].Cells["default"].State != "off" {
		t.Errorf("after off: %+v", m.Plugins[0].Cells["default"])
	}
	postJSON(h, "/api/advice/undo", `{"id":"`+res.Params["change"]+`"}`, cookie)
	getJSON(t, h, "/api/plugins", cookie, &m)
	if m.Plugins[0].Cells["default"].State != "everywhere" {
		t.Errorf("after undo: %+v", m.Plugins[0].Cells["default"])
	}

	// Install into work and uninstall run Claude Code's own commands under that profile.
	log := filepath.Join(s.home, "claude.log")
	s.ClaudeBinary = filepath.Join(s.home, "bin", "claude")
	os.MkdirAll(filepath.Dir(s.ClaudeBinary), 0o755)
	os.WriteFile(s.ClaudeBinary, []byte("#!/bin/sh\necho \"$CLAUDE_CONFIG_DIR $*\" >> "+log+"\n"), 0o755)
	if rec := postJSON(h, "/api/plugins/install", `{"profile":"work","id":"dash@market"}`, cookie); !strings.Contains(rec.Body.String(), "plugin-installed") {
		t.Fatalf("install: %s", rec.Body.String())
	}
	b, _ := os.ReadFile(log)
	if want := work + " plugin marketplace add someone/market\n" + work + " plugin install dash@market\n"; string(b) != want {
		t.Errorf("claude ran:\n%s\nwant:\n%s", b, want)
	}
	if rec := postJSON(h, "/api/plugins/uninstall", `{"profile":"default","id":"dash@market"}`, cookie); !strings.Contains(rec.Body.String(), "plugin-uninstalled") {
		t.Fatalf("uninstall: %s", rec.Body.String())
	}
	if rec := postJSON(h, "/api/plugins/uninstall", `{"profile":"default","id":"nope@x"}`, cookie); !strings.Contains(rec.Body.String(), "plugin-unknown") {
		t.Errorf("unknown plugin: %s", rec.Body.String())
	}
}

func TestProfileEditAPI(t *testing.T) {
	s := testServer(t)
	s.ProfilesFile = filepath.Join(s.home, ".config", "quipu", "profiles.json")
	h := s.Handler()
	cookie := &http.Cookie{Name: cookieName, Value: s.token}
	os.MkdirAll(filepath.Join(s.home, ".claude", "skills"), 0o755)
	os.WriteFile(filepath.Join(s.home, ".claude", "CLAUDE.md"), []byte("me\n"), 0o644)
	postJSON(h, "/api/profiles", `{"name":"work","share":["CLAUDE.md"]}`, cookie)

	if rec := postJSON(h, "/api/profiles/work/label", `{"label":"Office"}`, cookie); !strings.Contains(rec.Body.String(), "label-saved") {
		t.Fatalf("label: %s", rec.Body.String())
	}
	if rec := postJSON(h, "/api/profiles/work/share", `{"item":"skills","on":true}`, cookie); !strings.Contains(rec.Body.String(), "share-on") {
		t.Fatalf("share: %s", rec.Body.String())
	}
	var p Profiles
	getJSON(t, h, "/api/profiles", cookie, &p)
	var work ProfileInfo
	for _, pr := range p.Profiles {
		if pr.Name == "work" {
			work = pr
		}
	}
	if work.Title != "Office" || strings.Join(work.Shares, ",") != "CLAUDE.md,skills" {
		t.Errorf("work = %+v", work)
	}
	if rec := postJSON(h, "/api/profiles/work/share", `{"item":"skills","on":false}`, cookie); !strings.Contains(rec.Body.String(), "share-off") {
		t.Errorf("unshare: %s", rec.Body.String())
	}
	if rec := postJSON(h, "/api/profiles/default/share", `{"item":"skills","on":true}`, cookie); !strings.Contains(rec.Body.String(), "share-failed") {
		t.Errorf("share into default: %s", rec.Body.String())
	}
}

func TestUpgradeStatusLines(t *testing.T) {
	s := testServer(t)
	s.SelfBinary = filepath.Join(s.home, "bin", "quipu")
	cfg := filepath.Join(s.home, ".claude")
	os.MkdirAll(cfg, 0o755)
	// quipu's status line from an older version: no refresh interval, another binary path.
	os.WriteFile(filepath.Join(cfg, "settings.json"), []byte(`{"statusLine":{"type":"command","command":"/old/quipu statusline"},"theme":"dark"}`), 0o644)
	s.upgradeStatusLines()
	b, _ := os.ReadFile(filepath.Join(cfg, "settings.json"))
	if !strings.Contains(string(b), `"refreshInterval": 60`) || !strings.Contains(string(b), s.SelfBinary+" statusline") || !strings.Contains(string(b), `"theme": "dark"`) {
		t.Errorf("not upgraded: %s", b)
	}
	// Someone else's status line is left alone.
	os.WriteFile(filepath.Join(cfg, "settings.json"), []byte(`{"statusLine":{"type":"command","command":"node dash.js"}}`), 0o644)
	s.upgradeStatusLines()
	if b, _ := os.ReadFile(filepath.Join(cfg, "settings.json")); strings.Contains(string(b), "refreshInterval") {
		t.Errorf("another status line changed: %s", b)
	}
}

func TestAddProfileWithStatusLine(t *testing.T) {
	s := testServer(t)
	s.ProfilesFile = filepath.Join(s.home, ".config", "quipu", "profiles.json")
	s.SelfBinary = filepath.Join(s.home, "bin", "quipu")
	h := s.Handler()
	cookie := &http.Cookie{Name: cookieName, Value: s.token}
	if rec := postJSON(h, "/api/profiles", `{"name":"work","share":[],"statusLine":true}`, cookie); !strings.Contains(rec.Body.String(), "profile-added") {
		t.Fatalf("add: %s", rec.Body.String())
	}
	b, _ := os.ReadFile(filepath.Join(s.home, ".claude-work", "settings.json"))
	if !strings.Contains(string(b), "quipu statusline") {
		t.Errorf("new profile has no status line: %s", b)
	}
}

func TestProjectListing(t *testing.T) {
	s := testServer(t)
	proj := filepath.Join(s.home, "src", "demo")
	cfg := filepath.Join(s.home, ".claude")
	for _, name := range []string{"ship", "stale"} {
		os.MkdirAll(filepath.Join(cfg, "skills", name), 0o755)
		os.WriteFile(filepath.Join(cfg, "skills", name, "SKILL.md"), []byte("---\nname: "+name+"\ndescription: d\n---\n"), 0o644)
	}
	dir := filepath.Join(cfg, "projects", claudecode.ProjectDirName(proj))
	os.MkdirAll(dir, 0o755)
	b, _ := json.Marshal(map[string]any{"type": "attachment", "timestamp": "2026-10-12T10:00:00Z",
		"attachment": map[string]any{"type": "skill_listing", "isInitial": true, "names": []string{"ship", "loop"},
			"content": "- ship: d\n- loop: Run a prompt on a recurring interval"}})
	os.WriteFile(filepath.Join(dir, "s-2.jsonl"), append(b, '\n'), 0o600)

	var p Project
	getJSON(t, s.Handler(), "/api/projects/"+claudecode.ProjectDirName(proj), &http.Cookie{Name: cookieName, Value: s.token}, &p)
	l := p.Listing
	if l == nil || strings.Join(l.Missing, ",") != "stale" || len(l.Own) != 1 || l.Own[0].Label != "loop" || l.Own[0].Tokens == 0 {
		t.Fatalf("listing = %+v", l)
	}
}
