package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/tintaungkhant/tasky/internal/work"
)

func init() {
	command := &cobra.Command{Use: "reconnect <task-id> <old-path> <new-path>", Short: "Reconnect the original worktree after an external move; never move files", Args: cobra.ExactArgs(3), RunE: func(cmd *cobra.Command, args []string) error {
		c, err := requiredStored(cmd)
		if err != nil {
			return err
		}
		defer c.Store.Close()
		_, mappings, repos, err := c.worktrees(args[0])
		if err != nil {
			return err
		}
		old, err := filepath.Abs(args[1])
		if err != nil {
			return err
		}
		for _, wt := range mappings {
			if filepath.Clean(wt.Path) != old {
				continue
			}
			accept, _ := cmd.Flags().GetBool("accept-identity")
			if err := (work.Manager{Store: c.Store}).Reconnect(repos[wt.RepoID], wt, args[2], accept); err != nil {
				return err
			}
			cmd.Printf("Reconnected worktree %s. Task ownership, files, and branches are unchanged.\n", wt.ID)
			return nil
		}
		return fmt.Errorf("no mapping at %s for Task %s; inspect tasky list before retrying", old, args[0])
	}}
	command.Flags().String("root", "", "registered Root name or ID")
	command.Flags().Bool("accept-identity", false, "enroll a reviewed legacy mapping that has no recorded Git identity")
	rootCmd.AddCommand(command)
}
