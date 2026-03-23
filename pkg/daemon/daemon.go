package daemon

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/Manu343726/buildozer/pkg/logging"
	"github.com/Manu343726/buildozer/pkg/runtimes"
)

// httpServer encapsulates the low-level HTTP/Connect server infrastructure.
// It handles the network listening, request routing, and graceful shutdown.
type httpServer struct {
	*logging.Logger // Embedded logger for hierarchical logging

	config DaemonConfig
	server *http.Server
	mux    *http.ServeMux

	mu      sync.RWMutex
	running bool
	ctx     context.Context
	cancel  context.CancelFunc
}

// newHTTPServer creates a new HTTP server instance.
func newHTTPServer(config DaemonConfig) *httpServer {
	return &httpServer{
		Logger: Log().Child("httpServer").With("host", config.Host, "port", config.Port),
		config: config,
		mux:    http.NewServeMux(),
	}
}

// registerHandler registers an HTTP handler for a given path.
func (hs *httpServer) registerHandler(path string, handler http.Handler) {
	hs.mu.Lock()
	defer hs.mu.Unlock()

	hs.mux.Handle(path, handler)
	hs.Debug("Handler registered", "path", path)
}

// start initializes and starts the HTTP server.
func (hs *httpServer) start() error {
	hs.mu.Lock()
	defer hs.mu.Unlock()

	if hs.running {
		return hs.Errorf("server is already running")
	}

	hs.ctx, hs.cancel = context.WithCancel(context.Background())

	addr := fmt.Sprintf("%s:%d", hs.config.Host, hs.config.Port)
	hs.server = &http.Server{
		Addr:    addr,
		Handler: hs.mux,
	}

	hs.running = true

	go func() {
		hs.Info("HTTP server starting", "addr", addr)
		if err := hs.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			hs.Error("HTTP server error", "error", err)
		}
	}()

	return nil
}

// stop gracefully shuts down the HTTP server.
func (hs *httpServer) stop(ctx context.Context) error {
	hs.mu.Lock()
	defer hs.mu.Unlock()

	if !hs.running {
		return nil
	}

	hs.Info("HTTP server shutting down")

	if hs.server != nil {
		if err := hs.server.Shutdown(ctx); err != nil {
			return hs.Errorf("HTTP server shutdown failed: %w", err)
		}
	}

	if hs.cancel != nil {
		hs.cancel()
	}

	hs.running = false
	return nil
}

// isRunning returns true if the server is running.
func (hs *httpServer) isRunning() bool {
	hs.mu.RLock()
	defer hs.mu.RUnlock()
	return hs.running
}

// getContext returns the server's context.
func (hs *httpServer) getContext() context.Context {
	hs.mu.RLock()
	defer hs.mu.RUnlock()
	return hs.ctx
}

// DaemonConfig holds the configuration for the daemon.
type DaemonConfig struct {
	// Network configuration
	Host string `json:"host" yaml:"host"` // Host to listen on (e.g., "localhost", "0.0.0.0")
	Port int    `json:"port" yaml:"port"` // Port to listen on (e.g., 6789)

	// Resource constraints
	MaxConcurrentJobs int `json:"max_concurrent_jobs" yaml:"max_concurrent_jobs"` // Maximum number of jobs to run concurrently
	MaxRAMMB          int `json:"max_ram_mb" yaml:"max_ram_mb"`                   // Maximum RAM to use for jobs (in MB)

	// Feature flags
	EnableMDNS bool `json:"enable_mdns" yaml:"enable_mdns"` // Enable mDNS peer discovery

	// Logging configuration for daemon mode
	Logging logging.LoggingConfig `json:"logging" yaml:"logging"` // Logging config used by daemon
}

// DefaultDaemonLoggingConfig returns the default logging configuration for daemon mode.
// It includes stdout and file sinks for buildozer-daemon.log in the daemon log directory.
func DefaultDaemonLoggingConfig() logging.LoggingConfig {
	return logging.LoggingConfig{
		GlobalLevel: "debug",
		LoggingDir:  "~/.cache/buildozer/logs", // Daemon logs go to user cache
		Sinks: []logging.SinkConfig{
			{
				Name:  "stdout",
				Type:  "stdout",
				Level: "debug",
			},
			{
				Name:       "daemon_file",
				Type:       "file",
				Level:      "debug",
				Filename:   "buildozer-daemon.log",
				MaxSizeB:   100 * 1024 * 1024, // 100MB
				MaxFiles:   10,
				MaxAgeDays: 30,
				JSONFormat: false,
			},
		},
		Loggers: []logging.LoggerConfig{
			{
				Name:  "buildozer",
				Level: "debug",
				Sinks: []string{"stdout", "daemon_file"},
			},
		},
	}
}

