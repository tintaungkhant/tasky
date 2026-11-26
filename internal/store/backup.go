package store

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
)

// MappingBackup is a versioned logical snapshot, independent of SQLite's file
// format. It deliberately contains no code, live branch state, or YAML settings.
type MappingBackup struct {
	Version   int        `json:"version"`
	Roots     []Root     `json:"roots"`
	Repos     []Repo     `json:"repos"`
	Tasks     []Task     `json:"tasks"`
	Worktrees []Worktree `json:"worktrees"`
}

func noPending(tx *sql.Tx) error {
	var n int
	if err := tx.QueryRow("SELECT (SELECT count(*) FROM creations) + (SELECT count(*) FROM cleanups)").Scan(&n); err != nil {
		return err
	}
	if n != 0 {
		return fmt.Errorf("pending creation or cleanup intent exists; inspect tasky pending for each Root, then recover or explicitly --forget it before backup or restore")
	}
	return nil
}

func readBackupRows(tx *sql.Tx, query string, scan func(*sql.Rows) error, args ...any) error {
	rows, err := tx.Query(query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		if err := scan(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}

// Backup reads one transactionally consistent snapshot. Pending operations must
// be settled first so a mapping-only backup cannot lose recovery intent.
func (s *Store) Backup() (MappingBackup, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return MappingBackup{}, writeError("backup", err)
	}
	defer tx.Rollback()
	if err := noPending(tx); err != nil {
		return MappingBackup{}, err
	}
	b := MappingBackup{Version: 1}
	if err := readBackupRows(tx, "SELECT id, name, path FROM roots ORDER BY id", func(rows *sql.Rows) error {
		var v Root
		if err := rows.Scan(&v.ID, &v.Name, &v.Path); err != nil {
			return err
		}
		b.Roots = append(b.Roots, v)
		return nil
	}); err != nil {
		return MappingBackup{}, err
	}
	if err := readBackupRows(tx, "SELECT id, root_id, name, path FROM repos ORDER BY id", func(rows *sql.Rows) error {
		var v Repo
		if err := rows.Scan(&v.ID, &v.RootID, &v.Name, &v.Path); err != nil {
			return err
		}
		b.Repos = append(b.Repos, v)
		return nil
	}); err != nil {
		return MappingBackup{}, err
	}
	if err := readBackupRows(tx, "SELECT id, root_id, task_id, name FROM tasks ORDER BY id", func(rows *sql.Rows) error {
		var v Task
		if err := rows.Scan(&v.ID, &v.RootID, &v.TaskID, &v.Name); err != nil {
			return err
		}
		b.Tasks = append(b.Tasks, v)
		return nil
	}); err != nil {
		return MappingBackup{}, err
	}
	if err := readBackupRows(tx, "SELECT id, task_id, repo_id, path, identity FROM worktrees ORDER BY id", func(rows *sql.Rows) error {
		var v Worktree
		if err := rows.Scan(&v.ID, &v.TaskID, &v.RepoID, &v.Path, &v.Identity); err != nil {
			return err
		}
		b.Worktrees = append(b.Worktrees, v)
		return nil
	}); err != nil {
		return MappingBackup{}, err
	}
	if err := tx.Commit(); err != nil {
		return MappingBackup{}, writeError("backup", err)
	}
	return b, nil
}

func backupLocation(path string) (string, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return "", fmt.Errorf("backup location must be an absolute, cleaned path: %q", path)
	}
	_, key, err := location(path)
	return key, err
}

func backupID(ids map[string]bool, id string) error {
	if id == "" || ids[id] {
		return fmt.Errorf("empty or duplicate backup identity %q", id)
	}
	ids[id] = true
	return nil
}

