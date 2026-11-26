package cmd

import (
	"fmt"
	"os"
	"path/filepath"
)

// A synced temporary file and exclusive hard link publish complete contents
// without replacing another command's output or a user's existing file.
func publishNewFile(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".tasky-output-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Link(f.Name(), path); err != nil {
		return fmt.Errorf("publish %s without overwriting: %w; choose a new filename on a filesystem supporting hard links", path, err)
	}
	return nil
}
