package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/tintaungkhant/tasky/internal/config"
	"github.com/tintaungkhant/tasky/internal/git"
	"github.com/tintaungkhant/tasky/internal/task"
)

var addCmd = &cobra.Command{
	Use:   "add <task-id> <branch>",
	Short: "Create a task-scoped Git worktree",
	Args:  cobra.ExactArgs(2),
	RunE:  runAdd,
}

func init() {
	rootCmd.AddCommand(addCmd)
}

func runAdd(cmd *cobra.Command, args []string) error {
	taskID, branch := args[0], args[1]

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	repoRoot, err := git.Toplevel(cwd)
	if err != nil {
		return fmt.Errorf("not inside a git repo: %w", err)
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	root, err := cfg.MatchRoot(repoRoot)
	if err != nil {
		return err
	}

	base, err := git.CurrentBranch(repoRoot)
	if err != nil {
		return err
	}

	repoName := filepath.Base(repoRoot)
	renderedBranch := task.RenderBranch(root.BranchTemplate, taskID, branch)
	worktreePath := task.WorktreePath(root.TasksDir, taskID, repoName)

	if _, err := os.Stat(worktreePath); err == nil {
		return fmt.Errorf("worktree path already exists: %s", worktreePath)
	}

	if err := git.WorktreeAddNewBranch(repoRoot, worktreePath, renderedBranch, base); err != nil {
		return err
	}

	fmt.Printf("Created worktree at %s (branch %s, from %s)\n", worktreePath, renderedBranch, base)
	return nil
}
