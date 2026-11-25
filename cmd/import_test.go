package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tintaungkhant/tasky/internal/config"
	"github.com/tintaungkhant/tasky/internal/store"
)

func TestImportAmbiguousTaskRequiresReview(t *testing.T) {
	_, repo, tasksDir := setupRootWithRepo(t)
	chdir(t, repo)
	for _, name := range []string{"DUP@one", "DUP@two"} {
		if err := os.MkdirAll(filepath.Join(tasksDir, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	rootCmd.SetArgs([]string{"import"})
	if err := rootCmd.Execute(); err == nil || !strings.Contains(err.Error(), "review") {
		t.Fatalf("expected review error: %v", err)
	}
	path, _ := config.Path()
	if _, err := os.Stat(filepath.Join(filepath.Dir(path), "tasky.sqlite3")); !os.IsNotExist(err) {
		t.Fatalf("invalid plan wrote database: %v", err)
	}
}

func TestImportRejectsUnregisteredDirectory(t *testing.T) {
	_, repo, tasksDir := setupRootWithRepo(t)
	chdir(t, repo)
	path := filepath.Join(tasksDir, "T-1", "repo@fake")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	before := gitSnapshot(t, repo)
	rootCmd.SetArgs([]string{"import"})
	if err := rootCmd.Execute(); err == nil || !strings.Contains(err.Error(), "review") {
		t.Fatalf("unverified directory: %v", err)
	}
	if after := gitSnapshot(t, repo); after != before {
		t.Fatal("failed import changed Git state")
	}
}

func TestImportLegacyIsRepeatableAndPreservesWorkflow(t *testing.T) {
	_, repo, _ := setupRootWithRepo(t)
	chdir(t, repo)
	if err := execAdd(t, "OLD-1", "feature"); err != nil {
		t.Fatal(err)
	}
	before := gitSnapshot(t, repo)
	for range 2 {
		rootCmd.SetArgs([]string{"import"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatal(err)
		}
	}
	path, _ := config.Path()
	s, err := store.Open(filepath.Join(filepath.Dir(path), "tasky.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	roots, err := s.ListRoots()
	if err != nil || len(roots) != 1 {
		t.Fatalf("Roots: %+v %v", roots, err)
	}
	tasks, err := s.ListTasks(roots[0].ID)
	if err != nil || len(tasks) != 1 || tasks[0].TaskID != "OLD-1" {
		t.Fatalf("Tasks: %+v %v", tasks, err)
	}
	wts, err := s.ListWorktrees(tasks[0].ID)
	if err != nil || len(wts) != 1 {
		t.Fatalf("worktrees: %+v %v", wts, err)
	}
	if after := gitSnapshot(t, repo); after != before {
		t.Fatal("import changed Git state")
	}
	if _, err := execList(t, "OLD-1"); err != nil {
		t.Fatal(err)
	}
	if got, _ := completeTaskID(listCmd, nil, "OLD"); len(got) != 1 {
		t.Fatalf("completion: %v", got)
	}
	if err := execVSCode(t, "OLD-1"); err != nil {
		t.Fatal(err)
	}
	if err := execAdd(t, "OLD-1", "another"); err != nil {
		t.Fatal(err)
	}
	if err := execDelete(t, "OLD-1"); err != nil {
		t.Fatal(err)
	}
}
