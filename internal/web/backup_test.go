package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tintaungkhant/tasky/internal/app"
	"github.com/tintaungkhant/tasky/internal/store"
)

func TestDashboardBackupRestoreRoundTripAndConflict(t *testing.T) {
	source, target := newTestServer(t), newTestServer(t)
	db, err := store.Open(app.DatabasePath(source.configPath))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	root, err := db.CreateRoot("Missing Root", filepath.Join(t.TempDir(), "missing"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreateRoot("Other Root", source.cwd); err != nil {
		t.Fatal(err)
	}
	download := request(t, source, http.MethodGet, "/api/backup", authorizedHeaders(source), "")
	if download.Code != http.StatusOK || !strings.HasPrefix(download.Header().Get("Content-Disposition"), "attachment;") {
		t.Fatalf("download: %d %s", download.Code, download.Body)
	}
	data := download.Body.String()
	snapshot, err := app.DecodeMappingBackup(strings.NewReader(data))
	if err != nil || len(snapshot.Roots) != 2 {
		t.Fatalf("CLI decoder: %+v %v", snapshot, err)
	}
	h := authorizedHeaders(target)
	h.Set("Content-Type", "application/json")
	for range 2 {
		w := request(t, target, http.MethodPost, "/api/restore", h, data)
		var report app.RestoreReport
		if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &report) != nil ||
			len(report.Warnings) != 1 || report.Warnings[0].ID != root.ID || report.Warnings[0].Path != root.Path {
			t.Fatalf("restore: %d %s", w.Code, w.Body)
		}
	}
	snapshot.Roots = []store.Root{
		{ID: "new-root", Name: "Must roll back", Path: filepath.Join(target.cwd, "new")},
		{ID: root.ID, Name: "Conflicts", Path: root.Path},
	}
	conflicting, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	w := request(t, target, http.MethodPost, "/api/restore", h, string(conflicting))
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "no mappings changed") {
		t.Fatalf("conflict: %d %s", w.Code, w.Body)
	}
	after := request(t, target, http.MethodGet, "/api/backup", h, "")
	if after.Code != http.StatusOK || after.Body.String() != data {
		t.Fatalf("mappings changed: %d %s", after.Code, after.Body)
	}
}

func TestDashboardBackupRestoreRejectPendingInAnyRoot(t *testing.T) {
	f := newTaskMutationFixture(t, 1)
	db := associationDB(t, f)
	task, err := db.CreateTask(f.root.ID, "PENDING", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ReserveCreation(task.ID, f.repos[0].ID, "pending", filepath.Join(f.base, "pending")); err != nil {
		t.Fatal(err)
	}
	for _, w := range []*httptest.ResponseRecorder{
		f.call(t, http.MethodGet, "/api/backup", nil),
		f.call(t, http.MethodPost, "/api/restore", map[string]int{"version": 1}),
	} {
		if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "Recovery") {
			t.Fatalf("pending blocker: %d %s", w.Code, w.Body)
		}
	}
}

func TestDashboardRestoreRequestValidation(t *testing.T) {
	s := newTestServer(t)
	h := authorizedHeaders(s)
	h.Set("Content-Type", "application/json")
	for _, tc := range []struct {
		method, path, body string
		headers            http.Header
		status             int
	}{
		{http.MethodGet, "/api/backup", "", sameOrigin(s), 401},
		{http.MethodPost, "/api/restore", `{"version":1}`, sameOrigin(s), 401},
		{http.MethodGet, "/api/backup?root=one", "", h, 400},
		{http.MethodGet, "/api/restore", "", h, 405},
		{http.MethodPost, "/api/restore", `{"version":1}`, authorizedHeaders(s), 415},
		{http.MethodPost, "/api/restore?path=arbitrary", `{"version":1}`, h, 400},
		{http.MethodPost, "/api/restore", `{"version":1,"repos":[{"ID":"r","RootID":"missing"}]}`, h, 400},
	} {
		w := request(t, s, tc.method, tc.path, tc.headers, tc.body)
		if w.Code != tc.status {
			t.Errorf("%s %s: got %d, want %d: %s", tc.method, tc.path, w.Code, tc.status, w.Body)
		}
	}
	crossOrigin := h.Clone()
	crossOrigin.Set("Origin", "https://unrelated.example")
	if w := request(t, s, http.MethodPost, "/api/restore", crossOrigin, `{"version":1}`); w.Code != http.StatusForbidden {
		t.Fatalf("cross-origin restore: %d %s", w.Code, w.Body)
	}
}
