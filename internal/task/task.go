// Package task holds tasky's pure domain logic: branch-template rendering
// and worktree path calculation.
package task

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
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

// Entry is one task directory under a Root's tasksDir, split back into its ID
// and optional human-readable name (see ParseTaskDirName).
type Entry struct {
	ID   string
	Name string
}

// Label formats an entry for display: just the ID, or "ID (name)" when named.
func (e Entry) Label() string {
	if e.Name == "" {
		return e.ID
	}
	return fmt.Sprintf("%s (%s)", e.ID, e.Name)
}

// ListEntries returns every task directory under tasksDir, parsed into
// (ID, Name) pairs, sorted by ID. A missing tasksDir yields no entries.
func ListEntries(tasksDir string) ([]Entry, error) {
	entries, err := os.ReadDir(tasksDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read tasks directory %s: %w", tasksDir, err)
	}
	var out []Entry
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		id, name := ParseTaskDirName(e.Name())
		out = append(out, Entry{ID: id, Name: name})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// FindEntry looks up a task by ID among entries (see ListEntries).
func FindEntry(entries []Entry, id string) (Entry, bool) {
	for _, e := range entries {
		if e.ID == id {
			return e, true
		}
	}
	return Entry{}, false
}
