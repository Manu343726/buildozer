// Package toolchain provides detection and registry of available C/C++ toolchains on the system.
// Both drivers and runtimes use this package to discover and configure compilation environments.
package toolchain

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/Manu343726/buildozer/pkg/logging"
	"github.com/Manu343726/buildozer/pkg/runtimes/cpp/native"
)

// Registry maintains a cached list of available C/C++ toolchains on the system.
// It provides query methods for both drivers and runtimes to determine what
// compilation targets are available.
type Registry struct {
	*logging.Logger
	mu          sync.RWMutex
	detector    *native.Detector
	toolchains  map[string]*native.Toolchain // keyed by compiler name + language
	initialized bool
}

// NewRegistry creates a new toolchain registry.
func NewRegistry() *Registry {
	return &Registry{
		Logger:     Log().Child("Registry"),
		detector:   native.NewDetector(),
		toolchains: make(map[string]*native.Toolchain),
	}
}

// Initialize detects and caches all available toolchains on the system.
// This should be called once at startup.
func (r *Registry) Initialize(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	toolchains, err := r.detector.DetectToolchains(ctx)
	if err != nil {
		return fmt.Errorf("failed to detect toolchains: %w", err)
	}

	for i := range toolchains {
		tc := &toolchains[i]
		key := r.toolchainKey(tc)
		r.toolchains[key] = tc
		r.Info("registered toolchain", "key", key, "path", tc.CompilerPath, "version", tc.CompilerVersion)
	}

	r.initialized = true
	return nil
}

// GetGCC returns the GCC toolchain for C compilation, or nil if not available.
// If multiple variants exist (different runtimes/architectures), returns the first one found.
func (r *Registry) GetGCC(ctx context.Context) *native.Toolchain {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Find the first gcc-c toolchain
	for _, tc := range r.toolchains {
		if tc.Compiler == native.CompilerGCC && tc.Language == native.LanguageC {
			return tc
		}
	}

	return nil
}

// GetGxx returns the G++ toolchain for C++ compilation, or nil if not available.
// If multiple variants exist (different runtimes/architectures), returns the first one found.
func (r *Registry) GetGxx(ctx context.Context) *native.Toolchain {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Find the first gcc-cpp toolchain
	for _, tc := range r.toolchains {
		if tc.Compiler == native.CompilerGCC && tc.Language == native.LanguageCpp {
			return tc
		}
	}

	return nil
}

// GetClang returns the Clang toolchain for C compilation, or nil if not available.
// If multiple variants exist (different runtimes/architectures), returns the first one found.
func (r *Registry) GetClang(ctx context.Context) *native.Toolchain {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Find the first clang-c toolchain
	for _, tc := range r.toolchains {
		if tc.Compiler == native.CompilerClang && tc.Language == native.LanguageC {
			return tc
		}
	}

	return nil
}

// GetClangxx returns the Clang++ toolchain for C++ compilation, or nil if not available.
// If multiple variants exist (different runtimes/architectures), returns the first one found.
func (r *Registry) GetClangxx(ctx context.Context) *native.Toolchain {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Find the first clang-cpp toolchain
	for _, tc := range r.toolchains {
		if tc.Compiler == native.CompilerClang && tc.Language == native.LanguageCpp {
			return tc
		}
	}

	return nil
}

// GetByCompilerAndLanguage returns a toolchain for the specified compiler and language.
// If multiple variants exist (different runtimes/architectures), returns the first one found.
// Returns nil if not available.
func (r *Registry) GetByCompilerAndLanguage(compiler native.Compiler, language native.Language) *native.Toolchain {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Find the first matching toolchain
	for _, tc := range r.toolchains {
		if tc.Compiler == compiler && tc.Language == language {
			return tc
		}
	}

	return nil
}

// ListToolchains returns all registered toolchains.
func (r *Registry) ListToolchains() []*native.Toolchain {
	r.mu.RLock()
	defer r.mu.RUnlock()

	toolchains := make([]*native.Toolchain, 0, len(r.toolchains))
	for _, tc := range r.toolchains {
		toolchains = append(toolchains, tc)
	}

	return toolchains
}

// CanExecute checks if a requested toolchain matches an available one.
// Used by the runtime to determine if it can execute a job.
func (r *Registry) CanExecute(compiler native.Compiler, language native.Language, arch native.Architecture) bool {
	tc := r.GetByCompilerAndLanguage(compiler, language)
	if tc == nil {
		return false
	}

	// Check if architecture matches
	if arch != native.ArchitectureUnspecified && tc.Architecture != arch {
		return false
	}

	return true
}

// Summary returns a human-readable summary of available toolchains.
func (r *Registry) Summary() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.toolchains) == 0 {
		return "No toolchains available"
	}

	var summaries []string
	for _, tc := range r.toolchains {
		summaries = append(summaries, tc.String())
	}

	return strings.Join(summaries, "; ")
}

// toolchainKey generates a unique key for a toolchain based on all its variant dimensions:
// compiler, language, C runtime, C++ stdlib, and target architecture.
// This ensures each unique compilation environment gets its own registry entry.
func (r *Registry) toolchainKey(tc *native.Toolchain) string {
	compilerName := "unknown"
	if tc.Compiler == native.CompilerGCC {
		compilerName = "gcc"
	} else if tc.Compiler == native.CompilerClang {
		compilerName = "clang"
	}

	languageName := "unknown"
	if tc.Language == native.LanguageC {
		languageName = "c"
	} else if tc.Language == native.LanguageCpp {
		languageName = "cpp"
	}

	// Include C runtime variant
	runtimeName := "unknown"
	if tc.CRuntime == native.CRuntimeGlibc {
		runtimeName = "glibc"
	} else if tc.CRuntime == native.CRuntimeMusl {
		runtimeName = "musl"
	}

	// For C++, include stdlib variant
	stdlibName := ""
	if tc.Language == native.LanguageCpp {
		stdlibName = "-"
		if tc.CppStdlib == native.CppStdlibLibstdcxx {
			stdlibName += "libstdcxx"
		} else if tc.CppStdlib == native.CppStdlibLibcxx {
			stdlibName += "libc++"
		} else {
			stdlibName += "unknown"
		}
	}

	// Include architecture variant
	archName := "unknown"
	if tc.Architecture == native.ArchitectureX86_64 {
		archName = "x86_64"
	} else if tc.Architecture == native.ArchitectureAArch64 {
		archName = "aarch64"
	} else if tc.Architecture == native.ArchitectureARM {
		archName = "arm"
	}

	// Format: compiler-language-runtime-stdlib-arch (stdlib only for C++)
	// Example: gcc-c-glibc-x86_64, clang-cpp-glibc-libstdcxx-x86_64
	if tc.Language == native.LanguageCpp {
		return fmt.Sprintf("%s-%s-%s%s-%s", compilerName, languageName, runtimeName, stdlibName, archName)
	}
	return fmt.Sprintf("%s-%s-%s-%s", compilerName, languageName, runtimeName, archName)
}

// Global registry instance
var globalRegistry *Registry
var registryMutex sync.Mutex

// Global returns the global toolchain registry.
// Creates one on first use.
func Global() *Registry {
	registryMutex.Lock()
	defer registryMutex.Unlock()

	if globalRegistry == nil {
		globalRegistry = NewRegistry()
	}

	return globalRegistry
}

// Init initializes the global registry.
func Init(ctx context.Context) error {
	return Global().Initialize(ctx)
}
