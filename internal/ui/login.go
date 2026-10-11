package ui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

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
func (s *Service) startLogin(p *profile.Profile, email string) error {
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
	args := []string{"auth", "login"}
	if email != "" {
		// Pre-filled on the sign-in page, so a browser already signed in to another account
		// does not just confirm that one.
		args = append(args, "--email", email)
	}
	cmd := exec.CommandContext(ctx, s.ClaudeBinary, args...)
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

func (s *Service) invalidateProfiles() {
	s.profileCache.mu.Lock()
	s.profileCache.at = time.Time{}
	s.profileCache.mu.Unlock()
}

func (s *Service) loadProfiles() (*profile.Config, error) {
	if s.ProfilesFile == "" {
		return nil, fmt.Errorf("profiles are not configured")
	}
	return profile.Load(s.home, s.ProfilesFile)
}

// waitLoginURL returns the sign-in URL of a running login, waiting up to d for claude to hand it
// over.
func (s *Service) waitLoginURL(name string, d time.Duration) string {
	deadline := time.Now().Add(d)
	for {
		if l := s.logins.get(name); l != nil && !l.done {
			if b, err := os.ReadFile(l.urlFile); err == nil && len(b) > 0 {
				return string(b)
			}
		}
		if time.Now().After(deadline) {
			return ""
		}
		time.Sleep(100 * time.Millisecond)
	}
}
