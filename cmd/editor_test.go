package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tintaungkhant/tasky/internal/app"
	"github.com/tintaungkhant/tasky/internal/config"
	"github.com/tintaungkhant/tasky/internal/web"
	"github.com/tintaungkhant/tasky/internal/work"
)

func TestStoredWorkspaceVerifiesEveryAssociation(t *testing.T) {
	_, repo, tasksDir := storedSetup(t)
	for _, branch := range []string{"one", "two"} {
		if _, err := storedExec(t, "add", "editor", branch); err != nil {
			t.Fatal(err)
		}
	}
	s := storedDB(t)
	roots, _ := s.ListRoots()
	task, _ := s.FindTask(roots[0].ID, "editor")
	wts, _ := s.ListWorktrees(task.ID)
	// Even a replacement worktree belonging to the same Repo is unsafe.
	runGit(t, repo, "worktree", "remove", wts[1].Path)
	runGit(t, repo, "worktree", "add", "-b", "replacement", wts[1].Path)
	_, err := storedExec(t, "vscode", "editor")
	if err == nil || !strings.Contains(err.Error(), "identity mismatch") {
		t.Fatalf("unverified workspace: %v", err)
	}
	path := filepath.Join(work.TaskDirectory(tasksDir, task.ID), task.ID+".code-workspace")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("published partial workspace: %v", err)
	}
}

