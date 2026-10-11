package ui

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/axvilab/quipu/internal/analytics"
	"github.com/axvilab/quipu/internal/claudecode"
	"github.com/axvilab/quipu/internal/profile"
)

// LoginURLEnv names the file where quipu, started by claude as its BROWSER, stores the sign-in
// URL instead of opening a browser. See CaptureLoginURL.
const LoginURLEnv = "QUIPU_LOGIN_URL_FILE"

// CaptureLoginURL handles quipu being run by `claude auth login` as $BROWSER: it writes the URL
// for the UI to show and reports true. Called first thing in main.
func CaptureLoginURL(args []string) bool {
	file := os.Getenv(LoginURLEnv)
	if file == "" || len(args) != 2 || !strings.HasPrefix(args[1], "https://") {
		return false
	}
	os.WriteFile(file, []byte(args[1]), 0o600)
	return true
}

// loginTimeout bounds how long a started sign-in waits for the user.
const loginTimeout = 10 * time.Minute

// login is a running `claude auth login` for one profile.
type login struct {
	urlFile string
	done    bool
	err     error
}

type logins struct {
	mu sync.Mutex
	m  map[string]*login
}

func (l *logins) get(name string) *login {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.m[name]
}

// startLogin runs `claude auth login` for profile p in the background. claude opens the browser
// through $BROWSER, which is quipu itself, so the URL ends up in a file the profiles page reads
// and offers as a link: this works even when the daemon has no access to the desktop session.
func (s *Server) startLogin(p *profile.Profile) error {
	s.logins.mu.Lock()
	defer s.logins.mu.Unlock()
	if s.logins.m == nil {
		s.logins.m = map[string]*login{}
	}
	if l := s.logins.m[p.Name]; l != nil && !l.done {
		return nil // already waiting for this one
	}
	if s.ClaudeBinary == "" || s.SelfBinary == "" {
		return fmt.Errorf("claude was not found on PATH")
	}
	l := &login{urlFile: filepath.Join(s.dataDir, "login-"+p.Name+".url")}
	os.Remove(l.urlFile)

	ctx, cancel := context.WithTimeout(context.Background(), loginTimeout)
	cmd := exec.CommandContext(ctx, s.ClaudeBinary, "auth", "login")
	cmd.Env = append(claudecode.Env(s.home, p.Dir), "BROWSER="+s.SelfBinary, LoginURLEnv+"="+l.urlFile)
	if err := cmd.Start(); err != nil {
		cancel()
		return err
	}
	s.logins.m[p.Name] = l
	go func() {
		err := cmd.Wait()
		cancel()
		s.logins.mu.Lock()
		l.done, l.err = true, err
		s.logins.mu.Unlock()
		os.Remove(l.urlFile)
		s.invalidateProfiles()
	}()
	return nil
}

func (s *Server) invalidateProfiles() {
	s.profileCache.mu.Lock()
	s.profileCache.at = time.Time{}
	s.profileCache.mu.Unlock()
}

func (s *Server) loadProfiles() (*profile.Config, error) {
	if s.ProfilesFile == "" {
		return nil, fmt.Errorf("profiles are not configured")
	}
	return profile.Load(s.home, s.ProfilesFile)
}

type profileRow struct {
	profileView
	Dir    string
	Shares []string
	Login  string // "", "waiting", "url" while signing in
	URL    string
}

