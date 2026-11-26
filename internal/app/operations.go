package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/tintaungkhant/tasky/internal/git"
	"github.com/tintaungkhant/tasky/internal/store"
	"github.com/tintaungkhant/tasky/internal/task"
	"github.com/tintaungkhant/tasky/internal/work"
)

func (c *Context) Task(id string) (store.Task, error) {
	t, err := c.Store.FindTask(c.Root.ID, id)
	if errors.Is(err, store.ErrNotFound) {
		return store.Task{}, fmt.Errorf("Task %q not found in Root %s", id, c.Root.Name)
	}
	return t, err
}

type CreateRequest struct {
	TaskID, Name, Repo, Branch, Base string
	AsIs                             bool
}

type CreatedWorktree struct {
	Path, Branch string
}

func (c *Context) Branch(repo store.Repo, taskID, name, branch string, asIs bool) (string, error) {
	branches, err := git.ListLocalBranches(repo.Path)
	if err != nil {
		return "", err
	}
	if !asIs && !slices.Contains(branches, branch) {
		branch = task.RenderBranch(c.Config.BranchTemplate, taskID, name, branch)
	}
	return branch, git.ValidateBranch(repo.Path, branch)
}

func (c *Context) Create(req CreateRequest) (CreatedWorktree, error) {
	ctx, err := c.Registry.Resolve(c.Cwd, c.Root.ID, req.Repo)
	if err != nil {
		return CreatedWorktree{}, err
	}
	name := req.Name
	existing, err := c.Store.FindTask(c.Root.ID, req.TaskID)
	if err == nil {
		name = existing.Name
	} else if !errors.Is(err, store.ErrNotFound) {
		return CreatedWorktree{}, err
	}
	base := req.Base
	if base == "" {
		base, err = git.CurrentBranch(ctx.Repo.Path)
		if err != nil {
			return CreatedWorktree{}, err
		}
	}
	branch, err := c.Branch(ctx.Repo, req.TaskID, name, req.Branch, req.AsIs)
	if err != nil {
		return CreatedWorktree{}, err
	}
	m := work.Manager{Store: c.Store}
	t, err := m.EnsureTask(c.Root.ID, req.TaskID, name)
	if err != nil {
		return CreatedWorktree{}, err
	}
	path, err := m.Create(t, ctx.Repo, c.Config.TasksDir, branch, base)
	return CreatedWorktree{Path: path, Branch: branch}, err
}

func (c *Context) Rename(taskID, name string) error {
	t, err := c.Store.FindTask(c.Root.ID, taskID)
	if err != nil {
		return err
	}
	return c.Store.RenameTask(t.ID, name)
}

func (c *Context) Attach(taskID, location, repoSelector, name, from string) (store.Worktree, error) {
	path, _, err := git.VerifyWorktree(c.location(location))
	if err != nil {
		return store.Worktree{}, fmt.Errorf("attach made no mapping changes: %w", err)
	}
	ctx, err := c.Registry.Resolve(path, c.Root.ID, repoSelector)
	if err != nil {
		return store.Worktree{}, err
	}
	if err := (work.Manager{Store: c.Store}).Verify(ctx.Repo, path); err != nil {
		return store.Worktree{}, fmt.Errorf("attach made no mapping changes: %w", err)
	}
	identity, err := git.WorktreeIdentity(path)
	if err != nil {
		return store.Worktree{}, err
	}
	return c.Store.Attach(c.Root.ID, taskID, name, ctx.Repo.ID, path, from, identity)
}

func (c *Context) Detach(taskID, location string) (string, error) {
	if location == "" {
		return "", fmt.Errorf("worktree path must not be empty")
	}
	path, err := filepath.Abs(c.location(location))
	if err != nil {
		return "", err
	}
	// Missing locations still need to be detachable using their recorded path.
	if canonical, err := filepath.EvalSymlinks(path); err == nil {
		path = canonical
	}
	return path, c.Store.Detach(c.Root.ID, taskID, path)
}

func (c *Context) Reconnect(taskID, oldPath, newPath string, accept bool) (store.Worktree, error) {
	_, mappings, repos, err := c.Worktrees(taskID)
	if err != nil {
		return store.Worktree{}, err
	}
	old, err := filepath.Abs(c.location(oldPath))
	if err != nil {
		return store.Worktree{}, err
	}
	for _, wt := range mappings {
		if filepath.Clean(wt.Path) != old {
			continue
		}
		return wt, (work.Manager{Store: c.Store}).Reconnect(repos[wt.RepoID], wt, c.location(newPath), accept)
	}
	return store.Worktree{}, fmt.Errorf("no mapping at %s for Task %s; inspect tasky list before retrying", old, taskID)
}

func (c *Context) location(path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(c.Cwd, path)
}

type RemoveOptions struct{ DeleteBranch, ForceWorktree, ForceBranch bool }

// Remove uses the expected mapping snapshot so stale selections cannot remove
// a checkout that was reassigned or reconnected after the caller inspected it.
func (c *Context) Remove(expected store.Worktree, options RemoveOptions) error {
	tasks, err := c.Store.ListTasks(c.Root.ID)
	if err != nil {
		return err
	}
	for _, t := range tasks {
		if t.ID != expected.TaskID {
			continue
		}
		repos, err := c.Store.ListRepos(c.Root.ID)
		if err != nil {
			return err
		}
		for _, repo := range repos {
			if repo.ID != expected.RepoID {
				continue
			}
			if err := (work.Manager{Store: c.Store}).Remove(repo, expected, options.DeleteBranch, options.ForceWorktree, options.ForceBranch); err != nil {
				return err
			}
			// Never recursively remove a Task directory or its editor/user files.
			parent := filepath.Dir(expected.Path)
			owned := work.TaskDirectory(c.Config.TasksDir, t.ID)
			legacy := filepath.Join(c.Config.TasksDir, task.TaskDirName(t.TaskID, t.Name))
			if parent == owned || parent == legacy {
				_ = os.Remove(parent)
			}
			return nil
		}
	}
	return fmt.Errorf("worktree mapping does not belong to Root %s", c.Root.Name)
}

type RecoveryOptions struct{ Forget, ForceWorktree, ForceBranch bool }

// Recover returns true only when a cleanup was completed (rather than forgotten).
func (c *Context) Recover(id string, options RecoveryOptions) (bool, error) {
	cleanups, err := c.Store.ListCleanups(c.Root.ID)
	if err != nil {
		return false, err
	}
	for _, op := range cleanups {
		if op.ID != id {
			continue
		}
		if options.Forget {
			return false, c.Store.ForgetCleanup(op.ID)
		}
		repos, err := c.Store.ListRepos(c.Root.ID)
		if err != nil {
			return false, err
		}
		for _, repo := range repos {
			if repo.ID == op.RepoID {
				return true, (work.Manager{Store: c.Store}).RecoverCleanup(repo, op, options.ForceWorktree, options.ForceBranch)
			}
		}
	}
	operations, err := c.Store.ListCreations(c.Root.ID)
	if err != nil {
		return false, err
	}
	for _, op := range operations {
		if op.ID != id {
			continue
		}
		if options.Forget {
			return false, c.Store.ForgetCreation(op.ID)
		}
		repos, err := c.Store.ListRepos(c.Root.ID)
		if err != nil {
			return false, err
		}
		for _, repo := range repos {
			if repo.ID == op.RepoID {
				return false, (work.Manager{Store: c.Store}).Recover(op, repo)
			}
		}
	}
	return false, fmt.Errorf("pending operation %q not found; use `tasky pending`", id)
}
