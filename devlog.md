# Buildozer Development Log

**Project:** Peer-to-Peer Distributed Build System  
**Status:** Phase 1 - Core Abstractions & Local Foundation  
**Last Updated:** 2026-03-22

---

## Implementation Updates - Source Location Flag for Sinks (2026-03-22)

### Added Source Location Control per Sink

**Status:** ✅ COMPLETED

**Objective:** Add a flag to enable/disable source location (file:line) information in log output, disabled by default per sink.

**Changes Made:**

1. **Proto Definition** (`buildozer/proto/v1/logging.proto`)
   - Added `include_source_location` boolean field to SinkConfig message
   - Field number 6 to avoid conflicts with existing fields
   - Applies to all sink types (stdout, stderr, file, syslog)

2. **Go Configuration Structures**
   - Added `IncludeSourceLocation bool` to `SinkConfig` (config.go)
   - Added `IncludeSourceLocation bool` to FileSinkConfig (sinks/sinks.go)
   - Added `IncludeSourceLoc bool` to Sink struct (logger.go)
   - Added `IncludeSourceLocation bool` to SinkStatus (config_manager.go)

3. **Handler Creation** (`pkg/logging/config.go`)
   - Updated `CreateSink()` to set `AddSource` in slog.HandlerOptions based on flag
   - Passes flag through to all sink creation calls
   - Flag is included in HandlerOptions for proper log formatting

4. **Status Reporting**
   - LocalConfigManager populates IncludeSourceLocation from Sink
   - RemoteConfigManager extracts from proto response and populates SinkStatus
   - Service handler populates proto SinkConfig with IncludeSourceLocation

5. **CLI Display** (`pkg/cli/logging.go`)
   - Status() method now shows `include_source_location: true/false` for each sink
   - Displayed for all sink types right after the level

**Default Behavior:**
- **Default:** `IncludeSourceLocation = false` (disabled)
- Source location (file:line) NOT included in logs by default
- Can be enabled per-sink via configuration

**Example Output:**

```
sinks:
  - stdout:
      type: stdout
      level: DEBUG
      include_source_location: false
  - daemon_file:
      type: file
      level: DEBUG
      include_source_location: false
      path: buildozer-daemon.log
      json_format: false
      max_size_mb: 100
      max_backups: 10
      max_age_days: 30
```

**Configuration Usage:**

```yaml
sinks:
  - name: verbose_file
    type: file
    level: debug
    path: /var/log/app-verbose.log
    include_source_location: true
  
  - name: stdout
    type: stdout
    level: info
    include_source_location: false  # or omitted (defaults to false)
```

**Testing Status:**
- ✅ Proto code generation successful
- ✅ Full build successful (go build ./...)
- ✅ Daemon initializes properly
- ✅ logs status shows include_source_location flag (false by default)
- ✅ Flag correctly flows through all layers (proto → config → handler)

**Files Modified:**
- buildozer/proto/v1/logging.proto - Added field to proto
- pkg/logging/config.go - Added field and updated CreateSink
- pkg/logging/logger.go - Added field to Sink struct
- pkg/logging/config_manager.go - Updated SinkStatus
- pkg/logging/sinks/sinks.go - Updated FileSinkConfig
- pkg/logging/remote_config.go - Extracts from proto
- pkg/logging/service_handler.go - Populates proto response
- pkg/cli/logging.go - Display in status output

**Key Architectural Decision:**
The flag is per-sink, not global. This allows:
- Production sinks (stdout) to run without source location overhead
- Debug/file sinks to include source location when needed
- Mix and match configurations in the same logger setup

---

## Implementation Updates - Full Sink Configuration Display (2026-03-22)

### Enhanced `logs status` to Show Complete Sink Configuration

**Status:** ✅ COMPLETED

**Objective:** Display full sink configuration including file path, format, and rotation settings when running `buildozer-client logs status`.

**Changes Made:**

1. **SinkStatus Struct Enhancement** (`pkg/logging/config_manager.go`)
   - Added fields: `MaxSize`, `MaxBackups`, `MaxAgeDays` (in addition to existing `Path` and `JSONFormat`)
   - Now captures complete file sink rotation configuration

2. **Sink Struct Update** (`pkg/logging/logger.go`)
   - Added fields to track rotation parameters: `MaxSize`, `MaxBackups`, `MaxAgeDays`, `JSONFormat`
   - Enables proper status reporting with full configuration details

3. **LocalConfigManager** (`pkg/logging/config_manager.go`)
   - Updated `GetLoggingStatus()` to populate all rotation fields from Sink struct
   - Now includes: Path, MaxSize, MaxBackups, MaxAgeDays, JSONFormat

4. **Factory** (`pkg/logging/config.go`)
   - Updated `CreateSink()` to populate all rotation fields in Sink
   - Extracts values from SinkConfig and stores them for later retrieval

5. **RemoteConfigManager** (`pkg/logging/remote_config.go`)
   - Updated sink conversion to extract FileConfig details from proto
   - Populates: Path, MaxSize, MaxBackups, MaxAgeDays, JSONFormat from proto response

6. **Service Handler** (`pkg/logging/service_handler.go`)
   - Updated `GetLoggingStatus()` to populate FileConfig in proto response
   - Now includes all rotation parameters in SinkConfig.FileConfig

7. **CLI Display** (`pkg/cli/logging.go`)
   - Enhanced Status() method to show complete file sink information
   - Displays: path, json_format, max_size_mb, max_backups, max_age_days
   - Conditionally shows file-specific fields only for file sinks with path set

**Example Output:**

```
sinks:
  - stdout:
      type: stdout
      level: DEBUG
  - daemon_file:
      type: file
      level: DEBUG
      path: buildozer-daemon.log
      json_format: false
      max_size_mb: 100
      max_backups: 10
      max_age_days: 30
```

**Testing Status:**
- ✅ Full build successful (go build ./...)
- ✅ Daemon starts and initializes logging config properly
- ✅ logs status shows complete sink configuration
- ✅ File sinks display path, format, and all rotation parameters
- ✅ stdout/stderr sinks show basic configuration

**Backward Compatibility:**
- Optional fields (MaxSize, MaxBackups, MaxAgeDays) default to 0 if not set
- CLI conditionally displays file-specific fields only when relevant
- RemoteConfigManager handles missing FileConfig gracefully

**Key Files Modified:**
- pkg/logging/logger.go - Added fields to Sink struct
- pkg/logging/config_manager.go - Updated SinkStatus and LocalConfigManager
- pkg/logging/config.go - Updated Factory.CreateSink
- pkg/logging/remote_config.go - Updated proto conversion
- pkg/logging/service_handler.go - Updated GetLoggingStatus proto population
- pkg/cli/logging.go - Enhanced Status() display

---

## Implementation Updates - Default Sinks for New Loggers (2026-03-22)

### Automatic Default Sinks for New Loggers

**Status:** ✅ IMPLEMENTED

**Objective:** Automatically attach standard sinks to newly created loggers when no sinks are explicitly specified.

**Key Change:**

**AddLogger Enhancement** ([pkg/logging/config_manager.go](pkg/logging/config_manager.go))
- When creating a new logger **without explicitly specifying sinks**, automatically attaches default sinks
- **Default sinks** (in order of preference):
  1. `"stdout"` - If exists in registry
  2. `"buildozer-daemon.log"` - If exists in registry
- If no sinks are explicitly specified, attempts to use available defaults
- If some defaults don't exist, only uses the ones that do
- If no default sinks exist in the registry, logs a warning and creates logger with no sinks

**Usage Examples:**

```bash
# Without --sinks flag: automatically adds stdout and buildozer-daemon.log (if they exist)
buildozer-client logs add-logger my-logger
# Result: my-logger attached to [stdout, buildozer-daemon.log]

# With --sinks flag: uses only the specified sinks (bypasses defaults)
buildozer-client logs add-logger my-logger --sinks stderr,my-custom-sink
# Result: my-logger attached to [stderr, my-custom-sink] (no defaults added)

# If stdout doesn't exist: uses only available defaults
buildozer-client logs add-logger my-logger  # (only stderr and buildozer-daemon.log exist)
# Result: my-logger attached to [buildozer-daemon.log]
```

**Implementation Details:**

```go
// getDefaultSinks() helper method:
// 1. Maintains ordered list: ["stdout", "buildozer-daemon.log"]
// 2. Checks which ones exist in registry
// 3. Returns only the ones that exist
// 4. Used only when sinkNames is empty

// AddLogger logic:
if len(sinkNames) == 0 {
    sinkNames = m.getDefaultSinks()  // Apply defaults
    // Log warning if no defaults available
} else {
    // Use explicitly specified sinks as-is
}
```

**Design Rationale:**

1. **Sensible Defaults**: Most new loggers should output somewhere (stdout by default is standard)
2. **Opt-out via Flags**: Users can explicitly specify `--sinks ""` or specific sinks to bypass defaults (via CLI parsing)
3. **Safe Defaults**: Only adds sinks that actually exist in registry (no phantom sinks)
4. **Logging**: Warns if no default sinks available to help user understand the configuration
5. **Explicit Control**: Users who need different behavior can always remove/detach sinks after creation

**Backward Compatibility:**
- CLI: `add-logger` with `--sinks` flag works exactly as before
- Code: `AddLogger()` with explicit sinkNames works exactly as before
- New Behavior: Applies only when sinkNames is empty (which was previously creating loggers with no sinks)

**Testing Status:**
- ✅ Project compiles without errors
- ✅ All logging tests pass
- ✅ No regressions in functionality

**Typical Setup Sequence:**

```bash
# 1. Create stdout sink
buildozer-client logs add-sink stdout

# 2. Create daemon file sink
buildozer-client logs enable-file-sink daemon /var/log/buildozer-daemon.log

# 3. Create new logger (automatically attached to above sinks)
buildozer-client logs add-logger my-component

# 4. Verify logger has both sinks attached
buildozer-client logs status
```

---

## Implementation Updates - Implicit Sink Names for stdout/stderr (2026-03-22)

### Simplified CLI: Implicit Sink Names for Standard Sinks

**Status:** ✅ IMPLEMENTED

**Objective:** Simplify the `add-sink` command by using implicit sink names for stdout and stderr, eliminating the redundant name argument.

**Key Change:**

**CLI Command Update** ([cmd/buildozer-client/cmd/logging.go](cmd/buildozer-client/cmd/logging.go))
- **Before**: `buildozer-client logs add-sink <sink-name> <type>` (2 arguments required)
- **After**: `buildozer-client logs add-sink <type>` (1 argument, type only)
- The sink name is implicitly set to the type name:
  - `add-sink stdout` → Creates sink named "stdout" of type "stdout"
  - `add-sink stderr` → Creates sink named "stderr" of type "stderr"

**Rationale:**
- stdout and stderr are unique sinks (only one can exist of each type)
- The name and type are always identical for these standard sinks
- Removes unnecessary redundancy in the CLI interface
- Makes the command more intuitive and less verbose

**Usage Examples (Updated):**

```bash
# Create stdout sink (implicit name = "stdout")
buildozer-client logs add-sink stdout

# Create stderr sink (implicit name = "stderr")
buildozer-client logs add-sink stderr

# Remove the stdout sink
buildozer-client logs remove-sink stdout

# Attach stdout to a logger
buildozer-client logs attach-sink my-logger stdout
```

**Code Flow:**
```
CLI Input: "add-sink stdout"
    ↓
Parse as sinkType = "stdout"
    ↓
Set sinkName = sinkType (implicit)
    ↓
Call commands.AddSink("stdout", "stdout", slog.LevelInfo)
    ↓
ConfigManager validates no overlap
```

**Testing Status:**
- ✅ CLI client builds successfully
- ✅ All logging tests pass
- ✅ No regressions in functionality

**Related Commands** (Unchanged):
- `remove-sink <sink-name>` - Still requires sink name
- `attach-sink <logger-name> <sink-name>` - Still requires sink name (use "stdout" or "stderr")
- `detach-sink <logger-name> <sink-name>` - Still requires sink name
- `enable-file-sink <logger-name> <file-path>` - File sinks use explicit named sinks

---

## Implementation Updates - AddSink Unified Overlap Detection (2026-03-22)

### Unified AddSink with Smart Overlap Detection

**Status:** ✅ IMPLEMENTED

**Objective:** Enable AddSink to handle all sink types (stdout, stderr, file) with intelligent overlap detection based on sink characteristics.

**Key Changes:**

1. **Sink Struct Enhancement** (`pkg/logging/logger.go`)
   - Added `FilePath string` field to track file paths for file sinks
   - Enables detecting overlaps when multiple file sinks point to same file

2. **AddSink Implementation** (`pkg/logging/config_manager.go`)
   - Now accepts all sink types: `stdout`, `stderr`, `file`
   - Smart overlap detection:
     - **stdout**: Error if stdout sink already exists
     - **stderr**: Error if stderr sink already exists
     - **file**: Error if file sink already points to same file path
   - Unified error messages with overlap details (shows overlapping sink name)
   - Internal: `addSinkInternal(ctx, name, type, filePath, level)` handles file path
   - Public: `AddSink(ctx, name, type, level)` for stdout/stderr (no file path needed)

3. **Service Handler** (`pkg/logging/service_handler.go`)
   - AddSink RPC still restricted to stdout/stderr (protobuf lacks file path support)
   - Clear error message if file sink requested: "use EnableFileSink for file sinks"
   - Returns error code `CodeInvalidArgument` instead of `CodeInternal` for validation errors

4. **EnableFileSink** - Updated to use new internal method
   - Uses `addSinkInternal()` for file sink creation with path
   - Maintains shorthand semantics: create + attach + cleanup old

**Overlap Detection Examples:**

**Error Cases:**
```bash
# Error: stdout already exists
buildozer-client logs add-sink stdout1 stdout  # ✓ succeeds
buildozer-client logs add-sink stdout2 stdout  # ✗ error: "a stdout sink already exists (overlap with stdout1)"

# Error: stderr already exists  
buildozer-client logs add-sink stderr1 stderr  # ✓ succeeds
buildozer-client logs add-sink stderr2 stderr  # ✗ error: "a stderr sink already exists (overlap with stderr1)"

# Error: file sink for same path exists
buildozer-client logs enable-file-sink logger1 /var/log/app.log  # ✓ succeeds
buildozer-client logs enable-file-sink logger2 /var/log/app.log  # ✗ error: "a file sink for '/var/log/app.log' already exists"
```

**Successful Operations:**
```bash
# Different files = no overlap
buildozer-client logs enable-file-sink logger1 /var/log/app1.log  # ✓ succeeds
buildozer-client logs enable-file-sink logger2 /var/log/app2.log  # ✓ succeeds

# Multiple loggers to same file = error (intended)
# This is the overlap detection at work - prevents two loggers writing to same file
```

**Architecture:**

```
Code Level (ConfigManager):
  AddSink(name, type, level)                    # Works for stdout, stderr, "file" type errors
  addSinkInternal(name, type, filePath, level)  # Supports all types with file path
  
RPC Level (Service Handler):
  AddSinkRequest(name, type, level)             # Protobuf doesn't support file path
  AddSink RPC → AddSink code → stdout/stderr only
  
File Sinks:
  EnableFileSink RPC → addSinkInternal → full file sink creation with validation
```

**Design Rationale:**

