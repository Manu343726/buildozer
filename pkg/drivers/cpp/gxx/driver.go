package gxx

import (
	"context"
	"fmt"
	"os"

	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	"github.com/Manu343726/buildozer/pkg/drivers"
	gcc_common "github.com/Manu343726/buildozer/pkg/drivers/cpp/gcc_common"
	"github.com/Manu343726/buildozer/pkg/logging"
)

// BuildContext is an alias to the shared gcc_common BuildContext
type BuildContext = gcc_common.BuildContext

// ParsedArgs is an alias to the shared gcc_common ParsedArgs
type ParsedArgs = gcc_common.ParsedArgs

// CompileMode is an alias to the shared gcc_common CompileMode
type CompileMode = gcc_common.CompileMode

// ModeCompileOnly is an alias to the shared gcc_common ModeCompileOnly
const ModeCompileOnly = gcc_common.ModeCompileOnly

// RunGxx executes the G++ driver with the given arguments and build context.
// Returns exit code (0 for success, non-zero for failure).
func RunGxx(ctx context.Context, args []string, buildCtx *BuildContext) int {
	Log().InfoContext(ctx, "G++ driver started", "numArgs", len(args))

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
		fmt.Println("g++ version 11.2.0 (Buildozer distributed compiler)")
		return 0
	}

	// Check for input files
	if len(parsed.SourceFiles) == 0 && len(parsed.ObjectFiles) == 0 {
		fmt.Fprintf(os.Stderr, "g++: error: no input files specified\n")
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

	// Create the ToolArgsApplier callback for G++-specific flag handling
	gxxApplier := func(ctx context.Context, baseRuntime string, toolArgs []string) (string, error) {
		Log().DebugContext(ctx, "G++ ToolArgsApplier invoked", "baseRuntime", baseRuntime, "toolArgsCount", len(toolArgs))

		// Extract compiler flags from tool arguments
		flags := gcc_common.ExtractCompilerFlags(toolArgs)
		Log().DebugContext(ctx, "Extracted compiler flags",
			"version", flags.Version,
			"architecture", flags.Architecture,
			"cStandard", flags.CStandard,
			"cppStandard", flags.CppStandard,
			"stdLib", flags.StdLib,
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

	resolutionResult := resolver.Resolve(ctx, configPath, workDir, buildCtx.InitialRuntime, args, gxxApplier, "g++")
	Log().DebugContext(ctx, "Runtime resolution result",
		"hasError", resolutionResult.Error != "",
		"hasWarning", resolutionResult.Warning != "",
		"isNative", resolutionResult.IsNative,
		"foundRuntime", resolutionResult.FoundRuntime != nil)

	// Handle errors
	if resolutionResult.Error != "" {
		fmt.Fprintf(os.Stderr, "g++: error: %s\n", resolutionResult.Error)
		return 1
	}

	// Handle warnings
	if resolutionResult.Warning != "" {
		fmt.Fprintf(os.Stderr, "g++: warning: %s\n", resolutionResult.Warning)
	}

	// Runtime was found and validated
	if resolutionResult.FoundRuntime != nil {
		Log().InfoContext(ctx, "Runtime resolved successfully",
			"runtimeID", resolutionResult.FoundRuntime.GetId(),
			"isNative", resolutionResult.IsNative)
	}

	// TODO: Execute the build job using the resolved runtime
	Log().InfoContext(ctx, "Driver completed successfully (TODO: actual job execution not yet implemented)")
	return 0
}

// ListCompatibleRuntimes queries the daemon for available runtimes and displays
// only those compatible with G++ (i.e., those supporting C++ language).
func ListCompatibleRuntimes(ctx context.Context, buildCtx *BuildContext) int {
	// Set log level if specified
	if buildCtx.LogLevel != "" {
		level := logging.ParseLevel(buildCtx.LogLevel)
		logging.SetGlobalLevel(level)
	}

	Log().InfoContext(ctx, "G++ list-runtimes mode started")

	// Create the RuntimeResolver using daemon address from context
	resolver := drivers.NewRuntimeResolver(buildCtx.DaemonHost, buildCtx.DaemonPort)
	Log().DebugContext(ctx, "Created RuntimeResolver", "daemonHost", buildCtx.DaemonHost, "daemonPort", buildCtx.DaemonPort)

	// Create validator for C++ runtimes
	validator := func(runtime *v1.Runtime) (bool, string) {
		if runtime == nil {
			return false, "runtime is nil"
		}

		// Delegate to gcc_common validation
		compat := gcc_common.ValidateRuntimeForCxx(runtime)
		return compat.IsCompatible, compat.Reason
	}

	// Query daemon and filter compatible runtimes
	compatibleRuntimes, err := resolver.ListCompatibleRuntimes(ctx, validator, "g++")
	if err != nil {
		fmt.Fprintf(os.Stderr, "g++: error: failed to list runtimes: %v\n", err)
		return 1
	}

	if len(compatibleRuntimes) == 0 {
		fmt.Println("No compatible runtimes found")
		return 0
	}

	// Display compatible runtimes
	fmt.Println("Compatible C/C++ runtimes for G++:")
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
