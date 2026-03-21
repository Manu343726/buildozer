package native

import (
	"context"

	"github.com/Manu343726/buildozer/internal/logger"
	"github.com/Manu343726/buildozer/pkg/runtime"
)

// CppDiscoverer discovers and registers available C/C++ runtimes on the system.
// It implements the runtime.Discoverer interface and scans for installed compilers,
// then creates and registers corresponding runtime implementations.
// For each discovered compiler, it creates both C and C++ variants.
type CppDiscoverer struct {
	// workDir is the working directory for temporary files and compiler commands.
	workDir string
	// log is the logger for discoverer operations.
	log *logger.ComponentLogger
}

// NewCppDiscoverer creates and returns a new C/C++ runtime discoverer.
// Parameter:
// - workDir: The directory for temporary files and compiler operations
func NewCppDiscoverer(workDir string) *CppDiscoverer {
	return &CppDiscoverer{
		workDir: workDir,
		log:     logger.NewComponentLogger("cpp-discoverer"),
	}
}

// Discover scans the system for available C/C++ compilers and registers them as runtimes.
// This method implements the runtime.Discoverer interface.
// For each detected compiler, it creates both a C runtime and a C++ runtime variant,
// then registers them with the provided registry.
// Returns an error only if there are fatal system issues; missing compilers is not an error.
func (d *CppDiscoverer) Discover(ctx context.Context, registry *runtime.Registry) error {
	d.log.Info("discovering C/C++ runtimes")

	detector := NewDetector()
	compilers, err := detector.DetectCompilers(ctx)
	if err != nil {
		return d.log.Errorf("failed to detect compilers: %w", err)
	}

	if len(compilers) == 0 {
		d.log.Warn("no C/C++ compilers found on system")
		return nil
	}

	d.log.Info("discovered compilers", "count", len(compilers))

	// Register runtimes for each detected compiler
	for _, compiler := range compilers {
		d.log.Debug("registering compiler", "name", compiler.Name, "version", compiler.Version, "arch", compiler.Architecture)

		// Create concrete toolchain for C and C++ variants
		cToolchain := d.createCToolchain(compiler)
		cppToolchain := d.createCppToolchain(compiler)

		// Register C runtime
		cRuntime := NewNativeCppRuntime(cToolchain, d.workDir)
		if err := registry.Register(cRuntime); err != nil {
			d.log.Errorf("failed to register C runtime: %w", err)
		} else {
			d.log.Info("registered C runtime", "id", cRuntime.RuntimeID())
		}

		// Register C++ runtime
		cppRuntime := NewNativeCppRuntime(cppToolchain, d.workDir)
		if err := registry.Register(cppRuntime); err != nil {
			d.log.Errorf("failed to register C++ runtime: %w", err)
		} else {
			d.log.Info("registered C++ runtime", "id", cppRuntime.RuntimeID())
		}
	}

	d.log.Info("C/C++ runtime discovery complete")
	return nil
}

// Name returns the identifier for this discoverer.
// This is used to distinguish different discoverer implementations.
func (d *CppDiscoverer) Name() string {
	return "cpp-native"
}

// createCToolchain constructs a concrete C language Toolchain from compiler metadata.
// It initializes a Toolchain configured for C compilation with appropriate defaults.
func (d *CppDiscoverer) createCToolchain(info CompilerInfo) *Toolchain {
	arch := d.parseArchitecture(info.Architecture)
	return &Toolchain{
		Language:        LanguageC,
		Compiler:        d.parseCompiler(info.Name),
		CompilerPath:    info.Path,
		CompilerVersion: info.Version,
		Architecture:    arch,
		CRuntime:        d.parseCRuntime(info.Name),
		CRuntimeVersion: "",
		CppAbi:          CppAbiItanium,
		CppStdlib:       CppStdlibUnspecified,
		AbiModifiers:    []string{},
	}
}

// createCppToolchain constructs a concrete C++ language Toolchain from compiler metadata.
// It initializes a Toolchain configured for C++ compilation with appropriate C++ standard library.
func (d *CppDiscoverer) createCppToolchain(info CompilerInfo) *Toolchain {
	arch := d.parseArchitecture(info.Architecture)
	return &Toolchain{
		Language:        LanguageCpp,
		Compiler:        d.parseCompiler(info.Name),
		CompilerPath:    info.Path,
		CompilerVersion: info.Version,
		Architecture:    arch,
		CRuntime:        d.parseCRuntime(info.Name),
		CRuntimeVersion: "",
		CppAbi:          CppAbiItanium,
		CppStdlib:       d.parseStdlib(info.Name),
		AbiModifiers:    []string{},
	}
}

// parseCompiler determines the Compiler enum value from a compiler name string.
func (d *CppDiscoverer) parseCompiler(name string) Compiler {
	switch name {
	case "gcc", "g++":
		return CompilerGCC
	case "clang", "clang++":
		return CompilerClang
	default:
		return CompilerUnspecified
	}
}

// parseArchitecture converts an architecture string to the Architecture enum.
func (d *CppDiscoverer) parseArchitecture(arch string) Architecture {
	switch arch {
	case "x86_64":
		return ArchitectureX86_64
	case "aarch64", "arm64":
		return ArchitectureAArch64
	case "armv7", "armv7l", "armv6", "armv5", "arm":
		return ArchitectureARM
	default:
		return ArchitectureUnspecified
	}
}

// parseCRuntime determines the C runtime implementation from compiler name.
// GCC and Clang on Linux typically use glibc.
func (d *CppDiscoverer) parseCRuntime(name string) CRuntime {
	// Most Linux distributions use glibc
	// Future implementation could detect musl-based systems
	return CRuntimeGlibc
}

// parseStdlib determines the C++ standard library from the compiler name.
// GCC uses libstdc++ while Clang uses libc++.
func (d *CppDiscoverer) parseStdlib(name string) CppStdlib {
	switch name {
	case "clang", "clang++":
		return CppStdlibLibcxx
	case "gcc", "g++":
		return CppStdlibLibstdcxx
	default:
		return CppStdlibUnspecified
	}
}
