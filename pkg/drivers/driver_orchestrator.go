// Package drivers provides utilities for driver command execution and runtime resolution.
package drivers

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/Manu343726/buildozer/pkg/daemon"
	"github.com/Manu343726/buildozer/pkg/driver"
	"github.com/Manu343726/buildozer/pkg/logging"
)

// DriverConfig contains build context and configuration for driver execution.
// This is generic across all driver types.
type DriverConfig struct {
	// Standalone mode: start an in-process daemon if true
	Standalone bool
	// DaemonHost: hostname/IP of the daemon to connect to
	DaemonHost string
	// DaemonPort: port of the daemon to connect to
	DaemonPort int
	// LogLevel: logging level (debug, info, warn, error)
	LogLevel string
	// StartDir: starting directory for relative path resolution
	StartDir string
	// ConfigPath: optional path to .buildozer config file
	ConfigPath string
	// InitialRuntime: optional runtime ID to use directly
	InitialRuntime string
}

// RunDriver is the generic driver orchestration function.
// It handles the common algorithm steps for any driver type:
// - Starting standalone daemon if needed
// - Parsing command-line arguments
// - Resolving the runtime
// - Creating and submitting the job
// - Watching job progress
//
// This eliminates code duplication across different driver types (gcc, g++, clang, go, rust, etc.)
// by providing a single orchestration engine that all drivers use.
//
// Parameters:
//   - ctx: context for cancellation
//   - d: the driver implementation
//   - args: command-line arguments passed to the driver
//   - config: driver configuration (daemon address, log level, etc.)
//
// Returns the exit code (0 for success, non-zero for failure)
func RunDriver(ctx context.Context, d driver.Driver, args []string, config *DriverConfig) int {
	log := Log().Child(d.Name())
	log.Info(fmt.Sprintf("%s driver started", d.Name()), "numArgs", len(args), "standalone", config.Standalone)

	// If standalone mode, start an in-process daemon
	var dm *daemon.Daemon
	if config.Standalone {
		log.Debug("Standalone mode enabled, starting in-process daemon")

		// Create default daemon config
		daemonCfg := daemon.DefaultConfig()

		// Use the daemon host/port from config if they differ from defaults
		if config.DaemonHost != "localhost" {
			daemonCfg.Host = config.DaemonHost
		}
		if config.DaemonPort != 6789 {
			daemonCfg.Port = config.DaemonPort
		}

		var err error
		dm, err = daemon.NewDaemon(daemonCfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s failed to create in-process daemon: %v\n", d.ErrorPrefix(), err)
			return 1
		}

		// Start daemon in background
		if err := dm.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "%s failed to start in-process daemon: %v\n", d.ErrorPrefix(), err)
			return 1
		}

		log.Debug("Started in-process daemon", "host", daemonCfg.Host, "port", daemonCfg.Port)

		// Give daemon time to register handlers
		time.Sleep(100 * time.Millisecond)

		// Defer daemon cleanup
		defer func() {
			if err := dm.Stop(context.Background()); err != nil {
				log.Error("Error stopping in-process daemon", "error", err)
			}
		}()
	}

	// Parse command line arguments (driver-specific)
	parsed := d.ParseCommandLine(args)
	log.Debug("Parsed command line")

	// Set log level if specified
	if config.LogLevel != "" {
		level := logging.ParseLevel(config.LogLevel)
		logging.SetGlobalLevel(level)
		log.Debug("Log level set", "level", config.LogLevel)
	}

	// Handle --version flag
	if isVersionFlag(args) {
		fmt.Println(d.Version())
		return 0
	}

	// Determine working directory for config search
	workDir := config.StartDir
	if workDir == "" {
		workDir, _ = os.Getwd()
	}

	// Create the RuntimeResolver using daemon address from config
	resolver := NewRuntimeResolver(config.DaemonHost, config.DaemonPort)
	log.Debug("Created RuntimeResolver", "daemonHost", config.DaemonHost, "daemonPort", config.DaemonPort)

	// Resolve runtime using the generic framework
	configPath := config.ConfigPath
	if configPath == "" {
		// Let RuntimeResolver search for config
		configPath = workDir
	}

	resolutionResult := resolver.Resolve(ctx, configPath, workDir, config.InitialRuntime, args, d)
	log.Debug("Runtime resolution result",
		"hasError", resolutionResult.Error != "",
		"hasWarning", resolutionResult.Warning != "",
		"isNative", resolutionResult.IsNative,
		"foundRuntime", resolutionResult.FoundRuntime != nil)

	// Handle errors
	if resolutionResult.Error != "" {
		fmt.Fprintf(os.Stderr, "%s %s\n", d.ErrorPrefix(), resolutionResult.Error)
		return 1
	}

	// Handle warnings
	if resolutionResult.Warning != "" {
		fmt.Fprintf(os.Stderr, "%s: warning: %s\n", d.Name(), resolutionResult.Warning)
	}

	// Validate runtime using driver's ValidateRuntime method
	if resolutionResult.FoundRuntime != nil {
		isValid, reason := d.ValidateRuntime(resolutionResult.FoundRuntime)
		if !isValid {
			fmt.Fprintf(os.Stderr, "%s %s\n", d.ErrorPrefix(), reason)
			return 1
		}
	}

	// Runtime was found and validated
	if resolutionResult.FoundRuntime != nil {
		log.Info("Runtime resolved successfully",
			"runtimeID", resolutionResult.FoundRuntime.GetId(),
			"isNative", resolutionResult.IsNative)

		// Create job (driver-specific)
		job, err := d.CreateJob(ctx, parsed, resolutionResult.FoundRuntime, workDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s failed to create job: %v\n", d.ErrorPrefix(), err)
			return 1
		}

		if job == nil {
			fmt.Fprintf(os.Stderr, "%s failed to create job: job is nil\n", d.ErrorPrefix())
			return 1
		}

		// Submit job
		submitResp, err := SubmitJob(ctx, config.DaemonHost, config.DaemonPort, job)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s failed to submit job: %v\n", d.ErrorPrefix(), err)
			return 1
		}

		if !submitResp.Accepted {
			fmt.Fprintf(os.Stderr, "%s job rejected by daemon: %s\n", d.ErrorPrefix(), submitResp.ErrorMessage)
			return 1
		}

		log.Info("Job submitted, watching for progress", "jobID", job.Id)

		// Watch job progress and stream output to stdout
		result, exitCode, err := WatchAndStreamJobProgress(ctx, config.DaemonHost, config.DaemonPort, job.Id)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s failed to watch job progress: %v\n", d.ErrorPrefix(), err)
			return 1
		}

		if result != nil {
			log.Debug("Job completed with result", "jobID", job.Id, "exitCode", result.ExitCode)
			return int(result.ExitCode)
		}

		return exitCode
	}

	// No runtime found and no error was reported - unexpected state
	fmt.Fprintf(os.Stderr, "%s unable to resolve compiler runtime\n", d.ErrorPrefix())
	return 1
}

