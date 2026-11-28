package web

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tintaungkhant/tasky/internal/app"
)

func TestFolderBrowsingListsOnlyDirectories(t *testing.T) {
	s := newTestServer(t)
	for _, name := range []string{"a folder", "မြန်မာ", ".hidden"} {
		if err := os.Mkdir(filepath.Join(s.cwd, name), 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(s.cwd, "secret.env"), []byte("not returned"), 0600); err != nil {
		t.Fatal(err)
	}
	w := request(t, s, "GET", "/api/folders", authorizedHeaders(s), "")
	if w.Code != http.StatusOK {
		t.Fatalf("browse: %d %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "secret.env") || strings.Contains(w.Body.String(), "not returned") {
		t.Fatal("folder listing exposed a file name or contents")
	}
	var result struct {
		Path, Parent           string
		Directories, Locations []struct{ Name, Path string }
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	want, err := filepath.EvalSymlinks(s.cwd)
	if err != nil {
		t.Fatal(err)
	}
	if result.Path != want || result.Parent != filepath.Dir(want) || len(result.Directories) != 3 || len(result.Locations) == 0 {
		t.Fatalf("listing: %+v", result)
	}
	for _, entry := range result.Directories {
		info, err := os.Stat(entry.Path)
		if err != nil || !info.IsDir() || entry.Path != filepath.Join(want, entry.Name) {
			t.Fatalf("entry %+v: %v", entry, err)
		}
	}
	child := result.Directories[0].Path
	w = request(t, s, "GET", "/api/folders?path="+url.QueryEscape(child), authorizedHeaders(s), "")
	if w.Code != 200 {
		t.Fatalf("child: %d %s", w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Path != child || result.Parent != want || result.Directories == nil || len(result.Directories) != 0 {
		t.Fatalf("empty child: %+v", result)
	}
	if _, err := os.Stat(app.DatabasePath(s.configPath)); !os.IsNotExist(err) {
		t.Fatalf("browsing created a database: %v", err)
	}
}

func TestFolderBrowsingValidatesPathsAndBoundary(t *testing.T) {
	s := newTestServer(t)
	h := authorizedHeaders(s)
	file := filepath.Join(s.cwd, "file")
	if err := os.WriteFile(file, []byte("do not return"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		query  string
		status int
	}{
		{"?path=relative", 400}, {"?path=%00", 400}, {"?unknown=x", 400}, {"?path=a&path=b", 400},
		{"?path=" + url.QueryEscape(filepath.Join(s.cwd, "missing")), 404}, {"?path=" + url.QueryEscape(file), 400},
	} {
		if w := request(t, s, "GET", "/api/folders"+tc.query, h, ""); w.Code != tc.status {
			t.Errorf("%s: %d %s", tc.query, w.Code, w.Body.String())
		}
	}
	if w := request(t, s, "GET", "/api/folders", nil, ""); w.Code != 401 {
		t.Fatalf("unauthenticated: %d", w.Code)
	}
	foreign := h.Clone()
	foreign.Set("Origin", "http://unrelated.invalid")
	if w := request(t, s, "GET", "/api/folders", foreign, ""); w.Code != 403 {
		t.Fatalf("foreign origin: %d", w.Code)
	}
	foreign = h.Clone()
	foreign.Set("Sec-Fetch-Site", "cross-site")
	if w := request(t, s, "GET", "/api/folders", foreign, ""); w.Code != 403 {
		t.Fatalf("cross-site: %d", w.Code)
	}
	h.Set("Origin", s.origin)
	if w := request(t, s, "POST", "/api/folders", h, "{}"); w.Code != 405 {
		t.Fatalf("mutation: %d", w.Code)
	}
}
