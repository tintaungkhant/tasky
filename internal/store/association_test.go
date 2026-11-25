package store

import (
	"errors"
	"path/filepath"
	"sync"
	"testing"
)

func TestAssociationConcurrencyAndStaleRequests(t *testing.T) {
	base := t.TempDir()
	a, b := openTestStore(t, filepath.Join(base, "db.sqlite3")), openTestStore(t, filepath.Join(base, "db.sqlite3"))
	root, err := a.CreateRoot("root", base)
	if err != nil {
		t.Fatal(err)
	}
	repo, err := a.CreateRepo(root.ID, "repo", filepath.Join(base, "repo"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(base, "worktree")
	var wg sync.WaitGroup
	type result struct {
		Worktree
		err error
	}
	results := make(chan result, 2)
	for _, s := range []*Store{a, b} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			wt, err := s.Attach(root.ID, "T-1", "original", repo.ID, path, "")
			results <- result{wt, err}
		}()
	}
	wg.Wait()
	close(results)
	var source Worktree
	for r := range results {
		if r.err != nil {
			t.Fatal(r.err)
		}
		if source.ID != "" && source != r.Worktree {
			t.Fatalf("duplicate attach: %+v %+v", source, r.Worktree)
		}
		source = r.Worktree
	}
	results = make(chan result, 2)
	for i, s := range []*Store{a, b} {
		id := []string{"T-2", "T-3"}[i]
		wg.Add(1)
		go func() {
			defer wg.Done()
			wt, err := s.Attach(root.ID, id, "target", repo.ID, path, source.TaskID)
			results <- result{wt, err}
		}()
	}
	wg.Wait()
	close(results)
	success, conflicts := 0, 0
	var winner Worktree
	for r := range results {
		if r.err == nil {
			success++
			winner = r.Worktree
		} else if errors.Is(r.err, ErrConflict) {
			conflicts++
		} else {
			t.Fatal(r.err)
		}
	}
	if success != 1 || conflicts != 1 {
		t.Fatalf("success=%d conflicts=%d", success, conflicts)
	}
	tasks, err := a.ListTasks(root.ID)
	if err != nil || len(tasks) != 2 {
		t.Fatalf("failed reassignment left a Task: %+v %v", tasks, err)
	}
	if err := b.Detach(root.ID, "T-1", path); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale detach: %v", err)
	}
	for _, task := range tasks {
		if task.ID != winner.TaskID {
			continue
		}
		for range 2 {
			if err := a.Detach(root.ID, task.TaskID, path); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := a.Attach(root.ID, task.TaskID, "", repo.ID, path, source.TaskID); !errors.Is(err, ErrConflict) {
			t.Fatalf("stale reassign after detach: %v", err)
		}
	}
}

func TestAssociationRollbackAndPendingCreation(t *testing.T) {
	base := t.TempDir()
	s := openTestStore(t, filepath.Join(base, "db.sqlite3"))
	root, err := s.CreateRoot("root", base)
	if err != nil {
		t.Fatal(err)
	}
	repo, err := s.CreateRepo(root.ID, "repo", filepath.Join(base, "repo"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(base, "worktree")
	if _, err := s.db.Exec(`CREATE TRIGGER fail_attach BEFORE INSERT ON worktrees BEGIN SELECT RAISE(ABORT, 'injected failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Attach(root.ID, "T-1", "", repo.ID, path, ""); err == nil {
		t.Fatal("expected failed attach")
	}
	if _, err := s.FindTask(root.ID, "T-1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("partial Task: %v", err)
	}
	if _, err := s.db.Exec("DROP TRIGGER fail_attach"); err != nil {
		t.Fatal(err)
	}
	wt, err := s.Attach(root.ID, "T-1", "original", repo.ID, path, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`CREATE TRIGGER fail_reassign BEFORE UPDATE ON worktrees BEGIN SELECT RAISE(ABORT, 'injected failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Attach(root.ID, "T-2", "", repo.ID, path, wt.TaskID); err == nil {
		t.Fatal("expected failed reassignment")
	}
	if _, err := s.FindTask(root.ID, "T-2"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("partial target Task: %v", err)
	}
	wts, err := s.ListWorktrees(wt.TaskID)
	if err != nil || len(wts) != 1 || wts[0] != wt {
		t.Fatalf("source lost: %+v %v", wts, err)
	}
	if _, err := s.db.Exec("DROP TRIGGER fail_reassign"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Attach(root.ID, "T-2", "", repo.ID, path, wt.TaskID); err != nil {
		t.Fatal(err)
	}
	creation, err := s.ReserveCreation(wt.TaskID, repo.ID, "pending", filepath.Join(base, "pending"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Attach(root.ID, "T-3", "", repo.ID, creation.Path, ""); err == nil {
		t.Fatal("pending creation was bypassed")
	}
}

func TestReassignAcrossRootsRequiresExactSource(t *testing.T) {
	base := t.TempDir()
	s := openTestStore(t, filepath.Join(base, "db.sqlite3"))
	first, err := s.CreateRoot("first", filepath.Join(base, "first"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.CreateRoot("second", filepath.Join(base, "second"))
	if err != nil {
		t.Fatal(err)
	}
	var repos []Repo
	for _, root := range []Root{first, second} {
		repo, err := s.CreateRepo(root.ID, "shared", filepath.Join(base, "repo"))
		if err != nil {
			t.Fatal(err)
		}
		repos = append(repos, repo)
	}
	path := filepath.Join(base, "worktree")
	wt, err := s.Attach(first.ID, "same-ID", "", repos[0].ID, path, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Attach(second.ID, "same-ID", "", repos[1].ID, path, ""); !errors.Is(err, ErrConflict) {
		t.Fatalf("cross-Root reassignment implicit: %v", err)
	}
	moved, err := s.Attach(second.ID, "same-ID", "", repos[1].ID, path, wt.TaskID)
	if err != nil || moved.ID != wt.ID || moved.TaskID == wt.TaskID {
		t.Fatalf("reassigned: %+v %v", moved, err)
	}
}
