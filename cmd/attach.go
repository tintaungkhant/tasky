package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	attach := &cobra.Command{Use: "attach <task-id> <worktree-path>", Short: "Associate an existing Git checkout without moving it or changing branches", Args: cobra.ExactArgs(2), RunE: runAttach}
	attach.Flags().String("repo", "", "registered Repo name or ID (default: the supplied checkout's Git identity)")
	attach.Flags().String("name", "", "name for a newly created Task")
	attach.Flags().String("reassign-from", "", "authorize reassignment from this exact internal Task ID (shown by a conflicting attach)")
	detach := &cobra.Command{Use: "detach <task-id> <worktree-path>", Short: "Remove only a Task association; preserve the checkout and branch", Args: cobra.ExactArgs(2), RunE: runDetach}
	for _, cmd := range []*cobra.Command{attach, detach} {
		cmd.Flags().String("root", "", "registered Root name or ID (default: current-directory context)")
	}
	rootCmd.AddCommand(attach, detach)
}

func runAttach(cmd *cobra.Command, args []string) error {
	c, err := requiredStored(cmd)
	if err != nil {
		return err
	}
	defer c.Store.Close()
	repoSelector, _ := cmd.Flags().GetString("repo")
	name, _ := cmd.Flags().GetString("name")
	from, _ := cmd.Flags().GetString("reassign-from")
	wt, err := c.Attach(args[0], args[1], repoSelector, name, from)
	if err != nil {
		return err
	}
	path := wt.Path
	cmd.Printf("Associated %s with Task %s (internal Task %s, worktree %s). Git state is unchanged.\n", path, args[0], wt.TaskID, wt.ID)
	return nil
}

func runDetach(cmd *cobra.Command, args []string) error {
	if args[1] == "" {
		return fmt.Errorf("worktree path must not be empty")
	}
	c, err := requiredStored(cmd)
	if err != nil {
		return err
	}
	defer c.Store.Close()
	path, err := c.Detach(args[0], args[1])
	if err != nil {
		return err
	}
	cmd.Printf("Detached mapping for %s from Task %s. Files, worktrees, and branches are preserved.\n", path, args[0])
	return nil
}
