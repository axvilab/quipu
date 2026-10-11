// Package ui serves quipu's local web interface.
//
// It listens on 127.0.0.1 only. The first URL carries a random token that is exchanged for a
// cookie; every other request needs that cookie and a Host header pointing at our own address,
// so other local users, web pages and DNS-rebinding tricks cannot read the data.
package ui

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"net"
	"net/http"
	"time"

	"github.com/axvilab/quipu/internal/store"
)

//go:embed templates static
var assets embed.FS

const cookieName = "quipu_token"

// Server is the web UI over one store and one Claude Code profile.
type Server struct {
	home      string
	configDir string
	profile   string
	st        *store.Store
	token     string
	addr      string // host:port we listen on, for the Host check
	pages     map[string]*template.Template
}

// New prepares a server; call Listen to start it.
func New(home, configDir, profile string, st *store.Store) (*Server, error) {
	pages, err := parsePages()
	if err != nil {
		return nil, err
	}
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	return &Server{home: home, configDir: configDir, profile: profile, st: st, token: hex.EncodeToString(b), pages: pages}, nil
}

// Listen binds to 127.0.0.1:port (0 picks a free port) and serves until ctx is done.
// It calls ready with the URL to open, which includes the one-time token.
func (s *Server) Listen(ctx context.Context, port int, ready func(url string)) error {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return err
	}
	s.addr = ln.Addr().String()
	srv := &http.Server{Handler: s.Handler(), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		srv.Shutdown(shutdown)
	}()
	ready(fmt.Sprintf("http://%s/?t=%s", s.addr, s.token))
	if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Handler is the full HTTP handler, guarded by the Host and token checks.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	static, _ := fs.Sub(assets, "static")
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(static)))
	mux.HandleFunc("GET /{$}", s.overview)
	mux.HandleFunc("GET /project", s.project)
	mux.HandleFunc("GET /cache", s.cache)
	mux.HandleFunc("GET /advice", s.advice)
	return s.guard(mux)
}

func (s *Server) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.addr != "" && r.Host != s.addr && r.Host != "localhost"+s.addr[len("127.0.0.1"):] {
			http.Error(w, "unexpected Host header", http.StatusForbidden)
			return
		}
		if t := r.URL.Query().Get("t"); t != "" && s.validToken(t) {
			http.SetCookie(w, &http.Cookie{Name: cookieName, Value: s.token, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
			q := r.URL.Query()
			q.Del("t")
			r.URL.RawQuery = q.Encode()
			http.Redirect(w, r, r.URL.String(), http.StatusSeeOther)
			return
		}
		if c, err := r.Cookie(cookieName); err != nil || !s.validToken(c.Value) {
			http.Error(w, "open quipu through the link printed by `quipu ui`", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) validToken(t string) bool {
	return subtle.ConstantTimeCompare([]byte(t), []byte(s.token)) == 1
}
