# Buildozer Logging System

## Overview

The Buildozer logging system is built on top of Go's standard `log/slog` package, providing a flexible, hierarchical logging framework with configurable sinks, dynamic level management, and runtime configuration changes.

## Key Features

- **slog-based**: Uses Go's standard library `log/slog` for clean, structured logging
- **Hierarchical Loggers**: Loggers arranged in a hierarchy (e.g., `buildozer.runtime.detector`)
- **Configurable Sinks**: Multiple log destinations with independent level filtering (stdout, stderr, file, etc.)
- **Dynamic Level Changes**: Change logging levels at runtime without restarting
- **Logger-Specific File Sinks**: Enable debugging by capturing logs from specific components to files
- **Global Level Control**: Set a global minimum level affecting all loggers
- **Configuration File Support**: Define loggers and sinks in YAML config

## Architecture

### Core Components

1. **Logger**: A named logger that outputs to multiple sinks
   - Has a hierarchical name (e.g., `buildozer.runtime.cpp.native`)
   - Has its own level setting
   - Can have multiple sinks
   - Can create child loggers

2. **Sink**: A log destination with a slog Handler
   - Has a name, type (stdout/stderr/file), and level filter
   - Can be shared across multiple loggers
   - Filters messages by level

3. **Registry**: Manages all loggers and sinks
   - Provides hierarchical lookup for loggers
   - Manages global logging level
   - Tracks registered sinks

## Usage Examples

### Basic Logger Usage

```go
import "github.com/Manu343726/buildozer/pkg/logging"

// Get a logger
appLog := logging.GetLogger("buildozer")

// Log at different levels
appLog.Info("application started")
appLog.Debug("debug info", "key", "value")
appLog.Error("something went wrong", "error", err)
```

### Hierarchical Loggers (Package-Level)

Create a `logger.go` file in your package:

```go
// pkg/runtimes/cpp/native/logger.go
package native

import "github.com/Manu343726/buildozer/pkg/logging"

func Logger() *logging.Logger {
    return logging.GetLogger("buildozer.runtime.cpp.native")
}

func ChildLogger(name string) *logging.Logger {
    return Logger().Child(name)
}
```

Use in your code:

```go
// In .../native/executor.go
log := Logger()
log.Info("executor started")

// In .../native/detector.go
log := ChildLogger("detector")
log.Debug("detecting system runtimes")
```

### Dynamic Level Changes

```go
import "log/slog"

// Change global level
logging.SetGlobalLevel(slog.LevelDebug)

// Change specific logger level
logging.SetLoggerLevel("buildozer.runtime", slog.LevelDebug)

// Change sink level
logging.SetSinkLevel("stderr", slog.LevelWarn)
```

### Logger-Specific File Sinks (for debugging)

```go
// Enable a file sink for a specific logger
if err := logging.EnableLoggerFileSink(
    "buildozer.runtime",           // logger name
    "/tmp/runtime-debug.log",      // file path
    100,                            // max size in MB
); err != nil {
    log.Fatal(err)
}

// Later: disable the file sink
logging.DisableLoggerFileSink("buildozer.runtime")
```

### Using the CLI

The `buildozer-client logs` command manages logging:

```bash
# View current logging status
buildozer-client --standalone logs --status

# Set global level
buildozer-client --standalone logs --set-global-level debug

# Set logger level
buildozer-client --standalone logs --set-logger-level buildozer.runtime.detector --logger-level debug

# Change sink level
buildozer-client --standalone logs --set-sink-level stderr warn

# Enable debugging for a component
buildozer-client --standalone logs \
  --enable-file-sink buildozer.runtime.cpp.native \
  --file-sink-path /tmp/cpp-runtime-debug.log

# Disable the debugging file sink
buildozer-client --standalone logs --disable-file-sink buildozer.runtime.cpp.native
```

## Configuration File Format

### YAML Configuration

```yaml
logging:
  global_level: info  # error, warn, info, debug, trace
  
  sinks:
    - name: stdout
      type: stdout
      level: info
    
    - name: stderr
      type: stderr
      level: error
    
    - name: runtime_file
      type: file
      level: debug
      path: /var/log/buildozer/runtime.log
      max_size_b: 104857600  # 100 MB
      max_files: 5
      json_format: false
  
  loggers:
    - name: buildozer
      level: info
      sinks: [stdout, stderr]
    
    - name: buildozer.runtime
      level: debug
      sinks: [stdout, stderr, runtime_file]
    
    - name: buildozer.cache
      level: info
      sinks: [stdout]
```

## Log Levels (from slog)

```
Error   -  slog.LevelError    (highest priority)
Warn    -  slog.LevelWarn
Info    -  slog.LevelInfo      (default)
Debug   -  slog.LevelDebug
Trace   -  Level(-8)            (lowest priority)
```

## Implementation Details

### Hierarchical Lookup

When looking up a logger, the system searches:
1. Exact match: `buildozer.runtime.cpp.native`
2. Parent: `buildozer.runtime.cpp`
3. Grandparent: `buildozer.runtime`
4. Root: `buildozer`

This allows fine-grained control while defaulting to parent settings.

### Level Resolution

When logging, the effective level is the more restrictive (higher) of:
- Logger's level
- Global level

A message is logged if it passes the effective level AND the sink's level filter.

### Thread-Safe

All operations are protected with RWMutex locks:
- Logger and Sink levels can be changed from any goroutine
- Logging calls are thread-safe

## Integration with CLI

The `logs` command provides a full interface to the logging system:

**Flags:**
- `--status`: Show logger and sink configuration
- `--set-global-level <level>`: Change global level
- `--set-logger-level <name> --logger-level <level>`: Change logger level
- `--set-sink-level <name> --sink-level <level>`: Change sink level
- `--enable-file-sink <name> --file-sink-path <path>`: Enable debug file
- `--disable-file-sink <name>`: Disable debug file
- `--tail`: Stream logs (future implementation)

## Future Enhancements

1. **Log Streaming**: Real-time log tailing with filtering
2. **Remote Logging**: Configuration of remote daemon logs via RPC
3. **Log Aggregation**: Collect logs from multiple peers
4. **Syslog Support**: Full platform-specific syslog integration
5. **Metrics**: Integration with prometheus/openmetrics
6. **Sampling**: Rate limiting for high-volume logs
