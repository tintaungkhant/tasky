package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/tintaungkhant/tasky/internal/adopt"
	"github.com/tintaungkhant/tasky/internal/config"
	"github.com/tintaungkhant/tasky/internal/store"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{Use: "import", Short: "Verify and adopt YAML Roots and legacy Tasks without changing Git", Args: cobra.NoArgs, RunE: runImport})
}

func runImport(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	plan, err := adopt.Plan(cfg)
	if err != nil {
		return fmt.Errorf("import made no mapping changes: %w", err)
	}
	path, err := config.Path()
	if err != nil {
		return err
	}
	s, err := store.Open(filepath.Join(filepath.Dir(path), "tasky.sqlite3"))
	if err != nil {
		return err
	}
	defer s.Close()
	if err := s.Import(plan); err != nil {
		return fmt.Errorf("import rolled back; resolve the conflict and retry: %w", err)
	}
	cmd.Printf("Adopted %d Root(s). Existing files, worktrees, branches, and YAML are unchanged.\n", len(plan))
	return nil
}
