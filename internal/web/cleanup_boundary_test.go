package web

import (
	"net/http"
	"testing"
)

func TestDashboardCleanupRequestValidation(t *testing.T) {
	s := newTestServer(t)
	h := authorizedHeaders(s)
	h.Set("Content-Type", "application/json")
	for _, tc := range []struct{ method, path, body string }{
		{"POST", "/api/tasks/cleanup/preview", `{"RootID":"root","Mappings":[]}`},
		{"POST", "/api/tasks/cleanup/preview", `{"RootID":"root","Mappings":[{"ID":"m","TaskID":"t","RepoID":"r","Path":"relative","Identity":"i"}]}`},
		{"POST", "/api/tasks/cleanup", `{"Review":{"RootID":"root"},"Confirm":false}`},
		{"POST", "/api/recover", `{"RootID":"root","ID":"op","Confirm":false}`},
		{"POST", "/api/recover", `{"RootID":"root","ID":"op","Confirm":true,"Options":{"Forget":true,"ForceBranch":true}}`},
		{"POST", "/api/recover", `{"RootID":"root","ID":"op","Confirm":true,"unknown":true}`},
		{"GET", "/api/pending?root=root&root=other", ""},
	} {
		w := request(t, s, tc.method, tc.path, h, tc.body)
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s %s %s: got %d: %s", tc.method, tc.path, tc.body, w.Code, w.Body.String())
		}
	}
	for _, path := range []string{"/api/tasks/cleanup/preview", "/api/tasks/cleanup", "/api/recover"} {
		if w := request(t, s, "POST", path, sameOrigin(s), `{}`); w.Code != http.StatusUnauthorized {
			t.Errorf("unauthenticated %s: %d", path, w.Code)
		}
		foreign := h.Clone()
		foreign.Set("Origin", "http://unrelated.invalid")
		if w := request(t, s, "POST", path, foreign, `{}`); w.Code != http.StatusForbidden {
			t.Errorf("foreign Origin %s: %d", path, w.Code)
		}
	}
}
