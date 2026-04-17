package cli

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"connectrpc.com/connect"
	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	"github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1/protov1connect"
	"github.com/Manu343726/buildozer/pkg/config"
	"github.com/Manu343726/buildozer/pkg/daemon"
	"github.com/Manu343726/buildozer/pkg/logging"
)

// PeersCommands provides command-level implementations for peer discovery CLI operations.
type PeersCommands struct {
	*logging.Logger // Embedded logger for hierarchical logging

	cfg *config.Config
}

// NewPeersCommands creates a new PeersCommands handler.
func NewPeersCommands(cfg *config.Config) (*PeersCommands, error) {
	return &PeersCommands{
		Logger: Log().Child("PeersCommands"),
		cfg:    cfg,
	}, nil
}

// List lists all discovered peers
func (pc *PeersCommands) List() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create gRPC client
	daemonURL := fmt.Sprintf("http://%s:%d", pc.cfg.Daemon.Host, pc.cfg.Daemon.Port)
	daemonClient := protov1connect.NewDiscoveryServiceClient(
		&http.Client{},
		daemonURL,
	)

	// Call ListPeers with CLI requester identification
	resp, err := daemonClient.ListPeers(ctx, connect.NewRequest(&v1.ListPeersRequest{
		RequesterInfo: &v1.RequesterInfo{
			RequesterId:   "cli",
			RequesterType: "cli",
		},
	}))
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
		if peer.IsOnline == nil || !*peer.IsOnline {
			status = "🔴 Offline"
		}

		// Mark local daemon
		localIndicator := ""
		if peer.IsLocal != nil && *peer.IsLocal {
			localIndicator = " [THIS DAEMON]"
		}

		lastSeen := peer.LastSeen

		fmt.Printf("\n%d. %s%s (%s)\n", i+1, peer.PeerId, localIndicator, status)
		fmt.Printf("   Endpoint: %s:%d\n", peer.ApiUri.Host, peer.ApiUri.Port)
		if peer.Hostname != nil && *peer.Hostname != "" {
			fmt.Printf("   Hostname: %s\n", *peer.Hostname)
		}

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
}

// Discover triggers peer discovery
func (pc *PeersCommands) Discover() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create gRPC client
	daemonURL := daemon.RpcURL(pc.cfg.Daemon.Host, pc.cfg.Daemon.Port)
	daemonClient := protov1connect.NewDiscoveryServiceClient(
		&http.Client{},
		daemonURL,
	)

	// Call TriggerDiscovery with CLI requester identification
	resp, err := daemonClient.TriggerDiscovery(ctx, connect.NewRequest(&v1.TriggerDiscoveryRequest{
		RequesterInfo: &v1.RequesterInfo{
			RequesterId:   "cli",
			RequesterType: "cli",
		},
	}))
	if err != nil {
		return fmt.Errorf("failed to trigger discovery: %w", err)
	}

	fmt.Println("Discovery triggered:")
	fmt.Printf("Status: %v\n", resp.Msg.Status)
	fmt.Printf("Message: %s\n", resp.Msg.Message)

	return nil
}
