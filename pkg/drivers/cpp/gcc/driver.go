package gcc

import (
	"context"
	"fmt"
	"os"

	"github.com/Manu343726/buildozer/pkg/config"
	"github.com/Manu343726/buildozer/pkg/drivers"
	gcc_common "github.com/Manu343726/buildozer/pkg/drivers/cpp/gcc_common"
)

// BuildContext is an alias to the shared gcc_common BuildContext
type BuildContext = gcc_common.BuildContext

// RunGcc executes the GCC driver with the given arguments and build context.
// Returns exit code (0 for success, non-zero for failure).
func RunGcc(ctx context.Context, args []string, buildCtx *BuildContext) int {
	Log().Info("GCC driver started", "args", args)

	parsed := gcc_common.ParseCommandLine(args)
	Log().Debug("Parsed command line",
		"sourceFiles", parsed.SourceFiles,
		"objectFiles", parsed.ObjectFiles,
		"outputFile", parsed.OutputFile,
		"compilerFlags", parsed.CompilerFlags,
		"mode", parsed.Mode)

	// Set log level if specified
	if buildCtx.LogLevel != "" {
		Log().Debug("Setting log level", "level", buildCtx.LogLevel)
	}

	// Handle --version flag
	if len(parsed.CompilerFlags) > 0 && parsed.CompilerFlags[0] == "--version" {
		Log().Debug("Handling --version flag")
		fmt.Println("gcc version 11.2.0 (Buildozer distributed compiler)")
		return 0
	}

	// Check for input files
	if len(parsed.SourceFiles) == 0 && len(parsed.ObjectFiles) == 0 {
		Log().Error("No input files specified")
		fmt.Fprintf(os.Stderr, "error: no input files specified\n")
		return 1
	}

	Log().Debug("Input validation passed",
		"sourceFiles", len(parsed.SourceFiles),
		"objectFiles", len(parsed.ObjectFiles))

	// Load configuration from .buildozer file or use provided config
	var cfg *config.Config
	if buildCtx.Config != nil {
		Log().Debug("Using provided config")
		cfg = buildCtx.Config
	} else if buildCtx.ConfigPath != "" {
		// Load from explicit config path
		Log().Debug("Loading config from explicit path", "path", buildCtx.ConfigPath)
		cfg, _, _ = config.LoadDriverConfig(buildCtx.ConfigPath)
	} else {
		// Search for config starting from current directory
		Log().Debug("Searching for config from start directory", "startDir", buildCtx.StartDir)
		cfg, _, _ = config.LoadDriverConfig(buildCtx.StartDir)
	}

	if cfg != nil {
		Log().Debug("Config loaded successfully")
	} else {
		Log().Warn("No config found, using defaults")
	}

	// Resolve the requested toolchain based on configuration
	Log().Debug("Resolving GCC toolchain", "numArgs", len(args))
	toolchainResolution := drivers.ResolveGccToolchain(ctx, &cfg.Drivers.Gcc, args)
	Log().Debug("Toolchain resolved", "resolution", toolchainResolution)

	// Query daemon for available runtimes if address provided
	if buildCtx.DaemonAddr != "" {
		Log().Debug("Querying daemon for runtimes", "address", buildCtx.DaemonAddr)
		daemonClient, err := drivers.NewDaemonClient(ctx, buildCtx.DaemonAddr)
		if err != nil {
			Log().Warn("Failed to connect to daemon", "error", err)
		} else {
			Log().Debug("Connected to daemon")
			runtimes, err := daemonClient.ListRuntimes(ctx, true)
			if err != nil {
				Log().Warn("Failed to list runtimes", "error", err)
			} else if len(runtimes) > 0 {
				Log().Debug("Runtimes available", "count", len(runtimes))
				// Try to find matching runtime
				matching, _ := daemonClient.FindMatchingRuntime(ctx, toolchainResolution, runtimes)
				if matching == nil {
					Log().Error("No matching runtime found")
					// No runtime found
					return 1
				}
				Log().Debug("Found matching runtime")
			} else {
				Log().Warn("No runtimes available from daemon")
			}
		}
	} else {
		Log().Debug("No daemon address provided, running in standalone mode")
	}

	// TODO: Execute the build job
	Log().Info("Driver completed successfully (TODO: actual execution not yet implemented)")
	return 0
}
