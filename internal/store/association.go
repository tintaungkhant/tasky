package store

import (
	"crypto/rand"
	"database/sql"
	"fmt"
)

// Attach atomically creates a Task if needed and associates a verified checkout.
// expectedSource is the internal Task identity required for reassignment. It
// acts as a compare-and-swap guard against stale or competing requests.
func (s *Store) Attach(rootID, publicID, name, repoID, path, expectedSource string) (Worktree, error) {
	if publicID == "" {
		return Worktree{}, fmt.Errorf("Task ID must not be empty")
	}
	path, key, err := location(path)
	if err != nil {
		return Worktree{}, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return Worktree{}, writeError("attach", err)
	}
	defer tx.Rollback()
	var validRepo string
	err = tx.QueryRow("SELECT id FROM repos WHERE id = ? AND root_id = ?", repoID, rootID).Scan(&validRepo)
	if err == sql.ErrNoRows {
		return Worktree{}, ErrNotFound
	}
	if err != nil {
		return Worktree{}, err
	}
	var pending string
	err = tx.QueryRow("SELECT id FROM creations WHERE path_key = ?", key).Scan(&pending)
	if err == nil {
		return Worktree{}, fmt.Errorf("creation %s is pending for %s; use `tasky recover` before attaching", pending, path)
	}
	if err != sql.ErrNoRows {
		return Worktree{}, err
	}
	var target string
	err = tx.QueryRow("SELECT id FROM tasks WHERE root_id = ? AND task_id = ?", rootID, publicID).Scan(&target)
	if err != nil && err != sql.ErrNoRows {
		return Worktree{}, err
	}
	var current Worktree
	err = tx.QueryRow("SELECT id, task_id, repo_id, path FROM worktrees WHERE path_key = ?", key).Scan(&current.ID, &current.TaskID, &current.RepoID, &current.Path)
	if err != nil && err != sql.ErrNoRows {
		return Worktree{}, err
	}
	exists := err == nil
	if exists && current.TaskID == target && current.RepoID == repoID {
		return current, nil
	}
	if exists && (expectedSource == "" || expectedSource != current.TaskID) {
		return Worktree{}, fmt.Errorf("worktree %s belongs to internal Task %s; review its current mapping, then use --reassign-from %s to authorize reassignment: %w", path, current.TaskID, current.TaskID, ErrConflict)
	}
	if !exists && expectedSource != "" {
		return Worktree{}, fmt.Errorf("source association no longer exists; review before attaching without --reassign-from: %w", ErrConflict)
	}
	if target == "" {
		target = rand.Text()
		if _, err := tx.Exec("INSERT INTO tasks(id, root_id, task_id, name) VALUES(?, ?, ?, ?)", target, rootID, publicID, name); err != nil {
			return Worktree{}, writeError("attach Task", err)
		}
	}
	wt := Worktree{ID: current.ID, TaskID: target, RepoID: repoID, Path: path}
	if exists {
		_, err = tx.Exec("UPDATE worktrees SET root_id = ?, task_id = ?, repo_id = ?, path = ? WHERE id = ?", rootID, target, repoID, path, current.ID)
	} else {
		wt.ID = rand.Text()
		_, err = tx.Exec("INSERT INTO worktrees(id, root_id, task_id, repo_id, path, path_key) VALUES(?, ?, ?, ?, ?, ?)", wt.ID, rootID, target, repoID, path, key)
	}
	if err != nil {
		return Worktree{}, writeError("attach", err)
	}
	if err := tx.Commit(); err != nil {
		return Worktree{}, writeError("attach", err)
	}
	return wt, nil
}

// Detach is idempotent only while the location has no association or still
// belongs to the selected Task. A stale request cannot detach another Task.
func (s *Store) Detach(rootID, publicID, path string) error {
	_, key, err := location(path)
	if err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return writeError("detach", err)
	}
	defer tx.Rollback()
	var target string
	err = tx.QueryRow("SELECT id FROM tasks WHERE root_id = ? AND task_id = ?", rootID, publicID).Scan(&target)
	if err == sql.ErrNoRows {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	var current string
	err = tx.QueryRow("SELECT task_id FROM worktrees WHERE path_key = ?", key).Scan(&current)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	if current != target {
		return fmt.Errorf("worktree is now associated with internal Task %s; review its current mapping before detaching: %w", current, ErrConflict)
	}
	if _, err := tx.Exec("DELETE FROM worktrees WHERE path_key = ? AND task_id = ?", key, target); err != nil {
		return writeError("detach", err)
	}
	if err := tx.Commit(); err != nil {
		return writeError("detach", err)
	}
	return nil
}