func TestEditorLifecycleAndGitIndependence(t *testing.T) {
	root, repo, tasksDir := storedSetup(t)
	external := filepath.Join(t.TempDir(), "external")
	runGit(t, repo, "worktree", "add", "-b", "outside", external)
	for _, args := range [][]string{
		{"attach", "shared", external},
		{"add", "shared", "foo/bar", "--name", "Refund / ပြန်အမ်းခြင်း @ web"},
		{"add", "shared", "foo-bar"},
	} {
		if _, err := storedExec(t, args...); err != nil {
			t.Fatal(err)
		}
	}
	s := storedDB(t)
	roots, _ := s.ListRoots()
	repos, _ := s.ListRepos(roots[0].ID)
	task, _ := s.FindTask(roots[0].ID, "shared")
	nested := filepath.Join(root, "nested", "second")
	initTestRepo(t, nested)
	if _, err := registryRun(t, "repo", "add", "second", nested, "--root", "company"); err != nil {
		t.Fatal(err)
	}
	if _, err := storedExec(t, "add", "shared", "nested", "--repo", "second"); err != nil {
		t.Fatal(err)
	}
	otherRoot := t.TempDir()
	otherRepo := filepath.Join(otherRoot, "repo")
	initTestRepo(t, otherRepo)
	for _, args := range [][]string{
		{"root", "add", "other", otherRoot},
		{"repo", "add", "other", otherRepo, "--root", "other"},
	} {
		if _, err := registryRun(t, args...); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := storedExec(t, "add", "shared", "independent", "--root", "other", "--repo", "other"); err != nil {
		t.Fatal(err)
	}
	// Explicit selection from outside both Roots must still select only company.
	chdir(t, t.TempDir())
	if _, err := storedExec(t, "vscode", "shared", "--root", "company"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(work.TaskDirectory(tasksDir, task.ID), task.ID+".code-workspace")
	ws := readWorkspace(t, path)
	wts, _ := s.ListWorktrees(task.ID)
	if len(ws.Folders) != 4 || len(wts) != 4 {
		t.Fatalf("folders %+v; mappings %+v", ws, wts)
	}
	for _, wt := range wts {
		found := false
		for _, f := range ws.Folders {
			resolved := filepath.FromSlash(f.Path)
			if !filepath.IsAbs(resolved) {
				resolved = filepath.Join(filepath.Dir(path), resolved)
			}
			if app.SameLocation(resolved, wt.Path) {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing folder %s", wt.Path)
		}
	}
	if _, err := storedExec(t, "vscode", "shared", "--root", "company"); err != nil {
		t.Fatal(err)
	}
	// Repo IDs accepted by delete must also work for branch completion.
	if err := deleteCmd.Flags().Set("root", "company"); err != nil {
		t.Fatal(err)
	}
	got, _ := completeDeleteArgs(deleteCmd, []string{"shared", repos[0].ID}, "out")
	if len(got) != 1 || got[0] != "outside" {
		t.Fatalf("Repo ID completion: %v", got)
	}
	if _, err := storedExec(t, "rename", "shared", "Renamed / Task", "--root", "company"); err != nil {
		t.Fatal(err)
	}
	if _, err := storedExec(t, "vscode", "shared", "--root", "company"); err != nil {
		t.Fatal(err)
	}
	// User workspace settings survive Tasky cleanup of every associated checkout.
	edited := []byte("{\"folders\":[],\"settings\":{\"editor.fontSize\":17}}")
	if err := os.WriteFile(path, edited, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := storedExec(t, "detach", "shared", external, "--root", "company"); err != nil {
		t.Fatal(err)
	}
	if _, err := storedExec(t, "delete", "shared", "--root", "company"); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(path); string(data) != string(edited) {
		t.Fatal("cleanup changed editor settings")
	}
	// Tasky can be abandoned: an unavailable DB has no effect on ordinary Git.
	blocked := t.TempDir()
	if err := os.Mkdir(filepath.Join(blocked, "tasky.sqlite3"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TASKY_CONFIG", filepath.Join(blocked, "config.yaml"))
	if _, err := storedExec(t, "list", "shared", "--root", "company"); err == nil {
		t.Fatal("expected DB failure")
	}
	runGit(t, external, "switch", "-c", "after-tasky")
	if err := os.WriteFile(filepath.Join(external, "continued.txt"), []byte("ordinary Git"), 0600); err != nil {
		t.Fatal(err)
	}
	runGit(t, external, "add", "continued.txt")
	runGit(t, external, "commit", "-m", "Continue without Tasky")
	moved := filepath.Join(t.TempDir(), "moved")
	runGit(t, repo, "worktree", "move", external, moved)
	if data, _ := os.ReadFile(filepath.Join(moved, "continued.txt")); string(data) != "ordinary Git" {
		t.Fatal("Git checkout lost code")
	}
	runGit(t, moved, "status", "--porcelain")
	runGit(t, repo, "worktree", "remove", moved)
	runGit(t, repo, "show", "after-tasky:continued.txt")
}

func TestConcurrentWorkspacePublication(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "workspace.code-workspace")
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, content := range []string{"first complete file", "second complete file"} {
		wg.Add(1)
		go func() { defer wg.Done(); results <- app.PublishNewFile(path, []byte(content)) }()
	}
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		} else if !errors.Is(err, os.ErrExist) {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil || success != 1 || (string(data) != "first complete file" && string(data) != "second complete file") {
		t.Fatalf("publication: %q, %d successes, %v", data, success, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("temporary files remain: %+v %v", entries, err)
	}
}

func TestCLIAndDashboardConcurrentCreationAndRecovery(t *testing.T) {
	source, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	_, repo, _ := storedSetup(t)
	binary := filepath.Join(t.TempDir(), "tasky.exe")
	build := exec.Command("go", "build", "-o", binary, "..")
	build.Dir = source
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %s %v", out, err)
	}
	invoke := func(args ...string) (string, error) {
		command := exec.Command(binary, args...)
		command.Dir = repo
		out, err := command.CombinedOutput()
		return string(out), err
	}
	s := storedDB(t)
	roots, err := s.ListRoots()
	if err != nil || len(roots) != 1 {
		t.Fatalf("Roots: %+v %v", roots, err)
	}
	repos, err := s.ListRepos(roots[0].ID)
	if err != nil || len(repos) != 1 {
		t.Fatalf("Repos: %+v %v", repos, err)
	}
	post := dashboardClient(t, repo)
	var wg sync.WaitGroup
	type result struct {
		out string
		err error
	}
	results := make(chan result, 3)
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); out, err := invoke("add", "race", "racing"); results <- result{out, err} }()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		out, status, err := post("/api/tasks/create", map[string]string{
			"RootID": roots[0].ID, "RepoID": repos[0].ID, "TaskID": "race", "Branch": "racing",
		})
		if err == nil && status != http.StatusCreated {
			err = fmt.Errorf("dashboard status %d", status)
		}
		results <- result{out, err}
	}()
	wg.Wait()
	close(results)
	success := 0
	for r := range results {
		if r.err == nil {
			success++
		} else if !strings.Contains(r.out, "already") && !strings.Contains(r.out, "conflict") && !strings.Contains(r.out, "pending") {
			t.Fatalf("unexplained CLI/dashboard race failure: %s %v", r.out, r.err)
		}
	}
	if success != 1 {
		t.Fatalf("concurrent CLI/dashboard successes: %d", success)
	}
	task, err := s.FindTask(roots[0].ID, "race")
	if err != nil {
		t.Fatal(err)
	}
	wts, _ := s.ListWorktrees(task.ID)
	if len(wts) != 1 {
		t.Fatalf("duplicate mappings: %+v", wts)
	}
	// Persisted state at a process-exit boundary after Git and before mapping commit.
	path := filepath.Join(t.TempDir(), "interrupted")
	op, err := s.ReserveCreation(task.ID, repos[0].ID, "interrupted", path)
	if err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "worktree", "add", "-b", "interrupted", path)
	if out, err := invoke("pending"); err != nil || !strings.Contains(out, op.ID) {
		t.Fatalf("pending: %s %v", out, err)
	}
	if out, err := invoke("recover", op.ID); err != nil {
		t.Fatalf("recover: %s %v", out, err)
	}
	if out, err := invoke("list", "race"); err != nil || !strings.Contains(out, path) {
		t.Fatalf("recovered CLI: %s %v", out, err)
	}
	wts, _ = s.ListWorktrees(task.ID)
	if len(wts) != 2 {
		t.Fatalf("recovered mappings: %+v", wts)
	}
}

func dashboardClient(t *testing.T, cwd string) func(string, any) (string, int, error) {
	t.Helper()
	path, err := config.Path()
	if err != nil {
		t.Fatal(err)
	}
	service, err := web.New(path, cwd, 0)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- service.Serve(ctx) }()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("dashboard shutdown: %v", err)
		}
		service.Close()
	})
	access, err := url.Parse(service.URL())
	if err != nil {
		t.Fatal("invalid dashboard access URL")
	}
	token, err := url.ParseQuery(access.Fragment)
	if err != nil {
		t.Fatal("invalid dashboard access fragment")
	}
	origin := access.Scheme + "://" + access.Host
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Jar: jar, Timeout: 30 * time.Second}
	t.Cleanup(client.CloseIdleConnections)
	post := func(path string, body any) (string, int, error) {
		data, err := json.Marshal(body)
		if err != nil {
			return "", 0, err
		}
		req, err := http.NewRequest(http.MethodPost, origin+path, strings.NewReader(string(data)))
		if err != nil {
			return "", 0, err
		}
		req.Header.Set("Origin", origin)
		req.Header.Set("Content-Type", "application/json")
		response, err := client.Do(req)
		if err != nil {
			return "", 0, err
		}
		defer response.Body.Close()
		out, err := io.ReadAll(response.Body)
		return string(out), response.StatusCode, err
	}
	if out, status, err := post("/api/session", map[string]string{"token": token.Get("token")}); err != nil || status != http.StatusNoContent {
		t.Fatalf("dashboard authentication: %d %s %v", status, out, err)
	}
	return post
}

func TestLegacyWorkspacePreservesUserEditsAndIgnoresDirectories(t *testing.T) {
	_, repo, tasksDir := setupRootWithRepo(t)
	chdir(t, repo)
	if err := execAdd(t, "editor", "one"); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(tasksDir, "editor")
	if err := os.Mkdir(filepath.Join(dir, "notes"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := execVSCode(t, "editor"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "editor.code-workspace")
	if ws := readWorkspace(t, path); len(ws.Folders) != 1 {
		t.Fatalf("included arbitrary directory: %+v", ws)
	}
	content := []byte("// user settings\n{\"folders\": [], \"settings\": {}}\n")
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}
	if err := execVSCode(t, "editor"); err == nil {
		t.Fatal("overwrote user workspace")
	}
	if data, _ := os.ReadFile(path); string(data) != string(content) {
		t.Fatal("user edits changed")
	}
}
