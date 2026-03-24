// Package drivers provides utilities for driver command execution and runtime resolution.
package drivers

import (
	"context"
	"fmt"
	"os"

	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	"github.com/spf13/cobra"
)

// Context keys for generic driver configuration
// These are stored in context.Context so drivers can access generic config without it being in method signatures
type contextKey string

const (
	CommonConfigContextKey contextKey = "drivers.commonConfig"
)

// GetCommonConfig retrieves the common driver configuration from context.
// This is used by driver implementations to access generic config (daemon address, log level, etc.)
// without having it passed through every function signature.
func GetCommonConfig(ctx context.Context) *CommonDriverConfig {
	cfg, ok := ctx.Value(CommonConfigContextKey).(*CommonDriverConfig)
	if !ok {
		return nil
	}
	return cfg
}

// BuildContext is a generic build context that works for all driver types.
// It contains configuration common to all drivers extracted from command-line flags.
type BuildContext struct {
	DaemonHost     string
	DaemonPort     int
	Standalone     bool
	LogLevel       string
	ConfigPath     string
	InitialRuntime string
}

// Driver defines the interface for language-specific driver implementations.
// Each driver (gcc, g++, clang, go, rust, etc.) implements this interface
// to integrate with the generic driver CLI framework.
// Drivers provide metadata and callbacks; the generic CLI handles the common algorithm.
type Driver interface {
	// Name returns the tool name (e.g., "gcc", "g++", "clang", "rustc")
	Name() string

	// Version returns the version string displayed with --version flag
	Version() string

	// Long returns the long description of the tool
	Long() string

	// Short returns the short description of the tool
	Short() string

	// ErrorPrefix returns the error prefix for messages (e.g., "gcc: error:")
	ErrorPrefix() string

	// LanguageType returns the language type for CLI config validation.
	// For C/C++ drivers, return GCC/GXX/Clang/ClangCxx.
	LanguageType() CompilerType

	// SupportsStdlib returns whether this driver supports C++ standard library selection.
	// GCC/Clang C drivers return false. G++/Clang++ return true.
	SupportsStdlib() bool

	// ToolArgsApplier returns the tool-specific args applier callback.
	// This handles language-specific flag extraction and runtime modification.
	ToolArgsApplier(ctx context.Context) ToolArgsApplier

	// RunDriver executes the driver with the given arguments.
	// The driver is responsible for creating its own language-specific BuildContext.
	// Args contains the parsed tool-specific arguments (after buildozer flags removed).
	RunDriver(ctx context.Context, args []string) int

	// ListCompatibleRuntimes lists available runtimes compatible with this driver.
	// The driver is responsible for creating its own language-specific BuildContext.
	ListCompatibleRuntimes(ctx context.Context) int
}

// CommonDriverConfig contains configuration common to all drivers.
// This is extracted from parsed flags and should be used to populate
// the driver-specific BuildContext in each driver's main.go.
type CommonDriverConfig struct {
	DaemonHost     string
	DaemonPort     int
	Standalone     bool
	LogLevel       string
	ConfigPath     string
	InitialRuntime string
}

// ExtractCommonConfig extracts common driver configuration from already-parsed
// buildozer flags. This must be called AFTER StandardDriverFlags.Parse(args).
// Drivers should use this to avoid duplicating daemon configuration extraction logic.
//
// Example:
//
//	drivers.StandardDriverFlags.Parse(os.Args[1:])
//	commonCfg := drivers.ExtractCommonConfig()
//	buildCtx := &MyDriverContext{
//	    DaemonHost:     commonCfg.DaemonHost,
//	    DaemonPort:     commonCfg.DaemonPort,
//	    // ... other fields
//	}
func ExtractCommonConfig() *CommonDriverConfig {
	daemonHost := "localhost"
	daemonPort := 6789

	if DaemonHostPtr != nil && *DaemonHostPtr != nil {
		daemonHost = **DaemonHostPtr
	}
	if DaemonPortPtr != nil && *DaemonPortPtr != nil {
		daemonPort = **DaemonPortPtr
	}

	return &CommonDriverConfig{
		DaemonHost:     daemonHost,
		DaemonPort:     daemonPort,
		Standalone:     *StandalonePtr,
		LogLevel:       *LogLevelPtr,
		ConfigPath:     *ConfigPathPtr,
		InitialRuntime: *RuntimePtr,
	}
}

