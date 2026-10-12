package ui

import (
	"context"
	"encoding/json"
	"github.com/axvilab/quipu/internal/fsx"
	"path/filepath"
	"sync"
	"time"

	"github.com/axvilab/quipu/internal/claudecode"
	"github.com/axvilab/quipu/internal/profile"
)

// profileView is one profile in the sidebar.
type profileView struct {
	Name     string
	Account  string // email and plan, for the old pages
	Email    string
	Plan     string
	LoggedIn bool
	Current  bool // the profile this UI shows
}

// profileCache keeps login states for a few minutes: asking claude takes about half a second
// per profile, too slow for every page.
type profileCache struct {
	mu    sync.Mutex
	at    time.Time
	views []profileView
}

const profileCacheTTL = 5 * time.Minute

// profiles lists the configured profiles with their login state. Without ProfilesFile or a
// claude binary it shows just the current profile.
func (s *Service) profiles(ctx context.Context) []profileView {
	s.profileCache.mu.Lock()
	defer s.profileCache.mu.Unlock()
	if time.Since(s.profileCache.at) < profileCacheTTL && s.profileCache.views != nil {
		return s.profileCache.views
	}
	views := []profileView{{Name: s.profile, Current: true}}
	if s.ProfilesFile != "" && s.ClaudeBinary != "" {
		if c, err := profile.Load(s.home, s.ProfilesFile); err == nil {
			views = views[:0]
			for _, p := range c.Profiles {
				v := profileView{Name: p.Name, Current: p.Name == s.profile}
				actx, cancel := context.WithTimeout(ctx, 10*time.Second)
				if a, err := claudecode.AuthStatus(actx, s.ClaudeBinary, s.home, p.Dir); err == nil {
					v.LoggedIn, v.Account, v.Email, v.Plan = a.LoggedIn, a.Email, a.Email, a.SubscriptionType
					if tier := claudecode.PlanTier(s.home, p.Dir); a.LoggedIn && tier != "" {
						v.Plan = tier // "team standard" or "max 5×" says more about the limits than "team"
					}
					if v.Plan != "" {
						v.Account += " · " + v.Plan
					}
				}
				cancel()
				views = append(views, v)
			}
		}
	}
	s.profileCache.views, s.profileCache.at = views, time.Now()
	s.savePlans(views)
	return views
}

// PlansFile is where quipu notes each profile's subscription plan for the status line, which runs
// after every reply and cannot ask Claude Code itself.
func PlansFile(dataDir string) string { return filepath.Join(dataDir, "plans.json") }

func (s *Service) savePlans(views []profileView) {
	plans := map[string]string{}
	for _, v := range views {
		if v.LoggedIn && v.Plan != "" {
			plans[v.Name] = v.Plan
		}
	}
	if len(plans) == 0 {
		return
	}
	b, _ := json.Marshal(plans)
	fsx.WriteFile(PlansFile(s.dataDir), b)
}
