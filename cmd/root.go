// Package cmd wires tasky's CLI commands.
package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "tasky",
	Short: "tasky manages task-scoped Git worktrees across repos",
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
