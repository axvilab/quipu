package ui

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/axvilab/quipu/internal/advise"
	"github.com/axvilab/quipu/internal/analytics"
	"github.com/axvilab/quipu/internal/claudecode"
	"github.com/axvilab/quipu/internal/collect"
	"github.com/axvilab/quipu/internal/fsx"
	"github.com/axvilab/quipu/internal/profile"
)

// The types below are the JSON contract of the API; web/src/lib/api/types.ts mirrors them.
// Texts the user reads are sent as codes and parameters and translated in the frontend.

type Totals struct {
	Sessions   int64   `json:"sessions"`
	Requests   int64   `json:"requests"`
	Prompt     int64   `json:"prompt"`
	CacheRead  int64   `json:"cacheRead"`
	CacheWrite int64   `json:"cacheWrite"`
	Output     int64   `json:"output"`
	Misses     int64   `json:"misses"`
	HitRate    float64 `json:"hitRate"`
}

func totals(t analytics.Totals) Totals {
	return Totals{Sessions: t.Sessions, Requests: t.Requests, Prompt: t.Prompt(), CacheRead: t.CacheRead,
		CacheWrite: t.CacheCreation, Output: t.Output, Misses: t.Misses, HitRate: t.HitRate()}
}

type Start struct {
	Sessions int64  `json:"sessions"`
	Median   int64  `json:"median"`
	Latest   int64  `json:"latest"`
	LatestAt string `json:"latestAt,omitempty"`
}

func start(w analytics.StartWeight) Start {
	s := Start{Sessions: w.Sessions, Median: w.Median, Latest: w.Latest}
	if !w.LatestAt.IsZero() {
		s.LatestAt = w.LatestAt.Format(time.RFC3339)
	}
	return s
}

type DiffLine struct {
	Kind string `json:"kind"` // "add", "del" or "ctx"
	Text string `json:"text"`
}

type Finding struct {
	ID       string            `json:"id"`
	Rule     string            `json:"rule"`
	Params   map[string]string `json:"params"`
	Path     string            `json:"path,omitempty"`
	Diff     []DiffLine        `json:"diff,omitempty"`
	Saves    int64             `json:"saves"`
	CanApply bool              `json:"canApply"`
	Dir      string            `json:"dir"`      // project to post back with Apply
	Projects []string          `json:"projects"` // where it was found
	Scope    string            `json:"scope"`    // "user": your own settings; "project"
}

type ProjectRow struct {
	Dir      string `json:"dir"`
	Path     string `json:"path"`
	Name     string `json:"name"`
	Totals   Totals `json:"totals"`
	Start    Start  `json:"start"`
	Findings int    `json:"findings"`
}

type Overview struct {
	Days      int          `json:"days"`
	Totals    Totals       `json:"totals"`
	Previous  Totals       `json:"previous"`
	Projects  []ProjectRow `json:"projects"`
	Attention []Finding    `json:"attention"`
}

type Item struct {
	Label  string `json:"label"`
	Scope  string `json:"scope"`
	Kind   string `json:"kind"`
	Always bool   `json:"always"`
	Tokens int64  `json:"tokens"`
	Path   string `json:"path"`
}

type Day struct {
	Day    string `json:"day"`
	Totals Totals `json:"totals"`
}

type Project struct {
	Dir           string    `json:"dir"`
	Path          string    `json:"path"`
	Name          string    `json:"name"`
	Totals        Totals    `json:"totals"`
	Previous      Totals    `json:"previous"`
	Start         Start     `json:"start"`
	Items         []Item    `json:"items"`
	AlwaysTotal   int64     `json:"alwaysTotal"`
	OnDemandTotal int64     `json:"onDemandTotal"`
	Findings      []Finding `json:"findings"`
	Daily         []Day     `json:"daily"`
}

type Miss struct {
	Reason       string `json:"reason"`
	Count        int64  `json:"count"`
	MissedTokens int64  `json:"missedTokens"`
}

type CacheReport struct {
	Days     int    `json:"days"`
	Totals   Totals `json:"totals"`
	Previous Totals `json:"previous"`
	Daily    []Day  `json:"daily"`
	Misses   []Miss `json:"misses"`
}

type Change struct {
	ID     string `json:"id"`
	Path   string `json:"path"`
	Rule   string `json:"rule"`
	Title  string `json:"title"`
	Time   string `json:"time"`
	Undone bool   `json:"undone"`
}

type Advice struct {
	Findings []Finding `json:"findings"`
	Changes  []Change  `json:"changes"`
}

type SignIn struct {
	State string `json:"state"` // "", "starting" or "open"
	URL   string `json:"url,omitempty"`
}

