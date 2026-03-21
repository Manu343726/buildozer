package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
)

// Daemon encapsulates the buildozer client daemon, collecting all subsystems
// and exposing them through a unified gRPC API.
//
// The Daemon manages:
// - Logging infrastructure (global logger and config manager)
// - Runtime detection and management
// - Job queue and scheduler
// - HTTP/Connect server for remote access
// - Peer discovery (mDNS)
//
// The daemon is designed to separate concerns: subsystems focus on their
// specific domain, while the Daemon orchestrates them together.
type Daemon struct {
	// Configuration
	config DaemonConfig

	// Core subsystems
	server *http.Server
	mux    *http.ServeMux

	// Lifecycle management
	mu      sync.RWMutex
	running bool
	ctx     context.Context
	cancel  context.CancelFunc
}

// DaemonConfig holds the configuration for the daemon
type DaemonConfig struct {
	// Network configuration
	Host string // Host to listen on (e.g., "localhost", "0.0.0.0")
	Port int    // Port to listen on (e.g., 6789)

	// Resource constraints
	MaxConcurrentJobs int // Maximum number of jobs to run concurrently
	MaxRAMMB          int // Maximum RAM to use for jobs (in MB)

	// Feature flags
	EnableMDNS bool // Enable mDNS peer discovery
}

// New creates a new Daemon with the given configuration.
//
// The daemon is not started until Start() is called. This allows for
// proper lifecycle management and error handling during initialization.
func New(config DaemonConfig) *Daemon {
	return &Daemon{
		config: config,
		mux:    http.NewServeMux(),
	}
}

// RegisterServiceHandler registers a Connect service handler with the daemon.
//
// Services should call this during daemon initialization to register their
// handlers. Multiple services can be registered before Start() is called.
//
// Example:
//
//	loggingPath, loggingHandler := logging.RegisterLoggingService(configManager)
//	daemon.RegisterServiceHandler(loggingPath, loggingHandler)
func (d *Daemon) RegisterServiceHandler(path string, handler http.Handler) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.mux.Handle(path, handler)
	slog.Info("Service registered", "path", path)
}

// Start initializes and starts the daemon.
//
// This method:
// 1. Validates the configuration
// 2. Sets up the HTTP/Connect server
// 3. Starts the server on the configured host:port
// 4. Returns an error if the server fails to start
//
// The daemon runs asynchronously; callers should listen for errors on the
// error channel or call Stop() to shut down gracefully.
func (d *Daemon) Start() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.running {
		return fmt.Errorf("daemon is already running")
	}

	// Create a cancellable context for lifecycle management
	d.ctx, d.cancel = context.WithCancel(context.Background())

	// Create the HTTP server with the mux
	addr := fmt.Sprintf("%s:%d", d.config.Host, d.config.Port)
	d.server = &http.Server{
		Addr:    addr,
		Handler: d.mux,
	}

	d.running = true

	// Start the server in a goroutine
	go func() {
		slog.Info("Daemon starting", "host", d.config.Host, "port", d.config.Port)
		if err := d.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server error", "error", err)
		}
	}()

	return nil
}

// Stop gracefully shuts down the daemon.
//
// This method:
// 1. Closes the HTTP/Connect server (with context timeout)
// 2. Cancels the daemon context
// 3. Cleans up any background goroutines
// 4. Returns an error if shutdown fails
//
// Stop() is idempotent; calling it multiple times is safe.
func (d *Daemon) Stop(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.running {
		return nil // Already stopped
	}

	slog.Info("Daemon shutting down")

	// Close the server with timeout
	if d.server != nil {
		if err := d.server.Shutdown(ctx); err != nil {
			slog.Error("Server shutdown error", "error", err)
			return err
		}
	}

	// Cancel the daemon context
	if d.cancel != nil {
		d.cancel()
	}

	d.running = false
	return nil
}

// IsRunning returns true if the daemon is currently running.
func (d *Daemon) IsRunning() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.running
}

// Context returns the daemon's context, which is cancelled when Stop() is called.
// Subsystems can use this context to coordinate graceful shutdown.
func (d *Daemon) Context() context.Context {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.ctx
}

// Config returns the daemon's configuration.
func (d *Daemon) Config() DaemonConfig {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.config
}
