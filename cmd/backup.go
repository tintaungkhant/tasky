package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/tintaungkhant/tasky/internal/config"
	"github.com/tintaungkhant/tasky/internal/git"
	"github.com/tintaungkhant/tasky/internal/store"
	"github.com/tintaungkhant/tasky/internal/work"
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
	return filepath.Join(filepath.Dir(path), "tasky.sqlite3"), nil
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
	if err := writeMappingBackup(args[0], b); err != nil {
		return err
	}
	cmd.Printf("Backed up %d Roots, %d Repos, %d Tasks, and %d worktree mappings to %s. Keep the YAML configuration separately.\n", len(b.Roots), len(b.Repos), len(b.Tasks), len(b.Worktrees), args[0])
	return nil
}

func writeMappingBackup(path string, b store.MappingBackup) error {
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return fmt.Errorf("write backup: %w", err)
	}
	return publishNewFile(path, append(data, '\n'))
}

func readMappingBackup(path string) (store.MappingBackup, error) {
	f, err := os.Open(path)
	if err != nil {
		return store.MappingBackup{}, fmt.Errorf("open mapping backup: %w", err)
	}
	defer f.Close()
	// Bound untrusted input before decoding; mapping backups contain metadata only.
	const maxBackupSize = 64 << 20
	info, err := f.Stat()
	if err != nil {
		return store.MappingBackup{}, err
	}
	if !info.Mode().IsRegular() || info.Size() > maxBackupSize {
		return store.MappingBackup{}, fmt.Errorf("mapping backup must be a regular JSON file of at most 64 MiB")
	}
	decoder := json.NewDecoder(io.LimitReader(f, maxBackupSize+1))
	decoder.DisallowUnknownFields()
	var b store.MappingBackup
	if err := decoder.Decode(&b); err != nil {
		return b, fmt.Errorf("invalid mapping backup: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return b, fmt.Errorf("invalid mapping backup: trailing data")
	}
	if err := b.Validate(); err != nil {
		return b, err
	}
	return b, nil
}

func runRestore(cmd *cobra.Command, args []string) error {
	b, err := readMappingBackup(args[0])
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
	if err := s.Restore(b); err != nil {
		return err
	}
	cmd.Printf("Restored mappings from %s. Existing mappings were preserved; code, branches, and worktrees are unchanged.\n", args[0])
	for _, root := range b.Roots {
		if info, err := os.Stat(root.Path); err != nil || !info.IsDir() {
			cmd.Printf("Review Root %s at %s: location missing, inaccessible, or not a directory; check its location before use.\n", root.ID, root.Path)
		}
	}
	repos := map[string]store.Repo{}
	for _, repo := range b.Repos {
		repos[repo.ID] = repo
		if _, _, err := git.VerifyWorktree(repo.Path); err != nil {
			cmd.Printf("Review Repo %s at %s: %v; inspect Git registration before use.\n", repo.ID, repo.Path, err)
		}
	}
	warnings := 0
	for _, wt := range b.Worktrees {
		state := (work.Manager{Store: s}).Inspect(repos[wt.RepoID], wt)
		if state.Status != "ok" {
			warnings++
			cmd.Printf("Review worktree %s at %s [%s]: %s\n", wt.ID, wt.Path, state.Status, state.Detail)
		}
	}
	cmd.Printf("Checked %d restored worktree locations against Git; %d require review. Use tasky list, reconnect, or detach after reviewing them.\n", len(b.Worktrees), warnings)
	return nil
}
