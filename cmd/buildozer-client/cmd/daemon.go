package cmd

import (
	"github.com/Manu343726/buildozer/pkg/cli"
	pkgconfig "github.com/Manu343726/buildozer/pkg/config"
	"github.com/spf13/cobra"
)

// NewDaemonCommand creates the 'daemon' subcommand
func NewDaemonCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Launch the buildozer client daemon",
		Long: `Launch the buildozer client daemon.

The daemon accepts job submissions via gRPC, coordinates with peer nodes via mDNS,
and manages job scheduling and execution.

The daemon listens on the configured host:port (see 'config' command to view current settings).

All services are exposed through a unified Connect/gRPC server:
  - Logging service for runtime log manipulation
  - (Future) Job submission and monitoring
  - (Future) Cache management
  - (Future) Peer discovery

NOTE: The 'daemon' subcommand cannot be used with --standalone flag. Use --standalone
with other commands (status, peers, logs, etc.) to run without a separate daemon process.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Logging is already initialized by root command with daemon config
			daemonCommands, err := cli.NewDaemonCommands(pkgconfig.Get())
			if err != nil {
				return err
			}
			return daemonCommands.Start()
		},
	}

	return cmd
}
