package ui

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/axvilab/quipu/internal/advise"
	"github.com/axvilab/quipu/internal/analytics"
	"github.com/axvilab/quipu/internal/claudecode"
	"github.com/axvilab/quipu/internal/fsx"
	"github.com/axvilab/quipu/internal/scaffold"
)

// NewInfo is what the New project page offers.
type NewInfo struct {
	Profiles   []string       `json:"profiles"`
	Parent     string         `json:"parent"` // the folder most projects are in, with ~
	Extensions []NewExtension `json:"extensions"`
	Average    int64          `json:"average"` // tokens the user's projects add to every session, on average
}

// NewExtension is a skill, rule or agent of the user's projects that can be reused.
type NewExtension struct {
	Kind     string   `json:"kind"`
	Name     string   `json:"name"`
	Path     string   `json:"path"` // the most recently changed copy, with ~
	Projects []string `json:"projects"`
	Tokens   int64    `json:"tokens"`   // what it adds to every session
	Variants int      `json:"variants"` // how many different versions the projects have
	// Global: the copies are identical and can move to the user's own settings instead of being
	// copied once more, the fix the advice proposes for copies.
	Global bool `json:"global"`
}

// NewRequest describes the project to set up.
type NewRequest struct {
	Folder     string   `json:"folder"`
	Summary    string   `json:"summary"`
	Extensions []string `json:"extensions"` // NewExtension paths to copy into the project
	Global     []string `json:"global"`     // NewExtension paths to move to the user's settings
	Profile    string   `json:"profile"`
}

// NewFile is one file of the preview, with what it costs Claude: in every session, and when it is
// needed.
type NewFile struct {
	Rel      string `json:"rel"` // inside the project, or the full path (with ~) of a global one
	Content  string `json:"content"`
	From     string `json:"from,omitempty"`
	About    string `json:"about"`
	Always   int64  `json:"always"`
	OnDemand int64  `json:"onDemand"`
	Existing bool   `json:"existing"` // already in the folder, kept as it is
	Global   bool   `json:"global"`   // moved to the user's settings: works in every project
}

// NewInfo lists the profiles and the extensions of known projects.
func (s *Service) NewInfo(ctx context.Context) (NewInfo, error) {
	out := NewInfo{Extensions: []NewExtension{}}
	for _, p := range s.profileInfos(ctx) {
		out.Profiles = append(out.Profiles, p.Name)
	}

	data := s.adviceData(ctx)
	parents := map[string]int{}
	type group struct {
		ext      NewExtension
		newest   time.Time
		sums     map[string]bool
		seen     map[string]bool
		project  string // one project that has it, to ask the advice about moving it
		tokens   map[string]int64
		resolved bool
	}
	groups := map[string]*group{}
	var always int64
	for _, project := range sortedProjectPaths(data.projects) {
		parents[filepath.Dir(project)]++
		for _, it := range data.projects[project] {
			if it.Scope == "project" && it.Load == claudecode.Always {
				always += analytics.EstimateTokens(it.Text)
			}
			if it.Scope != "project" || (it.Kind != "skill" && it.Kind != "rule" && it.Kind != "agent") {
				continue
			}
			key := it.Kind + "\x00" + it.Name
			g := groups[key]
			if g == nil {
				g = &group{ext: NewExtension{Kind: it.Kind, Name: it.Name}, sums: map[string]bool{}, seen: map[string]bool{},
					project: project, tokens: map[string]int64{}}
				groups[key] = g
			}
			if it.Load == claudecode.Always {
				g.tokens[it.Path] += analytics.EstimateTokens(it.Text)
			}
			if g.seen[it.Path] {
				continue
			}
			g.seen[it.Path] = true
			root := it.Path
			if it.Kind == "skill" {
				root = filepath.Dir(it.Path)
			}
			sum, changed := digest(root)
			g.sums[sum] = true
			if changed.After(g.newest) {
				g.newest, g.ext.Path = changed, it.Path
			}
			if name := filepath.Base(project); !contains(g.ext.Projects, name) {
				g.ext.Projects = append(g.ext.Projects, name)
			}
		}
	}
	if len(data.projects) > 0 {
		out.Average = always / int64(len(data.projects))
	}
	for _, g := range groups {
		e := g.ext
		e.Tokens = g.tokens[e.Path]
		e.Variants = len(g.sums)
		e.Global = len(e.Projects) > 1 && e.Variants == 1 && s.globalFix(data, g.project, e.Kind, e.Name) != nil
		e.Path = s.shortPath(e.Path)
		out.Extensions = append(out.Extensions, e)
	}
	sort.Slice(out.Extensions, func(i, j int) bool {
		a, b := out.Extensions[i], out.Extensions[j]
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		return a.Name < b.Name
	})

	out.Parent = s.shortPath(s.home)
	best := 0
	for dir, n := range parents {
		if n > best || (n == best && dir < s.expandHome(out.Parent)) {
			out.Parent, best = s.shortPath(dir), n
		}
	}
	return out, nil
}

