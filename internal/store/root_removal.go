package store

import (
	"database/sql"
	"fmt"
)

type RootMappingSnapshot struct {
	Root      Root
	Repos     []Repo
	Tasks     []Task
	Worktrees []Worktree
}

// RootMappings and confirmed removal use the same snapshot under SQLite's
// writer transaction, so confirmation cannot delete newly added mappings.
func (s *Store) RootMappings(id string) (RootMappingSnapshot, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return RootMappingSnapshot{}, err
	}
	defer tx.Rollback()
	result, err := rootMappings(tx, id)
	if err != nil {
		return result, err
	}
	return result, tx.Commit()
}

func rootMappings(tx *sql.Tx, id string) (RootMappingSnapshot, error) {
	out := RootMappingSnapshot{Repos: []Repo{}, Tasks: []Task{}, Worktrees: []Worktree{}}
	err := tx.QueryRow("SELECT id,name,path FROM roots WHERE id = ?", id).Scan(&out.Root.ID, &out.Root.Name, &out.Root.Path)
	if err == sql.ErrNoRows {
		return out, ErrNotFound
	}
	if err != nil {
		return out, err
	}
	var pending int
	if err := tx.QueryRow("SELECT (SELECT count(*) FROM creations WHERE root_id = ?) + (SELECT count(*) FROM cleanups WHERE root_id = ?)", id, id).Scan(&pending); err != nil {
		return out, err
	}
	if pending != 0 {
		return out, fmt.Errorf("Root has pending operations; inspect tasky pending and recover or forget their intent before removal: %w", ErrConflict)
	}
	if err := readBackupRows(tx, "SELECT id,root_id,name,path FROM repos WHERE root_id = ? ORDER BY id", func(rows *sql.Rows) error {
		var v Repo
		if err := rows.Scan(&v.ID, &v.RootID, &v.Name, &v.Path); err != nil {
			return err
		}
		out.Repos = append(out.Repos, v)
		return nil
	}, id); err != nil {
		return out, err
	}
	if err := readBackupRows(tx, "SELECT id,root_id,task_id,name FROM tasks WHERE root_id = ? ORDER BY id", func(rows *sql.Rows) error {
		var v Task
		if err := rows.Scan(&v.ID, &v.RootID, &v.TaskID, &v.Name); err != nil {
			return err
		}
		out.Tasks = append(out.Tasks, v)
		return nil
	}, id); err != nil {
		return out, err
	}
	if err := readBackupRows(tx, "SELECT id,task_id,repo_id,path,identity FROM worktrees WHERE root_id = ? ORDER BY id", func(rows *sql.Rows) error {
		var v Worktree
		if err := rows.Scan(&v.ID, &v.TaskID, &v.RepoID, &v.Path, &v.Identity); err != nil {
			return err
		}
		out.Worktrees = append(out.Worktrees, v)
		return nil
	}, id); err != nil {
		return out, err
	}
	return out, nil
}
