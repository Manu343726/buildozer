package gcc

import (
	"context"
	"fmt"
	"os"
	"time"

	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	"github.com/Manu343726/buildozer/pkg/daemon"
	"github.com/Manu343726/buildozer/pkg/drivers"
	gcc_common "github.com/Manu343726/buildozer/pkg/drivers/cpp/gcc_common"
	"github.com/Manu343726/buildozer/pkg/logging"
)

// BuildContext is an alias to the shared gcc_common BuildContext
type BuildContext = gcc_common.BuildContext

// RunGcc executes the GCC driver with the given arguments and build context.
// Returns exit code (0 for success, non-zero for failure).
func RunGcc(ctx context.Context, args []string, buildCtx *BuildContext) int {
	Log().InfoContext(ctx, "GCC driver started", "numArgs", len(args), "standalone", buildCtx.Standalone)

	// If standalone mode, start an in-process daemon
	var d *daemon.Daemon
	if buildCtx.Standalone {
		// Create default daemon config
		daemonCfg := daemon.DefaultConfig()
		
		// Use the daemon host/port from buildCtx if they differ from defaults
		if buildCtx.DaemonHost != "localhost" {
			daemonCfg.Host = buildCtx.DaemonHost
		}
		if buildCtx.DaemonPort != 6789 {
			daemonCfg.Port = buildCtx.DaemonPort
		}

		var err error
		d, err = daemon.NewDaemon(daemonCfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "gcc: error: failed to create in-process daemon: %v\n", err)
			return 1
		}

		// Start daemon in background
		if err := d.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "gcc: error: failed to start in-process daemon: %v\n", err)
			return 1
		}

		Log().DebugContext(ctx, "Started in-process daemon", "host", daemonCfg.Host, "port", daemonCfg.Port)

		// Give daemon time to register handlers
		time.Sleep(100 * time.Millisecond)

		// Defer daemon cleanup
		defer func() {
			if err := d.Stop(context.Background()); err != nil {
				Log().ErrorContext(ctx, "Error stopping in-process daemon", "error", err)
			}
		}()
	}

	parsed := gcc_common.ParseCommandLine(args)
	Log().DebugContext(ctx, "Parsed command line",
		"sourceFiles", len(parsed.SourceFiles),
		"objectFiles", len(parsed.ObjectFiles),
		"outputFile", parsed.OutputFile,
		"mode", parsed.Mode)

	// Set log level if specified
	if buildCtx.LogLevel != "" {
		level := logging.ParseLevel(buildCtx.LogLevel)
		logging.SetGlobalLevel(level)
		Log().DebugContext(ctx, "Log level set", "level", buildCtx.LogLevel)
	}

	// Handle --version flag
	if len(parsed.CompilerFlags) > 0 && parsed.CompilerFlags[0] == "--version" {
		fmt.Println("gcc version 11.2.0 (Buildozer distributed compiler)")
		return 0
	}

	// Check for input files
	if len(parsed.SourceFiles) == 0 && len(parsed.ObjectFiles) == 0 {
		fmt.Fprintf(os.Stderr, "gcc: error: no input files specified\n")
		return 1
	}

	// Determine working directory for config search
	workDir := buildCtx.StartDir
	if workDir == "" {
		workDir, _ = os.Getwd()
	}

	// Create the RuntimeResolver using daemon address from context
	resolver := drivers.NewRuntimeResolver(buildCtx.DaemonHost, buildCtx.DaemonPort)
	Log().DebugContext(ctx, "Created RuntimeResolver", "daemonHost", buildCtx.DaemonHost, "daemonPort", buildCtx.DaemonPort)

	// Create the ToolArgsApplier callback for GCC-specific flag handling
	gccApplier := func(ctx context.Context, baseRuntime string, toolArgs []string) (string, error) {
		Log().DebugContext(ctx, "GCC ToolArgsApplier invoked", "baseRuntime", baseRuntime, "toolArgsCount", len(toolArgs))

		// Extract compiler flags from tool arguments
		flags := gcc_common.ExtractCompilerFlags(toolArgs)
		Log().DebugContext(ctx, "Extracted compiler flags",
			"version", flags.Version,
			"architecture", flags.Architecture,
			"cStandard", flags.CStandard,
			"cppStandard", flags.CppStandard,
			"optimization", flags.Optimization)

		// Modify the base runtime ID based on extracted flags
		modifiedRuntime, err := gcc_common.ModifyRuntimeIDWithFlags(baseRuntime, flags)
		if err != nil {
			Log().ErrorContext(ctx, "Failed to modify runtime ID", "error", err)
			return "", err
		}
		Log().DebugContext(ctx, "Modified runtime ID", "original", baseRuntime, "modified", modifiedRuntime)

		return modifiedRuntime, nil
	}

	// Resolve runtime using the generic framework
	configPath := buildCtx.ConfigPath
	if configPath == "" {
		// Let RuntimeResolver search for config
		configPath = workDir
	}

	resolutionResult := resolver.Resolve(ctx, configPath, workDir, buildCtx.InitialRuntime, args, gccApplier, "gcc")
	Log().DebugContext(ctx, "Runtime resolution result",
		"hasError", resolutionResult.Error != "",
		"hasWarning", resolutionResult.Warning != "",
		"isNative", resolutionResult.IsNative,
		"foundRuntime", resolutionResult.FoundRuntime != nil)

	// Handle errors
	if resolutionResult.Error != "" {
		fmt.Fprintf(os.Stderr, "gcc: error: %s\n", resolutionResult.Error)
		return 1
	}

	// Handle warnings
	if resolutionResult.Warning != "" {
		fmt.Fprintf(os.Stderr, "gcc: warning: %s\n", resolutionResult.Warning)
	}

	// Runtime was found and validated
	if resolutionResult.FoundRuntime != nil {
		Log().InfoContext(ctx, "Runtime resolved successfully",
			"runtimeID", resolutionResult.FoundRuntime.GetId(),
			"isNative", resolutionResult.IsNative)

		// Create job submission context
		jsc := &drivers.JobSubmissionContext{
			Runtime:       resolutionResult.FoundRuntime,
			SourceFiles:   parsed.SourceFiles,
			CompilerFlags: parsed.CompilerFlags,
			IncludeDirs:   parsed.IncludeDirs,
			Defines:       parsed.Defines,
			OutputFile:    parsed.OutputFile,
			IsLinkJob:     false, // GCC compile job
			Timeout:       5 * time.Minute,
			WorkDir:       workDir,
		}

		// Create and submit job
		job, err := jsc.CreateJob(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "gcc: error: failed to create job: %v\n", err)
			return 1
		}

		submitResp, err := drivers.SubmitJob(ctx, buildCtx.DaemonHost, buildCtx.DaemonPort, job)
		if err != nil {
			fmt.Fprintf(os.Stderr, "gcc: error: failed to submit job: %v\n", err)
			return 1
		}

		if !submitResp.Accepted {
			fmt.Fprintf(os.Stderr, "gcc: error: job rejected by daemon: %s\n", submitResp.ErrorMessage)
			return 1
		}

		Log().InfoContext(ctx, "Job submitted, watching for progress", "jobID", job.Id)

		// Watch job progress and stream output to stdout
		result, exitCode, err := drivers.WatchAndStreamJobProgress(ctx, buildCtx.DaemonHost, buildCtx.DaemonPort, job.Id)
		if err != nil {
			fmt.Fprintf(os.Stderr, "gcc: error: failed to watch job progress: %v\n", err)
			return 1
		}

		if result != nil {
			Log().DebugContext(ctx, "Job completed with result", "jobID", job.Id, "exitCode", result.ExitCode)
			return int(result.ExitCode)
		}

		return exitCode
	}

	// No runtime found and no error was reported - unexpected state
	fmt.Fprintf(os.Stderr, "gcc: error: unable to resolve compiler runtime\n")
	return 1
}

