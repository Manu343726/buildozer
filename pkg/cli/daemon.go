package cli

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/Manu343726/buildozer/pkg/config"
	"github.com/Manu343726/buildozer/pkg/daemon"
	"github.com/Manu343726/buildozer/pkg/logging"
)

type DaemonCommands struct {
	*logging.Logger // Embedded logger for hierarchical logging

	daemon  *daemon.Daemon
	sigChan chan os.Signal
}

func NewDaemonCommands(cfg *config.Config) (*DaemonCommands, error) {
	if cfg.Standalone {
		return nil, Log().Errorf("the 'daemon' command cannot be run in standalone mode. Please remove the --standalone flag")
	}

	// Logging system is already initialized globally by root command
	// based on whether we're in daemon mode or client mode

	// Extract daemon configuration from the full config
	daemonCfg := cfg.Daemon

	d, err := daemon.NewDaemon(daemonCfg)
	if err != nil {
		return nil, err
	}

	// Create signal channel for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	return &DaemonCommands{
		Logger:  Log().Child("DaemonCommands"),
		daemon:  d,
		sigChan: sigChan,
	}, nil
}

// Start starts the daemon and blocks until a shutdown signal is received.
// It installs a signal handler to gracefully shutdown the daemon when SIGINT or SIGTERM is received.
func (dc *DaemonCommands) Start() error {
	// Start the daemon
	if err := dc.daemon.Start(); err != nil {
		return dc.Errorf("failed to start daemon: %w", err)
	}

	dc.Info("Daemon started successfully")
	dc.Info("Daemon running. Press Ctrl+C to stop.")

	// Wait for shutdown signal
	sig := <-dc.sigChan
	dc.Info("Received signal", "signal", sig.String())
	dc.Info("Shutting down gracefully")

	// Graceful shutdown
	if err := dc.daemon.Stop(context.Background()); err != nil {
		return dc.Errorf("error during graceful shutdown: %w", err)
	}

	dc.Info("Daemon stopped successfully")
	return nil
}
