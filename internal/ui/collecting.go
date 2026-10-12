package ui

import (
	"context"
	"sync"
	"time"

	"github.com/axvilab/quipu/internal/collect"
)

// Collection is the state of collecting transcripts in the background.
type Collection struct {
	Running bool `json:"running"`
	Done    int  `json:"done"`  // transcripts gone through in the current run
	Total   int  `json:"total"` // transcripts in the current run
	// Generation grows whenever a run stored something new, so pages know to load again.
	Generation int `json:"generation"`
	// First: no run has finished yet and nothing is stored, so pages have nothing to show.
	First bool `json:"first"`
}

type collector struct {
	mu    sync.Mutex
	state Collection
	runs  int  // finished runs
	empty bool // the store was empty when the first run started
}

// Collection reports the background collection.
func (s *Service) Collection() Collection {
	s.coll.mu.Lock()
	c, runs, empty := s.coll.state, s.coll.runs, s.coll.empty
	started := c.Running || runs > 0
	s.coll.mu.Unlock()
	// Before the first run, the store tells; during it, what the store was when it started.
	c.First = runs == 0 && (empty || !started && s.st.Empty())
	return c
}

// Collect collects transcripts now and then every interval until ctx ends, and refreshes the
// profiles' login state with it. Pages read whatever is stored and never wait for it.
func (s *Service) Collect(ctx context.Context, every time.Duration) {
	s.upgradeStatusLines()
	for {
		s.collectOnce(ctx)
		// Login states and plans.json stay fresh without the UI open.
		s.refreshProfiles(ctx)
		s.autoCheck(ctx)
		select {
		case <-ctx.Done():
			return
		case <-time.After(every):
		}
	}
}

func (s *Service) collectOnce(ctx context.Context) {
	set := func(f func(c *Collection)) {
		s.coll.mu.Lock()
		f(&s.coll.state)
		s.coll.mu.Unlock()
	}
	empty := s.st.Empty()
	s.coll.mu.Lock()
	if s.coll.runs == 0 {
		s.coll.empty = empty
	}
	s.coll.mu.Unlock()
	set(func(c *Collection) { c.Running, c.Done, c.Total = true, 0, 0 })
	stats, _ := collect.RunProgress(ctx, s.home, s.st, func(done, total int) {
		set(func(c *Collection) { c.Done, c.Total = done, total })
	})
	s.coll.mu.Lock()
	s.coll.state.Running = false
	s.coll.runs++
	if stats.Collected > 0 {
		s.coll.state.Generation++
	}
	s.coll.mu.Unlock()
}
