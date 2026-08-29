package cmd

import (
	"slices"
	"testing"
)

func TestCompleteTaskID(t *testing.T) {
	_, repoPath, _ := setupRootWithRepo(t)
	chdir(t, repoPath)

	if err := execAdd(t, "CRO-001", "feat/x"); err != nil {
		t.Fatalf("tasky add: %v", err)
	}
	if err := execAdd(t, "CRO-002", "feat/y"); err != nil {
		t.Fatalf("tasky add: %v", err)
	}

	got, _ := completeTaskID(listCmd, nil, "")
	want := []string{"CRO-001", "CRO-002"}
	if !slices.Equal(got, want) {
		t.Fatalf("completeTaskID(\"\") = %v, want %v", got, want)
	}

	got, _ = completeTaskID(listCmd, nil, "CRO-00")
	if !slices.Equal(got, want) {
		t.Fatalf("completeTaskID(\"CRO-00\") = %v, want %v", got, want)
	}

	got, _ = completeTaskID(listCmd, nil, "CRO-002")
	if !slices.Equal(got, []string{"CRO-002"}) {
		t.Fatalf("completeTaskID(\"CRO-002\") = %v, want [CRO-002]", got)
	}

	// A second positional arg is out of range for list/vscode: no suggestions.
	got, _ = completeTaskID(listCmd, []string{"CRO-001"}, "")
	if len(got) != 0 {
		t.Fatalf("completeTaskID with 1 arg already given = %v, want none", got)
	}
}

func TestCompleteDeleteArgsTaskID(t *testing.T) {
	_, repoPath, _ := setupRootWithRepo(t)
	chdir(t, repoPath)

	if err := execAdd(t, "CRO-003", "feat/x"); err != nil {
		t.Fatalf("tasky add: %v", err)
	}

	got, _ := completeDeleteArgs(deleteCmd, nil, "")
	if !slices.Equal(got, []string{"CRO-003"}) {
		t.Fatalf("completeDeleteArgs(task-id) = %v, want [CRO-003]", got)
	}
}

func TestCompleteDeleteArgsRepo(t *testing.T) {
	root, repoPath, _ := setupRootWithRepo(t)
	chdir(t, repoPath)

	repoBPath := root + "/project-b"
	initTestRepo(t, repoBPath)

	if err := execAdd(t, "CRO-004", "feat/x"); err != nil {
		t.Fatalf("tasky add (a): %v", err)
	}
	if err := execAdd(t, "project-b", "CRO-004", "feat/y"); err != nil {
		t.Fatalf("tasky add (b): %v", err)
	}

	got, _ := completeDeleteArgs(deleteCmd, []string{"CRO-004"}, "")
	want := []string{"project-a", "project-b"}
	if !slices.Equal(got, want) {
		t.Fatalf("completeDeleteArgs(repo) = %v, want %v", got, want)
	}

	got, _ = completeDeleteArgs(deleteCmd, []string{"CRO-004"}, "project-b")
	if !slices.Equal(got, []string{"project-b"}) {
		t.Fatalf("completeDeleteArgs(repo, prefix) = %v, want [project-b]", got)
	}
}

func TestCompleteDeleteArgsBranch(t *testing.T) {
	_, repoPath, _ := setupRootWithRepo(t)
	chdir(t, repoPath)

	if err := execAdd(t, "CRO-005", "feat/a"); err != nil {
		t.Fatalf("tasky add (a): %v", err)
	}
	if err := execAdd(t, "CRO-005", "feat/b"); err != nil {
		t.Fatalf("tasky add (b): %v", err)
	}

	got, _ := completeDeleteArgs(deleteCmd, []string{"CRO-005", "project-a"}, "")
	want := []string{"task/CRO-005/feat/a", "task/CRO-005/feat/b"}
	if !slices.Equal(got, want) {
		t.Fatalf("completeDeleteArgs(branch) = %v, want %v", got, want)
	}
}

func TestCompleteDeleteArgsUnknownTaskID(t *testing.T) {
	_, repoPath, _ := setupRootWithRepo(t)
	chdir(t, repoPath)

	got, _ := completeDeleteArgs(deleteCmd, []string{"does-not-exist"}, "")
	if len(got) != 0 {
		t.Fatalf("completeDeleteArgs with unknown task-id = %v, want none", got)
	}
}
