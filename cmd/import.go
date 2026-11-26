package cmd

import (
	"github.com/spf13/cobra"
	"github.com/tintaungkhant/tasky/internal/app"
	"github.com/tintaungkhant/tasky/internal/config"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{Use: "import", Short: "Verify and adopt YAML Roots and legacy Tasks without changing Git", Args: cobra.NoArgs, RunE: runImport})
}

func runImport(cmd *cobra.Command, args []string) error {
	path, err := config.Path()
	if err != nil {
		return err
	}
	count, err := app.Import(path)
	if err != nil {
		return err
	}
	cmd.Printf("Adopted %d Root(s). Existing files, worktrees, branches, and YAML are unchanged.\n", count)
	return nil
}
