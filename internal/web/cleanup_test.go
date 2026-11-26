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
	"github.com/tintaungkhant/tasky/internal/store"
)

func cleanupPreview(t *testing.T, f taskMutationFixture, mappings []store.Worktree, options app.RemoveOptions) app.CleanupReview {
	t.Helper()
	w := f.call(t, http.MethodPost, "/api/tasks/cleanup/preview", map[string]any{
		"RootID": f.root.ID, "Mappings": mappings, "Options": options,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("cleanup preview: %d %s", w.Code, w.Body.String())
	}
	var review app.CleanupReview
	if err := json.Unmarshal(w.Body.Bytes(), &review); err != nil {
		t.Fatal(err)
	}
	return review
}

func submitCleanup(t *testing.T, f taskMutationFixture, review app.CleanupReview, confirm bool) *httptest.ResponseRecorder {
	t.Helper()
	return f.call(t, http.MethodPost, "/api/tasks/cleanup", map[string]any{"Review": review, "Confirm": confirm})
}

func TestDashboardCleanupBatchPartialAndRestartRecovery(t *testing.T) {
	f := newTaskMutationFixture(t, 1)
	task := createWorkspaceTaskMappings(t, f, filepath.Join(f.base, "clean"), filepath.Join(f.base, "dirty"), filepath.Join(f.base, "untouched"))
	db := associationDB(t, f)
	mappings, err := db.ListWorktrees(task.ID)
	if err != nil || len(mappings) != 3 {
		t.Fatalf("mappings: %+v %v", mappings, err)
	}
	if err := os.WriteFile(filepath.Join(mappings[1].Path, "keep.txt"), []byte("user data"), 0600); err != nil {
		t.Fatal(err)
	}
	workspace := filepath.Join(filepath.Dir(mappings[0].Path), "custom.code-workspace")
	if err := os.WriteFile(workspace, []byte("custom settings"), 0600); err != nil {
		t.Fatal(err)
	}
	review := cleanupPreview(t, f, mappings, app.RemoveOptions{})
	if review.RootID != f.root.ID || len(review.Targets) != 3 {
		t.Fatalf("unexpected cleanup review: %+v", review)
	}
	if got := submitCleanup(t, f, review, false); got.Code != http.StatusBadRequest {
		t.Fatalf("unconfirmed cleanup status: %d %s", got.Code, got.Body.String())
	}
	w := submitCleanup(t, f, review, true)
	if w.Code != http.StatusOK {
		t.Fatalf("cleanup: %d %s", w.Code, w.Body.String())
	}
	var result app.CleanupResult
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Completed) != 1 || result.Failed == nil || len(result.Remaining) != 1 || result.Remaining[0].ID != mappings[2].ID || result.Error == "" {
		t.Fatalf("cleanup did not stop at dirty checkout: %+v", result)
	}
	pending := f.call(t, http.MethodGet, "/api/pending?root="+f.root.ID, nil)
	if pending.Code != http.StatusOK || !strings.Contains(pending.Body.String(), "Cleanups") {
		t.Fatalf("pending cleanup: %d %s", pending.Code, pending.Body.String())
	}
	var pendingData struct{ Cleanups []json.RawMessage }
	if err := json.Unmarshal(pending.Body.Bytes(), &pendingData); err != nil || len(pendingData.Cleanups) != 1 {
		t.Fatalf("pending journal: %+v %v", pendingData, err)
	}
	// Recovery uses a fresh Server and credentials while retaining the same DB/config.
	f.s.Close()
	restarted, err := New(f.s.configPath, f.s.cwd, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(restarted.Close)
	f.s = restarted
	f.h = authorizedHeaders(restarted)
	f.h.Set("Content-Type", "application/json")
	var operation struct{ ID string }
	if err := json.Unmarshal(pendingData.Cleanups[0], &operation); err != nil || operation.ID == "" {
		t.Fatalf("cleanup operation: %+v %v", operation, err)
	}
	recovered := f.call(t, http.MethodPost, "/api/recover", map[string]any{
		"RootID": f.root.ID, "ID": operation.ID, "Options": map[string]bool{"ForceWorktree": true}, "Confirm": true,
	})
	if recovered.Code != http.StatusOK {
		t.Fatalf("recover: %d %s", recovered.Code, recovered.Body.String())
	}
	if _, err := os.Stat(mappings[1].Path); !os.IsNotExist(err) {
		t.Fatalf("dirty checkout remains after forced recovery: %v", err)
	}
	if _, err := os.Stat(filepath.Join(mappings[2].Path, ".git")); err != nil {
		t.Fatalf("unattempted sibling checkout changed: %v", err)
	}
	if data, err := os.ReadFile(workspace); err != nil || string(data) != "custom settings" {
		t.Fatalf("workspace changed: %q %v", data, err)
	}
}

func TestDashboardCleanupBranchProtectionAndIndependentForceOptions(t *testing.T) {
	f := newTaskMutationFixture(t, 1)
	task := createWorkspaceTaskMappings(t, f, filepath.Join(f.base, "branch"))
	db := associationDB(t, f)
	mappings, err := db.ListWorktrees(task.ID)
	if err != nil || len(mappings) != 1 {
		t.Fatalf("mappings: %+v %v", mappings, err)
	}
	branch := "workspace/a"
	if !strings.Contains(gitTest(t, f.repos[0].Path, "show-ref", "--heads", branch), branch) {
		t.Fatalf("branch %q absent", branch)
	}
	gitTest(t, mappings[0].Path, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "--allow-empty", "-m", "unmerged")
	if err := os.WriteFile(filepath.Join(mappings[0].Path, "dirty.txt"), []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	options := app.RemoveOptions{DeleteBranch: true}
	review := cleanupPreview(t, f, mappings, options)
	w := submitCleanup(t, f, review, true)
	if w.Code != http.StatusOK {
		t.Fatalf("cleanup: %d %s", w.Code, w.Body.String())
	}
	var result app.CleanupResult
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Failed == nil || len(result.Completed) != 0 {
		t.Fatalf("branch deletion should fail before removal: %+v", result)
	}
	if _, err := os.Stat(mappings[0].Path); err != nil {
		t.Fatalf("checkout removed despite branch protection: %v", err)
	}
	if !strings.Contains(result.Error, "recover") {
		t.Fatalf("missing recovery guidance: %s", result.Error)
	}
	// ForceBranch alone must not bypass dirty-worktree protection.
	pending := f.call(t, http.MethodGet, "/api/pending?root="+f.root.ID, nil)
	var pendingData struct{ Cleanups []struct{ ID string } }
	if pending.Code != http.StatusOK {
		t.Fatalf("pending: %d %s", pending.Code, pending.Body.String())
	}
	if err := json.Unmarshal(pending.Body.Bytes(), &pendingData); err != nil || len(pendingData.Cleanups) != 1 {
		t.Fatalf("cleanup journal: %+v %v", pendingData, err)
	}
	operationID := pendingData.Cleanups[0].ID
	forced := f.call(t, http.MethodPost, "/api/recover", map[string]any{
		"RootID": f.root.ID, "ID": operationID, "Options": map[string]bool{"ForceBranch": true}, "Confirm": true,
	})
	if forced.Code == http.StatusOK {
		t.Fatalf("ForceBranch incorrectly bypassed dirty worktree: %s", forced.Body.String())
	}
	if _, err := os.Stat(mappings[0].Path); err != nil {
		t.Fatalf("dirty checkout removed: %v", err)
	}
	forgotten := f.call(t, http.MethodPost, "/api/recover", map[string]any{
		"RootID": f.root.ID, "ID": operationID, "Options": map[string]bool{"Forget": true}, "Confirm": true,
	})
	if forgotten.Code != http.StatusOK {
		t.Fatalf("forget dirty cleanup intent: %d %s", forgotten.Code, forgotten.Body.String())
	}
	// A new intent starts with ForceBranch unset, so ForceWorktree can remove
	// the dirty checkout and leave an independently recoverable branch failure.
	review = cleanupPreview(t, f, mappings, options)
	w = submitCleanup(t, f, review, true)
	if w.Code != http.StatusOK {
		t.Fatalf("retry cleanup: %d %s", w.Code, w.Body.String())
	}
	pending = f.call(t, http.MethodGet, "/api/pending?root="+f.root.ID, nil)
	if pending.Code != http.StatusOK {
		t.Fatalf("pending after retry: %d %s", pending.Code, pending.Body.String())
	}
	if err := json.Unmarshal(pending.Body.Bytes(), &pendingData); err != nil || len(pendingData.Cleanups) != 1 {
		t.Fatalf("cleanup retry journal: %+v %v", pendingData, err)
	}
	operationID = pendingData.Cleanups[0].ID
	removed := f.call(t, http.MethodPost, "/api/recover", map[string]any{
		"RootID": f.root.ID, "ID": operationID, "Options": map[string]bool{"ForceWorktree": true}, "Confirm": true,
	})
	if removed.Code != http.StatusInternalServerError {
		t.Fatalf("force worktree should stop at unmerged branch: %d %s", removed.Code, removed.Body.String())
	}
	if _, err := os.Stat(mappings[0].Path); !os.IsNotExist(err) {
		t.Fatalf("force-worktree did not remove checkout: %v", err)
	}
	recovered := f.call(t, http.MethodPost, "/api/recover", map[string]any{
		"RootID": f.root.ID, "ID": operationID, "Options": map[string]bool{"ForceBranch": true}, "Confirm": true,
	})
	if recovered.Code != http.StatusOK {
		t.Fatalf("forced recovery: %d %s", recovered.Code, recovered.Body.String())
	}
	if strings.TrimSpace(gitTest(t, f.repos[0].Path, "branch", "--list", branch)) != "" {
		t.Fatalf("branch remains: %s", branch)
	}
}

func TestDashboardCleanupRejectsStaleReviewsAndReplacementCheckouts(t *testing.T) {
	f := newTaskMutationFixture(t, 1)
	task := createWorkspaceTaskMappings(t, f, filepath.Join(f.base, "stale"))
	db := associationDB(t, f)
	mappings, err := db.ListWorktrees(task.ID)
	if err != nil || len(mappings) != 1 {
		t.Fatalf("mappings: %+v %v", mappings, err)
	}
	review := cleanupPreview(t, f, mappings, app.RemoveOptions{})
	gitTest(t, mappings[0].Path, "checkout", "--detach")
	changed := submitCleanup(t, f, review, true)
	if changed.Code != http.StatusConflict {
		t.Fatalf("stale branch review: %d %s", changed.Code, changed.Body.String())
	}
	if _, err := os.Stat(mappings[0].Path); err != nil {
		t.Fatalf("stale review removed checkout: %v", err)
	}
	gitTest(t, mappings[0].Path, "checkout", review.Targets[0].Branch)
	newReview := cleanupPreview(t, f, mappings, app.RemoveOptions{})
	gitTest(t, mappings[0].Path, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "--allow-empty", "-m", "review became stale")
	commitChanged := submitCleanup(t, f, newReview, true)
	if commitChanged.Code != http.StatusConflict {
		t.Fatalf("stale commit review: %d %s", commitChanged.Code, commitChanged.Body.String())
	}
	newReview = cleanupPreview(t, f, mappings, app.RemoveOptions{})
	if _, err := db.Attach(f.root.ID, "TASK-REASSIGNED", "Reassigned", f.repos[0].ID, mappings[0].Path, task.ID, mappings[0].Identity); err != nil {
		t.Fatal(err)
	}
	reassigned := submitCleanup(t, f, newReview, true)
	if reassigned.Code != http.StatusConflict {
		t.Fatalf("reassigned mapping review: %d %s", reassigned.Code, reassigned.Body.String())
	}
	newTask, err := db.FindTask(f.root.ID, "TASK-REASSIGNED")
	if err != nil {
		t.Fatal(err)
	}
	mappings, err = db.ListWorktrees(newTask.ID)
	if err != nil || len(mappings) != 1 {
		t.Fatalf("reassigned mapping: %+v %v", mappings, err)
	}
	newReview = cleanupPreview(t, f, mappings, app.RemoveOptions{})
	oldPath := mappings[0].Path
	moved := filepath.Join(t.TempDir(), "moved")
	gitTest(t, f.repos[0].Path, "worktree", "move", oldPath, moved)
	gitTest(t, f.repos[0].Path, "worktree", "add", "-b", "replacement", oldPath)
	rejected := submitCleanup(t, f, newReview, true)
	if rejected.Code != http.StatusConflict {
		t.Fatalf("replacement checkout status: %d %s", rejected.Code, rejected.Body.String())
	}
	if _, err := os.Stat(filepath.Join(oldPath, ".git")); err != nil {
		t.Fatalf("replacement checkout damaged: %v", err)
	}
	if _, err := os.Stat(filepath.Join(moved, ".git")); err != nil {
		t.Fatalf("original moved checkout damaged: %v", err)
	}
}

func TestDashboardRecoveryRootScopeConfirmAndForgetCreation(t *testing.T) {
	f := newTaskMutationFixture(t, 1)
	otherPath := filepath.Join(t.TempDir(), "other-root")
	if err := os.MkdirAll(otherPath, 0755); err != nil {
		t.Fatal(err)
	}
	otherResponse := f.call(t, http.MethodPost, "/api/registration/roots", map[string]string{"Name": "Other", "Path": otherPath})
	if otherResponse.Code != http.StatusCreated {
		t.Fatalf("register other Root: %d %s", otherResponse.Code, otherResponse.Body.String())
	}
	var other store.Root
	if err := json.Unmarshal(otherResponse.Body.Bytes(), &other); err != nil {
		t.Fatal(err)
	}
	db := associationDB(t, f)
	task, err := db.CreateTask(f.root.ID, "TASK-RECOVERY", "Recovery")
	if err != nil {
		t.Fatal(err)
	}
	creationPath := filepath.Join(t.TempDir(), "recoverable")
	gitTest(t, f.repos[0].Path, "worktree", "add", "-b", "recovery/creation", creationPath)
	creation, err := db.ReserveCreation(task.ID, f.repos[0].ID, "recovery/creation", creationPath)
	if err != nil {
		t.Fatal(err)
	}
	wrongRoot := f.call(t, http.MethodPost, "/api/recover", map[string]any{
		"RootID": other.ID, "ID": creation.ID, "Options": map[string]bool{"Forget": true}, "Confirm": true,
	})
	if wrongRoot.Code != http.StatusNotFound {
		t.Fatalf("wrong Root status: %d %s", wrongRoot.Code, wrongRoot.Body.String())
	}
	noConfirm := f.call(t, http.MethodPost, "/api/recover", map[string]any{
		"RootID": f.root.ID, "ID": creation.ID, "Options": map[string]bool{"Forget": true}, "Confirm": false,
	})
	if noConfirm.Code != http.StatusBadRequest {
		t.Fatalf("unconfirmed recovery: %d %s", noConfirm.Code, noConfirm.Body.String())
	}
	unknown := f.call(t, http.MethodPost, "/api/recover", map[string]any{
		"RootID": f.root.ID, "ID": "unknown-operation", "Confirm": true,
	})
	if unknown.Code != http.StatusNotFound {
		t.Fatalf("unknown operation: %d %s", unknown.Code, unknown.Body.String())
	}
	recovered := f.call(t, http.MethodPost, "/api/recover", map[string]any{
		"RootID": f.root.ID, "ID": creation.ID, "Confirm": true,
	})
	if recovered.Code != http.StatusOK {
		t.Fatalf("recover creation: %d %s", recovered.Code, recovered.Body.String())
	}
	if _, err := os.Stat(filepath.Join(creationPath, ".git")); err != nil {
		t.Fatalf("recovery removed the checkout: %v", err)
	}
	recoveredMappings, err := db.RootMappings(f.root.ID)
	if err != nil || len(recoveredMappings.Worktrees) != 1 || recoveredMappings.Worktrees[0].Path != creationPath {
		t.Fatalf("recovery failed to record existing worktree: %+v %v", recoveredMappings.Worktrees, err)
	}
	forgottenPath := filepath.Join(t.TempDir(), "forgotten")
	gitTest(t, f.repos[0].Path, "worktree", "add", "-b", "recovery/forgotten", forgottenPath)
	marker := filepath.Join(forgottenPath, "keep.txt")
	if err := os.WriteFile(marker, []byte("preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	forgetIntent, err := db.ReserveCreation(task.ID, f.repos[0].ID, "recovery/forgotten", forgottenPath)
	if err != nil {
		t.Fatal(err)
	}
	forgotten := f.call(t, http.MethodPost, "/api/recover", map[string]any{
		"RootID": f.root.ID, "ID": forgetIntent.ID, "Options": map[string]bool{"Forget": true}, "Confirm": true,
	})
	if forgotten.Code != http.StatusOK {
		t.Fatalf("forget creation: %d %s", forgotten.Code, forgotten.Body.String())
	}
	if data, err := os.ReadFile(marker); err != nil || string(data) != "preserve" {
		t.Fatalf("forget changed user file: %q %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(forgottenPath, ".git")); err != nil {
		t.Fatalf("forget removed checkout: %v", err)
	}
	pending := f.call(t, http.MethodGet, "/api/pending?root="+f.root.ID, nil)
	if pending.Code != http.StatusOK || strings.Contains(pending.Body.String(), creation.ID) || strings.Contains(pending.Body.String(), forgetIntent.ID) {
		t.Fatalf("creation intent remains: %d %s", pending.Code, pending.Body.String())
	}
}
