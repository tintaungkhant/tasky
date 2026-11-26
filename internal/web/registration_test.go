package web

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/tintaungkhant/tasky/internal/config"
	"github.com/tintaungkhant/tasky/internal/store"
)

func TestRegistrationHTTP(t *testing.T) {
	s := newTestServer(t)
	h := sameOrigin(s)
	h.Set("Cookie", s.cookieName+"="+s.sessionToken)
	h.Set("Content-Type", "application/json")
	call := func(method, path string, body any) []byte {
		t.Helper()
		data, _ := json.Marshal(body)
		w := request(t, s, method, path, h, string(data))
		if w.Code < 200 || w.Code >= 300 {
			t.Fatalf("%s %s: %d %s", method, path, w.Code, w.Body.String())
		}
		return w.Body.Bytes()
	}
	dir := t.TempDir()
	gitRun := func(path string, args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", path}, args...)...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, output)
		}
		return string(output)
	}
	repo := filepath.Join(dir, "repo")
	if err := os.MkdirAll(repo, 0755); err != nil {
		t.Fatal(err)
	}
	gitRun(repo, "init")
	gitRun(repo, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "--allow-empty", "-m", "initial")
	legacyTask := filepath.Join(dir, "tasks", "T-1")
	if err := os.MkdirAll(legacyTask, 0755); err != nil {
		t.Fatal(err)
	}
	legacyWorktree := filepath.Join(legacyTask, "repo")
	gitRun(repo, "worktree", "add", "-b", "legacy", legacyWorktree)
	workspace := filepath.Join(legacyTask, "T-1.code-workspace")
	if err := os.WriteFile(workspace, []byte("preserve workspace"), 0600); err != nil {
		t.Fatal(err)
	}
	beforeGit := gitRun(repo, "show-ref") + gitRun(repo, "worktree", "list", "--porcelain")
	cfg := config.Config{Roots: []config.Root{{Name: "legacy", Path: dir, TasksDir: filepath.Join(dir, "tasks"), BranchTemplate: "task/{task-id}/{branch}"}}}
	call("PUT", "/api/preferences", map[string]any{"expected": config.Config{}, "config": cfg})
	yamlBefore, err := os.ReadFile(s.configPath)
	if err != nil {
		t.Fatal(err)
	}
	call("POST", "/api/import", struct{}{})
	first := call("GET", "/api/roots", nil)
	call("POST", "/api/import", struct{}{})
	if string(first) != string(call("GET", "/api/roots", nil)) {
		t.Fatal("import changed identities")
	}
	var roots []store.Root
	json.Unmarshal(first, &roots)
	var legacyPreview store.RootMappingSnapshot
	json.Unmarshal(call("GET", "/api/registration/removal?root="+roots[0].ID, nil), &legacyPreview)
	if len(legacyPreview.Tasks) != 1 || len(legacyPreview.Worktrees) != 1 {
		t.Fatal("legacy mappings missing")
	}
	duplicate := filepath.Join(dir, "tasks", "T-1@duplicate")
	if err := os.Mkdir(duplicate, 0755); err != nil {
		t.Fatal(err)
	}
	if w := request(t, s, "POST", "/api/import", h, `{}`); w.Code != 400 {
		t.Fatalf("ambiguous import accepted: %d", w.Code)
	}
	if err := os.Remove(duplicate); err != nil {
		t.Fatal(err)
	}
	var preserved store.RootMappingSnapshot
	json.Unmarshal(call("GET", "/api/registration/removal?root="+roots[0].ID, nil), &preserved)
	beforeMappings, _ := json.Marshal(legacyPreview)
	afterMappings, _ := json.Marshal(preserved)
	if string(beforeMappings) != string(afterMappings) {
		t.Fatal("failed import changed mappings")
	}
	yamlAfter, _ := os.ReadFile(s.configPath)
	if string(yamlBefore) != string(yamlAfter) {
		t.Fatal("import changed YAML")
	}
	if data, err := os.ReadFile(workspace); err != nil || string(data) != "preserve workspace" {
		t.Fatal("import changed workspace file")
	}
	if after := gitRun(repo, "show-ref") + gitRun(repo, "worktree", "list", "--porcelain"); after != beforeGit {
		t.Fatal("import changed Git state")
	}
	linkedDuplicate, _ := json.Marshal(map[string]string{"RootID": roots[0].ID, "Name": "linked duplicate", "Path": legacyWorktree})
	if w := request(t, s, "POST", "/api/registration/repos", h, string(linkedDuplicate)); w.Code != 409 {
		t.Fatalf("linked checkout registered twice: %d", w.Code)
	}
	call("POST", "/api/registration/roots", map[string]string{"Name": "nested", "Path": filepath.Join(dir, "tasks")})
	var root store.Root
	json.Unmarshal(call("POST", "/api/registration/roots", map[string]string{"Name": "legacy", "Path": t.TempDir()}), &root)
	if root.ID == roots[0].ID {
		t.Fatal("duplicate names collapsed")
	}
	if w := request(t, s, "GET", "/api/repos?root=legacy", h, ""); w.Code != 404 {
		t.Fatal("ambiguous name accepted as identity")
	}
	invalidRepo, _ := json.Marshal(map[string]string{"RootID": root.ID, "Name": "not Git", "Path": t.TempDir()})
	if w := request(t, s, "POST", "/api/registration/repos", h, string(invalidRepo)); w.Code != 400 {
		t.Fatal("non-Git directory registered")
	}
	external := t.TempDir()
	cmd := exec.Command("git", "init", external)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("init: %v %s", err, output)
	}
	sentinel := filepath.Join(external, "keep.txt")
	os.WriteFile(sentinel, []byte("keep"), 0600)
	call("POST", "/api/registration/repos", map[string]string{"RootID": root.ID, "Name": "external", "Path": external})
	var snapshot store.RootMappingSnapshot
	json.Unmarshal(call("GET", "/api/registration/removal?root="+root.ID, nil), &snapshot)
	if len(snapshot.Repos) != 1 {
		t.Fatal("removal preview omits Repo")
	}
	var nested store.Repo
	json.Unmarshal(call("POST", "/api/registration/repos", map[string]string{"RootID": root.ID, "Name": "nested", "Path": repo}), &nested)
	stale, _ := json.Marshal(map[string]any{"expected": snapshot, "confirm": true})
	if w := request(t, s, "POST", "/api/registration/remove", h, string(stale)); w.Code != 409 {
		t.Fatal("stale mapping preview accepted")
	}
	json.Unmarshal(call("GET", "/api/registration/removal?root="+root.ID, nil), &snapshot)
	data, _ := json.Marshal(map[string]any{"expected": snapshot})
	if w := request(t, s, "POST", "/api/registration/remove", h, string(data)); w.Code != 400 {
		t.Fatal("unconfirmed removal allowed")
	}
	call("POST", "/api/registration/remove", map[string]any{"expected": snapshot, "confirm": true})
	if data, err := os.ReadFile(sentinel); err != nil || string(data) != "keep" {
		t.Fatal("registration removal touched Git data")
	}
	if _, err := os.Stat(filepath.Join(external, ".git")); err != nil {
		t.Fatal("Git directory lost")
	}
	if after := gitRun(repo, "show-ref") + gitRun(repo, "worktree", "list", "--porcelain"); after != beforeGit {
		t.Fatal("removal changed Git state")
	}
}

