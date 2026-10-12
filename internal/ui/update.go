package ui

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/axvilab/quipu/internal/fsx"
	"github.com/axvilab/quipu/internal/update"
)

// UpdateInfo is what the UI shows next to the version: whether a newer release exists and
// whether quipu can install it itself.
type UpdateInfo struct {
	Current    string `json:"current"`
	Latest     string `json:"latest,omitempty"`
	Available  bool   `json:"available"`
	Checked    string `json:"checked,omitempty"` // RFC 3339, when GitHub was last asked
	Auto       bool   `json:"auto"`              // check once a day; off unless the user turns it on
	CanInstall bool   `json:"canInstall"`        // false for a dev build and when quipu's folder is read-only
	Restarts   bool   `json:"restarts"`          // the running quipu restarts itself after installing
	Notes      string `json:"notes,omitempty"`   // the release page
}

// updateState is kept in <dataDir>/update.json.
type updateState struct {
	Auto    bool      `json:"auto"`
	Checked time.Time `json:"checked"`
	Latest  string    `json:"latest"`
}

const autoCheckEvery = 24 * time.Hour

// updater serializes checks and installs.
type updater struct{ mu sync.Mutex }

func (s *Service) updateFile() string { return filepath.Join(s.dataDir, "update.json") }

func (s *Service) loadUpdate() updateState {
	var st updateState
	if b, err := os.ReadFile(s.updateFile()); err == nil {
		json.Unmarshal(b, &st)
	}
	return st
}

func (s *Service) saveUpdate(st updateState) error {
	b, err := json.Marshal(st)
	if err != nil {
		return err
	}
	return fsx.WriteFile(s.updateFile(), b)
}

func (s *Service) updateBase() string {
	if s.UpdateBase != "" {
		return s.UpdateBase
	}
	return update.DefaultBase
}

// UpdateStatus says what is known without asking GitHub.
func (s *Service) UpdateStatus() UpdateInfo {
	st := s.loadUpdate()
	info := UpdateInfo{Current: s.version, Latest: st.Latest, Auto: st.Auto, Restarts: s.Restart != nil}
	info.Available = update.Newer(s.version, st.Latest)
	if !st.Checked.IsZero() {
		info.Checked = st.Checked.Format(time.RFC3339)
	}
	if info.Available {
		info.Notes = s.updateBase() + "/releases/tag/" + st.Latest
	}
	info.CanInstall = s.canInstall()
	return info
}

// canInstall: a release build whose binary sits in a folder quipu may write to.
func (s *Service) canInstall() bool {
	if !update.Newer("v0.0.0", s.version) || s.SelfBinary == "" {
		return false
	}
	exe, err := filepath.EvalSymlinks(s.SelfBinary)
	if err != nil {
		return false
	}
	f, err := os.CreateTemp(filepath.Dir(exe), ".quipu-write-check-")
	if err != nil {
		return false
	}
	f.Close()
	os.Remove(f.Name())
	return true
}

// CheckUpdate asks GitHub for the newest release, the only request quipu makes on its own
// (ADR 0008).
func (s *Service) CheckUpdate(ctx context.Context) (UpdateInfo, error) {
	s.upd.mu.Lock()
	defer s.upd.mu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	tag, err := update.Latest(ctx, s.updateBase())
	if err != nil {
		return UpdateInfo{}, &Problem{Code: "update-check-failed", Params: map[string]string{"error": err.Error()}}
	}
	st := s.loadUpdate()
	st.Checked, st.Latest = time.Now(), tag
	if err := s.saveUpdate(st); err != nil {
		return UpdateInfo{}, err
	}
	return s.UpdateStatus(), nil
}

// SetAutoUpdate turns the daily check on or off.
func (s *Service) SetAutoUpdate(on bool) (Result, error) {
	s.upd.mu.Lock()
	st := s.loadUpdate()
	st.Auto = on
	err := s.saveUpdate(st)
	s.upd.mu.Unlock()
	if err != nil {
		return Result{}, err
	}
	code := "update-auto-off"
	if on {
		code = "update-auto-on"
	}
	return Result{OK: true, Code: code}, nil
}

// autoCheck checks once a day when the user asked for it; failures wait for the next day too.
func (s *Service) autoCheck(ctx context.Context) {
	st := s.loadUpdate()
	if !st.Auto || time.Since(st.Checked) < autoCheckEvery {
		return
	}
	if _, err := s.CheckUpdate(ctx); err != nil {
		st.Checked = time.Now()
		s.saveUpdate(st)
	}
}

// InstallUpdate installs the newest release found by the last check over the running binary,
// then restarts quipu when it knows how; otherwise the user restarts it.
func (s *Service) InstallUpdate(ctx context.Context) (Result, error) {
	s.upd.mu.Lock()
	defer s.upd.mu.Unlock()
	st := s.loadUpdate()
	if !update.Newer(s.version, st.Latest) {
		return Result{OK: true, Code: "update-none", Params: map[string]string{"version": s.version}}, nil
	}
	if !s.canInstall() {
		return Result{}, &Problem{Code: "update-cannot-install"}
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	apps := ""
	if s.home != "" {
		apps = filepath.Join(s.home, "Applications")
	}
	if err := update.Install(ctx, s.updateBase(), st.Latest, s.SelfBinary, apps); err != nil {
		return Result{}, &Problem{Code: "update-failed", Params: map[string]string{"error": err.Error()}}
	}
	params := map[string]string{"version": st.Latest}
	if s.Restart == nil {
		return Result{OK: true, Code: "update-installed-restart", Params: params}, nil
	}
	// Answer first, then restart: the page waits for the new version to come up.
	go func() {
		time.Sleep(500 * time.Millisecond)
		s.Restart()
	}()
	return Result{OK: true, Code: "update-installed", Params: params}, nil
}
