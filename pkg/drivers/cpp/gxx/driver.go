package gxx

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/Manu343726/buildozer/pkg/drivers"
	gcc_common "github.com/Manu343726/buildozer/pkg/drivers/cpp/gcc_common"
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
		Log().DebugContext(ctx, "Log level specified", "level", buildCtx.LogLevel)
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

	// Extract daemon address, default to localhost:6789 if not specified
	daemonHost := "localhost"
	daemonPort := 6789

	if strings.Contains(buildCtx.DaemonAddr, ":") {
		// Address already includes port
		parts := strings.Split(buildCtx.DaemonAddr, ":")
		if len(parts) == 2 {
			daemonHost = parts[0]
			// Parse port
			fmt.Sscanf(parts[1], "%d", &daemonPort)
		}
	} else if buildCtx.DaemonAddr != "" {
		daemonHost = buildCtx.DaemonAddr
	}

	// Determine working directory for config search
	workDir := buildCtx.StartDir
	if workDir == "" {
		workDir, _ = os.Getwd()
	}

	// Create the RuntimeResolver
	resolver := drivers.NewRuntimeResolver(daemonHost, daemonPort)
	Log().DebugContext(ctx, "Created RuntimeResolver", "daemonHost", daemonHost, "daemonPort", daemonPort)

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
		modifiedRuntime := gcc_common.ModifyRuntimeIDWithFlags(baseRuntime, flags)
		Log().DebugContext(ctx, "Modified runtime ID", "original", baseRuntime, "modified", modifiedRuntime)

		return modifiedRuntime, nil
	}

	// Resolve runtime using the generic framework
	configPath := buildCtx.ConfigPath
	if configPath == "" {
		// Let RuntimeResolver search for config
		configPath = workDir
	}

	resolutionResult := resolver.Resolve(ctx, configPath, workDir, args, gxxApplier, "g++")
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
