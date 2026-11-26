package web

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/tintaungkhant/tasky/internal/app"
	"github.com/tintaungkhant/tasky/internal/config"
	"github.com/tintaungkhant/tasky/internal/store"
)

type taskMutationFixture struct {
	s     *Server
	h     http.Header
	root  store.Root
	repos []store.Repo
	base  string
}

func newTaskMutationFixture(t *testing.T, repoCount int) taskMutationFixture {
	t.Helper()
	s := newTestServer(t)
	h := authorizedHeaders(s)
	h.Set("Content-Type", "application/json")
	baseDir := t.TempDir()
	rootDir := filepath.Join(baseDir, "root")
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		t.Fatal(err)
	}
	base := filepath.Join(baseDir, "tasks")
	cfg := config.Config{Roots: []config.Root{{Name: "Root", Path: rootDir, TasksDir: base, BranchTemplate: "tasks/{task-id}/{branch}"}}}
	data, _ := json.Marshal(map[string]any{"expected": config.Config{}, "config": cfg})
	if w := request(t, s, "PUT", "/api/preferences", h, string(data)); w.Code != 200 {
		t.Fatalf("preferences: %d %s", w.Code, w.Body.String())
	}
	var root store.Root
	data, _ = json.Marshal(map[string]string{"Name": "Root", "Path": rootDir})
	w := request(t, s, "POST", "/api/registration/roots", h, string(data))
	if w.Code != 201 {
		t.Fatalf("register Root: %d %s", w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &root); err != nil {
		t.Fatal(err)
	}
	repos := make([]store.Repo, 0, repoCount)
	for i := 0; i < repoCount; i++ {
		repoPath := filepath.Join(baseDir, "repo", string(rune('a'+i)))
		if err := os.MkdirAll(repoPath, 0755); err != nil {
			t.Fatal(err)
		}
		gitTest(t, repoPath, "init")
		gitTest(t, repoPath, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "--allow-empty", "-m", "initial")
		if i == 0 {
			gitTest(t, repoPath, "branch", "-M", "primary")
		}
		data, _ = json.Marshal(map[string]string{"RootID": root.ID, "Name": "Repo", "Path": repoPath})
		w = request(t, s, "POST", "/api/registration/repos", h, string(data))
		if w.Code != 201 {
			t.Fatalf("register Repo: %d %s", w.Code, w.Body.String())
		}
		var repo store.Repo
		if err := json.Unmarshal(w.Body.Bytes(), &repo); err != nil {
			t.Fatal(err)
		}
		repos = append(repos, repo)
	}
	return taskMutationFixture{s, h, root, repos, base}
}

func (f taskMutationFixture) call(t *testing.T, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	return request(t, f.s, method, path, f.h, string(data))
}

