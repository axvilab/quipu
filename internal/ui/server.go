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
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/axvilab/quipu/internal/fsx"
	"github.com/axvilab/quipu/internal/store"
)

//go:embed static all:dist
var assets embed.FS

const (
	keepBackups = 90 * 24 * time.Hour // changes older than this can no longer be undone
	cookieName  = "quipu_token"
	healthReply = "quipu"
)

// Service is everything the UI can read and do, as plain methods: the HTTP API wraps it, and a
// native window can bind the same methods later (redesign plan, step 6).
type Service struct {
	home      string
	configDir string
	profile   string
	st        *store.Store
	dataDir   string
	journal   *fsx.Journal
	version   string

	// Optional: where profiles are configured and the real claude binary, to show every
	// profile with its login state.
	ProfilesFile string
	ClaudeBinary string
	SelfBinary   string // quipu itself, used as BROWSER for `claude auth login`
	UpdateBase   string // where releases come from; update.DefaultBase when empty
	Restart      func() // starts the installed update in place of this process; nil when quipu cannot
	profileCache profileCache
	logins       logins
	coll         collector
	upd          updater
}

// NewService opens the backup journal under <dataDir>/backups.
func NewService(home, configDir, profile, dataDir, version string, st *store.Store) (*Service, error) {
	journal, err := fsx.OpenJournal(filepath.Join(dataDir, "backups"))
	if err != nil {
		return nil, err
	}
	journal.Prune(keepBackups) // best effort: old copies only take space
	return &Service{home: home, configDir: configDir, profile: profile, st: st, dataDir: dataDir, journal: journal, version: version}, nil
}

// Server serves a Service over HTTP on the loopback interface, guarded by a token.
type Server struct {
	*Service
	token string
	addr  string // host:port we listen on, for the Host check
}

// New prepares a server; call Listen to start it. version is shown in the UI.
func New(home, configDir, profile, dataDir, version string, st *store.Store) (*Server, error) {
	svc, err := NewService(home, configDir, profile, dataDir, version, st)
	if err != nil {
		return nil, err
	}
	return NewServer(svc)
}

// NewServer wraps svc in an HTTP server. The token is kept in <dataDir>/ui-token (0600) and reused,
// so an open tab or a bookmark keeps working after the daemon restarts (updates, reboots).
func NewServer(svc *Service) (*Server, error) {
	token, err := loadToken(filepath.Join(svc.dataDir, "ui-token"))
	if err != nil {
		return nil, err
	}
	return &Server{Service: svc, token: token}, nil
}

func loadToken(file string) (string, error) {
	if b, err := os.ReadFile(file); err == nil {
		if t := strings.TrimSpace(string(b)); len(t) == 32 {
			return t, nil
		}
	}
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	t := hex.EncodeToString(b)
	if err := os.WriteFile(file, []byte(t+"\n"), 0o600); err != nil {
		return "", err
	}
	return t, nil
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
	go s.Collect(ctx, time.Minute)
	ready(fmt.Sprintf("http://%s/?t=%s", s.addr, s.token))
	if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Handler is the full HTTP handler, guarded by the Host and token checks.
func (s *Server) Handler() http.Handler {
	return s.guard(s.Routes())
}

// Routes serves the app and the API without the network guard. Use it only where requests
// cannot come from outside the process, such as the native window's asset handler (ADR 0007).
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	static, _ := fs.Sub(assets, "static")
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(static)))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, healthReply) })
	mux.HandleFunc("GET /favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		w.Write(Icon())
	})
	s.apiRoutes(mux)
	if root := app(); root != nil {
		mux.Handle("GET /", appHandler(root))
	} else {
		mux.HandleFunc("GET /", notBuilt)
	}
	return mux
}

func (s *Server) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.addr != "" && !s.ourHost(r.Host) {
			http.Error(w, "unexpected Host header", http.StatusForbidden)
			return
		}
		// The health check and static files (styles, icon, app manifest) hold no data. Browsers
		// fetch the manifest without cookies, so these must not need the token.
		if r.Method == http.MethodGet && (r.URL.Path == "/healthz" || r.URL.Path == "/favicon.ico" || strings.HasPrefix(r.URL.Path, "/static/") || strings.HasPrefix(r.URL.Path, "/assets/")) {
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
			if strings.HasPrefix(r.URL.Path, "/api/") {
				writeJSON(w, nil, &Problem{Code: "unauthorized"})
				return
			}
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
