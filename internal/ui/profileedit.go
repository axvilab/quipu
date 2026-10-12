package ui

import (
	"errors"
	"strconv"

	"github.com/axvilab/quipu/internal/profile"
)

// SetProfileLabel sets how a profile is shown everywhere in quipu.
func (s *Service) SetProfileLabel(name, label string) (Result, error) {
	c, err := s.loadProfiles()
	if err != nil {
		return Result{}, problem("profiles-unavailable")
	}
	if err := c.SetLabel(name, label); err != nil {
		return Result{}, problem("label-failed", "error", err.Error())
	}
	if err := c.Save(); err != nil {
		return Result{}, problem("label-failed", "error", err.Error())
	}
	s.invalidateProfiles()
	return ok("label-saved", "name", c.Find(name).Title()), nil
}

// ShareItem makes a profile use the default profile's item (CLAUDE.md, skills…), or gives it its
// own copy again.
func (s *Service) ShareItem(name, item string, on bool) (Result, error) {
	c, err := s.loadProfiles()
	if err != nil {
		return Result{}, problem("profiles-unavailable")
	}
	if on {
		err = c.Share(name, item)
	} else {
		err = c.Unshare(name, item)
	}
	var own *profile.OwnError
	switch {
	case errors.As(err, &own):
		return Result{}, problem("share-own", "item", own.Item, "count", strconv.Itoa(own.Files))
	case err != nil:
		return Result{}, problem("share-failed", "error", err.Error())
	}
	if err := c.Save(); err != nil {
		return Result{}, problem("share-failed", "error", err.Error())
	}
	s.invalidateProfiles()
	if on {
		return ok("share-on", "item", item, "name", c.Find(name).Title()), nil
	}
	return ok("share-off", "item", item, "name", c.Find(name).Title()), nil
}
