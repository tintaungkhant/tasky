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

// WorktreePath computes the path a Task's worktree for repoName and branch
// lives at: <tasksDir>/<taskID>/<repoName>[<branch>], with slashes in branch
// replaced by dashes so the whole thing stays a single path segment. Folding
// the branch into the path lets the same task+repo have multiple worktrees
// checked out on different branches at once.
func WorktreePath(tasksDir, taskID, repoName, branch string) string {
	sanitizedBranch := strings.ReplaceAll(branch, "/", "-")
	return filepath.Join(tasksDir, taskID, repoName+"["+sanitizedBranch+"]")
}
