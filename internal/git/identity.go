package git

import (
	"fmt"
	"path/filepath"
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

func LocalChanges(path string) (string, error) {
	return run(path, "status", "--porcelain=v1", "--untracked-files=all")
}
