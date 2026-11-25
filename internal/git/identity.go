package git

import (
	"fmt"
	"path/filepath"
	"strings"
)

// WorktreeIdentity identifies Git's administrative directory, which survives
// worktree moves and branch switches. The native file identity detects reuse
// of an administrative directory name after a worktree has been removed.
func WorktreeIdentity(path string) (string, error) {
	if _, _, err := VerifyWorktree(path); err != nil {
		return "", err
	}
	dir, err := run(path, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return "", err
	}
	dir, err = filepath.EvalSymlinks(dir)
	if err != nil {
		return "", err
	}
	id, err := directoryIdentity(dir)
	if err != nil {
		return "", fmt.Errorf("identify Git registration: %w", err)
	}
	return id, nil
}

func WorktreePaths(repo string) ([]string, error) {
	out, err := run(repo, "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, record := range strings.Split(out, "\x00\x00") {
		first, _, _ := strings.Cut(record, "\x00")
		if path, ok := strings.CutPrefix(first, "worktree "); ok {
			paths = append(paths, path)
		}
	}
	return paths, nil
}

func BranchRevision(repo, branch string) (string, error) {
	return run(repo, "rev-parse", "--verify", "refs/heads/"+branch)
}

func LocalChanges(path string) (string, error) {
	return run(path, "--no-optional-locks", "status", "--porcelain=v1", "--untracked-files=all")
}
