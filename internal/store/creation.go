package store

import (
	"crypto/rand"
	"database/sql"
	"fmt"
)

const creationSchema = `
CREATE TABLE creations (
 id TEXT PRIMARY KEY NOT NULL,
 root_id TEXT NOT NULL,
 task_id TEXT NOT NULL,
 repo_id TEXT NOT NULL,
 branch TEXT NOT NULL,
 path TEXT NOT NULL,
 path_key TEXT NOT NULL UNIQUE,
 UNIQUE(repo_id, branch),
 FOREIGN KEY(task_id, root_id) REFERENCES tasks(id, root_id),
 FOREIGN KEY(repo_id, root_id) REFERENCES repos(id, root_id)
);
PRAGMA user_version = 2;
`

type Creation struct{ ID, RootID, TaskID, RepoID, Branch, Path string }

func (s *Store) FindTask(rootID, taskID string) (Task, error) {
	var task Task
	err := s.db.QueryRow("SELECT id, root_id, task_id, name FROM tasks WHERE root_id = ? AND task_id = ?", rootID, taskID).Scan(&task.ID, &task.RootID, &task.TaskID, &task.Name)
	if err == sql.ErrNoRows {
		return Task{}, ErrNotFound
	}
	return task, err
}

func (s *Store) RenameTask(id, name string) error {
	result, err := s.db.Exec("UPDATE tasks SET name = ? WHERE id = ?", name, id)
	return changed("rename Task", result, err)
}

func changed(action string, result sql.Result, err error) error {
	if err != nil {
		return writeError(action, err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s: %w", action, err)
	}
	if n == 0 {
		return fmt.Errorf("%s: %w", action, ErrNotFound)
	}
	return nil
}

func (s *Store) ForgetWorktree(id string) error {
	result, err := s.db.Exec("DELETE FROM worktrees WHERE id = ?", id)
	return changed("forget worktree", result, err)
}

func (s *Store) RemoveEmptyTask(id string) error {
	_, err := s.db.Exec("DELETE FROM tasks WHERE id = ? AND NOT EXISTS (SELECT 1 FROM worktrees WHERE task_id = ?) AND NOT EXISTS (SELECT 1 FROM creations WHERE task_id = ?)", id, id, id)
	if err != nil {
		return writeError("remove empty Task", err)
	}
	return nil
}

// ReserveCreation records intent before Git runs. Its unique Repo/branch key
// serializes competing creates and survives an interrupted process.
func (s *Store) ReserveCreation(taskID, repoID, branch, path string) (Creation, error) {
	path, key, err := location(path)
	if err != nil {
		return Creation{}, err
	}
	c := Creation{ID: rand.Text(), TaskID: taskID, RepoID: repoID, Branch: branch, Path: path}
	err = s.db.QueryRow("SELECT tasks.root_id FROM tasks JOIN repos ON tasks.root_id = repos.root_id WHERE tasks.id = ? AND repos.id = ?", taskID, repoID).Scan(&c.RootID)
	if err == sql.ErrNoRows {
		return Creation{}, ErrNotFound
	}
	if err != nil {
		return Creation{}, err
	}
	_, err = s.db.Exec("INSERT INTO creations(id, root_id, task_id, repo_id, branch, path, path_key) VALUES(?, ?, ?, ?, ?, ?, ?)", c.ID, c.RootID, taskID, repoID, branch, path, key)
	if err != nil {
		return Creation{}, writeError("reserve creation; inspect `tasky pending --root <Root>` before retrying", err)
	}
	return c, nil
}

func (s *Store) ListCreations(rootID string) ([]Creation, error) {
	rows, err := s.db.Query("SELECT id, root_id, task_id, repo_id, branch, path FROM creations WHERE root_id = ? ORDER BY id", rootID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Creation
	for rows.Next() {
		var c Creation
		if err := rows.Scan(&c.ID, &c.RootID, &c.TaskID, &c.RepoID, &c.Branch, &c.Path); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// CompleteCreation atomically exchanges a verified intent for an association.
func (s *Store) CompleteCreation(id string, identities ...string) error {
	identity := ""
	if len(identities) > 0 {
		identity = identities[0]
	}
	tx, err := s.db.Begin()
	if err != nil {
		return writeError("complete creation", err)
	}
	defer tx.Rollback()
	result, err := tx.Exec(`INSERT INTO worktrees(id, root_id, task_id, repo_id, path, path_key, identity)
 SELECT id, root_id, task_id, repo_id, path, path_key, ? FROM creations WHERE id = ?`, identity, id)
	if err := changed("complete creation", result, err); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM creations WHERE id = ?", id); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return writeError("complete creation", err)
	}
	return nil
}

// ForgetCreation abandons intent only. It never removes Git data.
func (s *Store) ForgetCreation(id string) error {
	result, err := s.db.Exec("DELETE FROM creations WHERE id = ?", id)
	return changed("forget creation", result, err)
}
