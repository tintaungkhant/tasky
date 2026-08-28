package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/tintaungkhant/tasky/internal/config"
	"github.com/tintaungkhant/tasky/internal/ui"
)

const (
	actionAdd    = "add"
	actionEdit   = "edit"
	actionRemove = "remove"
	actionDone   = "done"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Configure tasky's Roots",
	RunE:  runSetup,
}

func init() {
	rootCmd.AddCommand(setupCmd)
}

func runSetup(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	for {
		printRoots(cfg)

		action, err := promptAction(cfg)
		if err != nil {
			return err
		}

		switch action {
		case actionAdd:
			if err := addRoot(cfg); err != nil {
				return err
			}
		case actionEdit:
			if err := editRoot(cfg); err != nil {
				return err
			}
		case actionRemove:
			if err := removeRoot(cfg); err != nil {
				return err
			}
		case actionDone:
			if err := cfg.Save(); err != nil {
				return err
			}
			fmt.Println(ui.Success("Config saved."))
			return nil
		}
	}
}

func printRoots(cfg *config.Config) {
	if len(cfg.Roots) == 0 {
		fmt.Println(ui.Info("No roots configured yet."))
		return
	}
	fmt.Println(ui.Label("Configured roots:"))
	for _, r := range cfg.Roots {
		fmt.Printf("  - %s: %s (tasks_dir: %s, branch_template: %s)\n", r.Name, r.Path, r.TasksDir, r.BranchTemplate)
	}
}

func promptAction(cfg *config.Config) (string, error) {
	options := []huh.Option[string]{
		huh.NewOption("Add a root", actionAdd),
	}
	if len(cfg.Roots) > 0 {
		options = append(options,
			huh.NewOption("Edit a root", actionEdit),
			huh.NewOption("Remove a root", actionRemove),
		)
	}
	options = append(options, huh.NewOption("Done", actionDone))

	var action string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("What would you like to do?").
				Options(options...).
				Value(&action),
		),
	)
	if err := form.Run(); err != nil {
		return "", err
	}
	return action, nil
}

func addRoot(cfg *config.Config) error {
	var r config.Root
	nameAndPath := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Root name").Value(&r.Name).Validate(requireNonEmpty),
			huh.NewInput().Title("Root path (absolute)").Value(&r.Path).Validate(requireNonEmpty),
		),
	)
	if err := nameAndPath.Run(); err != nil {
		return err
	}

	r.TasksDir = filepath.Join(r.Path, "tasks")
	r.BranchTemplate = "{branch}"
	rest := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Tasks directory (absolute)").Value(&r.TasksDir).Validate(requireNonEmpty),
			huh.NewInput().Title("Branch template").
				Description("Use {task-id} and {branch} placeholders, e.g. task/{task-id}/{branch}").
				Value(&r.BranchTemplate).Validate(requireNonEmpty),
		),
	)
	if err := rest.Run(); err != nil {
		return err
	}

	cfg.AddRoot(r)
	return nil
}

func editRoot(cfg *config.Config) error {
	name, err := selectRootName(cfg, "Which root do you want to edit?")
	if err != nil {
		return err
	}
	r, ok := cfg.FindRoot(name)
	if !ok {
		return fmt.Errorf("root %q not found", name)
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Root name").Value(&r.Name).Validate(requireNonEmpty),
			huh.NewInput().Title("Root path (absolute)").Value(&r.Path).Validate(requireNonEmpty),
			huh.NewInput().Title("Tasks directory (absolute)").Value(&r.TasksDir).Validate(requireNonEmpty),
			huh.NewInput().Title("Branch template").
				Description("Use {task-id} and {branch} placeholders, e.g. task/{task-id}/{branch}").
				Value(&r.BranchTemplate).Validate(requireNonEmpty),
		),
	)
	return form.Run()
}

func removeRoot(cfg *config.Config) error {
	name, err := selectRootName(cfg, "Which root do you want to remove?")
	if err != nil {
		return err
	}
	cfg.RemoveRoot(name)
	return nil
}

func selectRootName(cfg *config.Config, title string) (string, error) {
	options := make([]huh.Option[string], 0, len(cfg.Roots))
	for _, r := range cfg.Roots {
		options = append(options, huh.NewOption(r.Name, r.Name))
	}
	var name string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().Title(title).Options(options...).Value(&name),
		),
	)
	if err := form.Run(); err != nil {
		return "", err
	}
	return name, nil
}

func requireNonEmpty(s string) error {
	if s == "" {
		return fmt.Errorf("required")
	}
	return nil
}
