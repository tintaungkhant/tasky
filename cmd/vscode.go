package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
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

// taskIDOptions builds a Task ID select option per entry, labeled with its
// name when set.
func taskIDOptions(entries []task.Entry) []huh.Option[string] {
	options := make([]huh.Option[string], 0, len(entries))
	for _, e := range entries {
		options = append(options, huh.NewOption(e.Label(), e.ID))
	}
	return options
}

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
	if c, err := registeredContext(cmd); err != nil {
		return err
	} else if c != nil {
		defer c.Store.Close()
		return runStoredVSCode(cmd, args, c)
	}
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

	taskEntries, err := task.ListEntries(root.TasksDir)
	if err != nil {
		return err
	}

	taskID := ""
	if len(args) == 1 {
		taskID = args[0]
	}
	if taskID == "" {
		if len(taskEntries) == 0 {
			return fmt.Errorf("no tasks found under %s", root.TasksDir)
		}
		if taskID, err = selectOne("Task ID", taskIDOptions(taskEntries)); err != nil {
			return err
		}
	}

	entry, found := task.FindEntry(taskEntries, taskID)
	if !found {
		return fmt.Errorf("task %q not found under %s", taskID, root.TasksDir)
	}

	taskDir := filepath.Join(root.TasksDir, task.TaskDirName(entry.ID, entry.Name))
	entries, err := os.ReadDir(taskDir)
	if err != nil {
		return fmt.Errorf("read task directory %s: %w", taskDir, err)
	}

	var folders []string
	for _, e := range entries {
		if e.IsDir() {
			path := filepath.Join(taskDir, e.Name())
			if _, _, err := git.VerifyWorktree(path); err != nil {
				cmd.Printf("Skipped %s: %v\n", path, err)
				continue
			}
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

	workspacePath := filepath.Join(taskDir, task.TaskDirName(entry.ID, entry.Name)+".code-workspace")
	return writeWorkspace(cmd, workspacePath, ws)
}

func writeWorkspace(cmd *cobra.Command, path string, ws vscodeWorkspace) error {
	data, err := json.MarshalIndent(ws, "", "  ")
	if err != nil {
		return fmt.Errorf("build workspace file: %w", err)
	}

	if err := publishNewFile(path, data); err != nil {
		if errors.Is(err, os.ErrExist) {
			previous, readErr := os.ReadFile(path)
			if readErr == nil && bytes.Equal(previous, data) {
				cmd.Printf("Workspace file is current: %s\n", path)
				return nil
			}
			return fmt.Errorf("workspace file %s exists; preserve or move it before regenerating", path)
		}
		return fmt.Errorf("write workspace file %s: %w", path, err)
	}
	cmd.Printf("Wrote workspace file at %s\n", path)
	return nil
}