// DefaultConfig returns the default daemon configuration.
func DefaultConfig() DaemonConfig {
	return DaemonConfig{
		Host:              "127.0.0.1",
		Port:              6789,
		MaxConcurrentJobs: 4,
		MaxRAMMB:          8192,
		EnableMDNS:        true,
		Logging:           DefaultDaemonLoggingConfig(),
	}
}

// Daemon sets up, configures, and manages all buildozer client services.
//
// The Daemon is responsible for:
// - Initializing logging infrastructure
// - Setting up service handlers (logging, runtime, jobs, etc.)
// - Running the HTTP/Connect server
// - Coordinating graceful shutdown
//
// This is the main entry point for starting a buildozer daemon.
type Daemon struct {
	*logging.Logger // Embedded logger for daemon-level logging

	httpServer           *httpServer
	loggingConfigManager logging.ConfigManager
	runtimeManager       *runtimes.Manager
}

// NewDaemon creates a new Daemon with all standard services configured.
//
// This is the recommended way to create and start a daemon with all
// subsystems initialized:
//
// Example:
//
//	d, err := daemon.NewDaemon(daemonConfig)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer d.Stop(context.Background())
//	if err := d.Start(); err != nil {
//	    log.Fatal(err)
//	}
//	// Daemon is now running
//
// The Daemon handles initialization of:
// - Logging infrastructure and configuration manager
// - Logging service handler registration
// - (Future) Runtime detection and job execution
// - (Future) Job queue and scheduler
// - (Future) Peer discovery
func NewDaemon(config DaemonConfig) (*Daemon, error) {
	// Use the global logging system initialized by the caller
	// The caller (e.g., DaemonCommands) is responsible for initializing
	// the global logging system with the appropriate config
	loggingConfigMgr := logging.GetLocalConfigManager()
	if loggingConfigMgr == nil {
		return nil, fmt.Errorf("failed to initialize logging config manager")
	}

	// Create the HTTP server
	httpSrv := newHTTPServer(config)

	// Register health check endpoint
	httpSrv.registerHandler("/health", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "buildozer-client daemon healthy")
	}))

	// Register logging service
	loggingPath, loggingHandler := logging.RegisterLoggingService(loggingConfigMgr)
	httpSrv.registerHandler(loggingPath, loggingHandler)
	Log().Debug("Logging service registered", "path", loggingPath)

	// Create and register runtime service
	runtimeMgr := runtimes.NewManager()
	runtimePath, runtimeHandler := runtimes.RegisterService(runtimeMgr)
	httpSrv.registerHandler(runtimePath, runtimeHandler)
	Log().Debug("Runtime service registered", "path", runtimePath)

	// TODO: Register other services
	// - Job execution service
	// - Peer discovery service
	// - Cache service
	// - Queue/Scheduler service

	return &Daemon{
		Logger:               Log(),
		httpServer:           httpSrv,
		loggingConfigManager: loggingConfigMgr,
		runtimeManager:       runtimeMgr,
	}, nil
}

// Start starts the daemon and all registered services.
//
// Returns an error if the daemon fails to start.
func (d *Daemon) Start() error {
	if err := d.httpServer.start(); err != nil {
		return d.Errorf("Failed to start daemon: %w", err)
	}
	return nil
}

// Stop gracefully shuts down the daemon and all services.
//
// Returns an error if graceful shutdown fails (though the daemon will still stop).
func (d *Daemon) Stop(ctx context.Context) error {
	d.Info("stopping daemon")
	return d.httpServer.stop(ctx)
}

// IsRunning returns true if the daemon is running.
func (d *Daemon) IsRunning() bool {
	return d.httpServer.isRunning()
}

// Context returns the daemon's context for coordinating graceful shutdown.
func (d *Daemon) Context() context.Context {
	return d.httpServer.getContext()
}

// Config returns the daemon's configuration.
func (d *Daemon) Config() DaemonConfig {
	return d.httpServer.config
}

// LoggingConfigManager returns the logging config manager used by the daemon.
// This can be used to query or update logging configuration.
func (d *Daemon) LoggingConfigManager() logging.ConfigManager {
	return d.loggingConfigManager
}

// RegisterServiceHandler registers a Connect service handler with the daemon.
// This is used internally by NewDaemon to register all service handlers.
// For advanced use cases that need custom handlers, you can call this directly
// on the daemon after creation but before calling Start().
func (d *Daemon) RegisterServiceHandler(path string, handler http.Handler) {
	d.httpServer.registerHandler(path, handler)
	d.Info("Service registered", "path", path)
}
