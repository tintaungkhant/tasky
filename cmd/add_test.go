package cmd

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/tintaungkhant/tasky/internal/config"
)

func TestAddHappyPathEndToEnd(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repoPath := filepath.Join(root, "project-a")
	tasksDir := filepath.Join(root, "tasks")
	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatal(err)
	}

	runGit := func(dir string, args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	runGit(repoPath, "init", "-b", "main")
	runGit(repoPath, "config", "user.email", "test@example.com")
	runGit(repoPath, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(repoPath, "README.md"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(repoPath, "add", "README.md")
	runGit(repoPath, "commit", "-m", "init")

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv("TASKY_CONFIG", configPath)
	cfg := &config.Config{}
	cfg.AddRoot(config.Root{
		Name:           "company",
		Path:           root,
		TasksDir:       tasksDir,
		BranchTemplate: "task/{task-id}/{branch}",
	})
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}

	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repoPath); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWd) })

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetArgs([]string{"add", "CRO-001-payment", "feat/payment"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("tasky add: %v", err)
	}

	wantWorktree := filepath.Join(tasksDir, "CRO-001-payment", "project-a")
	if _, err := os.Stat(filepath.Join(wantWorktree, "README.md")); err != nil {
		t.Fatalf("worktree not created at %s: %v", wantWorktree, err)
	}

	branchCmd := exec.Command("git", "branch", "--show-current")
	branchCmd.Dir = wantWorktree
	branchOut, err := branchCmd.Output()
	if err != nil {
		t.Fatalf("git branch --show-current: %v", err)
	}
	gotBranch := string(bytes.TrimSpace(branchOut))
	if gotBranch != "task/CRO-001-payment/feat/payment" {
		t.Fatalf("worktree branch = %q, want task/CRO-001-payment/feat/payment", gotBranch)
	}
}
