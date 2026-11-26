package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tintaungkhant/tasky/internal/app"
	"github.com/tintaungkhant/tasky/internal/git"
	"github.com/tintaungkhant/tasky/internal/store"
)

func associationDB(t *testing.T, f taskMutationFixture) *store.Store {
	t.Helper()
	db, err := store.Open(app.DatabasePath(f.s.configPath))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func attachmentReview(t *testing.T, f taskMutationFixture, path, taskID string) app.AttachmentReview {
	t.Helper()
	w := f.call(t, http.MethodPost, "/api/tasks/attach/preview", map[string]string{
		"RootID": f.root.ID, "RepoID": f.repos[0].ID, "TaskID": taskID, "Name": "Imported", "Path": path,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("attachment preview: %d %s", w.Code, w.Body.String())
	}
	var review app.AttachmentReview
	if err := json.Unmarshal(w.Body.Bytes(), &review); err != nil {
		t.Fatal(err)
	}
	return review
}

func attachReviewed(t *testing.T, f taskMutationFixture, review app.AttachmentReview, from string) *httptest.ResponseRecorder {
	t.Helper()
	return f.call(t, http.MethodPost, "/api/tasks/attach", map[string]any{"Review": review, "ReassignFrom": from})
}

func mappingAt(t *testing.T, db *store.Store, rootID, path string) store.Worktree {
	t.Helper()
	mappings, err := db.RootMappings(rootID)
	if err != nil {
		t.Fatal(err)
	}
	for _, mapping := range mappings.Worktrees {
		if mapping.Path == path {
			return mapping
		}
	}
	t.Fatalf("mapping at %s missing", path)
	return store.Worktree{}
}

func TestAttachmentPreviewAndAttachPreserveExternalCheckout(t *testing.T) {
	f := newTaskMutationFixture(t, 1)
	repo := f.repos[0]
	path := filepath.Join(t.TempDir(), "external")
	gitTest(t, repo.Path, "worktree", "add", "-b", "external/feature", path)
	if err := os.WriteFile(filepath.Join(path, "dirty.txt"), []byte("keep me"), 0600); err != nil {
		t.Fatal(err)
	}
	refsBefore := gitTest(t, repo.Path, "show-ref")
	contentsBefore, err := os.ReadFile(filepath.Join(path, "dirty.txt"))
	if err != nil {
		t.Fatal(err)
	}
	review := attachmentReview(t, f, path, "T-1")
	if review.RootID != f.root.ID || review.RepoID != repo.ID || review.TaskID != "T-1" || review.TargetID != "" || review.Identity == "" || review.Path != path || review.Source != nil {
		t.Fatalf("unexpected preview: %#v", review)
	}
	if got := gitTest(t, repo.Path, "show-ref"); got != refsBefore {
		t.Fatal("preview changed refs")
	}
	attached := attachReviewed(t, f, review, "")
	if attached.Code != http.StatusOK {
		t.Fatalf("attach: %d %s", attached.Code, attached.Body.String())
	}
	if got := gitTest(t, repo.Path, "show-ref"); got != refsBefore {
		t.Fatal("attach changed refs")
	}
	contentsAfter, err := os.ReadFile(filepath.Join(path, "dirty.txt"))
	if err != nil || string(contentsAfter) != string(contentsBefore) {
		t.Fatalf("attach changed external files: %q, %v", contentsAfter, err)
	}
}

func TestAttachmentReviewCASRejectsStaleTargetAndSource(t *testing.T) {
	t.Run("target created after preview", func(t *testing.T) {
		f := newTaskMutationFixture(t, 1)
		path := filepath.Join(t.TempDir(), "external")
		gitTest(t, f.repos[0].Path, "worktree", "add", "-b", "external", path)
		review := attachmentReview(t, f, path, "T-new")
		db := associationDB(t, f)
		if _, err := db.CreateTask(f.root.ID, "T-new", "created concurrently"); err != nil {
			t.Fatal(err)
		}
		w := attachReviewed(t, f, review, "")
		if w.Code != http.StatusConflict {
			t.Fatalf("stale target status %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("source reassigned after preview", func(t *testing.T) {
		f := newTaskMutationFixture(t, 1)
		db := associationDB(t, f)
		path := filepath.Join(t.TempDir(), "external")
		gitTest(t, f.repos[0].Path, "worktree", "add", "-b", "external", path)
		identity, err := git.WorktreeIdentity(path)
		if err != nil {
			t.Fatal(err)
		}
		original, err := db.Attach(f.root.ID, "T-old", "Old", f.repos[0].ID, path, "", identity)
		if err != nil {
			t.Fatal(err)
		}
		review := attachmentReview(t, f, path, "T-new")
		if review.Source == nil || review.SourceTask == nil || review.SourceTask.ID != original.TaskID || review.SourceRoot == nil || review.SourceRoot.ID != f.root.ID {
			t.Fatalf("preview omitted source owner: %#v", review)
		}
		current, err := db.Attach(f.root.ID, "T-other", "Other", f.repos[0].ID, path, original.TaskID, identity)
		if err != nil {
			t.Fatal(err)
		}
		w := attachReviewed(t, f, review, original.TaskID)
		if w.Code != http.StatusConflict {
			t.Fatalf("stale source status %d: %s", w.Code, w.Body.String())
		}
		if got := mappingAt(t, db, f.root.ID, path).TaskID; got != current.TaskID {
			t.Fatalf("stale attach replaced current owner %s", got)
		}
	})
}

func TestAttachRequiresExplicitSourceOwner(t *testing.T) {
	f := newTaskMutationFixture(t, 1)
	db := associationDB(t, f)
	path := filepath.Join(t.TempDir(), "external")
	gitTest(t, f.repos[0].Path, "worktree", "add", "-b", "external", path)
	identity, err := git.WorktreeIdentity(path)
	if err != nil {
		t.Fatal(err)
	}
	old, err := db.Attach(f.root.ID, "T-old", "Old", f.repos[0].ID, path, "", identity)
	if err != nil {
		t.Fatal(err)
	}
	review := attachmentReview(t, f, path, "T-new")
	if w := attachReviewed(t, f, review, ""); w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), old.TaskID) {
		t.Fatalf("implicit reassignment response: %d %s", w.Code, w.Body.String())
	}
	w := attachReviewed(t, f, review, old.TaskID)
	if w.Code != http.StatusOK {
		t.Fatalf("explicit reassignment: %d %s", w.Code, w.Body.String())
	}
	if got := mappingAt(t, db, f.root.ID, path).TaskID; got == old.TaskID {
		t.Fatal("explicit reassignment kept old owner")
	}
}

func TestDetachReviewedRejectsStaleSnapshot(t *testing.T) {
	f := newTaskMutationFixture(t, 1)
	db := associationDB(t, f)
	path := filepath.Join(t.TempDir(), "external")
	gitTest(t, f.repos[0].Path, "worktree", "add", "-b", "external", path)
	identity, err := git.WorktreeIdentity(path)
	if err != nil {
		t.Fatal(err)
	}
	old, err := db.Attach(f.root.ID, "T-old", "Old", f.repos[0].ID, path, "", identity)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Attach(f.root.ID, "T-new", "New", f.repos[0].ID, path, old.TaskID, identity); err != nil {
		t.Fatal(err)
	}
	w := f.call(t, http.MethodPost, "/api/tasks/detach", map[string]any{"RootID": f.root.ID, "Mapping": old, "Confirm": true})
	if w.Code != http.StatusConflict {
		t.Fatalf("stale detach status %d: %s", w.Code, w.Body.String())
	}
	if got := mappingAt(t, db, f.root.ID, path).TaskID; got == old.TaskID {
		t.Fatal("stale detach removed replacement mapping")
	}
}

func TestAssociationMutationReportsPendingCleanupRecovery(t *testing.T) {
	f := newTaskMutationFixture(t, 1)
	db := associationDB(t, f)
	path := filepath.Join(t.TempDir(), "external")
	gitTest(t, f.repos[0].Path, "worktree", "add", "-b", "external", path)
	identity, err := git.WorktreeIdentity(path)
	if err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(path, "keep.txt")
	if err := os.WriteFile(marker, []byte("preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	mapping, err := db.Attach(f.root.ID, "T-1", "Task", f.repos[0].ID, path, "", identity)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ReserveCleanup(mapping, "external", "revision", true, false, false); err != nil {
		t.Fatal(err)
	}
	w := f.call(t, http.MethodPost, "/api/tasks/detach", map[string]any{"RootID": f.root.ID, "Mapping": mapping, "Confirm": true})
	if w.Code != http.StatusConflict || !strings.Contains(strings.ToLower(w.Body.String()), "recover") {
		t.Fatalf("pending cleanup response: %d %s", w.Code, w.Body.String())
	}
	worktrees, err := db.ListWorktrees(mapping.TaskID)
	if err != nil || len(worktrees) != 1 || worktrees[0] != mapping {
		t.Fatalf("blocked detach changed mapping: %#v, %v", worktrees, err)
	}
	if contents, err := os.ReadFile(marker); err != nil || string(contents) != "preserve" {
		t.Fatalf("blocked detach changed checkout: %q, %v", contents, err)
	}
}

func TestReconnectMovedWorktreeAndRejectsReplacementIdentity(t *testing.T) {
	f := newTaskMutationFixture(t, 1)
	db := associationDB(t, f)
	oldPath := filepath.Join(t.TempDir(), "old")
	gitTest(t, f.repos[0].Path, "worktree", "add", "-b", "external", oldPath)
	identity, err := git.WorktreeIdentity(oldPath)
	if err != nil {
		t.Fatal(err)
	}
	mapping, err := db.Attach(f.root.ID, "T-1", "Task", f.repos[0].ID, oldPath, "", identity)
	if err != nil {
		t.Fatal(err)
	}
	moved := filepath.Join(filepath.Dir(oldPath), "moved")
	gitTest(t, f.repos[0].Path, "worktree", "move", oldPath, moved)
	w := f.call(t, http.MethodPost, "/api/tasks/reconnect", map[string]any{"RootID": f.root.ID, "Mapping": mapping, "Path": moved})
	if w.Code != http.StatusOK {
		t.Fatalf("reconnect moved worktree: %d %s", w.Code, w.Body.String())
	}
	updated := mappingAt(t, db, f.root.ID, moved)
	if updated.Identity != identity {
		t.Fatalf("reconnect changed identity: %#v", updated)
	}
	secondMove := filepath.Join(filepath.Dir(oldPath), "moved-again")
	gitTest(t, f.repos[0].Path, "worktree", "move", moved, secondMove)
	w = f.call(t, http.MethodPost, "/api/tasks/reconnect", map[string]any{"RootID": f.root.ID, "Mapping": mapping, "Path": secondMove})
	if w.Code != http.StatusConflict {
		t.Fatalf("stale reconnect status %d: %s", w.Code, w.Body.String())
	}
	if got := mappingAt(t, db, f.root.ID, moved).Path; got != moved {
		t.Fatalf("stale reconnect overwrote saved path with %s", got)
	}
	// Replace a mapping path with another linked checkout. AcceptIdentity is an
	// explicit acknowledgement, but cannot authorize attaching a different Git
	// worktree identity to an existing mapping.
	replacement := filepath.Join(filepath.Dir(oldPath), "replacement")
	gitTest(t, f.repos[0].Path, "worktree", "add", "-b", "replacement", replacement)
	w = f.call(t, http.MethodPost, "/api/tasks/reconnect", map[string]any{"RootID": f.root.ID, "Mapping": updated, "Path": replacement, "AcceptIdentity": true})
	if w.Code != http.StatusConflict {
		t.Fatalf("replacement identity status %d: %s", w.Code, w.Body.String())
	}
	if mappingAt(t, db, f.root.ID, moved).Path != moved {
		t.Fatal("identity mismatch altered saved mapping")
	}
}

func TestReconnectRequiresExplicitEnrollmentForUnverifiedMapping(t *testing.T) {
	f := newTaskMutationFixture(t, 1)
	db := associationDB(t, f)
	path := filepath.Join(t.TempDir(), "external")
	gitTest(t, f.repos[0].Path, "worktree", "add", "-b", "external", path)
	// Empty identity models a legacy mapping imported before identity checks.
	mapping, err := db.Attach(f.root.ID, "T-legacy", "Legacy", f.repos[0].ID, path, "")
	if err != nil {
		t.Fatal(err)
	}
	request := func(accept bool) *httptest.ResponseRecorder {
		t.Helper()
		return f.call(t, http.MethodPost, "/api/tasks/reconnect", map[string]any{"RootID": f.root.ID, "Mapping": mapping, "Path": path, "AcceptIdentity": accept})
	}
	if w := request(false); w.Code != http.StatusInternalServerError || !strings.Contains(strings.ToLower(w.Body.String()), "accept-identity") {
		t.Fatalf("unverified reconnect without acceptance: %d %s", w.Code, w.Body.String())
	}
	if w := request(true); w.Code != http.StatusOK {
		t.Fatalf("explicit identity enrollment: %d %s", w.Code, w.Body.String())
	}
	if got := mappingAt(t, db, f.root.ID, path).Identity; got == "" {
		t.Fatal("accepted identity was not persisted")
	}
}

func TestAttachmentPreviewValidatesRequestBoundary(t *testing.T) {
	f := newTaskMutationFixture(t, 1)
	path := filepath.Join(t.TempDir(), "unknown")
	for _, tc := range []struct {
		name string
		body map[string]string
		want int
	}{
		{"missing Root identity", map[string]string{"RepoID": f.repos[0].ID, "TaskID": "T-1", "Path": path}, http.StatusBadRequest},
		{"relative checkout", map[string]string{"RootID": f.root.ID, "RepoID": f.repos[0].ID, "TaskID": "T-1", "Path": "relative"}, http.StatusBadRequest},
		{"unknown Root identity", map[string]string{"RootID": "missing-root", "RepoID": f.repos[0].ID, "TaskID": "T-1", "Path": path}, http.StatusNotFound},
		{"Repo outside selected Root", map[string]string{"RootID": f.root.ID, "RepoID": "missing-repo", "TaskID": "T-1", "Path": path}, http.StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := f.call(t, http.MethodPost, "/api/tasks/attach/preview", tc.body)
			if w.Code != tc.want {
				t.Fatalf("status %d, want %d: %s", w.Code, tc.want, w.Body.String())
			}
		})
	}
	if w := request(t, f.s, http.MethodPost, "/api/tasks/attach/preview", f.h, "{"); w.Code != http.StatusBadRequest {
		t.Errorf("malformed JSON status %d: %s", w.Code, w.Body.String())
	}
	if w := request(t, f.s, http.MethodPost, "/api/tasks/attach/preview", sameOrigin(f.s), `{"RootID":"`+f.root.ID+`"}`); w.Code != http.StatusUnauthorized {
		t.Errorf("unauthenticated status %d: %s", w.Code, w.Body.String())
	}
	wrongOrigin := f.h.Clone()
	wrongOrigin.Set("Origin", "http://evil.test")
	if w := request(t, f.s, http.MethodPost, "/api/tasks/attach/preview", wrongOrigin, `{"RootID":"`+f.root.ID+`"}`); w.Code != http.StatusForbidden {
		t.Errorf("wrong Origin status %d: %s", w.Code, w.Body.String())
	}
}

func TestDetachRequiresConfirmation(t *testing.T) {
	s := newTestServer(t)
	h := authorizedHeaders(s)
	h.Set("Content-Type", "application/json")
	body, _ := json.Marshal(map[string]any{"RootID": "root", "Mapping": store.Worktree{ID: "mapping", TaskID: "task", RepoID: "repo", Path: t.TempDir(), Identity: "identity"}})
	w := request(t, s, http.MethodPost, "/api/tasks/detach", h, string(body))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("unconfirmed detach: %d %s", w.Code, w.Body.String())
	}
}
