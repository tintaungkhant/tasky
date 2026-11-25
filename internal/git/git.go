// Package git shells out to the git binary for worktree and branch operations.
package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

// CommonDir identifies the Git repository shared by all its worktrees.
func CommonDir(dir string) (string, error) {
	return run(dir, "rev-parse", "--path-format=absolute", "--git-common-dir")
}

// PrimaryWorktree returns the main checkout, including when dir is a linked
// worktree. Bare repositories have no checkout and cannot be registered.
func PrimaryWorktree(dir string) (string, error) {
	if _, err := Toplevel(dir); err != nil {
		return "", err
	}
	out, err := run(dir, "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return "", err
	}
	record, _, _ := strings.Cut(out, "\x00\x00")
	if strings.Contains(record, "\x00bare") {
		return "", fmt.Errorf("cannot register a bare Repo; use a Repo with a primary checkout")
	}
	first, _, _ := strings.Cut(record, "\x00")
	path, ok := strings.CutPrefix(first, "worktree ")
	if !ok || path == "" {
		return "", fmt.Errorf("cannot locate primary worktree; inspect `git worktree list`")
	}
	return path, nil
}

// VerifyWorktree requires an actual checkout root registered with Git. A
// directory merely inside a checkout cannot be attached or imported.
func VerifyWorktree(path string) (string, string, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return "", "", err
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return "", "", fmt.Errorf("worktree location: %w", err)
	}
	top, err := Toplevel(path)
	if err != nil {
		return "", "", err
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", "", err
	}
	topInfo, err := os.Stat(top)
	if err != nil || !os.SameFile(info, topInfo) {
		return "", "", fmt.Errorf("%s is not a Git worktree root", path)
	}
	primary, err := PrimaryWorktree(path)
	if err != nil {
		return "", "", err
	}
	out, err := run(path, "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return "", "", err
	}
	for _, record := range strings.Split(out, "\x00\x00") {
		first, _, _ := strings.Cut(record, "\x00")
		registered, ok := strings.CutPrefix(first, "worktree ")
		if !ok {
			continue
		}
		other, err := os.Stat(registered)
		if err == nil && os.SameFile(info, other) {
			primary, err = filepath.EvalSymlinks(primary)
			return path, primary, err
		}
	}
	return "", "", fmt.Errorf("%s is not registered with Git; inspect `git worktree list`", path)
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

// WorktreeRemove removes the worktree at path. It refuses when the worktree
// has uncommitted changes or is locked unless force is set.
func WorktreeRemove(repoDir, path string, force bool) error {
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, path)
	_, err := run(repoDir, args...)
	return err
}

// DeleteBranch deletes branch from the repo at dir. It refuses to delete a
// branch with commits not merged elsewhere unless force is set.
func DeleteBranch(dir, branch string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}
	_, err := run(dir, "branch", flag, branch)
	return err
}

// ListLocalBranches returns the names of all local branches in the repo at dir.
func ListLocalBranches(dir string) ([]string, error) {
	out, err := run(dir, "for-each-ref", "--format=%(refname:short)", "refs/heads/")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

// CheckedOutBranchPaths returns, for every local branch currently checked
// out in some worktree of the repo at dir (including the main worktree
// itself), the path of the worktree it's checked out in. These branches
// cannot be checked out again in a new worktree.
func CheckedOutBranchPaths(dir string) (map[string]string, error) {
	out, err := run(dir, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	paths := make(map[string]string)
	var currentPath string
	for _, line := range strings.Split(out, "\n") {
		if path, ok := strings.CutPrefix(line, "worktree "); ok {
			currentPath = path
			continue
		}
		if branch, ok := strings.CutPrefix(line, "branch refs/heads/"); ok {
			paths[branch] = currentPath
		}
	}
	return paths, nil
}

// IsRepo reports whether path is the root of a Git repository (contains a .git entry).
func IsRepo(path string) bool {
	_, err := os.Stat(filepath.Join(path, ".git"))
	return err == nil
}
