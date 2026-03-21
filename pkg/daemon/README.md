# Daemon Package

The `daemon` package provides the high-level orchestration for the buildozer client daemon. It collects all subsystems (logging, runtime detection, job scheduling, peer discovery) and exposes them through a unified HTTP/Connect server.

## Architecture

The daemon package is structured around two main abstractions:

### 1. **Daemon** (daemon.go)
The core daemon struct that manages:
- **Lifecycle**: Start/Stop with graceful shutdown
- **Service Registration**: Mounts HTTP handlers for all services
- **Context Management**: Provides a context for coordinating shutdown

**Key Methods:**
- `New()` - Create a daemon with a config
- `Start()` - Initialize and start the server
- `Stop()` - Gracefully shutdown
- `RegisterServiceHandler()` - Register service handlers
- `IsRunning()`, `Context()`, `Config()` - Query daemon state

### 2. **Server** (server.go)
A higher-level convenience wrapper that:
- Creates a fully-configured daemon with all standard services initialized
- Handles initialization of logging, runtimes, job queue, etc.
- Provides a single entry point for typical usage

**Key Methods:**
- `NewServer()` - Create and initialize all services
- `Start()` / `Stop()` - Manage the daemon lifecycle
- `LoggingConfigManager()` - Access the logging system

### 3. **Builder** (options.go)
A fluent builder pattern for flexible daemon configuration:
- Sensible defaults for all settings
- Validation of configuration values
- Chainable methods for readability

## Usage

### Standalone Daemon
```go
// Create and start a fully-configured daemon
server, err := daemon.NewServer(daemon.DaemonConfig{
    Host:              "localhost",
    Port:              6789,
    MaxConcurrentJobs: 4,
    MaxRAMMB:          4096,
    EnableMDNS:        true,
})
if err != nil {
    log.Fatal(err)
}
defer server.Stop(context.Background())

if err := server.Start(); err != nil {
    log.Fatal(err)
}

// Server is now running and accepting connections
// Routes = /buildozer.proto.v1.LoggingService/... (and others)
```

### Using Builder Pattern
```go
daemon := daemon.NewBuilder().
    Host("0.0.0.0").
    Port(6789).
    MaxConcurrentJobs(8).
    MaxRAMMB(8192).
    EnableMDNS(true).
    Build()

server, err := daemon.NewServer(daemon.Config())
```

### Custom Service Registration
```go
daemon := daemon.New(config)

// Register custom services
customPath, customHandler := myPackage.RegisterMyService(myConfig)
daemon.RegisterServiceHandler(customPath, customHandler)

// Then start
daemon.Start()
defer daemon.Stop(context.Background())
```

## Service Registration Patterns

Each service that wants to be exposed through the daemon follows this pattern:

1. **Implement a Handler** - Implement the Connect service interface
2. **Create a Registration Function** - Returns (path, http.Handler)
   ```go
   func RegisterMyService(config Config) (string, http.Handler) {
       handler := newMyServiceHandler(config)
       path, mux := protov1connect.NewMyServiceHandler(handler)
       return path, mux
   }
   ```
3. **Daemon Registers It** - The Server or custom code calls `RegisterServiceHandler()`

## Current Services

- **LoggingService** - Query and modify logging configuration at runtime

## Future Services

As development progresses, these services will be integrated:

- **RuntimeService** - Query available runtimes, auto-detect toolchains
- **JobService** - Submit, monitor, and manage build jobs
- **CacheService** - Query cache status, manage cached artifacts
- **QueueService** - Monitor job queue and scheduler status
- **PeerService** - Discover and manage peer connections

## Graceful Shutdown

The daemon supports graceful shutdown through context cancellation:

```go
// Bind to interrupt signal
sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

// Wait for signal
<-sigChan

// Graceful shutdown with timeout
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
if err := server.Stop(ctx); err != nil {
    log.Printf("Shutdown error: %v", err)
}
```

## Thread Safety

The Daemon is thread-safe:
- `Start()` and `Stop()` can be called from different goroutines
- `RegisterServiceHandler()` is safe during initialization
- Configuration accessors (`Config()`, `IsRunning()`) use RWMutex

## Integration with cmd/buildozer-client

The `cmd/buildozer-client/cmd/daemon.go` command uses the Server:

```go
func runDaemon(cmd *cobra.Command) error {
    cfg := GetConfig()
    
    server, err := daemon.NewServer(daemon.DaemonConfig{
        Host:              cfg.Daemon.Host,
        Port:              cfg.Daemon.Port,
        MaxConcurrentJobs: cfg.Daemon.MaxConcurrentJobs,
        MaxRAMMB:          cfg.Daemon.MaxRAMMB,
        EnableMDNS:        cfg.PeerDiscovery.Enabled,
    })
    if err != nil {
        return err
    }
    
    if err := server.Start(); err != nil {
        return err
    }
    
    // Handle shutdown...
    return server.Stop(context.Background())
}
```

## Design Philosophy

The daemon package embodies several design principles:

1. **Separation of Concerns** - Each subsystem handles its domain; daemon orchestrates
2. **Composition Over Inheritance** - Services are composed into the daemon
3. **Explicit Over Implicit** - All dependencies are explicitly injected/registered
4. **Graceful Degradation** - Services can be optional; daemon still works
5. **Testability** - Clean interfaces make mocking and testing straightforward
