# Driver Execution Architecture

This document traces the full execution path of a driver, from `main.go` entry point to driver-specific code.

## Call Flow

```
main.go
  └─ drivers.ExecuteDriver(gxx.NewDriver())        # driver_cli.go
       └─ cobra.Command.Execute()
            └─ runDriver(cmd, args, d)              # driver_cli.go
                 ├─ StandardDriverFlags.Parse(args)  # flagparser.go
                 ├─ d.ValidateArgs(parsedArgs)       # → driver-specific validation
                 └─ RunDriver(ctx, d, args, config)  # driver_orchestrator.go
                      ├─ d.ParseCommandLine(args)    # → driver-specific parsing
                      ├─ resolver.Resolve(ctx, ..., d) # runtime_resolution.go (calls d.ApplyToolArgs)
                      ├─ d.ValidateRuntime(runtime)   # → driver-specific runtime check
                      ├─ d.CreateJob(ctx,parsed,rt)  # → driver-specific job creation
                      ├─ SubmitJob(...)              # job_submission.go
                      └─ WatchAndStreamJobProgress() # job_submission.go
```

## Execution Steps

### 1. Entry — `cmd/drivers/cpp/<tool>/main.go`

Each driver binary has a one-line `main.go`:

```go
func main() {
    drivers.ExecuteDriver(gxx.NewDriver())
}
```

`NewDriver()` returns a type implementing `driver.Driver`, which is handed to the generic framework.

### 2. Interface — `pkg/driver/driver.go`

The `Driver` interface defines the contract. It is purely generic with no language-specific types:

| Method | Purpose |
|---|---|
| `Name()` | Tool name for CLI, logging, identification |
| `Version()` | Version string for `--version` |
| `Short()` | One-line help description |
| `Long()` | Extended help description |
| `ErrorPrefix()` | Prefix for error messages (e.g. `"gcc: error:"`) |
| `ValidateArgs(args) error` | Driver-specific argument validation |
| `ParseCommandLine(args) interface{}` | Parse raw args into driver-specific representation |
| `CreateJob(ctx, parsed, runtime, workDir) (*v1.Job, error)` | Build a Job proto from parsed args and runtime |
| `ApplyToolArgs(ctx, runtime, toolArgs) (*v1.Runtime, error)` | Modify runtime descriptor based on tool flags |
| `ValidateRuntime(runtime) (bool, string)` | Check runtime compatibility |

### 3. CLI Setup — `pkg/drivers/driver_cli.go` → `ExecuteDriver(d)`

1. Creates a **cobra.Command** using `d.Name()`, `d.Short()`, etc.
2. Cobra invokes `runDriver(cmd, args, d)`:
   - Handles `--help`
   - Parses buildozer-specific flags via `StandardDriverFlags.Parse(args)` — strips `--buildozer-*` flags, returns tool args
   - Extracts `CommonDriverConfig` (daemon host/port, standalone, log level)
   - Calls **`d.ValidateArgs(parsedArgs)`** — dispatches to driver-specific validation
   - Handles `--version` — prints `d.Version()`
   - Handles `--buildozer-list-runtimes` — calls `ListCompatibleRuntimes(ctx, d, config)` (uses `d.ValidateRuntime()`)
   - Builds `DriverConfig` and calls **`RunDriver(ctx, d, parsedArgs, driverConfig)`**

### 4. Orchestration — `pkg/drivers/driver_orchestrator.go` → `RunDriver(ctx, d, args, config)`

The generic algorithm, calling driver callbacks at each step:

1. **Standalone daemon** — if `config.Standalone`, starts an in-process `daemon.Daemon`
2. **`d.ParseCommandLine(args)`** — returns an opaque `parsed` value (driver-specific)
3. **Runtime resolution** — `NewRuntimeResolver()` + `resolver.Resolve(ctx, ..., d)` — resolver calls `d.ApplyToolArgs()` to adjust runtime descriptor based on flags like `-march`, `-std`
4. **`d.ValidateRuntime(runtime)`** — validates the resolved runtime is compatible
5. **`d.CreateJob(ctx, parsed, runtime, workDir)`** — builds a `*v1.Job` proto
6. **`SubmitJob()`** — sends job to daemon via gRPC
7. **`WatchAndStreamJobProgress()`** — streams output to stdout

### 5. Driver Implementation — `pkg/drivers/cpp/<tool>/interface.go`

Each C/C++ driver implements the `Driver` interface by delegating to shared code in `gcc_common`:

| Callback | Delegates to |
|---|---|
| `ValidateArgs` | `gcc_common.ValidateAndParseArgs` with a `CLIConfig` for the specific compiler |
| `ParseCommandLine` | `gcc_common.ParseCommandLine` |
| `CreateJob` | `gcc_common.CreateCppJob` |
| `ApplyToolArgs` | `gcc_common.ExtractCompilerFlags` → `gcc_common.ModifyRuntimeWithFlags` (operates on `*v1.Runtime` protos) |
| `ValidateRuntime` | `gcc_common.ValidateRuntimeForC` / `ValidateRuntimeForCxx` / `ValidateRuntimeForClang` / `ValidateRuntimeForClangxx` |

## Package Responsibilities

| Package | Role |
|---|---|
| `pkg/driver` | Generic `Driver` interface |
| `pkg/drivers` | Generic CLI (`ExecuteDriver`), orchestrator (`RunDriver`), flag parsing, runtime resolution, job submission |
| `pkg/drivers/cpp/gcc_common` | Shared C/C++ logic: arg parsing, CLI validation, job creation, runtime validation, compiler flag extraction |
| `pkg/drivers/cpp/gcc` | GCC driver (`NewDriver()`) |
| `pkg/drivers/cpp/gxx` | G++ driver (`NewDriver()`) |
| `pkg/drivers/cpp/clang` | Clang driver (`NewDriver()`) |
| `pkg/drivers/cpp/clangxx` | Clang++ driver (`NewDriver()`) |
| `cmd/drivers/cpp/*` | One-line `main.go` entry points |

## Design Principles

- **The generic framework has zero language-specific knowledge.** All C/C++ types (`CompilerType`, `CLIConfig`, `ParsedArgs`, etc.) live in `gcc_common`, not in `pkg/driver` or `pkg/drivers`.
- **Drivers are callbacks.** The orchestrator owns the algorithm; drivers supply the language-specific steps.
- **Adding a new driver** (e.g. Rust, Go) requires: implementing `driver.Driver` in a new package, writing a one-line `main.go`, and adding a build target. No changes to the generic framework.
