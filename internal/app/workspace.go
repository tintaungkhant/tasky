package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tintaungkhant/tasky/internal/task"
	"github.com/tintaungkhant/tasky/internal/work"
)

type Workspace struct {
	Folders []Folder `json:"folders"`
}

type Folder struct {
	Path string `json:"path"`
}

type WorkspaceResult struct {
	Path    string
	Current bool
}

func WriteWorkspace(path string, ws Workspace) (WorkspaceResult, error) {
	data, err := json.MarshalIndent(ws, "", "  ")
	if err != nil {
		return WorkspaceResult{}, fmt.Errorf("build workspace file: %w", err)
	}

	if err := PublishNewFile(path, data); err != nil {
		if errors.Is(err, os.ErrExist) {
			previous, readErr := os.ReadFile(path)
			if readErr == nil && bytes.Equal(previous, data) {
				return WorkspaceResult{Path: path, Current: true}, nil
			}
			return WorkspaceResult{}, fmt.Errorf("workspace file %s exists; preserve or move it before regenerating", path)
		}
		return WorkspaceResult{}, fmt.Errorf("write workspace file %s: %w", path, err)
	}
	return WorkspaceResult{Path: path}, nil
}

func (c *Context) GenerateWorkspace(taskID string) (WorkspaceResult, error) {
	t, err := c.Task(taskID)
	if err != nil {
		return WorkspaceResult{}, err
	}
	wts, mappings, repos, err := c.Worktrees(t.TaskID)
	if err != nil {
		return WorkspaceResult{}, err
	}
	if len(wts) == 0 {
		return WorkspaceResult{}, fmt.Errorf("no worktrees found for Task %s", t.TaskID)
	}
	for _, wt := range wts {
		mapping := mappings[wt.Path]
		state := (work.Manager{Store: c.Store}).Inspect(repos[mapping.RepoID], mapping)
		if state.Status != "ok" {
			return WorkspaceResult{}, fmt.Errorf("cannot generate workspace for Task %s: %s [%s]: %s; no workspace written", t.TaskID, wt.Path, state.Status, state.Detail)
		}
	}
	dir := work.TaskDirectory(c.Config.TasksDir, t.ID)
	filename := t.ID + ".code-workspace"
	// Preserve the location used by an adopted legacy workspace.
	legacy := filepath.Join(c.Config.TasksDir, task.TaskDirName(t.TaskID, t.Name))
	if filepath.Dir(wts[0].Path) == legacy {
		dir = legacy
		filename = filepath.Base(legacy) + ".code-workspace"
	} else {
		dir, err = work.EnsureTaskDirectory(c.Config.TasksDir, t.ID)
		if err != nil {
			return WorkspaceResult{}, err
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return WorkspaceResult{}, err
	}
	ws := Workspace{}
	for _, wt := range wts {
		rel, err := filepath.Rel(dir, wt.Path)
		if err != nil {
			ws.Folders = append(ws.Folders, Folder{Path: wt.Path})
			continue
		}
		ws.Folders = append(ws.Folders, Folder{Path: "./" + filepath.ToSlash(rel)})
	}
	path := filepath.Join(dir, filename)
	return WriteWorkspace(path, ws)
}
