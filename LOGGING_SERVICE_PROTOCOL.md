# LoggingService Protocol Definition

## Overview

`LoggingService` is a Connect RPC service defined in [buildozer/proto/v1/logging.proto](buildozer/proto/v1/logging.proto) that exposes logging configuration operations over the network. It enables:

1. **Remote Logging Control** - CLI can configure daemon logging from any network location
2. **Type-Safe API** - Uses protobuf enums and messages instead of strings
3. **Real-Time Streaming** - TailLogs supports streaming log records for monitoring
4. **Complete State Management** - GetLoggingStatus returns full current configuration

## Service Interface

### Service Definition

```protobuf
service LoggingService {
  rpc GetLoggingStatus(GetLoggingStatusRequest) returns (GetLoggingStatusResponse);
  rpc SetGlobalLevel(SetGlobalLevelRequest) returns (SetGlobalLevelResponse);
  rpc SetLoggerLevel(SetLoggerLevelRequest) returns (SetLoggerLevelResponse);
  rpc SetSinkLevel(SetSinkLevelRequest) returns (SetSinkLevelResponse);
  rpc EnableFileSink(EnableFileSinkRequest) returns (EnableFileSinkResponse);
  rpc DisableFileSink(DisableFileSinkRequest) returns (DisableFileSinkResponse);
  rpc TailLogs(TailLogsRequest) returns (stream TailLogsResponse);
}
```

## RPC Methods

### 1. GetLoggingStatus

**Purpose:** Retrieve the current logging configuration including all sinks and loggers.

**Request:** `GetLoggingStatusRequest` (empty)

**Response:** `GetLoggingStatusResponse`
- `LoggingStatus status`
  - `LogLevel global_level` - Global logging level
  - `repeated SinkConfig sinks` - All configured sinks
  - `repeated LoggerConfig loggers` - All configured loggers
  - `TimeStamp retrieved_at` - When configuration was retrieved

**Usage:**
```bash
buildozer-client logs status
```

**Maps to CLI command:**
```go
logs status  // Calls GetLoggingStatus() and displays JSON response
```

---

### 2. SetGlobalLevel

**Purpose:** Change the global logging level affecting all loggers and sinks.

**Request:** `SetGlobalLevelRequest`
- `LogLevel level` - New global level (ERROR, WARN, INFO, DEBUG, TRACE)

**Response:** `SetGlobalLevelResponse`
- `LogLevel level` - Confirmed level
- `uint32 affected_loggers` - Number of loggers affected
- `TimeStamp updated_at` - When change was applied

**Usage:**
```bash
buildozer-client logs set-global-level debug
buildozer-client logs set-global-level trace
```

**Maps to CLI command:**
```go
logs set-global-level <level>  // Calls SetGlobalLevel(level) with argument
```

---

### 3. SetLoggerLevel

**Purpose:** Change logging level for a specific named logger (affects only that logger).

**Request:** `SetLoggerLevelRequest`
- `string logger_name` - Logger name (e.g., "buildozer", "buildozer.runtime")
- `LogLevel level` - New level for this logger

**Response:** `SetLoggerLevelResponse`
- `string logger_name` - Confirmed logger name
- `LogLevel level` - Confirmed level
- `bool created_new_logger` - Whether a new logger was created
- `TimeStamp updated_at` - When change was applied

**Usage:**
```bash
buildozer-client logs set-logger-level buildozer info
buildozer-client logs set-logger-level buildozer.runtime debug
```

**Maps to CLI command:**
```go
logs set-logger-level <logger-name> <level>  // Calls SetLoggerLevel(name, level)
```

---

### 4. SetSinkLevel

**Purpose:** Change logging level for a specific sink (stdout, stderr, file, etc.).

**Request:** `SetSinkLevelRequest`
- `string sink_name` - Sink name (e.g., "stdout", "stderr", "file_1")
- `LogLevel level` - New level for this sink

**Response:** `SetSinkLevelResponse`
- `string sink_name` - Confirmed sink name
- `LogLevel level` - Confirmed level
- `TimeStamp updated_at` - When change was applied

**Usage:**
```bash
buildozer-client logs set-sink-level stdout debug
buildozer-client logs set-sink-level stderr warn
```

**Maps to CLI command:**
```go
logs set-sink-level <sink-name> <level>  // Calls SetSinkLevel(name, level)
```

---

### 5. EnableFileSink

**Purpose:** Create a new rotating file sink for a logger.

**Request:** `EnableFileSinkRequest`
- `string logger_name` - Logger to attach sink to
- `string file_path` - Absolute path to log file
- `int64 max_size_bytes` - Max file size before rotation (0 = no limit, default 100MB)
- `int32 max_backups` - Max backups to keep (0 = unlimited, default 10)
- `int32 max_age_days` - Max age before deletion (0 = no limit, default 0)
- `bool json_format` - Whether to use JSON format

**Response:** `EnableFileSinkResponse`
- `string sink_name` - Generated sink name
- `string logger_name` - Logger it was attached to
- `string file_path` - File path being used
- `TimeStamp created_at` - When sink was created

**Usage:**
```bash
buildozer-client logs enable-file-sink buildozer /var/log/buildozer.log
buildozer-client logs enable-file-sink buildozer.runtime /tmp/runtime-debug.log
```

**Maps to CLI command:**
```go
logs enable-file-sink <logger-name> <file-path>  // Creates file sink with defaults
```

