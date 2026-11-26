package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLocalOriginMatchesBrowserSerialization(t *testing.T) {
	for _, test := range []struct {
		port int
		want string
	}{
		{80, "http://127.0.0.1"},
		{9000, "http://127.0.0.1:9000"},
	} {
		if got := localOrigin(test.port); got != test.want {
			t.Errorf("port %d: origin %q, want %q", test.port, got, test.want)
		}
	}
}

func TestDefaultPortAcceptsBrowserHostAndOrigin(t *testing.T) {
	s := &Server{origin: localOrigin(80)}
	request := httptest.NewRequest(http.MethodGet, "http://127.0.0.1/", nil)
	request.Header.Set("Origin", "http://127.0.0.1")
	response := httptest.NewRecorder()
	s.handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("default-port browser request: %d", response.Code)
	}
}
