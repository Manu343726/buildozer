package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewStatusCommand creates the 'status' subcommand
func NewStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Query client status and capabilities",
		Long: `Query the running daemon for its status, including:
- Current load and resource usage
- Queue size
- Connected peer count
- Supported runtimes
- Cache statistics

Use --standalone to run without needing a separate daemon process (in-process mode).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			standalone, _ := IsStandaloneMode(cmd)

			if standalone {
				fmt.Println("[STANDALONE] Client Status (in-process daemon):")
				fmt.Println("===============================================")
				fmt.Println("Load: Will show in-process daemon metrics when implemented")
				fmt.Println("Queue Size: N/A (system not yet implemented)")
				fmt.Println("Connected Peers: N/A (no P2P in standalone mode)")
				fmt.Println("Supported Runtimes: N/A (runtime detection not yet integrated)")
				fmt.Println("Cache Stats: N/A (caching system not yet implemented)")
				return nil
			}

			// TODO: Connect to daemon at pkgconfig.Get().Daemon.Host:pkgconfig.Get().Daemon.Port
			// and call IntrospectionService.GetClientStatus()
			fmt.Println("Client Status:")
			fmt.Println("==============")
			fmt.Println("Load: N/A (daemon not yet implemented)")
			fmt.Println("Queue Size: N/A")
			fmt.Println("Connected Peers: N/A")
			fmt.Println("Supported Runtimes: N/A")
			fmt.Println("Cache Stats: N/A")
			return nil
		},
	}
}
