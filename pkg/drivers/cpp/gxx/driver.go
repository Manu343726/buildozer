package gxx

import (
	"context"

	"github.com/Manu343726/buildozer/internal/logger"
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
	// Create the ToolArgsApplier callback factory for G++-specific flag handling
	applierFactory := func(ctx context.Context) drivers.ToolArgsApplier {
		log := logger.NewComponentLogger("g++")
		return func(ctx context.Context, baseRuntime string, toolArgs []string) (string, error) {
			log.Debug("G++ ToolArgsApplier invoked", "baseRuntime", baseRuntime, "toolArgsCount", len(toolArgs))

			// Extract compiler flags from tool arguments
			flags := gcc_common.ExtractCompilerFlags(toolArgs)
			log.Debug("Extracted compiler flags",
				"version", flags.Version,
				"architecture", flags.Architecture,
				"cStandard", flags.CStandard,
				"cppStandard", flags.CppStandard,
				"stdLib", flags.StdLib,
				"optimization", flags.Optimization)

			// Modify the base runtime ID based on extracted flags
			modifiedRuntime, err := gcc_common.ModifyRuntimeIDWithFlags(baseRuntime, flags)
			if err != nil {
				log.Error("Failed to modify runtime ID", "error", err)
				return "", err
			}
			log.Debug("Modified runtime ID", "original", baseRuntime, "modified", modifiedRuntime)

			return modifiedRuntime, nil
		}
	}

	// Delegate to shared driver execution path (C++ language)
	return gcc_common.RunCppDriver(ctx, gcc_common.LanguageCxx, args, buildCtx, applierFactory)
}

// ListCompatibleRuntimes queries the daemon for available runtimes and displays
// only those compatible with G++ (i.e., those supporting C++ language).
func ListCompatibleRuntimes(ctx context.Context, buildCtx *BuildContext) int {
	// Delegate to shared implementation
	return gcc_common.ListCompatibleRuntimesShared(ctx, gcc_common.LanguageCxx, buildCtx)
}
