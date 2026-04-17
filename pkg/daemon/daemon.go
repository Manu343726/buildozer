package daemon

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/Manu343726/buildozer/pkg/logging"
	"github.com/Manu343726/buildozer/pkg/peers"
	"github.com/Manu343726/buildozer/pkg/runtimes"
	"github.com/Manu343726/buildozer/pkg/scheduler"
)

// httpServer encapsulates the low-level HTTP/Connect server infrastructure.
// It handles the network listening, request routing, and graceful shutdown.
type httpServer struct {
	*logging.Logger // Embedded logger for hierarchical logging

	config DaemonConfig
	server *http.Server
	mux    *http.ServeMux

	mu           sync.RWMutex
	running      bool
	ctx          context.Context
	cancel       context.CancelFunc
	startupErr   chan error    // Channel to report startup errors
	startupReady chan struct{} // Signal that server is ready/failed to bind
}

// newHTTPServer creates a new HTTP server instance.
func newHTTPServer(config DaemonConfig, daemonID string) *httpServer {
	return &httpServer{
		Logger:       logging.Log(daemonID).Child("httpServer").With("host", config.Host, "port", config.Port),
		config:       config,
		mux:          http.NewServeMux(),
		startupErr:   make(chan error, 1),
		startupReady: make(chan struct{}, 1),
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
// It waits for the server to either successfully bind or fail, returning any startup error.
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
			// Send error to startup error channel so caller knows about it
			select {
			case hs.startupErr <- err:
			default:
			}
		}
		// Signal that startup sequence is done (either success or failure)
		select {
		case hs.startupReady <- struct{}{}:
		default:
		}
	}()

	// Wait for server to either bind successfully or fail
	// Give it a small timeout to detect binding errors
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	select {
	case err := <-hs.startupErr:
		// Server failed to bind
		hs.running = false
		return hs.Errorf("failed to start HTTP server: %w", err)
	case <-hs.startupReady:
		// Server is ready (or startup sequence completed)
		// Check if an error was sent
		select {
		case err := <-hs.startupErr:
			hs.running = false
			return hs.Errorf("failed to start HTTP server: %w", err)
		default:
			// No error, server is running
			hs.Info("HTTP server started successfully", "addr", addr)
			return nil
		}
	case <-ctx.Done():
		// Timeout waiting for startup - assume server started successfully if we didn't get an error
		select {
		case err := <-hs.startupErr:
			hs.running = false
			return hs.Errorf("failed to start HTTP server: %w", err)
		default:
			// No error reported, assume startup succeeded
			hs.Info("HTTP server started", "addr", addr)
			return nil
		}
	}
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

// getAdvertisedHost returns the host address that should be advertised for peer discovery.
// If the listen address is 0.0.0.0 (all interfaces), returns the machine's hostname.
// If it's localhost/127.0.0.1, returns that as-is (for local-only daemons).
// Otherwise returns the configured host.
func getAdvertisedHost(listenHost string) string {
	if listenHost == "0.0.0.0" {
		// Listen on all interfaces, advertise the hostname
		if hostname, err := os.Hostname(); err == nil && hostname != "" {
			return hostname
		}
		// Fallback: try to get the primary local IP
		if ip := getLocalIP(); ip != "" {
			return ip
		}
		// Last resort: use localhost
		return "localhost"
	}
	// Use the configured host (localhost, 127.0.0.1, or explicit IP)
	return listenHost
}

