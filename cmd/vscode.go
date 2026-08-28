package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/tintaungkhant/tasky/internal/config"
	"github.com/tintaungkhant/tasky/internal/ui"
)

var vscodeCmd = &cobra.Command{
	Use:   "vscode [task-id]",
	Short: "Generate a multi-root workspace file for a task's worktrees",
	Long: "Generate a .code-workspace file listing every repo worktree under a task, " +
		"for editors that share VS Code's git extension (VS Code, Cursor, Windsurf, " +
		"VSCodium, ...). Those editors only auto-detect one git root per opened " +
		"folder; a multi-root workspace is how they see every repo's git state at once.",
	Args: cobra.MaximumNArgs(1),
	RunE: runVSCode,
}

func init() {
	rootCmd.AddCommand(vscodeCmd)
}

type vscodeWorkspace struct {
	Folders []vscodeFolder `json:"folders"`
}

type vscodeFolder struct {
	Path string `json:"path"`
}

func runVSCode(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	root, err := cfg.MatchRoot(cwd)
	if err != nil {
		return err
	}

	taskID := ""
	if len(args) == 1 {
		taskID = args[0]
	}
	if taskID == "" {
		ids, err := listTaskIDs(root.TasksDir)
		if err != nil {
			return err
		}
		if len(ids) == 0 {
			return fmt.Errorf("no tasks found under %s", root.TasksDir)
		}
		if taskID, err = promptSelectTaskID(ids); err != nil {
			return err
		}
	}

	taskDir := filepath.Join(root.TasksDir, taskID)
	entries, err := os.ReadDir(taskDir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("task %q not found under %s", taskID, root.TasksDir)
		}
		return fmt.Errorf("read task directory %s: %w", taskDir, err)
	}

	var folders []string
	for _, e := range entries {
		if e.IsDir() {
			folders = append(folders, e.Name())
		}
	}
	if len(folders) == 0 {
		return fmt.Errorf("no worktrees found under %s", taskDir)
	}
	sort.Strings(folders)

	ws := vscodeWorkspace{Folders: make([]vscodeFolder, 0, len(folders))}
	for _, f := range folders {
		ws.Folders = append(ws.Folders, vscodeFolder{Path: "./" + f})
	}

	data, err := json.MarshalIndent(ws, "", "  ")
	if err != nil {
		return fmt.Errorf("build workspace file: %w", err)
	}

	workspacePath := filepath.Join(taskDir, taskID+".code-workspace")
	if err := os.WriteFile(workspacePath, data, 0o644); err != nil {
		return fmt.Errorf("write workspace file %s: %w", workspacePath, err)
	}

	fmt.Println(ui.Success("Wrote workspace file at %s", workspacePath))
	return nil
}

func promptSelectTaskID(ids []string) (string, error) {
	options := make([]huh.Option[string], 0, len(ids))
	for _, id := range ids {
		options = append(options, huh.NewOption(id, id))
	}
	var taskID string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().Title("Task ID").Options(options...).Value(&taskID),
		),
	)
	if err := form.Run(); err != nil {
		return "", err
	}
	return taskID, nil
}
