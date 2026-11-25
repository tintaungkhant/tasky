package store

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
)

func openTestStore(t *testing.T, path string) *Store {
	t.Helper()
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	})
	return s
}

func TestPersistenceAndRootScopedIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "tasky.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	root, err := s.CreateRoot("company", filepath.Join(t.TempDir(), "root"))
	if err != nil {
		t.Fatal(err)
	}
	other, err := s.CreateRoot("personal", filepath.Join(t.TempDir(), "root"))
	if err != nil {
		t.Fatal(err)
	}
	task, err := s.CreateTask(root.ID, "42", "Refund / ပြန်အမ်းခြင်း @ web")
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.CreateTask(other.ID, "42", "")
	if err != nil {
		t.Fatal(err)
	}
	if task.ID == second.ID {
		t.Fatal("Tasks in different Roots share identity")
	}
	if _, err := s.CreateTask(root.ID, "42", "duplicate"); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate: %v", err)
	}
	repo, err := s.CreateRepo(root.ID, "api", filepath.Join(t.TempDir(), "repo"))
	if err != nil {
		t.Fatal(err)
	}
	repoB, err := s.CreateRepo(root.ID, "web", filepath.Join(t.TempDir(), "repo"))
	if err != nil {
		t.Fatal(err)
	}
	for i, repoID := range []string{repo.ID, repo.ID, repoB.ID} {
		if _, err := s.Associate(task.ID, repoID, filepath.Join(t.TempDir(), string(rune('a'+i)))); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s = openTestStore(t, path)
	roots, err := s.ListRoots()
	if err != nil || len(roots) != 2 {
		t.Fatalf("Roots after reopen: %v, %v", roots, err)
	}
	repos, err := s.ListRepos(root.ID)
	if err != nil || len(repos) != 2 || repos[0] != repo || repos[1] != repoB {
		t.Fatalf("Repos after reopen: %v, %v", repos, err)
	}
	tasks, err := s.ListTasks(root.ID)
	if err != nil || len(tasks) != 1 || tasks[0] != task {
		t.Fatalf("Tasks after reopen: %v, %v", tasks, err)
	}
	worktrees, err := s.ListWorktrees(task.ID)
	if err != nil || len(worktrees) != 3 {
		t.Fatalf("worktrees after reopen: %v, %v", worktrees, err)
	}
	child := exec.Command(os.Args[0], "-test.run=^TestReadInFreshProcess$")
	child.Env = append(os.Environ(), "TASKY_TEST_DB="+path, "TASKY_TEST_ROOT="+root.ID, "TASKY_TEST_TASK="+task.ID)
	if out, err := child.CombinedOutput(); err != nil {
		t.Fatalf("read in fresh process: %v\n%s", err, out)
	}
}

func TestReadInFreshProcess(t *testing.T) {
	path := os.Getenv("TASKY_TEST_DB")
	if path == "" {
		return
	}
	s := openTestStore(t, path)
	tasks, err := s.ListTasks(os.Getenv("TASKY_TEST_ROOT"))
	if err != nil || len(tasks) != 1 || tasks[0].ID != os.Getenv("TASKY_TEST_TASK") {
		t.Fatalf("persisted Tasks: %v, %v", tasks, err)
	}
	worktrees, err := s.ListWorktrees(tasks[0].ID)
	if err != nil || len(worktrees) != 3 {
		t.Fatalf("persisted worktrees: %v, %v", worktrees, err)
	}
}

func TestInterruptedWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasky.db")
	child := exec.Command(os.Args[0], "-test.run=^TestUncommittedProcess$")
	child.Env = append(os.Environ(), "TASKY_TEST_INTERRUPTED_DB="+path)
	if out, err := child.CombinedOutput(); err != nil {
		t.Fatalf("interrupted writer: %v\n%s", err, out)
	}
	s := openTestStore(t, path)
	roots, err := s.ListRoots()
	if err != nil || len(roots) != 0 {
		t.Fatalf("uncommitted write survived: %v, %v", roots, err)
	}
	if _, err := s.CreateRoot("retry", filepath.Join(t.TempDir(), "root")); err != nil {
		t.Fatalf("retry after interruption: %v", err)
	}
}

