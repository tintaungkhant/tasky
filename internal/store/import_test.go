package store

import (
	"path/filepath"
	"sync"
	"testing"
)

func TestImportConcurrentRetriesAndRollback(t *testing.T) {
	base := t.TempDir()
	path := filepath.Join(base, "db.sqlite3")
	a, b := openTestStore(t, path), openTestStore(t, path)
	plan := []ImportRoot{{Name: "root", Path: base, Repos: []ImportRepo{{Name: "repo", Path: filepath.Join(base, "repo")}}, Tasks: []ImportTask{{TaskID: "T-1", Worktrees: []ImportWorktree{{RepoPath: filepath.Join(base, "repo"), Path: filepath.Join(base, "wt")}}}}}}
	// Interrupt the transaction after Root, Repo, and Task inserts.
	if _, err := a.db.Exec(`CREATE TRIGGER fail_import BEFORE INSERT ON worktrees BEGIN SELECT RAISE(ABORT, 'injected interruption'); END`); err != nil {
		t.Fatal(err)
	}
	if err := a.Import(plan); err == nil {
		t.Fatal("expected failed import")
	}
	roots, err := a.ListRoots()
	if err != nil || len(roots) != 0 {
		t.Fatalf("partial import: %+v %v", roots, err)
	}
	if _, err := a.db.Exec("DROP TRIGGER fail_import"); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, s := range []*Store{a, b} {
		wg.Add(1)
		go func() { defer wg.Done(); results <- s.Import(plan) }()
	}
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}
	roots, err = a.ListRoots()
	if err != nil || len(roots) != 1 {
		t.Fatalf("duplicate Roots: %+v %v", roots, err)
	}
	tasks, err := a.ListTasks(roots[0].ID)
	if err != nil || len(tasks) != 1 {
		t.Fatalf("duplicate Tasks: %+v %v", tasks, err)
	}
	wts, err := a.ListWorktrees(tasks[0].ID)
	if err != nil || len(wts) != 1 {
		t.Fatalf("duplicate worktrees: %+v %v", wts, err)
	}
	plan[0].Tasks[0].Name = "conflicting name"
	if err := a.Import(plan); err == nil {
		t.Fatal("import silently replaced Task identity")
	}
}
