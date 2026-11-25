package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInspectAndReconnect(t *testing.T) {
	root, repo, _ := storedSetup(t)
	if _, err := storedExec(t, "add", "T-live", "live"); err != nil {
		t.Fatal(err)
	}
	s := storedDB(t)
	roots, _ := s.ListRoots()
	task, _ := s.FindTask(roots[0].ID, "T-live")
	wts, _ := s.ListWorktrees(task.ID)
	old := wts[0].Path
	runGit(t, old, "switch", "-c", "external")
	if err := os.WriteFile(filepath.Join(old, "dirty.txt"), []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	out, err := storedExec(t, "list", "T-live")
	if err != nil || !strings.Contains(out, "external") || !strings.Contains(out, "dirty.txt") {
		t.Fatalf("live state: %s %v", out, err)
	}
	moved := filepath.Join(root, "moved")
	runGit(t, repo, "worktree", "move", old, moved)
	out, err = storedExec(t, "list", "T-live")
	if err != nil || !strings.Contains(out, "missing") {
		t.Fatalf("missing: %s %v", out, err)
	}
	replacement := filepath.Join(root, "other")
	runGit(t, repo, "worktree", "add", "-b", "replacement", replacement)
	runGit(t, repo, "worktree", "move", replacement, old)
	out, err = storedExec(t, "list", "T-live")
	if err != nil || !strings.Contains(out, "identity mismatch") || strings.Contains(out, "replacement  ") {
		t.Fatalf("replacement: %s %v", out, err)
	}
	if _, err := storedExec(t, "reconnect", "T-live", old, old); err == nil {
		t.Fatal("reconnected unrelated worktree")
	}
	if _, err := storedExec(t, "reconnect", "T-live", old, moved); err != nil {
		t.Fatal(err)
	}
	after, _ := s.ListWorktrees(task.ID)
	if len(after) != 1 || after[0].ID != wts[0].ID || after[0].Path != moved {
		t.Fatalf("mapping: %+v", after)
	}
	if _, err := storedExec(t, "reconnect", "T-live", old, replacement); err == nil {
		t.Fatal("stale reconnect succeeded")
	}
	if data, _ := os.ReadFile(filepath.Join(moved, "dirty.txt")); string(data) != "keep" {
		t.Fatal("lost dirty file")
	}
	if err := os.Rename(filepath.Join(moved, ".git"), filepath.Join(moved, ".git-saved")); err != nil {
		t.Fatal(err)
	}
	out, err = storedExec(t, "list", "T-live")
	if err != nil || !strings.Contains(out, "[invalid]") {
		t.Fatalf("invalid registration: %s %v", out, err)
	}
}

func TestInspectLegacyEnrollment(t *testing.T) {
	root, repo, _ := storedSetup(t)
	path := filepath.Join(root, "legacy")
	runGit(t, repo, "worktree", "add", "-b", "enroll", path)
	s := storedDB(t)
	roots, _ := s.ListRoots()
	repos, _ := s.ListRepos(roots[0].ID)
	task, _ := s.CreateTask(roots[0].ID, "legacy", "")
	if _, err := s.Associate(task.ID, repos[0].ID, path); err != nil {
		t.Fatal(err)
	}
	if _, err := storedExec(t, "reconnect", "legacy", path, path); err == nil {
		t.Fatal("enrolled without explicit acknowledgment")
	}
	if _, err := storedExec(t, "reconnect", "legacy", path, path, "--accept-identity"); err != nil {
		t.Fatal(err)
	}
	out, err := storedExec(t, "list", "legacy")
	if err != nil || strings.Contains(out, "unverified") || !strings.Contains(out, "enroll") {
		t.Fatalf("enrollment: %s %v", out, err)
	}
}
