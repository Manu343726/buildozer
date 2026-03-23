// Package drivers provides utilities for driver command execution and runtime resolution.
package drivers

import (
	"context"
	"fmt"
	"strings"

	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	"github.com/Manu343726/buildozer/pkg/config"
)

// ToolchainResolution represents the resolved toolchain configuration
// that will be used for the job.
type ToolchainResolution struct {
	// CompilerVersion specifies the compiler version to use (e.g., "9", "10", "11")
	CompilerVersion string

	// CompilerType specifies the compiler type (e.g., "gcc", "clang")
	CompilerType string

	// CRuntime specifies the C runtime (e.g., "glibc", "musl")
	CRuntime string

	// CppStdLib specifies the C++ standard library (e.g., "libstdc++", "libc++")
	CppStdLib string

	// Architecture specifies the target architecture (e.g., "x86_64", "aarch64")
	Architecture string

	// Description is a human-readable description of the resolved toolchain
	Description string
}

// String returns a human-readable representation of the toolchain.
func (tr *ToolchainResolution) String() string {
	if tr.Description != "" {
		return tr.Description
	}

	parts := []string{}
	if tr.CompilerType != "" {
		parts = append(parts, tr.CompilerType)
	}
	if tr.CompilerVersion != "" {
		parts = append(parts, tr.CompilerVersion)
	}
	if tr.CRuntime != "" {
		parts = append(parts, tr.CRuntime)
	}
	if tr.CppStdLib != "" {
		parts = append(parts, tr.CppStdLib)
	}
	if tr.Architecture != "" {
		parts = append(parts, tr.Architecture)
	}

	if len(parts) == 0 {
		return "default"
	}

	return strings.Join(parts, " ")
}

// ResolveGccToolchain resolves the GCC toolchain based on config and command-line arguments.
//
// Precedence (highest to lowest):
//  1. Command-line flags (e.g., -m64, -march=armv8-a, etc.)
//  2. Configuration file (.buildozer)
//  3. System defaults
//
// Returns the resolved toolchain configuration.
func ResolveGccToolchain(ctx context.Context, cfg *config.CppDriverConfig, args []string) *ToolchainResolution {
	resolution := &ToolchainResolution{
		CompilerType: "gcc",
	}

	// Start with config file settings
	if cfg.CompilerVersion != "" {
		resolution.CompilerVersion = cfg.CompilerVersion
	}
	if cfg.CRuntime != "" {
		resolution.CRuntime = cfg.CRuntime
	}
	if cfg.Architecture != "" {
		resolution.Architecture = cfg.Architecture
	}

	// Parse command-line flags to override config
	parseGccArchitectureFlags(args, resolution)
	parseGccRuntimeFlags(args, resolution)

	return resolution
}

// ResolveGxxToolchain resolves the G++ toolchain based on config and command-line arguments.
//
// Similar to ResolveGccToolchain but for C++.
func ResolveGxxToolchain(ctx context.Context, cfg *config.CppDriverConfig, args []string) *ToolchainResolution {
	resolution := &ToolchainResolution{
		CompilerType: "gcc", // G++ uses GCC compiler behind the scenes
	}

	// Start with config file settings
	if cfg.CompilerVersion != "" {
		resolution.CompilerVersion = cfg.CompilerVersion
	}
	if cfg.CRuntime != "" {
		resolution.CRuntime = cfg.CRuntime
	}
	if cfg.Architecture != "" {
		resolution.Architecture = cfg.Architecture
	}
	if cfg.CppStdLib != "" {
		resolution.CppStdLib = cfg.CppStdLib
	}

	// Parse command-line flags to override config
	parseGccArchitectureFlags(args, resolution)
	parseGccRuntimeFlags(args, resolution)
	parseGxxStdLibFlags(args, resolution)

	return resolution
}

// parseGccArchitectureFlags extracts architecture-related flags from GCC/G++ arguments.
func parseGccArchitectureFlags(args []string, resolution *ToolchainResolution) {
	for i := 0; i < len(args); i++ {
		arg := args[i]

		// Handle -m flags for architecture
		if strings.HasPrefix(arg, "-m") {
			text := arg[2:]
			switch {
			case text == "64":
				resolution.Architecture = "x86_64"
			case text == "32":
				resolution.Architecture = "i386"
			case strings.HasPrefix(text, "march="):
				march := text[6:]
				switch {
				case march == "armv8-a" || march == "armv8" || march == "aarch64":
					resolution.Architecture = "aarch64"
				case march == "armv7-a" || march == "armv7" || march == "arm":
					resolution.Architecture = "armv7"
				case march == "x86-64" || march == "x86_64":
					resolution.Architecture = "x86_64"
				}
			}
		}

		// Handle -march flag separately
		if arg == "-march" && i+1 < len(args) {
			march := args[i+1]
			switch {
			case march == "armv8-a" || march == "armv8" || march == "aarch64":
				resolution.Architecture = "aarch64"
			case march == "armv7-a" || march == "armv7" || march == "arm":
				resolution.Architecture = "armv7"
			case march == "x86-64" || march == "x86_64":
				resolution.Architecture = "x86_64"
			}
			i++
		}
	}
}

// parseGccRuntimeFlags extracts C runtime flags from GCC/G++ arguments.
func parseGccRuntimeFlags(args []string, resolution *ToolchainResolution) {
	for i := 0; i < len(args); i++ {
		arg := args[i]

		// Check for -run-time or custom flags
		if arg == "-run-time" && i+1 < len(args) {
			resolution.CRuntime = args[i+1]
			i++
			continue
		}

		// Detect mlibc flag
		if arg == "-mlibc" && i+1 < len(args) {
			libc := args[i+1]
			if libc == "glibc" || libc == "musl" {
				resolution.CRuntime = libc
			}
			i++
		}

		// Some systems use -static-libgcc or -static-libmusl
		if arg == "-static-libgcc" {
			if resolution.CRuntime == "" {
				resolution.CRuntime = "glibc"
			}
		}
	}
}

