// Package collect copies Claude Code transcripts of every profile into the store before
// Claude Code's 30-day cleanup deletes them.
package collect

import (
	"context"
	"fmt"
	"os"

	"github.com/axvilab/quipu/internal/claudecode"
	"github.com/axvilab/quipu/internal/store"
)

// Stats summarizes one collection run.
type Stats struct {
	Profiles  int
	Seen      int // transcripts found
	Collected int // transcripts new or changed since the last run
	Responses int // responses in the collected transcripts
	Malformed int // unreadable lines skipped in the collected transcripts
	Failed    int // transcripts that could not be read or stored
}

// Run collects every new or changed transcript found under home.
// A transcript that fails is counted in Stats.Failed and the run continues; the first such
// error is returned together with the stats.
func Run(ctx context.Context, home string, st *store.Store) (Stats, error) {
	var stats Stats
	dirs, err := claudecode.ConfigDirs(home)
	if err != nil {
		return stats, fmt.Errorf("find config dirs: %w", err)
	}

	var firstErr error
	for _, dir := range dirs {
		stats.Profiles++
		transcripts, err := claudecode.Transcripts(dir)
		if err != nil {
			return stats, fmt.Errorf("list transcripts in %s: %w", dir, err)
		}
		profile := claudecode.ProfileName(dir)
		for _, t := range transcripts {
			if err := ctx.Err(); err != nil {
				return stats, err
			}
			stats.Seen++
			n, malformed, err := collectOne(ctx, st, profile, t)
			if err != nil {
				stats.Failed++
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			if n >= 0 {
				stats.Collected++
				stats.Responses += n
				stats.Malformed += malformed
			}
		}
	}
	return stats, firstErr
}

// collectOne stores one transcript. It returns -1 responses when the transcript is unchanged
// since the last run and was skipped.
func collectOne(ctx context.Context, st *store.Store, profile string, t claudecode.Transcript) (responses, malformed int, err error) {
	fi, err := os.Stat(t.Path)
	if err != nil {
		return 0, 0, err
	}
	if st.UpToDate(t.Path, fi.ModTime()) {
		return -1, 0, nil
	}

	f, err := os.Open(t.Path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()

	rs, rstats, err := claudecode.ReadResponses(f)
	if err != nil {
		return 0, 0, fmt.Errorf("%s: %w", t.Path, err)
	}
	src := store.Source{
		Path:       t.Path,
		ModTime:    fi.ModTime(),
		Profile:    profile,
		ProjectDir: t.ProjectDir,
		SessionID:  t.SessionID,
		Kind:       t.Kind,
	}
	if err := st.WriteResponses(ctx, src, rs); err != nil {
		return 0, 0, fmt.Errorf("store %s: %w", t.Path, err)
	}
	return len(rs), rstats.Malformed, nil
}
