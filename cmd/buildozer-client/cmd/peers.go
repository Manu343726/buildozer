package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewPeersCommand creates the 'peers' subcommand
func NewPeersCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "peers",
		Short: "List connected peer nodes",
		Long: `List all peer nodes discovered via mDNS and their capabilities:
- Peer address (host:port)
- Number of concurrent jobs available
- Supported runtimes
- Cache size and hit rate
- Last seen timestamp

Use --standalone to run without needing a separate daemon process (in-process mode).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			standalone, _ := IsStandaloneMode(cmd)

			if standalone {
				fmt.Println("[STANDALONE] Connected Peers:")
				fmt.Println("==============================")
				fmt.Println("(No P2P in standalone mode - running in-process daemon)")
				return nil
			}

			// TODO: Connect to daemon and call IntrospectionService.ListPeers()
			fmt.Println("Connected Peers:")
			fmt.Println("================")
			fmt.Println("No peers discovered (daemon not yet implemented)")
			return nil
		},
	}
}
