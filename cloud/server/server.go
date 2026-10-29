// Package server is the cloud's HTTP server: its API (cloud/api) over the
// database (cloud/store). It speaks plain HTTP behind a proxy that ends TLS.
package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/enowx/enowx/cloud/api"
	"github.com/enowx/enowx/cloud/store"
	"github.com/enowx/enowx/version"
)

// Config is how a server is set up (cmd/enowx-cloud reads it from the
// environment).
type Config struct {
	// URL is the address members reach it at.
	URL string
	// GitHubClientID and GitHubClientSecret are the OAuth app members sign
	// in with; empty, GitHub sign-in is off.
	GitHubClientID, GitHubClientSecret string
	// SMTPURL sends invites by email; empty, invites are links only.
	SMTPURL string
}

// Server answers the cloud's API. Its handlers run on net/http's
// goroutines; the database is safe for that.
type Server struct {
	cfg Config
	db  *store.DB
	log *slog.Logger
	mux *http.ServeMux
}

// New is a server over db.
func New(cfg Config, db *store.DB, log *slog.Logger) *Server {
	s := &Server{cfg: cfg, db: db, log: log, mux: http.NewServeMux()}
	s.mux.HandleFunc("GET "+api.PathHealth, s.health)
	s.mux.HandleFunc("GET "+api.PathInfo, s.info)
	s.mux.HandleFunc("GET "+api.PathMe, s.signedIn(s.me))
	s.mux.HandleFunc("GET "+api.PathDevices, s.signedIn(s.devices))
	s.mux.HandleFunc("DELETE "+api.PathDevice, s.signedIn(s.removeDevice))
	s.mux.HandleFunc("POST "+api.PathSignOut, s.signedIn(s.signOut))
	return s
}

// ServeHTTP serves the API, each request logged.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	rw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
	s.mux.ServeHTTP(rw, r)
	if r.URL.Path != api.PathHealth {
		s.log.Info("request", "method", r.Method, "path", r.URL.Path, "status", rw.status, "ms", time.Since(start).Milliseconds())
	}
}

// health is up when the database answers.
func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	if err := s.db.PingContext(r.Context()); err != nil {
		s.fail(w, http.StatusServiceUnavailable, "database: "+err.Error())
		return
	}
	w.Write([]byte("ok\n"))
}

func (s *Server) info(w http.ResponseWriter, r *http.Request) {
	in := api.Info{Name: "enowx-cloud", Version: version.String, API: api.Version, URL: s.cfg.URL, SignIn: []string{}}
	if s.cfg.GitHubClientID != "" && s.cfg.GitHubClientSecret != "" {
		in.SignIn = append(in.SignIn, api.SignInGitHub)
	}
	s.reply(w, in)
}

func (s *Server) reply(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		s.log.Warn("reply", "err", err)
	}
}

func (s *Server) fail(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(api.Error{Error: msg})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}
