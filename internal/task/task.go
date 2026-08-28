// Package task holds tasky's pure domain logic: branch-template rendering
// and worktree path calculation.
package task

import (
	"path/filepath"
	"strings"
)

// RenderBranch substitutes {task-id}, {task-name}, and {branch} into
// template, verbatim, with no truncation or sanitization. taskName is
// commonly empty, since it's an optional field on a task.
func RenderBranch(template, taskID, taskName, branch string) string {
	r := strings.NewReplacer("{task-id}", taskID, "{task-name}", taskName, "{branch}", branch)
	return r.Replace(template)
}

// TaskDirName computes the name of a task's directory under tasksDir: just
// taskID when taskName is empty, or taskID+"[taskName]" when it's set. This
// mirrors WorktreePath's branch-folding so a task's optional human-readable
// name rides along in the directory listing without a second lookup.
func TaskDirName(taskID, taskName string) string {
	if taskName == "" {
		return taskID
	}
	return taskID + "[" + taskName + "]"
}

// ParseTaskDirName reverses TaskDirName: given a task directory's name, it
// returns the taskID and taskName (empty if the directory has no [name]
// suffix). It's how callers recover a task's ID to match against typed or
// selected input, regardless of whether that task has a name.
func ParseTaskDirName(dirName string) (taskID, taskName string) {
	if strings.HasSuffix(dirName, "]") {
		if i := strings.Index(dirName, "["); i != -1 {
			return dirName[:i], dirName[i+1 : len(dirName)-1]
		}
	}
	return dirName, ""
}

// WorktreePath computes the path a Task's worktree for repoName and branch
// lives at: <tasksDir>/<taskDirName>/<repoName>[<branch>], with slashes in
// branch replaced by dashes so the whole thing stays a single path segment.
// Folding the branch into the path lets the same task+repo have multiple
// worktrees checked out on different branches at once.
func WorktreePath(tasksDir, taskDirName, repoName, branch string) string {
	sanitizedBranch := strings.ReplaceAll(branch, "/", "-")
	return filepath.Join(tasksDir, taskDirName, repoName+"["+sanitizedBranch+"]")
}