**Defaults in CLI:**
- `max_size_bytes` = 104857600 (100MB)
- `max_backups` = 10
- `max_age_days` = 0 (no age-based rotation)
- `json_format` = false (text format)

---

### 6. DisableFileSink

**Purpose:** Remove a file-based sink from a logger.

**Request:** `DisableFileSinkRequest`
- `string logger_name` - Logger with the sink
- `optional string sink_name` - Specific sink to remove (if empty, removes most recent file sink)

**Response:** `DisableFileSinkResponse`
- `string logger_name` - Confirmed logger
- `string sink_name` - Removed sink name
- `optional string file_path` - File that was removed
- `TimeStamp removed_at` - When sink was removed

**Usage:**
```bash
buildozer-client logs disable-file-sink buildozer
buildozer-client logs disable-file-sink buildozer.runtime
```

**Maps to CLI command:**
```go
logs disable-file-sink <logger-name>  // Calls DisableFileSink(name)
```

---

### 7. TailLogs

**Purpose:** Stream log records in real-time with optional filtering.

**Request:** `TailLogsRequest`
- `repeated LogLevel levels` - Filter by log levels (empty = all)
- `string logger_filter` - Logger name filter (supports wildcards like "buildozer*")
- `int32 history_lines` - Number of historical lines to retrieve first
- `bool follow` - Whether to stream new logs after history

**Response:** `TailLogsResponse` (streamed)
- `TimeStamp timestamp` - When log was created
- `string logger_name` - Logger that produced it
- `LogLevel level` - Log level
- `string message` - Log message
- `map<string, string> attributes` - Additional fields (JSON encoded)

**Usage:**
```bash
buildozer-client logs tail
```

**Maps to CLI command:**
```go
logs tail  // Calls TailLogs(follow=true) and streams output
```

---

## Type Definitions

### LogLevel Enum

```protobuf
enum LogLevel {
  LOG_LEVEL_UNSPECIFIED = 0;
  LOG_LEVEL_ERROR = 1;        // Only errors
  LOG_LEVEL_WARN = 2;         // Warnings and above
  LOG_LEVEL_INFO = 3;         // Informational messages and above
  LOG_LEVEL_DEBUG = 4;        // Detailed debugging
  LOG_LEVEL_TRACE = 5;        // Very detailed tracing
}
```

### SinkType Enum

```protobuf
enum SinkType {
  SINK_TYPE_UNSPECIFIED = 0;
  SINK_TYPE_STDOUT = 1;       // Standard output
  SINK_TYPE_STDERR = 2;       // Standard error
  SINK_TYPE_FILE = 3;         // File with rotation
  SINK_TYPE_SYSLOG = 4;       // Syslog
}
```

### SinkConfig Message

```protobuf
message SinkConfig {
  string name = 1;                           // Sink name
  SinkType type = 2;                         // Sink type
  LogLevel level = 3;                        // Log level threshold
  optional FileConfig file_config = 4;       // File-specific config
  optional SyslogConfig syslog_config = 5;   // Syslog-specific config
}
```

#### FileConfig (nested in SinkConfig)

```protobuf
message FileConfig {
  string path = 1;                    // File path
  int64 max_size_bytes = 2;          // Max size before rotation
  int32 max_backups = 3;             // Max backups to keep
  int32 max_age_days = 4;            // Max age before deletion
  bool json_format = 5;              // JSON or text format
}
```

### LoggerConfig Message

```protobuf
message LoggerConfig {
  string name = 1;                // Logger name (hierarchical)
  LogLevel level = 2;             // Logger-specific level
  repeated string sink_names = 3; // Sinks this logger outputs to
}
```

### LoggingStatus Message

```protobuf
message LoggingStatus {
  LogLevel global_level = 1;           // Global level
  repeated SinkConfig sinks = 2;       // All sinks
  repeated LoggerConfig loggers = 3;   // All loggers
  TimeStamp retrieved_at = 4;          // When retrieved
}
```

---

## Protocol vs. CLI Mapping

| CLI Command | Protocol RPC | Request Type | Response Type |
|---|---|---|---|
| `logs status` | `GetLoggingStatus` | Empty | `LoggingStatus` |
| `logs tail` | `TailLogs` | `logger_filter=""`, `follow=true` | stream `TailLogsResponse` |
| `logs set-global-level <level>` | `SetGlobalLevel` | `level` | `level`, `affected_loggers` |
| `logs set-logger-level <name> <level>` | `SetLoggerLevel` | `logger_name`, `level` | `logger_name`, `level` |
| `logs set-sink-level <name> <level>` | `SetSinkLevel` | `sink_name`, `level` | `sink_name`, `level` |
| `logs enable-file-sink <name> <path>` | `EnableFileSink` | `logger_name`, `file_path`, defaults | `sink_name`, `logger_name` |
| `logs disable-file-sink <name>` | `DisableFileSink` | `logger_name` | `sink_name`, `file_path` |

---

## Implementation Status

- ✅ Protocol defined in `logging.proto`
- ✅ Connect service generated (`logging.connect.go`)
- ✅ Message types generated (`logging.pb.go`)
- ✅ Ready for daemon server implementation
- ⏳ Pending: Daemon server implementing `LoggingServiceHandler` interface

---

## Next Steps

1. **Daemon Integration**: Implement `LoggingServiceHandler` in the daemon
2. **CLI Client**: Integrate `LoggingServiceClient` in buildozer-client
3. **Remote Mode**: Wire CLI to use remote daemon instead of in-process logging
4. **Testing**: Add integration tests for all RPC operations
