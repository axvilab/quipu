// Package analytics turns stored responses into cache and cost reports.
package analytics

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/axvilab/quipu/internal/store"
)

// Totals are token sums over a group of responses.
type Totals struct {
	Sessions      int64
	Requests      int64
	Input         int64 // uncached input tokens
	CacheRead     int64
	CacheCreation int64
	Output        int64
	Misses        int64 // responses with an API-reported cache miss reason
}

// Prompt is every input token sent: uncached, read from cache and written to cache.
func (t Totals) Prompt() int64 {
	return t.Input + t.CacheRead + t.CacheCreation
}

// HitRate is the share of prompt tokens served from cache, 0..1.
func (t Totals) HitRate() float64 {
	if t.Prompt() == 0 {
		return 0
	}
	return float64(t.CacheRead) / float64(t.Prompt())
}

// ProjectRow is one project in the per-project report.
type ProjectRow struct {
	ProjectDir string
	Path       string // working directory of the project's earliest response
	Totals
}

// DayRow is one day in the per-day report.
type DayRow struct {
	Day time.Time
	Totals
}

// MissRow counts responses by cache miss reason.
type MissRow struct {
	Reason       string
	Count        int64
	MissedTokens int64 // only part of the misses report this, so it is a lower bound
}

// Filter limits a report to responses in [Since, Until) and optionally to one project.
type Filter struct {
	Since      time.Time
	Until      time.Time // zero: up to now
	ProjectDir string    // empty for all projects
}

// Previous is the window of the same length right before f, for comparisons.
func (f Filter) Previous() Filter {
	until := f.Until
	if until.IsZero() {
		until = time.Now()
	}
	length := until.Sub(f.Since)
	return Filter{Since: f.Since.Add(-length), Until: f.Since, ProjectDir: f.ProjectDir}
}

func (f Filter) where() (string, []any) {
	w, args := "ts >= ?", []any{f.Since.UTC()}
	if !f.Until.IsZero() {
		w += " AND ts < ?"
		args = append(args, f.Until.UTC())
	}
	if f.ProjectDir != "" {
		w += " AND project_dir = ?"
		args = append(args, f.ProjectDir)
	}
	return w, args
}

const totalsCols = `
	count(DISTINCT session_id), count(*),
	coalesce(sum(input_tokens), 0), coalesce(sum(cache_read_tokens), 0),
	coalesce(sum(cache_creation_tokens), 0), coalesce(sum(output_tokens), 0),
	count(cache_miss_type)`

func scanTotals(t *Totals) []any {
	return []any{&t.Sessions, &t.Requests, &t.Input, &t.CacheRead, &t.CacheCreation, &t.Output, &t.Misses}
}

