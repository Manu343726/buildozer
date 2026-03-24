package gcc_common

import (
	"context"
	"fmt"
	"os"
	"time"

	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	"github.com/Manu343726/buildozer/internal/logger"
	"github.com/Manu343726/buildozer/pkg/daemon"
	"github.com/Manu343726/buildozer/pkg/drivers"
	"github.com/Manu343726/buildozer/pkg/logging"
)

// LanguageType represents the language (C or C++) for the compilation driver
type LanguageType int

const (
	LanguageC LanguageType = iota
	LanguageCxx
)

// String returns the string representation of the language type
func (lt LanguageType) String() string {
	switch lt {
	case LanguageC:
		return "C"
	case LanguageCxx:
		return "C++"
	default:
		return "unknown"
	}
}

// ToolName returns the tool name for the language (gcc or g++)
func (lt LanguageType) ToolName() string {
	switch lt {
	case LanguageC:
		return "gcc"
	case LanguageCxx:
		return "g++"
	default:
		return "unknown"
	}
}

// VersionString returns the version string for error messages
func (lt LanguageType) VersionString() string {
	switch lt {
	case LanguageC:
		return "gcc version 11.2.0 (Buildozer distributed compiler)"
	case LanguageCxx:
		return "g++ version 11.2.0 (Buildozer distributed compiler)"
	default:
		return "unknown version"
	}
}

// ErrorPrefix returns the error prefix for the language
func (lt LanguageType) ErrorPrefix() string {
	return lt.ToolName() + ": error:"
}

// RuntimeValidator validates if a runtime is compatible with a language
type RuntimeValidator func(runtime *v1.Runtime) RuntimeCompatibility

// GetRuntimeValidator returns the appropriate validator for the language
func (lt LanguageType) GetValidator() RuntimeValidator {
	switch lt {
	case LanguageC:
		return ValidateRuntimeForC
	case LanguageCxx:
		return ValidateRuntimeForCxx
	default:
		return nil
	}
}

