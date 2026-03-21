# Logging Configuration Management Interface

## Overview

The logging package provides a unified configuration management interface (`ConfigManager`) with two implementations:

1. **LocalConfigManager** — Manages logging for the local application
2. **RemoteConfigManager** — Manages logging on a remote daemon via RPC

Additionally, there's a private service handler that implements the `LoggingServiceHandler` interface for the Connect RPC service.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         CLI / Application                        │
└──────────────┬──────────────────────────┬──────────────────────┘
               │                          │
      ┌────────▼─────────┐        ┌─────────▼──────────────┐
      │ LocalConfigManager│        │RemoteConfigManager     │
      │ (implements      │        │ (implements           │
      │ ConfigManager)   │        │ ConfigManager)        │
      └────────┬─────────┘        └─────────┬──────────────┘
               │                            │
               │ Direct API                 │ RPC via Connect
               │                            │
      ┌────────▼─────────────────────────▼─┐
      │   ConfigManager Interface          │
      │ - GetLoggingStatus()                │
      │ - SetGlobalLevel()                  │
      │ - SetLoggerLevel()                  │
      │ - SetSinkLevel()                    │
      │ - EnableFileSink()                  │
      │ - DisableFileSink()                 │
      │ - TailLogs()                        │
      └─────────────────────────────────────┘
               │
               │ Uses
               │
      ┌────────▼──────────────────────────────┐
      │   loggingServiceHandler               │
      │   (private implementation)            │
      │                                       │
      │ Implements LoggingServiceHandler      │
      │ RPC methods delegate to ConfigManager│
      └────────┬───────────────────────────────┘
               │
      ┌────────▼──────────────────────┐
      │ LoggingServiceHandler Interface│
      │ (Protocol Generated)          │
      │                               │
      │ Unary RPC Methods             │
      │ - GetLoggingStatus            │
      │ - SetGlobalLevel              │
      │ - SetLoggerLevel              │
      │ - SetSinkLevel                │
      │ - EnableFileSink              │
      │ - DisableFileSink             │
      │                               │
      │ Server Stream RPC Methods     │
      │ - TailLogs                    │
      └───────────────────────────────┘
```

## ConfigManager Interface

The `ConfigManager` interface defines the contract for logging configuration management:

```go
type ConfigManager interface {
    // Status Management
    GetLoggingStatus(ctx context.Context) (*LoggingStatusSnapshot, error)

    // Global Configuration
    SetGlobalLevel(ctx context.Context, level slog.Level) error

    // Logger-Specific Configuration
    SetLoggerLevel(ctx context.Context, loggerName string, level slog.Level) error

    // Sink-Specific Configuration
    SetSinkLevel(ctx context.Context, sinkName string, level slog.Level) error

    // File Sink Management
    EnableFileSink(ctx context.Context, loggerName, filePath string, maxSizeMB int, maxBackups int, maxAgeDays int) error
    DisableFileSink(ctx context.Context, loggerName string) error

    // Log Streaming
    TailLogs(ctx context.Context, logLevels []slog.Level, loggerFilter string, historyLines int) (<-chan *LogRecord, error)
}
```

## Using LocalConfigManager

For local in-process logging configuration:

```go
package main

import (
    "context"
    "log/slog"
    "github.com/Manu343726/buildozer/pkg/logging"
)

func main() {
    // Initialize the logging system
    config := logging.DefaultLoggingConfig()
    logging.InitializeGlobal(config)

    // Get the registry and factory
    registry := logging.GetRegistry()
    factory := logging.NewFactory(registry)

    // Create a local config manager
    localManager := logging.NewLocalConfigManager(registry, factory)

    // Use the config manager
    ctx := context.Background()
    status, err := localManager.GetLoggingStatus(ctx)
    if err != nil {
        panic(err)
    }

    // Change logging levels
    err = localManager.SetGlobalLevel(ctx, slog.LevelDebug)
    if err != nil {
        panic(err)
    }

    // Enable file sink for a logger
    err = localManager.EnableFileSink(ctx, "buildozer", "/var/log/buildozer.log", 100, 10, 30)
    if err != nil {
        panic(err)
    }
}
```

## Using RemoteConfigManager

For remote daemon logging configuration:

```go
package main

