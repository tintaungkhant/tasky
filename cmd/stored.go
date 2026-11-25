package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/spf13/cobra"
	"github.com/tintaungkhant/tasky/internal/config"
	"github.com/tintaungkhant/tasky/internal/git"
	"github.com/tintaungkhant/tasky/internal/registry"
	"github.com/tintaungkhant/tasky/internal/store"
	"github.com/tintaungkhant/tasky/internal/task"
	"github.com/tintaungkhant/tasky/internal/work"
)

type storedContext struct {
	registry.Registry
	Root   store.Root
	Config config.Root
	Cwd    string
}

func init() {
	rename := &cobra.Command{Use: "rename <task-id> <name>", Short: "Rename a Task without changing its identity, branches, or paths", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		c, err := requiredStored(cmd)
		if err != nil {
			return err
		}
		defer c.Store.Close()
		t, err := c.Store.FindTask(c.Root.ID, args[0])
		if err != nil {
			return err
		}
		return c.Store.RenameTask(t.ID, args[1])
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
		cleanups, err := c.Store.ListCleanups(c.Root.ID)
		if err != nil {
			return err
		}
		for _, op := range cleanups {
			if op.ID != args[0] {
				continue
			}
			forget, _ := cmd.Flags().GetBool("forget")
			if forget {
				return c.Store.ForgetCleanup(op.ID)
			}
			repos, err := c.Store.ListRepos(c.Root.ID)
			if err != nil {
				return err
			}
			for _, repo := range repos {
				if repo.ID == op.RepoID {
					fw, _ := cmd.Flags().GetBool("force-worktree")
					fb, _ := cmd.Flags().GetBool("force-branch")
					if err := (work.Manager{Store: c.Store}).RecoverCleanup(repo, op, fw, fb); err != nil {
						return err
					}
					cmd.Printf("Cleanup %s completed.\n", op.ID)
					return nil
				}
			}
		}
		operations, err := c.Store.ListCreations(c.Root.ID)
		if err != nil {
			return err
		}
		for _, op := range operations {
			if op.ID != args[0] {
				continue
			}
			forget, _ := cmd.Flags().GetBool("forget")
			if forget {
				return c.Store.ForgetCreation(op.ID)
			}
			repos, err := c.Store.ListRepos(c.Root.ID)
			if err != nil {
				return err
			}
			for _, repo := range repos {
				if repo.ID == op.RepoID {
					return (work.Manager{Store: c.Store}).Recover(op, repo)
				}
			}
		}
		return fmt.Errorf("pending operation %q not found; use `tasky pending`", args[0])
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

// Unadopted YAML Roots retain the legacy workflow. A matching registered Root
// always uses persisted identity; errors never silently fall back to names.
func registeredContext(cmd *cobra.Command) (*storedContext, error) {
	selector, _ := cmd.Flags().GetString("root")
	path, err := config.Path()
	if err != nil {
		return nil, err
	}
	dbPath := filepath.Join(filepath.Dir(path), "tasky.sqlite3")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) && selector == "" {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	s, err := store.Open(dbPath)
	if err != nil {
		return nil, err
	}
	cwd, err := os.Getwd()
	if err != nil {
		s.Close()
		return nil, err
	}
	r := registry.Registry{Store: s}
	root, selectErr := r.ResolveRoot(cwd, selector)
	cfg, err := config.Load()
	if err != nil {
		s.Close()
		return nil, err
	}
	if selectErr != nil {
		if selector == "" {
			legacy, legacyErr := cfg.MatchRoot(cwd)
			if legacyErr == nil {
				roots, err := s.ListRoots()
				if err != nil {
					s.Close()
					return nil, err
				}
				registered := false
				for _, candidate := range roots {
					if sameLocation(candidate.Path, legacy.Path) {
						registered = true
					}
				}
				if !registered {
					s.Close()
					return nil, nil
				}
			}
		}
		s.Close()
		return nil, selectErr
	}
	settings := config.Root{Name: root.Name, Path: root.Path, TasksDir: filepath.Join(root.Path, "tasks"), BranchTemplate: "{branch}"}
	matches := 0
	for _, configured := range cfg.Roots {
		if sameLocation(configured.Path, root.Path) {
			settings = configured
			matches++
		}
	}
	if matches > 1 {
		s.Close()
		return nil, fmt.Errorf("multiple YAML settings match Root %q; review `tasky setup`", root.Name)
	}
	if settings.TasksDir == "" {
		s.Close()
		return nil, fmt.Errorf("Tasks Directory is empty; review `tasky setup`")
	}
	return &storedContext{Registry: r, Root: root, Config: settings, Cwd: cwd}, nil
}

func sameLocation(a, b string) bool {
	x, err := os.Stat(a)
	if err != nil {
		return false
	}
	y, err := os.Stat(b)
	return err == nil && os.SameFile(x, y)
}

func (c *storedContext) entries() ([]task.Entry, error) {
	tasks, err := c.Store.ListTasks(c.Root.ID)
	if err != nil {
		return nil, err
	}
	var entries []task.Entry
	for _, t := range tasks {
		entries = append(entries, task.Entry{ID: t.TaskID, Name: t.Name})
	}
	return entries, nil
}

func (c *storedContext) worktrees(taskID string) ([]task.WorktreeEntry, map[string]store.Worktree, map[string]store.Repo, error) {
	t, err := c.Store.FindTask(c.Root.ID, taskID)
	if err != nil {
		return nil, nil, nil, err
	}
	wts, err := c.Store.ListWorktrees(t.ID)
	if err != nil {
		return nil, nil, nil, err
	}
	repos, err := c.Store.ListRepos(c.Root.ID)
	if err != nil {
		return nil, nil, nil, err
	}
	byRepo, byPath := make(map[string]store.Repo), make(map[string]store.Worktree)
	for _, repo := range repos {
		byRepo[repo.ID] = repo
	}
	var entries []task.WorktreeEntry
	for _, wt := range wts {
		entries = append(entries, task.WorktreeEntry{RepoName: byRepo[wt.RepoID].Name, Path: wt.Path})
		byPath[wt.Path] = wt
	}
	return entries, byPath, byRepo, nil
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
	entries, err := c.entries()
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
	if !asIs && !slices.Contains(branches, branch) {
		branch = task.RenderBranch(c.Config.BranchTemplate, taskID, name, branch)
	}
	if err := git.ValidateBranch(ctx.Repo.Path, branch); err != nil {
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
	m := work.Manager{Store: c.Store}
	t, err := m.EnsureTask(c.Root.ID, taskID, name)
	if err != nil {
		return err
	}
	path, err := m.Create(t, ctx.Repo, c.Config.TasksDir, branch, base)
	if err != nil {
		return err
	}
	cmd.Printf("Created worktree at %s (branch %s)\n", path, branch)
	return nil
}

func selectedTask(cmd *cobra.Command, args []string, c *storedContext) (store.Task, error) {
	id := ""
	if len(args) > 0 {
		id = args[0]
	}
	if id == "" {
		entries, err := c.entries()
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
	t, err := c.Store.FindTask(c.Root.ID, id)
	if errors.Is(err, store.ErrNotFound) {
		return store.Task{}, fmt.Errorf("Task %q not found in Root %s", id, c.Root.Name)
	}
	return t, err
}

func runStoredList(cmd *cobra.Command, args []string, c *storedContext) error {
	entries, err := c.entries()
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
		wts, mappings, repos, err := c.worktrees(entry.ID)
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
