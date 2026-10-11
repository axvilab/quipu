package ui

import (
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/axvilab/quipu/internal/advise"
	"github.com/axvilab/quipu/internal/analytics"
	"github.com/axvilab/quipu/internal/claudecode"
	"github.com/axvilab/quipu/internal/collect"
	"github.com/axvilab/quipu/internal/fsx"
)

func parsePages() (map[string]*template.Template, error) {
	funcs := template.FuncMap{
		"human": analytics.Human,
		"pct":   func(f float64) string { return fmt.Sprintf("%.1f%%", 100*f) },
		"diffClass": func(line string) string {
			switch {
			case strings.HasPrefix(line, "+"):
				return "add"
			case strings.HasPrefix(line, "-"):
				return "del"
			}
			return ""
		},
		"lines": func(s string) []string { return strings.Split(s, "\n") },
		"last":  func(rows []dayRow) dayRow { return rows[len(rows)-1] },
		"head":  func(rows []itemRow, n int) []itemRow { return rows[:min(n, len(rows))] },
		"tail":  func(rows []itemRow, n int) []itemRow { return rows[min(n, len(rows)):] },
	}
	names, err := fs.Glob(assets, "templates/*.html")
	if err != nil {
		return nil, err
	}
	pages := map[string]*template.Template{}
	for _, name := range names {
		base := filepath.Base(name)
		if base == "layout.html" {
			continue
		}
		t, err := template.New("layout.html").Funcs(funcs).ParseFS(assets, "templates/layout.html", name)
		if err != nil {
			return nil, err
		}
		pages[strings.TrimSuffix(base, ".html")] = t
	}
	return pages, nil
}

// page is what every template gets: the active sidebar entry plus the page's own data.
type page struct {
	Nav      string
	Title    string
	Profile  string
	Profiles []profileView
	Msg      string // result of the last action, shown once
	Data     any
}

