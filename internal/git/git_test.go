package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runInit := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	runInit("init", "-b", "main")
	runInit("config", "user.email", "test@example.com")
	runInit("config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	runInit("add", "README.md")
	runInit("commit", "-m", "init")
	return dir
}

func TestToplevelAndCurrentBranch(t *testing.T) {
	repo := initRepo(t)

	top, err := Toplevel(repo)
	if err != nil {
		t.Fatalf("Toplevel: %v", err)
	}
	// Resolve symlinks (macOS temp dirs are often under /private via symlink).
	wantTop, _ := filepath.EvalSymlinks(repo)
	gotTop, _ := filepath.EvalSymlinks(top)
	if gotTop != wantTop {
		t.Fatalf("Toplevel = %q, want %q", gotTop, wantTop)
	}

	branch, err := CurrentBranch(repo)
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	if branch != "main" {
		t.Fatalf("CurrentBranch = %q, want main", branch)
	}
}

func TestWorktreeAddNewBranch(t *testing.T) {
	repo := initRepo(t)
	wtPath := filepath.Join(t.TempDir(), "wt")

	if err := WorktreeAddNewBranch(repo, wtPath, "feat/payment", "main"); err != nil {
		t.Fatalf("WorktreeAddNewBranch: %v", err)
	}
	if _, err := os.Stat(filepath.Join(wtPath, "README.md")); err != nil {
		t.Fatalf("worktree missing checked-out files: %v", err)
	}

	branch, err := CurrentBranch(wtPath)
	if err != nil {
		t.Fatalf("CurrentBranch on worktree: %v", err)
	}
	if branch != "feat/payment" {
		t.Fatalf("worktree branch = %q, want feat/payment", branch)
	}
}

func TestWorktreeAddExistingBranch(t *testing.T) {
	repo := initRepo(t)

	cmd := exec.Command("git", "branch", "existing-branch")
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git branch: %v\n%s", err, out)
	}

	wtPath := filepath.Join(t.TempDir(), "wt")
	if err := WorktreeAddExistingBranch(repo, wtPath, "existing-branch"); err != nil {
		t.Fatalf("WorktreeAddExistingBranch: %v", err)
	}

	branch, err := CurrentBranch(wtPath)
	if err != nil {
		t.Fatalf("CurrentBranch on worktree: %v", err)
	}
	if branch != "existing-branch" {
		t.Fatalf("worktree branch = %q, want existing-branch", branch)
	}
}

func TestWorktreeAddNewBranchErrorWrapped(t *testing.T) {
	repo := initRepo(t)
	err := WorktreeAddNewBranch(repo, t.TempDir(), "feat/payment", "does-not-exist")
	if err == nil {
		t.Fatal("expected error for missing base branch")
	}
}

func TestListLocalBranches(t *testing.T) {
	repo := initRepo(t)

	cmd := exec.Command("git", "branch", "feat/payment")
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git branch: %v\n%s", err, out)
	}

	branches, err := ListLocalBranches(repo)
	if err != nil {
		t.Fatalf("ListLocalBranches: %v", err)
	}

	want := map[string]bool{"main": true, "feat/payment": true}
	if len(branches) != len(want) {
		t.Fatalf("ListLocalBranches = %v, want keys of %v", branches, want)
	}
	for _, b := range branches {
		if !want[b] {
			t.Fatalf("unexpected branch %q in %v", b, branches)
		}
	}
}

func TestIsRepo(t *testing.T) {
	repo := initRepo(t)
	if !IsRepo(repo) {
		t.Fatalf("IsRepo(%q) = false, want true", repo)
	}

	notRepo := t.TempDir()
	if IsRepo(notRepo) {
		t.Fatalf("IsRepo(%q) = true, want false", notRepo)
	}
}
