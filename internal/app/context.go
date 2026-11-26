// Package app coordinates Tasky operations independently of CLI or HTTP presentation.
package app

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/tintaungkhant/tasky/internal/config"
	"github.com/tintaungkhant/tasky/internal/registry"
	"github.com/tintaungkhant/tasky/internal/store"
	"github.com/tintaungkhant/tasky/internal/task"
)

type Context struct {
	registry.Registry
	Root   store.Root
	Config config.Root
	Cwd    string
}

// Unadopted YAML Roots retain the legacy workflow. A matching registered Root
// always uses persisted identity; errors never silently fall back to names.
func OpenRegistered(path, cwd, selector string) (*Context, error) {
	dbPath := DatabasePath(path)
	if _, err := os.Stat(dbPath); os.IsNotExist(err) && selector == "" {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	s, err := store.Open(dbPath)
	if err != nil {
		return nil, err
	}
	r := registry.Registry{Store: s}
	root, selectErr := r.ResolveRoot(cwd, selector)
	cfg, err := config.LoadFile(path)
	if err != nil {
		s.Close()
		return nil, err
	}
	if selectErr != nil {
		if selector == "" {
			legacy, legacyErr := cfg.MatchRoot(cwd)
			if legacyErr == nil {
				roots, err := s.ListRoots()
				if err != nil {
					s.Close()
					return nil, err
				}
				registered := false
				for _, candidate := range roots {
					if SameLocation(candidate.Path, legacy.Path) {
						registered = true
					}
				}
				if !registered {
					s.Close()
					return nil, nil
				}
			}
		}
		s.Close()
		return nil, selectErr
	}
	settings := config.Root{Name: root.Name, Path: root.Path, TasksDir: filepath.Join(root.Path, "tasks"), BranchTemplate: "{branch}"}
	matches := 0
	for _, configured := range cfg.Roots {
		if SameLocation(configured.Path, root.Path) {
			settings = configured
			matches++
		}
	}
	if matches > 1 {
		s.Close()
		return nil, fmt.Errorf("multiple YAML settings match Root %q; review `tasky setup`", root.Name)
	}
	if settings.TasksDir == "" {
		s.Close()
		return nil, fmt.Errorf("Tasks Directory is empty; review `tasky setup`")
	}
	return &Context{Registry: r, Root: root, Config: settings, Cwd: cwd}, nil
}

func SameLocation(a, b string) bool {
	x, err := os.Stat(a)
	if err != nil {
		return false
	}
	y, err := os.Stat(b)
	return err == nil && os.SameFile(x, y)
}

func (c *Context) Entries() ([]task.Entry, error) {
	tasks, err := c.Store.ListTasks(c.Root.ID)
	if err != nil {
		return nil, err
	}
	var entries []task.Entry
	for _, t := range tasks {
		entries = append(entries, task.Entry{ID: t.TaskID, Name: t.Name})
	}
	return entries, nil
}

func (c *Context) Worktrees(taskID string) ([]task.WorktreeEntry, map[string]store.Worktree, map[string]store.Repo, error) {
	t, err := c.Store.FindTask(c.Root.ID, taskID)
	if err != nil {
		return nil, nil, nil, err
	}
	return c.worktreesForTask(t)
}

func (c *Context) worktreesForTask(t store.Task) ([]task.WorktreeEntry, map[string]store.Worktree, map[string]store.Repo, error) {
	wts, err := c.Store.ListWorktrees(t.ID)
	if err != nil {
		return nil, nil, nil, err
	}
	repos, err := c.Store.ListRepos(c.Root.ID)
	if err != nil {
		return nil, nil, nil, err
	}
	byRepo, byPath := make(map[string]store.Repo), make(map[string]store.Worktree)
	for _, repo := range repos {
		byRepo[repo.ID] = repo
	}
	var entries []task.WorktreeEntry
	for _, wt := range wts {
		entries = append(entries, task.WorktreeEntry{RepoName: byRepo[wt.RepoID].Name, Path: wt.Path})
		byPath[wt.Path] = wt
	}
	return entries, byPath, byRepo, nil
}
