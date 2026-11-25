// Package store persists Task identity and associations. It never changes Git
// state or infers identity from filesystem names.
package store

import (
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

var (
	ErrConflict = errors.New("record already exists; use the existing record or choose a different identity or location")
	ErrNotFound = errors.New("referenced Root, Task, or Repo does not exist in the same Root")
)

type Store struct{ db *sql.DB }

type Root struct{ ID, Name, Path string }
type Repo struct{ ID, RootID, Name, Path string }
type Task struct{ ID, RootID, TaskID, Name string }
type Worktree struct{ ID, TaskID, RepoID, Path string }

const schema = `
CREATE TABLE roots (
 id TEXT PRIMARY KEY NOT NULL,
 name TEXT NOT NULL,
 path TEXT NOT NULL,
 path_key TEXT NOT NULL UNIQUE
);
CREATE TABLE repos (
 id TEXT PRIMARY KEY NOT NULL,
 root_id TEXT NOT NULL REFERENCES roots(id),
 name TEXT NOT NULL,
 path TEXT NOT NULL,
 path_key TEXT NOT NULL,
 UNIQUE(root_id, path_key),
 UNIQUE(id, root_id)
);
CREATE TABLE tasks (
 id TEXT PRIMARY KEY NOT NULL,
 root_id TEXT NOT NULL REFERENCES roots(id),
 task_id TEXT NOT NULL CHECK(length(task_id) > 0),
 name TEXT NOT NULL,
 UNIQUE(root_id, task_id),
 UNIQUE(id, root_id)
);
CREATE TABLE worktrees (
 id TEXT PRIMARY KEY NOT NULL,
 root_id TEXT NOT NULL,
 task_id TEXT NOT NULL,
 repo_id TEXT NOT NULL,
 path TEXT NOT NULL,
 path_key TEXT NOT NULL UNIQUE,
 FOREIGN KEY(task_id, root_id) REFERENCES tasks(id, root_id),
 FOREIGN KEY(repo_id, root_id) REFERENCES repos(id, root_id)
);
CREATE INDEX worktrees_task ON worktrees(task_id);
PRAGMA user_version = 1;
`

// Open creates the database and parent directory if needed. Every connection
// enforces foreign keys and waits up to five seconds for another writer.
func Open(path string) (*Store, error) {
	path, _, err := location(path)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}
	uri := url.URL{Scheme: "file", Path: filepath.ToSlash(path)}
	if runtime.GOOS == "windows" {
		uri.Path = "/" + uri.Path
	}
	query := url.Values{}
	query.Add("_pragma", "foreign_keys(1)")
	query.Add("_pragma", "busy_timeout(5000)")
	query.Set("_txlock", "immediate")
	uri.RawQuery = query.Encode()
	db, err := sql.Open("sqlite", uri.String())
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.initialize(); err != nil {
		db.Close()
		return nil, writeError(fmt.Sprintf("initialize database %s", path), err)
	}
	return s, nil
}

func (s *Store) initialize() error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var version int
	if err := tx.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	switch version {
	case 0:
		if _, err := tx.Exec(schema); err != nil {
			return err
		}
	case 1:
	default:
		return fmt.Errorf("unsupported schema version %d; use a compatible Tasky version", version)
	}
	return tx.Commit()
}

func (s *Store) Close() error { return s.db.Close() }

