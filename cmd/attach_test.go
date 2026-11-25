package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tintaungkhant/tasky/internal/store"
)

func TestAttachVerifiesRepoAndPreservesDirtyFiles(t *testing.T) {
	root, repo, _ := storedSetup(t)
	path := filepath.Join(t.TempDir(), "existing")
	runGit(t, repo, "worktree", "add", "-b", "dirty", path)
	content := []byte("uncommitted work\n")
	if err := os.WriteFile(filepath.Join(path, "README.md"), content, 0o600); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(root, "other")
	initTestRepo(t, other)
	if _, err := registryRun(t, "repo", "add", "other", other, "--root", "company"); err != nil {
		t.Fatal(err)
	}
	before := gitSnapshot(t, repo)
	if _, err := storedExec(t, "attach", "wrong", path, "--repo", "other"); err == nil {
		t.Fatal("wrong Repo accepted")
	}
	subdir := filepath.Join(path, "subdir")
	if err := os.Mkdir(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := storedExec(t, "attach", "wrong", subdir); err == nil {
		t.Fatal("checkout subdirectory accepted")
	}
	if _, err := storedExec(t, "attach", "dirty", path); err != nil {
		t.Fatal(err)
	}
	if _, err := storedExec(t, "detach", "dirty", path); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(path, "README.md")); err != nil || string(data) != string(content) {
		t.Fatal("dirty file changed")
	}
	if after := gitSnapshot(t, repo); before != after {
		t.Fatal("Git state changed")
	}
}

func TestDetachMissingCheckout(t *testing.T) {
	_, repo, _ := storedSetup(t)
	path := filepath.Join(t.TempDir(), "missing")
	runGit(t, repo, "worktree", "add", "-b", "missing", path)
	if _, err := storedExec(t, "attach", "T-1", path); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "worktree", "remove", path)
	before := gitSnapshot(t, repo)
	if _, err := storedExec(t, "detach", "T-1", path); err != nil {
		t.Fatal(err)
	}
	if after := gitSnapshot(t, repo); before != after {
		t.Fatal("detaching missing checkout changed Git state")
	}
}

func TestAttachDetachAndExplicitReassignment(t *testing.T) {
	_, repo, _ := storedSetup(t)
	path := filepath.Join(t.TempDir(), "external-worktree")
	runGit(t, repo, "worktree", "add", "-b", "existing-work", path)
	before := gitSnapshot(t, repo)
	for range 2 {
		if _, err := storedExec(t, "attach", "T-1", path, "--name", "Existing / work"); err != nil {
			t.Fatal(err)
		}
	}
	s := storedDB(t)
	roots, _ := s.ListRoots()
	first, err := s.FindTask(roots[0].ID, "T-1")
	if err != nil {
		t.Fatal(err)
	}
	wts, _ := s.ListWorktrees(first.ID)
	if len(wts) != 1 {
		t.Fatalf("duplicate attach: %+v", wts)
	}
	if _, err := storedExec(t, "attach", "T-2", path); err == nil || !strings.Contains(err.Error(), "--reassign-from") {
		t.Fatalf("implicit reassignment: %v", err)
	}
	if _, err := s.FindTask(roots[0].ID, "T-2"); err != store.ErrNotFound {
		t.Fatalf("conflict created empty target Task: %v", err)
	}
	if _, err := storedExec(t, "attach", "T-2", path, "--reassign-from", first.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := storedExec(t, "attach", "T-2", path, "--reassign-from", first.ID); err != nil {
		t.Fatalf("retry reassignment: %v", err)
	}
	second, err := s.FindTask(roots[0].ID, "T-2")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := storedExec(t, "detach", "T-1", path); err == nil {
		t.Fatal("stale detach removed reassigned mapping")
	}
	for range 2 {
		if _, err := storedExec(t, "detach", "T-2", path); err != nil {
			t.Fatal(err)
		}
	}
	wts, _ = s.ListWorktrees(second.ID)
	if len(wts) != 0 {
		t.Fatalf("detach retained mapping: %+v", wts)
	}
	if after := gitSnapshot(t, repo); before != after {
		t.Fatal("association changes altered Git state")
	}
}
