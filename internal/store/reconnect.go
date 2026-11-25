package store

import "fmt"

// Reconnect preserves identity and ownership. A stale snapshot cannot overwrite
// a concurrent move, detach, or reassignment.
func (s *Store) Reconnect(wt Worktree, path, identity string) error {
	path, key, err := location(path)
	if err != nil {
		return err
	}
	if identity == "" || wt.Identity != "" && identity != wt.Identity {
		return fmt.Errorf("identity mismatch: %w", ErrConflict)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, oldKey, err := location(wt.Path)
	if err != nil {
		return err
	}
	for _, candidate := range []string{oldKey, key} {
		if err := cleanupReserved(tx, candidate); err != nil {
			return err
		}
	}
	result, err := tx.Exec(`UPDATE worktrees SET path = ?, path_key = ?, identity = ?
 WHERE id = ? AND task_id = ? AND repo_id = ? AND path = ? AND identity = ?
 AND NOT EXISTS (SELECT 1 FROM creations WHERE path_key = ?)`, path, key, identity, wt.ID, wt.TaskID, wt.RepoID, wt.Path, wt.Identity, key)
	if err := changed("reconnect; mapping changed or location reserved, inspect before retrying", result, err); err != nil {
		return err
	}
	return tx.Commit()
}
