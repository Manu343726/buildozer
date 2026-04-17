package ar

import (
	"context"
	"fmt"

	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	"github.com/Manu343726/buildozer/pkg/config"
	"github.com/Manu343726/buildozer/pkg/driver"
	"github.com/Manu343726/buildozer/pkg/drivers"
	"github.com/Manu343726/buildozer/pkg/drivers/cpp/ar_common"
	"github.com/Manu343726/buildozer/pkg/logging"
)

type arDriver struct{}

// NewDriver returns a driver.Driver implementation for AR (static library archiver).
func NewDriver() driver.Driver { return arDriver{} }

func init() {
	// Register this driver in the global registry
	drivers.RegisterDriver("ar", NewDriver())
}

func (arDriver) Name() string    { return "ar" }
func (arDriver) Version() string { return "2.37" }
func (arDriver) Short() string   { return "GNU AR - static library archiver" }
func (arDriver) Long() string {
	return "GNU AR - the GNU static library archiver for creating .a archive files."
}
func (arDriver) ErrorPrefix() string { return "ar: error:" }

func (arDriver) ValidateArgs(args []string) error {
	_, err := ar_common.ValidateAndParseArgs(args)
	return err
}

func (arDriver) ParseCommandLine(args []string) interface{} {
	return ar_common.ParseCommandLine(args)
}

// CreateJob creates a Job proto with RuntimeMatchQuery.
// AR loads driver configuration directly (not resolving a runtime)
// and creates a RuntimeMatchQuery targeting any runtime with compatible ABI
// (c_runtime, c_runtime_version, architecture).
// The configuration is loaded from .buildozer file or defaults.
func (arDriver) CreateJob(ctx context.Context, parsed interface{}, workDir string, rtCtx *driver.RuntimeContext) (*v1.Job, error) {
	logger := logging.Log().Child("drivers").Child("ar")

	parsedArgs := parsed.(*ar_common.ParsedArgs)

	logger.Debug("Parsed AR command line arguments",
		"mode", ar_common.ModeString(parsedArgs.Mode),
		"inputFileCount", len(parsedArgs.InputFiles),
		"outputFile", parsedArgs.OutputFile,
		"flagCount", len(parsedArgs.Flags),
	)

	if len(parsedArgs.InputFiles) > 0 {
		logger.Debug("AR input files", "files", parsedArgs.InputFiles)
	}
	if len(parsedArgs.Flags) > 0 {
		logger.Debug("AR tool arguments (ar flags)", "flags", parsedArgs.Flags)
	}

	// Load the driver configuration from .buildozer file or defaults
	cfg, _, err := config.LoadDriverConfig(rtCtx.WorkDir)
	if err != nil {
		logger.Error("Failed to load driver config", "error", err)
		return nil, fmt.Errorf("failed to load driver config: %w", err)
	}

	// Extract AR driver config from the loaded configuration
	arConfigMap, ok := cfg.Drivers["ar"]
	if !ok {
		logger.Error("AR driver configuration not found")
		return nil, fmt.Errorf("ar driver configuration not found in config file")
	}

	// Parse the AR config map into ArConfig
	arCfg, err := ar_common.ConfigFromMap(arConfigMap)
	if err != nil {
		logger.Error("Failed to parse AR config", "error", err)
		return nil, fmt.Errorf("failed to parse ar config: %w", err)
	}

	logger.Debug("Using AR driver configuration",
		"cRuntime", arCfg.CRuntime,
		"cRuntimeVersion", arCfg.CRuntimeVersion,
		"architecture", arCfg.Architecture,
	)

	// Create the archive job with RuntimeMatchQuery based on config
	return ar_common.CreateArchiveJobWithRuntimeABIFromConfig(ctx, parsedArgs, arCfg, workDir)
}

func (arDriver) ApplyToolArgs(_ context.Context, baseRuntime *v1.Runtime, toolArgs []string) (*v1.Runtime, error) {
	// AR doesn't have runtime-modifying flags like compilers do (-march, -std, etc.)
	// Return the base runtime unchanged
	return baseRuntime, nil
}

func (arDriver) ValidateRuntime(runtime *v1.Runtime) (bool, string) {
	return ar_common.ValidateRuntimeForArchive(runtime)
}

// ConstructRuntimeID is not used by AR driver.
// AR uses RuntimeMatchQuery in CppArchiveJob for flexible runtime selection instead.
// If this function is called, it indicates a bug in the driver orchestration code.
func (arDriver) ConstructRuntimeID(cfgMap map[string]interface{}) (string, error) {
	return "", fmt.Errorf("ar driver does not use ConstructRuntimeID: it uses RuntimeMatchQuery for flexible runtime matching")
}
