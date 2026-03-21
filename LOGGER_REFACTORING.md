# Logger Refactoring: Dynamic Hierarchical Loggers

**Date:** 2026-03-21  
**Status:** ✅ Complete

## Problem Statement

The original logger implementation stored `Logger` instances in the registry as persistent objects. This required pre-creation and storage of all logger instances and made hierarchical name-based lookup complex. The desired behavior was:

1. Registry should **only hold sinks** (handlers)
2. **Logger instances created on-the-fly** based on hierarchy
3. **Hierarchical lookup** happens automatically during logger creation
4. **Child loggers** created with `.Child(name)` method

## Original Design Issues

```go
// BEFORE: Logger objects stored in registry
registry.GetLogger("buildozer") -> returns Logger instance
registry.loggers map[string]*Logger  // problematic storage

// Using the logger
logger := registry.GetLogger("buildozer")
logger.AddSink(sink)  // manual sink management
logger.SetLevel(level)  // logger stores level
logger.compositeHandler  // explicit handler storage
```

## New Design

```go
// AFTER: Only sinks stored, loggers created on-the-fly
registry.GetLogger("buildozer.package.module") -> creates Logger dynamically
registry.loggerConfigs map[string][]string  // logger name -> sink names
registry.sinks map[string]*Sink  // only store sinks

// Using the logger
logger := logging.log().Child("package").Child("module")
logger.Debug("message")  // logs via configured sinks
```

## Key Changes

### 1. Registry Structure

**Before:**
```go
type Registry struct {
    loggers map[string]*Logger  // Problematic: stores instance
    sinks   map[string]*Sink
}
```

**After:**
```go
type Registry struct {
    loggerConfigs map[string][]string  // logger name -> sink names
    sinks         map[string]*Sink     // sink name -> Sink
}
```

### 2. Logger Type

**Before:**
- Stored in registry as persistent object
- Had its own compositeHandler and level
- Managed sinks directly

**After:**
- Wraps `*slog.Logger` instance
- Created on-the-fly from registry configuration
- Holds hierarchical name and reference to registry

```go
type Logger struct {
    slogger  *slog.Logger    // Underlying slog logger
    name     string          // Hierarchical name (e.g., "buildozer.runtime")
    registry *Registry        // Configuration source
}
```

### 3. Logger Creation

**Before:**
```go
logger := registry.GetLogger("name")  // Retrieved/created if not exists
```

**After:**
```go
logger := registry.GetLogger("buildozer.package")  // Created on-the-fly
childLogger := logger.Child("module")  // Creates "buildozer.package.module"
```

### 4. Handler Resolution

**Before:**
- Handlers explicitly set on Logger instance
- No hierarchical lookup for handler configuration

**After:**
- `buildLoggerForHierarchy()` implements hierarchical lookup
- Exact match: "buildozer.package.module" 
- Parent match: "buildozer.package"  
- Root match: "buildozer"
- No match: empty logger (no sinks)

### 5. Configuration

**Before:**
```go
logger := registry.GetLogger("buildozer.package")
logger.SetLevel(slog.LevelDebug)
registry.AddSinkToLogger("buildozer.package", "stdout")
```

**After:**
```go
registry.SetLoggerSinks("buildozer.package", []string{"stdout", "file-debug"})
```

## Benefits

✅ **Cleaner Architecture:**
- Registry only manages sinks (the actual handlers)
- Loggers are ephemeral, created as needed
- No state duplication

✅ **Hierarchical by Default:**
- `logger.Child("name")` automatically appends to hierarchy
- Lookup happens automatically during logger creation
- No manual configuration needed for every logger

✅ **Dynamic:**
- Logger instances created on-the-fly
- No pre-allocation or caching needed
- Memory efficient

✅ **Simpler API:**
```go
// Old: need to manage registry and logger separately
logger := logging.get LoggerWithHierarchy("buildozer.package")

// New: simple method chain
logger := logging.get().Child("package").Child("module")
```

✅ **slog Integration:**
- Logger wraps `*slog.Logger`
- Full slog API available: Info(), Debug(), Log(), etc.
- Composable with slog middleware

## Files Modified

1. **pkg/logging/logger.go** (completely refactored)
   - Registry: removed `loggers` map, added `loggerConfigs`
   - Logger: now wraps `*slog.Logger`, created on-the-fly
   - Methods: `buildLoggerForHierarchy()`, `buildLoggerFromSinks()`, `Child()`
   - Removed: `GetLoggerWithHierarchy()`, logger-level SetLevel()

2. **pkg/logging/global.go**
   - Removed: `GetLoggerWithHierarchy()`
   - Updated: `EnableLoggerFileSink()`, `DisableLoggerFileSink()`
   - Updated: `GetAvailableLoggers()` to use loggerConfigs

3. **pkg/logging/config.go**
   - Updated: `InitializeFromConfig()` to use `SetLoggerSinks()`
   - Updated: `GetLoggerStatus()` to iterate loggerConfigs
   - Removed: composite handler creation (now in logger.go)

4. **pkg/logging/config_manager.go**
   - Updated: LoggerStatus removed `Level` field
   - Updated: `GetLoggingStatus()` to read from loggerConfigs

5. **pkg/logging/service_handler.go**
   - Updated: LoggerConfig creation doesn't set Level

6. **pkg/logging/remote_config.go**
   - Updated: LoggerStatus creation doesn't set Level

7. **buildozer/proto/v1/logging.proto**
   - Removed: `LogLevel level` field from LoggerConfig
   - Renumbered: sink_names field

## Usage Examples

### Simple Hierarchical Logger

```go
// pkg/runtimes/cpp/logger.go
package cpp

import "github.com/Manu343726/buildozer/pkg/logging"

func log() *logging.Logger {
    return logging.GetLogger("buildozer.runtime.cpp")
}

func childLogger(name string) *logging.Logger {
    return log().Child(name)
}
```

### Usage in Code

```go
// simple logging
log := logging.GetLogger("buildozer.package")
log.Debug("debug message")
log.Info("info", "key", "value")
log.Error("error", "error", err)

// hierarchical
log := logging.GetLogger("buildozer")
runtime := log.Child("runtime")
cpp := runtime.Child("cpp")
cpp.Debug("found compiler", "path", "/usr/bin/g++")
```

### Configuration

```yaml
loggers:
  - name: buildozer
    sinks: [stdout, stderr]
  - name: buildozer.runtime
    sinks: [stdout, file-runtime]
  - name: buildozer.runtime.cpp
    sinks: [file-cpp-debug]  # inherits from buildozer.runtime if not found
```

## Backward Compatibility

- `GetLogger(name)` works the same way but returns dynamic logger
- `Child(name)` is the new idiom for hierarchical access
- Removed methods: `GetLoggerWithHierarchy()`, `AddSinkToLogger()`, `RemoveSinkFromLogger()`
- Removed: logger-level level management (levels are sink-controlled)

## Testing

✅ Compilation verified:
- `go build ./pkg/logging` ✅
- `go build ./cmd/buildozer-client` ✅
- Full project builds ✅

## Next Steps

1. Update usage in codebase packages to use new `Child()` idiom
2. Update documentation with new usage patterns
3. Consider adding integration tests for hierarchical lookup