type ProfileInfo struct {
	Name     string   `json:"name"`
	Account  string   `json:"account"`
	Plan     string   `json:"plan"`
	SignedIn bool     `json:"signedIn"`
	Current  bool     `json:"current"`
	Dir      string   `json:"dir"`
	Shares   []string `json:"shares"`
	SignIn   SignIn   `json:"signIn"`
}

type Rule struct {
	Path    string `json:"path"`
	Profile string `json:"profile"`
}

type Browser struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Default bool   `json:"default"`
}

type Profiles struct {
	Profiles  []ProfileInfo `json:"profiles"`
	Rules     []Rule        `json:"rules"`
	Folders   []string      `json:"folders"`
	Shareable []string      `json:"shareable"`
	Browsers  []Browser     `json:"browsers"`
}

type Session struct {
	Version  string        `json:"version"`
	Profile  string        `json:"profile"`
	Profiles []ProfileInfo `json:"profiles"`
}

// Result answers an action: a code the frontend translates, with parameters.
type Result struct {
	OK     bool              `json:"ok"`
	Code   string            `json:"code"`
	Params map[string]string `json:"params,omitempty"`
}

// Problem is an error the user can act on; Code is translated in the frontend.
type Problem struct {
	Code   string
	Params map[string]string
}

func (p *Problem) Error() string { return p.Code }

func problem(code string, kv ...string) *Problem {
	p := &Problem{Code: code, Params: map[string]string{}}
	for i := 0; i+1 < len(kv); i += 2 {
		p.Params[kv[i]] = kv[i+1]
	}
	return p
}

func ok(code string, kv ...string) Result {
	r := Result{OK: true, Code: code, Params: map[string]string{}}
	for i := 0; i+1 < len(kv); i += 2 {
		r.Params[kv[i]] = kv[i+1]
	}
	return r
}

// ---- reads ----

func (s *Service) collect(ctx context.Context) {
	collect.Run(ctx, s.home, s.st)
}

func window(days int) analytics.Filter {
	if days <= 0 || days > 365 {
		days = 30
	}
	return analytics.Filter{Since: time.Now().AddDate(0, 0, -days)}
}

// SessionInfo tells the frontend who it serves.
func (s *Service) SessionInfo(ctx context.Context) Session {
	return Session{Version: s.version, Profile: s.profile, Profiles: s.profileInfos(ctx)}
}

// Overview is the home screen: totals against the previous period, projects and what to fix.
func (s *Service) Overview(ctx context.Context, days int) (Overview, error) {
	s.collect(ctx)
	f := window(days)
	out := Overview{Days: days, Projects: []ProjectRow{}, Attention: []Finding{}}
	t, err := analytics.Total(ctx, s.st, f)
	if err != nil {
		return out, err
	}
	prev, _ := analytics.Total(ctx, s.st, f.Previous())
	out.Totals, out.Previous = totals(t), totals(prev)

	rows, err := analytics.ByProject(ctx, s.st, f)
	if err != nil {
		return out, err
	}
	perProject := map[string]int{}
	out.Attention = s.findingsAcross(rows, perProject)
	for _, p := range rows {
		pf := f
		pf.ProjectDir = p.ProjectDir
		w, _ := analytics.SessionStart(ctx, s.st, pf)
		out.Projects = append(out.Projects, ProjectRow{Dir: p.ProjectDir, Path: s.shortPath(p.Path),
			Name: filepath.Base(p.Path), Totals: totals(p.Totals), Start: start(w), Findings: perProject[p.ProjectDir]})
	}
	return out, nil
}

