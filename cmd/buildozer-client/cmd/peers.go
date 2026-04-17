package cmd

import (
	"github.com/Manu343726/buildozer/pkg/cli"
	pkgconfig "github.com/Manu343726/buildozer/pkg/config"
	"github.com/spf13/cobra"
)

// NewPeersCommand creates the 'peers' subcommand with list and discover subcommands
func NewPeersCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "peers",
		Short: "Query and manage peer discovery",
		Long: `Query and manage peer nodes discovered via mDNS.

Subcommands:
- peers list:    List all discovered peer nodes in the network
- peers discover: Trigger an immediate discovery cycle`,
	}

	cmd.AddCommand(newPeersListCommand())
	cmd.AddCommand(newPeersDiscoverCommand())

	return cmd
}

// newPeersListCommand lists all discovered peers
func newPeersListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all discovered peers",
		Long: `List all peer nodes discovered via mDNS.

Shows peer address, connection status, and available runtimes.

Use --standalone to run without needing a separate daemon process (in-process mode).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			commands, err := cli.NewPeersCommands(pkgconfig.Get())
			if err != nil {
				return err
			}
			return commands.List()
		},
	}
}

// newPeersDiscoverCommand triggers peer discovery
func newPeersDiscoverCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "discover",
		Short: "Trigger immediate peer discovery",
		Long: `Trigger an immediate peer discovery cycle.

Normally, peer discovery happens automatically at regular intervals.
Use this command to force an immediate discovery cycle.

Use --standalone to run without needing a separate daemon process (in-process mode).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			commands, err := cli.NewPeersCommands(pkgconfig.Get())
			if err != nil {
				return err
			}
			return commands.Discover()
		},
	}
}
