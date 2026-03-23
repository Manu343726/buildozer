package drivers

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"connectrpc.com/connect"
	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	"github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1/protov1connect"
)

// DaemonClient wraps a gRPC connection to the buildozer daemon
type DaemonClient struct {
	client protov1connect.RuntimeServiceClient
}

// NewDaemonClient creates a new client connecting to the daemon at the given endpoint
// The endpoint can be either "host:port" or a full URL like "http://host:port"
func NewDaemonClient(ctx context.Context, endpoint string) (*DaemonClient, error) {
	// Ensure endpoint has a protocol scheme
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		endpoint = "http://" + endpoint
	}

	httpClient := &http.Client{}
	client := protov1connect.NewRuntimeServiceClient(
		httpClient,
		endpoint,
	)

	return &DaemonClient{
		client: client,
	}, nil
}

// ListRuntimes queries the daemon for available C/C++ runtimes
func (dc *DaemonClient) ListRuntimes(ctx context.Context, localOnly bool) ([]*v1.Runtime, error) {
	resp, err := dc.client.ListRuntimes(ctx, &connect.Request[v1.ListRuntimesRequest]{
		Msg: &v1.ListRuntimesRequest{
			LocalOnly: localOnly,
		},
	})
	if err != nil {
		return nil, err
	}

	return resp.Msg.Runtimes, nil
}

// FindMatchingRuntime finds the best matching runtime for the given toolchain resolution
func (dc *DaemonClient) FindMatchingRuntime(ctx context.Context, resolution *ToolchainResolution, runtimes []*v1.Runtime) (*v1.Runtime, int) {
	if len(runtimes) == 0 {
		return nil, 0
	}

	// Filter to only C/C++ runtimes and score them
	bestScore := 0
	var bestFullRuntime *v1.Runtime

	for _, runtime := range runtimes {
		cpp := getRuntimeCppToolchain(runtime)
		if cpp == nil {
			continue
		}

		score := scoreToolchainMatch(resolution, cpp)
		if score > bestScore {
			bestScore = score
			bestFullRuntime = runtime
		}
	}

	return bestFullRuntime, bestScore
}

// getRuntimeCppToolchain extracts the C++ toolchain from a Runtime (if it's a C/C++ runtime)
func getRuntimeCppToolchain(runtime *v1.Runtime) *v1.CppToolchain {
	if runtime == nil {
		return nil
	}
	if cpp := runtime.GetCpp(); cpp != nil {
		return cpp
	}
	return nil
}

// scoreToolchainMatch scores how well a toolchain matches the requested configuration
// Returns: exact match (1000) > partial match (100) > any match (1)
func scoreToolchainMatch(resolution *ToolchainResolution, available *v1.CppToolchain) int {
	if available == nil {
		return 0
	}

	score := 0

	// Compiler type - critical match
	requested := stringToCppCompiler(resolution.CompilerType)
	if requested == available.Compiler {
		score += 500
	} else if requested == v1.CppCompiler_CPP_COMPILER_UNSPECIFIED {
		score += 250 // Wildcard matches
	} else {
		return 0 // Compiler type mismatch is fatal
	}

	// Compiler version - important match
	if versionFromString(resolution.CompilerVersion) != nil && versionEqual(versionFromString(resolution.CompilerVersion), available.CompilerVersion) {
		score += 250
	} else if resolution.CompilerVersion == "" {
		score += 125 // Wildcard
	} else {
		score += 50 // Partial match - same compiler type but different version
	}

	// Architecture - important match
	if stringToCpuArchitecture(resolution.Architecture) == available.Architecture {
		score += 100
	} else if resolution.Architecture == "" {
		score += 50
	}

	// C runtime - important for binary compatibility
	if stringToCRuntime(resolution.CRuntime) == available.CRuntime {
		score += 75
	} else if resolution.CRuntime == "" {
		score += 37
	}

	// C++ stdlib - important for C++ programs
	if stringToCppStdlib(resolution.CppStdLib) == available.CppStdlib {
		score += 75
	} else if resolution.CppStdLib == "" {
		score += 37
	}

	return score
}

// stringToCppCompiler converts a compiler name string to the protobuf enum
func stringToCppCompiler(s string) v1.CppCompiler {
	switch s {
	case "gcc":
		return v1.CppCompiler_CPP_COMPILER_GCC
	case "clang":
		return v1.CppCompiler_CPP_COMPILER_CLANG
	default:
		return v1.CppCompiler_CPP_COMPILER_UNSPECIFIED
	}
}

// stringToCpuArchitecture converts an architecture string to the protobuf enum
func stringToCpuArchitecture(s string) v1.CpuArchitecture {
	switch s {
	case "x86_64":
		return v1.CpuArchitecture_CPU_ARCHITECTURE_X86_64
	case "i386", "i686":
		return v1.CpuArchitecture_CPU_ARCHITECTURE_X86_64 // Normalize to x86_64 if possible
	case "aarch64":
		return v1.CpuArchitecture_CPU_ARCHITECTURE_AARCH64
	case "armv7", "arm":
		return v1.CpuArchitecture_CPU_ARCHITECTURE_ARM
	default:
		return v1.CpuArchitecture_CPU_ARCHITECTURE_UNSPECIFIED
	}
}

// stringToCRuntime converts a C runtime name string to the protobuf enum
func stringToCRuntime(s string) v1.CRuntime {
	switch s {
	case "glibc":
		return v1.CRuntime_C_RUNTIME_GLIBC
	case "musl":
		return v1.CRuntime_C_RUNTIME_MUSL
	default:
		return v1.CRuntime_C_RUNTIME_UNSPECIFIED
	}
}

