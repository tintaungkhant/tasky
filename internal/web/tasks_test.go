package web

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/tintaungkhant/tasky/internal/app"
	"github.com/tintaungkhant/tasky/internal/git"
	"github.com/tintaungkhant/tasky/internal/store"
)

func TestTasksInspectionIsRootScopedAndRefreshesGitState(t *testing.T) {
	s := newTestServer(t)
	db, err := store.Open(app.DatabasePath(s.configPath))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rootA, err := db.CreateRoot("Alpha", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	rootB, err := db.CreateRoot("Beta", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	makeRepo := func(name string) (store.Repo, string) {
		t.Helper()
		path := filepath.Join(t.TempDir(), "repo")
		if err := os.Mkdir(path, 0755); err != nil {
			t.Fatal(err)
		}
		gitTest(t, path, "init")
		gitTest(t, path, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "--allow-empty", "-m", "initial")
		repo, err := db.CreateRepo(rootA.ID, name, path)
		if err != nil {
			t.Fatal(err)
		}
		return repo, path
	}
	repo, source := makeRepo("Repo")
	linked := filepath.Join(t.TempDir(), "checkout")
	gitTest(t, source, "worktree", "add", "-b", "external/feature", linked)
	identity, err := git.WorktreeIdentity(linked)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Attach(rootA.ID, "same-id", "東京 🐈", repo.ID, linked, "", identity); err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreateTask(rootB.ID, "same-id", "別 Root"); err != nil {
		t.Fatal(err)
	}
	// Also cover a stale mapping whose checkout has been replaced: it must not
	// be trusted merely because its path is still a Git worktree.
	otherRepo, _ := makeRepo("Other")
	missing := filepath.Join(t.TempDir(), "missing")
	if _, err := db.Attach(rootA.ID, "second", "Missing", otherRepo.ID, missing, ""); err != nil {
		t.Fatal(err)
	}
	invalid := filepath.Join(t.TempDir(), "not-a-worktree")
	if err := os.Mkdir(invalid, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Attach(rootA.ID, "third", "Invalid", otherRepo.ID, invalid, ""); err != nil {
		t.Fatal(err)
	}
	detached := filepath.Join(t.TempDir(), "detached")
	gitTest(t, source, "worktree", "add", "--detach", detached)
	detachedID, err := git.WorktreeIdentity(detached)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Attach(rootA.ID, "fourth", "Detached", repo.ID, detached, "", detachedID); err != nil {
		t.Fatal(err)
	}
	unverified := filepath.Join(t.TempDir(), "unverified")
	gitTest(t, source, "worktree", "add", "-b", "unverified", unverified)
	if _, err := db.Attach(rootA.ID, "fifth", "Unverified", repo.ID, unverified, ""); err != nil {
		t.Fatal(err)
	}
	replacement := filepath.Join(t.TempDir(), "replacement")
	gitTest(t, source, "worktree", "add", "-b", "original", replacement)
	replacementID, err := git.WorktreeIdentity(replacement)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Attach(rootA.ID, "sixth", "Replaced", repo.ID, replacement, "", replacementID); err != nil {
		t.Fatal(err)
	}
	gitTest(t, source, "worktree", "remove", "--force", replacement)
	if err := os.Mkdir(replacement, 0755); err != nil {
		t.Fatal(err)
	}
	gitTest(t, replacement, "init")
	if _, err := db.CreateTask(rootA.ID, "empty", "No checkout"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(linked, "untracked.txt"), []byte("dirty"), 0600); err != nil {
		t.Fatal(err)
	}
	h := sameOrigin(s)
	h.Set("Cookie", s.cookieName+"="+s.sessionToken)
	before, err := db.RootMappings(rootA.ID)
	if err != nil {
		t.Fatal(err)
	}
	path := "/api/tasks?root=" + rootA.ID
	w := request(t, s, http.MethodGet, path, h, "")
	if w.Code != 200 {
		t.Fatalf("GET tasks: %d %s", w.Code, w.Body.String())
	}
	var got []struct {
		Task      store.Task
		Worktrees []struct {
			Mapping store.Worktree
			Repo    store.Repo
			State   struct{ Status, Branch, Changes, Detail string }
		}
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 7 {
		t.Fatalf("got %d Tasks, want 7", len(got))
	}
	if got[0].Task.TaskID != "empty" || got[0].Worktrees == nil {
		t.Fatalf("empty Task not represented with array: %#v", got[0])
	}
	var observed bool
	for _, task := range got {
		if task.Task.TaskID == "same-id" {
			if task.Task.RootID != rootA.ID || task.Task.Name != "東京 🐈" || len(task.Worktrees) != 1 {
				t.Fatalf("wrong root Task: %#v", task)
			}
			state := task.Worktrees[0].State
			if state.Status != "ok" || state.Branch != "external/feature" || state.Changes == "" {
				t.Fatalf("Git state was not refreshed: %#v", state)
			}
			observed = true
		}
		for _, wt := range task.Worktrees {
			if wt.Mapping.Path == missing && wt.State.Status != "missing" {
				t.Fatalf("missing checkout status = %#v", wt.State)
			}
			if wt.Mapping.Path == invalid && wt.State.Status != "invalid" {
				t.Fatalf("invalid checkout status = %#v", wt.State)
			}
			if wt.Mapping.Path == detached && (wt.State.Status != "ok" || wt.State.Branch != "(detached HEAD)") {
				t.Fatalf("detached checkout status = %#v", wt.State)
			}
			if wt.Mapping.Path == unverified && wt.State.Status != "unverified" {
				t.Fatalf("unverified checkout status = %#v", wt.State)
			}
			if wt.Mapping.Path == replacement && wt.State.Status != "invalid" {
				t.Fatalf("replacement checkout status = %#v", wt.State)
			}
		}
	}
	if !observed {
		t.Fatal("Root-scoped Task missing")
	}
	after, err := db.RootMappings(rootA.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("inspection changed persisted Root mappings")
	}
	for _, query := range []string{"", "root=", "root=" + rootA.ID + "&root=" + rootB.ID, "root=" + rootA.ID + "&name=x", "root=Alpha", "root=unknown"} {
		status := 400
		if query == "root=unknown" || query == "root=Alpha" {
			status = 404
		}
		w := request(t, s, http.MethodGet, "/api/tasks?"+query, h, "")
		if w.Code != status {
			t.Errorf("query %q status %d, want %d: %s", query, w.Code, status, w.Body.String())
		}
	}
	if w := request(t, s, http.MethodGet, path, nil, ""); w.Code != 401 {
		t.Errorf("unauthenticated status %d", w.Code)
	}
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		if w := request(t, s, method, path, h, "{}"); w.Code != 405 {
			t.Errorf("%s status %d, want 405", method, w.Code)
		}
	}
}

func gitTest(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
	return string(output)
}
