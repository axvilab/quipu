package ui

import (
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/axvilab/quipu/internal/advise"
	"github.com/axvilab/quipu/internal/analytics"
	"github.com/axvilab/quipu/internal/claudecode"
	"github.com/axvilab/quipu/internal/fsx"
)

// findingView is a finding plus the project it was found in, so the page can post it back.
type findingView struct {
	advise.Finding
	Dir      string
	ShowPath string // Path with the home directory shown as ~
}

// apply re-runs the advisor for the project, finds the finding by ID and writes its edits.
// Running the rules again instead of trusting the form means the edit always matches the files
// as they are now; a finding that no longer exists is reported instead of applied.
func (s *Server) apply(w http.ResponseWriter, r *http.Request) {
	dir, id := r.FormValue("dir"), r.FormValue("id")
	path, ok := s.projectPath(r, dir)
	if !ok {
		s.back(w, r, "Unknown project.")
		return
	}
	in := advise.Input{Home: s.home, ConfigDir: s.configDir, Project: path, Items: claudecode.Instructions(s.home, s.configDir, path)}
	var found *advise.Finding
	for _, f := range advise.Run(in, nil) {
		if f.ID() == id {
			found = &f
			break
		}
	}
	if found == nil || len(found.Edits) == 0 {
		s.back(w, r, "This finding is gone or cannot be applied automatically; the page was refreshed.")
		return
	}

	for _, e := range found.Edits {
		current, err := os.ReadFile(e.Path)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			s.back(w, r, fmt.Sprintf("Could not read %s: %v", s.shortPath(e.Path), err))
			return
		}
		next, err := e.Apply(current)
		if err != nil {
			s.back(w, r, fmt.Sprintf("Not applied: %s changed since the finding was made. Check it and try again.", s.shortPath(e.Path)))
			return
		}
		if _, err := s.journal.Write(e.Path, next, found.Rule, found.Title); err != nil {
			s.back(w, r, fmt.Sprintf("Could not write %s: %v", s.shortPath(e.Path), err))
			return
		}
	}
	s.back(w, r, "Applied: "+found.Title+". A backup was saved; open Claude Code sessions pick it up after /clear or a restart.")
}

func (s *Server) undo(w http.ResponseWriter, r *http.Request) {
	switch err := s.journal.Undo(r.FormValue("id")); {
	case errors.Is(err, fsx.ErrChanged):
		s.back(w, r, "Not undone: the file was edited after quipu changed it, and undoing would lose that edit.")
	case err != nil:
		s.back(w, r, "Could not undo: "+err.Error())
	default:
		s.back(w, r, "Undone: the file is back as it was.")
	}
}

// back redirects to the page the form was on, with a message to show.
func (s *Server) back(w http.ResponseWriter, r *http.Request, msg string) {
	to := "/advice"
	if ref, err := url.Parse(r.Referer()); err == nil && ref.Host == r.Host && ref.Path != "" {
		q := ref.Query()
		q.Del("msg")
		ref.RawQuery = q.Encode()
		to = ref.RequestURI()
	}
	u, _ := url.Parse(to)
	q := u.Query()
	q.Set("msg", msg)
	u.RawQuery = q.Encode()
	http.Redirect(w, r, u.RequestURI(), http.StatusSeeOther)
}

// projectPath finds the working directory of a project that has recent sessions.
func (s *Server) projectPath(r *http.Request, dir string) (string, bool) {
	projects, err := analytics.ByProject(r.Context(), s.st, analytics.Filter{Since: time.Now().AddDate(0, 0, -90), ProjectDir: dir})
	if err != nil || len(projects) == 0 || dir == "" {
		return "", false
	}
	return projects[0].Path, true
}
