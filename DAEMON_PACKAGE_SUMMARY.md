# Daemon Package Implementation Summary

## Overview
The `pkg/daemon` package provides the high-level orchestration layer for the buildozer client. It collects all subsystems (logging, runtime detection, job scheduling, peer discovery) and exposes them through a unified HTTP/Connect server.

## Key Components

### 1. **Daemon Core** (`daemon.go`)
- Main `Daemon` struct that orchestrates all subsystems
- Thread-safe lifecycle management (Start/Stop)
- Service handler registration interface
- Configuration management

**Key Methods:**
```go
New(config DaemonConfig) *Daemon
Start() error
Stop(ctx context.Context) error
RegisterServiceHandler(path string, handler http.Handler)
IsRunning() bool
Context() context.Context
Config() DaemonConfig
```

### 2. **Server** (`server.go`)
- High-level convenience wrapper (`Server`)
- Initializes all standard services automatically
- Single entry point for typical daemon setup
- Provides access to underlying components when needed

**Key Methods:**
```go
NewServer(config DaemonConfig) (*Server, error)
Start() error
Stop(ctx context.Context) error
LoggingConfigManager() logging.ConfigManager
```

### 3. **Builder Pattern** (`options.go`)
- Fluent builder for flexible configuration
- Sensible defaults for all settings
- Configuration validation

**Usage:**
```go
daemon := daemon.NewBuilder().
    Host("0.0.0.0").
    Port(6789).
    MaxConcurrentJobs(8).
    Build()
```

## Architecture

```
┌─────────────────────────────────────────────┐
│         cmd/buildozer-client                │
│      (daemon.go command handler)            │
└────────────────┬────────────────────────────┘
                 │ creates
                 ▼
         ┌───────────────┐
         │ daemon.Server │
         └───────┬───────┘
                 │ owns
                 ▼
         ┌───────────────────────┐
         │ daemon.Daemon         │
         │ (HTTP Server + Mux)   │
         └───────┬───────────────┘
                 │ mounts services
         ┌───────┴──────────────────┐
         │                          │
         ▼                          ▼
    LoggingService         (Future Services)
    - GetLoggingStatus          - RuntimeService
    - SetGlobalLevel            - JobService
    - SetLoggerLevel            - CacheService
    - SetSinkLevel              - QueueService
    - EnableFileSink            - PeerService
```

## Integration

The daemon is integrated into the CLI at `cmd/buildozer-client/cmd/daemon.go`:

```go
server, err := daemon.NewServer(daemon.DaemonConfig{
    Host:              cfg.Daemon.Host,
    Port:              cfg.Daemon.Port,
    MaxConcurrentJobs: cfg.Daemon.MaxConcurrentJobs,
    MaxRAMMB:          cfg.Daemon.MaxRAMMB,
    EnableMDNS:        cfg.PeerDiscovery.Enabled,
})

if err := server.Start(); err != nil {
    return err
}
defer server.Stop(ctx)
```

## Service Registration Pattern

Services follow a consistent pattern for integration:

1. **Implement Handler Interface** - Connect service handler implementation
2. **Create Registration Function** - Returns `(path, http.Handler)`:
   ```go
   func RegisterMyService(config Config) (string, http.Handler) {
       handler := newMyServiceHandler(config)
       path, mux := protov1connect.NewMyServiceHandler(handler)
       return path, mux
   }
   ```
3. **Daemon Registers It** - Command or custom code calls:
   ```go
   path, handler := mypackage.RegisterMyService(config)
   daemon.RegisterServiceHandler(path, handler)
   ```

## Current Services

- **LoggingService** - Runtime logging configuration management

## Design Principles

1. **Separation of Concerns** - Each subsystem handles its domain; daemon orchestrates
2. **Composition** - Services are composed into daemon (not inheritance)
3. **Explicit Dependencies** - All dependencies explicitly injected
4. **Graceful Shutdown** - Clean context-based lifecycle management
5. **Thread-Safe** - Safe for concurrent access
6. **Testable** - Clean interfaces enable mocking

## Future Extensions

As development progresses, additional services will be registered:
- **RuntimeService** - Discover and manage available runtimes
- **JobService** - Submit, monitor, and manage build jobs
- **CacheService** - Query cache status, manage cache
- **QueueService** - Monitor job queue and scheduler
- **PeerService** - Peer discovery and management

Each service will follow the same registration pattern, making it easy to extend the daemon.

## Thread Safety Guarantees

The Daemon uses `sync.RWMutex` for thread-safe operations:
- `Start()` and `Stop()` can be called from different goroutines
- Service registration is safe during initialization
- Configuration and state queries use read locks

## Error Handling

- Daemon validates configuration on creation
- Start() returns errors if server fails to bind
- Stop() gracefully handles already-stopped state (idempotent)
- Service registration errors are logged and propagated

## Documentation

Complete documentation available in [pkg/daemon/README.md](README.md)