// RemoveRoot forgets a Root and its mappings in one transaction. It never
// touches the filesystem or Git; callers must make the mapping loss explicit.
func (s *Store) RemoveRoot(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return writeError("remove Root", err)
	}
	defer tx.Rollback()
	for _, query := range []string{
		"DELETE FROM worktrees WHERE root_id = ?",
		"DELETE FROM tasks WHERE root_id = ?",
		"DELETE FROM repos WHERE root_id = ?",
	} {
		if _, err := tx.Exec(query, id); err != nil {
			return writeError("remove Root", err)
		}
	}
	result, err := tx.Exec("DELETE FROM roots WHERE id = ?", id)
	if err != nil {
		return writeError("remove Root", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("remove Root: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("remove Root: %w", ErrNotFound)
	}
	if err := tx.Commit(); err != nil {
		return writeError("remove Root", err)
	}
	return nil
}

func (s *Store) ListRoots() ([]Root, error) {
	rows, err := s.db.Query("SELECT id, name, path FROM roots ORDER BY name, id")
	if err != nil {
		return nil, fmt.Errorf("list Roots: %w", err)
	}
	defer rows.Close()
	var roots []Root
	for rows.Next() {
		var root Root
		if err := rows.Scan(&root.ID, &root.Name, &root.Path); err != nil {
			return nil, fmt.Errorf("read Root: %w", err)
		}
		roots = append(roots, root)
	}
	return roots, rows.Err()
}

func (s *Store) ListRepos(rootID string) ([]Repo, error) {
	rows, err := s.db.Query("SELECT id, root_id, name, path FROM repos WHERE root_id = ? ORDER BY name, id", rootID)
	if err != nil {
		return nil, fmt.Errorf("list Repos: %w", err)
	}
	defer rows.Close()
	var repos []Repo
	for rows.Next() {
		var repo Repo
		if err := rows.Scan(&repo.ID, &repo.RootID, &repo.Name, &repo.Path); err != nil {
			return nil, fmt.Errorf("read Repo: %w", err)
		}
		repos = append(repos, repo)
	}
	return repos, rows.Err()
}

func (s *Store) CreateRoot(name, path string) (Root, error) {
	path, key, err := location(path)
	if err != nil {
		return Root{}, err
	}
	root := Root{ID: rand.Text(), Name: name, Path: path}
	_, err = s.db.Exec("INSERT INTO roots(id, name, path, path_key) VALUES(?, ?, ?, ?)", root.ID, name, path, key)
	if err != nil {
		return Root{}, writeError("create Root", err)
	}
	return root, nil
}

func (s *Store) CreateRepo(rootID, name, path string) (Repo, error) {
	path, key, err := location(path)
	if err != nil {
		return Repo{}, err
	}
	repo := Repo{ID: rand.Text(), RootID: rootID, Name: name, Path: path}
	_, err = s.db.Exec("INSERT INTO repos(id, root_id, name, path, path_key) VALUES(?, ?, ?, ?, ?)", repo.ID, rootID, name, path, key)
	if err != nil {
		return Repo{}, writeError("create Repo", err)
	}
	return repo, nil
}

func (s *Store) CreateTask(rootID, taskID, name string) (Task, error) {
	if taskID == "" {
		return Task{}, errors.New("Task ID must not be empty")
	}
	task := Task{ID: rand.Text(), RootID: rootID, TaskID: taskID, Name: name}
	_, err := s.db.Exec("INSERT INTO tasks(id, root_id, task_id, name) VALUES(?, ?, ?, ?)", task.ID, rootID, taskID, name)
	if err != nil {
		return Task{}, writeError("create Task", err)
	}
	return task, nil
}

// Associate records a location supplied by the caller. Callers must verify Git
// registration before attaching a worktree; stored paths are not live Git state.
func (s *Store) Associate(taskID, repoID, path string) (Worktree, error) {
	path, key, err := location(path)
	if err != nil {
		return Worktree{}, err
	}
	wt := Worktree{ID: rand.Text(), TaskID: taskID, RepoID: repoID, Path: path}
	result, err := s.db.Exec(`INSERT INTO worktrees(id, root_id, task_id, repo_id, path, path_key)
 SELECT ?, tasks.root_id, tasks.id, repos.id, ?, ? FROM tasks
 JOIN repos ON repos.root_id = tasks.root_id WHERE tasks.id = ? AND repos.id = ?`, wt.ID, path, key, taskID, repoID)
	if err != nil {
		return Worktree{}, writeError("associate worktree", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return Worktree{}, fmt.Errorf("associate worktree: %w", err)
	}
	if n == 0 {
		return Worktree{}, fmt.Errorf("associate worktree: %w", ErrNotFound)
	}
	return wt, nil
}

func (s *Store) ListTasks(rootID string) ([]Task, error) {
	rows, err := s.db.Query("SELECT id, root_id, task_id, name FROM tasks WHERE root_id = ? ORDER BY task_id", rootID)
	if err != nil {
		return nil, fmt.Errorf("list Tasks: %w", err)
	}
	defer rows.Close()
	var tasks []Task
	for rows.Next() {
		var task Task
		if err := rows.Scan(&task.ID, &task.RootID, &task.TaskID, &task.Name); err != nil {
			return nil, fmt.Errorf("read Task: %w", err)
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

func (s *Store) ListWorktrees(taskID string) ([]Worktree, error) {
	rows, err := s.db.Query("SELECT id, task_id, repo_id, path FROM worktrees WHERE task_id = ? ORDER BY path", taskID)
	if err != nil {
		return nil, fmt.Errorf("list worktrees: %w", err)
	}
	defer rows.Close()
	var worktrees []Worktree
	for rows.Next() {
		var wt Worktree
		if err := rows.Scan(&wt.ID, &wt.TaskID, &wt.RepoID, &wt.Path); err != nil {
			return nil, fmt.Errorf("read worktree: %w", err)
		}
		worktrees = append(worktrees, wt)
	}
	return worktrees, rows.Err()
}

func location(path string) (string, string, error) {
	if path == "" {
		return "", "", errors.New("location must not be empty")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", "", fmt.Errorf("resolve location: %w", err)
	}
	key := absolute
	if runtime.GOOS == "windows" {
		key = strings.ToLower(key)
	}
	return absolute, key, nil
}

func writeError(action string, err error) error {
	var sqliteErr *sqlite.Error
	if errors.As(err, &sqliteErr) {
		switch sqliteErr.Code() {
		case sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY, sqlite3.SQLITE_CONSTRAINT_UNIQUE:
			err = ErrConflict
		case sqlite3.SQLITE_CONSTRAINT_FOREIGNKEY:
			err = ErrNotFound
		case sqlite3.SQLITE_BUSY, sqlite3.SQLITE_LOCKED:
			return fmt.Errorf("%s: database is busy; retry the command: %w", action, err)
		}
	}
	return fmt.Errorf("%s: %w", action, err)
}
