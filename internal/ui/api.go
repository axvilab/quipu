package ui

import (
	"encoding/json"
	"errors"
	"io/fs"
	"mime"
	"net/http"
	"strconv"
)

// apiRoutes registers the JSON API: thin wrappers around Service methods.
func (s *Server) apiRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/session", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, s.SessionInfo(r.Context()), nil)
	})
	mux.HandleFunc("GET /api/overview", func(w http.ResponseWriter, r *http.Request) {
		v, err := s.Overview(r.Context(), daysParam(r))
		writeJSON(w, v, err)
	})
	mux.HandleFunc("GET /api/projects/{dir}", func(w http.ResponseWriter, r *http.Request) {
		v, err := s.ProjectDetail(r.Context(), r.PathValue("dir"), daysParam(r))
		writeJSON(w, v, err)
	})
	mux.HandleFunc("GET /api/cache", func(w http.ResponseWriter, r *http.Request) {
		v, err := s.Cache(r.Context(), daysParam(r))
		writeJSON(w, v, err)
	})
	mux.HandleFunc("GET /api/advice", func(w http.ResponseWriter, r *http.Request) {
		v, err := s.AdviceReport(r.Context())
		writeJSON(w, v, err)
	})
	mux.HandleFunc("POST /api/advice/apply", func(w http.ResponseWriter, r *http.Request) {
		var in struct{ Dir, ID string }
		if readJSON(w, r, &in) {
			v, err := s.ApplyFinding(r.Context(), in.Dir, in.ID)
			writeJSON(w, v, err)
		}
	})
	mux.HandleFunc("POST /api/advice/undo", func(w http.ResponseWriter, r *http.Request) {
		var in struct{ ID string }
		if readJSON(w, r, &in) {
			v, err := s.UndoChange(in.ID)
			writeJSON(w, v, err)
		}
	})
	mux.HandleFunc("GET /api/profiles", func(w http.ResponseWriter, r *http.Request) {
		v, err := s.ProfilesReport(r.Context())
		writeJSON(w, v, err)
	})
	mux.HandleFunc("POST /api/profiles", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Name  string
			Share []string
		}
		if readJSON(w, r, &in) {
			v, err := s.AddProfile(in.Name, in.Share)
			writeJSON(w, v, err)
		}
	})
	mux.HandleFunc("POST /api/profiles/{name}/remove", func(w http.ResponseWriter, r *http.Request) {
		if readJSON(w, r, &struct{}{}) {
			v, err := s.RemoveProfile(r.PathValue("name"))
			writeJSON(w, v, err)
		}
	})
	mux.HandleFunc("POST /api/profiles/{name}/sign-in", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Email, Browser string
			Private        bool
		}
		if readJSON(w, r, &in) {
			v, err := s.SignInProfile(r.PathValue("name"), in.Email, in.Browser, in.Private)
			writeJSON(w, v, err)
		}
	})
	mux.HandleFunc("POST /api/profiles/{name}/private", func(w http.ResponseWriter, r *http.Request) {
		var in struct{ Browser string }
		if readJSON(w, r, &in) {
			v, err := s.OpenSignInPrivately(r.PathValue("name"), in.Browser)
			writeJSON(w, v, err)
		}
	})
	mux.HandleFunc("POST /api/rules", func(w http.ResponseWriter, r *http.Request) {
		var in struct{ Folder, Profile string }
		if readJSON(w, r, &in) {
			v, err := s.SetFolderRule(in.Folder, in.Profile)
			writeJSON(w, v, err)
		}
	})
	mux.HandleFunc("POST /api/rules/delete", func(w http.ResponseWriter, r *http.Request) {
		var in struct{ Folder string }
		if readJSON(w, r, &in) {
			v, err := s.DeleteFolderRule(in.Folder)
			writeJSON(w, v, err)
		}
	})
}

func daysParam(r *http.Request) int {
	d, _ := strconv.Atoi(r.URL.Query().Get("days"))
	if d <= 0 || d > 365 {
		return 30
	}
	return d
}

// readJSON decodes a JSON body. Requiring the JSON content type means a plain HTML form on another
// site cannot reach these endpoints even if every other check failed.
func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if ct, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); ct != "application/json" {
		writeJSON(w, nil, &Problem{Code: "json-required"})
		return false
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(v); err != nil {
		writeJSON(w, nil, &Problem{Code: "bad-request", Params: map[string]string{"error": err.Error()}})
		return false
	}
	return true
}

// writeJSON sends v, or the error as {ok: false, code, params}.
func writeJSON(w http.ResponseWriter, v any, err error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if err != nil {
		var p *Problem
		status := http.StatusBadRequest
		if !errors.As(err, &p) {
			p, status = &Problem{Code: "internal", Params: map[string]string{"error": err.Error()}}, http.StatusInternalServerError
		}
		switch p.Code {
		case "project-unknown", "profile-unknown":
			status = http.StatusNotFound
		case "json-required":
			status = http.StatusUnsupportedMediaType
		case "unauthorized":
			status = http.StatusUnauthorized
		}
		w.WriteHeader(status)
		v = Result{OK: false, Code: p.Code, Params: p.Params}
	}
	json.NewEncoder(w).Encode(v)
}

// app returns the built Svelte app (internal/ui/dist/app), or nil when the frontend was not built.
func app() fs.FS {
	sub, err := fs.Sub(assets, "dist/app")
	if err != nil {
		return nil
	}
	if _, err := fs.Stat(sub, "index.html"); err != nil {
		return nil
	}
	return sub
}

// appHandler serves the app's files and falls back to index.html for client-side routes.
func appHandler(root fs.FS) http.Handler {
	files := http.FileServerFS(root)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			if _, err := fs.Stat(root, r.URL.Path[1:]); err == nil {
				files.ServeHTTP(w, r)
				return
			}
		}
		w.Header().Set("Cache-Control", "no-store")
		http.ServeFileFS(w, r, root, "index.html")
	})
}
