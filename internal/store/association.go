package store

import (
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
)

// Attach atomically creates a Task if needed and associates a verified checkout.
// expectedSource is the internal Task identity required for reassignment. It
// acts as a compare-and-swap guard against stale or competing requests.
func (s *Store) Attach(rootID, publicID, name, repoID, path, expectedSource string, identities ...string) (Worktree, error) {
	identity := ""
	if len(identities) > 0 {
		identity = identities[0]
	}
	return s.attach(rootID, publicID, name, repoID, path, expectedSource, identity, nil)
}

// AssociationReview binds both the target identity (empty means not yet created)
// and the complete source mapping to what the caller inspected.
type AssociationReview struct {
	TargetID string
	Source   *Worktree
}

func (s *Store) AttachReviewed(rootID, publicID, name, repoID, path, from, identity string, review AssociationReview) (Worktree, error) {
	return s.attach(rootID, publicID, name, repoID, path, from, identity, &review)
}

func (s *Store) attach(rootID, publicID, name, repoID, path, expectedSource, identity string, review *AssociationReview) (Worktree, error) {
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
	if err := cleanupReserved(tx, key); err != nil {
		return Worktree{}, err
	}
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
	err = tx.QueryRow("SELECT id, task_id, repo_id, path, identity FROM worktrees WHERE path_key = ?", key).Scan(&current.ID, &current.TaskID, &current.RepoID, &current.Path, &current.Identity)
	if err != nil && err != sql.ErrNoRows {
		return Worktree{}, err
	}
	exists := err == nil
	if review != nil && (target != review.TargetID || exists != (review.Source != nil) || exists && current != *review.Source) {
		return Worktree{}, fmt.Errorf("association changed since review; refresh and review again: %w", ErrConflict)
	}
	if exists && identity != current.Identity {
		return Worktree{}, fmt.Errorf("identity mismatch or unverified legacy mapping; use reconnect after reviewing the location: %w", ErrConflict)
	}
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
	wt := Worktree{ID: current.ID, TaskID: target, RepoID: repoID, Path: path, Identity: identity}
	if exists {
		_, err = tx.Exec("UPDATE worktrees SET root_id = ?, task_id = ?, repo_id = ?, path = ? WHERE id = ?", rootID, target, repoID, path, current.ID)
	} else {
		wt.ID = rand.Text()
		_, err = tx.Exec("INSERT INTO worktrees(id, root_id, task_id, repo_id, path, path_key, identity) VALUES(?, ?, ?, ?, ?, ?, ?)", wt.ID, rootID, target, repoID, path, key, identity)
	}
	if err != nil {
		return Worktree{}, writeError("attach", err)
	}
	if err := tx.Commit(); err != nil {
		return Worktree{}, writeError("attach", err)
	}
	return wt, nil
}

// AssociationAt includes the source owner even when it belongs to another Root.
func (s *Store) AssociationAt(path string) (*Worktree, *Task, *Root, error) {
	_, key, err := location(path)
	if err != nil {
		return nil, nil, nil, err
	}
	var wt Worktree
	var task Task
	var root Root
	err = s.db.QueryRow(`SELECT w.id,w.task_id,w.repo_id,w.path,w.identity,
 t.id,t.root_id,t.task_id,t.name,r.id,r.name,r.path
 FROM worktrees w JOIN tasks t ON t.id=w.task_id JOIN roots r ON r.id=t.root_id
 WHERE w.path_key=?`, key).Scan(&wt.ID, &wt.TaskID, &wt.RepoID, &wt.Path, &wt.Identity,
		&task.ID, &task.RootID, &task.TaskID, &task.Name, &root.ID, &root.Name, &root.Path)
	if err == sql.ErrNoRows {
		return nil, nil, nil, nil
	}
	if err != nil {
		return nil, nil, nil, err
	}
	return &wt, &task, &root, nil
}

// DetachReviewed removes only this exact mapping, never a replacement at its path.
func (s *Store) DetachReviewed(rootID string, wt Worktree) error {
	_, key, err := location(wt.Path)
	if err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := cleanupReserved(tx, key); err != nil {
		return err
	}
	result, err := tx.Exec(`DELETE FROM worktrees WHERE root_id=? AND id=? AND task_id=? AND repo_id=? AND path=? AND identity=?`, rootID, wt.ID, wt.TaskID, wt.RepoID, wt.Path, wt.Identity)
	if err := changed("detach; mapping changed, refresh before retrying", result, err); err != nil {
		if errors.Is(err, ErrNotFound) {
			return fmt.Errorf("mapping changed; refresh before detaching: %w", ErrConflict)
		}
		return err
	}
	return tx.Commit()
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
	if err := cleanupReserved(tx, key); err != nil {
		return err
	}
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