func (s *Server) profilesPage(w http.ResponseWriter, r *http.Request) {
	c, err := s.loadProfiles()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	views := map[string]profileView{}
	for _, v := range s.profiles(r.Context()) {
		views[v.Name] = v
	}
	var rows []profileRow
	waiting := false
	for _, p := range c.Profiles {
		row := profileRow{profileView: views[p.Name], Dir: s.shortPath(p.Dir)}
		row.Name = p.Name
		for _, link := range p.Links {
			row.Shares = append(row.Shares, filepath.Base(link))
		}
		if l := s.logins.get(p.Name); l != nil && !l.done {
			waiting = true
			row.Login = "waiting"
			if b, err := os.ReadFile(l.urlFile); err == nil {
				row.Login, row.URL = "url", string(b)
			}
		}
		rows = append(rows, row)
	}

	var rules []profile.Rule
	for _, rule := range c.Rules {
		rule.Path = s.shortPath(rule.Path)
		rules = append(rules, rule)
	}
	var folders []string
	if projects, err := analytics.ByProject(r.Context(), s.st, analytics.Filter{Since: time.Now().AddDate(0, 0, -90)}); err == nil {
		for _, p := range projects {
			folders = append(folders, s.shortPath(p.Path))
		}
		sort.Strings(folders)
	}
	s.render(w, r, "profiles", "profiles", "Profiles", struct {
		Rows      []profileRow
		Rules     []profile.Rule
		Folders   []string
		Shareable []string
		Waiting   bool
	}{rows, rules, folders, profile.Shareable, waiting})
}

func (s *Server) profileAdd(w http.ResponseWriter, r *http.Request) {
	c, err := s.loadProfiles()
	if err != nil {
		s.back(w, r, err.Error())
		return
	}
	r.ParseForm()
	p, err := c.Add(strings.TrimSpace(r.FormValue("name")), r.Form["share"])
	if err == nil {
		err = c.Save()
	}
	if err != nil {
		s.back(w, r, "Not added: "+err.Error())
		return
	}
	s.invalidateProfiles()
	s.back(w, r, fmt.Sprintf("Profile %s created in %s. Sign in to it with the Sign in button.", p.Name, s.shortPath(p.Dir)))
}

func (s *Server) profileRemove(w http.ResponseWriter, r *http.Request) {
	c, err := s.loadProfiles()
	if err != nil {
		s.back(w, r, err.Error())
		return
	}
	dir, err := c.Remove(r.FormValue("name"))
	if err == nil {
		err = c.Save()
	}
	if err != nil {
		s.back(w, r, "Not removed: "+err.Error())
		return
	}
	s.invalidateProfiles()
	s.back(w, r, "Profile removed. Its directory with the login and sessions is kept: "+s.shortPath(dir))
}

func (s *Server) profileRule(w http.ResponseWriter, r *http.Request) {
	c, err := s.loadProfiles()
	if err != nil {
		s.back(w, r, err.Error())
		return
	}
	folder := strings.TrimSpace(r.FormValue("folder"))
	if rest, ok := strings.CutPrefix(folder, "~/"); ok {
		folder = filepath.Join(s.home, rest)
	}
	if !filepath.IsAbs(folder) {
		s.back(w, r, "Give the folder as an absolute path or starting with ~/.")
		return
	}
	err = c.SetRule(folder, r.FormValue("profile"))
	if err == nil {
		err = c.Save()
	}
	if err != nil {
		s.back(w, r, "Rule not saved: "+err.Error())
		return
	}
	s.back(w, r, fmt.Sprintf("Claude in %s and below now runs as %s (through quipu run).", s.shortPath(folder), r.FormValue("profile")))
}

func (s *Server) profileUnrule(w http.ResponseWriter, r *http.Request) {
	c, err := s.loadProfiles()
	if err != nil {
		s.back(w, r, err.Error())
		return
	}
	folder := r.FormValue("folder")
	if rest, ok := strings.CutPrefix(folder, "~/"); ok {
		folder = filepath.Join(s.home, rest)
	}
	if c.DeleteRule(folder) {
		if err := c.Save(); err != nil {
			s.back(w, r, err.Error())
			return
		}
	}
	s.back(w, r, "Rule deleted.")
}

func (s *Server) profileLogin(w http.ResponseWriter, r *http.Request) {
	c, err := s.loadProfiles()
	if err != nil {
		s.back(w, r, err.Error())
		return
	}
	p := c.Find(r.FormValue("name"))
	if p == nil {
		s.back(w, r, "Unknown profile.")
		return
	}
	if err := s.startLogin(p); err != nil {
		s.back(w, r, "Could not start sign-in: "+err.Error())
		return
	}
	http.Redirect(w, r, "/profiles", http.StatusSeeOther)
}
