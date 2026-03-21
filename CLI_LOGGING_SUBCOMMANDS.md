# Logging CLI Refactoring - Subcommands

## Overview
Refactored the `logs` command to use proper subcommands instead of flags, following CLI best practices where each operation is a distinct subcommand.

## New Command Structure

### Parent Command
```bash
buildozer-client logs [subcommand] [args]
```

### Available Subcommands

#### 1. View Logging Status
```bash
buildozer-client logs status
```
Shows current logging configuration including global level, sinks, and loggers.

#### 2. Tail Logs
```bash
buildozer-client logs tail
```
Stream logs from the running daemon (currently a placeholder).

#### 3. Set Global Logging Level
```bash
buildozer-client logs set-global-level <level>
```
Set the global logging level. Valid levels: error, warn, info, debug, trace

Examples:
```bash
buildozer-client --standalone logs set-global-level debug
buildozer-client --standalone logs set-global-level trace
```

#### 4. Set Logger-Specific Level
```bash
buildozer-client logs set-logger-level <logger-name> <level>
```
Set the logging level for a specific logger.

Examples:
```bash
buildozer-client --standalone logs set-logger-level buildozer info
buildozer-client --standalone logs set-logger-level buildozer.runtime debug
```

#### 5. Set Sink-Specific Level
```bash
buildozer-client logs set-sink-level <sink-name> <level>
```
Set the logging level for a specific sink (stdout, stderr, etc).

Examples:
```bash
buildozer-client --standalone logs set-sink-level stdout debug
buildozer-client --standalone logs set-sink-level stderr warn
```

#### 6. Enable Logger-Specific File Sink
```bash
buildozer-client logs enable-file-sink <logger-name> <file-path>
```
Enable a rotating file sink for a specific logger.

Examples:
```bash
buildozer-client --standalone logs enable-file-sink buildozer /var/log/buildozer.log
buildozer-client --standalone logs enable-file-sink buildozer.runtime /tmp/runtime-debug.log
```

#### 7. Disable Logger-Specific File Sink
```bash
buildozer-client logs disable-file-sink <logger-name>
```
Disable the file sink for a specific logger.

Examples:
```bash
buildozer-client --standalone logs disable-file-sink buildozer
buildozer-client --standalone logs disable-file-sink buildozer.runtime
```

## Global Flags
All subcommands support these global flags:
- `--standalone` - Operate on in-process daemon instead of connecting to remote daemon
- `--config <path>` - Specify config file path
- `--host <host>` - Daemon host address (for remote mode)
- `--port <port>` - Daemon gRPC port (for remote mode)

## Error Handling
Invalid arguments show helpful error messages:
```bash
$ ./buildozer-client logs set-global-level
Error: accepts 1 arg(s), received 0
Usage:
  buildozer-client logs set-global-level <level> [flags]
```

## Help
Get help for any subcommand:
```bash
buildozer-client logs --help
buildozer-client logs status --help
buildozer-client logs set-global-level --help
```

## Migration from Old Flag-Based API
Old API → New API:
```
logs --status           → logs status
logs --tail             → logs tail
logs --set-global-level debug      → logs set-global-level debug
logs --set-logger-level buildozer --logger-level info  → logs set-logger-level buildozer info
logs --set-sink-level stdout --sink-level warn         → logs set-sink-level stdout warn
logs --enable-file-sink buildozer --file-sink-path /tmp/file.log  → logs enable-file-sink buildozer /tmp/file.log
logs --disable-file-sink buildozer  → logs disable-file-sink buildozer
```

## Benefits
1. **Clarity** - Each operation is explicitly a subcommand
2. **Discoverability** - `logs --help` clearly shows available operations
3. **Standard CLI Pattern** - Follows conventions used by git, docker, kubectl
4. **Argument Validation** - Cobra automatically validates required arguments
5. **Better Help Text** - Context-specific help for each operation
