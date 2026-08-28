package cmd

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/tintaungkhant/tasky/internal/config"
	"github.com/tintaungkhant/tasky/internal/git"
)

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func initTestRepo(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "init", "-b", "main")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "README.md")
	runGit(t, dir, "commit", "-m", "init")
}

// setupRootWithRepo creates a Root at a fresh temp dir with one repo
// ("project-a") inside it, points TASKY_CONFIG at a fresh config with that
// Root registered, and returns (rootPath, repoPath, tasksDir).
func setupRootWithRepo(t *testing.T) (rootPath, repoPath, tasksDir string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repoPath = filepath.Join(root, "project-a")
	tasksDir = filepath.Join(root, "tasks")
	initTestRepo(t, repoPath)

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
	return root, repoPath, tasksDir
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWd) })
}

func execAdd(t *testing.T, args ...string) error {
	t.Helper()
	addFromFlag = ""
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetArgs(append([]string{"add"}, args...))
	return rootCmd.Execute()
}

func currentBranch(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git branch --show-current: %v", err)
	}
	return string(bytes.TrimSpace(out))
}

func TestAddHappyPathEndToEnd(t *testing.T) {
	_, repoPath, tasksDir := setupRootWithRepo(t)
	chdir(t, repoPath)

	if err := execAdd(t, "CRO-001-payment", "feat/payment"); err != nil {
		t.Fatalf("tasky add: %v", err)
	}

	wantWorktree := filepath.Join(tasksDir, "CRO-001-payment", "project-a[task-CRO-001-payment-feat-payment]")
	if _, err := os.Stat(filepath.Join(wantWorktree, "README.md")); err != nil {
		t.Fatalf("worktree not created at %s: %v", wantWorktree, err)
	}
	if got := currentBranch(t, wantWorktree); got != "task/CRO-001-payment/feat/payment" {
		t.Fatalf("worktree branch = %q, want task/CRO-001-payment/feat/payment", got)
	}
}

func TestAddExplicitRepoFromRootDir(t *testing.T) {
	root, _, tasksDir := setupRootWithRepo(t)
	chdir(t, root)

	if err := execAdd(t, "project-a", "CRO-002-refund", "feat/refund"); err != nil {
		t.Fatalf("tasky add: %v", err)
	}

	wantWorktree := filepath.Join(tasksDir, "CRO-002-refund", "project-a[task-CRO-002-refund-feat-refund]")
	if _, err := os.Stat(filepath.Join(wantWorktree, "README.md")); err != nil {
		t.Fatalf("worktree not created at %s: %v", wantWorktree, err)
	}
	if got := currentBranch(t, wantWorktree); got != "task/CRO-002-refund/feat/refund" {
		t.Fatalf("worktree branch = %q, want task/CRO-002-refund/feat/refund", got)
	}
}

func TestAddRepoArgNotFound(t *testing.T) {
	root, _, _ := setupRootWithRepo(t)
	chdir(t, root)

	err := execAdd(t, "does-not-exist", "CRO-003", "feat/x")
	if err == nil {
		t.Fatal("expected error for unknown repo arg")
	}
}

func TestAddRepoArgCaseSensitive(t *testing.T) {
	root, _, _ := setupRootWithRepo(t)
	chdir(t, root)

	err := execAdd(t, "Project-A", "CRO-003", "feat/x")
	if err == nil {
		t.Fatal("expected error for case-mismatched repo arg")
	}
}

