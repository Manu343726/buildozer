package native

import (
	"context"

	"github.com/Manu343726/buildozer/pkg/logging"
	"github.com/Manu343726/buildozer/pkg/runtime"
	"github.com/Manu343726/buildozer/pkg/sandbox"
)

// CppDiscoverer discovers and registers available C/C++ runtimes on the system.
// It implements the runtime.Discoverer interface and scans for installed compilers,
// then creates and registers corresponding runtime implementations.
type CppDiscoverer struct {
	*logging.Logger // Embed Logger for logging discoverer operations

	workDir string // working directory for temporary files and compiler commands
}

// NewCppDiscoverer creates and returns a new C/C++ runtime discoverer.
// Returns a runtime.Discoverer interface implementation.
func NewCppDiscoverer(workDir string) runtime.Discoverer {
	return &CppDiscoverer{
		Logger:  Log().Child("CppDiscoverer"),
		workDir: workDir,
	}
}

// Discover scans the system for available C/C++ compilers and registers them as runtimes.
// Implements the runtime.Discoverer interface.
func (d *CppDiscoverer) Discover(ctx context.Context, registry *runtime.Registry) error {
	d.Info("discovering C/C++ runtimes")

	detector := NewDetector()
	toolchains, err := detector.DetectToolchains(ctx)
	if err != nil {
		return d.Errorf("failed to detect toolchains: %w", err)
	}

	if len(toolchains) == 0 {
		d.Warn("no C/C++ toolchains found on system")
		return nil
	}

	d.Info("discovered toolchains", "count", len(toolchains))

	// Register runtimes for each detected toolchain
	for _, toolchain := range toolchains {
		d.Debug("registering toolchain", "compiler", toolchain.Compiler, "language", toolchain.Language, "version", toolchain.CompilerVersion, "arch", toolchain.Architecture)

		cppRuntime := NewNativeCppRuntime(&toolchain, d.workDir)

		// Wrap the C++ runtime with TempDir sandbox to handle file materialization
		// This allows the C++ runtime to work with reference data only without copying
		params := sandbox.SandboxParams{Logger: d.Logger}
		wrappedRuntime := sandbox.MustApply(cppRuntime, params, sandbox.TempDir())

		if err := registry.Register(wrappedRuntime); err != nil {
			d.Errorf("failed to register runtime: %w", err)
		} else {
			d.Info("registered runtime with tempdir sandbox", "id", wrappedRuntime.RuntimeID())
		}
	}

	d.Info("C/C++ runtime discovery complete")
	return nil
}

// Name returns the identifier for this discoverer.
// This is used to distinguish different discoverer implementations.
func (d *CppDiscoverer) Name() string {
	return "cpp-native"
}
