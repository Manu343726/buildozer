package cmd

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"connectrpc.com/connect"
	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	"github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1/protov1connect"
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
			standalone, _ := IsStandaloneMode(cmd)

			if standalone {
				fmt.Println("Peers (Standalone Mode):")
				fmt.Println("========================")
				fmt.Println("(No P2P in standalone mode - running in-process daemon)")
				return nil
			}

			// Get the gRPC client for the discovery service
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			// Connect to daemon
			daemonClient := protov1connect.NewDiscoveryServiceClient(
				&http.Client{},
				"http://localhost:6789",
			)

			// Call ListPeers
			resp, err := daemonClient.ListPeers(ctx, connect.NewRequest(&v1.ListPeersRequest{}))
			if err != nil {
				return fmt.Errorf("failed to list peers: %w", err)
			}

			peers := resp.Msg.Peers
			count := resp.Msg.Count

			fmt.Printf("Discovered Peers (%d total):\n", count)
			fmt.Println("============================")

			if count == 0 {
				fmt.Println("No peers discovered")
				return nil
			}

			for i, peer := range peers {
				status := "🟢 Online"
				if !peer.IsOnline {
					status = "🔴 Offline"
				}

				lastSeen := peer.LastSeen

				fmt.Printf("\n%d. %s (%s)\n", i+1, peer.PeerId, status)
				fmt.Printf("   Hostname: %s\n", peer.Hostname)

				if lastSeen != nil {
					fmt.Printf("   Last Seen:  %s\n", time.UnixMilli(lastSeen.UnixMillis).Format("2006-01-02 15:04:05"))
				}

				if len(peer.AvailableRuntimes) > 0 {
					fmt.Printf("   Runtimes: %d available\n", len(peer.AvailableRuntimes))
					for _, runtime := range peer.AvailableRuntimes {
						fmt.Printf("     - %s\n", runtime.Id)
					}
				}
			}

			return nil
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
			standalone, _ := IsStandaloneMode(cmd)

			if standalone {
				fmt.Println("Discovery (Standalone Mode):")
				fmt.Println("============================")
				fmt.Println("(No P2P in standalone mode - running in-process daemon)")
				return nil
			}

			// Get the gRPC client for the discovery service
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			// Connect to daemon
			daemonClient := protov1connect.NewDiscoveryServiceClient(
				&http.Client{},
				"http://localhost:6789",
			)

			// Call TriggerDiscovery
			resp, err := daemonClient.TriggerDiscovery(ctx, connect.NewRequest(&v1.TriggerDiscoveryRequest{}))
			if err != nil {
				return fmt.Errorf("failed to trigger discovery: %w", err)
			}

			fmt.Println("Discovery triggered:")
			fmt.Printf("Status: %v\n", resp.Msg.Status)
			fmt.Printf("Message: %s\n", resp.Msg.Message)

			return nil
		},
	}
}
