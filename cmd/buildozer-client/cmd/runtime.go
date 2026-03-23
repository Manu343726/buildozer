package cmd

import (
	"context"
	"time"

	"github.com/spf13/cobra"

	"github.com/Manu343726/buildozer/pkg/cli"
	pkgconfig "github.com/Manu343726/buildozer/pkg/config"
	"github.com/Manu343726/buildozer/pkg/daemon"
)

// NewRuntimeCommand creates the 'runtime' parent command with subcommands
func NewRuntimeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "runtime",
		Short: "Query available runtimes",
		Long: `Query runtimes available on the daemon and network.

The daemon probes the system to discover all available compilation environments
(C/C++, Go, Rust, etc.) and can query connected peers for their runtimes.`,
	}

	// Add subcommands
	cmd.AddCommand(newRuntimeListCommand())
	cmd.AddCommand(newRuntimeInfoCommand())

	return cmd
}

// newRuntimeListCommand lists available runtimes (local or network-wide)
func newRuntimeListCommand() *cobra.Command {
	var localOnly bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available runtimes",
		Long: `Display all available runtimes from the daemon and network.

By default, lists runtimes on this daemon and queries connected peers for their runtimes.
Use --local to list only runtimes available on this daemon without querying peers.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := pkgconfig.Get()

			// Start in-process daemon if in standalone mode
			var d *daemon.Daemon
			if cfg.Standalone {
				var err error
				d, err = daemon.NewDaemon(cfg.Daemon)
				if err != nil {
					return err
				}
				if err := d.Start(); err != nil {
					return err
				}
				defer d.Stop(context.Background())

				// Give daemon time to start and register handlers
				time.Sleep(100 * time.Millisecond)
			}

			commands, err := cli.NewRuntimeCommands(cfg)
			if err != nil {
				return err
			}
			return commands.List(localOnly)
		},
	}

	cmd.Flags().BoolVar(&localOnly, "local", false, "List only local daemon runtimes (don't query peers)")

	return cmd
}

// newRuntimeInfoCommand shows detailed runtime information
func newRuntimeInfoCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "info <runtime-id>",
		Short: "Show detailed information about a specific runtime",
		Long:  `Display detailed information about a specific runtime by its ID.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := pkgconfig.Get()

			// Start in-process daemon if in standalone mode
			var d *daemon.Daemon
			if cfg.Standalone {
				var err error
				d, err = daemon.NewDaemon(cfg.Daemon)
				if err != nil {
					return err
				}
				if err := d.Start(); err != nil {
					return err
				}
				defer d.Stop(context.Background())

				// Give daemon time to start and register handlers
				time.Sleep(100 * time.Millisecond)
			}

			commands, err := cli.NewRuntimeCommands(cfg)
			if err != nil {
				return err
			}
			return commands.Info(args[0])
		},
	}
}