// ProjectDetail is one project: what is sent at session start, findings and daily activity.
func (s *Service) ProjectDetail(ctx context.Context, dir string, days int) (Project, error) {
	s.collect(ctx)
	f := window(days)
	f.ProjectDir = dir
	rows, err := analytics.ByProject(ctx, s.st, analytics.Filter{Since: time.Now().AddDate(0, 0, -365), ProjectDir: dir})
	if err != nil {
		return Project{}, err
	}
	if len(rows) == 0 {
		return Project{}, problem("project-unknown")
	}
	path := rows[0].Path
	out := Project{Dir: dir, Path: s.shortPath(path), Name: filepath.Base(path), Items: []Item{}, Findings: []Finding{}, Daily: []Day{}}
	t, _ := analytics.Total(ctx, s.st, f)
	prev, _ := analytics.Total(ctx, s.st, f.Previous())
	out.Totals, out.Previous = totals(t), totals(prev)
	w, _ := analytics.SessionStart(ctx, s.st, f)
	out.Start = start(w)

	items := claudecode.Instructions(s.home, s.configDir, path)
	for _, it := range items {
		n := analytics.EstimateTokens(it.Text)
		always := it.Load == claudecode.Always
		out.Items = append(out.Items, Item{Label: s.itemLabel(it, path), Scope: it.Scope, Kind: it.Kind, Always: always, Tokens: n, Path: s.shortPath(it.Path)})
		if always {
			out.AlwaysTotal += n
		} else {
			out.OnDemandTotal += n
		}
	}
	sort.SliceStable(out.Items, func(i, j int) bool { return out.Items[i].Tokens > out.Items[j].Tokens })

	for _, fd := range advise.Run(advise.Input{Home: s.home, ConfigDir: s.configDir, Project: path, Items: items}, nil) {
		out.Findings = append(out.Findings, s.finding(fd, dir, []string{s.shortPath(path)}))
	}
	days30, _ := analytics.ByDay(ctx, s.st, f)
	for _, d := range days30 {
		out.Daily = append(out.Daily, Day{Day: d.Day.Format(time.DateOnly), Totals: totals(d.Totals)})
	}
	return out, nil
}

// Cache is cache use per day with the reasons for misses.
func (s *Service) Cache(ctx context.Context, days int) (CacheReport, error) {
	s.collect(ctx)
	f := window(days)
	out := CacheReport{Days: days, Daily: []Day{}, Misses: []Miss{}}
	t, err := analytics.Total(ctx, s.st, f)
	if err != nil {
		return out, err
	}
	prev, _ := analytics.Total(ctx, s.st, f.Previous())
	out.Totals, out.Previous = totals(t), totals(prev)
	daily, err := analytics.ByDay(ctx, s.st, f)
	if err != nil {
		return out, err
	}
	for _, d := range daily {
		out.Daily = append(out.Daily, Day{Day: d.Day.Format(time.DateOnly), Totals: totals(d.Totals)})
	}
	misses, _ := analytics.Misses(ctx, s.st, f)
	for _, m := range misses {
		out.Misses = append(out.Misses, Miss{Reason: m.Reason, Count: m.Count, MissedTokens: m.MissedTokens})
	}
	return out, nil
}

// AdviceReport is every finding across projects plus recent changes made by quipu.
func (s *Service) AdviceReport(ctx context.Context) (Advice, error) {
	s.collect(ctx)
	rows, err := analytics.ByProject(ctx, s.st, window(30))
	if err != nil {
		return Advice{}, err
	}
	out := Advice{Findings: s.findingsAcross(rows, map[string]int{}), Changes: []Change{}}
	changes, _ := s.journal.List()
	for i, c := range changes {
		if i == 20 {
			break
		}
		out.Changes = append(out.Changes, Change{ID: c.ID, Path: s.shortPath(c.Path), Rule: c.Rule, Title: c.Title, Time: c.Time.Format(time.RFC3339), Undone: c.Undone})
	}
	return out, nil
}

// findingsAcross runs the advisor on each project and merges identical findings, so a problem in
// the user's own settings shows once with every project it affects. perProject receives counts.
func (s *Service) findingsAcross(rows []analytics.ProjectRow, perProject map[string]int) []Finding {
	out := []Finding{}
	index := map[string]int{}
	for _, p := range rows {
		in := advise.Input{Home: s.home, ConfigDir: s.configDir, Project: p.Path, Items: claudecode.Instructions(s.home, s.configDir, p.Path)}
		for _, fd := range advise.Run(in, nil) {
			perProject[p.ProjectDir]++
			key := fd.Rule + "\x00" + fd.Title + "\x00" + fd.Path
			if i, seen := index[key]; seen {
				out[i].Projects = append(out[i].Projects, s.shortPath(p.Path))
				continue
			}
			index[key] = len(out)
			out = append(out, s.finding(fd, p.ProjectDir, []string{s.shortPath(p.Path)}))
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Saves > out[j].Saves })
	return out
}

func (s *Service) finding(f advise.Finding, dir string, projects []string) Finding {
	out := Finding{ID: f.ID(), Rule: f.Rule, Params: f.Params, Path: s.shortPath(f.Path), Saves: f.Saves,
		CanApply: len(f.Edits) > 0, Dir: dir, Projects: projects, Scope: "project"}
	if out.Params == nil {
		out.Params = map[string]string{}
	}
	// Files under the config dir are the user's own settings, except per-project data in projects/.
	sep := string(filepath.Separator)
	if f.Path != "" && strings.HasPrefix(f.Path, s.configDir+sep) && !strings.HasPrefix(f.Path, filepath.Join(s.configDir, "projects")+sep) {
		out.Scope = "user"
	}
	for _, line := range strings.Split(f.Diff, "\n") {
		if line == "" {
			continue
		}
		kind := "ctx"
		switch {
		case strings.HasPrefix(line, "+"):
			kind = "add"
		case strings.HasPrefix(line, "-"):
			kind = "del"
		}
		out.Diff = append(out.Diff, DiffLine{Kind: kind, Text: line})
	}
	return out
}

