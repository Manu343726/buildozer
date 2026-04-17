package clangxx

import (
	"context"
	"fmt"

	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	"github.com/Manu343726/buildozer/pkg/driver"
	"github.com/Manu343726/buildozer/pkg/drivers"
	gcc_common "github.com/Manu343726/buildozer/pkg/drivers/cpp/gcc_common"
	"github.com/Manu343726/buildozer/pkg/logging"
)

type clangxxDriver struct{}

// NewDriver returns a driver.Driver implementation for Clang++ (C++ compiler).
func NewDriver() driver.Driver { return clangxxDriver{} }

func init() {
	// Register this driver in the global registry
	drivers.RegisterDriver("clang++", NewDriver())
}

func (clangxxDriver) Name() string        { return "clang++" }
func (clangxxDriver) Version() string     { return "14.0.0" }
func (clangxxDriver) Short() string       { return "Clang++ C++ compiler" }
func (clangxxDriver) Long() string        { return "Clang++ - LLVM C++ compiler. Compatible with G++ flags." }
func (clangxxDriver) ErrorPrefix() string { return "clang++: error:" }

func (clangxxDriver) ValidateArgs(args []string) error {
	cfg := &gcc_common.CLIConfig{Name: "clang++", Type: gcc_common.ClangCxx, SupportsStdlib: true}
	_, err := gcc_common.ValidateAndParseArgs(args, cfg)
	return err
}

func (clangxxDriver) ParseCommandLine(args []string) interface{} {
	return gcc_common.ParseCommandLine(args)
}

func (clangxxDriver) CreateJob(ctx context.Context, parsed interface{}, workDir string, rtCtx *driver.RuntimeContext) (*v1.Job, error) {
	logger := logging.Log().Child("drivers").Child("clang++")

	parsedArgs := parsed.(*gcc_common.ParsedArgs)

	// Resolve runtime using the resolver with tool args that can affect runtime (e.g., -march, -std)
	toolArgs := append(parsedArgs.CompilerFlags, parsedArgs.LinkerFlags...)

	logger.Debug("Starting runtime resolution",
		"configPath", rtCtx.ConfigPath,
		"workDir", rtCtx.WorkDir,
		"toolArgCount", len(toolArgs),
	)
	if len(toolArgs) > 0 {
		logger.Debug("Tool arguments passed for runtime resolution", "args", toolArgs)
	}

	resolutionResultIface := rtCtx.Resolver.Resolve(ctx, rtCtx.ConfigPath, rtCtx.WorkDir, "", toolArgs, clangxxDriver{})
	resolutionResult, ok := resolutionResultIface.(*drivers.RuntimeResolutionResult)
	if !ok || resolutionResult == nil || resolutionResult.FoundRuntime == nil {
		// If resolution fails, return error
		if resolutionResult != nil && resolutionResult.Error != "" {
			logger.Error("Runtime resolution failed", "error", resolutionResult.Error)
			return nil, fmt.Errorf("failed to resolve runtime: %s", resolutionResult.Error)
		}
		logger.Error("Runtime resolution failed", "error", "no runtime found")
		return nil, fmt.Errorf("failed to resolve runtime: no runtime found")
	}

	cppSpec, ok := resolutionResult.FoundRuntime.ToolchainSpec.(*v1.Runtime_Cpp)
	cppCompiler := "unknown"
	cppVersion := "unknown"
	if ok && cppSpec.Cpp != nil {
		cppCompiler = cppSpec.Cpp.Compiler.String()
		if cppSpec.Cpp.CompilerVersion != nil {
			cppVersion = cppSpec.Cpp.CompilerVersion.String()
		}
	}
	logger.Debug("Runtime resolution completed successfully",
		"runtimeID", resolutionResult.FoundRuntime.Id,
		"platform", resolutionResult.FoundRuntime.Platform,
		"toolchain", resolutionResult.FoundRuntime.Toolchain,
		"compiler", cppCompiler,
		"compilerVersion", cppVersion,
	)

	// Now create the job with the resolved runtime
	return gcc_common.CreateCppJob(ctx, parsedArgs, resolutionResult.FoundRuntime, workDir)
}

func (clangxxDriver) ApplyToolArgs(_ context.Context, baseRuntime *v1.Runtime, toolArgs []string) (*v1.Runtime, error) {
	flags := gcc_common.ExtractCompilerFlags(toolArgs)
	return gcc_common.ModifyRuntimeWithFlags(baseRuntime, flags)
}

func (clangxxDriver) ValidateRuntime(runtime *v1.Runtime) (bool, string) {
	compat := gcc_common.ValidateRuntimeForClangxx(runtime)
	return compat.IsCompatible, compat.Reason
}

// ConstructRuntimeID constructs a runtime ID from Clang++-specific configuration.
// Converts the config map to a typed ClangxxConfig and delegates to gcc_common.
// Returns error if required config fields are missing or invalid.
func (clangxxDriver) ConstructRuntimeID(cfgMap map[string]interface{}) (string, error) {
	cfg, err := gcc_common.ConfigFromMap("clang++", cfgMap)
	if err != nil {
		return "", err
	}
	clangxxCfg := cfg.(gcc_common.ClangxxConfig)
	return gcc_common.ConstructRuntimeIDFromClangxxConfig(clangxxCfg), nil
}
