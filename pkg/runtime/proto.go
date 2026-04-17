package runtime

import (
	"fmt"
	"strconv"
	"strings"

	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
)

// RuntimePlatform identifies the runtime platform and operating system (native_linux, docker_linux, native_windows, etc.)
type RuntimePlatform int

const (
	RuntimePlatformNativeLinux RuntimePlatform = iota
	RuntimePlatformDockerLinux
	// RuntimePlatformNativeWindows, etc. can be added in the future
)

// String returns the string representation of RuntimePlatform
func (rp RuntimePlatform) String() string {
	switch rp {
	case RuntimePlatformNativeLinux:
		return "native_linux"
	case RuntimePlatformDockerLinux:
		return "docker_linux"
	default:
		return "unknown"
	}
}

// ParseRuntimePlatform converts a string to RuntimePlatform
func ParseRuntimePlatform(s string) (RuntimePlatform, error) {
	switch s {
	case "native_linux":
		return RuntimePlatformNativeLinux, nil
	case "docker_linux":
		return RuntimePlatformDockerLinux, nil
	default:
		return RuntimePlatformNativeLinux, fmt.Errorf("unknown runtime platform: %q", s)
	}
}

// RuntimeToolchain identifies the toolchain/language family (c, cpp, go, rust, etc.)
// Note: AR is NOT a separate toolchain - it's part of the C toolchain and uses C runtimes
type RuntimeToolchain int

const (
	RuntimeToolchainC RuntimeToolchain = iota
	RuntimeToolchainCpp
	// RuntimeToolchainGo, RuntimeToolchainRust, etc. can be added in the future
)

// String returns the string representation of RuntimeToolchain
func (rt RuntimeToolchain) String() string {
	switch rt {
	case RuntimeToolchainC:
		return "c"
	case RuntimeToolchainCpp:
		return "cpp"
	default:
		return "unknown"
	}
}

// ParseRuntimeToolchain converts a string to RuntimeToolchain
func ParseRuntimeToolchain(s string) (RuntimeToolchain, error) {
	switch s {
	case "c":
		return RuntimeToolchainC, nil
	case "cpp":
		return RuntimeToolchainCpp, nil
	default:
		return RuntimeToolchainC, fmt.Errorf("unknown runtime toolchain: %q", s)
	}
}

// RuntimeParser is an interface for parsing runtime-specific IDs.
// Implementations handle the format-specific parsing for different runtime toolchains.
type RuntimeParser interface {
	// Parse converts a runtime ID string (without the platform/toolchain prefix) into a *v1.Runtime proto.
	// idParts contains the dash-separated parts after the platform-toolchain prefix (e.g., "gcc", "10.2.1", ...)
	Parse(idParts []string) (*v1.Runtime, error)
}

// runtimeParsers maps runtime toolchain strings (e.g., "native_linux-c", "docker_linux-go") to their parser implementations
// This allows extending with new toolchains and platforms without modifying this package
var runtimeParsers = make(map[string]RuntimeParser)

// RegisterRuntimeParser registers a parser for a specific runtime toolchain and platform
// toolchainKey should be in format "platform-toolchain" (e.g., "native_linux-c", "docker_linux-go")
func RegisterRuntimeParser(toolchainKey string, parser RuntimeParser) {
	runtimeParsers[toolchainKey] = parser
}

