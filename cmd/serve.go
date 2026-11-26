package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/tintaungkhant/tasky/internal/config"
	"github.com/tintaungkhant/tasky/internal/web"
)

func init() { rootCmd.AddCommand(newServeCommand()) }

func newServeCommand() *cobra.Command {
	var port int
	command := &cobra.Command{
		Use: "serve", Short: "Run Tasky's protected local web service", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := config.Path()
			if err != nil {
				return err
			}
			path, err = filepath.Abs(path)
			if err != nil {
				return err
			}
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			service, err := web.New(path, cwd, port)
			if err != nil {
				return err
			}
			defer service.Close()
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Dashboard: %s\nPress Ctrl+C to stop.\n", service.URL()); err != nil {
				return err
			}
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return service.Serve(ctx)
		},
	}
	command.Flags().IntVar(&port, "port", 9000, "local port (0 selects an available port)")
	return command
}
