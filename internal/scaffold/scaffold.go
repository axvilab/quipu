// Package scaffold sets up a folder for Claude Code without imposing a way of working: a CLAUDE.md
// with the project's name and purpose, and the skills, rules and agents the user picks from their
// other projects. The folder may be new or already have files, such as a fresh clone; files that
// are already there are never touched.
package scaffold

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/axvilab/quipu/internal/claudecode"
	"github.com/axvilab/quipu/internal/fsx"
)

// Extension is a skill, rule or agent of another project to copy into the new one.
type Extension struct {
	Kind string `json:"kind"` // "skill", "rule" or "agent"
	Path string `json:"path"` // the skill's SKILL.md, or the rule or agent file
}

// Options describe the project to set up.
type Options struct {
	Dir        string // the project's folder; created if missing
	Name       string // defaults to the folder's name
	Summary    string // one or two sentences for CLAUDE.md; a placeholder if empty
	Extensions []Extension
}

// File is one file of the plan, with its path inside the project.
type File struct {
	Rel     string `json:"rel"`
	Content string `json:"content"`
	From    string `json:"from,omitempty"` // copied from another project
	About   string `json:"about"`          // what the file is for, see AboutOf
	Exists  bool   `json:"exists"`         // already in the folder: kept as it is, not written
}

// AboutOf names what a file in a project is for, so a preview can explain it: "claude-md", "rule",
// "skill", "agent" or "".
func AboutOf(rel string) string {
	switch {
	case rel == "CLAUDE.md" || rel == ".claude/CLAUDE.md":
		return "claude-md"
	case strings.HasPrefix(rel, ".claude/rules/"):
		return "rule"
	case strings.HasPrefix(rel, ".claude/skills/"):
		return "skill"
	case strings.HasPrefix(rel, ".claude/agents/"):
		return "agent"
	}
	return ""
}

// ErrNotFolder means the target exists but is a file.
var ErrNotFolder = errors.New("not a folder")

// Plan returns the files to set up, sorted by path, without writing anything. Files already in
// the folder are marked Exists.
func Plan(o Options) ([]File, error) {
	if info, err := os.Stat(o.Dir); err == nil && !info.IsDir() {
		return nil, ErrNotFolder
	}
	if o.Name == "" {
		o.Name = filepath.Base(o.Dir)
	}
	summary := o.Summary
	if summary == "" {
		summary = "What this project is for, in one or two sentences."
	}

	files := map[string]File{
		"CLAUDE.md": {Rel: "CLAUDE.md", Content: "# " + o.Name + "\n\n" + summary + "\n", About: "claude-md"},
	}
	for _, e := range o.Extensions {
		copied, err := extensionFiles(e)
		if err != nil {
			return nil, err
		}
		for _, f := range copied {
			files[f.Rel] = f
		}
	}

	out := make([]File, 0, len(files))
	for _, f := range files {
		if _, err := os.Lstat(filepath.Join(o.Dir, filepath.FromSlash(f.Rel))); err == nil {
			f.Exists = true
		}
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Rel < out[j].Rel })
	return out, nil
}

// extensionFiles reads a skill's whole folder, or a rule or agent file, and places it where the
// project keeps that kind.
func extensionFiles(e Extension) ([]File, error) {
	dest := filepath.ToSlash(claudecode.ProjectExtensionDir("", e.Kind))
	switch e.Kind {
	case "rule", "agent":
		b, err := os.ReadFile(e.Path)
		if err != nil {
			return nil, err
		}
		return []File{{Rel: path.Join(dest, filepath.Base(e.Path)), Content: string(b), From: e.Path, About: e.Kind}}, nil
	case "skill":
		root := filepath.Dir(e.Path)
		var out []File
		size := 0
		err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			if size += len(b); size > 1<<20 {
				return fmt.Errorf("skill %s is bigger than 1 MB", filepath.Base(root))
			}
			rel, _ := filepath.Rel(root, p)
			out = append(out, File{Rel: path.Join(dest, filepath.Base(root), filepath.ToSlash(rel)), Content: string(b), From: p, About: "skill"})
			return nil
		})
		return out, err
	}
	return nil, fmt.Errorf("cannot copy a %s", e.Kind)
}

// Apply writes the files that are not in dir yet through the journal, all with m (give it a Batch
// so they are undone at once), and returns the id of one of the changes, "" if nothing was new.
func Apply(dir string, files []File, j *fsx.Journal, m fsx.Meta) (string, error) {
	id := ""
	for _, f := range files {
		if f.Exists {
			continue
		}
		c, err := j.Write(filepath.Join(dir, filepath.FromSlash(f.Rel)), []byte(f.Content), m)
		if err != nil {
			return id, err
		}
		if id == "" {
			id = c.ID
		}
	}
	return id, nil
}