// ParseRuntimeID parses a runtime ID string into a *v1.Runtime proto.
// It determines the runtime platform and toolchain, then delegates to the appropriate parser.
//
// ID format stages:
// 1. Platform: "native_linux", "docker_linux", "native_windows", etc.
// 2. Toolchain: "c", "cpp", "go", "rust", etc.
// 3. Rest: toolchain-specific format parsed by the registered RuntimeParser
//
// Examples of valid IDs:
//
//	native_linux-c-gcc-10.2.1-glibc-2.31-x86_64
//	native_linux-cpp-gcc-10.2.1-glibc-2.31-libstdc++-x86_64
//	docker_linux-go-1.18
//
// Note: AR jobs run on C runtimes, not separate ar runtimes
func ParseRuntimeID(id string) (*v1.Runtime, error) {
	if id == "" {
		return nil, fmt.Errorf("empty runtime ID")
	}

	parts := strings.Split(id, "-")
	if len(parts) < 4 {
		return nil, fmt.Errorf("invalid runtime ID %q: expected at least 4 dash-separated parts (platform_os-toolchain-...)", id)
	}

	// Stage 1: Determine runtime platform (native_linux, docker_linux, etc.)
	// Handle platform names that contain underscores (e.g., "native_linux", "docker_linux")
	var runtimePlatform RuntimePlatform
	var toolchainStartIdx int

	// Check for two-part platform names (e.g., "native_linux", "docker_linux")
	if len(parts) >= 4 && (parts[0] == "native" || parts[0] == "docker") {
		platformStr := parts[0] + "_" + parts[1]
		p, err := ParseRuntimePlatform(platformStr)
		if err == nil {
			runtimePlatform = p
			toolchainStartIdx = 2
		} else {
			// Fallback to single-part platform
			p, err := ParseRuntimePlatform(parts[0])
			if err != nil {
				return nil, err
			}
			runtimePlatform = p
			toolchainStartIdx = 1
		}
	} else {
		p, err := ParseRuntimePlatform(parts[0])
		if err != nil {
			return nil, err
		}
		runtimePlatform = p
		toolchainStartIdx = 1
	}

	// Stage 2: Determine runtime toolchain (c, cpp, go, rust, etc.)
	if toolchainStartIdx >= len(parts) {
		return nil, fmt.Errorf("invalid runtime ID %q: missing toolchain after platform", id)
	}
	runtimeToolchain, err := ParseRuntimeToolchain(parts[toolchainStartIdx])
	if err != nil {
		return nil, err
	}

	// Stage 3: Delegate to platform-toolchain-specific parser
	// Look up parser by platform-toolchain combination
	parserKey := fmt.Sprintf("%s-%s", runtimePlatform.String(), runtimeToolchain.String())
	parser, ok := runtimeParsers[parserKey]
	if !ok {
		return nil, fmt.Errorf("no parser registered for %s", parserKey)
	}

	// Pass remaining parts (without platform and toolchain) to the parser
	remainingParts := parts[toolchainStartIdx+1:]
	rt, err := parser.Parse(remainingParts)
	if err != nil {
		return nil, fmt.Errorf("failed to parse runtime ID %q: %w", id, err)
	}

	// Set common fields
	rt.Id = id
	// Convert RuntimePlatform (Go type) to v1.RuntimePlatform (proto enum)
	switch runtimePlatform {
	case RuntimePlatformNativeLinux:
		rt.Platform = v1.RuntimePlatform_RUNTIME_PLATFORM_NATIVE_LINUX
	case RuntimePlatformDockerLinux:
		rt.Platform = v1.RuntimePlatform_RUNTIME_PLATFORM_DOCKER_LINUX
	default:
		rt.Platform = v1.RuntimePlatform_RUNTIME_PLATFORM_UNSPECIFIED
	}

	return rt, nil
}

// RuntimeToID generates a runtime ID string from a *v1.Runtime proto.
// This is the inverse of ParseRuntimeID.
// Generates IDs like: native_linux-c-gcc-10.2.1-glibc-2.31-x86_64
func RuntimeToID(rt *v1.Runtime) string {
	cpp := rt.GetCpp()
	if cpp == nil {
		return rt.GetId()
	}

	platform := "native_linux"
	if rt.Platform == v1.RuntimePlatform_RUNTIME_PLATFORM_DOCKER_LINUX {
		platform = "docker_linux"
	}

	lang := languageToString(cpp.Language)
	compiler := compilerToString(cpp.Compiler)
	version := versionToString(cpp.CompilerVersion)
	crt := cRuntimeToString(cpp.CRuntime)
	crtVersion := versionToString(cpp.CRuntimeVersion)
	arch := architectureToString(cpp.Architecture)

	// Build ID: platform-toolchain-compiler-version-cruntime-cruntime_version[-stdlib]-arch
	id := fmt.Sprintf("%s-%s-%s-%s-%s-%s", platform, lang, compiler, version, crt, crtVersion)

	if cpp.Language == v1.CppLanguage_CPP_LANGUAGE_CPP {
		id = fmt.Sprintf("%s-%s", id, stdlibToString(cpp.CppStdlib))
	}

	id = fmt.Sprintf("%s-%s", id, arch)
	return id
}

// --- String → Proto parsers (exported for use by driver packages) ---

// ParseVersionString parses a version string like "10.2.1" into a *v1.Version proto.
func ParseVersionString(s string) *v1.Version {
	return parseVersion(s)
}

// ParseArchitectureString parses an architecture string to a proto enum.
func ParseArchitectureString(s string) v1.CpuArchitecture {
	return parseArchitecture(s)
}

// ParseStdlibString parses a C++ stdlib string to a proto enum.
func ParseStdlibString(s string) v1.CppStdlib {
	return parseStdlib(s)
}

