package ui

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/axvilab/quipu/internal/profile"
)

// ShareMemory shares (or stops sharing) the project memory of folder with the profile its rule
// names, through the journal.
func (s *Service) ShareMemory(folder string, on bool) (Result, error) {
	c, err := s.loadProfiles()
	if err != nil {
		return Result{}, problem("profiles-unavailable")
	}
	folder = filepath.Clean(s.expandHome(strings.TrimSpace(folder)))
	var rule *profile.Rule
	for i := range c.Rules {
		if c.Rules[i].Path == folder {
			rule = &c.Rules[i]
		}
	}
	if rule == nil {
		return Result{}, problem("rule-missing", "folder", s.shortPath(folder))
	}
	var id string
	if on {
		id, err = profile.ShareMemory(c, s.home, folder, rule.Profile, s.journal)
	} else {
		id, err = profile.UnshareMemory(c, s.home, folder, rule.Profile, s.journal)
	}
	var conflict *profile.ConflictError
	switch {
	case errors.Is(err, profile.ErrNoMemory):
		return Result{}, problem("memory-none", "name", rule.Profile)
	case errors.As(err, &conflict):
		return Result{}, problem("memory-conflict", "files", strings.Join(conflict.Files, ", "))
	case err != nil:
		return Result{}, problem("memory-failed", "error", err.Error())
	}
	state := "unshared"
	if on {
		state = "shared"
	}
	return ok("memory-"+state, "name", rule.Profile, "folder", s.shortPath(folder), "change", id), nil
}
