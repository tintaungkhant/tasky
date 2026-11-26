package store

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestConfirmedRootRemovalRejectsChangedMappings(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	root, err := s.CreateRoot("same", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	preview, err := s.RootMappings(root.ID)
	if err != nil {
		t.Fatal(err)
	}
	task, err := s.CreateTask(root.ID, "new", "name")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RemoveRootConfirmed(preview); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale preview: %v", err)
	}
	current, err := s.RootMappings(root.ID)
	if err != nil || len(current.Tasks) != 1 || current.Tasks[0].ID != task.ID {
		t.Fatalf("mappings lost: %v", err)
	}
	if err := s.RemoveRootConfirmed(current); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RootMappings(root.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Root still exists: %v", err)
	}
}

func TestRootRemovalPreviewBlocksPendingCreation(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	root, err := s.CreateRoot("root", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	task, err := s.CreateTask(root.ID, "T-1", "")
	if err != nil {
		t.Fatal(err)
	}
	repo, err := s.CreateRepo(root.ID, "repo", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	preview, err := s.RootMappings(root.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReserveCreation(task.ID, repo.ID, "branch", filepath.Join(t.TempDir(), "wt")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RootMappings(root.ID); !errors.Is(err, ErrConflict) {
		t.Fatalf("preview lost pending intent: %v", err)
	}
	if err := s.RemoveRootConfirmed(preview); !errors.Is(err, ErrConflict) {
		t.Fatalf("removal lost pending intent: %v", err)
	}
	if pending, err := s.ListCreations(root.ID); err != nil || len(pending) != 1 {
		t.Fatalf("pending intent lost: %v", err)
	}
}
