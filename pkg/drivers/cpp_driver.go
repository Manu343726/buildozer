// Package drivers provides C/C++-specific driver implementations for the generic CLI framework.
package drivers

import (
	"context"
	"fmt"
)

// CppDriver is a helper implementation of the Driver interface for C/C++ compilers.
// It handles the common setup for GCC, Clang, and other C/C++ compiler drivers.
// This is a generic type that works with any C/C++ BuildContext type via the interface{} pattern.
type CppDriver struct {
	driverName      string                                           // "gcc", "g++", "clang", etc.
	driverVersion   string                                           // "11.2.0 (Buildozer)"
	driverShort     string                                           // "GCC C compiler"
	driverLong      string                                           // Full description
	buildCtx        interface{}                                      // The build context (driver-specific type)
	cliConfig       *CLIConfig                                       // Language/stdlib support
	toolArgsApplier func(context.Context) ToolArgsApplier            // Flag extraction callback
	runFunc         func(context.Context, []string, interface{}) int // Driver run function (takes generic BuildContext)
	listFunc        func(context.Context, interface{}) int           // Runtimes list function (takes generic BuildContext)
}

// NewCppDriver creates a new C/C++ driver with the given configuration.
// This is a factory function that language-specific drivers (gcc, g++, clang, clang++)
// use to create their Driver implementation.
//
// Parameters:
//   - name: Tool name (e.g., "gcc", "clang")
//   - version: Version string (e.g., "11.2.0")
//   - short: Short description (e.g., "GCC C compiler")
//   - long: Long description with usage information
//   - buildCtx: The build context containing daemon/config options
//   - cliConfig: CLI configuration (language type, stdlib support)
//   - toolArgsApplier: Function that extracts language-specific flags
//   - runFunc: The main driver run function
//   - listFunc: The list compatible runtimes function
func NewCppDriver(
	name, version, short, long string,
	buildCtx interface{},
	cliConfig *CLIConfig,
	toolArgsApplier func(context.Context) ToolArgsApplier,
	runFunc func(context.Context, []string, interface{}) int,
	listFunc func(context.Context, interface{}) int,
) Driver {
	return &CppDriver{
		driverName:      name,
		driverVersion:   version,
		driverShort:     short,
		driverLong:      long,
		buildCtx:        buildCtx,
		cliConfig:       cliConfig,
		toolArgsApplier: toolArgsApplier,
		runFunc:         runFunc,
		listFunc:        listFunc,
	}
}

// Name returns the driver name
func (d *CppDriver) Name() string {
	return d.driverName
}

// Version returns the driver version
func (d *CppDriver) Version() string {
	return d.driverVersion
}

// Short returns the short description
func (d *CppDriver) Short() string {
	return d.driverShort
}

// Long returns the long description
func (d *CppDriver) Long() string {
	return d.driverLong
}

// ErrorPrefix returns the error prefix
func (d *CppDriver) ErrorPrefix() string {
	return fmt.Sprintf("%s: error:", d.driverName)
}

// BuildContext returns the build context
func (d *CppDriver) BuildContext() interface{} {
	return d.buildCtx
}

// CLIConfig returns the CLI configuration
func (d *CppDriver) CLIConfig() *CLIConfig {
	return d.cliConfig
}

// ToolArgsApplier returns the tool-specific args applier
func (d *CppDriver) ToolArgsApplier(ctx context.Context) ToolArgsApplier {
	return d.toolArgsApplier(ctx)
}

// RunDriver executes the driver with the given arguments
func (d *CppDriver) RunDriver(ctx context.Context, args []string, buildCtx interface{}) int {
	return d.runFunc(ctx, args, buildCtx)
}

// ListCompatibleRuntimes lists available runtimes compatible with this driver
func (d *CppDriver) ListCompatibleRuntimes(ctx context.Context, buildCtx interface{}) int {
	return d.listFunc(ctx, buildCtx)
}
