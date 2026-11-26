package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tintaungkhant/tasky/internal/config"
	"github.com/tintaungkhant/tasky/internal/store"
)

// Callers serialize config edits and import. The expected list prevents a
// stale dashboard form from overwriting preferences saved since it was loaded.
func UpdatePreferences(path string, expected, next config.Config) error {
	for i, root := range next.Roots {
		if strings.TrimSpace(root.Name) == "" || strings.TrimSpace(root.BranchTemplate) == "" {
			return fmt.Errorf("YAML Root %d: name and Branch Template are required", i+1)
		}
		if !filepath.IsAbs(root.Path) || !filepath.IsAbs(root.TasksDir) {
			return fmt.Errorf("YAML Root %d: Root path and Tasks Directory must be absolute", i+1)
		}
		info, err := os.Stat(root.Path)
		if err != nil {
			return fmt.Errorf("YAML Root %d: %w", i+1, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("YAML Root %d: Root path must be a directory", i+1)
		}
		info, err = os.Stat(root.TasksDir)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("Tasks Directory: %w", err)
		}
		if err == nil && !info.IsDir() {
			return fmt.Errorf("Tasks Directory must be a directory")
		}
	}
	err := next.SaveFileIfUnchanged(path, expected)
	if errors.Is(err, config.ErrChanged) {
		return fmt.Errorf("%s: %w", err, store.ErrConflict)
	}
	return err
}
