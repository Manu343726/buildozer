package cli

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/Manu343726/buildozer/pkg/config"
	"github.com/Manu343726/buildozer/pkg/daemon"
	"github.com/Manu343726/buildozer/pkg/logging"
)

type DaemonCommands struct {
	*logging.Logger // Embedded logger for hierarchical logging

	daemons []*daemon.Daemon
	sigChan chan os.Signal
}

func NewDaemonCommands(cfg *config.Config, count int) (*DaemonCommands, error) {
	if cfg.Standalone {
		return nil, Log().Errorf("the 'daemon' command cannot be run in standalone mode. Please remove the --standalone flag")
	}

	if count < 1 {
		return nil, Log().Errorf("--count must be >= 1, got %d", count)
	}

	// Logging system is already initialized globally by root command
	// based on whether we're in daemon mode or client mode

	// Extract daemon configuration from the full config
	baseDaemonCfg := cfg.Daemon

	var daemons []*daemon.Daemon

	// Always create the first daemon with configured host and port
	d, err := daemon.NewDaemon(baseDaemonCfg)
	if err != nil {
		return nil, Log().Errorf("failed to create daemon 1: %w", err)
	}
	daemons = append(daemons, d)

	// If count > 1, create additional daemons with random free ports
	if count > 1 {
		ports, err := findFreeRandomPorts(baseDaemonCfg.Host, count-1)
		if err != nil {
			return nil, Log().Errorf("failed to find %d free ports: %w", count-1, err)
		}

		for i, port := range ports {
			daemonCfg := baseDaemonCfg
			daemonCfg.Port = port

			d, err := daemon.NewDaemon(daemonCfg)
			if err != nil {
				return nil, Log().Errorf("failed to create daemon %d: %w", i+2, err)
			}
			daemons = append(daemons, d)
		}
	}

	// Create signal channel for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	return &DaemonCommands{
		Logger:  Log().Child("DaemonCommands"),
		daemons: daemons,
		sigChan: sigChan,
	}, nil
}

// findFreeRandomPorts finds n random free ports on the given host.
func findFreeRandomPorts(host string, count int) ([]int, error) {
	var ports []int
	usedPorts := make(map[int]bool)

	for len(ports) < count {
		// Listen on port 0 to get a random free port
		addr := fmt.Sprintf("%s:0", host)
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			return nil, err
		}
		defer ln.Close()

		// Get the actual port that was allocated
		port := ln.Addr().(*net.TCPAddr).Port

		// Ensure we don't have duplicates
		if !usedPorts[port] {
			usedPorts[port] = true
			ports = append(ports, port)
		}
	}

	return ports, nil
}

// Start starts all daemons and blocks until a shutdown signal is received.
// It installs a signal handler to gracefully shutdown all daemons when SIGINT or SIGTERM is received.
func (dc *DaemonCommands) Start() error {
	// Start all daemons concurrently
	var wg sync.WaitGroup
	errChan := make(chan error, len(dc.daemons))

	for i, d := range dc.daemons {
		wg.Add(1)
		go func(index int, daemon *daemon.Daemon) {
			defer wg.Done()
			if err := daemon.Start(); err != nil {
				errChan <- fmt.Errorf("daemon %d failed to start: %w", index+1, err)
			}
		}(i, d)
	}

	// Wait for all daemons to start or report errors
	go wg.Wait()

	// Check for startup errors (non-blocking)
	select {
	case err := <-errChan:
		dc.Error("Daemon startup error", "error", err)
		return err
	default:
		// All daemons started successfully
	}

	dc.Info("Daemons started successfully", "count", len(dc.daemons))
	for i, d := range dc.daemons {
		cfg := d.Config()
		dc.Info("Daemon listening", "daemon", i+1, "host", cfg.Host, "port", cfg.Port)
	}

	dc.Info("Press Ctrl+C to stop all daemons")

	// Wait for shutdown signal
	sig := <-dc.sigChan
	dc.Info("Received signal", "signal", sig.String())
	dc.Info("Shutting down all daemons gracefully")

	// Graceful shutdown of all daemons
	var shutdownWg sync.WaitGroup
	shutdownErrors := make(chan error, len(dc.daemons))

	for i, d := range dc.daemons {
		shutdownWg.Add(1)
		go func(index int, daemon *daemon.Daemon) {
			defer shutdownWg.Done()
			if err := daemon.Stop(context.Background()); err != nil {
				shutdownErrors <- fmt.Errorf("daemon %d shutdown error: %w", index+1, err)
			}
		}(i, d)
	}

	shutdownWg.Wait()
	close(shutdownErrors)

	// Check for shutdown errors
	var shutdownError error
	for err := range shutdownErrors {
		dc.Error("Daemon shutdown error", "error", err)
		if shutdownError == nil {
			shutdownError = err
		}
	}

	dc.Info("All daemons stopped")
	return shutdownError
}