func TestAddFromFlagOverridesBase(t *testing.T) {
	_, repoPath, tasksDir := setupRootWithRepo(t)
	runGit(t, repoPath, "checkout", "-b", "develop")
	runGit(t, repoPath, "checkout", "main")
	chdir(t, repoPath)

	defer func() { addFromFlag = "" }()
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetArgs([]string{"add", "--from", "develop", "CRO-004-base", "feat/x"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("tasky add: %v", err)
	}

	wantWorktree := filepath.Join(tasksDir, "CRO-004-base", "project-a[task-CRO-004-base-feat-x]")
	if got := currentBranch(t, wantWorktree); got != "task/CRO-004-base/feat/x" {
		t.Fatalf("worktree branch = %q, want task/CRO-004-base/feat/x", got)
	}
}

func TestAddWorktreePathAlreadyExists(t *testing.T) {
	_, repoPath, tasksDir := setupRootWithRepo(t)
	chdir(t, repoPath)

	existing := filepath.Join(tasksDir, "CRO-006-collide", "project-a[task-CRO-006-collide-feat-x]")
	if err := os.MkdirAll(existing, 0o755); err != nil {
		t.Fatal(err)
	}

	err := execAdd(t, "CRO-006-collide", "feat/x")
	if err == nil {
		t.Fatal("expected error for pre-existing worktree path")
	}

	branches, lerr := git.ListLocalBranches(repoPath)
	if lerr != nil {
		t.Fatal(lerr)
	}
	if contains(branches, "task/CRO-006-collide/feat/x") {
		t.Fatal("branch should not have been created when worktree path already existed")
	}
}

func TestAddSameTaskAndRepoDifferentBranches(t *testing.T) {
	_, repoPath, tasksDir := setupRootWithRepo(t)
	chdir(t, repoPath)

	if err := execAdd(t, "CRO-009-multi", "feat/payment-bank-a"); err != nil {
		t.Fatalf("tasky add (branch a): %v", err)
	}
	if err := execAdd(t, "CRO-009-multi", "feat/payment-bank-b"); err != nil {
		t.Fatalf("tasky add (branch b): %v", err)
	}

	wtA := filepath.Join(tasksDir, "CRO-009-multi", "project-a[task-CRO-009-multi-feat-payment-bank-a]")
	wtB := filepath.Join(tasksDir, "CRO-009-multi", "project-a[task-CRO-009-multi-feat-payment-bank-b]")
	if _, err := os.Stat(filepath.Join(wtA, "README.md")); err != nil {
		t.Fatalf("worktree a not created at %s: %v", wtA, err)
	}
	if _, err := os.Stat(filepath.Join(wtB, "README.md")); err != nil {
		t.Fatalf("worktree b not created at %s: %v", wtB, err)
	}
}

func TestAddRawBranchMatchingExistingReusedVerbatim(t *testing.T) {
	_, repoPath, tasksDir := setupRootWithRepo(t)
	runGit(t, repoPath, "branch", "feat/payment")
	chdir(t, repoPath)

	if err := execAdd(t, "CRO-002", "feat/payment"); err != nil {
		t.Fatalf("tasky add: %v", err)
	}

	wantWorktree := filepath.Join(tasksDir, "CRO-002", "project-a[feat-payment]")
	if got := currentBranch(t, wantWorktree); got != "feat/payment" {
		t.Fatalf("worktree branch = %q, want feat/payment (reused verbatim, not templated)", got)
	}
}

func TestAddNoMatchingRoot(t *testing.T) {
	_, _, _ = setupRootWithRepo(t)
	outsideDir := t.TempDir()
	chdir(t, outsideDir)

	err := execAdd(t, "CRO-007", "feat/x")
	if err == nil {
		t.Fatal("expected error for cwd outside any configured root")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("tasky setup")) {
		t.Fatalf("error = %q, want it to mention `tasky setup`", err.Error())
	}
}

func TestAddTasksDirAutoCreated(t *testing.T) {
	_, repoPath, tasksDir := setupRootWithRepo(t)
	if err := os.RemoveAll(tasksDir); err != nil {
		t.Fatal(err)
	}
	chdir(t, repoPath)

	if err := execAdd(t, "CRO-008-autocreate", "feat/x"); err != nil {
		t.Fatalf("tasky add: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tasksDir, "CRO-008-autocreate", "project-a[task-CRO-008-autocreate-feat-x]")); err != nil {
		t.Fatalf("tasks_dir not auto-created: %v", err)
	}
}

func TestAddRootPathMissingOnDisk(t *testing.T) {
	root, _, _ := setupRootWithRepo(t)

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}

	// Call resolveRepo directly with cwd passed as a plain string: exercising
	// this via a real os.Chdir would leave the process's cwd pointing at a
	// directory that no longer exists, which is unreliable across platforms.
	_, _, rerr := resolveRepo(cfg, root, "project-a")
	if rerr == nil {
		t.Fatal("expected misconfiguration error for missing root path")
	}
}

func TestAddGitFailureWrapped(t *testing.T) {
	_, repoPath, _ := setupRootWithRepo(t)
	chdir(t, repoPath)

	defer func() { addFromFlag = "" }()
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetArgs([]string{"add", "--from", "does-not-exist", "CRO-010", "feat/x"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent base branch")
	}
	if bytes.Contains([]byte(err.Error()), []byte("exit status")) {
		t.Fatalf("error leaked raw exec.ExitError: %q", err.Error())
	}
}

func TestListTaskIDs(t *testing.T) {
	tasksDir := t.TempDir()

	ids, err := listTaskIDs(filepath.Join(tasksDir, "does-not-exist"))
	if err != nil {
		t.Fatalf("listTaskIDs on missing dir: %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("listTaskIDs on missing dir = %v, want empty", ids)
	}

	for _, id := range []string{"CRO-002", "CRO-001"} {
		if err := os.MkdirAll(filepath.Join(tasksDir, id), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(tasksDir, "not-a-dir"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	ids, err = listTaskIDs(tasksDir)
	if err != nil {
		t.Fatalf("listTaskIDs: %v", err)
	}
	want := []string{"CRO-001", "CRO-002"}
	if len(ids) != len(want) || ids[0] != want[0] || ids[1] != want[1] {
		t.Fatalf("listTaskIDs = %v, want %v", ids, want)
	}
}

func TestAddExistingBranchReuse(t *testing.T) {
	_, repoPath, tasksDir := setupRootWithRepo(t)
	renderedBranch := "task/CRO-005-existing/feat/x"
	runGit(t, repoPath, "branch", renderedBranch)
	chdir(t, repoPath)

	if err := execAdd(t, "CRO-005-existing", "feat/x"); err != nil {
		t.Fatalf("tasky add: %v", err)
	}

	wantWorktree := filepath.Join(tasksDir, "CRO-005-existing", "project-a[task-CRO-005-existing-feat-x]")
	if got := currentBranch(t, wantWorktree); got != renderedBranch {
		t.Fatalf("worktree branch = %q, want %q", got, renderedBranch)
	}
}