// ListCompatibleRuntimes queries the daemon for available runtimes and displays
// only those compatible with GCC (i.e., those supporting C language).
func ListCompatibleRuntimes(ctx context.Context, buildCtx *BuildContext) int {
	// Set log level if specified
	if buildCtx.LogLevel != "" {
		level := logging.ParseLevel(buildCtx.LogLevel)
		logging.SetGlobalLevel(level)
	}

	Log().InfoContext(ctx, "GCC list-runtimes mode started")

	// Create the RuntimeResolver using daemon address from context
	resolver := drivers.NewRuntimeResolver(buildCtx.DaemonHost, buildCtx.DaemonPort)
	Log().DebugContext(ctx, "Created RuntimeResolver", "daemonHost", buildCtx.DaemonHost, "daemonPort", buildCtx.DaemonPort)

	// Create validator for C runtimes
	validator := func(runtime *v1.Runtime) (bool, string) {
		if runtime == nil {
			return false, "runtime is nil"
		}

		// Delegate to gcc_common validation
		compat := gcc_common.ValidateRuntimeForC(runtime)
		return compat.IsCompatible, compat.Reason
	}

	// Query daemon and filter compatible runtimes
	compatibleRuntimes, err := resolver.ListCompatibleRuntimes(ctx, validator, "gcc")
	if err != nil {
		fmt.Fprintf(os.Stderr, "gcc: error: failed to list runtimes: %v\n", err)
		return 1
	}

	if len(compatibleRuntimes) == 0 {
		fmt.Println("No compatible runtimes found")
		return 0
	}

	// Display compatible runtimes
	fmt.Println("Compatible C/C++ runtimes for GCC:")
	fmt.Println("")
	for _, rt := range compatibleRuntimes {
		fmt.Printf("  %s\n", rt.Id)
		if rt.Description != nil && *rt.Description != "" {
			fmt.Printf("    %s\n", *rt.Description)
		}
	}
	fmt.Println("")
	fmt.Printf("Total: %d runtimes available\n", len(compatibleRuntimes))
	Log().InfoContext(ctx, "Listed compatible runtimes", "count", len(compatibleRuntimes))
	return 0
}
