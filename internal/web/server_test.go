package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tintaungkhant/tasky/internal/store"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	s, err := New(filepath.Join(dir, "config.yaml"), dir, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s
}

func request(t *testing.T, s *Server, method, path string, headers http.Header, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, s.origin+path, strings.NewReader(body))
	r.Host = strings.TrimPrefix(s.origin, "http://")
	for k, values := range headers {
		for _, v := range values {
			r.Header.Add(k, v)
		}
	}
	w := httptest.NewRecorder()
	s.handler().ServeHTTP(w, r)
	return w
}

func sameOrigin(s *Server) http.Header { h := http.Header{}; h.Set("Origin", s.origin); return h }

func TestNewValidatesInputsAndOccupiedPort(t *testing.T) {
	dir := t.TempDir()
	for _, tc := range []struct {
		name, config, cwd string
		port              int
	}{
		{"negative port", filepath.Join(dir, "c"), dir, -1}, {"large port", filepath.Join(dir, "c"), dir, 65536},
		{"relative config", "config.yaml", dir, 0}, {"relative cwd", filepath.Join(dir, "c"), "relative", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := New(tc.config, tc.cwd, tc.port); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	_, portText, _ := net.SplitHostPort(l.Addr().String())
	var port int
	fmt.Sscan(portText, &port)
	if _, err := New(filepath.Join(dir, "c"), dir, port); err == nil {
		t.Fatal("expected occupied-port error")
	}
}

func TestServerLoopbackIndependentCredentialsAndSafePage(t *testing.T) {
	a, b := newTestServer(t), newTestServer(t)
	if a.listener.Addr().(*net.TCPAddr).IP.String() != "127.0.0.1" {
		t.Fatalf("listener bound to %s", a.listener.Addr())
	}
	if a.accessToken == b.accessToken || a.sessionToken == b.sessionToken {
		t.Fatal("servers share credentials")
	}
	for _, s := range []*Server{a, b} {
		u, err := url.Parse(s.URL())
		if err != nil {
			t.Fatal(err)
		}
		if u.Fragment != "token="+s.accessToken {
			t.Fatalf("URL fragment missing credential")
		}
		resp := request(t, s, "GET", "/", nil, "")
		body := resp.Body.String()
		if resp.Code != http.StatusOK || strings.Contains(body, s.accessToken) || strings.Contains(body, s.sessionToken) {
			t.Fatalf("unsafe startup page response: status %d", resp.Code)
		}
		for name, want := range map[string]string{"Cache-Control": "no-store", "Referrer-Policy": "no-referrer", "X-Frame-Options": "DENY"} {
			if got := resp.Header().Get(name); got != want {
				t.Errorf("%s = %q, want %q", name, got, want)
			}
		}
		if !strings.Contains(resp.Header().Get("Content-Security-Policy"), "frame-ancestors 'none'") {
			t.Errorf("CSP does not block framing: %q", resp.Header().Get("Content-Security-Policy"))
		}
	}
}

func TestAPIRequiresValidSession(t *testing.T) {
	s := newTestServer(t)
	for _, cookie := range []string{"", "wrong", "stale"} {
		h := sameOrigin(s)
		if cookie != "" {
			h.Set("Cookie", s.cookieName+"="+cookie)
		}
		w := request(t, s, "GET", "/api/status", h, "")
		if w.Code != http.StatusUnauthorized {
			t.Errorf("invalid cookie rejected with status %d", w.Code)
		}
	}
}

func TestSessionExchangeValidatesRequestAndSetsCookie(t *testing.T) {
	s := newTestServer(t)
	valid := sameOrigin(s)
	valid.Set("Content-Type", "application/json")
	cases := []struct {
		name    string
		headers http.Header
		body    string
	}{
		{"missing origin", http.Header{"Content-Type": []string{"application/json"}}, `{"token":"` + s.accessToken + `"}`},
		{"wrong origin", func() http.Header { h := sameOrigin(s); h.Set("Origin", "http://evil.test"); return h }(), `{"token":"` + s.accessToken + `"}`},
		{"wrong token", valid, `{"token":"bad"}`}, {"wrong content type", func() http.Header { h := sameOrigin(s); h.Set("Content-Type", "text/plain"); return h }(), `{"token":"` + s.accessToken + `"}`},
		{"malformed", valid, `{"token":`}, {"unknown field", valid, `{"token":"` + s.accessToken + `","extra":true}`},
		{"trailing value", valid, `{"token":"` + s.accessToken + `"} {}`}, {"oversized", valid, `{"token":"` + s.accessToken + `","pad":"` + strings.Repeat("x", 5000) + `"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := request(t, s, "POST", "/api/session", tc.headers, tc.body)
			if w.Code < 400 {
				t.Fatalf("accepted invalid request: %d", w.Code)
			}
		})
	}
	w := request(t, s, "POST", "/api/session", valid, `{"token":"`+s.accessToken+`"}`)
	if w.Code != http.StatusNoContent {
		t.Fatalf("valid exchange status %d: %s", w.Code, w.Body.String())
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("got %d cookies", len(cookies))
	}
	c := cookies[0]
	if !c.HttpOnly || c.SameSite != http.SameSiteStrictMode || c.Value == "" {
		t.Fatalf("cookie attributes incorrect")
	}
	h := sameOrigin(s)
	h.Set("Cookie", c.Name+"="+c.Value)
	status := request(t, s, "GET", "/api/status", h, "")
	if status.Code != 200 {
		t.Fatalf("session status %d", status.Code)
	}
}

func TestRejectsHostAndCrossSiteRequestsEvenWithSession(t *testing.T) {
	s := newTestServer(t)
	cases := []struct{ name, host, origin, forwarded, site string }{
		{"malicious host", "attacker.test", s.origin, "", "same-origin"},
		{"forwarded host trick", "attacker.test", s.origin, strings.TrimPrefix(s.origin, "http://"), "same-origin"},
		{"evil origin", strings.TrimPrefix(s.origin, "http://"), "http://evil.test", "", "same-origin"},
		{"null origin", strings.TrimPrefix(s.origin, "http://"), "null", "", "same-origin"},
		{"other local port", strings.TrimPrefix(s.origin, "http://"), "http://127.0.0.1:1", "", "same-origin"},
		{"cross-site", strings.TrimPrefix(s.origin, "http://"), s.origin, "", "cross-site"},
		{"same-site", strings.TrimPrefix(s.origin, "http://"), s.origin, "", "same-site"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := http.Header{}
			h.Set("Origin", tc.origin)
			h.Set("Cookie", s.cookieName+"="+s.sessionToken)
			h.Set("Sec-Fetch-Site", tc.site)
			if tc.forwarded != "" {
				h.Set("X-Forwarded-Host", tc.forwarded)
			}
			r := httptest.NewRequest("GET", s.origin+"/api/status", nil)
			r.Host = tc.host
			for k, v := range h {
				r.Header[k] = v
			}
			w := httptest.NewRecorder()
			s.handler().ServeHTTP(w, r)
			if w.Code != http.StatusForbidden {
				t.Fatalf("request accepted with status %d", w.Code)
			}
		})
	}
	h := http.Header{}
	h.Set("Cookie", s.cookieName+"="+s.sessionToken)
	if w := request(t, s, "POST", "/api/session", h, `{"token":"`+s.accessToken+`"}`); w.Code != http.StatusForbidden {
		t.Fatalf("state change without Origin status %d", w.Code)
	}
}

func TestContextRejectsBadSelectorsAndExplainsMissingRoot(t *testing.T) {
	s := newTestServer(t)
	for _, q := range []string{"root=a&root=b", "unknown=x", "root=%zz"} {
		w := request(t, s, "GET", "/api/context?"+q, authorizedHeaders(s), "")
		if w.Code != 400 {
			t.Errorf("query %q status %d", q, w.Code)
		}
	}
	w := request(t, s, "GET", "/api/context?root=missing", authorizedHeaders(s), "")
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "use --root") || !strings.Contains(w.Body.String(), "tasky root list") {
		t.Fatalf("missing Root lacks actionable guidance: %d %s", w.Code, w.Body.String())
	}
}

func authorizedHeaders(s *Server) http.Header {
	h := sameOrigin(s)
	h.Set("Cookie", s.cookieName+"="+s.sessionToken)
	return h
}

func TestServeCancellationLifecycle(t *testing.T) {
	t.Run("running server releases port", func(t *testing.T) {
		s := newTestServer(t)
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { done <- s.Serve(ctx) }()
		client := http.Client{Timeout: time.Second}
		resp, err := client.Get(s.origin + "/")
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("Serve did not return after cancellation")
		}
		port := s.listener.Addr().(*net.TCPAddr).Port
		again, err := New(s.configPath, s.cwd, port)
		if err != nil {
			t.Fatalf("port not released: %v", err)
		}
		again.Close()
	})
	t.Run("cancel before Serve", func(t *testing.T) {
		s := newTestServer(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		done := make(chan error, 1)
		go func() { done <- s.Serve(ctx) }()
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("Serve hung")
		}
	})
	t.Run("Close before Serve", func(t *testing.T) {
		s := newTestServer(t)
		s.Close()
		done := make(chan error, 1)
		go func() { done <- s.Serve(context.Background()) }()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Fatal("Serve hung")
		}
	})
}

func TestWriteOperationErrorPreservesCauseAndRecoveryText(t *testing.T) {
	for _, tc := range []struct {
		cause     error
		status    int
		code, msg string
	}{{store.ErrConflict, 409, "conflict", "recovery id R-42"}, {store.ErrNotFound, 404, "not_found", "recovery id R-42"}} {
		w := httptest.NewRecorder()
		writeOperationError(w, fmt.Errorf("operation partially completed; recovery id R-42: %w", tc.cause))
		var got struct {
			Error apiError `json:"error"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if w.Code != tc.status || got.Error.Code != tc.code || !strings.Contains(got.Error.Message, tc.msg) {
			t.Errorf("response = %d %+v", w.Code, got)
		}
	}
}