// ---- actions ----

// ApplyFinding re-runs the rules for the project and applies the finding with that ID if it
// still exists, so the edit always matches the files as they are now.
func (s *Service) ApplyFinding(ctx context.Context, dir, id string) (Result, error) {
	path, found := s.projectPathFor(ctx, dir)
	if !found {
		return Result{}, problem("project-unknown")
	}
	in := advise.Input{Home: s.home, ConfigDir: s.configDir, Project: path, Items: claudecode.Instructions(s.home, s.configDir, path)}
	var target *advise.Finding
	for _, f := range advise.Run(in, nil) {
		if f.ID() == id {
			target = &f
			break
		}
	}
	if target == nil || len(target.Edits) == 0 {
		return Result{}, problem("finding-gone")
	}
	for _, e := range target.Edits {
		current, err := os.ReadFile(e.Path)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return Result{}, problem("read-failed", "path", s.shortPath(e.Path), "error", err.Error())
		}
		next, err := e.Apply(current)
		if err != nil {
			return Result{}, problem("file-changed", "path", s.shortPath(e.Path))
		}
		if _, err := s.journal.Write(e.Path, next, target.Rule, target.Title); err != nil {
			return Result{}, problem("write-failed", "path", s.shortPath(e.Path), "error", err.Error())
		}
	}
	return ok("applied", "rule", target.Rule), nil
}

// UndoChange restores a file changed by quipu, unless it was edited afterwards.
func (s *Service) UndoChange(id string) (Result, error) {
	switch err := s.journal.Undo(id); {
	case errors.Is(err, fsx.ErrChanged):
		return Result{}, problem("undo-changed")
	case err != nil:
		return Result{}, problem("undo-failed", "error", err.Error())
	}
	return ok("undone"), nil
}

func (s *Service) projectPathFor(ctx context.Context, dir string) (string, bool) {
	if dir == "" {
		return "", false
	}
	rows, err := analytics.ByProject(ctx, s.st, analytics.Filter{Since: time.Now().AddDate(0, 0, -365), ProjectDir: dir})
	if err != nil || len(rows) == 0 {
		return "", false
	}
	return rows[0].Path, true
}

// ---- profiles ----

func (s *Service) profileInfos(ctx context.Context) []ProfileInfo {
	views := map[string]profileView{}
	for _, v := range s.profiles(ctx) {
		views[v.Name] = v
	}
	out := []ProfileInfo{}
	c, err := s.loadProfiles()
	if err != nil {
		for _, v := range views {
			out = append(out, ProfileInfo{Name: v.Name, Current: v.Current, Shares: []string{}})
		}
		return out
	}
	for _, p := range c.Profiles {
		v := views[p.Name]
		info := ProfileInfo{Name: p.Name, Account: v.Email, Plan: v.Plan, SignedIn: v.LoggedIn, Current: p.Name == s.profile,
			Dir: s.shortPath(p.Dir), Shares: []string{}}
		for _, link := range p.Links {
			info.Shares = append(info.Shares, filepath.Base(link))
		}
		if l := s.logins.get(p.Name); l != nil && !l.done {
			info.SignIn.State = "starting"
			if b, err := os.ReadFile(l.urlFile); err == nil && len(b) > 0 {
				info.SignIn = SignIn{State: "open", URL: string(b)}
			}
		}
		out = append(out, info)
	}
	return out
}

// ProfilesReport lists profiles, folder rules and what the forms need.
func (s *Service) ProfilesReport(ctx context.Context) (Profiles, error) {
	out := Profiles{Profiles: s.profileInfos(ctx), Rules: []Rule{}, Folders: []string{}, Shareable: profile.Shareable, Browsers: []Browser{}}
	if c, err := s.loadProfiles(); err == nil {
		for _, r := range c.Rules {
			out.Rules = append(out.Rules, Rule{Path: s.shortPath(r.Path), Profile: r.Profile})
		}
	}
	if rows, err := analytics.ByProject(ctx, s.st, window(90)); err == nil {
		for _, p := range rows {
			out.Folders = append(out.Folders, s.shortPath(p.Path))
		}
		sort.Strings(out.Folders)
	}
	for _, b := range installedBrowsers() {
		out.Browsers = append(out.Browsers, Browser{ID: b.ID, Name: b.Name, Default: b.Default})
	}
	return out, nil
}

