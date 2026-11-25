package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/tintaungkhant/tasky/internal/config"
	"github.com/tintaungkhant/tasky/internal/git"
	"github.com/tintaungkhant/tasky/internal/task"
	"github.com/tintaungkhant/tasky/internal/ui"
)

var listAllFlag bool

var listCmd = &cobra.Command{
	Use:   "list [task-id]",
	Short: "List tasks and their worktrees",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runList,
}

func init() {
	listCmd.Flags().BoolVar(&listAllFlag, "all", false, "print every task as flat text instead of the interactive picker (no task-id arg)")
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	if c, err := registeredContext(cmd); err != nil {
		return err
	} else if c != nil {
		defer c.Store.Close()
		return runStoredList(cmd, args, c)
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

	out := cmd.OutOrStdout()

	switch {
	case len(args) == 1:
		entry, found := task.FindEntry(taskEntries, args[0])
		if !found {
			return fmt.Errorf("task %q not found under %s", args[0], root.TasksDir)
		}
		taskEntries = []task.Entry{entry}
	case !listAllFlag:
		if len(taskEntries) == 0 {
			fmt.Fprintf(out, "no tasks found under %s\n", root.TasksDir)
			return nil
		}
		taskID, err := selectOneFiltering("Task ID", taskIDOptions(taskEntries))
		if err != nil {
			return err
		}
		entry, _ := task.FindEntry(taskEntries, taskID)
		taskEntries = []task.Entry{entry}
	}

	if len(taskEntries) == 0 {
		fmt.Fprintf(out, "no tasks found under %s\n", root.TasksDir)
		return nil
	}

	for i, entry := range taskEntries {
		if i > 0 {
			fmt.Fprintln(out)
		}
		fmt.Fprintln(out, ui.Label(entry.Label()))

		worktrees, err := task.ListWorktrees(root.TasksDir, task.TaskDirName(entry.ID, entry.Name))
		if err != nil {
			return err
		}
		if len(worktrees) == 0 {
			fmt.Fprintln(out, "  (no worktrees)")
			continue
		}
		for _, wt := range worktrees {
			branch, err := git.CurrentBranch(wt.Path)
			if err != nil {
				fmt.Fprintf(out, "  %s  %s\n", wt.RepoName, wt.Path)
				continue
			}
			fmt.Fprintf(out, "  %s  %s  %s\n", wt.RepoName, branch, wt.Path)
		}
	}
	return nil
}
