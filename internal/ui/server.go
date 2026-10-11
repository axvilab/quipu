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
	"path/filepath"
	"strings"
	"time"

	"github.com/axvilab/quipu/internal/fsx"
	"github.com/axvilab/quipu/internal/store"
)

//go:embed templates static
var assets embed.FS

const (
	cookieName  = "quipu_token"
	healthReply = "quipu"
)

// Server is the web UI over one store and one Claude Code profile.
type Server struct {
	home      string
	configDir string
	profile   string
	st        *store.Store
	journal   *fsx.Journal
	token     string
	addr      string // host:port we listen on, for the Host check
	pages     map[string]*template.Template

	// Optional: where profiles are configured and the real claude binary, to show every
	// profile with its login state in the sidebar.
	ProfilesFile string
	ClaudeBinary string
	profileCache profileCache
}

// New prepares a server; call Listen to start it. Backups of files changed through the UI go
// to <dataDir>/backups.
func New(home, configDir, profile, dataDir string, st *store.Store) (*Server, error) {
	pages, err := parsePages()
	if err != nil {
		return nil, err
	}
	journal, err := fsx.OpenJournal(filepath.Join(dataDir, "backups"))
	if err != nil {
		return nil, err
	}
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	return &Server{home: home, configDir: configDir, profile: profile, st: st, journal: journal, token: hex.EncodeToString(b), pages: pages}, nil
}

// Listen binds to 127.0.0.1:port and serves until ctx is done. If the port is taken it falls
// back to a free one. It calls ready with the URL to open, which includes the token.
func (s *Server) Listen(ctx context.Context, port int, ready func(url string)) error {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil && port != 0 {
		ln, err = net.Listen("tcp", "127.0.0.1:0")
	}
	if err != nil {
		return err
	}
	s.addr = ln.Addr().String()
	srv := &http.Server{Handler: s.Handler(), ReadHeaderTimeout: 10 * time.Second}
	// quipu.localhost may resolve to ::1 first, so also listen on IPv6 loopback, same port.
	if ln6, err := net.Listen("tcp", "[::1]"+s.addr[strings.LastIndex(s.addr, ":"):]); err == nil {
		go srv.Serve(ln6)
	}
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
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, healthReply) })
	mux.HandleFunc("GET /{$}", s.overview)
	mux.HandleFunc("GET /project", s.project)
	mux.HandleFunc("GET /cache", s.cache)
	mux.HandleFunc("GET /advice", s.advice)
	mux.HandleFunc("POST /apply", s.apply)
	mux.HandleFunc("POST /undo", s.undo)
	return s.guard(mux)
}

func (s *Server) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.addr != "" && !s.ourHost(r.Host) {
			http.Error(w, "unexpected Host header", http.StatusForbidden)
			return
		}
		// The health check and static files (styles, icon, app manifest) hold no data. Browsers
		// fetch the manifest without cookies, so these must not need the token.
		if r.Method == http.MethodGet && (r.URL.Path == "/healthz" || strings.HasPrefix(r.URL.Path, "/static/")) {
			next.ServeHTTP(w, r)
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
		// Changes must come from our own pages: the SameSite cookie already blocks other sites,
		// and the Origin check also covers browsers that do not enforce it.
		if r.Method != http.MethodGet && !s.sameOrigin(r) {
			http.Error(w, "cross-origin request refused", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Hostname is the friendly name of the UI. Browsers and systemd-resolved send every *.localhost
// name to the loopback address, so it works without editing /etc/hosts.
const Hostname = "quipu.localhost"

// ourHost accepts the names this server can be reached by on its own port. Anything else is
// refused, which defeats DNS rebinding.
func (s *Server) ourHost(host string) bool {
	port := s.addr[strings.LastIndex(s.addr, ":"):]
	return host == s.addr || host == "localhost"+port || host == "[::1]"+port || host == Hostname+port
}

// sameOrigin requires an Origin header naming this server; the Host itself is checked earlier.
func (s *Server) sameOrigin(r *http.Request) bool {
	return r.Header.Get("Origin") == "http://"+r.Host
}

func (s *Server) validToken(t string) bool {
	return subtle.ConstantTimeCompare([]byte(t), []byte(s.token)) == 1
}