1. **Overlap Detection Granularity**: Different logic for different sink types
   - stdout/stderr: Single instance per type (mutual exclusion)
   - file: Path-based uniqueness (same file can't be a sink twice)
   
2. **RPC/Code Flexibility**:
   - Code level: Fully capable of handling file sinks with overlap detection
   - RPC level: Restricted by protobuf message schema; file sinks via EnableFileSink
   - Future: Can enhance protobuf if needed for direct file sink creation via RPC

3. **Error Clarity**: Messages identify which sink is causing the overlap:
   - "overlap with {sinkName}" helps users understand what's already registered

**Testing Status:**
- ✅ Project builds without errors
- ✅ All logging tests pass
- ✅ CLI client builds successfully
- ✅ No regressions in existing functionality

**Migration Path (if updating existing code):**
- Code that called `AddSink()` with file type: Now correctly errors
- Use `addSinkInternal()` or `EnableFileSink()` for file sinks
- Existing stdout/stderr creation via `AddSink()` works unchanged

---

## Implementation Updates - AddSink/EnableFileSink/DisableFileSink Refinements (2026-03-22)

### Sink Management Commands: Proper Validation and Shorthand Semantics

**Status:** ✅ IMPLEMENTED

**Objective:** Clarify and refine the semantics of sink management commands with proper validation and shorthand behavior.

**Key Changes:**

1. **AddSink Command** - Only for stdout/stderr sinks
   - Now **rejects file sinks** with clear error message
   - Returns error if a **stdout or stderr sink already exists** (prevents duplicates)
   - File sink creation must use `EnableFileSink` instead
   - Validation: `AddSink(ctx, "my-stdout", "stdout", level)` ✓
   - Validation: `AddSink(ctx, "my-file", "file", level)` ✗ Error: "use EnableFileSink instead"
   - Validation: Creating second stdout sink ✗ Error: "a stdout sink already exists"

2. **EnableFileSink Command** - Proper Shorthand Implementation
   - Creates a rotating file sink with **automatic naming**: `"file-{loggerName}"`
   - Automatically **attaches the sink only to that logger**
   - Smart cleanup: If logger already has a file sink attached, **removes the old one first**
   - Use case: Redirect logger output to file on-the-fly without manual sink management
   - Example: `EnableFileSink("my-logger", "/var/log/my-logger.log", 100, 10, 30)`
     - Creates sink named `"file-my-logger"`
     - Attaches to logger `"my-logger"`
     - Removes any previous file sink from the logger
     - Sets rotation: 100MB size limit, 10 backup files, 30 day retention

3. **DisableFileSink Command** - Proper Shorthand Implementation
   - Finds **automatically named file sink**: `"file-{loggerName}"`
   - **Detaches and removes** the sink from registry
   - Cleanup: Removes the sink so it's not reused elsewhere
   - Example: `DisableFileSink("my-logger")`
     - Finds and detaches `"file-my-logger"` from `"my-logger"`
     - Removes `"file-my-logger"` from sink registry entirely

**Implementation Details:**

**Files Modified:**
- `pkg/logging/config_manager.go`:
  - Updated `AddSink()` to validate sink types and reject duplicates
  - Added `AddFileSink()` internal method for creating file sinks with proper rotation
  - Refactored `EnableFileSink()` as proper shorthand: create + attach + cleanup old
  - Refactored `DisableFileSink()` as proper shorthand: find + detach + remove
  - Added import for `"github.com/Manu343726/buildozer/pkg/logging/sinks"`

- `pkg/logging/logger.go`:
  - Added `GetAllSinks()` method to Registry for validation checks across all sinks
  - Thread-safe with RWMutex for concurrent access

- `pkg/logging/service_handler.go`:
  - Updated `AddSink()` handler to enforce stdout/stderr only, reject file sinks
  - Added validation with clear error messages
  - Added import for `"fmt"` package

**Architecture Pattern:**

```
AddSink (stdout/stderr only)
  └─ Check no duplicate stdout/stderr exists
  └─ Create and register sink
  └─ NOT attached to any logger (manual attachment via AttachSink)

EnableFileSink (shorthand for logger redirect)
  └─ Detach old file sink from logger (if exists)
  └─ Remove old file sink from registry
  └─ Create file sink named "file-{loggerName}"
  └─ Attach to logger
  └─ Logger now outputs to file (automatically formatted as rotating)

DisableFileSink (shorthand to stop logger redirect)
  └─ Find file sink "file-{loggerName}"
  └─ Detach from logger
  └─ Remove from registry
  └─ Logger no longer outputs to that file
```

**Testing Status:**
- ✅ Project builds without errors
- ✅ All logging tests pass
- ✅ CLI client builds successfully
- ✅ No regressions in existing functionality

**Validation Examples:**

Failed Operations (with error messages):
```bash
# Error: prevent duplicate stdout sinks
buildozer-client logs add-sink stdout1 stdout   # ✓ succeeds
buildozer-client logs add-sink stdout2 stdout   # ✗ error: "a stdout sink already exists"

# Error: use EnableFileSink for file sinks
buildozer-client logs add-sink my-file file     # ✗ error: "use EnableFileSink instead"

# Error: logger or sink not found
buildozer-client logs enable-file-sink unknown /var/log/test.log  # ✗ error: "logger not found"
```

Successful Operations:
```bash
# Create stdout sink
buildozer-client logs add-sink my-stdout stdout

# Redirect logger to file (replaces old file sink if exists)
buildozer-client logs enable-file-sink my-logger /var/log/my-logger.log

# Stop file redirection
buildozer-client logs disable-file-sink my-logger

# Manual sink management (for advanced use cases)
buildozer-client logs add-sink custom stderr
buildozer-client logs attach-sink my-logger custom
```

---

## Implementation Updates - Logging Service AttachSink/DetachSink (2026-03-22)

### Logging Service RPC Methods for Dynamic Sink Management

**Status:** ✅ IMPLEMENTED

**Objective:** Enable dynamic attachment and detachment of logging sinks to/from loggers via RPC API.

**Components Completed:**

1. **Protobuf Message Definitions (`buildozer/proto/v1/logging.proto`)**
   - Added `AttachSinkRequest` - attaches existing sink to logger
   - Added `AttachSinkResponse` - confirms attachment and returns updated sink list
   - Added `DetachSinkRequest` - detaches sink from logger
   - Added `DetachSinkResponse` - confirms detachment and returns remaining sinks
   - All messages include timestamps and success messages

2. **Service Handler Implementation (`pkg/logging/service_handler.go`)**
   - `AttachSink()` handler - validates logger exists, attaches sink, returns updated config
   - `DetachSink()` handler - validates logger exists, detaches sink, returns updated config
   - Both handlers fetch updated logger status after modification for consistency
   - Proper error handling with Connect protocol codes (CodeInternal)
   - Debug logging for operation tracking

3. **Remote Config Manager Implementation (`pkg/logging/remote_config.go`)**
   - `AttachSink()` - calls remote `AttachSink` RPC with proper error handling
   - `DetachSink()` - calls remote `DetachSink` RPC with proper error handling
   - Replaced placeholder "not yet implemented" messages with actual RPC calls
   - Debug logging for remote operations

4. **Registry Methods (Already Implemented in `pkg/logging/logger.go`)**
   - `AttachSink(loggerName, sinkName)` - validates sink exists, checks for duplicates, appends
   - `DetachSink(loggerName, sinkName)` - finds and removes sink, error if not found
   - Both methods use RWMutex for thread-safe operations
   - Proper validation and error messages

5. **CLI Commands (Already Implemented)**
   - `logs attach-sink <logger-name> <sink-name>` - CLI command to attach sink
   - `logs detach-sink <logger-name> <sink-name>` - CLI command to detach sink
   - Commands integrated in logging command help text

6. **CLI Handler Methods (`pkg/cli/logging.go`)**
   - `AttachSink(loggerName, sinkName)` - calls config manager, logs success
   - `DetachSink(loggerName, sinkName)` - calls config manager, logs success
   - Both use context.Background() and proper error propagation

**Usage Examples:**

```bash
# Attach an existing sink to a logger
buildozer-client logs attach-sink my-logger my-sink

# Detach a sink from a logger
buildozer-client logs detach-sink my-logger my-sink

# View current sinks attached to loggers
buildozer-client logs status
```

**Architecture:**

The implementation follows the established pattern:
1. CLI command → LoggingCommands method → ConfigManager method → RemoteConfigManager or LocalConfigManager
2. Remote operations call Connect RPC to daemon service
3. Local operations call Registry methods with thread-safe locks
4. All operations include proper error handling and logging

**Testing Status:**
- ✅ Project builds without errors
- ✅ All existing tests pass
- ✅ Protobuf code regenerated successfully
- ✅ No regressions in functionality

**Files Modified:**
- `buildozer/proto/v1/logging.proto` - Added AttachSinkRequest/Response and DetachSinkRequest/Response messages
- `pkg/logging/service_handler.go` - Added AttachSink and DetachSink handler implementations
- `pkg/logging/remote_config.go` - Implemented remote AttachSink and DetachSink RPC calls

**Next Steps:**
- Create integration tests for attach/detach operations
- Add user documentation for sink management commands
- Monitor for any edge cases in production usage

---

## Design Decisions - Logging Architecture (2026-03-22)

### Logger Hierarchy with Embedded Component Loggers

**Status:** ✅ DOCUMENTED

**Decision:** Establish hierarchical logging with embedded loggers in components for cleaner code.

**Pattern:**
1. Every package has `logger.go` with `Log()` function returning package logger
2. Package loggers are children of root logger: `logging.Log().Child("package_name")`
3. Each component (struct like `httpServer`) embeds its logger as unnamed field `*logging.Logger`
4. Components can call logging methods directly: `hs.Debug("message")` instead of `hs.logger.Debug("message")`
5. Components return errors using `.Errorf(format, args...)` which both logs AND returns the error

**Rationale:**
- Clean logging API throughout components
- Proper hierarchical structure for remote log queries
- Method promotion via embedding reduces verbosity
- Logger lives with component, easy to track ownership
- `.Errorf()` ensures all errors are logged while maintaining standard error return pattern
- Example: `daemon.Log().Child("httpServer")` embedded in httpServer struct

**Implementation pattern:**
```go
type httpServer struct {
    *logging.Logger  // unnamed embedded for method promotion
    config DaemonConfig
}

// In constructor:
return &httpServer{
    Logger: daemon.Log().Child("httpServer"),
    config: config,
}

// Usage - methods are promoted:
hs.Info("starting server")  // Works directly on httpServer
if err != nil {
    return hs.Errorf("failed to listen: %w", err)  // Both logs and returns error
}
```

### Stack Depth Fix: Wrapper Methods (Errorf & Panicf) - (2026-03-22)

**Status:** ✅ IMPLEMENTED

**Issue:** Logger.Errorf() and Logger.Panicf() wrapper methods were incorrectly reporting the caller location. When you called `errorf("msg")` from a user function, the log would show the error came from inside Errorf() rather than from the actual call site.

**Root Cause:** Calling `l.Error(msg)` from within a wrapper delegates to slog, which uses `runtime.Caller()` with a fixed skip count. This skip count doesn't account for custom wrapper functions added above slog's internal callstack.

**Solution:** Instead of calling `l.Error(msg)`, use `runtime.Callers()` to manually capture the actual caller's program counter, then create a `slog.Record` with that PC and call the handler directly.

**Implementation:**
- Created private helper `logAtCallSite(skipFrames, level, msg)` method
- Uses `runtime.Callers(2+skipFrames, ...)` to get PC of actual caller (skips this method + wrapper method)
- Creates `slog.Record` with the correct PC value
- Calls `l.Handler().Handle(ctx, record)` directly
- Errorf/Panicf now call `logAtCallSite(1, level, msg)` 
- Result: Logs show the exact line where Errorf/Panicf was called, not wrapper internals

**Code Pattern:**
```go
func (l *Logger) Errorf(format string, args ...any) error {
    return l.logAtCallSite(1, slog.LevelError, fmt.Sprintf(format, args...)).(error)
}

func (l *Logger) Panicf(format string, args ...any) {
    msg := fmt.Sprintf(format, args...)
    l.logAtCallSite(1, slog.LevelError, msg)
    panic(msg)
}

func (l *Logger) logAtCallSite(skipFrames int, level slog.Level, msg string interface{} {
    var pcs [1]uintptr
    runtime.Callers(2+skipFrames, pcs[:])  // Skip: logAtCallSite + wrapper + skipFrames
    
    r := slog.Record{
        Time:    time.Now(),
        Level:   level,
        PC:      pcs[0],      // Use actual caller's PC
        Message: msg,
    }
    l.Handler().Handle(context.Background(), r)
    
    if level == slog.LevelError {
        return errors.New(msg)
    }
    return nil
}
```

**Testing:** All existing logging tests pass. Stack traces now correctly show user code location.

---

## Implementation Updates - Component Logger Pattern Refactoring (2026-03-22)

### Component Logger Embedding Pattern Applied to All Components

**Status:** ✅ COMPLETED

**Objective:** Ensure all major components in Buildozer follow the component logger embedding pattern established in the logging architecture design.

**Components Refactored:**

1. **`pkg/daemon/daemon.go` - HTTP Server Component (httpServer)**
   - **Before:** Used explicit `logger` field: `hs.logger.Info(...)`
   - **After:** Embedded `*logging.Logger` field for method promotion: `hs.Info(...)`
   - **Changes:**
     - Removed explicit `logger` field
     - Added unnamed embedded `*logging.Logger` field to httpServer struct
     - Updated constructor: `Logger: Log().Child("httpServer")`
     - Replaced all `slog.Debug/Info/Error` calls with direct methods: `hs.Debug(...)`
     - Replaced all `fmt.Errorf` with `hs.Errorf()` for automatic logging
     - Removed unused `fmt` and `slog` imports
   - **Methods Updated:** All HTTP/Connect server setup and teardown paths
   - **Benefit:** Cleaner logging API, automatic error logging, hierarchical context

2. **`pkg/cli/daemon.go` - Daemon Command Handler (DaemonCommands)**
   - **Before:** Used explicit `logger` field: `dc.logger.Info(...)`
   - **After:** Embedded `*logging.Logger` field: `dc.Info(...)`
   - **Changes:**
     - Removed explicit `logger` field (was `logger *logging.Logger`)
     - Added unnamed embedded `*logging.Logger` field
     - Updated constructor: `Logger: Log().Child("DaemonCommands")`
     - Replaced all `dc.logger.Info()` with `dc.Info()` (method promotion)
     - Replaced all `fmt.Errorf` with `dc.Errorf()` for automatic logging
   - **Methods Updated:** Start() (daemon startup, signal handling, shutdown logging)
   - **Key Method Behavior:**
     - Start() logs daemon startup, handles signals (SIGTERM, SIGINT), logs graceful shutdown
     - All error returns now use Errorf() for automatic logging

3. **`pkg/cli/logging.go` - Logging Command Handler (LoggingCommands)**
   - **Before:** Used explicit `logger` field: `lc.logger.Info(...)`
   - **After:** Embedded `*logging.Logger` field: `lc.Info(...)`
   - **Changes:**
     - Removed explicit `logger` field
     - Added unnamed embedded `*logging.Logger` field
     - Updated constructor: `Logger: Log().Child("LoggingCommands")`
     - Replaced 8 instances of `lc.logger.Info()` with `lc.Info()`
     - Replaced all `fmt.Errorf` with `lc.Errorf()` for automatic logging
   - **Methods Updated:** Status(), Tail(), SetGlobalLevel(), SetLoggerLevel(), SetSinkLevel(), EnableFileSink(), DisableFileSink()
   - **Benefit:** All logging operations have automatic context tracking and error logging

4. **`pkg/cli/config.go` - Configuration Command Handler (ConfigCommands)**
   - **Before:** Used explicit `logger` field: `cc.logger.Info(...)`
   - **After:** Embedded `*logging.Logger` field: `cc.Info(...)`
   - **Changes:**
     - Removed explicit `logger` field
     - Added unnamed embedded `*logging.Logger` field
     - Updated constructor: `Logger: Log().Child("ConfigCommands")`
     - Replaced `cc.logger.Info()` calls with `cc.Info()`
     - Replaced `fmt.Errorf` with `cc.Errorf()`
   - **Methods Updated:** All config command implementations

**Pattern Verification:**

All refactored components now:
- ✅ Embed `*logging.Logger` as unnamed field
- ✅ Initialize logger with component name: `Log().Child("ComponentName")`
- ✅ Use direct method calls via promotion: `c.Debug()`, `c.Info()`, `c.Error()`, `c.Errorf()`
- ✅ Use `Errorf()` for all error returns to ensure logging
- ✅ Log entry points with Debug() and relevant context
- ✅ Log successful completions in Debug() calls
- ✅ Create hierarchical logger structure visible in logs

**Build Status:**
```bash
$ go build ./...
# SUCCESS - All packages compile without errors
```

**Testing Status:**
- All existing logging tests pass
- Full project builds successfully
- CLI commands work correctly with new logging pattern
- No regressions in functionality

**Logging Hierarchy Created:**
```
daemon                        (from pkg/daemon/logger.go)
├── httpServer              (embedded in daemon HTTP server)

cli                           (from pkg/cli/logger.go)
├── DaemonCommands          (CLI daemon command handler)
├── LoggingCommands         (CLI logging command handler)
└── ConfigCommands          (CLI config command handler)
```

**Benefits Realized:**
1. **Consistency:** All components follow identical logging pattern
2. **Auditability:** All errors automatically logged before returning
3. **Method Promotion:** Cleaner syntax reduces verbosity and indentation
4. **Hierarchical Tracking:** Log hierarchy makes component relationships visible
5. **Remote Querying:** Future API can query logs by component path: `daemon.httpServer.*`
6. **Testing:** Embedded loggers easier to mock in unit tests
7. **Maintenance:** New components can follow established pattern with minimal boilerplate

**Reference Implementation:** All refactored components follow pattern established by `pkg/logging/remote_config.go` (ConfigManager).

**Files Modified:**
- pkg/daemon/daemon.go (httpServer struct)
- pkg/cli/daemon.go (DaemonCommands struct)
- pkg/cli/logging.go (LoggingCommands struct)
- pkg/cli/config.go (ConfigCommands struct)

**Next Steps:**
- As new components are added (scheduler, cache manager, peer discovery, etc.), apply same pattern
- Update code review guidelines to enforce pattern
- Consider adding lint rule to detect components violating pattern

---

## Phase 1: Core Abstractions & Local Foundation (Weeks 1-5)

### Architecture Decision: Runtime-First Approach (2026-03-21)

**Status:** ✅ PLANNED

**Key Insight:** Runtime system is the foundational abstraction that all other Phase 1 components depend on. Build runtime discovery, Docker API, and execution abstractions FIRST (Milestones 1.0-1.2), then build job abstractions on proven runtime system.

**Updated Phase 1 Structure (10 Milestones, 5 Weeks):**
- **Milestone 1.0-1.2 (Weeks 1-3):** Runtime Foundation (Docker API, native C/C++ toolchains, Docker-based runtimes)
- **Milestone 1.3-1.9 (Weeks 3-5):** Job abstractions, logging, persistence, executor, daemon, drivers

**Benefits:**
- Execution logic built on proven, tested runtime system
- No architectural refactoring when adding P2P scheduling (Phase 4)
- Docker runtime design enables code reuse (native logic executes in containers)
- Smart image tagging with full toolchain metadata enables reproducible builds

**Third-Party Dependency - Docker Go API:**
- **Official library:** `github.com/docker/docker/client` (Docker's official Go SDK)
- **Not custom:** We implement a thin abstraction wrapper over Docker API; do NOT write custom Docker client driver
- **Why wrapper:** Isolates Docker specifics, easier to test/mock, cleaner in job executor code
- **Docker client setup:** Initialize once at daemon startup; reuse for all image/container operations

---

### Milestone 1.0: Docker API Abstraction Implementation (2026-03-21)

**Status:** ✅ DOCUMENTED (ready for implementation)

**Key Architectural Decisions:**

1. **Embedded Dockerfile Templates (Not External Files)**
   - All predefined Dockerfile templates **embedded in the binary** via Go `embed` package
   - No external Dockerfile files needed for deployment
   - Examples of embedded templates:
     - `ubuntu-gcc-11-glibc-2.35.Dockerfile`
     - `ubuntu-gcc-12-glibc-2.36.Dockerfile`
     - `ubuntu-clang-14-glibc-2.35.Dockerfile`
     - `alpine-gcc-11-musl-1.2.3.Dockerfile`
     - And more for different compiler/cruntime/architecture combinations
   - **Binary portability:** Deploy single buildozer binary to any system; it builds required runtimes on first use

2. **On-Demand Image Building**
   - When job requests runtime: `buildozer-c-gcc-11-x86_64-glibc-2.35`
   - Detector checks if image already exists in Docker daemon
   - If missing:
     1. Load embedded Dockerfile template from binary
     2. Build image via Docker API
     3. Tag with canonical name
     4. Cache in local Docker daemon
   - If exists: Use immediately (fast path, no rebuild)
   - **Result:** First job requesting a runtime triggers build; subsequent jobs use cached image

3. **Predefined Docker Images with Common Toolchains**
   - Covers: gcc-11/12, clang-14/15, glibc/musl, x86_64/aarch64, various versions
   - Combinations selected for common use cases (C/C++ development)
   - Each image tagged twice: C and C++ variants use same underlying image

4. **Comprehensive Docker Image Tagging with Canonical Compiler Names**
   - Tag format: `buildozer-<language>-<compiler>-<version>-<arch>-<cruntime>-<cruntimever>`
   - **Canonical naming:** Use "gcc" in tag (not g++), "clang" (not clang++)
   - Examples:
     - `buildozer-c-gcc-11-x86_64-glibc-2.35` (C with gcc-11)
     - `buildozer-cxx-gcc-11-x86_64-glibc-2.35` (C++ with gcc-11, same image)
     - `buildozer-cxx-clang-14-x86_64-glibc-2.35` (C++ with clang-14)
   - **Rationale:** gcc/g++ are same compiler; language field determines driver selection

5. **Smart Image Reuse Pattern**
   - One Docker image with gcc-11 + g++-11 provides TWO runtimes:
     ```
     buildozer-c-gcc-11-x86_64-glibc-2.35      (uses gcc driver)
     buildozer-cxx-gcc-11-x86_64-glibc-2.35    (uses g++ driver)
     ```
   - Both tags point to same image (zero duplication)
   - Job language field determines which driver (gcc vs g++) to invoke

6. **Metadata-Driven Matching**
   - Job runtime spec: (language=c, compiler=gcc, ver=11, arch=x86_64, cruntime=glibc, ver=2.35)
   - Docker detector parses image tags → Extracts full toolchain metadata
   - Runtime matcher finds exact Docker image by complete metadata match
   - Enables precise, reproducible job-to-runtime matching

**Implementation Files:**
- `pkg/runtimes/cpp/docker/dockerfiles/` — Embedded Dockerfile templates (via `embed` package)
- `pkg/runtimes/cpp/docker/dockerfile_builder.go` — Load embedded templates, on-demand build, caching
- `pkg/runtimes/cpp/docker/docker_cpp_runtime.go` — Docker runtime implementing Runtime interface
- `pkg/runtimes/cpp/docker/detector.go` — Scan images, auto-build if missing, parse metadata, register runtimes

**Deployment Benefit:**
- **Single unit of deployment:** buildozer binary contains Dockerfiles
- **No setup required:** Run binary on any system with Docker; it builds needed runtimes automatically
- **First-use overhead:** First job requesting runtime X triggers Docker build (~1-2 min); subsequent jobs use cached image
- **Network-friendly:** Binary can be deployed offline; doesn't require downloading Dockerfiles from registry

**Testing Strategy:**
- Verify embedded Dockerfiles can be extracted from binary
- On-demand build workflow: Request non-existent runtime → Builds → Returns image
- Compile C file on native → hash X
- Compile same C file via Docker runtime (triggers build on first use) → hash X (verified identical)
- Compile C++ file on native → hash Y
- Compile same C++ file via Docker runtime (uses cached image) → hash Y (verified identical)
- Verify driver selection: Language field determines gcc vs g++
- Verify binary portability: Deploy binary to fresh system; runtimes build on first use

**Next Steps:** Implementation of Milestones 1.0-1.2 (runtime foundation and C/C++ implementations)

---

### Milestone 1.0: Runtime System Foundation - STARTED (2026-03-21)

**Status:** ✅ IMPLEMENTATION STARTED

**Completed Components:**

1. **Runtime Package (`pkg/runtime/`)**
   - `types.go` — Core types and Runtime interface
     - `Runtime` interface: Execute, Available, Metadata, RuntimeID
     - `ExecutionRequest` and `ExecutionResult` types for job execution
     - `Metadata` struct with 9 fields for runtime identification (id, language, compiler, version, arch, OS, C runtime, C runtime version, details)
     - `AvailabilityError` for runtime discovery failures
   - `registry.go` — Runtime registry with search/matching
     - `Registry` type with thread-safe map of runtimes
     - Methods: Register, Get, All, Find, FindByLanguageAndCompiler, Available, Count
     - Thread-safe with RWMutex for concurrent access
   - `discoverer.go` — Discoverer interface for runtime discovery
     - `Discoverer` interface: Discover(ctx, registry), Name()
     - Used by native and Docker runtime implementations to register themselves
   - `runtime_test.go` — Unit tests for registry and interfaces
     - MockRuntime for testing
     - Tests: Register, Get, Duplicate detection, FindByLanguageAndCompiler
     - All 4 tests passing ✅

2. **Docker API Package (`pkg/docker/`)**
   - `types.go` — Docker abstraction types
     - `ContainerConfig` for container creation
     - `ExecResult` for command execution results
     - `createTarArchive()` helper for Dockerfile building
   - `client.go` — Docker API abstraction wrapper
     - `Client` struct wrapping official moby/moby client
     - `NewClient()` with environment variable support and connectivity check
     - Placeholder methods (stubs with TODO comments) for:
       - `PullImage()` - Pull container image
       - `ImageExists()` - Check local image existence
       - `BuildImage()` - Build image from Dockerfile
       - `StartContainer()` - Create and start container
       - `ExecInContainer()` - Execute command in running container
       - `StopContainer()` - Stop running container
       - `RemoveContainer()` - Remove container
       - `ContainerWait()` - Wait for container exit
     - Thread-safe with RWMutex
     - Proper error handling and resource cleanup

3. **Dependencies Added (`go.mod`)**
   - `github.com/moby/moby/client` — Official Docker Go API
   - `github.com/moby/moby/api` — Docker API types
   - Full transitive dependency tree resolved with `go mod tidy`
   - 20+ additional dependencies (docker, containerd, opentelemetry, etc.)

4. **Build Status**
   - ✅ `go build ./...` — Success
   - ✅ `go test ./pkg/runtime/...` — All 4 tests passing
   - ✅ Protocol still compiles with new packages
   - ✅ No lint errors in new code

**Architecture Decisions Made:**

- **Docker client initialization:** Verify connectivity on creation; reuse single client instance
- **Wrapper pattern:** Thin abstraction over moby/moby client; future implementations can swap details
- **Thread safety:** All client operations use RWMutex for concurrent-safe access
- **Placeholder strategy:** Core methods stubbed with TODO comments for implementation in next phase

**REFACTOR (2026-03-21): Made Runtime Package Language-Agnostic**

Initial implementation had C/C++-specific types:
- `Metadata.Compiler`, `Metadata.CRuntime`, `Metadata.CRuntimeVersion`
- `Registry.FindByLanguageAndCompiler()` method
- `ExecutionRequest.Command []string` — tied to subprocess execution

But the development plan and protocol define a **multi-language system**:
- Protocol has `CppToolchain`, `GoToolchain`, `RustToolchain` (oneof)
- Future languages: Java, Python, etc.
- Each language has different toolchain metadata

**Refactored to Generic Design:**
- Removed C/C++-specific fields from `Metadata` — now language-agnostic
- `ExecutionRequest.Job interface{}` — opaque to registry, interpreted by implementation
- `Registry.FindByLanguage(lang)` — works for any language
- `Metadata` has generic fields: `Language`, `Version`, `RuntimeType`, `IsNative`, `Details`
- C/C++-specific metadata handled by **CppDiscoverer implementation**, not core package

**Result:**
- ✅ Core runtime package works with C/C++, Go, Rust, and future languages
- ✅ Implementations (CppDiscoverer, GoDiscoverer, etc.) are language-specific
- ✅ Registry and discovery remain generic and extensible

**Test Results (After Refactoring):**
```
=== RUN   TestRegistryRegister
--- PASS: TestRegistryRegister (0.00s)
=== RUN   TestRegistryGet
--- PASS: TestRegistryGet (0.00s)
=== RUN   TestRegistryDuplicateRegister
--- PASS: TestRegistryDuplicateRegister (0.00s)
=== RUN   TestRegistryFindByLanguage
--- PASS: TestRegistryFindByLanguage (0.00s)
PASS
ok      github.com/Manu343726/buildozer/pkg/runtime     0.002s
```

**Files Created/Modified:**
- ✅ `/pkg/runtime/types.go` — Generic Runtime interface and types (refactored for multi-language support)
- ✅ `/pkg/runtime/registry.go` — Runtime registry with FindByLanguage (refactored)
- ✅ `/pkg/runtime/discoverer.go` — Generic discoverer interface
- ✅ `/pkg/runtime/runtime_test.go` — Unit tests with multi-language support (refactored)
- ✅ `/pkg/docker/types.go` — Docker types and helpers
- ✅ `/pkg/docker/client.go` — Docker API abstraction with TODO stubs
- ✅ `/go.mod` — Added moby/moby dependency

**Next: Milestone 1.0 Completion**
- Implement Docker API methods (PullImage, ImageExists, BuildImage, StartContainer, ExecInContainer, etc.)
- Create tests for Docker API abstraction
- Then proceed to implement language-specific runtimes:
  - **Milestone 1.1**: Native C/C++ toolchain detection (CppDiscoverer implementation)
  - **Milestone 1.2**: Docker-based C/C++ runtime with embedded Dockerfiles
  - Future: Go, Rust, and other language runtime implementations

---

## Phase 1: Core Protocol & Job Model (Weeks 1-4)

### Foundation: Tooling & Protocol Stack (2026-03-21)

**Status:** ✅ ESTABLISHED

**Buf Configuration & Proto Management:**
- **[buf](https://buf.build/) v1.40.1** installed and integrated for:
  - Protocol Buffer linting (STANDARD rule set enforcing Google protobuf best practices)
  - Code generation via `buf generate` (replaces system protoc dependency)
  - Breaking change detection for API evolution
  - VS Code integration with automatic proto formatting on save
  - CI/CD compatibility (no system dependencies)
- **Configuration files:**
  - `buf.yaml` - Linting rules (STANDARD), module dependencies (protovalidate)
  - `buf.gen.yaml` - Code generation plugins with proper Go package configuration
    - Protobuf code generation: `protoc-gen-go` → `internal/gen/`
    - Connect code generation: `protoc-gen-connect-go` → `internal/gen/`
    - Managed mode enabled with go_package_prefix override for correct import paths
    - protovalidate module disabled from managed code generation (annotations-only)
  - `.vscode/settings.json` - VS Code buf extension configuration
- **All proto files:** 100% buf lint compliant (STANDARD rule set)
  - Enum values prefixed with enum name (e.g., `TIME_UNIT_MILLISECOND`)
  - Enum zero values use `_UNSPECIFIED` suffix
  - RPC methods follow `<Service><Method>Request/Response` naming
  - Package versioning aligned with directory structure (`buildozer.proto.v1`)

**[Connect](https://connectrpc.com/) Protocol for RPC:**
- **Selected:** [Connect](https://connectrpc.com/) (connectrpc/connect-go v1.19.1)
- **Setup:**
  - `protoc-gen-connect-go` plugin installed via `go install connectrpc.com/connect/cmd/protoc-gen-connect-go@latest`
  - Generated code: `services.connect.go` with handler/client types for all RPC services
  - Handles gRPC, Connect, and gRPC-Web protocols transparently
- **Rationale:**
  - Supports gRPC-compatible protocol with simpler streaming semantics
  - Single protocol supporting gRPC, REST (HTTP/1.1), and WebSocket transports
  - Better web compatibility and browser support compared to gRPC
  - Cleaner error handling and bidirectional streaming implementation
  - Seamless Go library integration; low overhead
- **Implementation strategy:**
  - RPC method definitions remain in proto services (Connect compatible)
  - Connect code generated in `internal/gen/buildozer/proto/v1/protov1connect/`
  - gRPC compatibility maintained for existing clients
  - Backward compatible: existing protos work with Connect without modification
  - Future: Can support REST transport without proto changes

**Protovalidate Integration:**
- **Status:** Configured as optional enhancement (dependency in buf.yaml)
- **buf.yaml dependency:** `buf.build/bufbuild/protovalidate` for validation annotations
- **Usage pattern:** Annotations define validation rules in proto messages; runtime validation via business logic
- **Future:** Can add validation interceptor when implementing Connect handlers

**Proto File Organization:**
- **Location:** `buildozer/proto/v1/` (semantic versioning aligned with package)
- **Generated code:** `internal/gen/buildozer/proto/v1/` (buf managed)
  - `.pb.go` files: Protobuf message definitions
  - `protov1connect/` directory: Connect service handlers and clients
- **Core proto files:**
  - `vocabulary.proto` - Vocabulary types (fundamental building blocks)
  - `runtime.proto` - Runtime model and toolchain specifications
  - `job.proto` - Job model, progress tracking, and statistics
  - `job_data.proto` - JobData abstraction and artifact storage
  - `auth.proto` - Authentication and request metadata
  - `network_messages.proto` - All P2P message types (peer discovery, job lifecycle)
  - `services.proto` - Service (RPC) definitions for Connect (no gRPC dependency)

**Go Module Dependencies:**
- `connectrpc.com/connect v1.19.1` - Connect RPC library (includes gRPC/gRPC-Web compatibility)
- `google.golang.org/protobuf v1.36.11` - Protobuf runtime
- `google.golang.org/grpc v1.79.3` - gRPC (transitive from Connect)

**Build & Verification:**
- ✅ `buf generate` produces all .pb.go and .connect.go files
- ✅ `buf lint` reports 0 errors/warnings
- ✅ `go build ./...` completes successfully
- ✅ Project compiles and builds cleanly

**Vocabulary Type Enhancements (2026-03-21):**
- **Signature type added:** Cryptographic signature representation for artifact/message authentication
  - `SignatureAlgorithm` enum: RSA-SHA256, RSA-SHA512, ECDSA-SHA256, ECDSA-SHA512, Ed25519
  - `Signature` message: algorithm, base64-encoded value, optional key_id
  - Complements `Hash` vocabulary type for complete crypto support
  - Use cases: peer authentication, artifact signing, build provenance, message authentication

---

### Step 1: Protocol Definitions ✅ COMPLETE

**Objective:** Define comprehensive protocol buffer definitions for all P2P communication, job types, and data models.

**Files Created:**
- `pkg/proto/vocabulary.proto` - Common vocabulary types (TimeUnit, TimeDuration, TimeStamp, TimeRange, Percentage, Size, SizeUnit, HashAlgorithm, Hash, Version, ApiProtocol, ApiUri, LoadInfo)
- `pkg/proto/runtime.proto` - Runtime with oneof toolchain (CppToolchain, GoToolchain, RustToolchain), RuntimeRecipe, ResourceLimit
- `pkg/proto/job.proto` - Job with oneof job_spec (CppCompileJob, CppLinkJob), JobProgress, JobResult, JobDependency
- `pkg/proto/job_data.proto` - JobData, FileJobData, DirectoryJobData, StreamChunk, JobDataReference, RetentionPolicy, JobDataIndex
- `pkg/proto/auth.proto` - RequestMetadata, AuthResponse
- `pkg/proto/network_messages.proto` - NetworkMessage envelope, PeerAnnouncement, all P2P message types
- `pkg/proto/services.proto` - gRPC service definitions (JobService, ExecutorService, PeerService, SchedulerService)
- `pkg/proto/generate.go` - go:generate directive for proto compilation

**Key Design Decisions:**
- Protocol uses Google Protobuf 3.12.4 with gRPC services
- **Pure oneof pattern:** No redundant type enums - Job and Runtime types are discriminated by oneof field alone
- Vocabulary types: Reusable types across protocol (TimeDuration, TimeStamp, Version, Hash, ResourceSpec, etc.)
- Generic toolchain support: Runtime contains oneof for C++, Go, Rust (extensible to other languages)
- Generic job support: Job contains oneof for CppCompile, CppLink (extensible to other job types)
- Content-addressed artifact storage (SHA256 hashing)
- Real-time progress streaming for job execution
- Quorum-based scheduling via gRPC broadcasts
- Network messages wrapped in NetworkMessage envelope with metadata

**Compilation Status:**
- ✅ All `.proto` files compile successfully via `go generate ./...`
- ✅ 8 `.pb.go` files generated (vocabulary, runtime, job, job_data, auth, network_messages, services)
- ✅ 1 `.pb.grpc.go` file generated (services_grpc)
- ✅ Go dependencies resolved: protobuf v1.36.11, gRPC v1.79.2

**Next Step:** Step 2 - Job & Runtime Abstractions (Go implementation layer for job types)

---

## User Feedback & Notes

### Feedback on Step 1: Protocol Definitions

**Issue 1: Toolchain not generic**
- **User feedback:** "What you wrote as Toolchain is not generic, is really a C/C++ toolchain... remember what the plan said about generic messages and oneofs?"
- **Fix applied (iteration 1):** Refactored `Toolchain` to use oneof pattern with ToolchainType enum
- **User feedback:** "you don't need a Toolchain message, you can put the oneof in the runtime directly"
- **Fix applied (iteration 2):** Removed separate Toolchain message, moved oneof directly into Runtime with ToolchainType enum
- **User feedback:** "no need for toolchain type since we have the oneof"
- **Fix applied (iteration 3):** Removed ToolchainType enum, kept only oneof
  - `Runtime.toolchain` is now a pure oneof with CppToolchain, GoToolchain, RustToolchain
  - Oneof itself discriminates the toolchain type (no separate enum needed)
  - Field naming simplified: `cpp`, `go`, `rust` instead of `cpp_toolchain`, etc.
- **Status:** ✅ Recompiled successfully, all protos generate and build without errors
- **Design principle:** Elegant use of proto oneof pattern - the union itself carries the type information

**Issue 2: Job had redundant JobType enum**
- **User feedback:** "in Job, no job type enum, for the same reason"
- **Fix applied:** Removed `JobType` enum from Job message
  - Job now only has `oneof job_spec` with CppCompileJob, CppLinkJob
  - Job type is discriminated by which oneof field is set
  - Updated field numbers to be sequential (id=1, runtime=2, input_data_ids=3, etc.)
  - Updated content_hash comment to reflect job_spec_type instead of type
- **Status:** ✅ Recompiled successfully, all protos generate and build without errors
- **Design principle:** Consistency with Runtime pattern - oneof pattern provides type discrimination implicitly

**Addition: Vocabulary Types File**
- **User feedback:** "Add a vocabulary types proto file with basic types used along the protocol, such as TimeDuration (count + time unit), TimeStamp, etc etc"
- **Files created:** New `pkg/proto/vocabulary.proto` with common types:
  - TimeUnit enum (MILLISECOND, SECOND, MINUTE, HOUR, DAY)
  - TimeDuration (count + unit pair)
  - TimeStamp (unix milliseconds)
  - TimeRange (start + end) [renamed from DateRange]
  - Percentage (0-100)
  - Size with SizeUnit enum (BYTE through TERABYTE)
  - HashAlgorithm enum (SHA256, SHA512, BLAKE3)
  - Hash (algorithm enum + value)
  - Version (semantic versioning)
- **Additional feedback:**
  - Renamed DateRange → TimeRange for clarity
  - HashAlgorithm: Changed from string to enum (SHA256, SHA512, BLAKE3)
  - Removed Status message: Each API will define its own result message with specific code enums/details
  - Removed Progress message: Same reasoning - each RPC defines its own progress format
  - Removed ResourceAmount and ResourceSpec: Resource-specific types, will be defined by APIs that manipulate them
  - Kept Size/SizeUnit: Generic measurement types useful across protocol
  - Removed Identifier, Address, Label, Taggable: Network addresses and identifiers are represented as strings in context where needed; labels/tagging handled per-API
- **Status:** ✅ All changes compile successfully, proto package builds without errors
- **Design principle:** Lightweight vocabulary for fundamental types (time, size, hash, version) reused across protocol; API-specific results, identifiers, addresses, and metadata defined at point of use

**Vocabulary Type Integration:**
- **Objective:** Use vocabulary types consistently across all protocols where applicable
- **Changes applied:**
  - All proto files now import `vocabulary.proto`
  - **TimeStamp** replaces all `int64 unix_ms` timestamp fields (submitted_at, created_at, sent_at, updated_at, joined_at, cancelled_at, decided_at, error_time, last_seen, keep_until, etc.)
  - **TimeDuration** replaces duration fields (timeout in Job, keep_for in RetentionPolicy)
  - **Version** replaces version strings (compiler_version, c_runtime_version, go_version, rust_version, protocol_version, buildozer_version)
  - **Hash** replaces all content_hash string fields (RuntimeRecipe, Job, FileJobData, DirectoryJobData, StreamChunk, CacheQueryMessage, ArtifactFetchRequestMessage)
  - **Percentage** replaces progress_percent and current_load_percent uint32 fields
  - **Size** replaces size_bytes and total_size_bytes uint64 fields (JobData, DirectoryJobData, CacheQueryResponseMessage, CacheAnnouncementMessage, ArtifactFetchResponseMessage, PeerCapabilities cache_size)
- **Files modified:** runtime.proto, job.proto, job_data.proto, network_messages.proto, services.proto
- **Compilation status:** ✅ All protos compile successfully, proto package builds without errors
- **Design principle:** Consistent use of vocabulary layer throughout protocol reduces code duplication and ensures type-safe handling of common constructs

**Job Message Refactoring: Inputs Moved Into Job**
- **Rationale:** Job inputs must be part of the Job message itself to ensure they are never lost when the job is passed around between peers
- **Changes applied:**
  - Added `repeated JobData inputs = 25;` to Job message (keeping input and expected output IDs for caching)
  - Removed `repeated JobData inputs` from JobSubmissionMessage (now only contains Job + submitted_at)
  - Removed `repeated JobData inputs` from ExecuteJobRequest (now only contains Job)
  - Added job_data.proto import to job.proto (no circular dependencies)
  - Removed unused job_data.proto import from services.proto
- **Result:** Job is self-contained with all inputs, preventing data loss and simplifying message passing
- **Compilation status:** ✅ All protos compile successfully without warnings

**ApiUri Vocabulary Type Addition:**
- **Objective:** Add a network endpoint vocabulary type for consistent representation of API addresses
- **Added ApiProtocol enum (simplified):**
  - GRPC: gRPC protocol
  - REST: REST API
  - Note: Can be extended later (HTTP/HTTPS, GRPCS, etc.)
- **ApiUri fields:**
  - `host` (string): Hostname or IP address
  - `port` (uint32): Port number
  - `protocol` (ApiProtocol enum): Communication protocol
  - `subpath` (string, optional): Optional path component (e.g., "/api/v1", "/rpc")
- **Benefits:** Type-safe protocol specification, extensible for future protocols
- **Compilation status:** ✅ Protos compile successfully with simplified enum

**ApiUri Usage Throughout Protocol:**
- **Objective:** Use ApiUri vocabulary type for all network endpoint specifications
- **Changes applied:**
  - **network_messages.proto:**
    - NetworkMessage: `sender_address` (string) → `sender_uri` (ApiUri)
    - NetworkMessage: `reply_to_address` (string) → `reply_to_uri` (ApiUri, optional)
    - PeerAnnouncement: `grpc_address` (string) → `grpc_uri` (ApiUri)
    - PeerAnnouncement: `rest_api_address` (string) → `rest_api_uri` (ApiUri, optional)
  - **job_data.proto:**
    - JobDataReference: `peer_address` (string) → `peer_uri` (ApiUri, optional)
  - **services.proto:**
    - PeerInfo: `grpc_address` (string) → `grpc_uri` (ApiUri)
    - PeerInfo: `rest_api_address` (string) → `rest_api_uri` (ApiUri, optional)
- **Benefits:** Consistent, type-safe endpoint specification; replaces ad-hoc host:port string parsing
- **Compilation status:** ✅ All protos compile successfully with ApiUri usage

**PeerInfo Enhancement: Added Runtime and Resource Information:**
- **Objective:** Enrich PeerInfo with peer capabilities (runtimes, resources, load details)
- **Fields added to PeerInfo:**
  - `repeated Runtime available_runtimes`: Available toolchains/runtimes on the peer
  - `ResourceLimit resources`: Resource constraints and limits (CPU, RAM, disk, concurrent jobs)
  - `uint32 running_jobs_count`: Number of jobs currently running
  - `uint32 queued_jobs_count`: Number of jobs queued
- **Result:** PeerInfo now contains essential peer metadata for intelligent job scheduling and load balancing
- **Files modified:** services.proto (added runtime.proto import to resolve Runtime and ResourceLimit types)
- **Compilation status:** ✅ All protos compile successfully with enriched PeerInfo

**LoadInfo Message: Consolidated Load Reporting:**
- **Objective:** Extract load/utilization metrics into a reusable message, enable runtimes to report their own load
- **LoadInfo message (added to vocabulary.proto):**
  - `Percentage current_load`: Current resource utilization (0-100%)
  - `uint32 running_jobs_count`: Number of jobs currently running
  - `uint32 queued_jobs_count`: Number of jobs queued
  - `repeated Percentage cpu_per_thread`: CPU usage per thread (type-safe percentage per thread)
  - `Size ram_usage`: Current RAM usage
- **Applied to:**
  - **Runtime** (runtime.proto): Added `LoadInfo load` field for runtime to report current utilization
  - **PeerCapabilities** (network_messages.proto): Replaced 3 individual fields with single `LoadInfo load` field
  - **PeerInfo** (services.proto): Replaced 3 individual fields with single `LoadInfo load` field
- **Benefits:** Single source of truth for load metrics, detailed CPU/RAM insights, reusable across different message types, cleaner structure
- **Compilation status:** ✅ All protos compile successfully with LoadInfo consolidation

**Timestamp Standardization: Complete Protocol Audit:**
- **Objective:** Ensure all timestamps use TimeStamp vocabulary type (no raw int64 timestamp fields)
- **Audit performed on all .proto files:**
  - Found 8 raw int64 timestamp fields across 4 files:
    - auth.proto: RequestMetadata timestamp_ms, AuthResponse timestamp_ms
    - build_request.proto: created_at_ms, modified_at_ms, announced_at_ms
    - job_data.proto: JobDataMetadata created_at_ms, last_accessed_at_ms
    - services.proto: CommitScheduleResponse estimated_start_ms
- **Changes applied:**
  - auth.proto: Added vocabulary import, renamed `int64 timestamp_ms` → `TimeStamp timestamp` (2 fields)
  - build_request.proto: Added vocabulary import, replaced all 3 int64 timestamp fields with TimeStamp
  - job_data.proto: Replaced 2 int64 timestamp fields in JobDataMetadata with TimeStamp
  - services.proto: Replaced `int64 estimated_start_ms` with `TimeStamp estimated_start`
- **Result:** All timestamps in protocol now use vocabulary type, ensuring consistency and type safety
- **Compilation status:** ✅ All protos compile successfully with complete timestamp standardization

**Duration Standardization: Complete Protocol Audit:**
- **Objective:** Ensure all durations and TTLs use TimeDuration vocabulary type (no raw seconds/milliseconds fields)
- **Audit performed on all .proto files:**
  - Found 3 raw uint32 duration fields in build_request.proto:
    - Line 140: Build timeout_seconds
    - Line 306: P2P transfer timeout_seconds
    - Line 378: Peer announcement ttl_seconds
- **Changes applied:**
  - Line 140: Renamed `uint32 timeout_seconds` → `TimeDuration timeout`
  - Line 306: Renamed `uint32 timeout_seconds` → `TimeDuration timeout`
  - Line 378: Renamed `uint32 ttl_seconds` → `TimeDuration ttl`
- **Result:** All duration fields in protocol now use vocabulary type, ensuring consistency and explicit time unit specification
- **Compilation status:** ✅ All protos compile successfully with complete duration standardization

**Note on TimeRange:** Currently no start/end timestamp pairs in protocol messages that would benefit from TimeRange type. TimeRange is available for future use when needed (e.g., time window specifications).

---

## Next Phase: Step 2 - Job & Runtime Abstractions

**CppToolchain Type Safety Enhancement:**
- **Objective:** Convert CppToolchain string fields to enums for type safety and validation
- **Enums created (in runtime.proto):**
  - **CppLanguage**: C, CPP
  - **CppCompiler**: GCC, CLANG (extensible for additional compilers)
  - **CppArchitecture**: X86_64, AARCH64, ARM, PPC64LE (extensible for new architectures)
  - **CRuntime**: GLIBC, MUSL (C runtime implementations, extensible for other runtimes)
- **CppToolchain message refactored:**
  - `string language` → `CppLanguage language` (enum)
  - `string compiler` → `CppCompiler compiler` (enum)
  - `string architecture` → `CppArchitecture architecture` (enum)
  - `string c_runtime` → `CRuntime c_runtime` (enum)
  - Other fields (compiler_version, c_runtime_version) remain as Version types
- **Benefits:** Type-safe toolchain specification, validated values, extensible for future compilers/architectures, prevents typos and invalid values
- **Compilation status:** ✅ All protos compile successfully with CppToolchain enums

**Enum Simplification: Removed UNSPECIFIED Values:**
- **Objective:** Remove unnecessary UNSPECIFIED enum values since protobuf3 allows checking field presence without sentinel values
- **Rationale:** Protobuf3 tracks field presence implicitly; explicit UNSPECIFIED values are not needed and simplify enums
- **Changes applied across all proto files:**
  - vocabulary.proto: TimeUnit, HashAlgorithm, SizeUnit, ApiProtocol
  - runtime.proto: CppLanguage, CppCompiler, CppArchitecture, CRuntime
  - job.proto: JobProgress.JobStatus, JobResult.JobStatus
  - job_data.proto: JobData.DataType
  - network_messages.proto: NetworkMessage.MessageType, JobErrorMessage.ErrorType
  - build_request.proto: BuildType, JobDependency.DependencyType
- **Result:** All enums now start at 0 with meaningful values, reducing cognitive overhead and simplifying enum handling
- **Compilation status:** ✅ All protos compile successfully with simplified enums

**CppToolchain Enhancement: C++ ABI and Standard Library:**
- **Objective:** Add comprehensive ABI and standard library specification to CppToolchain for precise C++ compilation environment capture
- **Enums created (in runtime.proto):**
  - **CppAbi**: ITANIUM (default for Unix-like systems), MICROSOFT (for Windows/MSVC)
  - **CppStdlib**: LIBSTDCXX (GCC), LIBCXX (LLVM/Clang), MSVC_STL (Microsoft)
- **Fields added to CppToolchain:**
  - `CppAbi cpp_abi`: C++ ABI specification
  - `CppStdlib cpp_stdlib`: C++ standard library implementation
  - `repeated string abi_modifiers`: Compiler-specific ABI modification flags
    - Examples: `-fabi-version=X` (GCC C++ ABI version), `-fglibcxx-use-cxx11-abi` (GCC std::string ABI), other compiler-specific ABI control flags
- **Benefits:** Captures ABI/stdlib choices for correct cross-compilation and reproducible builds; abi_modifiers allows compiler-specific fine-tuning (e.g., std::string ABI changes) without modifying core enums
- **Compilation status:** ✅ All protos compile successfully with ABI/stdlib additions

**NetworkMessage MessageType Removal:**
- **Objective:** Remove redundant MessageType enum since oneof payload already discriminates message types
- **Change applied (in network_messages.proto):**
  - Removed `enum MessageType` (12 values: PEER_ANNOUNCEMENT, JOB_SUBMISSION, JOB_PROGRESS, JOB_RESULT, JOB_ERROR, JOB_CANCELLATION, SCHEDULE_DECISION, CACHE_QUERY, CACHE_QUERY_RESPONSE, CACHE_ANNOUNCEMENT, ARTIFACT_FETCH_REQUEST, ARTIFACT_FETCH_RESPONSE)
  - Removed `MessageType message_type = 6;` field
  - Updated comment on oneof payload to clarify type discrimination
- **Rationale:** The oneof field implicitly provides type discrimination - the concrete message type is determined by which field is set, making the explicit enum redundant
- **Note:** ErrorType enum in JobErrorMessage remains since it categorizes error types within a single message type (not discriminating between different message types in a oneof)
- **Compilation status:** ✅ All protos compile successfully after MessageType removal

**PeerGoodbye Message Addition:**
- **Objective:** Add peer departure announcement to complement PeerAnnouncement
- **PeerGoodbye message (in network_messages.proto):**
  - `string peer_id`: Peer ID that is leaving
  - `TimeStamp left_at`: Timestamp when peer is leaving
  - `string reason`: Optional reason for departure (e.g., "graceful shutdown", "network error")
- **Integration:** Added to NetworkMessage payload oneof as field 32
- **Benefits:** Enables peers to detect departures and clean up state; complements peer discovery with peer departure notification
- **Compilation status:** ✅ All protos compile successfully with PeerGoodbye addition

**Job Status Enhancement: Data Transfer Phases:**
- **Objective:** Track input and output data transfer phases separately from execution
- **JobProgress.JobStatus enhancements (in job.proto):**
  - Added `INPUT_TRANSFER = 3`: Inputs being transferred to executing peer (after SCHEDULED)
  - Added `OUTPUT_TRANSFER = 6`: Outputs being transferred back to requesting client (after COMPLETED execution)
  - Updated sequence: PENDING → READY → SCHEDULED → INPUT_TRANSFER → RUNNING → COMPLETED → OUTPUT_TRANSFER → [FAILED/CANCELLED at any point]
  - Previous statuses renumbered: RUNNING=3→4, COMPLETED=4→5, FAILED=5→7, CANCELLED=6→8
- **JobResult.JobStatus design (in job.proto):**
  - JobResult message is only published after output transfer completes
  - Status enum: COMPLETED=0 (fully delivered), FAILED=1, CANCELLED=2
  - No intermediate states in JobResult since it represents the final state
- **Benefits:** Separates computation phases from data transfer; enables complete job lifecycle tracking through JobProgress; JobResult represents truly final state
- **Compilation status:** ✅ All protos compile successfully with refined job status model

**JobStatus Field Naming Clarification:**
- **Objective:** Clarify field naming in JobStatus message for clarity
- **Change applied (in services.proto):**
  - Renamed `string submitted_to_peer_id = 2;` → `string submitter_id = 2;`
  - Updated comment to: "Client ID of the client who received the job submission"
- **Rationale:** The field represents the client that accepted and received the submission, not the source client. The terminology and comment should be clearer about this semantic meaning.
- **Compilation status:** ✅ Proto compiles successfully with renamed field
- **Note:** Generated .pb.go file will be regenerated on next proto compilation

**JobTimings Message Addition:**
- **Objective:** Track exact time ranges and durations of job processing through all phases
- **JobTimings message (added to job.proto):**
  - **Phase time ranges (using TimeRange: start_time + end_time):**
    - `pending_time_range`: Job submitted until READY (dependencies met)
    - `ready_time_range`: READY until SCHEDULED (assigned to peer)
    - `scheduled_time_range`: SCHEDULED until INPUT_TRANSFER (ready to transfer inputs)
    - `input_transfer_time_range`: INPUT_TRANSFER until RUNNING (inputs transferred)
    - `running_time_range`: RUNNING until COMPLETED (execution finished)
    - `completed_time_range`: COMPLETED until OUTPUT_TRANSFER (ready to transfer outputs)
    - `output_transfer_time_range`: OUTPUT_TRANSFER until final completion (outputs transferred)
  - **Terminal state timestamps:**
    - `failed_at`: When job failed (can occur at any phase)
    - `cancelled_at`: When job cancelled (can occur at any phase)
  - **Phase durations (derived from time ranges):**
    - `pending_duration`, `ready_duration`, `scheduled_duration`, `input_transfer_duration`, `running_duration`, `output_transfer_duration`
  - **Aggregate metrics:**
    - `total_duration`: End-to-end from submission to final state
    - `wall_clock_duration`: Total elapsed time including any gaps
    - `compute_duration`: Actual execution time (same as running_duration)
- **Design rationale:** Using TimeRange instead of individual timestamps naturally handles gaps and provides exact timing information. If a job is paused, interrupted, or suspended at any point, the time ranges capture the exact contiguous periods when in each phase.
- **Benefits:** Precise visibility into job lifecycle; enables bottleneck analysis (queue time vs. transfer time vs. compute time); handles edge cases like job suspension or multi-phase execution
- **Compilation status:** ✅ Proto compiles successfully with JobTimings using TimeRange

**JobStatistics Message Addition:**
- **Objective:** Aggregate timing, resource usage, and performance metrics for job analysis
- **Refactored with sub-messages (in job.proto):**
  - **JobResourceUsage** - CPU, memory, and disk I/O resource metrics:
    - `uint32 peak_cpu_cores_used`: Peak number of CPU cores actively used
    - `uint32 min_cpu_cores_used`: Minimum number of CPU cores actively used
    - `uint32 avg_cpu_cores_used`: Average number of CPU cores actively used
    - `Size peak_memory_usage`: Peak memory consumption
    - `Size min_memory_usage`: Minimum memory consumption
    - `Size avg_memory_usage`: Average memory consumption
    - `uint64 total_disk_read_bytes`: Total bytes read from disk during execution
    - `uint64 total_disk_write_bytes`: Total bytes written to disk during execution
    - `double peak_disk_read_bandwidth`: Peak read bandwidth (bytes/sec)
    - `double peak_disk_write_bandwidth`: Peak write bandwidth (bytes/sec)
    - `double avg_disk_read_bandwidth`: Average read bandwidth (bytes/sec)
    - `double avg_disk_write_bandwidth`: Average write bandwidth (bytes/sec)
  - **JobDataTransfer** - All data size and network I/O metrics:
    - `Size input_data_size`: Total size of all inputs
    - `Size output_data_size`: Total size of all outputs
    - `Size total_data_transferred`: Combined total (inputs + outputs)
    - `Size network_input_size`: Data fetched from network (vs. local cache)
    - `Size network_output_size`: Data sent to network peers
  - **JobCacheInfo** - Cache information:
    - `bool cache_hit`: Whether output was served from cache
    - `string cache_source_peer_id`: Which peer provided the cached result
  - **JobExecutionMetrics** - Execution details, results, and resource consumption:
    - `string executing_peer_id`: Which peer executed the job
    - `int32 exit_code`: Process exit code
    - `bool success`: Whether execution completed successfully
    - `repeated string stdout_lines`: Standard output captured during execution (one line per entry)
    - `repeated string stderr_lines`: Standard error captured during execution (one line per entry)
    - `JobResourceUsage resource_usage`: Resource consumption during execution (CPU, memory, disk I/O)
  - **JobStatistics** - Top-level aggregator:
    - `string job_id`: Job identifier
    - `JobTimings timings`: Embedded timing information
    - `JobDataTransfer data_transfer`: Embedded data transfer metrics
    - `CacheQueryStatistics cache_query_statistics`: Embedded cache query statistics and timing metrics (from vocabulary)
    - `JobExecutionMetrics execution_metrics`: Embedded execution details, results, and resource consumption
- **Structural refactoring:** JobResourceUsage moved from direct sub-message in JobStatistics to be embedded within JobExecutionMetrics, since resource consumption is semantically part of execution metrics (not a separate category)
- **Design rationale:** Sub-message organization mirrors the pattern used for JobTimings. Groups related metrics by category, making the protocol cleaner and easier to extend with new categories (e.g., energy consumption, network latency distribution).
- **Benefits:** Better organization and readability; enables independent evolution of each metric category; cleaner API when selecting specific metric subsets; extensible for future metrics without modifying top-level JobStatistics
- **Compilation status:** ✅ Proto compiles successfully with JobStatistics sub-messages

**JobExecutionMetrics Enhancement: Output Capture:**
- **Objective:** Include stdout and stderr output in execution metrics for debugging and auditing
- **Change applied (in job.proto):**
  - Added `repeated string stdout_lines = 4;` - captures stdout as a list of text lines
  - Added `repeated string stderr_lines = 5;` - captures stderr as a list of text lines
- **Design rationale:** Line-based storage enables efficient streaming and log-level filtering; avoids storing massive single strings for long-running jobs; each entry represents one line of output
- **Benefits:** Enables complete job output inspection; supports debugging failed jobs; aids in audit trails; allows per-line processing without buffering entire output
- **Compilation status:** ✅ Proto compiles successfully with stdout/stderr additions

**JobResourceUsage Enhancement: Min and Average Metrics:**
- **Objective:** Track resource usage patterns beyond peak values
- **Change applied (in job.proto):**
  - Added `uint32 min_cpu_cores_used = 2;` - minimum CPU cores actively used
  - Added `uint32 avg_cpu_cores_used = 3;` - average CPU cores actively used
  - Added `Size min_memory_usage = 5;` - minimum memory consumption
  - Added `Size avg_memory_usage = 6;` - average memory consumption
- **Design rationale:** Min and average values complement peak metrics to provide complete resource utilization patterns. Peak alone can be misleading (e.g., brief spikes); min/avg provide insights into baseline resource needs.
- **Benefits:** Enables accurate resource provisioning and scheduling; helps identify jobs with volatile vs. stable resource patterns; supports cost optimization and performance profiling
- **Compilation status:** ✅ Proto compiles successfully with expanded resource metrics

**JobResourceUsage Reorganization and Disk I/O Enhancement:**
- **Objective:** Include JobResourceUsage as part of execution metrics and add disk I/O statistics
- **Changes applied (in job.proto):**
  - **Structural reorganization:**
    - Moved JobResourceUsage from direct sub-message in JobStatistics to be embedded within JobExecutionMetrics
    - Rationale: Resource consumption is semantically part of execution metrics, not a separate analytics category
    - Note: JobStatistics now contains 5 fields instead of 6 (execution_metrics now includes resource_usage)
  - **Disk I/O metrics added to JobResourceUsage:**
    - `Size total_disk_read`: Total data read from disk
    - `Size total_disk_write`: Total data written to disk
    - `Size peak_disk_read_bandwidth`: Peak read bandwidth
    - `Size peak_disk_write_bandwidth`: Peak write bandwidth
    - `Size avg_disk_read_bandwidth`: Average read bandwidth
    - `Size avg_disk_write_bandwidth`: Average write bandwidth
- **Design rationale:** Disk I/O is critical for understanding job performance, especially for I/O-bound workloads. Peak/avg bandwidth helps identify sustained I/O patterns vs. brief spikes.
- **Benefits:** Complete resource profiling (CPU, memory, disk); enables identification of bottlenecks; supports resource provisioning decisions; bandwidth metrics help with scheduling optimization
- **Compilation status:** ✅ Proto compiles successfully with reorganized and enhanced resource metrics

**Size Type Enhancement: Double Support for Flexible Measurements:**
- **Objective:** Enable Size type to represent decimal values for bandwidth and other fractional measurements
- **Change applied (in vocabulary.proto):**
  - Changed `int64 count = 1;` → `double count = 1;` in Size message
  - Updated comment: "Size count (supports decimal values)"
  - Added message-level comment: "Supports decimal values for flexible representation (e.g., bandwidth in bytes/sec)"
- **Impact on JobResourceUsage (in job.proto):**
  - Disk I/O metrics now use Size type instead of uint64 and double primitives
  - Field naming simplified: removed "_bytes" suffix since Size includes units
  - Total disk metrics: `total_disk_read` and `total_disk_write`
  - Bandwidth metrics: `peak_disk_read_bandwidth`, `peak_disk_write_bandwidth`, `avg_disk_read_bandwidth`, `avg_disk_write_bandwidth`
- **Benefits:** Unified type for all size and bandwidth measurements; consistent unit handling; enables flexible representation of both discrete sizes and continuous rates; cleaner API with fewer primitive types
- **Compilation status:** ✅ Proto compiles successfully with unified Size type

**Field Naming Cleanup: Remove Redundant Unit Suffixes:**
- **Objective:** Eliminate redundant "_bytes" suffix since Size type already specifies units
- **Changes applied (in job.proto):**
  - Renamed `total_disk_read_bytes` → `total_disk_read`
  - Renamed `total_disk_write_bytes` → `total_disk_write`
  - Updated field comments to remove "(bytes/sec)" and "bytes read/written" references since unit information is in the Size message
- **Rationale:** Size type carries unit information; field names should describe the quantity, not repeat the unit. Cleaner, DRY naming pattern.
- **Compilation status:** ✅ Proto compiles successfully with cleaned-up field names

**CPU and Memory Metrics Separation into Sub-messages:**
- **Objective:** Extract CPU and memory metrics into dedicated messages for detailed per-core and aggregate utilization statistics
- **New messages created (in job.proto):**
  - **CpuUsage** - CPU/core utilization metrics (peak, min, avg percentages):
    - `Percentage peak`: Peak CPU/core utilization percentage (0-100)
    - `Percentage min`: Minimum CPU/core utilization percentage (0-100)
    - `Percentage avg`: Average CPU/core utilization percentage (0-100)
  - **JobMemoryUsage** - Memory resource consumption:
    - `Size peak_memory`: Peak memory usage
    - `Size min_memory`: Minimum memory usage
    - `Size avg_memory`: Average memory usage
- **JobResourceUsage refactored:**
  - `CpuUsage avg_cpu_usage`: Aggregate CPU utilization across all cores
  - `repeated CpuUsage per_core_usage`: Per-core CPU utilization (one CpuUsage entry per CPU core)
  - `JobMemoryUsage memory_usage`: Memory resource consumption
  - Disk I/O metrics: total_disk_read, total_disk_write, bandwidth statistics
  - Field numbering updated for consistency (1-9)
- **Design rationale:** CpuUsage message provides peak/min/avg percentages for both aggregate and per-core analysis. Separates resource types enables independent expansion. Per-core stats are essential for performance debugging on multi-core systems.
- **Benefits:** Cleaner structure; enables detailed per-core performance profiling; supports NUMA and CPU affinity analysis; consistent message pattern for aggregate + per-unit metrics
- **Compilation status:** ✅ Proto compiles successfully with refined CPU/memory metrics structure

**CpuUsage Message Clarification and Simplification:**
- **Objective:** Clarify CPU usage metrics as utilization percentages for both aggregate and per-core analysis
- **Change applied (in job.proto):**
  - Renamed `JobCpuUsage` → `CpuUsage` - simpler, reusable name for CPU utilization metrics
  - Removed core counting fields (peak_cores_used, min_cores_used, avg_cores_used) - not needed; focus on utilization percentages
  - Simplified to three fields capturing utilization %: peak (0-100), min (0-100), avg (0-100)
  - `JobResourceUsage` now uses `CpuUsage avg_cpu_usage` (aggregate) and `repeated CpuUsage per_core_usage` (per-core)
- **Design rationale:** CpuUsage represents utilization percentage for any CPU unit (aggregate or single core). Per-core array provides core-by-core breakdown without needing separate counting fields.
- **Benefits:** Unified type for CPU analysis; cleaner API; enables direct per-core utilization comparison with aggregate; flexible for future multi-socket/NUMA architectures
- **Compilation status:** ✅ Proto compiles successfully with simplified CpuUsage message

**Resource Usage Types Promoted to Vocabulary:**
- **Objective:** Establish generic, reusable resource usage tracking suitable for jobs, peer monitoring, and system metrics
- **Messages moved to vocabulary.proto (from job.proto):**
  - **CpuUsage** - CPU/core utilization percentages (peak, min, avg)
  - **MemoryUsage** - Memory consumption with peak/min/avg metrics (renamed from JobMemoryUsage for clarity)
  - **ResourceUsage** - Comprehensive resource tracking (renamed from JobResourceUsage for generic use):
    - `CpuUsage avg_cpu_usage`: Aggregate CPU utilization
    - `repeated CpuUsage per_core_usage`: Per-core utilization breakdown
    - `MemoryUsage memory_usage`: Memory metrics
    - Disk I/O metrics: total_disk_read, total_disk_write, bandwidth statistics (peak and average)
- **Updated job.proto:**
  - JobExecutionMetrics now references `ResourceUsage resource_usage` from vocabulary (not JobResourceUsage)
  - Removed local definitions of CpuUsage, JobMemoryUsage, JobResourceUsage
  - vocabulary.proto import already present; ResourceUsage now available
- **Design rationale:** Resource consumption is a fundamental measurement applicable to jobs, peers, system monitoring, and performance analysis. Moving to vocabulary makes it a first-class protocol type, enabling consistent resource tracking across all distributed system components.
- **Benefits:** Enables resource reporting at multiple levels (job, peer, system); reusable for quota tracking, scheduling, and monitoring; consistent metrics across protocol; future-extensible for energy, network I/O, and other resources
- **Compilation status:** ✅ Proto compiles successfully with vocabulary-based resource types

**Cache Info Promoted to Vocabulary with Timing Metrics:**
- **Objective:** Establish generic, reusable cache tracking suitable for caching any artifact (job outputs, data, etc.), including detailed cache operation timings
- **Message moved and enhanced in vocabulary.proto:**
  - **CacheQueryStatistics** - Cache query and hit information with timing metrics:
    - `bool cache_hit`: Whether the item was served from cache
    - `string cache_source_peer_id`: Which peer had the cached item (if cache_hit=true)
    - `TimeDuration hash_time`: Time spent computing hash of the item
    - `TimeDuration query_time`: Time spent querying the cache
    - `TimeDuration extraction_time`: Time spent extracting item from cache (if cache_hit=true)
- **Updated job.proto:**
  - JobStatistics now references `CacheQueryStatistics cache_query_statistics` from vocabulary
  - Removed JobCacheInfo message definition
  - Updated field comment to include timing metrics
- **Design rationale:** Cache performance is critical for distributed systems. Timing metrics enable identification of cache bottlenecks (hashing vs. querying vs. extraction). Generic type supports caching at multiple levels (job results, artifacts, data, etc.).
- **Benefits:** Cache operation profiling for performance analysis; generic cache tracking across protocol; supports cache optimization decisions; enables SLA tracking for cache-hit operations
- **Compilation status:** ✅ Proto compiles successfully with vocabulary-based cache info

**IOUsage Message Creation and ResourceUsage Refactoring:**
- **Objective:** Extract I/O and bandwidth metrics into a reusable generic message for disk, network, and other I/O types
- **New message created in vocabulary.proto:**
  - **IOUsage** - Generic I/O and bandwidth tracking:
    - `Size total_read`: Total data read
    - `Size total_write`: Total data written
    - `Size peak_read_bandwidth`: Peak read bandwidth
    - `Size peak_write_bandwidth`: Peak write bandwidth
    - `Size avg_read_bandwidth`: Average read bandwidth
    - `Size avg_write_bandwidth`: Average write bandwidth
- **ResourceUsage refactored (in vocabulary.proto):**
  - Removed individual `Size total_disk_*` and bandwidth fields
  - Added `IOUsage disk_io`: Disk I/O metrics (read/write data and bandwidth)
  - Simplified field numbering (now 1-4 instead of 1-9)
  - Cleaner structure with grouped I/O metrics
- **Design rationale:** IOUsage is generic enough to represent I/O for disk, network, or other channels. Keeps protocol extensible without duplicating I/O metric definitions. Future: memory bandwidth, storage I/O, or other contexts can reuse IOUsage.
- **Benefits:** Reusable I/O usage tracking; supports disk and network metrics with same interface; enables consistent I/O monitoring across protocol; simpler ResourceUsage structure
- **Compilation status:** ✅ Proto compiles successfully with IOUsage separation

**ResourceUsage Enhancement: Network I/O Metrics:**
- **Objective:** Add network I/O tracking to ResourceUsage for complete resource visibility
- **Change applied (in vocabulary.proto):**
  - Added `IOUsage network_io = 5;` field to ResourceUsage
  - Updated ResourceUsage comment to include "network I/O"
- **Rationale:** Network I/O is equally important as disk I/O for distributed systems. Using the same IOUsage type (total_read, total_write, bandwidth metrics) ensures consistent metrics across I/O types.
- **Benefits:** Complete resource telemetry (CPU, memory, disk, network); enables network bottleneck identification; consistent monitoring across all I/O channels
- **Compilation status:** ✅ Proto compiles successfully with network_io field added

**MemoryUsage Enhancement: Optional Memory I/O Metrics:**
- **Objective:** Track memory bandwidth and I/O performance in addition to memory consumption
- **Change applied (in vocabulary.proto):**
  - Added `optional IOUsage memory_io = 4;` field to MemoryUsage
  - Comment: "Optional: Memory I/O metrics (bandwidth and throughput)"
- **Rationale:** Memory bandwidth can be a performance bottleneck in CPU-intensive workloads. IOUsage (with total_read/write and bandwidth metrics) provides comprehensive memory access performance data.
- **Benefits:** Enables memory bandwidth profiling; identifies memory performance bottlenecks; optional field keeps it backward compatible
- **Compilation status:** ✅ Proto compiles successfully with optional memory_io field

---

## Dev Environment Setup

### Buf Installation & VS Code Integration (2026-03-18)

**Status:** ✅ COMPLETE - Including Full Lint Compliance

**Changes:**
1. **Switched from system protoc to buf (Go-based alternative)**
   - Removed dependency on system protobuf-compiler
   - Buf v1.40.1 installed via Go module (`go install github.com/bufbuild/buf/cmd/buf@v1.40.1`)
   - No system dependencies required

2. **Created buf configuration files**
   - `buf.yaml` - Linting and breaking change detection rules
   - `buf.gen.yaml` - Code generation plugin configuration
   - `pkg/proto/generate.go` - Updated to use `buf generate`

3. **Updated dev container configuration**
   - Added `bufbuild.vscode-buf` extension to `.devcontainer/devcontainer.json`
   - Updated postCreateCommand to install buf v1.40.1
   - Added [proto] language settings for auto-formatting

4. **Created VS Code workspace configuration**
   - `.vscode/settings.json` - Buf linting on save, buf as default proto formatter
   - `.vscode/extensions.json` - Added buf extension to recommendations

5. **Created documentation**
   - `pkg/proto/README.md` - Comprehensive guide to buf, advantages/disadvantages vs protoc, workflow examples, troubleshooting

**Benefits:**
- ✅ No system protoc dependency (containers, CI/CD, cross-platform)
- ✅ Integrated linting with buf lint
- ✅ Breaking change detection
- ✅ VS Code integration for real-time diagnostics
- ✅ Automatic proto file formatting on save
- ✅ Consistent development environment via devcontainer

**Proto Compilation Status:**
- All 8 proto files successfully compile
- 8 .pb.go files generated
- 1 _grpc.pb.go file generated
- Project builds cleanly

### buf Lint Compliance (2026-03-18)

**Status:** ✅ COMPLETE - All 36+ Linting Issues Fixed

**Issues Fixed:**

1. **buf.yaml Deprecation (1 issue)**
   - Changed lint rule from deprecated `DEFAULT` to `STANDARD`
   - No functional change; `STANDARD` is the recommended category

2. **Enum Value Naming (21 issues)**
   - **Issue:** Enum values must be prefixed with their enum name in UPPER_CASE
   - **Fix:** Renamed all enum values across 7 proto files
   - Examples:
     - `MILLISECOND` → `TIME_UNIT_MILLISECOND`
     - `GCC` → `CPP_COMPILER_GCC`
     - `FILE` → `DATA_TYPE_FILE`
     - `TIMEOUT` → `ERROR_TYPE_TIMEOUT`
   - **Benefit:** Eliminates naming ambiguity in compound type names

3. **Enum Zero Values (21 issues)**
   - **Issue:** Enum zero values must use `_UNSPECIFIED` suffix in proto3
   - **Fix:** Renamed all unknown/default enum values to `<ENUM>_UNSPECIFIED`
   - Examples:
     - `UNKNOWN_TIME_UNIT` → `TIME_UNIT_UNSPECIFIED`
     - `UNKNOWN_CPP_COMPILER` → `CPP_COMPILER_UNSPECIFIED`
   - **Benefit:** Proto3 compatibility; zero value represents "unknown" state

4. **Package Versioning & Directory Structure (6 issues)**
   - **Issue:** Package `buildozer.proto` detected; should be `buildozer.proto.v1`
   - **Issue:** Proto files in `pkg/proto/` but package suggests `buildozer/proto/v1/`
   - **Fix:** 
     - Reorganized proto files: `pkg/proto/` → `buildozer/proto/v1/`
     - Updated package declarations to `buildozer.proto.v1`
     - Updated all import paths to `buildozer/proto/v1/*.proto`
     - Updated `go_package` option to reflect new location
   - **Benefit:** Aligns file structure with semantic versioning; enables multiple API versions

5. **RPC Request/Response Naming (24+ issues)**
   - **Issue:** RPC request/response types must follow `<Service><RPC>Request/Response` pattern
   - **Fix:** 
     - Renamed RPC message types across all 4 services
     - Created wrapper message types for proper naming convention
     - Examples:
       - `JobSubmissionMessage` + `JobStatus` → `SubmitJobRequest` + `SubmitJobResponse`
       - `GetJobStatusRequest` + `JobProgress` → `GetJobStatusRequest` + `GetJobStatusResponse`
       - `PeerAnnouncement` → `AnnounceSelfRequest`
       - `CacheQueryMessage` → `QueryCacheRequest`
   - **Benefit:** Consistent RPC naming enables auto-documentation and tool generation

**Arc Linting Summary:**
- Before: 36 lint errors/warnings across all 8 proto files
- After: ✅ 0 errors/warnings (100% compliant)
- Status: Clean buf lint output

**Files Reorganized:**
```
OLD:                          NEW:
pkg/proto/                    buildozer/proto/v1/
  ├─ vocabulary.proto           ├─ vocabulary.proto
  ├─ runtime.proto              ├─ runtime.proto
  ├─ job.proto                  ├─ job.proto
  ├─ job_data.proto             ├─ job_data.proto
  ├─ auth.proto                 ├─ auth.proto
  ├─ network_messages.proto      ├─ network_messages.proto
  ├─ services.proto             ├─ services.proto
  └─ generate.go                └─ generate.go
```

**buf LSP Field Documentation Format:**
- **Issue:** buf language server does not recognize field/enum value documentation when comments appear on the same line as declarations
- **Root Cause:** buf LSP parser expects comments on the previous line, not trailing comments (LSP limitation)
- **Fix Applied (2026-03-21):** Moved all inline field/enum value documentation to previous lines across all proto files
  - **Files modified:** vocabulary.proto, runtime.proto, job.proto, job_data.proto, network_messages.proto
  - **Scope:** Approximately 50+ field/enum documentation comments across entire protocol
  - **Examples of fixes:**
    - `SignatureAlgorithm enum values` (5 items): Moved comments from `ALGORITHM = N; // comment` to previous lines
    - `SizeUnit enum values` (5 items): Same pattern
    - `ApiProtocol enum values` (3 items): Same pattern
    - `CppAbi enum values` (1 item): `CPP_ABI_ITANIUM = 1; // Itanium ABI...` → comment on previous line
    - `CppStdlib enum values` (2 items): Similar format fixes
    - `JobStatus enum in JobProgress` (9 items): All inline comments moved to previous lines
    - `JobStatus enum in JobResult` (3 items): Same fix
    - `ErrorType enum in JobErrorMessage` (9 items): All 8 error types with inline comments fixed
    - `JobTimings message fields` (8 items): Duration and timestamp field comments moved to previous lines
    - `IOUsage message fields` (4 items): Bandwidth and usage fields
  - **Verification:**
    - ✅ Zero remaining inline comments (grep `= \d+; //` returns 0 matches)
    - ✅ buf lint passes with 0 errors/warnings
    - ✅ `go generate ./...` completes successfully
    - ✅ `go build ./...` completes successfully
  - **Benefit:** buf LSP now correctly displays field documentation on hover in VS Code
- **Pattern Applied:** For consistency across all proto files:
  ```protobuf
  // Good: Comment on previous line (recognized by buf LSP)
  enum Status {
    // Description of value
    STATUS_ACTIVE = 1;
  }

  message Example {
    // Description of field
    string field_name = 1;
  }

  // Bad (old pattern): Comment on same line (not recognized by buf LSP)
  enum Status {
    STATUS_ACTIVE = 1; // Description (NOT recognized)
  }
  ```

**Complete Enum Value Documentation (2026-03-21):**
- **Objective:** Ensure every enum value across all proto files has a preceding documentation comment
- **Scope:** All enums in vocabulary.proto, runtime.proto, and job_data.proto
- **Enums documented:**
  - **vocabulary.proto:**
    - `TimeUnit`: All 6 values (UNSPECIFIED, MILLISECOND, SECOND, MINUTE, HOUR, DAY)
    - `HashAlgorithm`: All 4 values (UNSPECIFIED, SHA256, SHA512, BLAKE3)
    - `SignatureAlgorithm`: Added UNSPECIFIED comment (other values already documented)
    - `SizeUnit`: All 6 values (UNSPECIFIED, BYTE, KILOBYTE, MEGABYTE, GIGABYTE, TERABYTE) - fixed incorrect BYTE comment
    - `ApiProtocol`: All 3 values (UNSPECIFIED, GRPC, REST) - improved descriptions
  - **runtime.proto:**
    - `CppLanguage`: All 3 values (UNSPECIFIED, C, CPP)
    - `CppCompiler`: All 3 values (UNSPECIFIED, GCC, CLANG)
    - `CppArchitecture`: All 4 values (UNSPECIFIED, X86_64, AARCH64, ARM)
    - `CRuntime`: All 3 values (UNSPECIFIED, GLIBC, MUSL)
    - `CppAbi`: Added UNSPECIFIED comment (ITANIUM already had one)
    - `CppStdlib`: Added UNSPECIFIED comment (LIBSTDCXX, LIBCXX already had comments)
  - **job_data.proto:**
    - `DataType`: All 5 values (UNSPECIFIED, FILE, DIRECTORY, STREAM_CHUNK, REFERENCE)
- **Verification:**
  - ✅ Zero undocumented enum values (grep `= \d+;` with preceding comment check returns 0)
  - ✅ buf lint passes with 0 errors/warnings
  - ✅ `go generate ./...` completes successfully
  - ✅ `go build ./...` completes successfully
- **Pattern Applied:** Every enum value has a comment on the previous line explaining its purpose:
  ```protobuf
  enum Status {
    // Unspecified status (default)
    STATUS_UNSPECIFIED = 0;
    // Active/running state
    STATUS_ACTIVE = 1;
    // Paused/suspended state
    STATUS_PAUSED = 2;
  }
  ```

**Protocol Organization & API Separation (2026-03-21):**
- **Objective:** Split the protocol into logically distinct packages to clarify the different APIs and their use cases
- **Separation:** Four distinct APIs with clear purposes:
  1. **Driver API** (`driver.proto`): Used by gcc/g++/make CLIs to submit jobs
  2. **Introspection API** (`introspection.proto`): Used by tools/CLI/UI to query client state
  3. **Peer APIs** (`executor.proto`, `discovery.proto`, `coordination.proto`): Used by clients to coordinate
  4. **Common Types** (`common/`): Shared vocabulary, job, runtime types used by all APIs
- **Package Structure:**
  - `buildozer.proto.v1.common` - Shared vocabulary types (TimeUnit, Hash, Signature, Size, Job, Runtime, etc.)
  - `buildozer.proto.v1.driver` - Driver API (JobService)
  - `buildozer.proto.v1.introspection` - Introspection API (IntrospectionService)
  - `buildozer.proto.v1.peer` - Peer APIs (ExecutorService, DiscoveryService, CoordinationService)
- **Shared Versioning:** All APIs are version `buildozer.proto.v1` (protocol changes are coordinated across all APIs)
- **buf Configuration:** Added exception for `PACKAGE_VERSION_SUFFIX` rule (not needed when all APIs share v1)
- **Generated Code:** Organized under `internal/gen/buildozer/proto/v1/{common,driver,introspection,peer}/` with Connect service handlers in `*connect/` subdirectories
- **Verification:**
  - ✅ buf lint: 0 errors/warnings (STANDARD rule set minus PACKAGE_VERSION_SUFFIX)
  - ✅ go generate: All 11 proto files compile successfully
  - ✅ go build: Builds successfully
  - ✅ Proto structure clearly separates four distinct APIs

---

## Milestone 1.3: Logging System Implementation (2026-03-21)

**Status:** ✅ COMPLETE - Production-Ready Logging with Age-Based Rotation & CLI Refactoring

### Phase 1: Library Integration - slog-multi + lumberjack (2026-03-21)

**Objective:** Leverage industry-standard libraries for file rotation instead of custom implementations.

**Libraries Integrated:**
- **lumberjack v2.2.1**: Handles file rotation by size (MaxSize in MB), backup count (MaxBackups), and age (MaxAge in days)
- **slog-multi v1.7.1**: Provides Fanout pattern for broadcasting logs to multiple handlers
- **Dependencies:** `gopkg.in/natefinch/lumberjack.v2` and `github.com/samber/slog-multi`

**Code Changes:**

1. **sinks.go Refactoring**
   - Replaced custom 120-line `FileSinkWithRotation` implementation with lumberjack
   - Created `FileSink(path, maxSizeMB, maxBackups, maxAgeDays)` function returning `slog.Handler`
   - Updated helper functions: `JSONFileSink()`, `TextFileSink()` to accept all rotation parameters
   - Embedded sink configuration into slog handlers (no custom iteration logic)

2. **config.go Updates**
   - Added `MaxAgeDays` field to `SinkConfig` struct
   - Updated `Factory.CreateSink()` to pass MaxAgeDays to FileSink()
   - Introduced slog-multi `Fanout()` pattern in `InitializeFromConfig()` for composite handlers

3. **logger.go Refactoring**
   - Changed Logger struct: removed `handlers []slog.Handler` array
   - Added `compositeHandler slog.Handler` field for single composed handler
   - Simplified `Log()` and `LogAttrs()` methods to delegate to compositeHandler if set
   - Added `SetCompositeHandler()` method for factory setup

4. **global.go Updates**
   - Updated `EnableLoggerFileSink()` signature to accept `maxAgeDays` parameter
   - Creates `SinkConfig` with age-based rotation settings

**Benefits:**
- Removed 250+ lines of custom rotation and handler composition code
- Battle-tested implementations replace fragile custom code
- Size-based, count-based, and age-based rotation all supported
- Single compositeHandler pattern cleaner than manual handler list iteration

**Build Status:**
- ✅ `go build ./...` succeeds
- ✅ All logging operations functional
- ✅ No breaking changes to public API

---

### Phase 2: Age-Based Log Rotation Feature (2026-03-21)

**Objective:** Support retention policies based on log file age (days).

**Implementation:**

1. **Configuration Enhancement**
   - Added `MaxAgeDays` field to `FileSinkConfig` (0 = no age-based rotation)
   - Updated YAML configuration schema: `max_age_days: 90`
   - Updated helper functions to accept maxAgeDays parameter

2. **Test Suite (4 tests, all passing)**
   - `TestFileSinkWithAgeRotation`: Verifies lumberjack MaxAge parameter set correctly
   - `TestFileSinkWithoutAgeRotation`: Verifies age rotation disabled when maxAgeDays=0
   - `TestJSONFileSinkWithAge`: Tests JSON sink with age-based rotation
   - `TestTextFileSinkWithAge`: Tests text sink with age-based rotation

3. **Example Usage**
   - Created `examples/logging_with_age_rotation.go` demonstrating:
     - File sink creation with age-based rotation
     - Multiple rotation strategies (size + age)
     - Real-world configuration patterns

**Benefits:**
- Automated cleanup of old log files
- Configurable retention windows (e.g., keep 7, 14, 30, or 90 days)
- Prevents unbounded log disk usage
- Production-ready retention policy

**Test Results:**
- ✅ All 4 tests passing
- ✅ Build succeeds with tests included

---

### Phase 3: CLI Redesign - From Flags to Subcommands (2026-03-21)

**Objective:** Refactor logs command to use proper subcommand pattern instead of flags.

**Design Principle Applied:** "In a CLI, different operations should always be different subcommands" (not flags)

**Old Design (Flag-Based):**
```bash
logs --status
logs --tail
logs --set-global-level debug
logs --set-logger-level buildozer --logger-level info
logs --enable-file-sink buildozer --file-sink-path /tmp/log
```

**New Design (Subcommand-Based):**
```bash
logs status
logs tail
logs set-global-level debug
logs set-logger-level buildozer info
logs enable-file-sink buildozer /tmp/log
```

**Implementation:**

1. **Subcommand Functions (7 total)**
   - `newLogsStatusCommand()` - Display logging configuration
   - `newLogsTailCommand()` - Stream logs in real-time
   - `newLogsSetGlobalLevelCommand()` - Change global logging level
   - `newLogsSetLoggerLevelCommand()` - Change level for specific logger
   - `newLogsSetSinkLevelCommand()` - Change level for specific sink
   - `newLogsEnableFileSinkCommand()` - Create file sink for logger
   - `newLogsDisableFileSinkCommand()` - Remove file sink from logger

2. **Cobra Integration**
   - Parent command `NewLogsCommand()` returns root with 7 subcommands
   - Each subcommand uses `cobra.ExactArgs()` for strict argument validation
   - Automatic help text generation per subcommand
   - Help: `logs --help`, `logs status --help`, `logs set-global-level --help`, etc.

3. **Error Handling**
   - Missing arguments: "Error: accepts X arg(s), received Y"
   - Invalid operations fail with clear Cobra error messages
   - All error messages follow Cobra standard format

4. **Code Cleanup**
   - Removed old `handleLogsInProcess()` and `handleLogsRemote()` functions
   - Removed 10+ boolean/string flags
   - Removed flag-based dispatch logic (~200 lines)

**Command Reference:**

| Operation | Old Flag-Based | New Subcommand | Args |
|-----------|---|---|---|
| View config | `logs --status` | `logs status` | 0 |
| Stream logs | `logs --tail` | `logs tail` | 0 |
| Set global level | `logs --set-global-level debug` | `logs set-global-level debug` | 1 (level) |
| Set logger level | `logs --set-logger-level buildozer --logger-level info` | `logs set-logger-level buildozer info` | 2 (name, level) |
| Set sink level | `logs --set-sink-level stdout --sink-level warn` | `logs set-sink-level stdout warn` | 2 (name, level) |
| Enable file sink | `logs --enable-file-sink buildozer --file-sink-path /tmp/log` | `logs enable-file-sink buildozer /tmp/log` | 2 (name, path) |
| Disable file sink | `logs --disable-file-sink buildozer` | `logs disable-file-sink buildozer` | 1 (name) |

**Benefits:**
- **Clarity**: Each operation is an explicit subcommand
- **Discoverability**: `logs --help` shows all 7 operations clearly
- **Standard Pattern**: Follows conventions of git, docker, kubectl
- **Validation**: Cobra automatically validates argument counts
- **Extensibility**: Easy to add new operations as new subcommands

**Testing performed:**
- ✅ All 7 subcommands functional
- ✅ Help text generation working
- ✅ Error handling for missing arguments
- ✅ Build succeeds
- ✅ No runtime errors

**Files Created/Updated:**
- ✅ `cmd/buildozer-client/cmd/logs.go` - Complete refactor (7 subcommands)
- ✅ `CLI_LOGGING_SUBCOMMANDS.md` - Complete command reference
- ✅ `pkg/logging/sinks/sinks.go` - lumberjack integration
- ✅ `pkg/logging/config.go` - Age-based rotation config
- ✅ `pkg/logging/logger.go` - Composite handler pattern
- ✅ `pkg/logging/global.go` - Updated API signatures
- ✅ `pkg/logging/sinks/sinks_test.go` - Age rotation tests

**Status Summary:**
- ✅ 250+ lines of custom code removed
- ✅ 4 comprehensive tests passing
- ✅ 7 subcommands fully implemented and tested
- ✅ Production-ready logging system
- ✅ Industry-standard libraries (slog-multi, lumberjack)
- ✅ Proper CLI pattern (subcommands, not flags)

---

### Phase 4: Logging Configuration Interface System (2026-03-21)

**Objective:** Create a pluggable logging configuration interface with local and remote implementations, plus RPC service handler.

**Architecture:**

1. **ConfigManager Interface** (`pkg/logging/config_manager.go`)
   - Unified interface for logging configuration operations
   - Methods: GetLoggingStatus, SetGlobalLevel, SetLoggerLevel, SetSinkLevel, EnableFileSink, DisableFileSink, TailLogs
   - Works with both local and remote implementations

2. **LocalConfigManager** (`pkg/logging/config_manager.go`)
   - Implements ConfigManager for local in-process logging
   - Uses existing Registry and Factory from pkg/logging
   - Direct access to logging configuration functions
   - No network overhead

3. **RemoteConfigManager** (`pkg/logging/remote_config.go`)
   - Implements ConfigManager for remote daemon communication
   - Uses Connect client to call LoggingService RPC methods
   - Same interface as LocalConfigManager for seamless switching
   - Handles protocol buffer conversion and error handling

4. **Private Service Handler** (`pkg/logging/service_handler.go`)
   - `loggingServiceHandler` struct (private implementation)
   - Implements `LoggingServiceHandler` from protocol (generated interface)
   - Uses ConfigManager interface internally (can be any implementation)
   - `RegisterLoggingService()` creates and registers handler with HTTP mux

5. **Convenience Factory** (`pkg/logging/factory.go`)
   - `NewLocalConfigManagerFromGlobal()` - Creates local manager from global registry
   - `NewRemoteConfigManagerFromURL()` - Creates remote manager from URL
   - `NewRemoteConfigManagerFromClient()` - Creates remote manager from explicit client
   - `GetLocalConfigManager()` - Simple accessor for global local manager
   - `NewHTTPHandler()` - Convenience for registering service handler

**Type Conversions:**

- `SlogLevelToProtoLogLevel()` - Convert slog.Level to protobuf enum
- `ProtoLogLevelToSlogLevel()` - Convert protobuf enum to slog.Level
- `sinkTypeFromString()` - Convert string to protobuf SinkType enum
- `sinkTypeToString()` - Convert protobuf enum to string
- `timeToTimestamp()` - Convert time.Time to protobuf Timestamp
- `timestampToTime()` - Convert protobuf Timestamp to time.Time

**Data Structures:**

- `LoggingStatusSnapshot` - Complete configuration snapshot with sinks and loggers
- `SinkStatus` - Individual sink configuration and level
- `LoggerStatus` - Individual logger configuration and level
- `LogRecord` - Single log record with timestamp, level, message, attributes

**Usage Examples:**

Local usage:
```go
manager := logging.GetLocalConfigManager()
status, err := manager.GetLoggingStatus(ctx)
err = manager.SetGlobalLevel(ctx, slog.LevelDebug)
err = manager.EnableFileSink(ctx, "buildozer", "/var/log/buildozer.log", 100, 10, 30)
```

Remote usage:
```go
manager := logging.NewRemoteConfigManagerFromURL(httpClient, "http://localhost:6789")
status, err := manager.GetLoggingStatus(ctx)
err = manager.SetGlobalLevel(ctx, slog.LevelDebug)
```

Service registration:
```go
manager := logging.GetLocalConfigManager()
path, handler := logging.RegisterLoggingService(manager)
mux.Handle(path, handler)
```

**Files Created/Modified:**
- ✅ `pkg/logging/config_manager.go` - ConfigManager interface and LocalConfigManager
- ✅ `pkg/logging/remote_config.go` - RemoteConfigManager implementation
- ✅ `pkg/logging/service_handler.go` - Private loggingServiceHandler
- ✅ `pkg/logging/factory.go` - Convenience factory functions
- ✅ `pkg/logging/CONFIG_MANAGER.md` - Comprehensive documentation

**Status Summary:**
- ✅ ConfigManager interface fully defined
- ✅ LocalConfigManager implements interface
- ✅ RemoteConfigManager implements interface
- ✅ Service handler implements LoggingServiceHandler
- ✅ Type conversions complete (slog ↔ protobuf)
- ✅ Convenience API for easy integration
- ✅ Project builds successfully
- ✅ Ready for CLI and daemon integration

---

### Phase 5: Protocol Service Definition - LoggingService (2026-03-21)

**Objective:** Define protocol buffer RPC service for daemon logging configuration.

**Service Definition:**

Created [buildozer/proto/v1/logging.proto](buildozer/proto/v1/logging.proto) with:

1. **LoggingService** - 7 RPC methods
   - `GetLoggingStatus()` - Retrieve current logging configuration
   - `SetGlobalLevel()` - Change global logging level
   - `SetLoggerLevel()` - Change specific logger level
   - `SetSinkLevel()` - Change specific sink level
   - `EnableFileSink()` - Create file sink for logger
   - `DisableFileSink()` - Remove file sink from logger
   - `TailLogs()` - Stream logs in real-time (with filtering)

2. **Message Types**
   - `LogLevel` enum - error, warn, info, debug, trace
   - `SinkType` enum - stdout, stderr, file, syslog
   - `SinkConfig` - Sink configuration with file/syslog options
   - `LoggerConfig` - Named logger configuration
   - `LoggingStatus` - Complete logging state snapshot
   - Request/response messages for each RPC operation
   - `TailLogsResponse` - Streamed log records

3. **Type-Safe Configuration**
   - Enums for LogLevel and SinkType (instead of strings)
   - Nested message types for file and syslog configuration
   - FileConfig: path, max_size_bytes, max_backups, max_age_days, json_format
   - SyslogConfig: tag
   - TimeStamp vocabulary type for all response timestamps

**Protocol Generation:**

- ✅ `buf generate` produces:
  - `logging.pb.go` - Message type definitions
  - `logging.connect.go` - Connect RPC handlers and client
- ✅ Project compiles successfully
- ✅ All 7 RPC methods available for service implementation

**Service Architecture:**

```
CLI (buildozer-client logs commands)
        ↓
Connect Client (generated from LoggingService)
        ↓
Network (HTTP/gRPC/gRPC-Web)
        ↓
Connect Server Handler (to be implemented)
        ↓
In-Process Logging System (pkg/logging)
```

**Integration Points:**

- CLI commands will use `NewLoggingServiceClient()` to invoke RPC methods
- Daemon will implement `LoggingServiceHandler` interface
- Request/response types match CLI operations exactly
- TailLogs supports streaming for real-time log monitoring

**Status Summary:**
- ✅ Service defined with 7 RPC methods
- ✅ Type-safe enums for LogLevel and SinkType
- ✅ Comprehensive request/response messages
- ✅ Connect code generated successfully
- ✅ Ready for daemon implementation

---

### Milestone 1.4: Daemon Package & Service Orchestration (2026-03-21)

**Status:** ✅ COMPLETE

**Objective:** Create a high-level daemon package that collects all subsystems and exposes them through a unified Connect/gRPC server.

**Key Architecture Decisions:**

1. **Daemon Core** (`pkg/daemon/daemon.go`)
   - Main `Daemon` struct managing HTTP/Connect server and service registration
   - Thread-safe lifecycle management (Start/Stop with RWMutex)
   - Service handler registration interface for plugging in services
   - Graceful shutdown with context cancellation

2. **Server Wrapper** (`pkg/daemon/server.go`)
   - High-level `Server` type for typical daemon setup
   - Initializes all standard services automatically (logging, runtime detection, etc.)
   - Single entry point for daemon CLI command
   - Provides access to underlying components when needed

3. **Builder Pattern** (`pkg/daemon/options.go`)
   - Fluent builder for flexible daemon configuration
   - Sensible defaults: Host=localhost, Port=6789, MaxJobs=4, MaxRAM=4GB
   - Configuration validation on builder methods

**Completed Components:**

1. **`pkg/daemon/daemon.go`** (160 lines)
   - `DaemonConfig` struct with network, resource, and feature configuration
   - `Daemon` struct with HTTP server, mux, and lifecycle management
   - Methods: `New()`, `Start()`, `Stop()`, `RegisterServiceHandler()`
   - State queries: `IsRunning()`, `Context()`, `Config()`
   - Thread-safe with RWMutex for concurrent access
   - Graceful shutdown with context timeout support

2. **`pkg/daemon/server.go`** (100 lines)
   - `Server` wrapper type for high-level daemon setup
   - `NewServer()` initializes logging service and registers it with daemon
   - Methods: `Start()`, `Stop()`, `IsRunning()`, `Context()`, `Config()`
   - Access to logging config manager: `LoggingConfigManager()`
   - Access to underlying daemon: `Daemon()`

3. **`pkg/daemon/options.go`** (70 lines)
   - `Builder` type with chainable methods
   - Methods: `Host()`, `Port()`, `MaxConcurrentJobs()`, `MaxRAMMB()`, `EnableMDNS()`
   - Validation: Port range (1-65535), positive max jobs/RAM
   - Methods: `Build()` creates daemon, `BuildWithConfig()` from explicit config

4. **`pkg/daemon/README.md`** (150 lines)
   - Architecture overview with ASCII diagram
   - Usage examples for standalone daemon and builder pattern
   - Service registration pattern documentation
   - Graceful shutdown implementation guide
   - Thread safety guarantees
   - Integration with cmd/buildozer-client

5. **Integration with CLI** (`cmd/buildozer-client/cmd/daemon.go`)
   - Updated daemon command to use new `daemon.Server`
   - Creates and starts daemon with CLI configuration
   - Implements graceful shutdown with signal handling
   - Timeout-based server shutdown (30 second default)

**Service Registration Pattern:**

Each service (logging, runtime, job, etc.) follows this pattern:
```go
// Service implements handler interface
type MyServiceHandler struct { ... }

// Registration function returns path and http.Handler
func RegisterMyService(config Config) (string, http.Handler) {
    handler := newMyServiceHandler(config)
    path, mux := protov1connect.NewMyServiceHandler(handler)
    return path, mux
}

// Daemon registers it
server.Daemon().RegisterServiceHandler(path, handler)
```

**Currently Registered Services:**
- `LoggingService` — Query and modify logging configuration at runtime

**Design Principles:**

1. **Separation of Concerns** — Each subsystem handles its domain; daemon orchestrates
2. **Composition Over Inheritance** — Services composed into daemon, not inherited
3. **Explicit Dependencies** — All dependencies explicitly injected/registered
4. **Graceful Degradation** — Services can be optional; daemon still works
5. **Thread Safety** — RWMutex protects state; safe for concurrent access
6. **Testability** — Clean interfaces enable mocking and testing

**Build Status:**
- ✅ `go build ./pkg/daemon` — Success
- ✅ `go build ./cmd/buildozer-client` — Success
- ✅ `./buildozer-client daemon --help` — Works correctly
- No lint errors or warnings
- Full project builds successfully

**Future Service Integration Points:**

As development progresses, services will be registered in `daemon.NewServer()`:

1. **RuntimeService** — Discover runtimes, query metadata, detect toolchains
2. **JobService** — Submit jobs, monitor progress, retrieve results
3. **CacheService** — Query cache status, manage artifacts, garbage collection
4. **QueueService** — Monitor job queue, scheduler status, load distribution
5. **PeerService** — Peer discovery (mDNS), connectivity status, statistics

**Documentation:**
- ✅ Comprehensive README at `pkg/daemon/README.md`
- ✅ Inline code comments for all public types and methods
- ✅ Usage examples in README and code
- ✅ Architecture diagram in README

**Next Steps:**
- Implement remaining Docker API methods (Milestone 1.0 continuation)
- Implement native C/C++ toolchain detector (Milestone 1.1)
- Implement Docker-based C/C++ runtime (Milestone 1.2)
- Integrate job queue and scheduler into daemon (Milestone 1.3 continuation)

---

## Logger Interface Refactoring Complete (2026-03-21)

**Status:** ✅ COMPLETE

**Completed Work:**

1. **Full slog.Logger Interface Implementation**
   - Implemented ALL slog.Logger methods:
     - **Log levels:** Debug, DebugContext, Info, InfoContext, Warn, WarnContext, Error, ErrorContext
     - **Generic logging:** Log, LogContext, LogAttrs, LogAttrsContext
     - **Attributes:** WithAttrs(), WithGroup() (no-ops for dynamic routing, maintain interface)
   - All methods delegate to underlying slog.Logger with proper context handling
   - Line counts: 418 lines in complete logger.go

2. **Dynamic Handler Routing Implementation**
   - Created `registryHandler` type implementing slog.Handler interface
   - Handler routes all log records through Registry.Log() for dynamic sink resolution
   - Supports hierarchical logger name tracking via "_logger" attribute
   - Enables runtime reconfiguration without logger recreation

3. **Registry Enhancements for Dynamic Routing**
   - `Registry.Log(ctx, record)` — Routes records to configured sinks using hierarchical lookup
   - Hierarchical lookup: exact match → parent loggers → default
   - Thread-safe with RWMutex for concurrent access
   - Full sink management API (register, get, configure levels)

4. **Custom Logger Methods**
   - `Child(name)` — Create child logger maintaining hierarchy (e.g., "parent" + "module" = "parent.module")
   - `Errorf(format, args)` — Log error and return error object
   - `Panicf(format, args)` — Log error and panic with formatted message
   - `Name()` — Get logger's hierarchical name
   - All custom methods properly maintain registry and name context

**Key Architecture Decisions:**

1. **No Persistent Logger Storage** — Loggers created on-the-fly per GetLogger() call
2. **Registry Stores Only Sinks** — loggerConfigs maps logger names to sink names; sinks are actual handlers
3. **Dynamic Routing** — Log records routed at runtime based on current configuration
4. **Hierarchical Lookup** — Settings inherit from parent loggers (e.g., "a.b" inherits from "a")
5. **Complete Interface Compliance** — Logger implements full slog.Logger interface plus custom methods

**Files Modified/Created:**

1. ✅ `pkg/logging/logger.go` — Completely rewritten (418 lines)
   - Logger type wrapping slog.Logger with dynamic routing
   - Registry type managing sinks and configurations
   - registryHandler implementation for slog.Handler interface
   - All slog.Logger methods (Debug, Info, Error, etc.)
   - Custom methods (Child, Errorf, Panicf)

2. ✅ `pkg/logging/global.go` — Updated for new Registry API
3. ✅ `pkg/logging/config.go` — Updated initialization, removed slog-multi
4. ✅ `pkg/logging/config_manager.go` — Updated LoggerStatus structure
5. ✅ `pkg/logging/remote_config.go` — Updated status conversion
6. ✅ `pkg/logging/service_handler.go` — Updated LoggerConfig creation
7. ✅ `buildozer/proto/v1/logging.proto` — Removed LogLevel from LoggerConfig (regenerated with buf)

**Compilation Results:**
- ✅ `go build ./cmd/buildozer-client` — Success (no errors, no warnings)
- ✅ `go build ./pkg/logging` — Success
- ✅ All dependent files compile correctly
- ✅ Proto regeneration successful with `buf generate`

**Testing Validation:**
- Logger methods accessible and callable
- Hierarchical naming works correctly (e.g., logger.Child() extends name)
- Registry sink routing functional
- All required methods present for slog.Logger interface
- Errorf() and Panicf() work as expected

**Design Benefits:**

1. **Full Logging Interface Compliance** — Logger implements complete slog.Logger API
2. **Dynamic Reconfiguration** — Sinks and routes can change without recreating loggers
3. **Hierarchical Configuration** — Parent logger settings apply to children
4. **Zero Boilerplate** — No need to create and manage Logger instances
5. **Clean Separation** — Registry handles routing, Logger handles interface
6. **Thread-Safe** — All state protected with RWMutex

**Remaining TODOs (Ordered by Dependency):**

1. **Performance Validation** — Benchmark hierarchical lookup vs. cached loggers (if needed)
2. **Error Detection in Registry.Log()** — Add error handling for failed sink writes
3. **Attribute Filtering** — Consider filtering "_logger" attribute from final output
4. **Connection to Services**:
   - LoggingService integration with new Logger interface
   - Remote config setting via logging.proto RPC
   - Status queries via LoggerStatus proto
5. **Test Suite** — Unit tests for Logger, Registry, Child(), Errorf(), Panicf()
6. **Documentation** — Update code comments for new dynamic architecture

**Next Steps:**

1. ✅ Complete logger interface (THIS STEP COMPLETE)
2. → Implement test suite for logging package
3. → Validate with real-world logging scenarios
4. → Move to Docker API abstraction (Milestone 1.0)

---

## Logger Attributes & Groups Support (2026-03-21)

**Status:** ✅ COMPLETE

**Enhancement Overview:**

Updated Logger to fully support fixed attributes and groups, enabling loggers to carry context through their lifecycle.

**Completed Features:**

1. **WithAttrs() Now Fully Implemented**
   - Accumulates attributes that are included in all subsequent log calls
   - Returns a new Logger instance with accumulated attributes
   - Attributes properly combined with message-level attributes
   - Multiple WithAttrs() calls stack attributes

2. **WithGroup() Now Fully Implemented**
   - Sets a group context for all subsequent log calls
   - Returns a new Logger instance with group set
   - Group attributes properly scoped

3. **Attribute Inheritance in Child Loggers**
   - Child loggers inherit accumulated attributes from parents  
   - Child can add additional attributes on top of inherited ones
   - Full attribute hierarchy preserved through logger tree
   - Example: parent with `user=alice` creates child that logs with `user=alice` + child's new attrs

4. **Logger Struct Enhancements**
   - Added `attrs []slog.Attr` field to track accumulated attributes
   - Added `group string` field for group context
   - Thread-safe with RWMutex for concurrent access

5. **Logging Method Updates**
   - `log()` method now combines accumulated attrs with varargs
   - `logAttrs()` method appends accumulated attrs to provided attrs
   - Both methods properly handle edge cases (no attrs, attrs + varargs, attrs + LogAttrs)
   - Automatic conversion of varargs to slog.Attr when accumulated attrs present

**Test Coverage:**

Created comprehensive test suite in `logger_test.go`:
- **TestLoggerWithAttrs**: Verifies basic WithAttrs() functionality
  - Output: `user=alice id=42` properly included in messages
- **TestLoggerWithGroup**: Verifies WithGroup() context setting
- **TestLoggerHierarchy**: Verifies attribute accumulation through hierarchy
  - Output shows all three levels: `env=prod type=postgres host=localhost`
  - Demonstrates full attribute inheritance through nested Child() calls

All tests pass with proper attribute accumulation verified in output.

**Example Usage:**

```go
// Parent logger with environment
appLog := registry.GetLogger("app").WithAttrs(slog.String("env", "prod"))

// Database module inherits env, adds type
dbLog := appLog.Child("db").WithAttrs(slog.String("type", "postgres"))

// Postgres component inherits both, adds specific host
pgLog := dbLog.Child("postgres").WithAttrs(slog.String("host", "localhost"))

// All attributes present: env=prod type=postgres host=localhost
pgLog.Info("connected")
```

**Technical Implementation Details:**

1. **Child() Inheritance**
   - Updated to copy parent's accumulated attrs and group to child
   - Child loggers maintain full context from parent chain

2. **Log Call Handling**
   - Varargs form (msg, key1, val1, key2, val2...):
     - If accumulated attrs: convert varargs to slog.Attr and combine
     - If no accumulated attrs: use standard Log() call
   - LogAttrs form: append accumulated attrs to provided attrs

3. **Thread Safety**
   - RWMutex protects attrs and group fields
   - No race conditions in concurrent access
   - Copy-on-write semantics for new logger instances

**Files Modified:**

- ✅ `pkg/logging/logger.go` — Enhanced Logger struct and methods (added ~80 lines)
- ✅ `pkg/logging/logger_test.go` — NEW file with comprehensive tests (~160 lines)

**Compilation & Validation:**

- ✅ Full build succeeds: `go build ./cmd/buildozer-client`
- ✅ All tests pass: 3/3 Logger attribute/group tests passing
- ✅ Zero compiler errors or warnings
- ✅ Proper attribute values verified in test output

**Remaining TODOs:**

1. ~~Implement fixed attributes support~~ ✅ DONE
2. ~~Implement fixed group support~~ ✅ DONE
3. Attribute Filtering — Consider filtering "_logger" attribute from final output
4. Error Detection in Registry.Log() — Add error handling for failed sink writes
5. Connection to Services:
   - LoggingService integration with new Logger interface
   - Remote config setting via logging.proto RPC
   - Status queries via LoggerStatus proto
6. Performance Validation — Benchmark attribute inheritance (if needed)

**Next Steps:**

1. ✅ Complete logger interface with attributes/groups (COMPLETE)
2. → Move to Docker API abstraction (Milestone 1.0)
3. → Implement native C/C++ toolchain detector (Milestone 1.1)
4. → Implement Docker-based C/C++ runtime (Milestone 1.2)

---

## Next Phase: Step 2 - Job & Runtime Abstractions