func (s *Service) AddProfile(name string, share []string) (Result, error) {
	c, err := s.loadProfiles()
	if err != nil {
		return Result{}, problem("profiles-unavailable")
	}
	p, err := c.Add(strings.TrimSpace(name), share)
	if err == nil {
		err = c.Save()
	}
	if err != nil {
		return Result{}, problem("profile-add-failed", "error", err.Error())
	}
	s.invalidateProfiles()
	return ok("profile-added", "name", p.Name, "dir", s.shortPath(p.Dir)), nil
}

func (s *Service) RemoveProfile(name string) (Result, error) {
	c, err := s.loadProfiles()
	if err != nil {
		return Result{}, problem("profiles-unavailable")
	}
	dir, err := c.Remove(name)
	if err == nil {
		err = c.Save()
	}
	if err != nil {
		return Result{}, problem("profile-remove-failed", "error", err.Error())
	}
	s.invalidateProfiles()
	return ok("profile-removed", "name", name, "dir", s.shortPath(dir)), nil
}

func (s *Service) SetFolderRule(folder, name string) (Result, error) {
	c, err := s.loadProfiles()
	if err != nil {
		return Result{}, problem("profiles-unavailable")
	}
	folder = s.expandHome(strings.TrimSpace(folder))
	if !filepath.IsAbs(folder) {
		return Result{}, problem("folder-not-absolute")
	}
	err = c.SetRule(folder, name)
	if err == nil {
		err = c.Save()
	}
	if err != nil {
		return Result{}, problem("rule-failed", "error", err.Error())
	}
	return ok("rule-saved", "folder", s.shortPath(folder), "name", name), nil
}

func (s *Service) DeleteFolderRule(folder string) (Result, error) {
	c, err := s.loadProfiles()
	if err != nil {
		return Result{}, problem("profiles-unavailable")
	}
	if c.DeleteRule(s.expandHome(folder)) {
		if err := c.Save(); err != nil {
			return Result{}, problem("rule-failed", "error", err.Error())
		}
	}
	return ok("rule-deleted"), nil
}

// SignInProfile starts `claude auth login` for the profile and, when it can, opens the sign-in
// page in a private window of the chosen (or default) browser.
func (s *Service) SignInProfile(name, email, browserID string, private bool) (Result, error) {
	c, err := s.loadProfiles()
	if err != nil {
		return Result{}, problem("profiles-unavailable")
	}
	p := c.Find(name)
	if p == nil {
		return Result{}, problem("profile-unknown")
	}
	email = strings.TrimSpace(email)
	if email != "" && (!strings.Contains(email, "@") || strings.ContainsAny(email, " \t\n")) {
		return Result{}, problem("email-invalid")
	}
	if err := s.startLogin(p, email); err != nil {
		return Result{}, problem("sign-in-failed", "error", err.Error())
	}
	if !private {
		return ok("sign-in-started", "name", name), nil
	}
	if u := s.waitLoginURL(name, 5*time.Second); u != "" && openPrivateWith(browserID, u) {
		return ok("sign-in-private", "name", name), nil
	}
	return ok("sign-in-started", "name", name), nil
}

// OpenSignInPrivately opens a pending sign-in page in a private window again.
func (s *Service) OpenSignInPrivately(name, browserID string) (Result, error) {
	if u := s.waitLoginURL(name, 0); u != "" && openPrivateWith(browserID, u) {
		return ok("sign-in-private", "name", name), nil
	}
	return Result{}, problem("private-window-failed")
}

func (s *Service) expandHome(p string) string {
	if rest, found := strings.CutPrefix(p, "~/"); found {
		return filepath.Join(s.home, rest)
	}
	if p == "~" {
		return s.home
	}
	return p
}

// shortPath shows a path with the home directory as ~.
func (s *Service) shortPath(p string) string {
	if rest, found := strings.CutPrefix(p, s.home); found && p != "" {
		return "~" + rest
	}
	return p
}

// itemLabel names an instruction item: skills, agents, commands and memory notes by name, files by
// their path inside the project when they are in it.
func (s *Service) itemLabel(it claudecode.Item, project string) string {
	switch it.Kind {
	case "skill", "agent", "command", "memory":
		return it.Name
	}
	if rel, err := filepath.Rel(project, it.Path); err == nil && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return s.shortPath(it.Path)
}