// getLocalIP returns the IP address of the primary local network interface.
func getLocalIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return ""
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
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

	// mDNS configuration
	MDNSIntervalSecs int `json:"mdns_interval_secs" yaml:"mdns_interval_secs"` // mDNS discovery interval in seconds

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
				Name:                          "stdout",
				Type:                          "stdout",
				Level:                         "debug",
				IncludeSourceLocation:         true, // Include source location by default
				OmitLoggerNameIfSourceEnabled: true, // Omit logger name when source is enabled
			},
			{
				Name:                          "daemon_file",
				Type:                          "file",
				Level:                         "debug",
				Filename:                      "buildozer-daemon.log",
				MaxSizeB:                      100 * 1024 * 1024, // 100MB
				MaxFiles:                      10,
				MaxAgeDays:                    30,
				JSONFormat:                    false,
				IncludeSourceLocation:         true, // Include source location by default
				OmitLoggerNameIfSourceEnabled: true, // Omit logger name when source is enabled
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
		MDNSIntervalSecs:  30,
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
// - Managing peer discovery via mDNS
// - Coordinating distributed job scheduling
//
// This is the main entry point for starting a buildozer daemon.
type Daemon struct {
	*logging.Logger // Embedded logger for daemon-level logging

	daemonID    string // Unique identifier for this daemon instance
	config      DaemonConfig
	httpServer  *httpServer
	discoverer  *peers.MulticastDiscoverer
	peerManager *peers.Manager
	logging     logging.ConfigManager
	runtimes    runtimes.Manager
	jobManager  *JobManager
	scheduler   *scheduler.Scheduler // Scheduler for job placement and remote execution
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
// - Runtime detection and job execution
// - Peer discovery via mDNS
// - (Future) Cache service
// - (Future) Queue/Scheduler service
func NewDaemon(config DaemonConfig) (*Daemon, error) {
	// Use the global logging system initialized by the caller
	// The caller (e.g., DaemonCommands) is responsible for initializing
	// the global logging system with the appropriate config
	loggingConfigMgr := logging.GetLocalConfigManager()
	if loggingConfigMgr == nil {
		return nil, fmt.Errorf("failed to initialize logging config manager")
	}

	// Compute daemon ID early so all components can use it
	advertisedHost := getAdvertisedHost(config.Host)
	daemonID := fmt.Sprintf("%s:%d", advertisedHost, config.Port)

	// Create the HTTP server with daemon ID
	httpSrv := newHTTPServer(config, daemonID)

	// Register health check endpoint
	httpSrv.registerHandler("/health", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "buildozer-client daemon healthy")
	}))

	// Register logging service
	loggingPath, loggingHandler := logging.RegisterLoggingService(loggingConfigMgr)
	httpSrv.registerHandler(loggingPath, loggingHandler)
	logging.Log(daemonID).Debug("Logging service registered", "path", loggingPath)

	// Create peer manager early (will be shared with services and discoverer)
	peerManager := peers.NewManager()

	// Create local runtime manager with local daemon ID
	localRuntimeMgr := runtimes.NewLocalRuntimesManager(daemonID)

	// Create remote runtime manager for discovering peer runtimes
	// The remote manager wraps remote runtimes with RPC implementations
	remoteRuntimeMgr := runtimes.NewRemoteRuntimesManager(peerManager)

	// Create aggregated manager combining local and remote runtimes
	aggregatedRuntimeMgr := runtimes.NewAggregatedRuntimesManager(localRuntimeMgr, remoteRuntimeMgr)

	// Register runtime service with aggregated manager for network-wide runtime discovery
	runtimePath, runtimeHandler := runtimes.RegisterService(aggregatedRuntimeMgr)
	httpSrv.registerHandler(runtimePath, runtimeHandler)
	logging.Log(daemonID).Debug("Runtime service registered", "path", runtimePath)

	// Create and register job service (use local runtime manager for execution)
	jobManager := NewJobManager(daemonID, localRuntimeMgr)
	jobPath, jobHandler := RegisterJobService(daemonID, jobManager)
	httpSrv.registerHandler(jobPath, jobHandler)
	logging.Log(daemonID).Debug("Job service registered", "path", jobPath)

	// Create scheduler for distributed job scheduling
	// The scheduler manages job placement decisions and remote job execution
	schedulerImpl := createScheduler(daemonID, peerManager, localRuntimeMgr)
	if schedulerImpl != nil {
		jobManager.SetScheduler(schedulerImpl)
		logging.Log(daemonID).Debug("Scheduler integrated with job manager")
	}

	// Register scheduler service for peer-to-peer scheduling coordination
	schedulerPath, schedulerHandler := createSchedulerService(daemonID, schedulerImpl, peerManager, localRuntimeMgr)
	httpSrv.registerHandler(schedulerPath, schedulerHandler)
	logging.Log(daemonID).Debug("Scheduler service registered", "path", schedulerPath)

	// Create multicast discoverer with RPC URI
	// Inject peer manager and local runtime manager for announcement
	rpcURI := fmt.Sprintf("%s:%d", advertisedHost, config.Port)
	discoverer := peers.NewMulticastDiscoverer(daemonID, rpcURI, config.MDNSIntervalSecs, peerManager, localRuntimeMgr)

	// Register discovery service (will be initialized after we create the Daemon struct below,
	// but we create the discoverer object now so it's available in the service handler)
	// We'll register it after returning the Daemon instance in a separate call

	// TODO: Register other services
	// - Cache service

	d := &Daemon{
		Logger:      logging.Log(daemonID),
		daemonID:    daemonID,
		config:      config,
		httpServer:  httpSrv,
		discoverer:  discoverer,
		peerManager: peerManager,
		logging:     loggingConfigMgr,
		runtimes:    localRuntimeMgr,
		jobManager:  jobManager,
		scheduler:   schedulerImpl,
	}

	// Register discovery service now that we have the daemon instance
	discoveryPath, discoveryHandler := RegisterDiscoveryService(daemonID, d)
	httpSrv.registerHandler(discoveryPath, discoveryHandler)
	logging.Log(daemonID).Debug("Discovery service registered", "path", discoveryPath)

	// Register introspection service for daemon state queries
	introspectionPath, introspectionHandler := RegisterIntrospectionService(daemonID, d)
	httpSrv.registerHandler(introspectionPath, introspectionHandler)
	logging.Log(daemonID).Debug("Introspection service registered", "path", introspectionPath)

	return d, nil
}