// digest is a hash of a skill folder or a rule or agent file, and when it last changed.
func digest(root string) (string, time.Time) {
	h := sha256.New()
	var latest time.Time
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		fmt.Fprintf(h, "%s\x00%d\x00%s", rel, len(b), b)
		if info, err := d.Info(); err == nil && info.ModTime().After(latest) {
			latest = info.ModTime()
		}
		return nil
	})
	return fmt.Sprintf("%x", h.Sum(nil)), latest
}

// globalFix is the advice's fix that moves the identical copies of an extension to the user's
// settings, or nil when there is none (the copies differ, or the user already has one).
func (s *Service) globalFix(data adviceData, project, kind, name string) *advise.Finding {
	for _, f := range advise.Run(s.adviceInput(project, data), nil) {
		if f.Rule == "copied-extension" && f.Params["kind"] == kind && f.Params["name"] == name && len(f.Edits) > 0 {
			return &f
		}
	}
	return nil
}

func sortedProjectPaths(m map[string][]claudecode.Item) []string {
	out := make([]string, 0, len(m))
	for p := range m {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// newOptions turns the request into scaffold options; global extensions are planned as copies too,
// so the preview can count them, and are told apart by their paths.
func (s *Service) newOptions(in NewRequest) (scaffold.Options, map[string]bool, error) {
	dir := s.expandHome(strings.TrimSpace(in.Folder))
	if !filepath.IsAbs(dir) {
		return scaffold.Options{}, nil, problem("folder-not-absolute")
	}
	o := scaffold.Options{Dir: filepath.Clean(dir), Summary: strings.TrimSpace(in.Summary)}
	global := map[string]bool{}
	add := func(p string, isGlobal bool) error {
		kind, file, ok := claudecode.ExtensionAt(s.expandHome(p))
		if !ok {
			return problem("extension-missing", "path", p)
		}
		o.Extensions = append(o.Extensions, scaffold.Extension{Kind: kind, Path: file})
		if isGlobal {
			root := file
			if kind == "skill" {
				root = filepath.Dir(file)
			}
			global[root] = true
		}
		return nil
	}
	for _, p := range in.Extensions {
		if err := add(p, false); err != nil {
			return o, nil, err
		}
	}
	for _, p := range in.Global {
		if err := add(p, true); err != nil {
			return o, nil, err
		}
	}
	return o, global, nil
}

// isGlobal tells whether a planned file comes from an extension that moves to the user's settings.
func isGlobal(f scaffold.File, global map[string]bool) bool {
	for root := range global {
		if f.From == root || strings.HasPrefix(f.From, root+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func (s *Service) planNew(in NewRequest) (scaffold.Options, []scaffold.File, map[string]bool, error) {
	o, global, err := s.newOptions(in)
	if err != nil {
		return o, nil, nil, err
	}
	files, err := scaffold.Plan(o)
	switch {
	case errors.Is(err, scaffold.ErrNotFolder):
		return o, nil, nil, problem("not-a-folder", "folder", s.shortPath(o.Dir))
	case err != nil:
		return o, nil, nil, problem("new-failed", "error", err.Error())
	}
	return o, files, global, nil
}

// PlanNew previews what Claude will get: the new files, the global ones, and the Claude files
// already in the folder.
func (s *Service) PlanNew(in NewRequest) ([]NewFile, error) {
	o, files, global, err := s.planNew(in)
	if err != nil {
		return nil, err
	}
	var fresh []scaffold.File
	for _, f := range files {
		if !f.Exists {
			fresh = append(fresh, f)
		}
	}
	cost, err := loadCost(fresh)
	if err != nil {
		return nil, problem("new-failed", "error", err.Error())
	}
	out := []NewFile{}
	planned := map[string]bool{}
	for _, f := range fresh {
		c := cost[f.Rel]
		nf := NewFile{Rel: f.Rel, Content: f.Content, From: s.shortPath(f.From), About: f.About, Always: c[0], OnDemand: c[1]}
		if isGlobal(f, global) {
			nf.Global = true
			nf.Rel = s.shortPath(s.userPath(f.Rel))
		}
		planned[f.Rel] = true
		out = append(out, nf)
	}
	// What the folder already has that Claude reads, and planned files it already has.
	existing := map[string]bool{}
	for rel, c := range projectCost(o.Dir) {
		if planned[rel] {
			continue
		}
		existing[rel] = true
		b, _ := os.ReadFile(filepath.Join(o.Dir, filepath.FromSlash(rel)))
		out = append(out, NewFile{Rel: rel, Content: clip(string(b)), About: scaffold.AboutOf(rel), Always: c[0], OnDemand: c[1], Existing: true})
	}
	for _, f := range files {
		if f.Exists && !existing[f.Rel] {
			b, _ := os.ReadFile(filepath.Join(o.Dir, filepath.FromSlash(f.Rel)))
			out = append(out, NewFile{Rel: f.Rel, Content: clip(string(b)), About: f.About, Existing: true})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Rel < out[j].Rel })
	return out, nil
}

// userPath is where a project file of a global extension goes in the user's settings.
func (s *Service) userPath(rel string) string {
	parts := strings.SplitN(rel, "/", 3) // ".claude", "<kind>s", rest
	if len(parts) < 3 {
		return rel
	}
	return filepath.Join(claudecode.UserExtensionDir(s.configDir, strings.TrimSuffix(parts[1], "s")), filepath.FromSlash(parts[2]))
}

func clip(text string) string {
	const max = 64 << 10
	if len(text) > max {
		return text[:max] + "\n…"
	}
	return text
}

// loadCost tells, per file, the tokens Claude Code loads in every session and on demand. It lays
// the files out in a scratch folder and reads it the way quipu reads a real project, so the
// preview counts exactly what the project page will show once the project exists.
func loadCost(files []scaffold.File) (map[string][2]int64, error) {
	tmp, err := os.MkdirTemp("", "quipu-new-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	for _, f := range files {
		p := filepath.Join(tmp, filepath.FromSlash(f.Rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(p, []byte(f.Content), 0o644); err != nil {
			return nil, err
		}
	}
	return projectCost(tmp), nil
}

// projectCost reads the Claude files of a folder, without the user's settings, and returns the
// tokens per file: [every session, on demand].
func projectCost(dir string) map[string][2]int64 {
	out := map[string][2]int64{}
	if _, err := os.Stat(dir); err != nil {
		return out
	}
	for _, it := range claudecode.Instructions(dir, filepath.Join(dir, ".quipu-no-config"), dir) {
		rel, err := filepath.Rel(dir, it.Path)
		if err != nil || it.Scope != "project" || strings.HasPrefix(rel, "..") {
			continue
		}
		rel = filepath.ToSlash(rel)
		c := out[rel]
		if it.Load == claudecode.Always {
			c[0] += analytics.EstimateTokens(it.Text)
		} else {
			c[1] += analytics.EstimateTokens(it.Text)
		}
		out[rel] = c
	}
	return out
}

// CreateNew writes the project through the journal, moves the chosen extensions to the user's
// settings in the same batch (so one undo reverts both), and sets the folder's profile.
func (s *Service) CreateNew(ctx context.Context, in NewRequest) (Result, error) {
	o, files, global, err := s.planNew(in)
	if err != nil {
		return Result{}, err
	}
	if in.Profile != "" {
		c, err := s.loadProfiles()
		if err != nil || c.Find(in.Profile) == nil {
			return Result{}, problem("profile-missing", "name", in.Profile)
		}
	}
	var copies []scaffold.File
	for _, f := range files {
		if !isGlobal(f, global) {
			copies = append(copies, f)
		}
	}

	name := filepath.Base(o.Dir)
	batch, err := fsx.NewBatch()
	if err != nil {
		return Result{}, err
	}
	meta := fsx.Meta{Rule: "new-project", Title: "new project " + name, Params: map[string]string{"name": name}, Batch: batch}
	var change string
	if len(global) > 0 {
		data := s.adviceData(ctx)
		for _, e := range o.Extensions {
			root := e.Path
			if e.Kind == "skill" {
				root = filepath.Dir(e.Path)
			}
			if !global[root] {
				continue
			}
			fix := s.globalFix(data, projectOf(root), e.Kind, extensionName(data, e.Path))
			if fix == nil {
				return Result{}, problem("global-gone", "path", s.shortPath(e.Path))
			}
			if change, err = s.applyEdits(fix.Edits, meta); err != nil {
				return Result{}, err
			}
		}
	}
	id, err := scaffold.Apply(o.Dir, copies, s.journal, meta)
	if err != nil {
		return Result{}, problem("new-failed", "error", err.Error())
	}
	if id != "" {
		change = id
	}
	if change == "" && in.Profile == "" {
		return Result{}, problem("nothing-new")
	}
	r := ok("project-created", "name", name, "folder", s.shortPath(o.Dir), "change", change)
	if in.Profile != "" {
		if _, err := s.SetFolderRule(o.Dir, in.Profile); err != nil {
			r.Params["ruleError"] = err.Error()
		}
	}
	return r, nil
}

// projectOf is the project folder an extension lives in: the folder above its .claude.
func projectOf(root string) string {
	for d := root; d != filepath.Dir(d); d = filepath.Dir(d) {
		if filepath.Base(d) == ".claude" {
			return filepath.Dir(d)
		}
	}
	return root
}

// extensionName is the name the instructions give the extension at path.
func extensionName(data adviceData, path string) string {
	for _, items := range data.projects {
		for _, it := range items {
			if it.Path == path {
				return it.Name
			}
		}
	}
	return ""
}
