// Package adopt verifies legacy directories before recording their identities.
package adopt

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/tintaungkhant/tasky/internal/config"
	"github.com/tintaungkhant/tasky/internal/git"
	"github.com/tintaungkhant/tasky/internal/store"
	"github.com/tintaungkhant/tasky/internal/task"
	"github.com/tintaungkhant/tasky/internal/work"
)

func canonical(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("configured path is empty; review `tasky setup`")
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(path)
}

func key(path string) string {
	if runtime.GOOS == "windows" {
		return strings.ToLower(path)
	}
	return path
}

// Plan performs no writes. Unknown directories and duplicate Task IDs require
// review instead of being interpreted as new identities.
func Plan(cfg *config.Config) ([]store.ImportRoot, error) {
	var roots []store.ImportRoot
	seenRoots := make(map[string]bool)
	for _, root := range cfg.Roots {
		if strings.TrimSpace(root.Name) == "" || root.TasksDir == "" {
			return nil, fmt.Errorf("Root name and Tasks Directory must be configured; review `tasky setup`")
		}
		path, err := canonical(root.Path)
		if err != nil {
			return nil, fmt.Errorf("Root %q: %w", root.Name, err)
		}
		if seenRoots[key(path)] {
			return nil, fmt.Errorf("duplicate Root %s; review YAML before import", path)
		}
		seenRoots[key(path)] = true
		planned := store.ImportRoot{Name: root.Name, Path: path}
		repos := make(map[string]bool)
		addRepo := func(path string) {
			if !repos[key(path)] {
				planned.Repos = append(planned.Repos, store.ImportRepo{Name: filepath.Base(path), Path: path})
				repos[key(path)] = true
			}
		}
		entries, err := os.ReadDir(path)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			candidate := filepath.Join(path, entry.Name())
			if !entry.IsDir() || !git.IsRepo(candidate) {
				continue
			}
			_, primary, err := git.VerifyWorktree(candidate)
			if err != nil {
				return nil, fmt.Errorf("review Repo %s: %w", candidate, err)
			}
			addRepo(primary)
		}
		legacyEntries, err := os.ReadDir(root.TasksDir)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		for _, entry := range legacyEntries {
			if entry.Type()&os.ModeSymlink != 0 {
				return nil, fmt.Errorf("review symbolic link in Tasks Directory: %s", entry.Name())
			}
		}
		taskEntries, err := task.ListEntries(root.TasksDir)
		if err != nil {
			return nil, err
		}
		seenTasks := make(map[string]bool)
		for _, entry := range taskEntries {
			if entry.ID == ".tasky" && entry.Name == "" && work.IsManagedDirectory(filepath.Join(root.TasksDir, ".tasky")) {
				continue
			}
			if entry.ID == "" || seenTasks[entry.ID] || strings.Count(task.TaskDirName(entry.ID, entry.Name), "@") > 1 {
				return nil, fmt.Errorf("ambiguous legacy Task %q; review its ID and name before import", entry.Label())
			}
			seenTasks[entry.ID] = true
			plannedTask := store.ImportTask{TaskID: entry.ID, Name: entry.Name}
			worktrees, err := task.ListWorktrees(root.TasksDir, task.TaskDirName(entry.ID, entry.Name))
			if err != nil {
				return nil, err
			}
			for _, wt := range worktrees {
				path, primary, err := git.VerifyWorktree(wt.Path)
				if err != nil {
					return nil, fmt.Errorf("review Task %q directory %s: %w", entry.ID, wt.Path, err)
				}
				addRepo(primary)
				identity, err := git.WorktreeIdentity(path)
				if err != nil {
					return nil, err
				}
				plannedTask.Worktrees = append(plannedTask.Worktrees, store.ImportWorktree{RepoPath: primary, Path: path, Identity: identity})
			}
			planned.Tasks = append(planned.Tasks, plannedTask)
		}
		roots = append(roots, planned)
	}
	return roots, nil
}
