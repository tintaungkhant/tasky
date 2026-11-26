package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tintaungkhant/tasky/internal/app"
	"github.com/tintaungkhant/tasky/internal/config"
	"github.com/tintaungkhant/tasky/internal/git"
	"github.com/tintaungkhant/tasky/internal/store"
	"github.com/tintaungkhant/tasky/internal/task"
	"github.com/tintaungkhant/tasky/internal/work"
)

type storedContext = app.Context

func init() {
	rename := &cobra.Command{Use: "rename <task-id> <name>", Short: "Rename a Task without changing its identity, branches, or paths", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		c, err := requiredStored(cmd)
		if err != nil {
			return err
		}
		defer c.Store.Close()
		return c.Rename(args[0], args[1])
	}}
	pending := &cobra.Command{Use: "pending", Short: "List unfinished worktree creations and cleanup", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		c, err := requiredStored(cmd)
		if err != nil {
			return err
		}
		defer c.Store.Close()
		operations, err := c.Store.ListCreations(c.Root.ID)
		if err != nil {
			return err
		}
		for _, op := range operations {
			cmd.Printf("%s\t%s\t%s\n", op.ID, op.Branch, op.Path)
		}
		cleanups, err := c.Store.ListCleanups(c.Root.ID)
		if err != nil {
			return err
		}
		for _, op := range cleanups {
			cmd.Printf("%s\tcleanup\t%s\t%s\tworktree removed: %t; branch deletion requested: %t\n", op.ID, op.Branch, op.Path, op.Removed, op.DeleteBranch)
		}
		return nil
	}}
	recover := &cobra.Command{Use: "recover <operation-ID>", Short: "Recover a pending creation or cleanup, or abandon its intent", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		c, err := requiredStored(cmd)
		if err != nil {
			return err
		}
		defer c.Store.Close()
		forget, _ := cmd.Flags().GetBool("forget")
		fw, _ := cmd.Flags().GetBool("force-worktree")
		fb, _ := cmd.Flags().GetBool("force-branch")
		cleanup, err := c.Recover(args[0], app.RecoveryOptions{Forget: forget, ForceWorktree: fw, ForceBranch: fb})
		if err == nil && cleanup {
			cmd.Printf("Cleanup %s completed.\n", args[0])
		}
		return err
	}}
	recover.Flags().Bool("forget", false, "forget operation intent only; preserve all files, mappings, worktrees, and branches")
	recover.Flags().Bool("force-worktree", false, "override dirty-worktree protection for pending cleanup")
	recover.Flags().Bool("force-branch", false, "override unmerged-branch protection for pending cleanup")
	for _, cmd := range []*cobra.Command{addCmd, listCmd, deleteCmd, vscodeCmd, rename, pending, recover} {
		cmd.Flags().String("root", "", "registered Root name or ID (default: current-directory context)")
	}
	addCmd.Flags().String("repo", "", "registered Repo name or ID")
	addCmd.Flags().Bool("as-is", false, "use the branch argument verbatim instead of the Branch Template")
	rootCmd.AddCommand(rename, pending, recover)
}

func requiredStored(cmd *cobra.Command) (*storedContext, error) {
	c, err := registeredContext(cmd)
	if err == nil && c == nil {
		err = fmt.Errorf("register a Root and Repo, or run `tasky import` first")
	}
	return c, err
}

