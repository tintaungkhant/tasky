package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/tintaungkhant/tasky/internal/git"
	"github.com/tintaungkhant/tasky/internal/task"
	"github.com/tintaungkhant/tasky/internal/work"
)

func runStoredDelete(cmd *cobra.Command, args []string, c *storedContext) error {
	t, err := selectedTask(cmd, args, c)
	if err != nil {
		return err
	}
	wts, byPath, repos, err := c.worktrees(t.TaskID)
	if err != nil {
		return err
	}
	if len(wts) == 0 {
		return fmt.Errorf("no worktrees found for Task %s", t.TaskID)
	}
	repoArg, branchArg := "", ""
	if len(args) > 1 {
		repoArg = args[1]
	}
	if len(args) > 2 {
		branchArg = args[2]
	}
	if repoArg != "" {
		matchingIDs := make(map[string]bool)
		for i, wt := range wts {
			repo := repos[byPath[wt.Path].RepoID]
			if repo.ID == repoArg {
				wts[i].RepoName = repoArg
			}
			if repo.Name == repoArg || repo.ID == repoArg {
				matchingIDs[repo.ID] = true
			}
		}
		if len(matchingIDs) > 1 {
			return fmt.Errorf("Repo %q is ambiguous; use its ID from `tasky repo list`", repoArg)
		}
	}
	targets, err := selectDeleteTargets(wts, repoArg, branchArg, len(args) == 0)
	if err != nil {
		return err
	}
	if len(args) == 0 {
		ok, err := confirmDelete(targets)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("aborted")
		}
	}
	m := work.Manager{Store: c.Store}
	for _, wt := range targets {
		mapping := byPath[wt.Path]
		repo := repos[mapping.RepoID]
		if err := m.Verify(repo, wt.Path); err != nil {
			return fmt.Errorf("refusing removal of unverified location %s: %w", wt.Path, err)
		}
		branch, err := git.CurrentBranch(wt.Path)
		if err != nil {
			return err
		}
		if err := git.WorktreeRemove(repo.Path, wt.Path, deleteForceFlag); err != nil {
			return fmt.Errorf("earlier removals, if printed, succeeded; %w", err)
		}
		if err := c.Store.ForgetWorktree(mapping.ID); err != nil {
			return fmt.Errorf("Git removed %s but its mapping remains; detach the stale mapping after reviewing Git: %w", wt.Path, err)
		}
		cmd.Printf("Removed worktree %s\n", wt.Path)
		removeBranch := deleteDeleteBranchFlag
		if len(args) == 0 && branch != "" {
			removeBranch, err = promptConfirmDefault(fmt.Sprintf("Delete branch %q too?", branch), false)
			if err != nil {
				return err
			}
		}
		if removeBranch && branch != "" {
			if err := git.DeleteBranch(repo.Path, branch, deleteForceFlag); err != nil {
				return fmt.Errorf("worktree and mapping removed; branch %q remains: %w", branch, err)
			}
		}
		// Only remove an empty parent. Workspace files and unrelated files remain.
		parent := filepath.Dir(wt.Path)
		owned := work.TaskDirectory(c.Config.TasksDir, t.ID)
		legacy := filepath.Join(c.Config.TasksDir, task.TaskDirName(t.TaskID, t.Name))
		if parent == owned || parent == legacy {
			_ = os.Remove(parent)
		}
	}
	return c.Store.RemoveEmptyTask(t.ID)
}

func runStoredVSCode(cmd *cobra.Command, args []string, c *storedContext) error {
	t, err := selectedTask(cmd, args, c)
	if err != nil {
		return err
	}
	wts, _, _, err := c.worktrees(t.TaskID)
	if err != nil {
		return err
	}
	if len(wts) == 0 {
		return fmt.Errorf("no worktrees found for Task %s", t.TaskID)
	}
	dir := work.TaskDirectory(c.Config.TasksDir, t.ID)
	filename := t.ID + ".code-workspace"
	// Preserve the location used by an adopted legacy workspace.
	legacy := filepath.Join(c.Config.TasksDir, task.TaskDirName(t.TaskID, t.Name))
	if filepath.Dir(wts[0].Path) == legacy {
		dir = legacy
		filename = filepath.Base(legacy) + ".code-workspace"
	} else {
		dir, err = work.EnsureTaskDirectory(c.Config.TasksDir, t.ID)
		if err != nil {
			return err
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	ws := vscodeWorkspace{}
	for _, wt := range wts {
		rel, err := filepath.Rel(dir, wt.Path)
		if err != nil {
			ws.Folders = append(ws.Folders, vscodeFolder{Path: wt.Path})
			continue
		}
		ws.Folders = append(ws.Folders, vscodeFolder{Path: "./" + filepath.ToSlash(rel)})
	}
	data, err := json.MarshalIndent(ws, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(dir, filename)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if os.IsExist(err) {
		previous, readErr := os.ReadFile(path)
		if readErr == nil && bytes.Equal(previous, data) {
			cmd.Printf("Workspace file is current: %s\n", path)
			return nil
		}
		return fmt.Errorf("workspace file %s exists; preserve or move it before regenerating", path)
	}
	if err != nil {
		return err
	}
	_, writeErr := f.Write(data)
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	cmd.Printf("Wrote workspace file at %s\n", path)
	return nil
}
