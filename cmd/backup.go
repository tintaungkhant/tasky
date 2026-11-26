package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tintaungkhant/tasky/internal/app"
	"github.com/tintaungkhant/tasky/internal/config"
	"github.com/tintaungkhant/tasky/internal/store"
)

func init() {
	rootCmd.AddCommand(
		&cobra.Command{Use: "backup <file>", Short: "Back up all registered mapping identities to a new JSON file", Args: cobra.ExactArgs(1), RunE: runBackup},
		&cobra.Command{Use: "restore <file>", Short: "Atomically restore mappings without replacing existing records or changing Git", Args: cobra.ExactArgs(1), RunE: runRestore},
	)
}

func mappingDatabase() (string, error) {
	path, err := config.Path()
	if err != nil {
		return "", err
	}
	return app.DatabasePath(path), nil
}

func runBackup(cmd *cobra.Command, args []string) error {
	path, err := mappingDatabase()
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("cannot back up Tasky database %s: %w; check its location and permissions or register/import mappings first; ordinary Git remains usable", path, err)
	}
	s, err := store.Open(path)
	if err != nil {
		return err
	}
	defer s.Close()
	b, err := s.Backup()
	if err != nil {
		return err
	}
	if err := app.WriteMappingBackup(args[0], b); err != nil {
		return err
	}
	cmd.Printf("Backed up %d Roots, %d Repos, %d Tasks, and %d worktree mappings to %s. Keep the YAML configuration separately.\n", len(b.Roots), len(b.Repos), len(b.Tasks), len(b.Worktrees), args[0])
	return nil
}

func runRestore(cmd *cobra.Command, args []string) error {
	b, err := app.ReadMappingBackup(args[0])
	if err != nil {
		return err
	}
	path, err := mappingDatabase()
	if err != nil {
		return err
	}
	s, err := store.Open(path)
	if err != nil {
		return err
	}
	defer s.Close()
	report, err := app.Restore(s, b)
	if err != nil {
		return err
	}
	cmd.Printf("Restored mappings from %s. Existing mappings were preserved; code, branches, and worktrees are unchanged.\n", args[0])
	for _, warning := range report.Warnings {
		switch warning.Kind {
		case "Root":
			cmd.Printf("Review Root %s at %s: location missing, inaccessible, or not a directory; check its location before use.\n", warning.ID, warning.Path)
		case "Repo":
			cmd.Printf("Review Repo %s at %s: %s; inspect Git registration before use.\n", warning.ID, warning.Path, warning.Detail)
		case "worktree":
			cmd.Printf("Review worktree %s at %s [%s]: %s\n", warning.ID, warning.Path, warning.Status, warning.Detail)
		}
	}
	cmd.Printf("Checked %d restored worktree locations against Git; %d require review. Use tasky list, reconnect, or detach after reviewing them.\n", len(b.Worktrees), report.WorktreeWarnings)
	return nil
}
