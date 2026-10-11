package analytics

import (
	"context"
	"fmt"
	"math"
	"time"
	"unicode/utf8"

	"github.com/axvilab/quipu/internal/store"
)

// EstimateTokens approximates how many tokens text costs. There is no tokenizer offline, so it
// uses typical ratios: about 4 characters per token for ASCII text and code, fewer for other
// scripts such as Cyrillic. Good enough to rank items, not to bill them.
func EstimateTokens(text string) int64 {
	var ascii, other float64
	for _, r := range text {
		if r < utf8.RuneSelf {
			ascii++
		} else {
			other++
		}
	}
	return int64(math.Ceil(ascii/4 + other/1.6))
}

// StartWeight is the measured size of the first request of a project's sessions: everything
// Claude Code sends before any work, including its own system prompt and tool definitions.
type StartWeight struct {
	Sessions int64
	Median   int64
	Min      int64
	Max      int64
	Latest   int64 // most recent session, reflects the current setup
	LatestAt time.Time
}

// SessionStart measures the start weight of main sessions (subagents excluded) of a project.
func SessionStart(ctx context.Context, st *store.Store, f Filter) (StartWeight, error) {
	var w StartWeight
	where, args := f.where()
	rows, err := st.Query(ctx, `
		WITH firsts AS (
			SELECT session_id,
			       arg_min(input_tokens + cache_read_tokens + cache_creation_tokens, ts) AS tokens,
			       min(ts) AS started
			FROM responses
			WHERE `+where+` AND transcript_kind = 'session' AND NOT sidechain
			GROUP BY session_id
		)
		SELECT count(*), coalesce(median(tokens), 0)::BIGINT, coalesce(min(tokens), 0), coalesce(max(tokens), 0),
		       coalesce(arg_max(tokens, started), 0), max(started)
		FROM firsts`, args...)
	if err != nil {
		return w, fmt.Errorf("query session start: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		return w, rows.Err()
	}
	var latestAt *time.Time
	if err := rows.Scan(&w.Sessions, &w.Median, &w.Min, &w.Max, &w.Latest, &latestAt); err != nil {
		return w, err
	}
	if latestAt != nil {
		w.LatestAt = *latestAt
	}
	return w, nil
}
