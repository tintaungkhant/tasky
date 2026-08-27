// Package task holds tasky's pure domain logic: branch-template rendering
// and worktree path calculation.
package task

import (
	"path/filepath"
	"strings"
)

// RenderBranch substitutes {task-id} and {branch} into template, verbatim,
// with no truncation or sanitization.
func RenderBranch(template, taskID, branch string) string {
	r := strings.NewReplacer("{task-id}", taskID, "{branch}", branch)
	return r.Replace(template)
}

// WorktreePath computes the path a Task's worktree for repoName lives at:
// <tasksDir>/<taskID>/<repoName>.
func WorktreePath(tasksDir, taskID, repoName string) string {
	return filepath.Join(tasksDir, taskID, repoName)
}
