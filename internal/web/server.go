// Package web serves Tasky's protected, loopback-only HTTP interface.
package web

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"time"
)

type Server struct {
	listener                                      net.Listener
	http                                          *http.Server
	origin, accessToken, sessionToken, cookieName string
	configPath, cwd                               string
}

func New(configPath, cwd string, port int) (*Server, error) {
	if port < 0 || port > 65535 {
		return nil, fmt.Errorf("port must be between 0 and 65535 (0 selects an available port)")
	}
	if !filepath.IsAbs(configPath) || !filepath.IsAbs(cwd) {
		return nil, fmt.Errorf("configuration path and directory context must be absolute")
	}
	// The address is fixed here, not supplied by flags or request input.
	listener, err := net.Listen("tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return nil, fmt.Errorf("cannot listen on 127.0.0.1:%d: %w; choose another --port or check local permissions", port, err)
	}
	s := &Server{
		listener: listener, origin: localOrigin(listener.Addr().(*net.TCPAddr).Port),
		accessToken: rand.Text(), sessionToken: rand.Text(),
		cookieName: "tasky_session_" + strconv.Itoa(listener.Addr().(*net.TCPAddr).Port),
		configPath: configPath, cwd: cwd,
	}
	s.http = &http.Server{
		Handler: s.handler(), ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10,
	}
	return s, nil
}

func localOrigin(port int) string {
	// Browsers omit the default HTTP port from both Origin and Host.
	if port == 80 {
		return "http://127.0.0.1"
	}
	return "http://" + net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
}

// URL includes a per-run credential in the fragment, never in an HTTP request URL.
func (s *Server) URL() string { return s.origin + "/#token=" + s.accessToken }

func (s *Server) Close() {
	_ = s.http.Close()
	_ = s.listener.Close()
}

func (s *Server) Serve(ctx context.Context) error {
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		select {
		case <-ctx.Done():
			shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := s.http.Shutdown(shutdown); err != nil {
				_ = s.http.Close()
			}
		case <-stop:
		}
	}()
	err := s.http.Serve(s.listener)
	close(stop)
	<-done
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return fmt.Errorf("serve local dashboard: %w", err)
}
