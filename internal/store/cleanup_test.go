package store

import (
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
)

func TestCleanupSerializesAndPreservesIntent(t *testing.T) {
	base := t.TempDir()
	a := openTestStore(t, filepath.Join(base, "db"))
	b := openTestStore(t, filepath.Join(base, "db"))
	root, _ := a.CreateRoot("root", base)
	repo, _ := a.CreateRepo(root.ID, "repo", base)
	wt, err := a.Attach(root.ID, "T", "", repo.ID, filepath.Join(base, "wt"), "", "identity")
	if err != nil {
		t.Fatal(err)
	}
	op, err := a.ReserveCleanup(wt, "branch", "revision", true, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Attach(root.ID, "other", "", repo.ID, wt.Path, wt.TaskID, wt.Identity); err == nil {
		t.Fatal("reassigned reserved mapping")
	}
	if err := b.Reconnect(wt, filepath.Join(base, "new"), wt.Identity); err == nil {
		t.Fatal("reconnected reserved mapping")
	}
	if err := b.RemoveRoot(root.ID); err == nil {
		t.Fatal("discarded pending cleanup")
	}
	var called atomic.Int32
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, s := range []*Store{a, b} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- s.ExecuteCleanup(op.ID, false, false, func(c Cleanup) (bool, error) { called.Add(1); return true, nil })
		}()
	}
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		}
	}
	if successes != 1 || called.Load() != 1 {
		t.Fatalf("successes %d side effects %d", successes, called.Load())
	}
	rows, _ := a.ListWorktrees(wt.TaskID)
	ops, _ := a.ListCleanups(root.ID)
	if len(rows) != 0 || len(ops) != 0 {
		t.Fatalf("unfinished: %+v %+v", rows, ops)
	}
}
