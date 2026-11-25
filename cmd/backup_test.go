package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tintaungkhant/tasky/internal/config"
	"github.com/tintaungkhant/tasky/internal/store"
)

func TestBackupRestoreChecksLiveMappingsAndPreservesGit(t *testing.T) {
	root, repo, _ := storedSetup(t)
	for _, branch := range []string{"one", "two"} {
		if _, err := storedExec(t, "add", "T", branch); err != nil {
			t.Fatal(err)
		}
	}
	second := filepath.Join(root, "second")
	initTestRepo(t, second)
	s := storedDB(t)
	roots, _ := s.ListRoots()
	if _, err := registryRun(t, "repo", "add", "second", second, "--root", roots[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, err := storedExec(t, "add", "T", "three", "--repo", "second"); err != nil {
		t.Fatal(err)
	}
	task, _ := s.FindTask(roots[0].ID, "T")
	wts, _ := s.ListWorktrees(task.ID)
	if len(wts) != 3 {
		t.Fatalf("worktrees: %+v", wts)
	}
	backup := filepath.Join(t.TempDir(), "mappings.json")
	if _, err := storedExec(t, "backup", backup); err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(backup)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := storedExec(t, "backup", backup); err == nil {
		t.Fatal("overwrote backup")
	}
	if after, _ := os.ReadFile(backup); string(after) != string(original) {
		t.Fatal("changed existing backup")
	}
	// A moved checkout remains a stale mapping after restoration, never trusted.
	moved := filepath.Join(root, "moved")
	repos, _ := s.ListRepos(roots[0].ID)
	for _, wt := range wts {
		if strings.Contains(wt.Path, ".tasky") {
			for _, registered := range repos {
				if registered.ID == wt.RepoID {
					runGit(t, registered.Path, "worktree", "move", wt.Path, moved)
				}
			}
			break
		}
	}
	dirtyPath := filepath.Join(moved, "untracked.txt")
	if err := os.WriteFile(dirtyPath, []byte("keep source"), 0600); err != nil {
		t.Fatal(err)
	}
	before, secondBefore := gitSnapshot(t, repo), gitSnapshot(t, second)
	// Restore into a fresh database while leaving YAML and Git untouched.
	newConfig := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv("TASKY_CONFIG", newConfig)
	out, err := storedExec(t, "restore", backup)
	if err != nil || !strings.Contains(out, "[missing]") || !strings.Contains(out, "1 require review") {
		t.Fatalf("restore: %s %v", out, err)
	}
	path, _ := config.Path()
	target, err := store.Open(filepath.Join(filepath.Dir(path), "tasky.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	restoredTask, err := target.FindTask(roots[0].ID, "T")
	if err != nil || restoredTask.ID != task.ID {
		t.Fatalf("Task: %+v %v", restoredTask, err)
	}
	after, _ := target.ListWorktrees(task.ID)
	if len(after) != 3 {
		t.Fatalf("restored: %+v", after)
	}
	if _, err := storedExec(t, "restore", backup); err != nil {
		t.Fatal(err)
	}
	if gitSnapshot(t, repo) != before || gitSnapshot(t, second) != secondBefore {
		t.Fatal("backup/restore changed Git")
	}
	if _, err := os.Stat(newConfig); !os.IsNotExist(err) {
		t.Fatal("restore changed YAML")
	}
	if data, err := os.ReadFile(dirtyPath); err != nil || string(data) != "keep source" {
		t.Fatal("restore changed checkout files")
	}
}

func TestInvalidBackupAndUnavailableDatabase(t *testing.T) {
	_, repo, _ := storedSetup(t)
	path, _ := mappingDatabase()
	// An ordinary Git checkout remains usable when Tasky's database is unavailable.
	unavailable := filepath.Join(t.TempDir(), "blocked")
	if err := os.Mkdir(unavailable, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TASKY_CONFIG", filepath.Join(unavailable, "config.yaml"))
	db := filepath.Join(unavailable, "tasky.sqlite3")
	if err := os.Mkdir(db, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := storedExec(t, "list", "T"); err == nil || !strings.Contains(err.Error(), "Ordinary Git commands remain usable") {
		t.Fatalf("database error: %v (original %s)", err, path)
	}
	runGit(t, repo, "status", "--porcelain")
	file := filepath.Join(t.TempDir(), "invalid.json")
	if err := os.WriteFile(file, []byte(`{"version":1,"unknown":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := storedExec(t, "restore", file); err == nil || !strings.Contains(err.Error(), "invalid mapping backup") {
		t.Fatalf("invalid backup: %v", err)
	}
	for _, data := range []string{`{"version":1} {}`, `{"version":1`, `null`} {
		if err := os.WriteFile(file, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := storedExec(t, "restore", file); err == nil {
			t.Fatalf("accepted malformed backup %s", data)
		}
	}
}
