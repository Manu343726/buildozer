package gxx

import (
	"context"
	"fmt"

	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	"github.com/Manu343726/buildozer/pkg/driver"
	"github.com/Manu343726/buildozer/pkg/drivers"
	gcc_common "github.com/Manu343726/buildozer/pkg/drivers/cpp/gcc_common"
	"github.com/Manu343726/buildozer/pkg/logging"
)

type gxxDriver struct{}

// NewDriver returns a driver.Driver implementation for G++ (C++ compiler).
func NewDriver() driver.Driver { return gxxDriver{} }

func init() {
	// Register this driver in the global registry
	drivers.RegisterDriver("g++", NewDriver())
}

func (gxxDriver) Name() string        { return "g++" }
func (gxxDriver) Version() string     { return "10.2.1" }
func (gxxDriver) Short() string       { return "G++ C++ compiler" }
func (gxxDriver) Long() string        { return "G++ - the GNU Compiler Collection for C++." }
func (gxxDriver) ErrorPrefix() string { return "g++: error:" }

func (gxxDriver) ValidateArgs(args []string) error {
	cfg := &gcc_common.CLIConfig{Name: "g++", Type: gcc_common.GXX, SupportsStdlib: true}
	_, err := gcc_common.ValidateAndParseArgs(args, cfg)
	return err
}

func (gxxDriver) ParseCommandLine(args []string) interface{} {
	return gcc_common.ParseCommandLine(args)
}

func (gxxDriver) CreateJob(ctx context.Context, parsed interface{}, workDir string, rtCtx *driver.RuntimeContext) (*v1.Job, error) {
	logger := logging.Log().Child("drivers").Child("gxx")

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

	resolutionResultIface := rtCtx.Resolver.Resolve(ctx, rtCtx.ConfigPath, rtCtx.WorkDir, "", toolArgs, gxxDriver{})
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

func (gxxDriver) ApplyToolArgs(_ context.Context, baseRuntime *v1.Runtime, toolArgs []string) (*v1.Runtime, error) {
	flags := gcc_common.ExtractCompilerFlags(toolArgs)
	return gcc_common.ModifyRuntimeWithFlags(baseRuntime, flags)
}

func (gxxDriver) ValidateRuntime(runtime *v1.Runtime) (bool, string) {
	compat := gcc_common.ValidateRuntimeForCxx(runtime)
	return compat.IsCompatible, compat.Reason
}

// ConstructRuntimeID constructs a runtime ID from G++-specific configuration.
// Converts the config map to a typed GxxConfig and delegates to gcc_common.
// Returns error if required config fields are missing or invalid.
func (gxxDriver) ConstructRuntimeID(cfgMap map[string]interface{}) (string, error) {
	cfg, err := gcc_common.ConfigFromMap("g++", cfgMap)
	if err != nil {
		return "", err
	}
	gxxCfg := cfg.(gcc_common.GxxConfig)
	return gcc_common.ConstructRuntimeIDFromGxxConfig(gxxCfg), nil
}
