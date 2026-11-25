package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/tintaungkhant/tasky/internal/git"
	"github.com/tintaungkhant/tasky/internal/work"
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
	path, _, err := git.VerifyWorktree(args[1])
	if err != nil {
		return fmt.Errorf("attach made no mapping changes: %w", err)
	}
	repoSelector, _ := cmd.Flags().GetString("repo")
	ctx, err := c.Registry.Resolve(path, c.Root.ID, repoSelector)
	if err != nil {
		return err
	}
	if err := (work.Manager{Store: c.Store}).Verify(ctx.Repo, path); err != nil {
		return fmt.Errorf("attach made no mapping changes: %w", err)
	}
	name, _ := cmd.Flags().GetString("name")
	from, _ := cmd.Flags().GetString("reassign-from")
	wt, err := c.Store.Attach(c.Root.ID, args[0], name, ctx.Repo.ID, path, from)
	if err != nil {
		return err
	}
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
	path, err := filepath.Abs(args[1])
	if err != nil {
		return err
	}
	// Detaching missing locations must still work. Canonicalize aliases only
	// when the location is accessible; otherwise use the stored absolute path.
	if canonical, err := filepath.EvalSymlinks(path); err == nil {
		path = canonical
	}
	if err := c.Store.Detach(c.Root.ID, args[0], path); err != nil {
		return err
	}
	cmd.Printf("Detached mapping for %s from Task %s. Files, worktrees, and branches are preserved.\n", path, args[0])
	return nil
}
