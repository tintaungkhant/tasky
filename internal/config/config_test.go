package config

import (
	"path/filepath"
	"testing"
)

func TestMatchRoot(t *testing.T) {
	cfg := &Config{
		Roots: []Root{
			{Name: "company", Path: "/Users/dev/Work/company"},
			{Name: "company-nested", Path: "/Users/dev/Work/company/nested"},
			{Name: "personal", Path: "/Users/dev/Work/personal"},
		},
	}

	tests := []struct {
		name    string
		dir     string
		want    string
		wantErr bool
	}{
		{name: "exact match", dir: "/Users/dev/Work/company", want: "company"},
		{name: "child of root", dir: "/Users/dev/Work/company/project-a", want: "company"},
		{name: "longest prefix wins", dir: "/Users/dev/Work/company/nested/project-b", want: "company-nested"},
		{name: "unrelated dir", dir: "/Users/dev/Other", wantErr: true},
		{name: "sibling prefix collision", dir: "/Users/dev/Work/company-extra", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := cfg.MatchRoot(tt.dir)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("MatchRoot(%q) = %v, want error", tt.dir, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("MatchRoot(%q) unexpected error: %v", tt.dir, err)
			}
			if got.Name != tt.want {
				t.Fatalf("MatchRoot(%q) = %q, want %q", tt.dir, got.Name, tt.want)
			}
		})
	}
}

func TestAddFindRemoveRoot(t *testing.T) {
	cfg := &Config{}
	cfg.AddRoot(Root{Name: "company", Path: "/Users/dev/Work/company"})

	got, ok := cfg.FindRoot("company")
	if !ok {
		t.Fatal("FindRoot(company) not found")
	}
	if got.Path != "/Users/dev/Work/company" {
		t.Fatalf("FindRoot(company).Path = %q", got.Path)
	}

	if _, ok := cfg.FindRoot("missing"); ok {
		t.Fatal("FindRoot(missing) unexpectedly found")
	}

	if !cfg.RemoveRoot("company") {
		t.Fatal("RemoveRoot(company) = false, want true")
	}
	if len(cfg.Roots) != 0 {
		t.Fatalf("Roots after remove = %v, want empty", cfg.Roots)
	}
	if cfg.RemoveRoot("company") {
		t.Fatal("RemoveRoot(company) second call = true, want false")
	}
}

func TestLoadSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TASKY_CONFIG", filepath.Join(dir, "config.yaml"))

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() on missing file: %v", err)
	}
	if len(cfg.Roots) != 0 {
		t.Fatalf("Load() on missing file = %v, want empty", cfg.Roots)
	}

	cfg.AddRoot(Root{
		Name:           "company",
		Path:           "/Users/dev/Work/company",
		TasksDir:       "/Users/dev/Work/company/tasks",
		BranchTemplate: "task/{task-id}/{branch}",
	})
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	reloaded, err := Load()
	if err != nil {
		t.Fatalf("Load() after save: %v", err)
	}
	if len(reloaded.Roots) != 1 || reloaded.Roots[0] != cfg.Roots[0] {
		t.Fatalf("reloaded.Roots = %v, want %v", reloaded.Roots, cfg.Roots)
	}
}
