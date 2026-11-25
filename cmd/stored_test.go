package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/tintaungkhant/tasky/internal/config"
	"github.com/tintaungkhant/tasky/internal/git"
	"github.com/tintaungkhant/tasky/internal/store"
	"github.com/tintaungkhant/tasky/internal/work"
)

func storedSetup(t *testing.T) (string, string, string) {
	t.Helper()
	root, repo, tasks := setupRootWithRepo(t)
	chdir(t, repo)
	rootCmd.SetArgs([]string{"import"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatal(err)
	}
	return root, repo, tasks
}

func storedExec(t *testing.T, args ...string) (string, error) {
	t.Helper()
	addFromFlag, addNameFlag, listAllFlag, deleteForceFlag, deleteDeleteBranchFlag = "", "", false, false, false
	for _, cmd := range rootCmd.Commands() {
		for _, name := range []string{"root", "repo", "as-is", "name", "reassign-from"} {
			if f := cmd.Flags().Lookup(name); f != nil {
				_ = cmd.Flags().Set(name, f.DefValue)
			}
		}
	}
	t.Cleanup(func() {
		for _, cmd := range rootCmd.Commands() {
			for _, name := range []string{"root", "repo", "as-is", "name", "reassign-from"} {
				if f := cmd.Flags().Lookup(name); f != nil {
					_ = cmd.Flags().Set(name, f.DefValue)
				}
			}
		}
	})
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	rootCmd.SetArgs(args)
	err := rootCmd.Execute()
	return out.String(), err
}

func storedDB(t *testing.T) *store.Store {
	t.Helper()
	path, _ := config.Path()
	s, err := store.Open(filepath.Join(filepath.Dir(path), "tasky.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestStoredCreateAndRename(t *testing.T) {
	root, repo, tasksDir := storedSetup(t)
	name := "Refund / ပြန်အမ်းခြင်း @ web"
	if _, err := storedExec(t, "add", "T-1", "foo/bar", "--name", name); err != nil {
		t.Fatal(err)
	}
	if _, err := storedExec(t, "add", "T-1", "foo-bar"); err != nil {
		t.Fatal(err)
	}
	s := storedDB(t)
	roots, _ := s.ListRoots()
	tasks, _ := s.ListTasks(roots[0].ID)
	if len(tasks) != 1 || tasks[0].Name != name {
		t.Fatalf("Tasks: %+v", tasks)
	}
	wts, _ := s.ListWorktrees(tasks[0].ID)
	if len(wts) != 2 || wts[0].Path == wts[1].Path {
		t.Fatalf("worktrees: %+v", wts)
	}
	second := filepath.Join(root, "nested", "second")
	initTestRepo(t, second)
	if _, err := registryRun(t, "repo", "add", "second", second, "--root", roots[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, err := storedExec(t, "add", "T-1", "second-branch", "--root", roots[0].ID, "--repo", "second"); err != nil {
		t.Fatal(err)
	}
	wts, _ = s.ListWorktrees(tasks[0].ID)
	if len(wts) != 3 {
		t.Fatalf("multiple Repos: %+v", wts)
	}
	if _, err := storedExec(t, "import"); err != nil {
		t.Fatalf("repeat import after managed creation: %v", err)
	}
	tasksAfterImport, _ := s.ListTasks(roots[0].ID)
	if len(tasksAfterImport) != 1 {
		t.Fatalf("managed paths were reimported as legacy Tasks: %+v", tasksAfterImport)
	}
	for _, wt := range wts {
		rel, err := filepath.Rel(tasksDir, wt.Path)
		if err != nil || filepath.IsAbs(rel) || rel == ".." {
			t.Fatalf("escaped path: %s", wt.Path)
		}
	}
	before := gitSnapshot(t, repo)
	if _, err := storedExec(t, "rename", "T-1", "new / name"); err != nil {
		t.Fatal(err)
	}
	updated, _ := s.ListTasks(roots[0].ID)
	if len(updated) != 1 || updated[0].ID != tasks[0].ID || updated[0].Name != "new / name" {
		t.Fatalf("renamed Tasks: %+v", updated)
	}
	if after := gitSnapshot(t, repo); before != after {
		t.Fatal("rename changed Git state")
	}
	for _, wt := range wts {
		if _, err := os.Stat(wt.Path); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := storedExec(t, "list", "T-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := storedExec(t, "vscode", "T-1"); err != nil {
		t.Fatal(err)
	}
	got, _ := completeTaskID(listCmd, nil, "T-")
	if len(got) != 1 || got[0] != "T-1" {
		t.Fatalf("stored completion: %v", got)
	}
}

func TestStoredBranchModesAndPathSafety(t *testing.T) {
	_, repo, tasksDir := storedSetup(t)
	runGit(t, repo, "branch", "existing")
	for _, args := range [][]string{
		{"add", "../escape", "explicit", "--as-is", "--name", "name/with:punctuation"},
		{"add", "T-2", "existing"},
		{"add", "T-2", "new"},
	} {
		if _, err := storedExec(t, args...); err != nil {
			t.Fatal(err)
		}
	}
	s := storedDB(t)
	roots, _ := s.ListRoots()
	tasks, _ := s.ListTasks(roots[0].ID)
	for _, task := range tasks {
		wts, _ := s.ListWorktrees(task.ID)
		for _, wt := range wts {
			rel, err := filepath.Rel(tasksDir, wt.Path)
			if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				t.Fatalf("path escape: %s", wt.Path)
			}
		}
	}
	branches, _ := git.ListLocalBranches(repo)
	for _, want := range []string{"existing", "explicit", "task/T-2/new"} {
		found := false
		for _, branch := range branches {
			if branch == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("branches: %v missing %s", branches, want)
		}
	}
	if _, err := storedExec(t, "add", "T-3", "invalid branch"); err == nil || !strings.Contains(err.Error(), "fatal:") {
		t.Fatalf("Git stderr lost: %v", err)
	}
	if _, err := s.FindTask(roots[0].ID, "T-3"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("invalid branch recorded Task: %v", err)
	}
	newTask, err := s.CreateTask(roots[0].ID, "collision", "")
	if err != nil {
		t.Fatal(err)
	}
	protected := work.TaskDirectory(tasksDir, newTask.ID)
	if err := os.WriteFile(protected, []byte("preserve"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := storedExec(t, "add", "collision", "no-overwrite"); err == nil {
		t.Fatal("existing file accepted as Task directory")
	}
	if data, err := os.ReadFile(protected); err != nil || string(data) != "preserve" {
		t.Fatal("existing file overwritten")
	}
}

func TestStoredCreationConcurrencyAndRecovery(t *testing.T) {
	_, repoPath, tasksDir := storedSetup(t)
	a, b := storedDB(t), storedDB(t)
	roots, _ := a.ListRoots()
	repos, _ := a.ListRepos(roots[0].ID)
	m := work.Manager{Store: a}
	task, err := m.EnsureTask(roots[0].ID, "concurrent", "")
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, s := range []*store.Store{a, b} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := (work.Manager{Store: s}).Create(task, repos[0], tasksDir, "racing", "main")
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	success, failures := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else {
			failures++
		}
	}
	if success != 1 || failures != 1 {
		t.Fatalf("success %d, failures %d", success, failures)
	}
	wts, _ := a.ListWorktrees(task.ID)
	if len(wts) != 1 {
		t.Fatalf("duplicate association: %+v", wts)
	}
	// Simulate a process exit after intent, then after Git but before completion.
	path := filepath.Join(t.TempDir(), "recoverable")
	op, err := a.ReserveCreation(task.ID, repos[0].ID, "recoverable", path)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Recover(op, repos[0]); err == nil {
		t.Fatal("uncreated worktree recovered")
	}
	runGit(t, repoPath, "worktree", "add", "-b", "recoverable", path)
	if err := (work.Manager{Store: b}).Recover(op, repos[0]); err != nil {
		t.Fatal(err)
	}
	wts, _ = a.ListWorktrees(task.ID)
	if len(wts) != 2 {
		t.Fatalf("recovered mappings: %+v", wts)
	}
	before := gitSnapshot(t, repoPath)
	op, err = a.ReserveCreation(task.ID, repos[0].ID, "abandoned", filepath.Join(t.TempDir(), "absent"))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.ForgetCreation(op.ID); err != nil {
		t.Fatal(err)
	}
	if after := gitSnapshot(t, repoPath); before != after {
		t.Fatal("forget changed Git state")
	}
}
