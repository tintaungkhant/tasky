package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func execVSCode(t *testing.T, args ...string) error {
	t.Helper()
	rootCmd.SetArgs(append([]string{"vscode"}, args...))
	return rootCmd.Execute()
}

func TestVSCodeGeneratesWorkspaceFile(t *testing.T) {
	_, repoPath, tasksDir := setupRootWithRepo(t)
	chdir(t, repoPath)

	if err := execAdd(t, "CRO-020-multi", "feat/x"); err != nil {
		t.Fatalf("add: %v", err)
	}

	if err := execVSCode(t, "CRO-020-multi"); err != nil {
		t.Fatalf("vscode: %v", err)
	}

	workspacePath := filepath.Join(tasksDir, "CRO-020-multi", "CRO-020-multi.code-workspace")
	data, err := os.ReadFile(workspacePath)
	if err != nil {
		t.Fatalf("read workspace file: %v", err)
	}

	var ws vscodeWorkspace
	if err := json.Unmarshal(data, &ws); err != nil {
		t.Fatalf("unmarshal workspace file: %v", err)
	}
	if len(ws.Folders) != 1 {
		t.Fatalf("workspace folders = %v, want 1 entry", ws.Folders)
	}
	wantFolder := "./project-a[task-CRO-020-multi-feat-x]"
	if ws.Folders[0].Path != wantFolder {
		t.Fatalf("workspace folder path = %q, want %q", ws.Folders[0].Path, wantFolder)
	}
}

func TestVSCodeRerunPicksUpNewFolder(t *testing.T) {
	_, repoPath, tasksDir := setupRootWithRepo(t)
	chdir(t, repoPath)

	if err := execAdd(t, "project-a", "CRO-022-multi", "feat/x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := execVSCode(t, "CRO-022-multi"); err != nil {
		t.Fatalf("vscode (first run): %v", err)
	}

	workspacePath := filepath.Join(tasksDir, "CRO-022-multi", "CRO-022-multi.code-workspace")
	ws := readWorkspace(t, workspacePath)
	if len(ws.Folders) != 1 {
		t.Fatalf("workspace folders after first run = %v, want 1 entry", ws.Folders)
	}

	// A second repo worktree added to the same task after the first run...
	repoBPath := filepath.Join(filepath.Dir(repoPath), "project-b")
	initTestRepo(t, repoBPath)
	if err := execAdd(t, "project-b", "CRO-022-multi", "feat/y"); err != nil {
		t.Fatalf("add project-b: %v", err)
	}

	// ...must show up when vscode is re-run, not just what was there originally.
	if err := execVSCode(t, "CRO-022-multi"); err != nil {
		t.Fatalf("vscode (second run): %v", err)
	}
	ws = readWorkspace(t, workspacePath)
	if len(ws.Folders) != 2 {
		t.Fatalf("workspace folders after second run = %v, want 2 entries", ws.Folders)
	}
}

func readWorkspace(t *testing.T, path string) vscodeWorkspace {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read workspace file: %v", err)
	}
	var ws vscodeWorkspace
	if err := json.Unmarshal(data, &ws); err != nil {
		t.Fatalf("unmarshal workspace file: %v", err)
	}
	return ws
}

func TestVSCodeWorkspaceFileNameIncludesTaskName(t *testing.T) {
	_, repoPath, tasksDir := setupRootWithRepo(t)
	chdir(t, repoPath)

	if err := execAdd(t, "--name", "laravel-upgrade", "CRO-023", "feat/x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := execVSCode(t, "CRO-023"); err != nil {
		t.Fatalf("vscode: %v", err)
	}

	workspacePath := filepath.Join(tasksDir, "CRO-023[laravel-upgrade]", "CRO-023[laravel-upgrade].code-workspace")
	if _, err := os.Stat(workspacePath); err != nil {
		t.Fatalf("workspace file not created at %s: %v", workspacePath, err)
	}
}

func TestVSCodeUnknownTaskErrors(t *testing.T) {
	_, repoPath, _ := setupRootWithRepo(t)
	chdir(t, repoPath)

	err := execVSCode(t, "does-not-exist")
	if err == nil {
		t.Fatal("expected error for unknown task")
	}
}

func TestVSCodeNoWorktreesErrors(t *testing.T) {
	_, repoPath, tasksDir := setupRootWithRepo(t)
	chdir(t, repoPath)

	emptyTaskDir := filepath.Join(tasksDir, "CRO-021-empty")
	if err := os.MkdirAll(emptyTaskDir, 0o755); err != nil {
		t.Fatal(err)
	}

	err := execVSCode(t, "CRO-021-empty")
	if err == nil {
		t.Fatal("expected error for task with no worktrees")
	}
}
