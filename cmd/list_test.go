package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func execList(t *testing.T, args ...string) (string, error) {
	t.Helper()
	listAllFlag = false
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetArgs(append([]string{"list"}, args...))
	err := rootCmd.Execute()
	return out.String(), err
}

func TestListAllShowsAllTasksWithWorktrees(t *testing.T) {
	root, repoPath, _ := setupRootWithRepo(t)
	chdir(t, repoPath)

	repoBPath := root + "/project-b"
	initTestRepo(t, repoBPath)

	if err := execAdd(t, "CRO-001", "feat/payment"); err != nil {
		t.Fatalf("tasky add: %v", err)
	}
	if err := execAdd(t, "--name", "laravel-upgrade", "project-b", "CRO-002", "chore/upgrade"); err != nil {
		t.Fatalf("tasky add: %v", err)
	}

	out, err := execList(t, "--all")
	if err != nil {
		t.Fatalf("tasky list --all: %v", err)
	}

	for _, want := range []string{
		"CRO-001",
		"project-a", "task/CRO-001/feat/payment",
		"CRO-002 (laravel-upgrade)",
		"project-b", "task/CRO-002/chore/upgrade",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("tasky list output missing %q, got:\n%s", want, out)
		}
	}
}

func TestListFiltersByTaskID(t *testing.T) {
	_, repoPath, _ := setupRootWithRepo(t)
	chdir(t, repoPath)

	if err := execAdd(t, "CRO-003", "feat/a"); err != nil {
		t.Fatalf("tasky add: %v", err)
	}
	if err := execAdd(t, "CRO-004", "feat/b"); err != nil {
		t.Fatalf("tasky add: %v", err)
	}

	out, err := execList(t, "CRO-003")
	if err != nil {
		t.Fatalf("tasky list: %v", err)
	}
	if !strings.Contains(out, "CRO-003") {
		t.Fatalf("tasky list CRO-003 missing CRO-003, got:\n%s", out)
	}
	if strings.Contains(out, "CRO-004") {
		t.Fatalf("tasky list CRO-003 should not mention CRO-004, got:\n%s", out)
	}
}

func TestListUnknownTaskID(t *testing.T) {
	_, repoPath, _ := setupRootWithRepo(t)
	chdir(t, repoPath)

	if _, err := execList(t, "does-not-exist"); err == nil {
		t.Fatal("expected error for unknown task ID")
	}
}

func TestListNoTasksFound(t *testing.T) {
	_, repoPath, _ := setupRootWithRepo(t)
	chdir(t, repoPath)

	out, err := execList(t)
	if err != nil {
		t.Fatalf("tasky list: %v", err)
	}
	if !strings.Contains(out, "no tasks found") {
		t.Fatalf("expected 'no tasks found' message, got:\n%s", out)
	}
}
