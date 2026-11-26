package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tintaungkhant/tasky/internal/app"
	"github.com/tintaungkhant/tasky/internal/config"
	"github.com/tintaungkhant/tasky/internal/registry"
	"github.com/tintaungkhant/tasky/internal/store"
)

func init() { rootCmd.AddCommand(newRegistryCommands().Commands()...) }

// newRegistryCommands also gives tests fresh commands and flag state.
func newRegistryCommands() *cobra.Command {
	commands := &cobra.Command{Use: "tasky"}
	root := &cobra.Command{Use: "root", Short: "Manage registered Roots in SQLite"}
	repo := &cobra.Command{Use: "repo", Short: "Manage registered Repos in SQLite"}
	commands.AddCommand(root, repo)
	root.AddCommand(&cobra.Command{
		Use: "add <name> <path>", Short: "Register an existing Root directory", Args: cobra.ExactArgs(2),
		RunE: withRegistry(func(cmd *cobra.Command, args []string, r registry.Registry, cwd string) error {
			root, err := r.AddRoot(args[0], args[1])
			if err == nil {
				cmd.Printf("%s\t%s\t%s\n", root.ID, root.Name, root.Path)
			}
			return err
		}),
	}, &cobra.Command{
		Use: "list", Short: "List Root IDs, names, and paths", Args: cobra.NoArgs,
		RunE: withRegistry(func(cmd *cobra.Command, args []string, r registry.Registry, cwd string) error {
			roots, err := r.Store.ListRoots()
			if err != nil {
				return err
			}
			for _, root := range roots {
				cmd.Printf("%s\t%s\t%s\n", root.ID, root.Name, root.Path)
			}
			return nil
		}),
	}, &cobra.Command{
		Use: "remove <name-or-ID>", Short: "Forget a Root and all its mappings; preserve files and Git state", Args: cobra.ExactArgs(1),
		RunE: withRegistry(func(cmd *cobra.Command, args []string, r registry.Registry, cwd string) error {
			root, err := r.ResolveRoot(cwd, args[0])
			if err != nil {
				return err
			}
			if err := r.Store.RemoveRoot(root.ID); err != nil {
				return err
			}
			cmd.Printf("Forgot Root %s and its mappings. Files, worktrees, and branches are preserved.\n", root.Name)
			return nil
		}),
	})
	addRepo := &cobra.Command{
		Use: "add <name> <path>", Short: "Register a Git Repo without moving it", Args: cobra.ExactArgs(2),
		RunE: withRegistry(func(cmd *cobra.Command, args []string, r registry.Registry, cwd string) error {
			selector, _ := cmd.Flags().GetString("root")
			root, err := r.ResolveRoot(cwd, selector)
			if err != nil {
				return err
			}
			repo, err := r.AddRepo(root, args[0], args[1])
			if err == nil {
				cmd.Printf("%s\t%s\t%s\n", repo.ID, repo.Name, repo.Path)
			}
			return err
		}),
	}
	listRepos := &cobra.Command{
		Use: "list", Short: "List registered Repo IDs, names, and paths", Args: cobra.NoArgs,
		RunE: withRegistry(func(cmd *cobra.Command, args []string, r registry.Registry, cwd string) error {
			selector, _ := cmd.Flags().GetString("root")
			root, err := r.ResolveRoot(cwd, selector)
			if err != nil {
				return err
			}
			repos, err := r.Store.ListRepos(root.ID)
			if err != nil {
				return err
			}
			for _, repo := range repos {
				cmd.Printf("%s\t%s\t%s\n", repo.ID, repo.Name, repo.Path)
			}
			return nil
		}),
	}
	context := &cobra.Command{
		Use: "context", Short: "Resolve a registered Root and Repo from flags or the current directory", Args: cobra.NoArgs,
		RunE: withRegistry(func(cmd *cobra.Command, args []string, r registry.Registry, cwd string) error {
			root, _ := cmd.Flags().GetString("root")
			repo, _ := cmd.Flags().GetString("repo")
			selected, err := r.Resolve(cwd, root, repo)
			if err != nil {
				return err
			}
			cmd.Printf("Root\t%s\t%s\t%s\nRepo\t%s\t%s\t%s\n", selected.Root.ID, selected.Root.Name, selected.Root.Path, selected.Repo.ID, selected.Repo.Name, selected.Repo.Path)
			return nil
		}),
	}
	for _, cmd := range []*cobra.Command{addRepo, listRepos, context} {
		cmd.Flags().String("root", "", "Root name or ID (default: current-directory context)")
	}
	context.Flags().String("repo", "", "Repo name or ID (default: current Git repository)")
	repo.AddCommand(addRepo, listRepos)
	commands.AddCommand(context)
	return commands
}

func withRegistry(run func(*cobra.Command, []string, registry.Registry, string) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("current directory: %w", err)
		}
		path, err := config.Path()
		if err != nil {
			return err
		}
		s, err := store.Open(app.DatabasePath(path))
		if err != nil {
			return err
		}
		defer s.Close()
		return run(cmd, args, registry.Registry{Store: s}, cwd)
	}
}
