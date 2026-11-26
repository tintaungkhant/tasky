package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tintaungkhant/tasky/internal/store"
	"github.com/tintaungkhant/tasky/internal/work"
)

func workspaceForTask(t *testing.T, f taskMutationFixture, taskID string) *httptest.ResponseRecorder {
	t.Helper()
	return f.call(t, http.MethodPost, "/api/tasks/workspace", map[string]string{"RootID": f.root.ID, "TaskID": taskID})
}

func createWorkspaceTaskMappings(t *testing.T, f taskMutationFixture, paths ...string) store.Task {
	t.Helper()
	for i, path := range paths {
		branch := "workspace/" + string(rune('a'+i))
		gitTest(t, f.repos[0].Path, "worktree", "add", "-b", branch, path)
		review := attachmentReview(t, f, path, "TASK-WORKSPACE")
		if w := attachReviewed(t, f, review, ""); w.Code != http.StatusOK {
			t.Fatalf("attach worktree: %d %s", w.Code, w.Body.String())
		}
	}
	db := associationDB(t, f)
	tasks, err := db.ListTasks(f.root.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, task := range tasks {
		if task.TaskID == "TASK-WORKSPACE" {
			return task
		}
	}
	t.Fatal("attached Task not found")
	return store.Task{}
}

func TestWorkspaceEndpointUsesAllVerifiedMappingsAndIsIdempotent(t *testing.T) {
	f := newTaskMutationFixture(t, 1)
	paths := []string{filepath.Join(t.TempDir(), "first"), filepath.Join(t.TempDir(), "second")}
	task := createWorkspaceTaskMappings(t, f, paths...)
	w := workspaceForTask(t, f, task.ID)
	if w.Code != http.StatusOK {
		t.Fatalf("generate workspace: %d %s", w.Code, w.Body.String())
	}
	var result struct {
		Path    string
		Current bool
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Path == "" || result.Current {
		t.Fatalf("first result = %+v", result)
	}
	data, err := os.ReadFile(result.Path)
	if err != nil {
		t.Fatal(err)
	}
	var workspace struct {
		Folders []struct{ Path string }
	}
	if err := json.Unmarshal(data, &workspace); err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, folder := range workspace.Folders {
		resolved := folder.Path
		if !filepath.IsAbs(resolved) {
			resolved = filepath.Join(filepath.Dir(result.Path), filepath.FromSlash(resolved))
		}
		resolved, err = filepath.Abs(resolved)
		if err != nil {
			t.Fatal(err)
		}
		got[filepath.Clean(resolved)] = true
	}
	for _, path := range paths {
		if !got[filepath.Clean(path)] {
			t.Errorf("workspace omits mapped worktree %s: %+v", path, workspace.Folders)
		}
	}
	w = workspaceForTask(t, f, task.ID)
	if w.Code != http.StatusOK {
		t.Fatalf("repeat workspace: %d %s", w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil || !result.Current {
		t.Fatalf("repeat result = %+v, decode error %v", result, err)
	}
}

func TestWorkspaceEndpointPreservesEditedWorkspace(t *testing.T) {
	f := newTaskMutationFixture(t, 1)
	path := filepath.Join(t.TempDir(), "checkout")
	task := createWorkspaceTaskMappings(t, f, path)
	w := workspaceForTask(t, f, task.ID)
	if w.Code != http.StatusOK {
		t.Fatalf("generate workspace: %d %s", w.Code, w.Body.String())
	}
	var result struct{ Path string }
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	custom := []byte(`{"folders":[],"settings":{"editor.fontSize":17}}`)
	if err := os.WriteFile(result.Path, custom, 0600); err != nil {
		t.Fatal(err)
	}
	w = workspaceForTask(t, f, task.ID)
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "preserve or move it") {
		t.Fatalf("edited workspace response: %d %s", w.Code, w.Body.String())
	}
	if got, err := os.ReadFile(result.Path); err != nil || string(got) != string(custom) {
		t.Fatalf("edited workspace changed: %q, %v", got, err)
	}
}

func TestWorkspaceEndpointRejectsInvalidMappingsWithoutWriting(t *testing.T) {
	f := newTaskMutationFixture(t, 1)
	paths := []string{filepath.Join(t.TempDir(), "valid"), filepath.Join(t.TempDir(), "invalid")}
	task := createWorkspaceTaskMappings(t, f, paths...)
	if err := os.Rename(paths[1], paths[1]+"-moved"); err != nil {
		t.Fatal(err)
	}
	w := workspaceForTask(t, f, task.ID)
	if w.Code < 400 || !strings.Contains(w.Body.String(), "no workspace written") {
		t.Fatalf("invalid mapping response: %d %s", w.Code, w.Body.String())
	}
	dir := work.TaskDirectory(f.base, task.ID)
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("invalid mapping created workspace directory: %v", err)
	}
}

func TestWorkspaceEndpointUsesExactRootScopedInternalTaskID(t *testing.T) {
	f := newTaskMutationFixture(t, 1)
	path := filepath.Join(t.TempDir(), "checkout")
	task := createWorkspaceTaskMappings(t, f, path)
	otherRoot := filepath.Join(t.TempDir(), "other-root")
	if err := os.MkdirAll(otherRoot, 0755); err != nil {
		t.Fatal(err)
	}
	w := f.call(t, http.MethodPost, "/api/registration/roots", map[string]string{"Name": "Other", "Path": otherRoot})
	if w.Code != http.StatusCreated {
		t.Fatalf("register other Root: %d %s", w.Code, w.Body.String())
	}
	var other store.Root
	if err := json.Unmarshal(w.Body.Bytes(), &other); err != nil {
		t.Fatal(err)
	}
	otherTask, err := associationDB(t, f).CreateTask(other.ID, "TASK-WORKSPACE", "")
	if err != nil {
		t.Fatal(err)
	}
	w = workspaceForTask(t, f, otherTask.ID)
	if w.Code != http.StatusNotFound {
		t.Fatalf("foreign Task ID status %d: %s", w.Code, w.Body.String())
	}
	if w := workspaceForTask(t, f, "TASK-WORKSPACE"); w.Code != http.StatusNotFound {
		t.Fatalf("Task ID accepted where internal ID required: %d %s", w.Code, w.Body.String())
	}
	if w := f.call(t, http.MethodPost, "/api/tasks/workspace", map[string]any{"RootID": f.root.ID, "TaskID": task.ID, "extra": true}); w.Code != http.StatusBadRequest {
		t.Fatalf("unknown request field status %d: %s", w.Code, w.Body.String())
	}
	if w := request(t, f.s, http.MethodPost, "/api/tasks/workspace", sameOrigin(f.s), `{"RootID":"`+f.root.ID+`","TaskID":"`+task.ID+`"}`); w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated request status %d: %s", w.Code, w.Body.String())
	}
	wrongOrigin := f.h.Clone()
	wrongOrigin.Set("Origin", "http://evil.test")
	if w := request(t, f.s, http.MethodPost, "/api/tasks/workspace", wrongOrigin, `{"RootID":"`+f.root.ID+`","TaskID":"`+task.ID+`"}`); w.Code != http.StatusForbidden {
		t.Fatalf("wrong Origin status %d: %s", w.Code, w.Body.String())
	}
}
