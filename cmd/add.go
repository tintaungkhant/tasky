package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/tintaungkhant/tasky/internal/config"
	"github.com/tintaungkhant/tasky/internal/git"
	"github.com/tintaungkhant/tasky/internal/task"
	"github.com/tintaungkhant/tasky/internal/ui"
)

var addFromFlag string
var addNameFlag string

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
	addCmd.Flags().StringVar(&addNameFlag, "name", "", "optional human-readable task name (ignored if the task ID already exists)")
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

	checkedOut, err := git.CheckedOutBranchPaths(repoRoot)
	if err != nil {
		return err
	}

	taskEntries, err := listTaskEntries(root.TasksDir)
	if err != nil {
		return err
	}

	if taskID == "" {
		if taskID, _, err = promptSelectOrNewTaskID(taskEntries); err != nil {
			return err
		}
	}

	var taskName string
	if existing, found := findTaskEntry(taskEntries, taskID); found {
		taskName = existing.Name
	} else {
		taskName = addNameFlag
		if taskName == "" && interactive {
			if taskName, err = promptOptionalText("Task name (optional)"); err != nil {
				return err
			}
		}
	}
	taskDirName := task.TaskDirName(taskID, taskName)

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

	var branchAlreadyRendered bool
	if branch == "" {
		if branch, branchAlreadyRendered, err = promptBranchStep(taskID, taskName, base, root.BranchTemplate); err != nil {
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
	if !branchAlreadyRendered && !contains(branches, branch) {
		renderedBranch = task.RenderBranch(root.BranchTemplate, taskID, taskName, branch)
	}
	worktreePath := task.WorktreePath(root.TasksDir, taskDirName, repoName, renderedBranch)

	if _, err := os.Stat(worktreePath); err == nil {
		return fmt.Errorf("worktree path already exists: %s", worktreePath)
	}

	reuseExisting := contains(branches, renderedBranch)

	if busyPath, busy := checkedOut[renderedBranch]; reuseExisting && busy {
		if busyPath == repoRoot {
			return fmt.Errorf("branch %q is checked out in %s's working directory — switch it to another branch first, then retry", renderedBranch, repoName)
		}
		return fmt.Errorf("branch %q is already checked out at %s — remove that worktree or pick a different branch", renderedBranch, busyPath)
	}

	if interactive {
		confirmed, err := confirmCreate(taskID, taskName, repoName, base, renderedBranch, worktreePath, reuseExisting)
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
		fmt.Println(ui.Success("Created worktree at %s (existing branch %s)", worktreePath, renderedBranch))
		return nil
	}

	if err := git.WorktreeAddNewBranch(repoRoot, worktreePath, renderedBranch, base); err != nil {
		return err
	}
	fmt.Println(ui.Success("Created worktree at %s (branch %s, from %s)", worktreePath, renderedBranch, base))
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

// promptBranchStep resolves the worktree's branch interactively. The
// developer types a branch name, pre-filled with base (leaving it unchanged
// or clearing it means "use base itself" — e.g. to reuse an existing branch
// picked as base). Whatever name results (base or typed) is then offered
// as-is or with the configured template prefix applied, unless the template
// makes no difference. alreadyRendered reports whether the returned branch
// is already final and must not be passed through task.RenderBranch again.
func promptBranchStep(taskID, taskName, base, template string) (branch string, alreadyRendered bool, err error) {
	var typed string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Branch name").Placeholder(base).Value(&typed),
		),
	)
	if err := form.Run(); err != nil {
		return "", false, err
	}
	typed = strings.TrimSpace(typed)

	candidate := typed
	if candidate == "" {
		candidate = base
	}

	rendered := task.RenderBranch(template, taskID, taskName, candidate)
	if rendered == candidate {
		return candidate, true, nil
	}

	options := []huh.Option[string]{
		huh.NewOption(candidate+" (as-is)", candidate),
		huh.NewOption(rendered+" (with prefix)", rendered),
	}
	// Reusing base as-is (blank input) defaults to as-is; typing a distinct
	// new name defaults to the prefixed form, since that's the template's point.
	selected := candidate
	if typed != "" {
		selected = rendered
	}
	pickForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().Title("Use branch as").Options(options...).Value(&selected),
		),
	)
	if err := pickForm.Run(); err != nil {
		return "", false, err
	}
	return selected, true, nil
}

// taskEntry is one task directory under a Root's tasksDir, split back into
// its ID and optional human-readable name (see task.ParseTaskDirName).
type taskEntry struct {
	ID   string
	Name string
}

// listTaskEntries returns every task directory under tasksDir, parsed into
// (ID, Name) pairs. Task IDs already in use by any repo under the same Root.
func listTaskEntries(tasksDir string) ([]taskEntry, error) {
	entries, err := os.ReadDir(tasksDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read tasks directory %s: %w", tasksDir, err)
	}
	var out []taskEntry
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		id, name := task.ParseTaskDirName(e.Name())
		out = append(out, taskEntry{ID: id, Name: name})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// findTaskEntry looks up a task by ID among entries (see listTaskEntries).
func findTaskEntry(entries []taskEntry, id string) (taskEntry, bool) {
	for _, e := range entries {
		if e.ID == id {
			return e, true
		}
	}
	return taskEntry{}, false
}

// promptSelectOrNewTaskID offers existing task IDs (labeled with their name,
// if any) plus a "+ New" entry that falls through to a free-text prompt.
func promptSelectOrNewTaskID(entries []taskEntry) (value string, isNew bool, err error) {
	if len(entries) == 0 {
		v, err := promptText("Task ID")
		return v, true, err
	}

	huhOptions := make([]huh.Option[string], 0, len(entries)+1)
	for _, e := range entries {
		label := e.ID
		if e.Name != "" {
			label = fmt.Sprintf("%s (%s)", e.ID, e.Name)
		}
		huhOptions = append(huhOptions, huh.NewOption(label, e.ID))
	}
	huhOptions = append(huhOptions, huh.NewOption("+ New", newOptionValue))

	var selected string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().Title("Task ID").Options(huhOptions...).Value(&selected),
		),
	)
	if err := form.Run(); err != nil {
		return "", false, err
	}
	if selected != newOptionValue {
		return selected, false, nil
	}
	v, err := promptText("Task ID")
	return v, true, err
}

// promptOptionalText prompts for free text with no non-empty validation,
// trimming whitespace. An empty result is a valid answer ("skip").
func promptOptionalText(title string) (string, error) {
	var value string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title(title).Value(&value),
		),
	)
	if err := form.Run(); err != nil {
		return "", err
	}
	return strings.TrimSpace(value), nil
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

func confirmCreate(taskID, taskName, repoName, base, branch, worktreePath string, reuseExisting bool) (bool, error) {
	fmt.Println(ui.Label("Task:      "), taskID)
	if taskName != "" {
		fmt.Println(ui.Label("Name:      "), taskName)
	}
	fmt.Println(ui.Label("Repository:"), repoName)
	if reuseExisting {
		fmt.Println(ui.Label("Branch:    "), branch, ui.Info("(existing)"))
	} else {
		fmt.Println(ui.Label("Base:      "), base)
		fmt.Println(ui.Label("Branch:    "), branch)
	}
	fmt.Println(ui.Label("Worktree:  "), worktreePath)

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
