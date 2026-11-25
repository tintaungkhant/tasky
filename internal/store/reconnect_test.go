package store

import (
	"path/filepath"
	"sync"
	"testing"
)

func TestReconnectConcurrentAndRollback(t *testing.T) {
	base := t.TempDir()
	a := openTestStore(t, filepath.Join(base, "db"))
	b := openTestStore(t, filepath.Join(base, "db"))
	root, _ := a.CreateRoot("root", base)
	repo, _ := a.CreateRepo(root.ID, "repo", filepath.Join(base, "repo"))
	wt, err := a.Attach(root.ID, "T", "", repo.ID, filepath.Join(base, "old"), "", "identity")
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Reconnect(wt, filepath.Join(base, "bad"), "different"); err == nil {
		t.Fatal("identity changed")
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i, s := range []*Store{a, b} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- s.Reconnect(wt, filepath.Join(base, []string{"new-a", "new-b"}[i]), wt.Identity)
		}()
	}
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("successful concurrent moves: %d", success)
	}
	rows, _ := a.ListWorktrees(wt.TaskID)
	if len(rows) != 1 || rows[0].ID != wt.ID || rows[0].TaskID != wt.TaskID || rows[0].Identity != wt.Identity {
		t.Fatalf("mapping: %+v", rows)
	}
	if _, err := a.Attach(root.ID, "other", "", repo.ID, filepath.Join(base, "duplicate"), "", wt.Identity); err == nil {
		t.Fatal("duplicate identity")
	}
}
