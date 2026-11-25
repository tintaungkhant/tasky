package store

import (
	"path/filepath"
	"testing"
)

func TestRemoveRootRollsBackAllMappingsOnFailure(t *testing.T) {
	s := openTestStore(t, filepath.Join(t.TempDir(), "db.sqlite3"))
	root, err := s.CreateRoot("root", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repo, err := s.CreateRepo(root.ID, "repo", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	task, err := s.CreateTask(root.ID, "T-1", "")
	if err != nil {
		t.Fatal(err)
	}
	wt, err := s.Associate(task.ID, repo.ID, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	// Fail the final deletion after the child records have been deleted.
	if _, err := s.db.Exec(`CREATE TRIGGER prevent_removal BEFORE DELETE ON roots BEGIN SELECT RAISE(ABORT, 'injected failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err := s.RemoveRoot(root.ID); err == nil {
		t.Fatal("expected removal failure")
	}
	roots, err := s.ListRoots()
	if err != nil || len(roots) != 1 || roots[0] != root {
		t.Fatalf("Roots: %+v %v", roots, err)
	}
	repos, err := s.ListRepos(root.ID)
	if err != nil || len(repos) != 1 || repos[0] != repo {
		t.Fatalf("Repos: %+v %v", repos, err)
	}
	tasks, err := s.ListTasks(root.ID)
	if err != nil || len(tasks) != 1 || tasks[0] != task {
		t.Fatalf("Tasks: %+v %v", tasks, err)
	}
	wts, err := s.ListWorktrees(task.ID)
	if err != nil || len(wts) != 1 || wts[0] != wt {
		t.Fatalf("worktrees: %+v %v", wts, err)
	}
}
