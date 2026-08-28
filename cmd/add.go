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
	Args:  addArgsValidator,
	RunE:  runAdd,
}

// addArgsValidator accepts 0 args (fully interactive), or 2/3 args
// (task-id + branch, optionally preceded by an explicit repo).
func addArgsValidator(cmd *cobra.Command, args []string) error {
	switch len(args) {
	case 0, 2, 3:
		return nil
	default:
		return fmt.Errorf("accepts 0, 2, or 3 arg(s), received %d", len(args))
	}
}

func init() {
	addCmd.Flags().StringVar(&addFromFlag, "from", "", "base branch to create the new branch from (default: current branch)")
	rootCmd.AddCommand(addCmd)
}

func runAdd(cmd *cobra.Command, args []string) error {
	interactive := len(args) == 0

	var repoArg, taskID, branch string
	if len(args) == 3 {
		repoArg, taskID, branch = args[0], args[1], args[2]
	} else if len(args) == 2 {
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

	repoRoot, root, err := resolveRepo(cfg, cwd, repoArg)
	if err != nil {
		return err
	}

	branches, err := git.ListLocalBranches(repoRoot)
	if err != nil {
		return err
	}

	checkedOut, err := git.CheckedOutBranches(repoRoot)
	if err != nil {
		return err
	}
	reusableBranches := make([]string, 0, len(branches))
	for _, b := range branches {
		if !contains(checkedOut, b) {
			reusableBranches = append(reusableBranches, b)
		}
	}

	if taskID == "" {
		ids, err := listTaskIDs(root.TasksDir)
		if err != nil {
			return err
		}
		if taskID, _, err = promptSelectOrNew("Task ID", "Task ID", ids); err != nil {
			return err
		}
	}

	base := addFromFlag
	if base == "" {
		currentBranch, err := git.CurrentBranch(repoRoot)
		if err != nil {
			return err
		}
		if interactive {
			if base, err = promptSelectBase(branches, currentBranch); err != nil {
				return err
			}
		} else {
			base = currentBranch
		}
	}

	if branch == "" {
		if branch, _, err = promptSelectOrNew("Branch", "Branch name", reusableBranches); err != nil {
			return err
		}
	}

	if err := os.MkdirAll(root.TasksDir, 0o755); err != nil {
		return fmt.Errorf("create tasks directory %s: %w", root.TasksDir, err)
	}

	repoName := filepath.Base(repoRoot)
	// A branch that already exists verbatim (typed, selected, or passed as an
	// arg) is reused as-is; only a genuinely new branch name gets the template.
	renderedBranch := branch
	if !contains(branches, branch) {
		renderedBranch = task.RenderBranch(root.BranchTemplate, taskID, branch)
	}
	worktreePath := task.WorktreePath(root.TasksDir, taskID, repoName, renderedBranch)

	if _, err := os.Stat(worktreePath); err == nil {
		return fmt.Errorf("worktree path already exists: %s", worktreePath)
	}

	reuseExisting := contains(branches, renderedBranch)

	if interactive {
		confirmed, err := confirmCreate(taskID, repoName, base, renderedBranch, worktreePath, reuseExisting)
		if err != nil {
			return err
		}
		if !confirmed {
			return fmt.Errorf("aborted")
		}
	}

	if reuseExisting {
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

func promptText(title string) (string, error) {
	var value string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title(title).Value(&value).Validate(requireNonEmpty),
		),
	)
	if err := form.Run(); err != nil {
		return "", err
	}
	return value, nil
}

const newOptionValue = "\x00new"

// promptSelectOrNew offers existing options plus a "+ New" entry that falls
// through to a free-text prompt. isNew reports whether the returned value
// came from that free-text prompt rather than being picked from options.
func promptSelectOrNew(selectTitle, inputTitle string, options []string) (value string, isNew bool, err error) {
	if len(options) == 0 {
		v, err := promptText(inputTitle)
		return v, true, err
	}

	huhOptions := make([]huh.Option[string], 0, len(options)+1)
	for _, o := range options {
		huhOptions = append(huhOptions, huh.NewOption(o, o))
	}
	huhOptions = append(huhOptions, huh.NewOption("+ New", newOptionValue))

	var selected string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().Title(selectTitle).Options(huhOptions...).Value(&selected),
		),
	)
	if err := form.Run(); err != nil {
		return "", false, err
	}
	if selected != newOptionValue {
		return selected, false, nil
	}
	v, err := promptText(inputTitle)
	return v, true, err
}

// listTaskIDs returns the names of tasksDir's direct subdirectories, i.e.
// task IDs already in use by any repo under the same Root.
func listTaskIDs(tasksDir string) ([]string, error) {
	entries, err := os.ReadDir(tasksDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read tasks directory %s: %w", tasksDir, err)
	}
	var ids []string
	for _, e := range entries {
		if e.IsDir() {
			ids = append(ids, e.Name())
		}
	}
	sort.Strings(ids)
	return ids, nil
}

func promptSelectBase(branches []string, current string) (string, error) {
	options := make([]huh.Option[string], 0, len(branches))
	for _, b := range branches {
		options = append(options, huh.NewOption(b, b))
	}
	base := current
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().Title("Base branch").Options(options...).Value(&base),
		),
	)
	if err := form.Run(); err != nil {
		return "", err
	}
	return base, nil
}

func confirmCreate(taskID, repoName, base, branch, worktreePath string, reuseExisting bool) (bool, error) {
	fmt.Println("Task:      ", taskID)
	fmt.Println("Repository:", repoName)
	if reuseExisting {
		fmt.Println("Branch:    ", branch, "(existing)")
	} else {
		fmt.Println("Base:      ", base)
		fmt.Println("Branch:    ", branch)
	}
	fmt.Println("Worktree:  ", worktreePath)

	confirmed := true
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().Title("Create worktree?").Value(&confirmed),
		),
	)
	if err := form.Run(); err != nil {
		return false, err
	}
	return confirmed, nil
}

func contains(list []string, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}

// resolveRepo determines the repo's root directory and its configured Root.
// When cwd is inside a Git repo, that repo is used (unless repoArg overrides
// it). Otherwise cwd is treated as a Root directory: repoArg selects the
// repo explicitly, or (if empty) the developer is prompted to pick one of
// the Root's repos.
func resolveRepo(cfg *config.Config, cwd, repoArg string) (string, *config.Root, error) {
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
		return "", nil, err
	}
	if _, err := os.Stat(root.Path); err != nil {
		return "", nil, fmt.Errorf("root %q is misconfigured: path %s not found on disk (check `tasky setup`)", root.Name, root.Path)
	}

	if repoArg != "" {
		entries, err := os.ReadDir(root.Path)
		if err != nil {
			return "", nil, fmt.Errorf("read root directory: %w", err)
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
			return "", nil, fmt.Errorf("repo %q not found under root %s (expected a directory containing .git)", repoArg, root.Path)
		}
		return candidate, root, nil
	}

	if insideRepo {
		return repoRoot, root, nil
	}

	repos, err := listRepos(root.Path)
	if err != nil {
		return "", nil, err
	}
	if len(repos) == 0 {
		return "", nil, fmt.Errorf("no git repositories found under root %s", root.Path)
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
		return "", nil, err
	}
	return filepath.Join(root.Path, selected), root, nil
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
