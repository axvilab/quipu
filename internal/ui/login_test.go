package ui

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

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

func TestProfilesAPI(t *testing.T) {
	s := testServer(t)
	h := s.Handler()
	cookie := &http.Cookie{Name: cookieName, Value: s.token}
	bin := t.TempDir()

	// Fake claude: "auth login" opens $BROWSER with a URL, waits a moment and marks the profile
	// as signed in; "auth status" reports that marker.
	claude := filepath.Join(bin, "claude")
	os.WriteFile(claude, []byte(`#!/bin/sh
dir="${CLAUDE_CONFIG_DIR:-$HOME/.claude}"
if [ "$2" = login ]; then [ "$3" = --email ] && echo "$4" > "$HOME/email-arg"; "$BROWSER" "https://claude.com/oauth/authorize?x=1"; sleep 1; mkdir -p "$dir"; touch "$dir/.signed"; exit 0; fi
if [ -f "$dir/.signed" ]; then echo '{"loggedIn":true,"email":"me@work.example","subscriptionType":"team"}'; else echo '{"loggedIn":false}'; fi
`), 0o755)
	browser := filepath.Join(bin, "browser")
	os.WriteFile(browser, []byte("#!/bin/sh\nprintf %s \"$1\" > \"$"+LoginURLEnv+"\"\n"), 0o755)
	s.ClaudeBinary, s.SelfBinary = claude, browser
	s.ProfilesFile = filepath.Join(s.home, ".config", "quipu", "profiles.json")
	t.Setenv("HOME", s.home)

	result := func(path, body string) Result {
		t.Helper()
		var r Result
		json.Unmarshal(postJSON(h, path, body, cookie).Body.Bytes(), &r)
		return r
	}
	if r := result("/api/profiles", `{"name":"work","share":["skills"]}`); !r.OK || r.Code != "profile-added" {
		t.Fatalf("add = %+v", r)
	}
	if r := result("/api/rules", `{"folder":"~/src/work","profile":"work"}`); !r.OK || r.Params["folder"] != "~/src/work" {
		t.Fatalf("rule = %+v", r)
	}
	if r := result("/api/profiles", `{"name":"Bad Name"}`); r.OK || r.Code != "profile-add-failed" {
		t.Errorf("bad name = %+v", r)
	}

	var p Profiles
	getJSON(t, h, "/api/profiles", cookie, &p)
	if len(p.Profiles) != 2 || p.Profiles[1].Name != "work" || p.Profiles[1].SignedIn || len(p.Rules) != 1 {
		t.Fatalf("profiles = %+v", p)
	}

	if r := result("/api/profiles/work/sign-in", `{"email":"me@work.example","private":false}`); !r.OK {
		t.Fatalf("sign-in = %+v", r)
	}
	sawURL, signed := false, false
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline) && !signed; time.Sleep(100 * time.Millisecond) {
		s.invalidateProfiles()
		getJSON(t, h, "/api/profiles", cookie, &p)
		w := p.Profiles[1]
		sawURL = sawURL || (w.SignIn.State == "open" && strings.Contains(w.SignIn.URL, "oauth/authorize"))
		signed = w.SignedIn && w.Account == "me@work.example" && w.Plan == "team"
	}
	if !sawURL {
		t.Error("the sign-in URL captured through $BROWSER was never offered")
	}
	if !signed {
		t.Error("profile not shown as signed in after login finished")
	}
	if b, _ := os.ReadFile(filepath.Join(s.home, "email-arg")); strings.TrimSpace(string(b)) != "me@work.example" {
		t.Errorf("claude auth login got --email %q", b)
	}
	if r := result("/api/profiles/work/sign-in", `{"email":"not an email"}`); r.Code != "email-invalid" {
		t.Errorf("bad email = %+v", r)
	}

	if r := result("/api/profiles/work/remove", `{}`); !r.OK || r.Code != "profile-removed" {
		t.Errorf("remove = %+v", r)
	}
	if _, err := os.Stat(filepath.Join(s.home, ".claude-work")); err != nil {
		t.Error("profile dir must be kept after remove")
	}
}