import (
    "context"
    "log/slog"
    "net/http"
    "github.com/Manu343726/buildozer/pkg/logging"
)

func main() {
    // Create HTTP client
    httpClient := &http.Client{}

    // Create remote config manager pointing to daemon
    remoteManager := logging.NewRemoteConfigManager(httpClient, "http://localhost:6789")

    // Use the config manager (same interface as local)
    ctx := context.Background()
    status, err := remoteManager.GetLoggingStatus(ctx)
    if err != nil {
        panic(err)
    }

    // Change logging levels on remote daemon
    err = remoteManager.SetGlobalLevel(ctx, slog.LevelDebug)
    if err != nil {
        panic(err)
    }

    // Stream logs from remote daemon
    logChan, err := remoteManager.TailLogs(ctx, []slog.Level{slog.LevelError, slog.LevelWarn}, "buildozer*", 100)
    if err != nil {
        panic(err)
    }

    for record := range logChan {
        println(record.Message)
    }
}
```

## Registering the RPC Service

To register the logging service handler with a Connect mux:

```go
package main

import (
    "net/http"
    "github.com/Manu343726/buildozer/pkg/logging"
)

func main() {
    // Create local config manager
    registry := logging.GetRegistry()
    factory := logging.NewFactory(registry)
    localManager := logging.NewLocalConfigManager(registry, factory)

    // Create HTTP mux and register logging service
    mux := http.NewServeMux()
    path, handler := logging.RegisterLoggingService(localManager)
    mux.Handle(path, handler)

    // Start server
    http.ListenAndServe(":6789", mux)
}
```

## Type Conversions

The logging package provides helper functions to convert between slog and protocol buffer types:

```go
// Convert slog.Level to protobuf LogLevel
protoLevel := logging.SlogLevelToProtoLogLevel(slog.LevelDebug)

// Convert protobuf LogLevel to slog.Level
slogLevel := logging.ProtoLogLevelToSlogLevel(v1.LogLevel_LOG_LEVEL_DEBUG)
```

## Data Types

### LoggingStatusSnapshot

```go
type LoggingStatusSnapshot struct {
    GlobalLevel slog.Level        // Global logging threshold
    Sinks       []SinkStatus      // All configured sinks
    Loggers     []LoggerStatus    // All configured loggers
    RetrievedAt time.Time         // When status was retrieved
}
```

### SinkStatus

```go
type SinkStatus struct {
    Name       string       // Sink name
    Type       string       // Type: stdout, stderr, file, syslog
    Level      slog.Level   // Minimum log level for this sink
    Path       string       // File path (file sinks only)
    JSONFormat bool         // JSON format flag (file sinks only)
}
```

### LoggerStatus

```go
type LoggerStatus struct {
    Name      string   // Logger name
    Level     slog.Level // Logger's configured level
    SinkNames []string // Sink names this logger outputs to
}
```

### LogRecord

```go
type LogRecord struct {
    Timestamp  time.Time
    LoggerName string
    Level      slog.Level
    Message    string
    Attributes map[string]string  // Structured attributes
}
```

## Files

- `config_manager.go` — ConfigManager interface and LocalConfigManager implementation
- `remote_config.go` — RemoteConfigManager implementation
- `service_handler.go` — Private loggingServiceHandler for RPC
- `config.go` — Existing factory and configuration types
- `logger.go` — Existing logger and registry types
- `global.go` — Existing global API functions

## Design Principles

1. **Interface-Based** — Single `ConfigManager` interface works with both local and remote implementations
2. **Connect RPC Ready** — Service handler implements generated `LoggingServiceHandler` interface
3. **Type Safety** — Uses enums and strong types instead of strings where possible
4. **Context-Aware** — All operations accept context for cancellation and timeout support
5. **Error Handling** — Proper error propagation with descriptive messages
6. **Private Service** — `loggingServiceHandler` is unexported; creation via `RegisterLoggingService()`

## Future Enhancements

- [ ] Circular buffer for TailLogs history
- [ ] Real-time log record streaming implementation
- [ ] Metrics/observability for configuration changes
- [ ] Configuration persistence/reloading
- [ ] Audit logging for configuration changes