func TestTaskCreationAndOptionsUseExactIdentities(t *testing.T) {
	f := newTaskMutationFixture(t, 2)
	create := func(repo store.Repo, id, name, branch string, asIs bool) (string, string) {
		t.Helper()
		w := f.call(t, "POST", "/api/tasks/create", map[string]any{"RootID": f.root.ID, "RepoID": repo.ID, "TaskID": id, "Name": name, "Branch": branch, "AsIs": asIs})
		if w.Code != 201 {
			t.Fatalf("create %s: %d %s", id, w.Code, w.Body.String())
		}
		var result struct{ Path, Branch string }
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result.Path, result.Branch
	}
	p1, b1 := create(f.repos[0], "TASK-α", "名前 🧪", "feature/one", true)
	if b1 != "feature/one" {
		t.Fatalf("explicit branch = %q", b1)
	}
	p2, b2 := create(f.repos[0], "TASK-α", "名前 🧪", "second", false)
	if b2 != "tasks/TASK-α/second" {
		t.Fatalf("templated branch = %q", b2)
	}
	if got, want := gitTest(t, f.repos[0].Path, "rev-parse", b2), gitTest(t, f.repos[0].Path, "rev-parse", "primary"); got != want {
		t.Fatalf("default base ref %s, want primary %s", got, want)
	}
	p3, _ := create(f.repos[1], "TASK-α", "名前 🧪", "other", true)
	gitTest(t, f.repos[0].Path, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "--allow-empty", "-m", "base commit")
	gitTest(t, f.repos[0].Path, "branch", "base-only")
	createdBase := f.call(t, "POST", "/api/tasks/create", map[string]any{"RootID": f.root.ID, "RepoID": f.repos[0].ID, "TaskID": "TASK-α", "Name": "名前 🧪", "Branch": "from-base", "Base": "base-only", "AsIs": true})
	if createdBase.Code != 201 {
		t.Fatalf("create from base: %d %s", createdBase.Code, createdBase.Body.String())
	}
	if got, want := gitTest(t, f.repos[0].Path, "rev-parse", "from-base"), gitTest(t, f.repos[0].Path, "rev-parse", "base-only"); got != want {
		t.Fatalf("new branch ref %s, want base %s", got, want)
	}
	gitTest(t, f.repos[0].Path, "branch", "existing/verbatim")
	reused := f.call(t, "POST", "/api/tasks/create", map[string]any{"RootID": f.root.ID, "RepoID": f.repos[0].ID, "TaskID": "TASK-α", "Name": "名前 🧪", "Branch": "existing/verbatim", "AsIs": true})
	if reused.Code != 201 || !strings.Contains(reused.Body.String(), `"Branch":"existing/verbatim"`) {
		t.Fatalf("existing branch was not reused verbatim: %d %s", reused.Code, reused.Body.String())
	}
	for _, p := range []string{p1, p2, p3} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("worktree %s: %v", p, err)
		}
	}
	if p1 == p2 || p2 == p3 || !strings.HasPrefix(p1, f.base) {
		t.Fatalf("unexpected opaque worktree paths: %q %q %q", p1, p2, p3)
	}
	q := "/api/tasks/options?root=" + url.QueryEscape(f.root.ID) + "&repo=" + url.QueryEscape(f.repos[0].ID)
	w := request(t, f.s, "GET", q, authorizedHeaders(f.s), "")
	if w.Code != 200 {
		t.Fatalf("options: %d %s", w.Code, w.Body.String())
	}
	var options struct {
		Branches                                []string
		CurrentBranch, TasksDir, BranchTemplate string
	}
	if err := json.Unmarshal(w.Body.Bytes(), &options); err != nil {
		t.Fatal(err)
	}
	if options.CurrentBranch != "primary" || options.TasksDir != f.base || options.BranchTemplate != "tasks/{task-id}/{branch}" {
		t.Fatalf("options = %+v", options)
	}
	if len(options.Branches) < 3 {
		t.Fatalf("options omit created branches: %+v", options.Branches)
	}
}

