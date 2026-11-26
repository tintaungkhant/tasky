package app

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/tintaungkhant/tasky/internal/config"
	"github.com/tintaungkhant/tasky/internal/store"
)

func TestPreferencesValidateAndRejectStaleEdits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	cfg := config.Config{Roots: []config.Root{{Name: "same", Path: t.TempDir(), TasksDir: filepath.Join(t.TempDir(), "missing"), BranchTemplate: "{branch}"}}}
	if err := UpdatePreferences(path, config.Config{}, cfg); err != nil {
		t.Fatal(err)
	}
	if err := UpdatePreferences(path, config.Config{}, config.Config{}); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("stale edit: %v", err)
	}
	before, _ := os.ReadFile(path)
	invalid := config.Config{Roots: []config.Root{{Name: "invalid", Path: "relative"}}}
	if err := UpdatePreferences(path, cfg, invalid); err == nil {
		t.Fatal("invalid paths accepted")
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("invalid preferences changed YAML")
	}
	if err := UpdatePreferences(path, cfg, config.Config{}); err != nil {
		t.Fatal(err)
	}
}
