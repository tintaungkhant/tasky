package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/tintaungkhant/tasky/internal/config"
	"github.com/tintaungkhant/tasky/internal/git"
	"github.com/tintaungkhant/tasky/internal/task"
	"github.com/tintaungkhant/tasky/internal/ui"
)

var deleteForceFlag bool
var deleteDeleteBranchFlag bool

// scopeAllOptionValue is the sentinel option value for "whole task" in the
// interactive worktree-scope prompt (see promptSelectWorktreeScope).
const scopeAllOptionValue = "\x00all"

var deleteCmd = &cobra.Command{
	Use:   "delete [task-id] [repo] [branch]",
	Short: "Remove a task's worktree(s), and optionally their branch(es)",
	Args:  cobra.MaximumNArgs(3),
	RunE:  runDelete,
}

func init() {
	deleteCmd.Flags().BoolVar(&deleteForceFlag, "force", false, "force-remove worktrees/branches with uncommitted or unmerged changes")
	deleteCmd.Flags().BoolVar(&deleteDeleteBranchFlag, "delete-branch", false, "also delete each removed worktree's branch (non-interactive only)")
	rootCmd.AddCommand(deleteCmd)
}

func runDelete(cmd *cobra.Command, args []string) error {
	if c, err := registeredContext(cmd); err != nil {
		return err
	} else if c != nil {
		defer c.Store.Close()
		return runStoredDelete(cmd, args, c)
	}
	interactive := len(args) == 0

	var taskID, repoArg, branchArg string
	switch len(args) {
	case 3:
		taskID, repoArg, branchArg = args[0], args[1], args[2]
	case 2:
		taskID, repoArg = args[0], args[1]
	case 1:
		taskID = args[0]
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	root, err := cfg.MatchRoot(cwd)
	if err != nil {
		return err
	}

	taskEntries, err := task.ListEntries(root.TasksDir)
	if err != nil {
		return err
	}

	if taskID == "" {
		if len(taskEntries) == 0 {
			return fmt.Errorf("no tasks found under %s", root.TasksDir)
		}
		if taskID, err = selectOne("Task ID", taskIDOptions(taskEntries)); err != nil {
			return err
		}
	}

	entry, found := task.FindEntry(taskEntries, taskID)
	if !found {
		return fmt.Errorf("task %q not found under %s", taskID, root.TasksDir)
	}
	taskDirName := task.TaskDirName(entry.ID, entry.Name)

	worktrees, err := task.ListWorktrees(root.TasksDir, taskDirName)
	if err != nil {
		return err
	}
	if len(worktrees) == 0 {
		return fmt.Errorf("no worktrees found under task %s", taskID)
	}

	targets, err := selectDeleteTargets(worktrees, repoArg, branchArg, interactive)
	if err != nil {
		return err
	}

	if interactive {
		confirmed, err := confirmDelete(targets)
		if err != nil {
			return err
		}
		if !confirmed {
			return fmt.Errorf("aborted")
		}
	}

	for _, wt := range targets {
		repoRoot := filepath.Join(root.Path, wt.RepoName)
		branch, branchErr := git.CurrentBranch(wt.Path)

		if err := git.WorktreeRemove(repoRoot, wt.Path, deleteForceFlag); err != nil {
			return err
		}
		fmt.Println(ui.Success("Removed worktree %s", wt.Path))

		if branchErr != nil || branch == "" {
			continue
		}

		deleteBranch := deleteDeleteBranchFlag
		if interactive {
			if deleteBranch, err = promptConfirmDefault(fmt.Sprintf("Delete branch %q too?", branch), false); err != nil {
				return err
			}
		}
		if !deleteBranch {
			continue
		}
		if err := git.DeleteBranch(repoRoot, branch, deleteForceFlag); err != nil {
			return err
		}
		fmt.Println(ui.Success("Deleted branch %s", branch))
	}

	taskDir := filepath.Join(root.TasksDir, taskDirName)
	remaining, err := os.ReadDir(taskDir)
	if err == nil && len(remaining) == 0 {
		if err := os.Remove(taskDir); err != nil {
			return fmt.Errorf("remove empty task directory %s: %w", taskDir, err)
		}
	}

	return nil
}

// selectDeleteTargets narrows worktrees down to the ones runDelete should
// remove, based on the optional repo/branch args:
//   - no repo: the whole task (non-interactive), or a scope prompt (interactive)
//   - repo, no branch: that repo's one worktree, or an error/prompt if it has several
//   - repo + branch: that exact worktree
func selectDeleteTargets(worktrees []task.WorktreeEntry, repoArg, branchArg string, interactive bool) ([]task.WorktreeEntry, error) {
	if repoArg == "" {
		if !interactive {
			return worktrees, nil
		}
		return promptSelectWorktreeScope(worktrees)
	}

	var matches []task.WorktreeEntry
	for _, wt := range worktrees {
		if wt.RepoName == repoArg {
			matches = append(matches, wt)
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("repo %q has no worktrees under this task", repoArg)
	}

	if branchArg != "" {
		for _, wt := range matches {
			branch, err := git.CurrentBranch(wt.Path)
			if err == nil && branch == branchArg {
				return []task.WorktreeEntry{wt}, nil
			}
		}
		return nil, fmt.Errorf("repo %q has no worktree on branch %q under this task", repoArg, branchArg)
	}

	if len(matches) == 1 {
		return matches, nil
	}
	if !interactive {
		return nil, fmt.Errorf("repo %q has multiple worktrees under this task; specify a branch", repoArg)
	}
	selected, err := promptSelectWorktree(matches)
	if err != nil {
		return nil, err
	}
	return []task.WorktreeEntry{selected}, nil
}

func promptSelectWorktreeScope(worktrees []task.WorktreeEntry) ([]task.WorktreeEntry, error) {
	options := []huh.Option[string]{huh.NewOption(fmt.Sprintf("Whole task (%d worktree(s))", len(worktrees)), scopeAllOptionValue)}
	for i, wt := range worktrees {
		options = append(options, huh.NewOption(worktreeLabel(wt), strconv.Itoa(i)))
	}
	selected, err := selectOne("Delete", options)
	if err != nil {
		return nil, err
	}
	if selected == scopeAllOptionValue {
		return worktrees, nil
	}
	idx, _ := strconv.Atoi(selected)
	return []task.WorktreeEntry{worktrees[idx]}, nil
}

func promptSelectWorktree(worktrees []task.WorktreeEntry) (task.WorktreeEntry, error) {
	options := make([]huh.Option[string], 0, len(worktrees))
	for i, wt := range worktrees {
		options = append(options, huh.NewOption(worktreeLabel(wt), strconv.Itoa(i)))
	}
	selected, err := selectOne("Which worktree?", options)
	if err != nil {
		return task.WorktreeEntry{}, err
	}
	idx, _ := strconv.Atoi(selected)
	return worktrees[idx], nil
}

func worktreeLabel(wt task.WorktreeEntry) string {
	branch, err := git.CurrentBranch(wt.Path)
	if err != nil || branch == "" {
		return wt.RepoName
	}
	return fmt.Sprintf("%s (%s)", wt.RepoName, branch)
}

func confirmDelete(targets []task.WorktreeEntry) (bool, error) {
	fmt.Println(ui.Label("Deleting:"))
	for _, wt := range targets {
		fmt.Println(" ", wt.Path)
	}
	return promptConfirm("Delete these worktree(s)?")
}