func TestRegistrationBoundaryValidation(t *testing.T) {
	s := newTestServer(t)
	h := sameOrigin(s)
	h.Set("Cookie", s.cookieName+"="+s.sessionToken)
	h.Set("Content-Type", "application/json")
	for _, tc := range []struct {
		path, body string
		status     int
	}{
		{"/api/registration/roots", `{"Name":"x","Path":"relative"}`, 400},
		{"/api/registration/roots", `{"Name":"x","Path":"x","extra":true}`, 400},
		{"/api/registration/repos", `{"RootID":"","Name":"x","Path":"x"}`, 400},
		{"/api/preferences", `null`, 400},
		{"/api/import", `{} {}`, 400},
	} {
		method := "POST"
		if tc.path == "/api/preferences" {
			method = "PUT"
		}
		if w := request(t, s, method, tc.path, h, tc.body); w.Code != tc.status {
			t.Errorf("%s: %d %s", tc.path, w.Code, w.Body.String())
		}
	}
	if w := request(t, s, "GET", "/api/repos?root=missing&root=other", h, ""); w.Code != 400 {
		t.Fatal("ambiguous query accepted")
	}
	if w := request(t, s, "POST", "/api/import", nil, `{}`); w.Code < 400 {
		t.Fatal("unauthenticated mutation accepted")
	}
}
