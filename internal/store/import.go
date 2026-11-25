package store

import (
	"crypto/rand"
	"database/sql"
	"fmt"
)

// Import records a completely verified legacy snapshot in one transaction.
// Existing identities must agree; retries never replace mappings or names.
type ImportRoot struct {
	Name, Path string
	Repos      []ImportRepo
	Tasks      []ImportTask
}
type ImportRepo struct{ Name, Path string }
type ImportTask struct {
	TaskID, Name string
	Worktrees    []ImportWorktree
}
type ImportWorktree struct{ RepoPath, Path, Identity string }

func (s *Store) Import(roots []ImportRoot) error {
	tx, err := s.db.Begin()
	if err != nil {
		return writeError("import", err)
	}
	defer tx.Rollback()
	for _, root := range roots {
		path, key, err := location(root.Path)
		if err != nil {
			return err
		}
		rootID, err := importIdentity(tx, "SELECT id, name FROM roots WHERE path_key = ?", []any{key}, root.Name,
			"INSERT INTO roots(id, name, path, path_key) VALUES(?, ?, ?, ?)", []any{root.Name, path, key})
		if err != nil {
			return fmt.Errorf("import Root %q: %w", root.Name, err)
		}
		repos := make(map[string]string)
		for _, repo := range root.Repos {
			path, key, err := location(repo.Path)
			if err != nil {
				return err
			}
			id, err := importIdentity(tx, "SELECT id, name FROM repos WHERE root_id = ? AND path_key = ?", []any{rootID, key}, repo.Name,
				"INSERT INTO repos(id, root_id, name, path, path_key) VALUES(?, ?, ?, ?, ?)", []any{rootID, repo.Name, path, key})
			if err != nil {
				return fmt.Errorf("import Repo %q: %w", repo.Name, err)
			}
			repos[key] = id
		}
		for _, task := range root.Tasks {
			if task.TaskID == "" {
				return fmt.Errorf("import: Task ID must not be empty")
			}
			id, err := importIdentity(tx, "SELECT id, name FROM tasks WHERE root_id = ? AND task_id = ?", []any{rootID, task.TaskID}, task.Name,
				"INSERT INTO tasks(id, root_id, task_id, name) VALUES(?, ?, ?, ?)", []any{rootID, task.TaskID, task.Name})
			if err != nil {
				return fmt.Errorf("import Task %q: %w", task.TaskID, err)
			}
			for _, wt := range task.Worktrees {
				_, repoKey, err := location(wt.RepoPath)
				if err != nil {
					return err
				}
				repoID, ok := repos[repoKey]
				if !ok {
					return fmt.Errorf("import: worktree %s has no verified Repo", wt.Path)
				}
				path, key, err := location(wt.Path)
				if err != nil {
					return err
				}
				var existingTask, existingRepo string
				err = tx.QueryRow("SELECT task_id, repo_id FROM worktrees WHERE path_key = ?", key).Scan(&existingTask, &existingRepo)
				if err == nil {
					if existingTask != id || existingRepo != repoID {
						return fmt.Errorf("import %s: association requires review: %w", path, ErrConflict)
					}
					continue
				}
				if err != sql.ErrNoRows {
					return err
				}
				if _, err := tx.Exec("INSERT INTO worktrees(id, root_id, task_id, repo_id, path, path_key, identity) VALUES(?, ?, ?, ?, ?, ?, ?)", rand.Text(), rootID, id, repoID, path, key, wt.Identity); err != nil {
					return writeError("import worktree", err)
				}
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return writeError("import", err)
	}
	return nil
}

func importIdentity(tx *sql.Tx, query string, lookup []any, name, insert string, values []any) (string, error) {
	var id, existingName string
	err := tx.QueryRow(query, lookup...).Scan(&id, &existingName)
	if err == nil {
		if existingName != name {
			return "", fmt.Errorf("name %q disagrees with %q; review before importing: %w", name, existingName, ErrConflict)
		}
		return id, nil
	}
	if err != sql.ErrNoRows {
		return "", err
	}
	id = rand.Text()
	_, err = tx.Exec(insert, append([]any{id}, values...)...)
	if err != nil {
		return "", writeError("import identity", err)
	}
	return id, nil
}
