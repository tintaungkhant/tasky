package app

import (
	"fmt"
	"path/filepath"

	"github.com/tintaungkhant/tasky/internal/adopt"
	"github.com/tintaungkhant/tasky/internal/config"
	"github.com/tintaungkhant/tasky/internal/store"
)

func DatabasePath(configPath string) string {
	return filepath.Join(filepath.Dir(configPath), "tasky.sqlite3")
}

func Import(configPath string) (int, error) {
	cfg, err := config.LoadFile(configPath)
	if err != nil {
		return 0, err
	}
	plan, err := adopt.Plan(cfg)
	if err != nil {
		return 0, fmt.Errorf("import made no mapping changes: %w", err)
	}
	s, err := store.Open(DatabasePath(configPath))
	if err != nil {
		return 0, err
	}
	defer s.Close()
	if err := s.Import(plan); err != nil {
		return 0, fmt.Errorf("import rolled back; resolve the conflict and retry: %w", err)
	}
	return len(plan), nil
}
