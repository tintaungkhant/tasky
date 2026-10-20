package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/tintaungkhant/tasky/internal/git"
)

func execDelete(t *testing.T, args ...string) error {
	t.Helper()
	deleteForceFlag = false
	deleteDeleteBranchFlag = false
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetArgs(append([]string{"delete"}, args...))
	return rootCmd.Execute()
}

func TestDeleteWholeTaskRemovesWorktreesAndFolder(t *testing.T) {
	_, repoPath, tasksDir := setupRootWithRepo(t)
	chdir(t, repoPath)

	if err := execAdd(t, "CRO-001", "feat/payment"); err != nil {
		t.Fatalf("tasky add: %v", err)
	}

	taskDir := filepath.Join(tasksDir, "CRO-001")
	if err := execDelete(t, "CRO-001"); err != nil {
		t.Fatalf("tasky delete: %v", err)
	}

	if _, err := os.Stat(taskDir); !os.IsNotExist(err) {
		t.Fatalf("task directory %s still exists after whole-task delete", taskDir)
	}

	branches, err := git.ListLocalBranches(repoPath)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, b := range branches {
		if b == "task/CRO-001/feat/payment" {
			found = true
		}
	}
	if !found {
		t.Fatal("branch should still exist: --delete-branch was not passed")
	}
}

func TestDeleteSingleRepoLeavesOtherWorktrees(t *testing.T) {
	root, repoPath, tasksDir := setupRootWithRepo(t)
	chdir(t, repoPath)

	repoBPath := filepath.Join(root, "project-b")
	initTestRepo(t, repoBPath)

	if err := execAdd(t, "CRO-002", "feat/x"); err != nil {
		t.Fatalf("tasky add (project-a): %v", err)
	}
	if err := execAdd(t, "project-b", "CRO-002", "feat/y"); err != nil {
		t.Fatalf("tasky add (project-b): %v", err)
	}

	if err := execDelete(t, "CRO-002", "project-a"); err != nil {
		t.Fatalf("tasky delete: %v", err)
	}

	taskDir := filepath.Join(tasksDir, "CRO-002")
	if _, err := os.Stat(filepath.Join(taskDir, "project-a@task-CRO-002-feat-x")); !os.IsNotExist(err) {
		t.Fatal("project-a worktree should have been removed")
	}
	if _, err := os.Stat(filepath.Join(taskDir, "project-b@task-CRO-002-feat-y", "README.md")); err != nil {
		t.Fatalf("project-b worktree should still exist: %v", err)
	}
}

func TestDeleteWithDeleteBranchFlagRemovesBranch(t *testing.T) {
	_, repoPath, _ := setupRootWithRepo(t)
	chdir(t, repoPath)

	if err := execAdd(t, "CRO-003", "feat/payment"); err != nil {
		t.Fatalf("tasky add: %v", err)
	}

	deleteForceFlag = false
	deleteDeleteBranchFlag = true
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetArgs([]string{"delete", "--delete-branch", "CRO-003"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("tasky delete: %v", err)
	}

	branches, err := git.ListLocalBranches(repoPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range branches {
		if b == "task/CRO-003/feat/payment" {
			t.Fatal("branch should have been deleted with --delete-branch")
		}
	}
}

func TestDeleteRepoWithMultipleWorktreesRequiresBranch(t *testing.T) {
	_, repoPath, _ := setupRootWithRepo(t)
	chdir(t, repoPath)

	if err := execAdd(t, "CRO-004", "feat/a"); err != nil {
		t.Fatalf("tasky add (a): %v", err)
	}
	if err := execAdd(t, "CRO-004", "feat/b"); err != nil {
		t.Fatalf("tasky add (b): %v", err)
	}

	err := execDelete(t, "CRO-004", "project-a")
	if err == nil {
		t.Fatal("expected error: repo has multiple worktrees under this task, branch required")
	}
}

func TestDeleteRepoAndBranchRemovesExactWorktree(t *testing.T) {
	_, repoPath, tasksDir := setupRootWithRepo(t)
	chdir(t, repoPath)

	if err := execAdd(t, "CRO-005", "feat/a"); err != nil {
		t.Fatalf("tasky add (a): %v", err)
	}
	if err := execAdd(t, "CRO-005", "feat/b"); err != nil {
		t.Fatalf("tasky add (b): %v", err)
	}

	if err := execDelete(t, "CRO-005", "project-a", "task/CRO-005/feat/a"); err != nil {
		t.Fatalf("tasky delete: %v", err)
	}

	taskDir := filepath.Join(tasksDir, "CRO-005")
	if _, err := os.Stat(filepath.Join(taskDir, "project-a@task-CRO-005-feat-a")); !os.IsNotExist(err) {
		t.Fatal("feat/a worktree should have been removed")
	}
	if _, err := os.Stat(filepath.Join(taskDir, "project-a@task-CRO-005-feat-b", "README.md")); err != nil {
		t.Fatalf("feat/b worktree should still exist: %v", err)
	}
}

func TestDeleteDirtyWorktreeRequiresForce(t *testing.T) {
	_, repoPath, _ := setupRootWithRepo(t)
	chdir(t, repoPath)

	if err := execAdd(t, "CRO-006", "feat/payment"); err != nil {
		t.Fatalf("tasky add: %v", err)
	}

	wtPath := filepath.Join(repoPath, "..", "tasks", "CRO-006", "project-a@task-CRO-006-feat-payment")
	if err := os.WriteFile(filepath.Join(wtPath, "dirty.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := execDelete(t, "CRO-006"); err == nil {
		t.Fatal("expected error deleting dirty worktree without --force")
	}

	deleteForceFlag = false
	deleteDeleteBranchFlag = false
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetArgs([]string{"delete", "--force", "CRO-006"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("tasky delete --force: %v", err)
	}
}

func TestDeleteTaskNotFound(t *testing.T) {
	_, repoPath, _ := setupRootWithRepo(t)
	chdir(t, repoPath)

	if err := execDelete(t, "does-not-exist"); err == nil {
		t.Fatal("expected error for unknown task ID")
	}
}

func TestDeleteRepoNotFoundUnderTask(t *testing.T) {
	_, repoPath, _ := setupRootWithRepo(t)
	chdir(t, repoPath)

	if err := execAdd(t, "CRO-007", "feat/x"); err != nil {
		t.Fatalf("tasky add: %v", err)
	}

	if err := execDelete(t, "CRO-007", "does-not-exist"); err == nil {
		t.Fatal("expected error for repo with no worktrees under this task")
	}
}