// This is the generic CLI handler that all drivers use to avoid boilerplate.
//
// Parameters:
//   - driver: Implementation of the Driver interface with language-specific callbacks
//
// The function:
//  1. Creates a cobra root command with driver metadata
//  2. Parses buildozer-specific and tool-specific flags
//  3. Validates arguments
//  4. Constructs the driver's BuildContext
//  5. Delegates to RunDriver or ListCompatibleRuntimes based on flags
func ExecuteDriver(driver Driver) {
	rootCmd := &cobra.Command{
		Use:     fmt.Sprintf("%s [options] [files...]", driver.Name()),
		Short:   driver.Short(),
		Long:    driver.Long(),
		Version: driver.Version(),
		Args:    cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDriver(cmd, args, driver)
		},
		DisableFlagParsing: true, // Allow all driver flags without cobra validation
	}

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// runDriver is the cobra RunE handler that delegates to the driver implementation.
func runDriver(cmd *cobra.Command, args []string, driver Driver) error {
	// Handle --help and -h flags before parsing driver flags
	for _, arg := range args {
		if arg == "--help" || arg == "-h" || arg == "-help" {
			cmd.Help()
			return nil
		}
	}

	// Parse buildozer driver flags and extract tool-specific flags
	parsedArgs := StandardDriverFlags.Parse(args)

	// Extract generic driver configuration
	commonCfg := ExtractCommonConfig()

	// Create CLI config from driver metadata
	cliCfg := &CLIConfig{
		Name:           driver.Name(),
		Type:           driver.LanguageType(),
		SupportsStdlib: driver.SupportsStdlib(),
	}

	// Validate arguments using CLI validator
	_, err := ValidateAndParseArgs(parsedArgs, cliCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s %v\n", driver.ErrorPrefix(), err)
		os.Exit(1)
	}

	// Handle --version flag
	if isVersionFlag(parsedArgs) {
		fmt.Println(driver.Version())
		return nil
	}

	// Handle --buildozer-list-runtimes flag
	if *ListRuntimesPtr {
		exitCode := driver.ListCompatibleRuntimes(cmd.Context())
		os.Exit(exitCode)
		return nil
	}

	// For SimpleDriver, extract callbacks and wrap in DriverTool
	var simpleDriver *SimpleDriver
	if sd, ok := driver.(*SimpleDriver); ok {
		simpleDriver = sd
	}

	if simpleDriver == nil {
		fmt.Fprintf(os.Stderr, "%s: only SimpleDriver is supported\n", driver.ErrorPrefix())
		return fmt.Errorf("only SimpleDriver is supported")
	}

	// Create the DriverTool with driver-specific callbacks
	tool := &DriverTool{
		Name:             simpleDriver.DriverName,
		VersionString:    simpleDriver.DriverVersion,
		ErrorPrefix:      simpleDriver.DriverPrefix,
		ParseCommandLine: simpleDriver.DriverParseCommandLine,
		CreateJob:        simpleDriver.DriverCreateJob,
		ApplierFactory:   simpleDriver.DriverToolArgsApplier,
		RuntimeValidator: simpleDriver.DriverRuntimeValidator,
	}

	// Call the generic driver orchestrator
	driverConfig := &DriverConfig{
		Standalone:     commonCfg.Standalone,
		DaemonHost:     commonCfg.DaemonHost,
		DaemonPort:     commonCfg.DaemonPort,
		LogLevel:       commonCfg.LogLevel,
		StartDir:       "",
		ConfigPath:     commonCfg.ConfigPath,
		InitialRuntime: commonCfg.InitialRuntime,
	}

	exitCode := RunDriver(cmd.Context(), tool, parsedArgs, driverConfig)
	os.Exit(exitCode)
	return nil
}

func isVersionFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--version" || arg == "-version" {
			return true
		}
	}
	return false
}

// SimpleDriver is a helper base implementation of the Driver interface
// that drivers can embed and override only the methods they need to customize.
// This reduces boilerplate for simple driver implementations.
type SimpleDriver struct {
	DriverName             string
	DriverVersion          string
	DriverShort            string
	DriverLong             string
	DriverPrefix           string
	DriverLanguageType     CompilerType
	DriverSupportsStdlib   bool
	DriverParseCommandLine func(args []string) interface{}
	DriverCreateJob        func(ctx context.Context, parsed interface{}, runtime *v1.Runtime, workDir string) (*v1.Job, error)
	DriverToolArgsApplier  func(context.Context) ToolArgsApplier
	DriverRuntimeValidator func(runtime *v1.Runtime) (bool, string)
	DriverRunFunc          func(ctx context.Context, args []string) int
	DriverListFunc         func(ctx context.Context) int
}

// Name returns the driver name
func (d *SimpleDriver) Name() string {
	return d.DriverName
}

// Version returns the driver version
func (d *SimpleDriver) Version() string {
	return d.DriverVersion
}

// Short returns the short description
func (d *SimpleDriver) Short() string {
	return d.DriverShort
}

// Long returns the long description
func (d *SimpleDriver) Long() string {
	return d.DriverLong
}

// ErrorPrefix returns the error prefix
func (d *SimpleDriver) ErrorPrefix() string {
	return d.DriverPrefix
}

// LanguageType returns the language type
func (d *SimpleDriver) LanguageType() DriverType {
	return d.DriverLanguageType
}

// SupportsStdlib returns whether the driver supports stdlib selection
func (d *SimpleDriver) SupportsStdlib() bool {
	return d.DriverSupportsStdlib
}

// ToolArgsApplier returns the args applier
func (d *SimpleDriver) ToolArgsApplier(ctx context.Context) ToolArgsApplier {
	if d.DriverToolArgsApplier != nil {
		return d.DriverToolArgsApplier
	}
	// Default: return args unchanged
	return func(ctx context.Context, baseRuntime string, toolArgs []string) (string, error) {
		return baseRuntime, nil
	}
}

// RunDriver delegates to the provided run function
func (d *SimpleDriver) RunDriver(ctx context.Context, args []string) int {
	if d.DriverRunFunc != nil {
		return d.DriverRunFunc(ctx, args)
	}
	return 1 // Error if not implemented
}

// ListCompatibleRuntimes delegates to the provided list function
func (d *SimpleDriver) ListCompatibleRuntimes(ctx context.Context) int {
	if d.DriverListFunc != nil {
		return d.DriverListFunc(ctx)
	}
	return 1 // Error if not implemented
}
