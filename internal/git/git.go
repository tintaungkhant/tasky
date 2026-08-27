// Package git shells out to the git binary for worktree and branch operations.
package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

func run(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}
	return strings.TrimSpace(stdout.String()), nil
}

// Toplevel returns the top-level directory of the Git repo containing dir.
func Toplevel(dir string) (string, error) {
	return run(dir, "rev-parse", "--show-toplevel")
}

// CurrentBranch returns the current branch name in the repo at dir.
func CurrentBranch(dir string) (string, error) {
	return run(dir, "branch", "--show-current")
}

// WorktreeAddNewBranch creates a worktree at path on a new branch, based on base.
func WorktreeAddNewBranch(repoDir, path, branch, base string) error {
	_, err := run(repoDir, "worktree", "add", "-b", branch, path, base)
	return err
}

// WorktreeAddExistingBranch creates a worktree at path checking out the existing branch.
func WorktreeAddExistingBranch(repoDir, path, branch string) error {
	_, err := run(repoDir, "worktree", "add", path, branch)
	return err
}