// Start starts the daemon and all registered services.
//
// Returns an error if the daemon fails to start.
// Discovers runtimes and starts mDNS peer discovery on startup.
func (d *Daemon) Start() error {
	ctx := context.Background()

	// Discover runtimes on startup
	d.Info("discovering runtimes on startup")
	runtimes, notes, err := d.runtimes.GetRuntimes(ctx)
	if err != nil {
		d.Error("failed to discover runtimes", "error", err)
		// Don't fail startup - continue with partial or no runtimes available
	} else {
		d.Info("runtime discovery complete", "count", len(runtimes), "notes", notes)
	}

	// Get full proto runtime objects for the peer manager
	protoRuntimes, _, _ := d.runtimes.ListRuntimes(ctx)

	// Parse daemon address to get host and port
	advertisedHost := getAdvertisedHost(d.config.Host)
	localPeer := &peers.PeerInfo{
		ID:       d.daemonID,
		Host:     advertisedHost,
		Port:     d.config.Port,
		Runtimes: protoRuntimes,
		IsAlive:  true,
		Endpoint: fmt.Sprintf("%s:%d", advertisedHost, d.config.Port),
		IsLocal:  true,
	}
	d.peerManager.SetLocalPeer(localPeer)
	d.Info("local peer info set in peer manager", "peer_id", d.daemonID, "runtime_count", len(protoRuntimes))

	// Start UDP multicast discovery if enabled
	if d.config.EnableMDNS {
		d.Info("starting UDP multicast peer discovery", "interval_secs", d.config.MDNSIntervalSecs)
		if err := d.discoverer.Start(ctx); err != nil {
			d.Error("failed to start multicast discoverer", "error", err)
			// Stop the daemon if multicast discovery fails
			d.Stop(ctx)
			return d.Errorf("failed to start daemon: multicast discovery failed: %w", err)
		}
	} else {
		d.Info("UDP multicast peer discovery disabled in config")
	}

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

	// Stop mDNS discoverer
	if d.discoverer.IsRunning() {
		if err := d.discoverer.Stop(); err != nil {
			d.Error("error stopping mDNS discoverer", "error", err)
		}
	}

	// Stop HTTP server
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
	return d.config
}

// LoggingConfigManager returns the logging config manager used by the daemon.
// This can be used to query or update logging configuration.
func (d *Daemon) LoggingConfigManager() logging.ConfigManager {
	return d.logging
}

// RegisterServiceHandler registers a Connect service handler with the daemon.
// This is used internally by NewDaemon to register all service handlers.
// For advanced use cases that need custom handlers, you can call this directly
// on the daemon after creation but before calling Start().
func (d *Daemon) RegisterServiceHandler(path string, handler http.Handler) {
	d.httpServer.registerHandler(path, handler)
	d.Info("Service registered", "path", path)
}

// Discoverer returns the mDNS discoverer used by the daemon.
// This can be used to query discovered peers.
func (d *Daemon) Discoverer() *peers.MulticastDiscoverer {
	return d.discoverer
}

// GetDiscoveredPeers returns all peers discovered via mDNS.
func (d *Daemon) GetDiscoveredPeers() []*peers.PeerInfo {
	if d.discoverer == nil {
		return []*peers.PeerInfo{}
	}
	return d.discoverer.GetDiscoveredPeers()
}

// createScheduler creates a new local scheduler instance
// Uses a type-switch pattern to avoid direct imports of the scheduler package
func createScheduler(daemonID string, peerManager *peers.Manager, runtimeManager runtimes.Manager) *scheduler.Scheduler {
	// Create the scheduler for job placement decisions and remote execution
	config := &scheduler.SchedulerConfig{
		Heuristic:      scheduler.NewSimpleLocalFirstHeuristic(),
		RuntimeManager: runtimeManager,
		PeerManager:    peerManager,
		LocalDaemonID:  daemonID,
	}

	sched, err := scheduler.NewScheduler(config)
	if err != nil {
		logging.Log(daemonID).Error("Failed to create scheduler", "error", err)
		return nil
	}

	return sched
}

// createSchedulerService creates and returns the scheduler service handler
func createSchedulerService(daemonID string, sched *scheduler.Scheduler, peerManager *peers.Manager, runtimeManager runtimes.Manager) (string, http.Handler) {
	// Placeholder implementation - will be properly implemented when scheduler package is fully integrated
	// This is to allow the build to succeed before full integration
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintf(w, "Scheduler service not yet available")
	})
	return "/buildozer.proto.v1.SchedulerService/", mux
}