func TestUncommittedProcess(t *testing.T) {
	path := os.Getenv("TASKY_TEST_INTERRUPTED_DB")
	if path == "" {
		return
	}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := s.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec("INSERT INTO roots(id, name, path, path_key) VALUES('unfinished', 'unfinished', 'unfinished', 'unfinished')"); err != nil {
		t.Fatal(err)
	}
	// Exit without closing SQLite or rolling back, like an interrupted command.
	os.Exit(0)
}

func TestInvalidDatabaseAndSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invalid.db")
	original := []byte("not a SQLite database")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	if s, err := Open(path); err == nil {
		s.Close()
		t.Fatal("accepted invalid database")
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != string(original) {
		t.Fatalf("invalid database modified: %q, %v", got, err)
	}
	path = filepath.Join(t.TempDir(), "future.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec("PRAGMA user_version = 99"); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if s, err := Open(path); err == nil {
		s.Close()
		t.Fatal("accepted unsupported schema")
	}
}

func TestAssociationIntegrityAndRecovery(t *testing.T) {
	s := openTestStore(t, filepath.Join(t.TempDir(), "tasky.db"))
	root, err := s.CreateRoot("one", filepath.Join(t.TempDir(), "one"))
	if err != nil {
		t.Fatal(err)
	}
	other, err := s.CreateRoot("two", filepath.Join(t.TempDir(), "two"))
	if err != nil {
		t.Fatal(err)
	}
	task, err := s.CreateTask(root.ID, "42", "")
	if err != nil {
		t.Fatal(err)
	}
	another, err := s.CreateTask(root.ID, "43", "")
	if err != nil {
		t.Fatal(err)
	}
	repo, err := s.CreateRepo(root.ID, "api", filepath.Join(t.TempDir(), "repo"))
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := s.CreateRepo(other.ID, "web", filepath.Join(t.TempDir(), "repo"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "arbitrary-location")
	if _, err := s.Associate(task.ID, foreign.ID, path); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-Root association: %v", err)
	}
	if _, err := s.Associate("missing", repo.ID, path); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing Task: %v", err)
	}
	first, err := s.Associate(task.ID, repo.ID, path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Associate(another.ID, repo.ID, filepath.Join(path, ".")); !errors.Is(err, ErrConflict) {
		t.Fatalf("conflicting association: %v", err)
	}
	got, err := s.ListWorktrees(task.ID)
	if err != nil || len(got) != 1 || got[0] != first {
		t.Fatalf("existing association changed: %v, %v", got, err)
	}
	if _, err := s.Associate(another.ID, repo.ID, filepath.Join(t.TempDir(), "next")); err != nil {
		t.Fatalf("write after failure: %v", err)
	}
}

func TestConcurrentWriters(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasky.db")
	s := openTestStore(t, path)
	root, err := s.CreateRoot("company", filepath.Join(t.TempDir(), "root"))
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 8)
	for range 8 {
		wg.Go(func() {
			writer, err := Open(path)
			if err != nil {
				results <- err
				return
			}
			defer writer.Close()
			_, err = writer.CreateTask(root.ID, "42", "")
			results <- err
		})
	}
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		} else if !errors.Is(err, ErrConflict) {
			t.Errorf("unexpected write error: %v", err)
		}
	}
	tasks, err := s.ListTasks(root.ID)
	if successes != 1 || err != nil || len(tasks) != 1 {
		t.Fatalf("successes=%d Tasks=%v error=%v", successes, tasks, err)
	}
	repo, err := s.CreateRepo(root.ID, "api", filepath.Join(t.TempDir(), "repo"))
	if err != nil {
		t.Fatal(err)
	}
	worktreePath := filepath.Join(t.TempDir(), "worktree")
	results = make(chan error, 8)
	for range 8 {
		wg.Go(func() {
			writer := openConcurrent(path, results)
			if writer == nil {
				return
			}
			defer writer.Close()
			_, err := writer.Associate(tasks[0].ID, repo.ID, worktreePath)
			results <- err
		})
	}
	wg.Wait()
	close(results)
	successes = 0
	for err := range results {
		if err == nil {
			successes++
		} else if !errors.Is(err, ErrConflict) {
			t.Errorf("unexpected association error: %v", err)
		}
	}
	got, err := s.ListWorktrees(tasks[0].ID)
	if successes != 1 || err != nil || len(got) != 1 {
		t.Fatalf("successes=%d worktrees=%v error=%v", successes, got, err)
	}
}

func openConcurrent(path string, results chan<- error) *Store {
	s, err := Open(path)
	if err != nil {
		results <- err
	}
	return s
}