func (s *Server) render(w http.ResponseWriter, r *http.Request, name, nav, title string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	p := page{Nav: nav, Title: title, Profile: s.profile, Profiles: s.profiles(r.Context()), Msg: r.URL.Query().Get("msg"), Data: data}
	if err := s.pages[name].Execute(w, p); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func days(r *http.Request) int {
	if d, err := strconv.Atoi(r.URL.Query().Get("days")); err == nil && d > 0 && d <= 365 {
		return d
	}
	return 30
}

// refresh collects new transcripts so every page shows current numbers.
func (s *Server) refresh(r *http.Request) {
	collect.Run(r.Context(), s.home, s.st)
}

func (s *Server) shortPath(p string) string {
	if rel, ok := strings.CutPrefix(p, s.home); ok {
		return "~" + rel
	}
	return p
}

// ---- overview ----

type projectRow struct {
	Dir      string
	Path     string
	Totals   analytics.Totals
	Start    analytics.StartWeight
	Findings int
}

func (s *Server) overview(w http.ResponseWriter, r *http.Request) {
	s.refresh(r)
	d := days(r)
	f := analytics.Filter{Since: time.Now().AddDate(0, 0, -d)}
	projects, err := analytics.ByProject(r.Context(), s.st, f)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var rows []projectRow
	var total analytics.Totals
	for _, p := range projects {
		pf := f
		pf.ProjectDir = p.ProjectDir
		start, _ := analytics.SessionStart(r.Context(), s.st, pf)
		findings := advise.Run(advise.Input{Home: s.home, ConfigDir: s.configDir, Project: p.Path,
			Items: claudecode.Instructions(s.home, s.configDir, p.Path)}, nil)
		rows = append(rows, projectRow{Dir: p.ProjectDir, Path: s.shortPath(p.Path), Totals: p.Totals, Start: start, Findings: len(findings)})
		total.Sessions += p.Sessions
		total.Requests += p.Requests
		total.Input += p.Input
		total.CacheRead += p.CacheRead
		total.CacheCreation += p.CacheCreation
		total.Output += p.Output
		total.Misses += p.Misses
	}
	s.render(w, r, "overview", "overview", "Projects", struct {
		Days  int
		Rows  []projectRow
		Total analytics.Totals
	}{d, rows, total})
}

// ---- project ----

type itemRow struct {
	Tokens int64
	Bar    int // width in percent of the heaviest item on the page
	Scope  string
	Kind   string
	Label  string
}

func (s *Server) project(w http.ResponseWriter, r *http.Request) {
	s.refresh(r)
	dir := r.URL.Query().Get("dir")
	f := analytics.Filter{Since: time.Now().AddDate(0, 0, -30), ProjectDir: dir}
	projects, err := analytics.ByProject(r.Context(), s.st, f)
	if err != nil || len(projects) == 0 {
		http.Error(w, "unknown project", http.StatusNotFound)
		return
	}
	path := projects[0].Path
	items := claudecode.Instructions(s.home, s.configDir, path)

	var always, onDemand []itemRow
	var alwaysTotal, onDemandTotal, max int64
	for _, it := range items {
		row := itemRow{Tokens: analytics.EstimateTokens(it.Text), Scope: it.Scope, Kind: it.Kind, Label: s.itemLabel(it, path)}
		if row.Tokens > max {
			max = row.Tokens
		}
		if it.Load == claudecode.Always {
			always = append(always, row)
			alwaysTotal += row.Tokens
		} else {
			onDemand = append(onDemand, row)
			onDemandTotal += row.Tokens
		}
	}
	for _, rows := range [][]itemRow{always, onDemand} {
		sort.SliceStable(rows, func(i, j int) bool { return rows[i].Tokens > rows[j].Tokens })
		for i := range rows {
			if max > 0 {
				rows[i].Bar = int(100 * rows[i].Tokens / max)
			}
		}
	}
	start, _ := analytics.SessionStart(r.Context(), s.st, f)
	var findings []findingView
	for _, f := range advise.Run(advise.Input{Home: s.home, ConfigDir: s.configDir, Project: path, Items: items}, nil) {
		findings = append(findings, findingView{Finding: f, Dir: dir, ShowPath: s.shortPath(f.Path)})
	}

	s.render(w, r, "project", "overview", s.shortPath(path), struct {
		Path          string
		Totals        analytics.Totals
		Start         analytics.StartWeight
		Always        []itemRow
		OnDemand      []itemRow
		AlwaysTotal   int64
		OnDemandTotal int64
		Findings      []findingView
	}{s.shortPath(path), projects[0].Totals, start, always, onDemand, alwaysTotal, onDemandTotal, findings})
}

func (s *Server) itemLabel(it claudecode.Item, project string) string {
	switch it.Kind {
	case "skill", "agent", "command", "memory":
		return it.Name
	}
	if rel, err := filepath.Rel(project, it.Path); err == nil && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return s.shortPath(it.Path)
}

// ---- cache ----

type dayRow struct {
	Day    time.Time
	Totals analytics.Totals
	Bar    int // cache writes in percent of the busiest day
}

type missRow struct {
	Reason string
	Count  int64
	Hint   string
}

func (s *Server) cache(w http.ResponseWriter, r *http.Request) {
	s.refresh(r)
	d := days(r)
	f := analytics.Filter{Since: time.Now().AddDate(0, 0, -d)}
	daily, err := analytics.ByDay(r.Context(), s.st, f)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var rows []dayRow
	var total analytics.Totals
	var max int64
	for _, day := range daily {
		if day.CacheCreation > max {
			max = day.CacheCreation
		}
		total.Sessions += day.Sessions
		total.Requests += day.Requests
		total.Input += day.Input
		total.CacheRead += day.CacheRead
		total.CacheCreation += day.CacheCreation
		total.Output += day.Output
		total.Misses += day.Misses
	}
	for _, day := range daily {
		bar := 0
		if max > 0 {
			bar = int(100 * day.CacheCreation / max)
		}
		rows = append(rows, dayRow{Day: day.Day, Totals: day.Totals, Bar: bar})
	}
	// A session spanning several days appears in each of them; count it once.
	total.Sessions = 0
	if projects, err := analytics.ByProject(r.Context(), s.st, f); err == nil {
		for _, p := range projects {
			total.Sessions += p.Sessions
		}
	}
	misses, _ := analytics.Misses(r.Context(), s.st, f)
	var mrows []missRow
	for _, m := range misses {
		mrows = append(mrows, missRow{Reason: m.Reason, Count: m.Count, Hint: analytics.MissHints[m.Reason]})
	}
	s.render(w, r, "cache", "cache", "Cache and tokens", struct {
		Days   int
		Daily  []dayRow
		Total  analytics.Totals
		Misses []missRow
	}{d, rows, total, mrows})
}

// ---- advice ----

type adviceGroup struct {
	findingView
	Projects []string
}

// advice runs the advisor on every project with recent sessions and merges identical findings,
// so a user-level problem shows once with the list of projects it affects.
func (s *Server) advice(w http.ResponseWriter, r *http.Request) {
	s.refresh(r)
	projects, err := analytics.ByProject(r.Context(), s.st, analytics.Filter{Since: time.Now().AddDate(0, 0, -30)})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var groups []*adviceGroup
	byKey := map[string]*adviceGroup{}
	for _, p := range projects {
		in := advise.Input{Home: s.home, ConfigDir: s.configDir, Project: p.Path, Items: claudecode.Instructions(s.home, s.configDir, p.Path)}
		for _, f := range advise.Run(in, nil) {
			key := f.Rule + "\x00" + f.Title + "\x00" + f.Path
			g, ok := byKey[key]
			if !ok {
				g = &adviceGroup{findingView: findingView{Finding: f, Dir: p.ProjectDir, ShowPath: s.shortPath(f.Path)}}
				byKey[key] = g
				groups = append(groups, g)
			}
			g.Projects = append(g.Projects, s.shortPath(p.Path))
		}
	}
	sort.SliceStable(groups, func(i, j int) bool { return groups[i].Saves > groups[j].Saves })
	changes, _ := s.journal.List()
	if len(changes) > 10 {
		changes = changes[:10]
	}
	for i := range changes {
		changes[i].Path = s.shortPath(changes[i].Path)
	}
	s.render(w, r, "advice", "advice", "What to improve", struct {
		Groups  []*adviceGroup
		Changes []fsx.Change
	}{groups, changes})
}