// RunCppDriver is the shared execution path for GCC/G++ drivers.
// It handles all common logic: parsing, resolving, submitting, and watching job execution.
//
// This function eliminates code duplication between gcc and gxx drivers by:
// - Taking language-specific callbacks as parameters
// - Handling standalone daemon startup if needed
// - Managing runtime resolution, job submission, and progress watching
//
// langType specifies whether this is C or C++ compilation
// args are the command-line arguments passed to the driver
// buildCtx contains the build context (daemon address, log level, etc.)
// applierFactory creates the ToolArgsApplier callback for language-specific handling
//
// Returns the exit code (0 for success, non-zero for failure)
func RunCppDriver(ctx context.Context, langType LanguageType, args []string,
	buildCtx *BuildContext, applierFactory func(context.Context) drivers.ToolArgsApplier) int {

	log := logger.NewComponentLogger(langType.ToolName())
	log.Info(fmt.Sprintf("%s driver started", langType.String()), "numArgs", len(args), "standalone", buildCtx.Standalone)

	// If standalone mode, start an in-process daemon
	var d *daemon.Daemon
	if buildCtx.Standalone {
		log.Debug("Standalone mode enabled, starting in-process daemon")

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
			fmt.Fprintf(os.Stderr, "%s failed to create in-process daemon: %v\n", langType.ErrorPrefix(), err)
			return 1
		}

		// Start daemon in background
		if err := d.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "%s failed to start in-process daemon: %v\n", langType.ErrorPrefix(), err)
			return 1
		}

		log.Debug("Started in-process daemon", "host", daemonCfg.Host, "port", daemonCfg.Port)

		// Give daemon time to register handlers
		time.Sleep(100 * time.Millisecond)

		// Defer daemon cleanup
		defer func() {
			if err := d.Stop(context.Background()); err != nil {
				log.Error("Error stopping in-process daemon", "error", err)
			}
		}()
	}

	// Parse command line arguments
	parsed := ParseCommandLine(args)
	log.Debug("Parsed command line",
		"sourceFiles", len(parsed.SourceFiles),
		"objectFiles", len(parsed.ObjectFiles),
		"outputFile", parsed.OutputFile,
		"mode", parsed.Mode)

	// Set log level if specified
	if buildCtx.LogLevel != "" {
		level := logging.ParseLevel(buildCtx.LogLevel)
		logging.SetGlobalLevel(level)
		log.Debug("Log level set", "level", buildCtx.LogLevel)
	}

	// Handle --version flag
	if len(parsed.CompilerFlags) > 0 && parsed.CompilerFlags[0] == "--version" {
		fmt.Println(langType.VersionString())
		return 0
	}

	// Check for input files
	if len(parsed.SourceFiles) == 0 && len(parsed.ObjectFiles) == 0 {
		fmt.Fprintf(os.Stderr, "%s no input files specified\n", langType.ErrorPrefix())
		return 1
	}

	// Determine working directory for config search
	workDir := buildCtx.StartDir
	if workDir == "" {
		workDir, _ = os.Getwd()
	}

	// Create the RuntimeResolver using daemon address from context
	resolver := drivers.NewRuntimeResolver(buildCtx.DaemonHost, buildCtx.DaemonPort)
	log.Debug("Created RuntimeResolver", "daemonHost", buildCtx.DaemonHost, "daemonPort", buildCtx.DaemonPort)

	// Create the ToolArgsApplier callback (language-specific handling)
	applier := applierFactory(ctx)

	// Resolve runtime using the generic framework
	configPath := buildCtx.ConfigPath
	if configPath == "" {
		// Let RuntimeResolver search for config
		configPath = workDir
	}

	resolutionResult := resolver.Resolve(ctx, configPath, workDir, buildCtx.InitialRuntime, args, applier, langType.ToolName())
	log.Debug("Runtime resolution result",
		"hasError", resolutionResult.Error != "",
		"hasWarning", resolutionResult.Warning != "",
		"isNative", resolutionResult.IsNative,
		"foundRuntime", resolutionResult.FoundRuntime != nil)

	// Handle errors
	if resolutionResult.Error != "" {
		fmt.Fprintf(os.Stderr, "%s %s\n", langType.ErrorPrefix(), resolutionResult.Error)
		return 1
	}

	// Handle warnings
	if resolutionResult.Warning != "" {
		fmt.Fprintf(os.Stderr, "%s: warning: %s\n", langType.ToolName(), resolutionResult.Warning)
	}

	// Runtime was found and validated
	if resolutionResult.FoundRuntime != nil {
		log.Info(fmt.Sprintf("Runtime resolved successfully"),
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
			IsLinkJob:     false, // Compile job
			Timeout:       5 * time.Minute,
			WorkDir:       workDir,
		}

		// Create and submit job
		job, err := jsc.CreateJob(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s failed to create job: %v\n", langType.ErrorPrefix(), err)
			return 1
		}

		submitResp, err := drivers.SubmitJob(ctx, buildCtx.DaemonHost, buildCtx.DaemonPort, job)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s failed to submit job: %v\n", langType.ErrorPrefix(), err)
			return 1
		}

		if !submitResp.Accepted {
			fmt.Fprintf(os.Stderr, "%s job rejected by daemon: %s\n", langType.ErrorPrefix(), submitResp.ErrorMessage)
			return 1
		}

		log.Info("Job submitted, watching for progress", "jobID", job.Id)

		// Watch job progress and stream output to stdout
		result, exitCode, err := drivers.WatchAndStreamJobProgress(ctx, buildCtx.DaemonHost, buildCtx.DaemonPort, job.Id)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s failed to watch job progress: %v\n", langType.ErrorPrefix(), err)
			return 1
		}

		if result != nil {
			log.Debug("Job completed with result", "jobID", job.Id, "exitCode", result.ExitCode)
			return int(result.ExitCode)
		}

		return exitCode
	}

	// No runtime found and no error was reported - unexpected state
	fmt.Fprintf(os.Stderr, "%s unable to resolve compiler runtime\n", langType.ErrorPrefix())
	return 1
}

// ListCompatibleRuntimesShared is the shared implementation for listing compatible runtimes.
// It handles the common logic for both gcc and g++.
func ListCompatibleRuntimesShared(ctx context.Context, langType LanguageType, buildCtx *BuildContext) int {
	log := logger.NewComponentLogger(langType.ToolName())

	// Set log level if specified
	if buildCtx.LogLevel != "" {
		level := logging.ParseLevel(buildCtx.LogLevel)
		logging.SetGlobalLevel(level)
	}

	log.Info(fmt.Sprintf("%s list-runtimes mode started", langType.String()))

	// Create the RuntimeResolver using daemon address from context
	resolver := drivers.NewRuntimeResolver(buildCtx.DaemonHost, buildCtx.DaemonPort)
	log.Debug("Created RuntimeResolver", "daemonHost", buildCtx.DaemonHost, "daemonPort", buildCtx.DaemonPort)

	// Create validator for the appropriate language
	validator := func(runtime *v1.Runtime) (bool, string) {
		if runtime == nil {
			return false, "runtime is nil"
		}

		// Use language-specific validator
		compat := langType.GetValidator()(runtime)
		return compat.IsCompatible, compat.Reason
	}

	// Query daemon and filter compatible runtimes
	compatibleRuntimes, err := resolver.ListCompatibleRuntimes(ctx, validator, langType.ToolName())
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s failed to list runtimes: %v\n", langType.ErrorPrefix(), err)
		return 1
	}

	if len(compatibleRuntimes) == 0 {
		fmt.Println("No compatible runtimes found")
		return 0
	}

	// Display compatible runtimes
	fmt.Printf("Compatible C/C++ runtimes for %s:\n", langType.ToolName())
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
