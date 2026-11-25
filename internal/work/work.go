// Package work coordinates persisted mappings with Git. Git and SQLite cannot
// share a transaction, so creation intent is persisted before running Git.
package work

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tintaungkhant/tasky/internal/git"
	"github.com/tintaungkhant/tasky/internal/store"
)

type Manager struct{ Store *store.Store }

const managedFormat = "tasky managed worktrees v1\n"

func TaskDirectory(tasksDir, id string) string { return filepath.Join(tasksDir, ".tasky", id) }

func IsManagedDirectory(path string) bool {
	data, err := os.ReadFile(filepath.Join(path, "format"))
	return err == nil && string(data) == managedFormat
}

func (m Manager) EnsureTask(rootID, taskID, name string) (store.Task, error) {
	task, err := m.Store.FindTask(rootID, taskID)
	if err == nil {
		return task, nil
	}
	if !errors.Is(err, store.ErrNotFound) {
		return store.Task{}, err
	}
	task, err = m.Store.CreateTask(rootID, taskID, name)
	if errors.Is(err, store.ErrConflict) {
		return m.Store.FindTask(rootID, taskID)
	}
	return task, err
}

func segment(id string) bool {
	return id != "" && strings.IndexFunc(id, func(c rune) bool {
		return !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-')
	}) < 0
}

func (m Manager) Create(task store.Task, repo store.Repo, tasksDir, branch, base string) (string, error) {
	if !segment(task.ID) {
		return "", fmt.Errorf("invalid internal Task identity")
	}
	branches, err := git.ListLocalBranches(repo.Path)
	if err != nil {
		return "", err
	}
	existing := false
	for _, name := range branches {
		if name == branch {
			existing = true
		}
	}
	busy, err := git.CheckedOutBranchPaths(repo.Path)
	if err != nil {
		return "", err
	}
	if path, ok := busy[branch]; ok {
		return "", fmt.Errorf("branch %q is already checked out at %s", branch, path)
	}
	if err := git.ValidateBranch(repo.Path, branch); err != nil {
		return "", err
	}
	dir, err := EnsureTaskDirectory(tasksDir, task.ID)
	if err != nil {
		return "", err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return "", err
	}
	defer root.Close()
	relative := rand.Text()
	path := filepath.Join(dir, relative)
	c, err := m.Store.ReserveCreation(task.ID, repo.ID, branch, path)
	if err != nil {
		return "", err
	}
	partial := func(err error) (string, error) {
		return path, fmt.Errorf("creation %s is pending at %s: %w; inspect `tasky pending --root %s`, then `tasky recover %s --root %s` to record a verified checkout or add --forget to abandon intent without deleting Git data", c.ID, path, err, task.RootID, c.ID, task.RootID)
	}
	if err := root.Mkdir(relative, 0o755); err != nil {
		return partial(err)
	}
	if existing {
		err = git.WorktreeAddExistingBranch(repo.Path, path, branch)
	} else {
		err = git.WorktreeAddNewBranch(repo.Path, path, branch, base)
	}
	if err != nil {
		return partial(err)
	}
	if err := m.Verify(repo, path); err != nil {
		return partial(err)
	}
	identity, err := git.WorktreeIdentity(path)
	if err != nil {
		return partial(err)
	}
	if err := m.Store.CompleteCreation(c.ID, identity); err != nil {
		return partial(err)
	}
	return path, nil
}

// EnsureTaskDirectory creates only owned directories beneath the selected
// Tasks Directory. OpenRoot constrains filesystem writes across symlinks.
func EnsureTaskDirectory(tasksDir, id string) (string, error) {
	if !segment(id) {
		return "", fmt.Errorf("invalid internal Task identity")
	}
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		return "", err
	}
	tasksDir, err := filepath.Abs(tasksDir)
	if err != nil {
		return "", err
	}
	tasksDir, err = filepath.EvalSymlinks(tasksDir)
	if err != nil {
		return "", err
	}
	root, err := os.OpenRoot(tasksDir)
	if err != nil {
		return "", err
	}
	defer root.Close()
	if err := root.Mkdir(".tasky", 0o755); err == nil {
		if err := root.WriteFile(filepath.Join(".tasky", "format"), []byte(managedFormat), 0o600); err != nil {
			return "", err
		}
	} else if !os.IsExist(err) {
		return "", err
	}
	info, err := root.Lstat(".tasky")
	if err != nil {
		return "", err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || !IsManagedDirectory(filepath.Join(tasksDir, ".tasky")) {
		return "", fmt.Errorf(".tasky exists without a valid managed directory marker; inspect it or select another Tasks Directory")
	}
	taskRelative := filepath.Join(".tasky", id)
	if err := root.Mkdir(taskRelative, 0o755); err != nil && !os.IsExist(err) {
		return "", err
	}
	info, err = root.Lstat(taskRelative)
	if err != nil {
		return "", err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("Task directory is not an ordinary directory; inspect %s", TaskDirectory(tasksDir, id))
	}
	return TaskDirectory(tasksDir, id), nil
}

func (m Manager) Verify(repo store.Repo, path string) error {
	_, primary, err := git.VerifyWorktree(path)
	if err != nil {
		return err
	}
	expected, err := git.PrimaryWorktree(repo.Path)
	if err != nil {
		return err
	}
	a, err := os.Stat(primary)
	if err != nil {
		return err
	}
	b, err := os.Stat(expected)
	if err != nil {
		return err
	}
	if !os.SameFile(a, b) {
		return fmt.Errorf("worktree %s belongs to another Repo", path)
	}
	return nil
}

func (m Manager) Recover(c store.Creation, repo store.Repo) error {
	if err := m.Verify(repo, c.Path); err != nil {
		return fmt.Errorf("creation remains pending: %w; inspect Git and the path, or use --forget to abandon intent", err)
	}
	branch, err := git.CurrentBranch(c.Path)
	if err != nil {
		return err
	}
	if branch != c.Branch {
		return fmt.Errorf("creation remains pending: expected branch %q, found %q; review before recovery", c.Branch, branch)
	}
	identity, err := git.WorktreeIdentity(c.Path)
	if err != nil {
		return err
	}
	return m.Store.CompleteCreation(c.ID, identity)
}
