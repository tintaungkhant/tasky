package web

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"strings"
	"testing"

	"github.com/tintaungkhant/tasky/internal/app"
	"github.com/tintaungkhant/tasky/internal/store"
)

func TestDashboardAssets(t *testing.T) {
	s := newTestServer(t)
	page := request(t, s, "GET", "/", nil, "")
	if !strings.Contains(page.Body.String(), `type="module"`) {
		t.Fatal("dashboard module missing")
	}
	references := 0
	for _, field := range strings.Fields(page.Body.String()) {
		if !strings.HasPrefix(field, `src="/assets/`) && !strings.HasPrefix(field, `href="/assets/`) {
			continue
		}
		references++
		path := strings.Split(field, `"`)[1]
		asset := request(t, s, "GET", path, nil, "")
		if asset.Code != http.StatusOK || asset.Body.Len() == 0 {
			t.Fatalf("missing embedded asset: %s", path)
		}
	}
	if references < 2 {
		t.Fatal("dashboard must reference production JS and CSS")
	}
	if err := fs.WalkDir(dashboardFiles, "dist/assets", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		w := request(t, s, "GET", strings.TrimPrefix(path, "dist"), nil, "")
		if w.Code != 200 {
			t.Errorf("asset %s returned %d", path, w.Code)
		}
		contentType := w.Header().Get("Content-Type")
		if strings.HasSuffix(path, ".js") && !strings.Contains(contentType, "javascript") {
			t.Errorf("JS MIME type: %s", contentType)
		}
		if strings.HasSuffix(path, ".css") && !strings.HasPrefix(contentType, "text/css") {
			t.Errorf("CSS MIME type: %s", contentType)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/assets/", "/assets/missing.js", "/dist/index.html", "/unknown"} {
		if w := request(t, s, "GET", path, nil, ""); w.Code != http.StatusNotFound {
			t.Fatalf("%s returned %d", path, w.Code)
		}
	}
	csp := page.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "script-src 'self'") || strings.Contains(csp, "unsafe-inline") {
		t.Fatal("unsafe module CSP")
	}
}

func TestRootsRequireSessionAndPreserveIdentity(t *testing.T) {
	s := newTestServer(t)
	if w := request(t, s, "GET", "/api/roots", nil, ""); w.Code != 401 {
		t.Fatal("Roots accessible without session")
	}
	h := sameOrigin(s)
	h.Set("Cookie", s.cookieName+"="+s.sessionToken)
	empty := request(t, s, "GET", "/api/roots", h, "")
	if empty.Code != 200 || strings.TrimSpace(empty.Body.String()) != "[]" {
		t.Fatal("empty Roots must be an array")
	}
	db, err := store.Open(app.DatabasePath(s.configPath))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for range 2 {
		if _, err := db.CreateRoot("same", t.TempDir()); err != nil {
			t.Fatal(err)
		}
	}
	w := request(t, s, "GET", "/api/roots", h, "")
	var roots []store.Root
	if err := json.Unmarshal(w.Body.Bytes(), &roots); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || len(roots) != 2 || roots[0].ID == roots[1].ID {
		t.Fatal("Root identities lost")
	}
	if w := request(t, s, "POST", "/api/roots", h, ""); w.Code != 405 {
		t.Fatal("Roots endpoint accepts mutation")
	}
	if w := request(t, s, "GET", "/api/roots?root=a", h, ""); w.Code != 400 {
		t.Fatal("unexpected selector accepted")
	}
}
