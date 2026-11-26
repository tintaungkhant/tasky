package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tintaungkhant/tasky/internal/app"
)

func runStoredDelete(cmd *cobra.Command, args []string, c *storedContext) error {
	t, err := selectedTask(cmd, args, c)
	if err != nil {
		return err
	}
	wts, byPath, repos, err := c.Worktrees(t.TaskID)
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
	for _, wt := range targets {
		mapping := byPath[wt.Path]
		removeBranch := deleteDeleteBranchFlag
		if len(args) == 0 {
			removeBranch, err = promptConfirmDefault(fmt.Sprintf("Delete the branch of %s too?", wt.Path), false)
			if err != nil {
				return err
			}
		}
		forceWorktree, _ := cmd.Flags().GetBool("force-worktree")
		forceBranch, _ := cmd.Flags().GetBool("force-branch")
		if err := c.Remove(mapping, app.RemoveOptions{DeleteBranch: removeBranch, ForceWorktree: deleteForceFlag || forceWorktree, ForceBranch: deleteForceFlag || forceBranch}); err != nil {
			return fmt.Errorf("earlier removals printed above completed; this and later targets may remain associated; %w", err)
		}
		cmd.Printf("Removed worktree and mapping %s (branch deletion requested: %t)\n", wt.Path, removeBranch)
	}
	return c.Store.RemoveEmptyTask(t.ID)
}

func runStoredVSCode(cmd *cobra.Command, args []string, c *storedContext) error {
	t, err := selectedTask(cmd, args, c)
	if err != nil {
		return err
	}
	result, err := c.GenerateWorkspace(t.TaskID)
	if err != nil {
		return err
	}
	printWorkspace(cmd, result)
	return nil
}
