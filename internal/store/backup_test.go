package store

import (
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

func TestMappingBackupRoundTripAndConflicts(t *testing.T) {
	base := t.TempDir()
	source := openTestStore(t, filepath.Join(base, "source"))
	root, _ := source.CreateRoot("root", base)
	a, _ := source.CreateRepo(root.ID, "a", filepath.Join(base, "a"))
	b, _ := source.CreateRepo(root.ID, "b", filepath.Join(base, "b"))
	task, _ := source.CreateTask(root.ID, "42", "Refund / ပြန်အမ်းခြင်း @ web")
	for i, repo := range []Repo{a, a, b} {
		if _, err := source.Attach(root.ID, task.TaskID, "", repo.ID, filepath.Join(base, []string{"wt-a", "wt-b", "wt-c"}[i]), "", []string{"id-a", "id-b", "id-c"}[i]); err != nil {
			t.Fatal(err)
		}
	}
	snapshot, err := source.Backup()
	if err != nil {
		t.Fatal(err)
	}
	target := openTestStore(t, filepath.Join(base, "target"))
	if err := target.Restore(snapshot); err != nil {
		t.Fatal(err)
	}
	if err := target.Restore(snapshot); err != nil {
		t.Fatalf("idempotent restore: %v", err)
	}
	restored, err := target.Backup()
	if err != nil || !reflect.DeepEqual(snapshot, restored) {
		t.Fatalf("round trip: %+v %v", restored, err)
	}
	conflict := snapshot
	conflict.Tasks = append([]Task(nil), snapshot.Tasks...)
	conflict.Tasks[0].Name = "changed"
	if err := target.Restore(conflict); err == nil {
		t.Fatal("overwrote existing Task")
	}
	after, _ := target.Backup()
	if !reflect.DeepEqual(restored, after) {
		t.Fatal("failed restore changed mappings")
	}
	invalid := snapshot
	invalid.Version = 99
	if err := target.Restore(invalid); err == nil {
		t.Fatal("accepted unsupported backup")
	}
	// A failure late in the transaction must roll back earlier inserted Roots.
	empty := openTestStore(t, filepath.Join(base, "empty"))
	if _, err := empty.db.Exec(`CREATE TRIGGER fail_restore BEFORE INSERT ON worktrees BEGIN SELECT RAISE(ABORT, 'injected'); END`); err != nil {
		t.Fatal(err)
	}
	if err := empty.Restore(snapshot); err == nil {
		t.Fatal("expected injected failure")
	}
	roots, _ := empty.ListRoots()
	if len(roots) != 0 {
		t.Fatal("partial restore committed")
	}
}

func TestConcurrentRestoreAndPendingProtection(t *testing.T) {
	base := t.TempDir()
	a := openTestStore(t, filepath.Join(base, "db"))
	b := openTestStore(t, filepath.Join(base, "db"))
	root, _ := a.CreateRoot("root", base)
	repo, _ := a.CreateRepo(root.ID, "repo", base)
	task, _ := a.CreateTask(root.ID, "T", "")
	snapshot, _ := a.Backup()
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, s := range []*Store{a, b} {
		wg.Add(1)
		go func() { defer wg.Done(); results <- s.Restore(snapshot) }()
	}
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err := a.ReserveCreation(task.ID, repo.ID, "branch", filepath.Join(base, "pending")); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Backup(); err == nil {
		t.Fatal("backup omitted active intent")
	}
	if err := b.Restore(snapshot); err == nil {
		t.Fatal("restore raced active operation")
	}
}

func TestBackupRejectsInvalidReferencesAndCompetingRestore(t *testing.T) {
	base := t.TempDir()
	source := openTestStore(t, filepath.Join(base, "source"))
	root, _ := source.CreateRoot("root", base)
	repo, _ := source.CreateRepo(root.ID, "repo", base)
	task, _ := source.CreateTask(root.ID, "T", "first")
	if _, err := source.Associate(task.ID, repo.ID, filepath.Join(base, "wt")); err != nil {
		t.Fatal(err)
	}
	snapshot, _ := source.Backup()
	invalid := snapshot
	invalid.Worktrees = append([]Worktree(nil), snapshot.Worktrees...)
	invalid.Worktrees[0].TaskID = "absent"
	if err := invalid.Validate(); err == nil {
		t.Fatal("accepted dangling reference")
	}
	invalid = snapshot
	invalid.Roots = append(append([]Root(nil), snapshot.Roots...), root)
	if err := invalid.Validate(); err == nil {
		t.Fatal("accepted duplicate identity")
	}
	a := openTestStore(t, filepath.Join(base, "target"))
	b := openTestStore(t, filepath.Join(base, "target"))
	other := snapshot
	other.Tasks = append([]Task(nil), snapshot.Tasks...)
	other.Tasks[0].Name = "second"
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for i, s := range []*Store{a, b} {
		wg.Add(1)
		go func() { defer wg.Done(); results <- s.Restore([]MappingBackup{snapshot, other}[i]) }()
	}
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("competing successful restores: %d", successes)
	}
	tasks, _ := a.ListTasks(root.ID)
	if len(tasks) != 1 || tasks[0].ID != task.ID {
		t.Fatalf("inconsistent Tasks: %+v", tasks)
	}
}