// parseGxxStdLibFlags extracts C++ standard library flags from G++ arguments.
func parseGxxStdLibFlags(args []string, resolution *ToolchainResolution) {
	for i := 0; i < len(args); i++ {
		arg := args[i]

		// -stdlib=libc++ or -stdlib=libstdc++
		if strings.HasPrefix(arg, "-stdlib=") {
			stdlib := arg[8:]
			if stdlib == "libc++" || stdlib == "libstdc++" {
				resolution.CppStdLib = stdlib
			}
		}

		// Separate -stdlib flag
		if arg == "-stdlib" && i+1 < len(args) {
			stdlib := args[i+1]
			if stdlib == "libc++" || stdlib == "libstdc++" {
				resolution.CppStdLib = stdlib
			}
			i++
		}
	}
}

// CppToolchainForResolution creates a protobuf CppToolchain from a ToolchainResolution.
func CppToolchainForResolution(resolution *ToolchainResolution, language v1.CppLanguage) *v1.CppToolchain {
	toolchain := &v1.CppToolchain{
		Language: language,
		Compiler: v1.CppCompiler_CPP_COMPILER_GCC,
	}

	if resolution.CompilerVersion != "" {
		toolchain.CompilerVersion = versionFromResolution(resolution.CompilerVersion)
	}

	if resolution.Architecture != "" {
		switch resolution.Architecture {
		case "x86_64":
			toolchain.Architecture = v1.CpuArchitecture_CPU_ARCHITECTURE_X86_64
		case "i386":
			toolchain.Architecture = v1.CpuArchitecture_CPU_ARCHITECTURE_X86_64 // Normalize
		case "aarch64":
			toolchain.Architecture = v1.CpuArchitecture_CPU_ARCHITECTURE_AARCH64
		case "armv7", "arm":
			toolchain.Architecture = v1.CpuArchitecture_CPU_ARCHITECTURE_ARM
		}
	}

	if resolution.CRuntime != "" {
		switch resolution.CRuntime {
		case "glibc":
			toolchain.CRuntime = v1.CRuntime_C_RUNTIME_GLIBC
		case "musl":
			toolchain.CRuntime = v1.CRuntime_C_RUNTIME_MUSL
		}
	}

	if resolution.CppStdLib != "" {
		switch resolution.CppStdLib {
		case "libstdc++":
			toolchain.CppStdlib = v1.CppStdlib_CPP_STDLIB_LIBSTDCXX
		case "libc++":
			toolchain.CppStdlib = v1.CppStdlib_CPP_STDLIB_LIBCXX
		}
	}

	return toolchain
}

// versionFromResolution creates a Version message from a version string
func versionFromResolution(versionStr string) *v1.Version {
	if versionStr == "" {
		return nil
	}

	// Simple version string parsing - store in prerelease field for now
	return &v1.Version{
		Major:      0,
		Prerelease: &versionStr,
	}
}

// FormatUnavailableRuntimeWarning formats a user-friendly warning message
// about an unavailable runtime.
func FormatUnavailableRuntimeWarning(compilerName string, resolution *ToolchainResolution, availableRuntimes []*v1.Runtime) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Warning: The %s toolchain '%s' is not available on this machine.\n\n", compilerName, resolution.String()))
	sb.WriteString("This means you cannot build the project locally without Buildozer.\n")

	if len(availableRuntimes) > 0 {
		sb.WriteString("Available toolchains on this machine:\n")
		for _, rt := range availableRuntimes {
			if cpp := rt.GetCpp(); cpp != nil {
				desc := formatCppToolchain(cpp)
				if desc != "" {
					sb.WriteString(fmt.Sprintf("  - %s\n", desc))
				}
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString(fmt.Sprintf("Requested toolchain: %s\n", resolution.String()))

	return sb.String()
}

// formatCppToolchain formats a C/C++ toolchain for display
func formatCppToolchain(cpp *v1.CppToolchain) string {
	var parts []string

	switch cpp.Compiler {
	case v1.CppCompiler_CPP_COMPILER_GCC:
		parts = append(parts, "gcc")
	case v1.CppCompiler_CPP_COMPILER_CLANG:
		parts = append(parts, "clang")
	default:
		parts = append(parts, "unknown")
	}

	if cpp.CompilerVersion != nil && cpp.CompilerVersion.Prerelease != nil {
		parts = append(parts, *cpp.CompilerVersion.Prerelease)
	}

	switch cpp.Language {
	case v1.CppLanguage_CPP_LANGUAGE_CPP:
		parts = append(parts, "c++")
	}

	if cpp.Architecture != v1.CpuArchitecture_CPU_ARCHITECTURE_UNSPECIFIED {
		archStr := cpp.Architecture.String()[len("CPU_ARCHITECTURE_"):]
		archStr = strings.ToLower(archStr)
		parts = append(parts, fmt.Sprintf("(%s)", archStr))
	}

	if cpp.CRuntime != v1.CRuntime_C_RUNTIME_UNSPECIFIED {
		cruntime := cpp.CRuntime.String()[len("C_RUNTIME_"):]
		cruntime = strings.ToLower(cruntime)
		parts = append(parts, cruntime)
	}

	if cpp.CppStdlib != v1.CppStdlib_CPP_STDLIB_UNSPECIFIED {
		stdlib := cpp.CppStdlib.String()[len("CPP_STDLIB_"):]
		stdlib = strings.ToLower(stdlib)
		parts = append(parts, stdlib)
	}

	return strings.Join(parts, " ")
}
