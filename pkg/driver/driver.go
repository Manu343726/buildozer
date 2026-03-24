// Package driver defines the Driver interface that all buildozer driver
// implementations must satisfy.
//
// A Driver represents a build-tool proxy (gcc, g++, clang, clang++, rustc, …)
// that can parse its own command line, create jobs, resolve runtimes, and
// validate runtime compatibility.
//
// The generic CLI handler in pkg/drivers calls these methods to orchestrate
// the full driver lifecycle without any language-specific knowledge.
package driver

import (
	"context"

	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
)

// Driver is the interface that every buildozer driver must implement.
// It provides all the information and methods that the generic
// drivers.ExecuteDriver function needs to work.
type Driver interface {
	// --- Metadata ---

	// Name returns the tool name used in the CLI, logging and identification
	// (e.g. "gcc", "g++", "clang", "rustc").
	Name() string

	// Version returns the version string displayed with --version.
	Version() string

	// Short returns the one-line description shown in help output.
	Short() string

	// Long returns the extended description shown in help output.
	Long() string

	// ErrorPrefix returns the prefix prepended to error messages
	// (e.g. "gcc: error:").
	ErrorPrefix() string

	// --- CLI configuration ---

	// ValidateArgs validates the raw command-line arguments for this driver.
	// Returns an error if any argument is invalid or unrecognized.
	ValidateArgs(args []string) error

	// --- Driver callbacks ---

	// ParseCommandLine parses the raw command-line arguments into a
	// driver-specific parsed representation. The returned value is opaque
	// to the generic framework and will be passed back to CreateJob.
	ParseCommandLine(args []string) interface{}

	// CreateJob builds a Job protocol buffer from the parsed arguments,
	// the resolved runtime, and the working directory.
	CreateJob(ctx context.Context, parsed interface{}, runtime *v1.Runtime, workDir string) (*v1.Job, error)

	// ApplyToolArgs modifies a base runtime descriptor based on tool-specific
	// flags extracted from the command line (e.g. -march, -std, etc.).
	// It returns a new runtime descriptor with the flags applied.
	ApplyToolArgs(ctx context.Context, baseRuntime *v1.Runtime, toolArgs []string) (*v1.Runtime, error)

	// ValidateRuntime checks whether a given runtime is compatible with
	// this driver. It returns (true, "") when the runtime is acceptable,
	// or (false, reason) when it is not.
	ValidateRuntime(runtime *v1.Runtime) (bool, string)
}
