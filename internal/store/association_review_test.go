package store

import (
	"errors"
	"path/filepath"
	"sync"
	"testing"
)

func TestReviewedAssociationCompetingRequests(t *testing.T) {
	base := t.TempDir()
	a, b := openTestStore(t, filepath.Join(base, "db")), openTestStore(t, filepath.Join(base, "db"))
	root, err := a.CreateRoot("root", base)
	if err != nil {
		t.Fatal(err)
	}
	repo, err := a.CreateRepo(root.ID, "repo", filepath.Join(base, "repo"))
	if err != nil {
		t.Fatal(err)
	}
	source, err := a.Attach(root.ID, "source", "", repo.ID, filepath.Join(base, "checkout"), "", "git-identity")
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i, db := range []*Store{a, b} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := db.AttachReviewed(root.ID, []string{"a", "b"}[i], "", repo.ID, source.Path, source.TaskID, source.Identity, AssociationReview{Source: &source})
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	successes, conflicts := 0, 0
	for err := range results {
		if err == nil {
			successes++
		} else if errors.Is(err, ErrConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("success=%d conflicts=%d", successes, conflicts)
	}
	if err := a.DetachReviewed(root.ID, source); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale detach: %v", err)
	}
	current, _, _, err := a.AssociationAt(source.Path)
	if err != nil || current == nil || current.TaskID == source.TaskID {
		t.Fatalf("winner lost: %v %v", current, err)
	}
}

func TestReviewedAttachRejectsReusedTargetAndSource(t *testing.T) {
	base := t.TempDir()
	db := openTestStore(t, filepath.Join(base, "db"))
	root, _ := db.CreateRoot("root", base)
	repo, _ := db.CreateRepo(root.ID, "repo", filepath.Join(base, "repo"))
	target, _ := db.CreateTask(root.ID, "target", "")
	review := AssociationReview{TargetID: target.ID}
	// Simulate removal and recreation of an empty Task with the same public ID.
	if _, err := db.db.Exec("DELETE FROM tasks WHERE id=?", target.ID); err != nil {
		t.Fatal(err)
	}
	replacement, err := db.CreateTask(root.ID, "target", "")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(base, "checkout")
	if _, err := db.AttachReviewed(root.ID, "target", "", repo.ID, path, "", "identity", review); !errors.Is(err, ErrConflict) {
		t.Fatalf("reused target: %v", err)
	}
	source, err := db.Attach(root.ID, "target", "", repo.ID, path, "", "identity")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.DetachReviewed(root.ID, source); err != nil {
		t.Fatal(err)
	}
	fresh, err := db.Attach(root.ID, "target", "", repo.ID, path, "", "identity")
	if err != nil {
		t.Fatal(err)
	}
	if fresh.ID == source.ID {
		t.Fatal("mapping identity reused")
	}
	_, err = db.AttachReviewed(root.ID, "target", "", repo.ID, path, source.TaskID, "identity", AssociationReview{TargetID: replacement.ID, Source: &source})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("reused source: %v", err)
	}
}