// Validate requires a self-contained snapshot; foreign references may not be
// satisfied accidentally by unrelated records already in the target database.
func (b MappingBackup) Validate() error {
	if b.Version != 1 {
		return fmt.Errorf("unsupported mapping backup version %d; use a compatible Tasky version", b.Version)
	}
	roots, repos, tasks, wts := map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}
	repoRoots, taskRoots := map[string]string{}, map[string]string{}
	for _, v := range b.Roots {
		if err := backupID(roots, v.ID); err != nil {
			return err
		}
		if _, err := backupLocation(v.Path); err != nil {
			return err
		}
	}
	for _, v := range b.Repos {
		if err := backupID(repos, v.ID); err != nil {
			return err
		}
		if !roots[v.RootID] {
			return fmt.Errorf("backup Repo %s references an absent Root", v.ID)
		}
		repoRoots[v.ID] = v.RootID
		if _, err := backupLocation(v.Path); err != nil {
			return err
		}
	}
	for _, v := range b.Tasks {
		if err := backupID(tasks, v.ID); err != nil {
			return err
		}
		if !roots[v.RootID] || v.TaskID == "" {
			return fmt.Errorf("backup Task %s has an invalid Root or Task ID", v.ID)
		}
		taskRoots[v.ID] = v.RootID
	}
	for _, v := range b.Worktrees {
		if err := backupID(wts, v.ID); err != nil {
			return err
		}
		if !tasks[v.TaskID] || !repos[v.RepoID] || taskRoots[v.TaskID] != repoRoots[v.RepoID] {
			return fmt.Errorf("backup worktree %s has invalid or cross-Root references", v.ID)
		}
		if _, err := backupLocation(v.Path); err != nil {
			return err
		}
	}
	return nil
}

// mergeBackupRow inserts or accepts an identical record. All SQL identifiers
// come from fixed call sites; backup values are always bound parameters.
func mergeBackupRow(tx *sql.Tx, table, fields string, values ...any) error {
	columns := strings.Split(fields, ",")
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(columns)), ",")
	if _, err := tx.Exec("INSERT INTO "+table+"("+fields+") VALUES("+placeholders+") ON CONFLICT DO NOTHING", values...); err != nil {
		return writeError("restore "+table, err)
	}
	conditions := make([]string, len(columns))
	for i, column := range columns {
		conditions[i] = column + " IS ?"
	}
	var count int
	if err := tx.QueryRow("SELECT count(*) FROM "+table+" WHERE "+strings.Join(conditions, " AND "), values...).Scan(&count); err != nil {
		return err
	}
	if count != 1 {
		return fmt.Errorf("restore conflicts with existing %s record %v; no mappings changed; review the backup and current mappings: %w", table, values[0], ErrConflict)
	}
	return nil
}

// Restore merges all mappings in one transaction. It never deletes, replaces,
// or renames existing records, and repeated identical restores are harmless.
func (s *Store) Restore(b MappingBackup) error {
	if err := b.Validate(); err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return writeError("restore", err)
	}
	defer tx.Rollback()
	if err := noPending(tx); err != nil {
		return err
	}
	for _, v := range b.Roots {
		key, _ := backupLocation(v.Path)
		if err := mergeBackupRow(tx, "roots", "id,name,path,path_key", v.ID, v.Name, v.Path, key); err != nil {
			return err
		}
	}
	for _, v := range b.Repos {
		key, _ := backupLocation(v.Path)
		if err := mergeBackupRow(tx, "repos", "id,root_id,name,path,path_key", v.ID, v.RootID, v.Name, v.Path, key); err != nil {
			return err
		}
	}
	taskRoots := map[string]string{}
	for _, v := range b.Tasks {
		taskRoots[v.ID] = v.RootID
		if err := mergeBackupRow(tx, "tasks", "id,root_id,task_id,name", v.ID, v.RootID, v.TaskID, v.Name); err != nil {
			return err
		}
	}
	for _, v := range b.Worktrees {
		key, _ := backupLocation(v.Path)
		if err := mergeBackupRow(tx, "worktrees", "id,root_id,task_id,repo_id,path,path_key,identity", v.ID, taskRoots[v.TaskID], v.TaskID, v.RepoID, v.Path, key, v.Identity); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return writeError("restore; retry the same backup after checking database availability", err)
	}
	return nil
}
