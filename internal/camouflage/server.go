package camouflage

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/web/network"
)

// Server is a loopback static HTTP(S) site; Xray fallbacks forward unmatched
// connections to it. Only one cover site runs per panel.
type Server struct {
	mu       sync.Mutex
	srv      *http.Server
	listener net.Listener
	dir      string
	port     int
	redirect string
}

var Default = &Server{}

var ErrNotRunning = errors.New("camouflage site is not running")

type status struct {
	Running  bool   `json:"running" example:"true"`
	Port     int    `json:"port" example:"2082"`
	Redirect string `json:"redirectUrl,omitempty" example:"https://example.com"`
}

// Status reports the current listener without starting anything.
func (s *Server) Status() status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return status{Running: s.srv != nil, Port: s.port, Redirect: s.redirect}
}

// Start (re)binds the server on 127.0.0.1:port serving dir, or issuing 302s
// to redirect when non-empty. Port 0 keeps the current/ephemeral port.
func (s *Server) Start(dir string, port int, redirect string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if port <= 0 {
		port = s.port
	}
	if port <= 0 {
		return errors.New("camouflage port must be 1-65535")
	}
	// Release our own previous bind first: re-applying on the same port must not
	// collide with the still-listening old instance.
	s.closeLocked()
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return fmt.Errorf("bind camouflage site on 127.0.0.1:%d: %w", port, err)
	}
	s.dir, s.port, s.redirect = dir, port, redirect
	cert, err := ensureTLS(dir)
	if err != nil {
		return fmt.Errorf("prepare camouflage TLS certificate: %w", err)
	}
	ln = &sniffListener{Listener: ln, cfg: &tls.Config{
		Certificates: []tls.Certificate{*cert},
		MinVersion:   tls.VersionTLS12,
		NextProtos:   []string{"h2", "http/1.1"},
	}}
	srv := &http.Server{
		Handler:           handler(dir, redirect),
		ReadHeaderTimeout: 10 * time.Second,
		ErrorLog:          nil,
	}
	s.srv = srv
	s.listener = ln
	// Capture srv/ln locally: a concurrent rebind clears the fields, and the
	// serve goroutine must not read them back (nil deref under restart).
	go func() {
		network.ServeHTTP(srv, ln, "camouflage site")
		s.mu.Lock()
		crashed := s.listener == ln
		if crashed {
			s.srv, s.listener = nil, nil
		}
		dir, port, redirect := s.dir, s.port, s.redirect
		s.mu.Unlock()
		if crashed {
			// A Serve failure that was not our own Stop/rebind must not leave
			// the cover dark in front of the REALITY fallback forever.
			time.Sleep(2 * time.Second)
			_ = s.Start(dir, port, redirect)
		}
	}()
	return nil
}

// Stop shuts the listener down; a stopped server is not an error.
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closeLocked()
	return nil
}

func (s *Server) closeLocked() {
	if s.srv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = s.srv.Shutdown(ctx)
	}
	if s.listener != nil {
		_ = s.listener.Close()
	}
	s.srv, s.listener = nil, nil
}

// handler captures dir/redirect so in-flight requests never read mutated fields.
func handler(dir, redirect string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if redirect != "" {
			http.Redirect(w, r, redirect, http.StatusFound)
			return
		}
		// Clean("/"+path) rejects ".." traversal; 404 falls back to index.html.
		clean := filepath.Clean("/" + r.URL.Path)
		target := filepath.Join(dir, clean)
		if info, err := os.Stat(target); err == nil && info.IsDir() {
			target = filepath.Join(target, "index.html")
		}
		if _, err := os.Stat(target); err != nil {
			notFound := filepath.Join(dir, "404.html")
			if _, err := os.Stat(notFound); err == nil {
				w.WriteHeader(http.StatusNotFound)
				http.ServeFile(w, r, notFound)
				return
			}
			index := filepath.Join(dir, "index.html")
			if _, err := os.Stat(index); err == nil {
				http.ServeFile(w, r, index)
				return
			}
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, target)
	})
}
