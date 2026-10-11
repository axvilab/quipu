package ui

import (
	"encoding/json"
	"errors"
	"io"
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
	mux.HandleFunc("GET /api/file", func(w http.ResponseWriter, r *http.Request) {
		v, err := s.ReadFile(r.Context(), r.URL.Query().Get("path"))
		writeJSON(w, v, err)
	})
	mux.HandleFunc("POST /api/file", func(w http.ResponseWriter, r *http.Request) {
		var in struct{ Path, Content, Version string }
		if readJSON(w, r, &in) {
			v, err := s.SaveFile(r.Context(), in.Path, in.Content, in.Version)
			writeJSON(w, v, err)
		}
	})
	mux.HandleFunc("POST /api/tokens", func(w http.ResponseWriter, r *http.Request) {
		var in struct{ Text string }
		if readJSON(w, r, &in) {
			writeJSON(w, map[string]int64{"tokens": s.Tokens(in.Text)}, nil)
		}
	})
	mux.HandleFunc("GET /api/live", func(w http.ResponseWriter, r *http.Request) {
		v, err := s.LiveSessions(r.Context())
		writeJSON(w, v, err)
	})
	mux.HandleFunc("POST /api/live/watch", func(w http.ResponseWriter, r *http.Request) {
		var in struct{ On bool }
		if readJSON(w, r, &in) {
			v, err := s.WatchRemoteControl(in.On)
			writeJSON(w, v, err)
		}
	})
	mux.HandleFunc("GET /api/backup", func(w http.ResponseWriter, r *http.Request) {
		v, err := s.BackupInfo()
		writeJSON(w, v, err)
	})
	mux.HandleFunc("POST /api/backup", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			GitDir             string
			Push, AllowSecrets bool
		}
		if readJSON(w, r, &in) {
			v, err := s.CreateBackup(r.Context(), in.GitDir, in.Push, in.AllowSecrets)
			writeJSON(w, v, err)
		}
	})
	mux.HandleFunc("POST /api/restore/plan", func(w http.ResponseWriter, r *http.Request) {
		var in struct{ Source string }
		if readJSON(w, r, &in) {
			v, err := s.PlanRestore(in.Source)
			writeJSON(w, v, err)
		}
	})
	mux.HandleFunc("POST /api/restore", func(w http.ResponseWriter, r *http.Request) {
		var in struct{ Source string }
		if readJSON(w, r, &in) {
			v, err := s.ApplyRestore(in.Source)
			writeJSON(w, v, err)
		}
	})
	mux.HandleFunc("GET /api/doctor", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"checks": s.Doctor(r.Context()), "shim": s.Shim()}, nil)
	})
	mux.HandleFunc("POST /api/shim", func(w http.ResponseWriter, r *http.Request) {
		var in struct{ On bool }
		if readJSON(w, r, &in) {
			v, err := s.SetShim(in.On)
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
		case "file-not-editable":
			status = http.StatusForbidden
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

// notBuilt answers when the binary was built without the frontend (go build without make web).
func notBuilt(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	io.WriteString(w, `<!doctype html><meta charset="utf-8"><title>Quipu</title>
<body style="font:15px system-ui;background:#171a23;color:#e8e4d8;padding:40px">
<p>The web interface was not built into this binary. Build it with <code>make build</code>
(or <code>make web</code> before <code>go build</code>); the API under /api/ works.</p>`)
}
