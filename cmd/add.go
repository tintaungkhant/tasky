package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/tintaungkhant/tasky/internal/config"
	"github.com/tintaungkhant/tasky/internal/git"
	"github.com/tintaungkhant/tasky/internal/task"
)

var addFromFlag string

var addCmd = &cobra.Command{
	Use:   "add [repo] <task-id> <branch>",
	Short: "Create a task-scoped Git worktree",
	Args:  cobra.RangeArgs(2, 3),
	RunE:  runAdd,
}

func init() {
	addCmd.Flags().StringVar(&addFromFlag, "from", "", "base branch to create the new branch from (default: current branch)")
	rootCmd.AddCommand(addCmd)
}

func runAdd(cmd *cobra.Command, args []string) error {
	var repoArg, taskID, branch string
	if len(args) == 3 {
		repoArg, taskID, branch = args[0], args[1], args[2]
	} else {
		taskID, branch = args[0], args[1]
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	repoRoot, err := resolveRepo(cfg, cwd, repoArg)
	if err != nil {
		return err
	}

	root, err := cfg.MatchRoot(repoRoot)
	if err != nil {
		return err
	}

	base := addFromFlag
	if base == "" {
		base, err = git.CurrentBranch(repoRoot)
		if err != nil {
			return err
		}
	}

	repoName := filepath.Base(repoRoot)
	renderedBranch := task.RenderBranch(root.BranchTemplate, taskID, branch)
	worktreePath := task.WorktreePath(root.TasksDir, taskID, repoName)

	if _, err := os.Stat(worktreePath); err == nil {
		return fmt.Errorf("worktree path already exists: %s", worktreePath)
	}

	branches, err := git.ListLocalBranches(repoRoot)
	if err != nil {
		return err
	}

	if contains(branches, renderedBranch) {
		if err := git.WorktreeAddExistingBranch(repoRoot, worktreePath, renderedBranch); err != nil {
			return err
		}
		fmt.Printf("Created worktree at %s (existing branch %s)\n", worktreePath, renderedBranch)
		return nil
	}

	if err := git.WorktreeAddNewBranch(repoRoot, worktreePath, renderedBranch, base); err != nil {
		return err
	}
	fmt.Printf("Created worktree at %s (branch %s, from %s)\n", worktreePath, renderedBranch, base)
	return nil
}

func contains(list []string, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}

// resolveRepo determines the repo's root directory. When cwd is inside a
// Git repo, that repo is used (unless repoArg overrides it). Otherwise cwd
// is treated as a Root directory: repoArg selects the repo explicitly, or
// (if empty) the developer is prompted to pick one of the Root's repos.
func resolveRepo(cfg *config.Config, cwd, repoArg string) (string, error) {
	repoRoot, insideRepo := "", true
	toplevel, err := git.Toplevel(cwd)
	if err != nil {
		insideRepo = false
	} else {
		repoRoot = toplevel
	}

	rootDir := repoRoot
	if !insideRepo {
		rootDir = cwd
	}
	root, err := cfg.MatchRoot(rootDir)
	if err != nil {
		return "", err
	}

	if repoArg != "" {
		entries, err := os.ReadDir(root.Path)
		if err != nil {
			return "", fmt.Errorf("read root directory: %w", err)
		}
		found := false
		for _, e := range entries {
			if e.Name() == repoArg {
				found = true
				break
			}
		}
		candidate := filepath.Join(root.Path, repoArg)
		if !found || !git.IsRepo(candidate) {
			return "", fmt.Errorf("repo %q not found under root %s (expected a directory containing .git)", repoArg, root.Path)
		}
		return candidate, nil
	}

	if insideRepo {
		return repoRoot, nil
	}

	repos, err := listRepos(root.Path)
	if err != nil {
		return "", err
	}
	if len(repos) == 0 {
		return "", fmt.Errorf("no git repositories found under root %s", root.Path)
	}

	var selected string
	options := make([]huh.Option[string], 0, len(repos))
	for _, r := range repos {
		options = append(options, huh.NewOption(r, r))
	}
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().Title("Which repo?").Options(options...).Value(&selected),
		),
	)
	if err := form.Run(); err != nil {
		return "", err
	}
	return filepath.Join(root.Path, selected), nil
}

// listRepos returns the names of rootPath's direct subdirectories that are Git repos.
func listRepos(rootPath string) ([]string, error) {
	entries, err := os.ReadDir(rootPath)
	if err != nil {
		return nil, fmt.Errorf("read root directory: %w", err)
	}
	var repos []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if git.IsRepo(filepath.Join(rootPath, e.Name())) {
			repos = append(repos, e.Name())
		}
	}
	sort.Strings(repos)
	return repos, nil
}