// ByProject reports cache use per project, largest prompt volume first.
func ByProject(ctx context.Context, st *store.Store, f Filter) ([]ProjectRow, error) {
	where, args := f.where()
	rows, err := st.Query(ctx, `
		SELECT project_dir, arg_min(cwd, ts), `+totalsCols+`
		FROM responses WHERE `+where+`
		GROUP BY project_dir
		ORDER BY sum(input_tokens + cache_read_tokens + cache_creation_tokens) DESC`, args...)
	if err != nil {
		return nil, fmt.Errorf("query by project: %w", err)
	}
	defer rows.Close()

	var out []ProjectRow
	for rows.Next() {
		var r ProjectRow
		if err := rows.Scan(append([]any{&r.ProjectDir, &r.Path}, scanTotals(&r.Totals)...)...); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Total sums every response in the filter.
func Total(ctx context.Context, st *store.Store, f Filter) (Totals, error) {
	where, args := f.where()
	rows, err := st.Query(ctx, `SELECT `+totalsCols+` FROM responses WHERE `+where, args...)
	if err != nil {
		return Totals{}, fmt.Errorf("query total: %w", err)
	}
	defer rows.Close()
	var t Totals
	if rows.Next() {
		err = rows.Scan(scanTotals(&t)...)
	}
	return t, err
}

// ByDay reports cache use per day (UTC), oldest first.
func ByDay(ctx context.Context, st *store.Store, f Filter) ([]DayRow, error) {
	where, args := f.where()
	rows, err := st.Query(ctx, `
		SELECT date_trunc('day', ts), `+totalsCols+`
		FROM responses WHERE `+where+`
		GROUP BY 1 ORDER BY 1`, args...)
	if err != nil {
		return nil, fmt.Errorf("query by day: %w", err)
	}
	defer rows.Close()

	var out []DayRow
	for rows.Next() {
		var r DayRow
		if err := rows.Scan(append([]any{&r.Day}, scanTotals(&r.Totals)...)...); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Misses counts API-reported cache miss reasons, most frequent first.
func Misses(ctx context.Context, st *store.Store, f Filter) ([]MissRow, error) {
	where, args := f.where()
	rows, err := st.Query(ctx, `
		SELECT cache_miss_type, count(*), coalesce(sum(cache_miss_tokens), 0)
		FROM responses WHERE `+where+` AND cache_miss_type IS NOT NULL
		GROUP BY 1 ORDER BY 2 DESC, 1`, args...)
	if err != nil {
		return nil, fmt.Errorf("query misses: %w", err)
	}
	defer rows.Close()

	var out []MissRow
	for rows.Next() {
		var r MissRow
		if err := rows.Scan(&r.Reason, &r.Count, &r.MissedTokens); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Earliest is the time of the first stored response, limited to f.ProjectDir when set, ignoring
// the time window. It is zero when nothing is stored.
func Earliest(ctx context.Context, st *store.Store, f Filter) (time.Time, error) {
	where, args := "true", []any{}
	if f.ProjectDir != "" {
		where, args = "project_dir = ?", []any{f.ProjectDir}
	}
	rows, err := st.Query(ctx, `SELECT min(ts) FROM responses WHERE `+where, args...)
	if err != nil {
		return time.Time{}, fmt.Errorf("query earliest: %w", err)
	}
	defer rows.Close()
	var ts *time.Time
	if rows.Next() {
		err = rows.Scan(&ts)
	}
	if ts == nil {
		return time.Time{}, err
	}
	return *ts, err
}

// PluginUse is where a plugin was used: the project, its folder, and how often and when last.
type PluginUse struct {
	Plugin     string
	ProjectDir string
	Path       string
	Responses  int64
	Last       time.Time
}

// PluginUses reports, per project, the plugins its responses used in the filter's window.
func PluginUses(ctx context.Context, st *store.Store, f Filter) ([]PluginUse, error) {
	where, args := f.where()
	rows, err := st.Query(ctx, `
		SELECT project_dir, arg_min(cwd, ts), plugins, count(*), max(ts)
		FROM responses WHERE `+where+` AND plugins IS NOT NULL AND plugins <> ''
		GROUP BY project_dir, plugins`, args...)
	if err != nil {
		return nil, fmt.Errorf("query plugin use: %w", err)
	}
	defer rows.Close()
	type key struct{ project, plugin string }
	merged := map[key]*PluginUse{}
	var order []key
	for rows.Next() {
		var dir, path, plugins string
		var n int64
		var last time.Time
		if err := rows.Scan(&dir, &path, &plugins, &n, &last); err != nil {
			return nil, err
		}
		for _, p := range strings.Split(plugins, ",") {
			k := key{dir, p}
			u, ok := merged[k]
			if !ok {
				u = &PluginUse{Plugin: p, ProjectDir: dir, Path: path}
				merged[k] = u
				order = append(order, k)
			}
			u.Responses += n
			if last.After(u.Last) {
				u.Last = last
			}
		}
	}
	out := make([]PluginUse, 0, len(order))
	for _, k := range order {
		out = append(out, *merged[k])
	}
	return out, rows.Err()
}
