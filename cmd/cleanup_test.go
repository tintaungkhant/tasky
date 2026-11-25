package cmd

import (
	"github.com/tintaungkhant/tasky/internal/git"
	"github.com/tintaungkhant/tasky/internal/work"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCleanupRejectsReplacementAndKeepsMapping(t *testing.T) {
	root, repo, _ := storedSetup(t)
	if _, err := storedExec(t, "add", "T", "original"); err != nil {
		t.Fatal(err)
	}
	s := storedDB(t)
	roots, _ := s.ListRoots()
	task, _ := s.FindTask(roots[0].ID, "T")
	wts, _ := s.ListWorktrees(task.ID)
	moved := filepath.Join(root, "moved")
	runGit(t, repo, "worktree", "move", wts[0].Path, moved)
	runGit(t, repo, "worktree", "add", "-b", "replacement", wts[0].Path)
	if _, err := storedExec(t, "delete", "T", "--force", "--delete-branch"); err == nil {
		t.Fatal("deleted a replacement worktree")
	}
	if _, err := os.Stat(filepath.Join(wts[0].Path, ".git")); err != nil {
		t.Fatal(err)
	}
	after, _ := s.ListWorktrees(task.ID)
	if len(after) != 1 {
		t.Fatal("lost original mapping")
	}
}

func TestCleanupDirtyProtectionAndInterruptedRecovery(t *testing.T) {
	_, repoPath, _ := storedSetup(t)
	if _, err := storedExec(t, "add", "T", "dirty"); err != nil {
		t.Fatal(err)
	}
	s := storedDB(t)
	roots, _ := s.ListRoots()
	task, _ := s.FindTask(roots[0].ID, "T")
	wts, _ := s.ListWorktrees(task.ID)
	repos, _ := s.ListRepos(roots[0].ID)
	dirty := filepath.Join(wts[0].Path, "untracked.txt")
	if err := os.WriteFile(dirty, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	workspace := filepath.Join(filepath.Dir(wts[0].Path), "user.code-workspace")
	if err := os.WriteFile(workspace, []byte("custom settings"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := storedExec(t, "delete", "T", "--force-branch"); err == nil {
		t.Fatal("branch force bypassed dirty protection")
	}
	if data, _ := os.ReadFile(dirty); string(data) != "keep" {
		t.Fatal("dirty file lost")
	}
	ops, _ := s.ListCleanups(roots[0].ID)
	if len(ops) != 1 || ops[0].Removed {
		t.Fatalf("pending: %+v", ops)
	}
	if _, err := storedExec(t, "detach", "T", wts[0].Path); err == nil {
		t.Fatal("detached pending cleanup")
	}
	if _, err := storedExec(t, "recover", ops[0].ID, "--forget"); err != nil {
		t.Fatal(err)
	}
	// Simulate a crash immediately after Git succeeds, before the database write.
	repoIdentity, err := git.WorktreeIdentity(repoPath)
	if err != nil {
		t.Fatal(err)
	}
	op, err := s.ReserveCleanup(wts[0], "task/T/dirty", "", false, true, false, repoIdentity)
	if err != nil {
		t.Fatal(err)
	}
	if err := git.WorktreeRemove(repoPath, wts[0].Path, true); err != nil {
		t.Fatal(err)
	}
	if err := (work.Manager{Store: s}).RecoverCleanup(repos[0], op, false, false); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(workspace); string(data) != "custom settings" {
		t.Fatal("workspace changed")
	}
	branches, _ := git.ListLocalBranches(repoPath)
	found := false
	for _, b := range branches {
		if b == "task/T/dirty" {
			found = true
		}
	}
	if !found {
		t.Fatal("worktree-only removal deleted branch")
	}
}

func TestCleanupRecoversBranchFailure(t *testing.T) {
	_, _, _ = storedSetup(t)
	if _, err := storedExec(t, "add", "T", "unmerged"); err != nil {
		t.Fatal(err)
	}
	s := storedDB(t)
	roots, _ := s.ListRoots()
	task, _ := s.FindTask(roots[0].ID, "T")
	wts, _ := s.ListWorktrees(task.ID)
	runGit(t, wts[0].Path, "commit", "--allow-empty", "-m", "unmerged")
	out, err := storedExec(t, "delete", "T", "--delete-branch")
	if err == nil {
		t.Fatal("deleted unmerged branch")
	}
	if !strings.Contains(out+err.Error(), "recover") {
		t.Fatalf("no recovery guidance: %s %v", out, err)
	}
	ops, err := s.ListCleanups(roots[0].ID)
	if err != nil || len(ops) != 1 || !ops[0].Removed {
		t.Fatalf("partial state: %+v %v", ops, err)
	}
	if _, err := storedExec(t, "recover", ops[0].ID, "--force-branch"); err != nil {
		t.Fatal(err)
	}
	ops, _ = s.ListCleanups(roots[0].ID)
	if len(ops) != 0 {
		t.Fatal("cleanup still pending")
	}
}

func TestCleanupRecoveryRejectsReplacementRepo(t *testing.T) {
	root, repo, _ := storedSetup(t)
	if _, err := storedExec(t, "add", "T", "unmerged"); err != nil {
		t.Fatal(err)
	}
	s := storedDB(t)
	roots, _ := s.ListRoots()
	task, _ := s.FindTask(roots[0].ID, "T")
	wts, _ := s.ListWorktrees(task.ID)
	runGit(t, wts[0].Path, "commit", "--allow-empty", "-m", "unmerged")
	if _, err := storedExec(t, "delete", "T", "--delete-branch"); err == nil {
		t.Fatal("deleted unmerged branch")
	}
	ops, _ := s.ListCleanups(roots[0].ID)
	if len(ops) != 1 || !ops[0].Removed {
		t.Fatalf("pending: %+v", ops)
	}
	chdir(t, root)
	original := filepath.Join(root, "original-repo")
	if err := os.Rename(repo, original); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "clone", original, repo)
	runGit(t, repo, "branch", ops[0].Branch, ops[0].Revision)
	if _, err := storedExec(t, "recover", ops[0].ID, "--root", roots[0].ID, "--force-branch"); err == nil {
		t.Fatal("deleted a branch in a replacement Repo")
	}
	if revision, err := git.BranchRevision(repo, ops[0].Branch); err != nil || revision != ops[0].Revision {
		t.Fatalf("replacement branch changed: %s %v", revision, err)
	}
}
