package ui

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestProfilesPage(t *testing.T) {
	s := testServer(t)
	h := s.Handler()
	cookie := &http.Cookie{Name: cookieName, Value: s.token}
	origin := "http://127.0.0.1:4242"
	bin := t.TempDir()

	// Fake claude: "auth login" opens $BROWSER with a URL, waits a moment and marks the profile
	// as signed in; "auth status" reports that marker.
	claude := filepath.Join(bin, "claude")
	os.WriteFile(claude, []byte(`#!/bin/sh
dir="${CLAUDE_CONFIG_DIR:-$HOME/.claude}"
if [ "$2" = login ]; then [ "$3" = --email ] && echo "$4" > "$HOME/email-arg"; "$BROWSER" "https://claude.com/oauth/authorize?x=1"; sleep 1; mkdir -p "$dir"; touch "$dir/.signed"; exit 0; fi
if [ -f "$dir/.signed" ]; then echo '{"loggedIn":true,"email":"me@work.example"}'; else echo '{"loggedIn":false}'; fi
`), 0o755)
	browser := filepath.Join(bin, "browser")
	os.WriteFile(browser, []byte("#!/bin/sh\nprintf %s \"$1\" > \"$"+LoginURLEnv+"\"\n"), 0o755)
	s.ClaudeBinary, s.SelfBinary = claude, browser
	s.ProfilesFile = filepath.Join(s.home, ".config", "quipu", "profiles.json")
	t.Setenv("HOME", s.home)

	rec := post(h, "/profiles/add", origin, url.Values{"name": {"work"}, "share": {"skills"}}, cookie)
	if !strings.Contains(rec.Header().Get("Location"), "created") {
		t.Fatalf("add: %q", rec.Header().Get("Location"))
	}
	work := filepath.Join(s.home, "src", "work")
	rec = post(h, "/profiles/rule", origin, url.Values{"folder": {"~/src/work"}, "profile": {"work"}}, cookie)
	if !strings.Contains(rec.Header().Get("Location"), "now+runs+as+work") {
		t.Fatalf("rule: %q", rec.Header().Get("Location"))
	}

	page := get(h, "/profiles", "127.0.0.1:4242", cookie).Body.String()
	for _, want := range []string{"~/.claude-work", "~/src/work", "not signed in"} {
		if !strings.Contains(page, want) {
			t.Errorf("profiles page lacks %q", want)
		}
	}
	_ = work

	post(h, "/profiles/login", origin, url.Values{"name": {"work"}, "email": {"me@work.example"}}, cookie)
	deadline := time.Now().Add(5 * time.Second)
	sawURL := false
	for time.Now().Before(deadline) {
		page = get(h, "/profiles", "127.0.0.1:4242", cookie).Body.String()
		if strings.Contains(page, "https://claude.com/oauth/authorize?x=1") {
			sawURL = true
		}
		if strings.Contains(page, "me@work.example") {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !sawURL {
		t.Error("the sign-in URL captured through $BROWSER was never offered")
	}
	if !strings.Contains(page, "me@work.example") {
		t.Error("profile not shown as signed in after login finished")
	}
	if b, _ := os.ReadFile(filepath.Join(s.home, "email-arg")); strings.TrimSpace(string(b)) != "me@work.example" {
		t.Errorf("claude auth login got --email %q", b)
	}
	rec = post(h, "/profiles/login", origin, url.Values{"name": {"work"}, "email": {"not an email"}}, cookie)
	if !strings.Contains(rec.Header().Get("Location"), "email") {
		t.Errorf("bad email accepted: %q", rec.Header().Get("Location"))
	}

	rec = post(h, "/profiles/remove", origin, url.Values{"name": {"work"}}, cookie)
	if !strings.Contains(rec.Header().Get("Location"), "kept") {
		t.Errorf("remove: %q", rec.Header().Get("Location"))
	}
	if _, err := os.Stat(filepath.Join(s.home, ".claude-work")); err != nil {
		t.Error("profile dir must be kept after remove")
	}
}

func TestCaptureLoginURL(t *testing.T) {
	file := filepath.Join(t.TempDir(), "url")
	t.Setenv(LoginURLEnv, file)
	if CaptureLoginURL([]string{"quipu", "ui"}) {
		t.Error("a normal command was taken for a browser call")
	}
	if !CaptureLoginURL([]string{"quipu", "https://claude.com/x"}) {
		t.Fatal("browser call not captured")
	}
	if b, _ := os.ReadFile(file); string(b) != "https://claude.com/x" {
		t.Errorf("file = %q", b)
	}
}