func parseLanguage(s string) v1.CppLanguage {
	switch s {
	case "c":
		return v1.CppLanguage_CPP_LANGUAGE_C
	case "cpp":
		return v1.CppLanguage_CPP_LANGUAGE_CPP
	default:
		return v1.CppLanguage_CPP_LANGUAGE_UNSPECIFIED
	}
}

func parseCompiler(s string) v1.CppCompiler {
	switch s {
	case "gcc":
		return v1.CppCompiler_CPP_COMPILER_GCC
	case "clang":
		return v1.CppCompiler_CPP_COMPILER_CLANG
	default:
		return v1.CppCompiler_CPP_COMPILER_UNSPECIFIED
	}
}

func parseArchitecture(s string) v1.CpuArchitecture {
	switch s {
	case "x86_64":
		return v1.CpuArchitecture_CPU_ARCHITECTURE_X86_64
	case "aarch64":
		return v1.CpuArchitecture_CPU_ARCHITECTURE_AARCH64
	case "arm":
		return v1.CpuArchitecture_CPU_ARCHITECTURE_ARM
	default:
		return v1.CpuArchitecture_CPU_ARCHITECTURE_UNSPECIFIED
	}
}

func parseCRuntime(s string) v1.CRuntime {
	switch s {
	case "glibc":
		return v1.CRuntime_C_RUNTIME_GLIBC
	case "musl":
		return v1.CRuntime_C_RUNTIME_MUSL
	default:
		return v1.CRuntime_C_RUNTIME_UNSPECIFIED
	}
}

func parseStdlib(s string) v1.CppStdlib {
	switch s {
	case "libstdc++":
		return v1.CppStdlib_CPP_STDLIB_LIBSTDCXX
	case "libc++":
		return v1.CppStdlib_CPP_STDLIB_LIBCXX
	default:
		return v1.CppStdlib_CPP_STDLIB_UNSPECIFIED
	}
}

func parseVersion(s string) *v1.Version {
	if s == "" || s == "unknown" {
		return &v1.Version{Major: 0}
	}

	pv := &v1.Version{}
	parts := strings.Split(s, ".")

	if len(parts) > 0 {
		if major, err := strconv.ParseUint(parts[0], 10, 32); err == nil {
			pv.Major = uint32(major)
		}
	}
	if len(parts) > 1 {
		if minor, err := strconv.ParseUint(parts[1], 10, 32); err == nil {
			m := uint32(minor)
			pv.Minor = &m
		}
	}
	if len(parts) > 2 {
		if patch, err := strconv.ParseUint(parts[2], 10, 32); err == nil {
			p := uint32(patch)
			pv.Patch = &p
		}
	}

	return pv
}

// --- Proto → String converters ---

func languageToString(l v1.CppLanguage) string {
	switch l {
	case v1.CppLanguage_CPP_LANGUAGE_C:
		return "c"
	case v1.CppLanguage_CPP_LANGUAGE_CPP:
		return "cpp"
	default:
		return "unknown"
	}
}

func compilerToString(c v1.CppCompiler) string {
	switch c {
	case v1.CppCompiler_CPP_COMPILER_GCC:
		return "gcc"
	case v1.CppCompiler_CPP_COMPILER_CLANG:
		return "clang"
	default:
		return "unknown"
	}
}

func architectureToString(a v1.CpuArchitecture) string {
	switch a {
	case v1.CpuArchitecture_CPU_ARCHITECTURE_X86_64:
		return "x86_64"
	case v1.CpuArchitecture_CPU_ARCHITECTURE_AARCH64:
		return "aarch64"
	case v1.CpuArchitecture_CPU_ARCHITECTURE_ARM:
		return "arm"
	default:
		return "unknown"
	}
}

func cRuntimeToString(c v1.CRuntime) string {
	switch c {
	case v1.CRuntime_C_RUNTIME_GLIBC:
		return "glibc"
	case v1.CRuntime_C_RUNTIME_MUSL:
		return "musl"
	default:
		return "unknown"
	}
}

func stdlibToString(s v1.CppStdlib) string {
	switch s {
	case v1.CppStdlib_CPP_STDLIB_LIBSTDCXX:
		return "libstdc++"
	case v1.CppStdlib_CPP_STDLIB_LIBCXX:
		return "libc++"
	default:
		return "unknown"
	}
}

func versionToString(v *v1.Version) string {
	if v == nil {
		return "0"
	}

	s := strconv.FormatUint(uint64(v.Major), 10)

	if v.Minor != nil {
		s += "." + strconv.FormatUint(uint64(*v.Minor), 10)
	}
	if v.Patch != nil {
		s += "." + strconv.FormatUint(uint64(*v.Patch), 10)
	}

	return s
}
