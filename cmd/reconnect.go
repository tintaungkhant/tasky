package cmd

import (
	"github.com/spf13/cobra"
)

func init() {
	command := &cobra.Command{Use: "reconnect <task-id> <old-path> <new-path>", Short: "Reconnect the original worktree after an external move; never move files", Args: cobra.ExactArgs(3), RunE: func(cmd *cobra.Command, args []string) error {
		c, err := requiredStored(cmd)
		if err != nil {
			return err
		}
		defer c.Store.Close()
		accept, _ := cmd.Flags().GetBool("accept-identity")
		wt, err := c.Reconnect(args[0], args[1], args[2], accept)
		if err != nil {
			return err
		}
		cmd.Printf("Reconnected worktree %s. Task ownership, files, and branches are unchanged.\n", wt.ID)
		return nil
	}}
	command.Flags().String("root", "", "registered Root name or ID")
	command.Flags().Bool("accept-identity", false, "enroll a reviewed legacy mapping that has no recorded Git identity")
	rootCmd.AddCommand(command)
}
