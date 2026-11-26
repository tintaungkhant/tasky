package cmd

import (
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/tintaungkhant/tasky/internal/config"
	"github.com/tintaungkhant/tasky/internal/git"
	"github.com/tintaungkhant/tasky/internal/task"
	"github.com/tintaungkhant/tasky/internal/work"
)

func init() {
	listCmd.ValidArgsFunction = completeTaskID
	vscodeCmd.ValidArgsFunction = completeTaskID
	deleteCmd.ValidArgsFunction = completeDeleteArgs
}

// currentRootTaskEntries loads the config and matches the current
// directory's Root, returning its task entries. Completion functions must
// never error the shell, so any failure here yields no suggestions rather
// than propagating an error.
func currentRootTaskEntries() (*config.Root, []task.Entry, bool) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, nil, false
	}
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, false
	}
	root, err := cfg.MatchRoot(cwd)
	if err != nil {
		return nil, nil, false
	}
	entries, err := task.ListEntries(root.TasksDir)
	if err != nil {
		return nil, nil, false
	}
	return root, entries, true
}

// filterPrefix narrows candidates to those starting with toComplete, since
// not every shell's completion driver filters custom completions itself.
func filterPrefix(candidates []string, toComplete string) []string {
	if toComplete == "" {
		return candidates
	}
	var out []string
	for _, c := range candidates {
		if strings.HasPrefix(c, toComplete) {
			out = append(out, c)
		}
	}
	return out
}

func taskIDs(entries []task.Entry) []string {
	ids := make([]string, len(entries))
	for i, e := range entries {
		ids[i] = e.ID
	}
	return ids
}

// completeTaskID completes a single leading task-id argument (used by list
// and vscode, which each take just [task-id]).
func completeTaskID(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	if c, err := registeredContext(cmd); err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	} else if c != nil {
		defer c.Store.Close()
		entries, err := c.Entries()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return filterPrefix(taskIDs(entries), toComplete), cobra.ShellCompDirectiveNoFileComp
	}
	_, entries, ok := currentRootTaskEntries()
	if !ok {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return filterPrefix(taskIDs(entries), toComplete), cobra.ShellCompDirectiveNoFileComp
}

// completeDeleteArgs completes delete's <task-id> [repo] [branch] positional
// arguments, each narrowed by what's already been typed.
func completeDeleteArgs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if c, err := registeredContext(cmd); err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	} else if c != nil {
		defer c.Store.Close()
		entries, err := c.Entries()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		if len(args) == 0 {
			return filterPrefix(taskIDs(entries), toComplete), cobra.ShellCompDirectiveNoFileComp
		}
		wts, mappings, repos, err := c.Worktrees(args[0])
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		var candidates []string
		seen := make(map[string]bool)
		nameCount := make(map[string]int)
		for _, repo := range repos {
			nameCount[repo.Name]++
		}
		for _, wt := range wts {
			mapping := mappings[wt.Path]
			repo := repos[mapping.RepoID]
			candidate := ""
			if len(args) == 1 {
				candidate = wt.RepoName
				if nameCount[repo.Name] > 1 {
					candidate = repo.ID
				}
			} else if len(args) == 2 && (repo.Name == args[1] || repo.ID == args[1]) {
				state := (work.Manager{Store: c.Store}).Inspect(repo, mapping)
				if state.Status == "ok" && state.Branch != "(detached HEAD)" {
					candidate = state.Branch
				}
			}
			if candidate != "" && !seen[candidate] {
				candidates = append(candidates, candidate)
				seen[candidate] = true
			}
		}
		return filterPrefix(candidates, toComplete), cobra.ShellCompDirectiveNoFileComp
	}
	root, entries, ok := currentRootTaskEntries()
	if !ok {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	if len(args) == 0 {
		return filterPrefix(taskIDs(entries), toComplete), cobra.ShellCompDirectiveNoFileComp
	}
	if len(args) > 2 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	entry, found := task.FindEntry(entries, args[0])
	if !found {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	worktrees, err := task.ListWorktrees(root.TasksDir, task.TaskDirName(entry.ID, entry.Name))
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	if len(args) == 1 {
		seen := make(map[string]bool)
		var repos []string
		for _, wt := range worktrees {
			if !seen[wt.RepoName] {
				seen[wt.RepoName] = true
				repos = append(repos, wt.RepoName)
			}
		}
		return filterPrefix(repos, toComplete), cobra.ShellCompDirectiveNoFileComp
	}

	var branches []string
	for _, wt := range worktrees {
		if wt.RepoName != args[1] {
			continue
		}
		if branch, err := git.CurrentBranch(wt.Path); err == nil {
			branches = append(branches, branch)
		}
	}
	return filterPrefix(branches, toComplete), cobra.ShellCompDirectiveNoFileComp
}
