package store

import (
	"crypto/rand"
	"database/sql"
	"fmt"
)

const cleanupSchema = `
CREATE TABLE cleanups (
 id TEXT PRIMARY KEY NOT NULL,
 root_id TEXT NOT NULL REFERENCES roots(id),
 task_id TEXT NOT NULL REFERENCES tasks(id),
 repo_id TEXT NOT NULL REFERENCES repos(id),
 worktree_id TEXT NOT NULL UNIQUE,
 path TEXT NOT NULL,
 path_key TEXT NOT NULL UNIQUE,
 identity TEXT NOT NULL,
 branch TEXT NOT NULL,
 revision TEXT NOT NULL,
 delete_branch INTEGER NOT NULL,
 force_worktree INTEGER NOT NULL,
 force_branch INTEGER NOT NULL,
 removed INTEGER NOT NULL DEFAULT 0
);
PRAGMA user_version = 4;
`

type Cleanup struct {
	ID, RootID, TaskID, RepoID, WorktreeID, Path, Identity, Branch, Revision string
	DeleteBranch, ForceWorktree, ForceBranch, Removed                        bool
}

const cleanupColumns = "id, root_id, task_id, repo_id, worktree_id, path, identity, branch, revision, delete_branch, force_worktree, force_branch, removed"

func scanCleanup(row interface{ Scan(...any) error }) (Cleanup, error) {
	var c Cleanup
	err := row.Scan(&c.ID, &c.RootID, &c.TaskID, &c.RepoID, &c.WorktreeID, &c.Path, &c.Identity, &c.Branch, &c.Revision, &c.DeleteBranch, &c.ForceWorktree, &c.ForceBranch, &c.Removed)
	return c, err
}

func (s *Store) ListCleanups(rootID string) ([]Cleanup, error) {
	rows, err := s.db.Query("SELECT "+cleanupColumns+" FROM cleanups WHERE root_id = ? ORDER BY id", rootID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Cleanup
	for rows.Next() {
		c, err := scanCleanup(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ReserveCleanup persists exact intent before any Git side effects. The SELECT
// guards against a stale snapshot after detach, reconnect, or reassignment.
func (s *Store) ReserveCleanup(wt Worktree, branch, revision string, deleteBranch, forceWorktree, forceBranch bool) (Cleanup, error) {
	_, key, err := location(wt.Path)
	if err != nil {
		return Cleanup{}, err
	}
	c := Cleanup{ID: rand.Text(), TaskID: wt.TaskID, RepoID: wt.RepoID, WorktreeID: wt.ID, Path: wt.Path, Identity: wt.Identity, Branch: branch, Revision: revision, DeleteBranch: deleteBranch, ForceWorktree: forceWorktree, ForceBranch: forceBranch}
	result, err := s.db.Exec(`INSERT INTO cleanups(id, root_id, task_id, repo_id, worktree_id, path, path_key, identity, branch, revision, delete_branch, force_worktree, force_branch)
 SELECT ?, root_id, task_id, repo_id, id, path, path_key, identity, ?, ?, ?, ?, ? FROM worktrees
 WHERE id = ? AND task_id = ? AND repo_id = ? AND path_key = ? AND identity = ?`, c.ID, branch, revision, deleteBranch, forceWorktree, forceBranch, wt.ID, wt.TaskID, wt.RepoID, key, wt.Identity)
	if err := changed("reserve cleanup; inspect tasky pending before retrying", result, err); err != nil {
		return Cleanup{}, err
	}
	return c, nil
}

// ExecuteCleanup holds SQLite's writer lock while Git runs. Tasky mapping
// mutations cannot race removal. Durable intent survives a process interruption;
// a failed branch step commits the completed worktree step and retains intent.
func (s *Store) ExecuteCleanup(id string, forceWorktree, forceBranch bool, action func(Cleanup) (bool, error)) error {
	tx, err := s.db.Begin()
	if err != nil {
		return writeError("cleanup", err)
	}
	defer tx.Rollback()
	c, err := scanCleanup(tx.QueryRow("SELECT "+cleanupColumns+" FROM cleanups WHERE id = ?", id))
	if err == sql.ErrNoRows {
		return fmt.Errorf("cleanup not found; inspect tasky pending: %w", ErrNotFound)
	}
	if err != nil {
		return err
	}
	c.ForceWorktree = c.ForceWorktree || forceWorktree
	c.ForceBranch = c.ForceBranch || forceBranch
	if !c.Removed {
		var task, repo, path, identity string
		if err := tx.QueryRow("SELECT task_id, repo_id, path, identity FROM worktrees WHERE id = ?", c.WorktreeID).Scan(&task, &repo, &path, &identity); err != nil {
			return err
		}
		if task != c.TaskID || repo != c.RepoID || path != c.Path || identity != c.Identity {
			return fmt.Errorf("cleanup mapping changed: %w", ErrConflict)
		}
	}
	removed, actionErr := action(c)
	if removed {
		if _, err := tx.Exec("DELETE FROM worktrees WHERE id = ?", c.WorktreeID); err != nil {
			return err
		}
	}
	if actionErr == nil {
		_, err = tx.Exec("DELETE FROM cleanups WHERE id = ?", id)
	} else {
		_, err = tx.Exec("UPDATE cleanups SET removed = ?, force_worktree = ?, force_branch = ? WHERE id = ?", removed, c.ForceWorktree, c.ForceBranch, id)
	}
	if err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("Git may have completed; cleanup intent remains for recovery: %w", err)
	}
	return actionErr
}

func (s *Store) ForgetCleanup(id string) error {
	result, err := s.db.Exec("DELETE FROM cleanups WHERE id = ?", id)
	return changed("forget cleanup intent only", result, err)
}

func cleanupReserved(tx *sql.Tx, key string) error {
	var id string
	err := tx.QueryRow("SELECT id FROM cleanups WHERE path_key = ?", key).Scan(&id)
	if err == nil {
		return fmt.Errorf("cleanup %s is pending; use tasky recover %s or --forget before changing mappings: %w", id, id, ErrConflict)
	}
	if err == sql.ErrNoRows {
		return nil
	}
	return err
}
