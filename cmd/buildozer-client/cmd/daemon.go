package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Manu343726/buildozer/pkg/daemon"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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
			// Check if --standalone was set globally
			standalone, _ := cmd.Flags().GetBool("standalone")
			if standalone {
				return fmt.Errorf("cannot use 'daemon' subcommand with --standalone flag\n\nUse --standalone with other commands to run in-process:\n  buildozer-client --standalone status\n  buildozer-client --standalone peers\n  buildozer-client --standalone logs")
			}
			return runDaemon(cmd)
		},
	}

	return cmd
}

// runDaemon is the main daemon execution function
func runDaemon(cmd *cobra.Command) error {
	cfg := GetConfig()

	fmt.Printf("[INFO] Starting buildozer client daemon on %s:%d\n", cfg.Daemon.Host, cfg.Daemon.Port)
	fmt.Printf("[INFO] Listening on %s (will bind to all interfaces for mDNS)\n", cfg.Daemon.Listen)
	fmt.Printf("[INFO] Max concurrent jobs: %d\n", cfg.Daemon.MaxConcurrentJobs)
	fmt.Printf("[INFO] Max RAM: %d MB\n", cfg.Daemon.MaxRAMMB)
	fmt.Printf("[INFO] mDNS discovery: %v (interval: %d seconds)\n", cfg.PeerDiscovery.Enabled, cfg.PeerDiscovery.MDNSIntervalSecs)

	// Print configuration
	if viper.GetBool("debug") {
		fmt.Println("\n[DEBUG] Full configuration:")
		PrintConfigSummary()
		fmt.Println()
	}

	// Create daemon with configuration
	server, err := daemon.NewServer(daemon.DaemonConfig{
		Host:              cfg.Daemon.Host,
		Port:              cfg.Daemon.Port,
		MaxConcurrentJobs: cfg.Daemon.MaxConcurrentJobs,
		MaxRAMMB:          cfg.Daemon.MaxRAMMB,
		EnableMDNS:        cfg.PeerDiscovery.Enabled,
	})
	if err != nil {
		return fmt.Errorf("failed to initialize daemon: %w", err)
	}

	// Start the daemon
	if err := server.Start(); err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}
	fmt.Println("[INFO] Daemon started successfully")

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println("[INFO] Daemon running. Press Ctrl+C to stop.")

	// Wait for signal
	sig := <-sigChan
	fmt.Printf("\n[INFO] Received signal: %v\n", sig)
	fmt.Println("[INFO] Shutting down gracefully...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Stop(ctx); err != nil {
		slog.Error("Error during graceful shutdown", "error", err)
		return err
	}

	fmt.Println("[INFO] Daemon stopped.")
	return nil
}