func registeredContext(cmd *cobra.Command) (*storedContext, error) {
	selector, _ := cmd.Flags().GetString("root")
	path, err := config.Path()
	if err != nil {
		return nil, err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return app.OpenRegistered(path, cwd, selector)
}

func runStoredAdd(cmd *cobra.Command, args []string, c *storedContext) error {
	interactive := len(args) == 0
	var repoArg, taskID, branch string
	if len(args) == 3 {
		repoArg, taskID, branch = args[0], args[1], args[2]
	} else if len(args) == 2 {
		taskID, branch = args[0], args[1]
	}
	selectedRepo, _ := cmd.Flags().GetString("repo")
	if repoArg != "" && selectedRepo != "" {
		return fmt.Errorf("use either positional Repo or --repo")
	}
	if selectedRepo != "" {
		repoArg = selectedRepo
	}
	ctx, err := c.Registry.Resolve(c.Cwd, c.Root.ID, repoArg)
	if err != nil && interactive {
		repos, listErr := c.Store.ListRepos(c.Root.ID)
		if listErr != nil {
			return listErr
		}
		var ids []string
		for _, repo := range repos {
			ids = append(ids, repo.ID)
		}
		if len(ids) == 0 {
			return err
		}
		id, promptErr := selectOne("Repo ID (see tasky repo list)", stringOptions(ids))
		if promptErr != nil {
			return promptErr
		}
		ctx, err = c.Registry.Resolve(c.Cwd, c.Root.ID, id)
	}
	if err != nil {
		return err
	}
	entries, err := c.Entries()
	if err != nil {
		return err
	}
	if taskID == "" {
		taskID, err = promptSelectOrNewTaskID(entries)
		if err != nil {
			return err
		}
	}
	name := addNameFlag
	if existing, ok := task.FindEntry(entries, taskID); ok {
		name = existing.Name
	} else if interactive && name == "" {
		name, err = promptOptionalText("Task name (optional)")
		if err != nil {
			return err
		}
	}
	base := addFromFlag
	if base == "" {
		base, err = git.CurrentBranch(ctx.Repo.Path)
		if err != nil {
			return err
		}
	}
	branches, err := git.ListLocalBranches(ctx.Repo.Path)
	if err != nil {
		return err
	}
	if interactive {
		base, err = promptSelectBase(branches, base)
		if err != nil {
			return err
		}
	}
	asIs, _ := cmd.Flags().GetBool("as-is")
	if branch == "" {
		branch, asIs, err = promptBranchStep(taskID, name, base, c.Config.BranchTemplate)
		if err != nil {
			return err
		}
	}
	branch, err = c.Branch(ctx.Repo, taskID, name, branch, asIs)
	if err != nil {
		return err
	}
	if interactive {
		ok, err := promptConfirm(fmt.Sprintf("Create worktree for %s on %s?", taskID, branch))
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("aborted")
		}
	}
	result, err := c.Create(app.CreateRequest{TaskID: taskID, Name: name, Repo: ctx.Repo.ID, Branch: branch, Base: base, AsIs: true})
	if err != nil {
		return err
	}
	cmd.Printf("Created worktree at %s (branch %s)\n", result.Path, result.Branch)
	return nil
}

func selectedTask(cmd *cobra.Command, args []string, c *storedContext) (store.Task, error) {
	id := ""
	if len(args) > 0 {
		id = args[0]
	}
	if id == "" {
		entries, err := c.Entries()
		if err != nil {
			return store.Task{}, err
		}
		if len(entries) == 0 {
			return store.Task{}, fmt.Errorf("no tasks found in Root %s", c.Root.Name)
		}
		id, err = selectOneFiltering("Task ID", taskIDOptions(entries))
		if err != nil {
			return store.Task{}, err
		}
	}
	return c.Task(id)
}

func runStoredList(cmd *cobra.Command, args []string, c *storedContext) error {
	entries, err := c.Entries()
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		cmd.Printf("no tasks found in Root %s\n", c.Root.Name)
		return nil
	}
	if len(args) > 0 || !listAllFlag {
		t, err := selectedTask(cmd, args, c)
		if err != nil {
			return err
		}
		entries = []task.Entry{{ID: t.TaskID, Name: t.Name}}
	}
	for _, entry := range entries {
		cmd.Println(entry.Label())
		wts, mappings, repos, err := c.Worktrees(entry.ID)
		if err != nil {
			return err
		}
		for _, wt := range wts {
			mapping := mappings[wt.Path]
			state := (work.Manager{Store: c.Store}).Inspect(repos[mapping.RepoID], mapping)
			if state.Status != "ok" {
				cmd.Printf("  %s  %s  [%s] %s\n", wt.RepoName, wt.Path, state.Status, state.Detail)
				continue
			}
			cmd.Printf("  %s  %s  %s\n", wt.RepoName, state.Branch, wt.Path)
			if state.Changes == "" {
				cmd.Println("    clean")
			} else {
				cmd.Printf("    local changes:\n%s\n", state.Changes)
			}
		}
	}
	return nil
}
