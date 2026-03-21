package daemon

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Manu343726/buildozer/pkg/logging"
)

// Server provides high-level convenience functions for setting up a fully-configured daemon
// with all standard services initialized and registered.
type Server struct {
	daemon               *Daemon
	loggingConfigManager logging.ConfigManager
}

// NewServer creates a new Server with all standard services configured.
//
// This is the recommended way to create and start a daemon with all
// subsystems initialized:
//
// Example:
//
//	server, err := daemon.NewServer(daemonConfig)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer server.Stop(context.Background())
//	if err := server.Start(); err != nil {
//	    log.Fatal(err)
//	}
//	// Server is now running
//
// The Server handles initialization of:
// - Logging infrastructure and configuration manager
// - Runtime detection
// - Logging service handler registration
// - (Future) Job queue and scheduler
// - (Future) Peer discovery
func NewServer(config DaemonConfig) (*Server, error) {
	// Initialize logging
	// Currently uses global logging infrastructure
	// In the future, we'll pass context/options for customization
	loggingConfigMgr := logging.GetLocalConfigManager()
	if loggingConfigMgr == nil {
		return nil, fmt.Errorf("failed to initialize logging config manager")
	}

	// Create the daemon
	daemon := New(config)

	// Register logging service
	loggingPath, loggingHandler := logging.RegisterLoggingService(loggingConfigMgr)
	daemon.RegisterServiceHandler(loggingPath, loggingHandler)
	slog.Debug("Logging service registered", "path", loggingPath)

	// TODO: Register other services
	// - Runtime/Job service
	// - Peer discovery service
	// - Cache service
	// - Queue/Scheduler service

	return &Server{
		daemon:               daemon,
		loggingConfigManager: loggingConfigMgr,
	}, nil
}

// Start starts the daemon and all registered services.
//
// Returns an error if the daemon fails to start.
func (s *Server) Start() error {
	if err := s.daemon.Start(); err != nil {
		slog.Error("Failed to start daemon", "error", err)
		return err
	}
	return nil
}

// Stop gracefully shuts down the daemon and all services.
//
// Returns an error if graceful shutdown fails (though the daemon will still stop).
func (s *Server) Stop(ctx context.Context) error {
	return s.daemon.Stop(ctx)
}

// IsRunning returns true if the daemon is running.
func (s *Server) IsRunning() bool {
	return s.daemon.IsRunning()
}

// Context returns the daemon's context for coordinating graceful shutdown.
func (s *Server) Context() context.Context {
	return s.daemon.Context()
}

// Config returns the daemon's configuration.
func (s *Server) Config() DaemonConfig {
	return s.daemon.Config()
}

// Daemon returns the underlying Daemon instance.
// This is exposed for advanced use cases that need direct access to the daemon.
func (s *Server) Daemon() *Daemon {
	return s.daemon
}

// LoggingConfigManager returns the logging config manager used by the server.
// This can be used to query or update logging configuration.
func (s *Server) LoggingConfigManager() logging.ConfigManager {
	return s.loggingConfigManager
}
