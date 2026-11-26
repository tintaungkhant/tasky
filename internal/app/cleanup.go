package app

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/tintaungkhant/tasky/internal/store"
	"github.com/tintaungkhant/tasky/internal/task"
	"github.com/tintaungkhant/tasky/internal/work"
)

type RemoveOptions struct{ DeleteBranch, ForceWorktree, ForceBranch bool }

type CleanupReview struct {
	RootID  string
	Targets []work.RemovalReview
	Options RemoveOptions
}

type CleanupResult struct {
	Completed []store.Worktree
	Failed    *store.Worktree
	Error     string
	Remaining []store.Worktree
}

func (c *Context) mappingInRoot(want store.Worktree) (store.Task, store.Repo, store.Worktree, error) {
	tasks, err := c.Store.ListTasks(c.Root.ID)
	if err != nil {
		return store.Task{}, store.Repo{}, store.Worktree{}, err
	}
	repos, err := c.Store.ListRepos(c.Root.ID)
	if err != nil {
		return store.Task{}, store.Repo{}, store.Worktree{}, err
	}
	var taskRow store.Task
	for _, t := range tasks {
		if t.ID == want.TaskID {
			taskRow = t
		}
	}
	var repo store.Repo
	for _, r := range repos {
		if r.ID == want.RepoID {
			repo = r
		}
	}
	if taskRow.ID == "" || repo.ID == "" {
		return store.Task{}, store.Repo{}, store.Worktree{}, fmt.Errorf("worktree mapping changed or does not belong to Root %s: %w", c.Root.Name, store.ErrConflict)
	}
	wts, err := c.Store.ListWorktrees(taskRow.ID)
	if err != nil {
		return store.Task{}, store.Repo{}, store.Worktree{}, err
	}
	for _, actual := range wts {
		if actual.ID == want.ID {
			if actual != want {
				return store.Task{}, store.Repo{}, store.Worktree{}, fmt.Errorf("worktree mapping changed since review: %w", store.ErrConflict)
			}
			return taskRow, repo, actual, nil
		}
	}
	return store.Task{}, store.Repo{}, store.Worktree{}, fmt.Errorf("worktree mapping changed or does not belong to Root %s: %w", c.Root.Name, store.ErrConflict)
}

func (c *Context) ReviewCleanup(mappings []store.Worktree, options RemoveOptions) (CleanupReview, error) {
	if len(mappings) == 0 {
		return CleanupReview{}, fmt.Errorf("select at least one worktree mapping")
	}
	seen := make(map[string]bool, len(mappings))
	review := CleanupReview{RootID: c.Root.ID, Options: options}
	m := work.Manager{Store: c.Store}
	for _, mapping := range mappings {
		if mapping.ID == "" || seen[mapping.ID] {
			return CleanupReview{}, fmt.Errorf("cleanup mappings must be complete and unique")
		}
		seen[mapping.ID] = true
		_, repo, actual, err := c.mappingInRoot(mapping)
		if err != nil {
			return CleanupReview{}, err
		}
		target, err := m.ReviewRemoval(repo, actual)
		if err != nil {
			return CleanupReview{}, err
		}
		review.Targets = append(review.Targets, target)
	}
	return review, nil
}

func (c *Context) ExecuteCleanup(review CleanupReview) (CleanupResult, error) {
	result := CleanupResult{Completed: []store.Worktree{}, Remaining: []store.Worktree{}}
	if review.RootID != c.Root.ID || len(review.Targets) == 0 {
		return result, fmt.Errorf("cleanup review does not belong to this Root or has no targets: %w", store.ErrConflict)
	}
	// Revalidate the whole selection before the first destructive action.
	seen := make(map[string]bool, len(review.Targets))
	for _, target := range review.Targets {
		if target.Mapping.ID == "" || seen[target.Mapping.ID] {
			return result, fmt.Errorf("cleanup review has duplicate or incomplete mappings: %w", store.ErrConflict)
		}
		seen[target.Mapping.ID] = true
		_, repo, actual, err := c.mappingInRoot(target.Mapping)
		if err == nil {
			var current work.RemovalReview
			current, err = (work.Manager{Store: c.Store}).ReviewRemoval(repo, actual)
			if err == nil && current != target {
				err = fmt.Errorf("cleanup review is stale: branch, revision, or Repo identity changed: %w", store.ErrConflict)
			}
		}
		if err != nil {
			return result, err
		}
	}
	cleanups, err := c.Store.ListCleanups(c.Root.ID)
	if err != nil {
		return result, err
	}
	creations, err := c.Store.ListCreations(c.Root.ID)
	if err != nil {
		return result, err
	}
	for _, target := range review.Targets {
		for _, op := range cleanups {
			if op.WorktreeID == target.Mapping.ID {
				return result, fmt.Errorf("cleanup %s is already pending for %s; recover it or forget its intent before retrying: %w", op.ID, op.Path, store.ErrConflict)
			}
		}
		for _, op := range creations {
			if op.TaskID == target.Mapping.TaskID && op.RepoID == target.Mapping.RepoID && op.Path == target.Mapping.Path {
				return result, fmt.Errorf("creation %s is pending for %s; recover it or forget its intent before retrying: %w", op.ID, op.Path, store.ErrConflict)
			}
		}
	}
	for i, target := range review.Targets {
		t, repo, _, err := c.mappingInRoot(target.Mapping)
		if err == nil {
			err = (work.Manager{Store: c.Store}).RemoveReviewed(repo, target, review.Options.DeleteBranch, review.Options.ForceWorktree, review.Options.ForceBranch)
		}
		if err != nil {
			failed := target.Mapping
			result.Failed = &failed
			result.Error = err.Error()
			for _, rest := range review.Targets[i+1:] {
				result.Remaining = append(result.Remaining, rest.Mapping)
			}
			return result, nil
		}
		cleanupTaskParent(c, t, target.Mapping)
		result.Completed = append(result.Completed, target.Mapping)
	}
	return result, nil
}

func cleanupTaskParent(c *Context, t store.Task, wt store.Worktree) {
	parent := filepath.Dir(wt.Path)
	owned := work.TaskDirectory(c.Config.TasksDir, t.ID)
	legacy := filepath.Join(c.Config.TasksDir, task.TaskDirName(t.TaskID, t.Name))
	if parent == owned || parent == legacy {
		_ = os.Remove(parent)
	}
}

// Remove preserves the CLI's existing single-mapping operation through the reviewed path.
func (c *Context) Remove(expected store.Worktree, options RemoveOptions) error {
	t, repo, actual, err := c.mappingInRoot(expected)
	if err != nil {
		return err
	}
	if err := (work.Manager{Store: c.Store}).Remove(repo, actual, options.DeleteBranch, options.ForceWorktree, options.ForceBranch); err != nil {
		return err
	}
	cleanupTaskParent(c, t, actual)
	return nil
}
