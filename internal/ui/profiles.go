package ui

import (
	"context"
	"sync"
	"time"

	"github.com/axvilab/quipu/internal/claudecode"
	"github.com/axvilab/quipu/internal/profile"
)

// profileView is one profile in the sidebar.
type profileView struct {
	Name     string
	Account  string
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
func (s *Server) profiles(ctx context.Context) []profileView {
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
					v.LoggedIn, v.Account = a.LoggedIn, a.Email
					if a.SubscriptionType != "" {
						v.Account += " · " + a.SubscriptionType
					}
				}
				cancel()
				views = append(views, v)
			}
		}
	}
	s.profileCache.views, s.profileCache.at = views, time.Now()
	return views
}
