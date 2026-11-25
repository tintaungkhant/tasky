package work

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/tintaungkhant/tasky/internal/git"
	"github.com/tintaungkhant/tasky/internal/store"
)

func (m Manager) Remove(repo store.Repo, wt store.Worktree, deleteBranch, forceWorktree, forceBranch bool) error {
	state := m.Inspect(repo, wt)
	if state.Status != "ok" {
		return fmt.Errorf("refusing cleanup of %s [%s]: %s", wt.Path, state.Status, state.Detail)
	}
	branch, err := git.CurrentBranch(wt.Path)
	if err != nil {
		return err
	}
	revision := ""
	if branch != "" && deleteBranch {
		revision, err = git.BranchRevision(repo.Path, branch)
		if err != nil {
			return err
		}
	}
	repoIdentity, err := git.WorktreeIdentity(repo.Path)
	if err != nil {
		return err
	}
	c, err := m.Store.ReserveCleanup(wt, branch, revision, deleteBranch, forceWorktree, forceBranch, repoIdentity)
	if err != nil {
		return err
	}
	return m.RecoverCleanup(repo, c, false, false)
}

func samePath(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func (m Manager) RecoverCleanup(repo store.Repo, c store.Cleanup, forceWorktree, forceBranch bool) error {
	err := m.Store.ExecuteCleanup(c.ID, forceWorktree, forceBranch, func(op store.Cleanup) (bool, error) {
		removed := op.Removed
		if op.RepoIdentity == "" {
			return removed, fmt.Errorf("cleanup predates Repo identity verification; inspect Git, then --forget intent and review remaining mappings and branches manually")
		}
		repoIdentity, err := git.WorktreeIdentity(repo.Path)
		if err != nil {
			return removed, err
		}
		if repoIdentity != op.RepoIdentity {
			return removed, fmt.Errorf("Repo identity mismatch at %s; replacement Repo is protected; restore the original Repo location or --forget intent after review", repo.Path)
		}
		if !removed {
			if _, err := os.Stat(op.Path); os.IsNotExist(err) {
				// Git may have completed immediately before a crash. Only forget the
				// mapping when Git has no registration at the old path or moved identity.
				paths, err := git.WorktreePaths(repo.Path)
				if err != nil {
					return false, err
				}
				for _, path := range paths {
					if samePath(path, op.Path) {
						return false, fmt.Errorf("checkout missing but Git registration remains; inspect git worktree list, then --forget intent and reconnect or detach")
					}
					identity, err := git.WorktreeIdentity(path)
					if err == nil && identity == op.Identity {
						return false, fmt.Errorf("original worktree moved to %s; --forget cleanup intent, then reconnect", path)
					}
				}
				removed = true
			} else {
				wt := store.Worktree{ID: op.WorktreeID, TaskID: op.TaskID, RepoID: op.RepoID, Path: op.Path, Identity: op.Identity}
				state := m.Inspect(repo, wt)
				if state.Status != "ok" {
					return false, fmt.Errorf("refusing unverified cleanup [%s]: %s", state.Status, state.Detail)
				}
				branch, err := git.CurrentBranch(op.Path)
				if err != nil {
					return false, err
				}
				if branch != op.Branch {
					return false, fmt.Errorf("branch changed from %q to %q; --forget intent and review before deleting", op.Branch, branch)
				}
				if err := git.WorktreeRemove(repo.Path, op.Path, op.ForceWorktree); err != nil {
					return false, err
				}
				removed = true
			}
		}
		if op.DeleteBranch && op.Branch != "" {
			branches, err := git.ListLocalBranches(repo.Path)
			if err != nil {
				return removed, err
			}
			exists := false
			for _, name := range branches {
				if name == op.Branch {
					exists = true
				}
			}
			if exists {
				revision, err := git.BranchRevision(repo.Path, op.Branch)
				if err != nil {
					return removed, err
				}
				if revision != op.Revision {
					return removed, fmt.Errorf("worktree removed; branch %q changed since cleanup started; --forget intent and review it manually", op.Branch)
				}
				if err := git.DeleteBranch(repo.Path, op.Branch, op.ForceBranch); err != nil {
					return removed, fmt.Errorf("worktree removed; branch %q remains: %w", op.Branch, err)
				}
			}
		}
		return removed, nil
	})
	if err != nil {
		return fmt.Errorf("cleanup %s incomplete for %s: %w; inspect tasky pending and tasky list; retry tasky recover %s (optionally --force-worktree or --force-branch), or --forget to abandon intent without deleting data", c.ID, c.Path, err, c.ID)
	}
	return m.Store.RemoveEmptyTask(c.TaskID)
}