func TestTaskCreationInvalidBaseReportsDurableRecovery(t *testing.T) {
	f := newTaskMutationFixture(t, 1)
	w := f.call(t, "POST", "/api/tasks/create", map[string]any{"RootID": f.root.ID, "RepoID": f.repos[0].ID, "TaskID": "BROKEN", "Name": "Broken", "Branch": "broken", "Base": "missing-base", "AsIs": true})
	if w.Code < 400 {
		t.Fatalf("invalid base succeeded: %d %s", w.Code, w.Body.String())
	}
	var got struct {
		Error apiError `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.Error.Message, "tasky recover") || !strings.Contains(got.Error.Message, "pending") {
		t.Fatalf("error lacks recovery details: %s", got.Error.Message)
	}
	if !strings.Contains(got.Error.Message, "creation ") || !strings.Contains(got.Error.Message, "recover") {
		t.Fatalf("error lacks creation ID and recovery guidance: %s", got.Error.Message)
	}
	db, err := sql.Open("sqlite", app.DatabasePath(f.s.configPath))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var pendingID, pendingPath string
	if err := db.QueryRow("SELECT id, path FROM creations WHERE root_id = ?", f.root.ID).Scan(&pendingID, &pendingPath); err != nil {
		t.Fatalf("pending creation not durable: %v", err)
	}
	if !strings.Contains(got.Error.Message, pendingID) || !strings.Contains(got.Error.Message, pendingPath) {
		t.Fatalf("error/journal mismatch: %q; creation %s at %s", got.Error.Message, pendingID, pendingPath)
	}
	var completed int
	if err := db.QueryRow("SELECT count(*) FROM worktrees WHERE path = ?", pendingPath).Scan(&completed); err != nil {
		t.Fatal(err)
	}
	if completed != 0 {
		t.Fatalf("failed create has completed mapping for %s", pendingPath)
	}
}

func TestTaskMutationValidationAndRenameIdentity(t *testing.T) {
	f := newTaskMutationFixture(t, 1)
	path, wantBranch := "", ""
	w := f.call(t, "POST", "/api/tasks/create", map[string]any{"RootID": f.root.ID, "RepoID": f.repos[0].ID, "TaskID": "foo/bar", "Name": "old", "Branch": "feature/opaque", "AsIs": true})
	if w.Code != 201 {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	var created struct{ Path, Branch string }
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	path, wantBranch = created.Path, created.Branch
	otherPathResponse := f.call(t, "POST", "/api/tasks/create", map[string]any{"RootID": f.root.ID, "RepoID": f.repos[0].ID, "TaskID": "foo-bar", "Name": "other", "Branch": "feature/other", "AsIs": true})
	if otherPathResponse.Code != 201 {
		t.Fatalf("create similar Task ID: %d %s", otherPathResponse.Code, otherPathResponse.Body.String())
	}
	var otherPath struct{ Path string }
	_ = json.Unmarshal(otherPathResponse.Body.Bytes(), &otherPath)
	if otherPath.Path == path || !strings.HasPrefix(path, f.base) || !strings.HasPrefix(otherPath.Path, f.base) {
		t.Fatalf("opaque paths collided or escaped Tasks Directory: %q %q", path, otherPath.Path)
	}
	db, err := store.Open(app.DatabasePath(f.s.configPath))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	tasks, err := db.ListTasks(f.root.ID)
	if err != nil {
		t.Fatal(err)
	}
	var internalID string
	for _, task := range tasks {
		if task.TaskID == "foo/bar" {
			internalID = task.ID
		}
	}
	if internalID == "" {
		t.Fatal("created Task missing from store")
	}
	w = f.call(t, "POST", "/api/tasks/create", map[string]any{"RootID": f.root.ID, "RepoID": f.repos[0].ID, "TaskID": "foo-bar", "Name": "other", "Branch": "feature/opaque", "AsIs": true})
	if w.Code < 400 {
		t.Fatalf("branch checked out twice accepted: %d", w.Code)
	}
	beforeMappings, err := db.ListWorktrees(internalID)
	if err != nil {
		t.Fatal(err)
	}
	beforeGit := gitTest(t, f.repos[0].Path, "show-ref") + gitTest(t, f.repos[0].Path, "worktree", "list", "--porcelain")
	w = f.call(t, "POST", "/api/tasks/rename", map[string]any{"RootID": f.root.ID, "ID": internalID, "Name": "renamed"})
	if w.Code != 200 {
		t.Fatalf("rename: %d %s", w.Code, w.Body.String())
	}
	afterMappings, err := db.ListWorktrees(internalID)
	if err != nil || !reflect.DeepEqual(beforeMappings, afterMappings) {
		t.Fatalf("rename changed associations: %v; before=%+v after=%+v", err, beforeMappings, afterMappings)
	}
	if afterGit := gitTest(t, f.repos[0].Path, "show-ref") + gitTest(t, f.repos[0].Path, "worktree", "list", "--porcelain"); afterGit != beforeGit {
		t.Fatal("rename changed Git refs or worktrees")
	}
	var rows []struct {
		Task      store.Task `json:"Task"`
		Worktrees []struct {
			Mapping store.Worktree `json:"Mapping"`
		} `json:"Worktrees"`
	}
	w = request(t, f.s, "GET", "/api/tasks?root="+url.QueryEscape(f.root.ID), authorizedHeaders(f.s), "")
	if w.Code != 200 {
		t.Fatalf("tasks: %d %s", w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, row := range rows {
		if row.Task.ID == internalID {
			found = row.Task.Name == "renamed" && len(row.Worktrees) == 1 && row.Worktrees[0].Mapping.Path == path
		}
	}
	if !found {
		t.Fatalf("rename did not preserve identity/mapping: %+v", rows)
	}
	if got := strings.TrimSpace(gitTest(t, path, "branch", "--show-current")); got != wantBranch {
		t.Fatalf("rename changed Git branch: %q", got)
	}
	for _, body := range []string{`{`, `{"RootID":"x","RepoID":"x","TaskID":"x","Branch":"x","extra":true}`, `{"RootID":"x","RepoID":"x","TaskID":"x","Branch":"x"} {}`} {
		h := authorizedHeaders(f.s)
		h.Set("Content-Type", "application/json")
		if w := request(t, f.s, "POST", "/api/tasks/create", h, body); w.Code < 400 {
			t.Errorf("accepted invalid JSON %s", body)
		}
	}
	for _, query := range []string{"root=x&root=y&repo=z", "root=x&repo=z&extra=1", "root=x&repo=%zz", "root=x&repo=missing"} {
		if w := request(t, f.s, "GET", "/api/tasks/options?"+query, authorizedHeaders(f.s), ""); w.Code < 400 {
			t.Errorf("accepted query %q: %d", query, w.Code)
		}
	}
	other := newTaskMutationFixture(t, 0)
	w = other.call(t, "POST", "/api/tasks/rename", map[string]any{"RootID": other.root.ID, "ID": internalID, "Name": "wrong root"})
	if w.Code < 400 {
		t.Fatal("cross Root rename succeeded")
	}
	for _, missing := range []map[string]any{{"RepoID": "x", "TaskID": "x", "Branch": "x"}, {"RootID": "x", "TaskID": "x", "Branch": "x"}, {"RootID": "x", "RepoID": "x", "Branch": "x"}, {"RootID": "x", "RepoID": "x", "TaskID": "x"}} {
		if w := f.call(t, "POST", "/api/tasks/create", missing); w.Code < 400 {
			t.Errorf("accepted missing selectors: %v", missing)
		}
	}
	for _, selectors := range []map[string]any{
		{"RootID": f.root.Name, "RepoID": f.repos[0].ID, "TaskID": "name-selector", "Branch": "x"},
		{"RootID": f.root.ID, "RepoID": f.repos[0].Name, "TaskID": "name-selector", "Branch": "x"},
	} {
		if w := f.call(t, "POST", "/api/tasks/create", selectors); w.Code < 400 {
			t.Errorf("accepted display name as identity: %v", selectors)
		}
	}
	if w := request(t, f.s, "GET", "/api/tasks/options?root="+url.QueryEscape(f.root.Name)+"&repo="+url.QueryEscape(f.repos[0].ID), authorizedHeaders(f.s), ""); w.Code < 400 {
		t.Errorf("accepted Root name as identity: %d", w.Code)
	}
	invalid := f.call(t, "POST", "/api/tasks/create", map[string]any{"RootID": f.root.ID, "RepoID": f.repos[0].ID, "TaskID": "bad-branch", "Name": "bad", "Branch": "not a valid branch", "AsIs": true})
	if invalid.Code < 400 {
		t.Fatalf("invalid branch reported success: %d %s", invalid.Code, invalid.Body.String())
	}
	unauthorized := request(t, f.s, "POST", "/api/tasks/create", sameOrigin(f.s), `{"RootID":"x","RepoID":"x","TaskID":"x","Branch":"x"}`)
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated create status %d", unauthorized.Code)
	}
	wrongOrigin := authorizedHeaders(f.s)
	wrongOrigin.Set("Origin", "http://evil.test")
	if w := request(t, f.s, "POST", "/api/tasks/rename", wrongOrigin, `{"RootID":"x","ID":"x","Name":"x"}`); w.Code != http.StatusForbidden {
		t.Fatalf("cross-origin rename status %d", w.Code)
	}
	if w := request(t, f.s, "GET", "/api/tasks/options?root="+url.QueryEscape(f.root.ID)+"&repo="+url.QueryEscape(f.repos[0].ID), authorizedHeaders(f.s), ""); w.Code != http.StatusOK {
		t.Fatalf("options method setup: %d", w.Code)
	}
	if w := request(t, f.s, "PUT", "/api/tasks/create", authorizedHeaders(f.s), `{}`); w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("wrong method status %d", w.Code)
	}
}

func TestTaskMutationRenameRejectsWrongRootAndReusedPublicID(t *testing.T) {
	f := newTaskMutationFixture(t, 0)
	db, err := store.Open(app.DatabasePath(f.s.configPath))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	other, err := db.CreateRoot("Other", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	original, err := db.CreateTask(f.root.ID, "same-id", "Original")
	if err != nil {
		t.Fatal(err)
	}
	otherTask, err := db.CreateTask(other.ID, "same-id", "Other")
	if err != nil {
		t.Fatal(err)
	}
	if w := f.call(t, "POST", "/api/tasks/rename", map[string]any{"RootID": other.ID, "ID": original.ID, "Name": "Wrong"}); w.Code != 404 {
		t.Fatalf("cross-Root rename: %d %s", w.Code, w.Body.String())
	}
	if err := db.RemoveEmptyTask(original.ID); err != nil {
		t.Fatal(err)
	}
	replacement, err := db.CreateTask(f.root.ID, "same-id", "Replacement")
	if err != nil {
		t.Fatal(err)
	}
	if w := f.call(t, "POST", "/api/tasks/rename", map[string]any{"RootID": f.root.ID, "ID": original.ID, "Name": "Wrong"}); w.Code != 404 {
		t.Fatalf("stale identity renamed replacement: %d %s", w.Code, w.Body.String())
	}
	for _, expected := range []store.Task{replacement, otherTask} {
		current, err := db.FindTask(expected.RootID, expected.TaskID)
		if err != nil || current != expected {
			t.Fatalf("unselected Task changed: %+v %v", current, err)
		}
	}
}
