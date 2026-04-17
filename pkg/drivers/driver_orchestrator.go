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
	"github.com/Manu343726/buildozer/pkg/staging"
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
	// LoggingDir: directory for driver log files (default: ~/.cache/buildozer/logs)
	LoggingDir string
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
	// Configure driver logging early with custom config using driver name and config settings
	// Logger is always debug, stdout sink level is configurable by CLI flag, file sink is always debug
	loggingDir := config.LoggingDir
	if loggingDir == "" {
		loggingDir = "~/.cache/buildozer/logs"
	}
	loggingDir = logging.ExpandHome(loggingDir)

	driverLoggingConfig := logging.LoggingConfig{
		GlobalLevel: "debug",
		LoggingDir:  loggingDir,
		Sinks: []logging.SinkConfig{
			{
				Name:  "stdout",
				Type:  "stdout",
				Level: config.LogLevel, // Use CLI flag (defaults to "warn")
			},
			{
				Name:       "file-" + d.Name(),
				Type:       "file",
				Level:      "debug",
				Filename:   d.Name() + ".log",
				MaxSizeB:   50 * 1024 * 1024, // 50MB
				MaxFiles:   5,
				MaxAgeDays: 30,
			},
		},
		Loggers: []logging.LoggerConfig{
			{
				Name:  "buildozer",
				Level: "debug", // Logger always debug
				Sinks: []string{"stdout", "file-" + d.Name()},
			},
		},
	}
	if err := logging.InitializeGlobal(driverLoggingConfig); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logging: %v\n", err)
		return 1
	}

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
	log.Debug("Original command line arguments", "args", args)
	parsed := d.ParseCommandLine(args)

	// Note: Logging is configured in RunDriver() at the start
	// with custom config: logger at debug, stdout sink configurable, file sink at debug

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

	// Create the RuntimeResolver for drivers to use during CreateJob
	resolver := NewRuntimeResolver(config.DaemonHost, config.DaemonPort)
	log.Debug("Created RuntimeResolver", "daemonHost", config.DaemonHost, "daemonPort", config.DaemonPort)

	// Create RuntimeContext with resolver for driver-side resolution
	rtCtx := &driver.RuntimeContext{
		Resolver:   resolver,
		DaemonHost: config.DaemonHost,
		DaemonPort: config.DaemonPort,
		ConfigPath: config.ConfigPath,
		WorkDir:    workDir,
	}

	// Create job (driver-specific) - driver is responsible for runtime resolution inside CreateJob
	job, err := d.CreateJob(ctx, parsed, workDir, rtCtx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s failed to create job: %v\n", d.ErrorPrefix(), err)
		return 1
	}

	if job == nil {
		fmt.Fprintf(os.Stderr, "%s failed to create job: job is nil\n", d.ErrorPrefix())
		return 1
	}

	// Verify job inputs are accessible on disk before submitting
	log.Debug("Verifying job inputs on disk", "jobID", job.Id, "inputCount", len(job.Inputs))
	stager := staging.NewJobDataStager(job.Cwd)
	if err := stager.VerifyJobInputs(ctx, job, staging.VerificationModeSaved); err != nil {
		fmt.Fprintf(os.Stderr, "%s input verification failed: %v\n", d.ErrorPrefix(), err)
		return 1
	}
	log.Debug("Job inputs verified", "jobID", job.Id)

	// Enrich context with driver information for requester identification
	driverCtx := context.WithValue(ctx, ContextKeyDriverName, d.Name())
	driverCtx = context.WithValue(driverCtx, ContextKeyDriverVersion, d.Version())

	log.Debug("About to submit job to daemon", "jobID", job.Id, "daemonHost", config.DaemonHost, "daemonPort", config.DaemonPort)

	// Submit job with driver identification
	confirmation, stream, err := SubmitJob(driverCtx, config.DaemonHost, config.DaemonPort, job)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s failed to submit job: %v\n", d.ErrorPrefix(), err)
		return 1
	}

	if !confirmation.Accepted {
		fmt.Fprintf(os.Stderr, "%s job rejected by daemon: %s\n", d.ErrorPrefix(), confirmation.ErrorMessage)
		return 1
	}

	Log().Info("Job submitted, watching for progress", "jobID", job.Id)

	// Stream job progress from the submission response stream and stream output to stdout (with driver context)
	result, exitCode, err := WatchAndStreamJobProgressFromStream(driverCtx, stream, config.DaemonHost, config.DaemonPort, job.Id, workDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s failed to watch job progress: %v\n", d.ErrorPrefix(), err)
		return 1
	}

	// Use the exit code returned from WatchAndStreamJobProgressWithOutputDir
	// which is set based on the final job status (COMPLETED=0, FAILED=1, CANCELLED=1)
	// The result object contains additional details but we trust the status-based exit code
	if result != nil {
		log.Debug("Job completed with result", "jobID", job.Id, "status", result.Status, "exitCode", exitCode)
	}

	return exitCode
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