// ListCompatibleRuntimes lists compatible runtimes for the given driver.
// This is a generic function that works for any driver type.
//
// Parameters:
//   - ctx: context for cancellation
//   - d: the driver implementation (used for Name and ValidateRuntime)
//   - config: driver configuration with daemon address
//
// Returns the exit code (0 for success, non-zero for failure)
func ListCompatibleRuntimes(ctx context.Context, d driver.Driver, config *DriverConfig) int {
	log := Log().Child(d.Name())

	// Set log level if specified
	if config.LogLevel != "" {
		level := logging.ParseLevel(config.LogLevel)
		logging.SetGlobalLevel(level)
	}

	log.Info(fmt.Sprintf("%s list-runtimes mode started", d.Name()))

	// Create the RuntimeResolver using daemon address from config
	resolver := NewRuntimeResolver(config.DaemonHost, config.DaemonPort)
	log.Debug("Created RuntimeResolver", "daemonHost", config.DaemonHost, "daemonPort", config.DaemonPort)

	// Query daemon and filter compatible runtimes
	compatibleRuntimes, err := resolver.ListCompatibleRuntimes(ctx, d)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s failed to list runtimes: %v\n", d.ErrorPrefix(), err)
		return 1
	}

	if len(compatibleRuntimes) == 0 {
		fmt.Println("No compatible runtimes found")
		return 0
	}

	// Display compatible runtimes
	fmt.Printf("Compatible runtimes for %s:\n", d.Name())
	fmt.Println("")
	for _, rt := range compatibleRuntimes {
		fmt.Printf("  %s\n", rt.Id)
		if rt.Description != nil && *rt.Description != "" {
			fmt.Printf("    %s\n", *rt.Description)
		}
	}
	fmt.Println("")
	fmt.Printf("Total: %d runtimes available\n", len(compatibleRuntimes))
	log.Info("Listed compatible runtimes", "count", len(compatibleRuntimes))
	return 0
}

// isVersionFlag checks if --version is the first flag in args
func isVersionFlag(args []string) bool {
	if len(args) == 0 {
		return false
	}
	return args[0] == "--version"
}