// stringToCppStdlib converts a C++ stdlib name string to the protobuf enum
func stringToCppStdlib(s string) v1.CppStdlib {
	switch s {
	case "libstdc++":
		return v1.CppStdlib_CPP_STDLIB_LIBSTDCXX
	case "libc++":
		return v1.CppStdlib_CPP_STDLIB_LIBCXX
	default:
		return v1.CppStdlib_CPP_STDLIB_UNSPECIFIED
	}
}

// versionFromString converts a version string (e.g., "11.2.0") to a Version message
func versionFromString(s string) *v1.Version {
	if s == "" {
		return nil
	}

	// Convert string to Version message - for now, just store in prerelease field
	// In a real implementation, you'd parse major.minor.patch
	zeros := uint32(0)
	return &v1.Version{
		Major:      0,
		Minor:      &zeros,
		Patch:      &zeros,
		Prerelease: &s,
	}
}

// versionEqual compares two Version messages for equality
func versionEqual(v1Msg, v2Msg *v1.Version) bool {
	if v1Msg == nil && v2Msg == nil {
		return true
	}
	if v1Msg == nil || v2Msg == nil {
		return false
	}

	// Compare major version
	if v1Msg.Major != v2Msg.Major {
		return false
	}

	// Compare optional fields
	v1Minor := uint32(0)
	if v1Msg.Minor != nil {
		v1Minor = *v1Msg.Minor
	}
	v2Minor := uint32(0)
	if v2Msg.Minor != nil {
		v2Minor = *v2Msg.Minor
	}

	if v1Minor != v2Minor {
		return false
	}

	v1Patch := uint32(0)
	if v1Msg.Patch != nil {
		v1Patch = *v1Msg.Patch
	}
	v2Patch := uint32(0)
	if v2Msg.Patch != nil {
		v2Patch = *v2Msg.Patch
	}

	if v1Patch != v2Patch {
		return false
	}

	v1Pre := ""
	if v1Msg.Prerelease != nil {
		v1Pre = *v1Msg.Prerelease
	}
	v2Pre := ""
	if v2Msg.Prerelease != nil {
		v2Pre = *v2Msg.Prerelease
	}

	return v1Pre == v2Pre
}

// NoRuntimeFound returns a sorted list (for deterministic output) of available runtimes for display
func NoRuntimeFound(runtime string, availableRuntimes []*v1.Runtime) error {
	// Extract only C++ runtimes
	var available []*v1.CppToolchain
	for _, rt := range availableRuntimes {
		if cpp := getRuntimeCppToolchain(rt); cpp != nil {
			available = append(available, cpp)
		}
	}

	if len(available) == 0 {
		return fmt.Errorf("no C/C++ runtimes available on this machine")
	}

	// Sort runtimes for deterministic display
	sort.Slice(available, func(i, j int) bool {
		return formatToolchain(available[i]) < formatToolchain(available[j])
	})

	msg := fmt.Sprintf("Requested C/C++ runtime configuration [%s] not found.\nAvailable runtimes:\n", runtime)
	for _, tc := range available {
		msg += fmt.Sprintf("  - %s\n", formatToolchain(tc))
	}

	return fmt.Errorf("%s", msg)
}

// formatToolchain creates a human-readable string representation of a toolchain
func formatToolchain(toolchain *v1.CppToolchain) string {
	if toolchain == nil {
		return "(unknown)"
	}

	parts := []string{}

	// Add language
	langStr := ""
	switch toolchain.Language {
	case v1.CppLanguage_CPP_LANGUAGE_C:
		langStr = "c"
	case v1.CppLanguage_CPP_LANGUAGE_CPP:
		langStr = "cpp"
	}
	if langStr != "" {
		parts = append(parts, langStr)
	}

	// Add compiler type
	compStr := toolchain.Compiler.String()
	if compStr != "" && compStr != "CPP_COMPILER_UNSPECIFIED" {
		compStr = compStr[len("CPP_COMPILER_"):]
		parts = append(parts, compStr)
	}

	// Add compiler version
	if toolchain.CompilerVersion != nil && toolchain.CompilerVersion.Prerelease != nil && *toolchain.CompilerVersion.Prerelease != "" {
		parts = append(parts, *toolchain.CompilerVersion.Prerelease)
	}

	// Add architecture
	archStr := toolchain.Architecture.String()
	if archStr != "" && archStr != "CPU_ARCHITECTURE_UNSPECIFIED" {
		archStr = archStr[len("CPU_ARCHITECTURE_"):]
		parts = append(parts, archStr)
	}

	// Add C runtime
	if toolchain.CRuntime != v1.CRuntime_C_RUNTIME_UNSPECIFIED {
		runtime := toolchain.CRuntime.String()
		runtime = runtime[len("C_RUNTIME_"):]
		parts = append(parts, runtime)
	}

	// Add C++ stdlib
	if toolchain.CppStdlib != v1.CppStdlib_CPP_STDLIB_UNSPECIFIED {
		stdlib := toolchain.CppStdlib.String()[len("CPP_STDLIB_"):]
		stdlib = fmt.Sprintf("[%s]", stdlib)
		parts = append(parts, stdlib)
	}

	result := ""
	for i, part := range parts {
		if i == 0 {
			result = part
		} else {
			result += " " + part
		}
	}

	return result
}
