package runtime

import (
	"fmt"
	"strconv"
	"strings"

	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
)

// ParseRuntimeID parses a runtime ID string into a *v1.Runtime proto.
// The runtime ID format for native C/C++ runtimes is:
//
//	native-<lang>-<compiler>-<version>-<cruntime>-<cruntime_version>-<arch>           (C)
//	native-<lang>-<compiler>-<version>-<cruntime>-<cruntime_version>-<stdlib>-<arch>  (C++)
//
// Examples:
//
//	native-c-gcc-10.2.1-glibc-2.31-x86_64
//	native-cpp-gcc-10.2.1-glibc-2.31-libstdc++-x86_64
//	native-cpp-clang-14.0.0-glibc-2.31-libc++-aarch64
func ParseRuntimeID(id string) (*v1.Runtime, error) {
	if id == "" {
		return nil, fmt.Errorf("empty runtime ID")
	}

	parts := strings.Split(id, "-")
	if len(parts) < 7 {
		return nil, fmt.Errorf("invalid runtime ID %q: expected at least 7 dash-separated parts", id)
	}

	// parts[0] = "native" (or other type prefix in the future)
	isNative := parts[0] == "native"

	// parts[1] = language ("c" or "cpp")
	lang := parseLanguage(parts[1])

	// parts[2] = compiler ("gcc" or "clang")
	compiler := parseCompiler(parts[2])

	// parts[3] = compiler version (e.g. "10.2.1")
	compilerVersion := parseVersion(parts[3])

	// parts[4] = C runtime ("glibc" or "musl")
	cRuntime := parseCRuntime(parts[4])

	// parts[5] = C runtime version (e.g. "2.31")
	cRuntimeVersion := parseVersion(parts[5])

	// For C: parts[6] = architecture
	// For C++: parts[6] = stdlib, parts[7] = architecture
	var stdlib v1.CppStdlib
	var arch v1.CpuArchitecture

	if lang == v1.CppLanguage_CPP_LANGUAGE_CPP && len(parts) >= 8 {
		stdlib = parseStdlib(parts[6])
		arch = parseArchitecture(parts[7])
	} else {
		arch = parseArchitecture(parts[6])
	}

	rt := &v1.Runtime{
		Id:       id,
		IsNative: isNative,
		Toolchain: &v1.Runtime_Cpp{
			Cpp: &v1.CppToolchain{
				Language:        lang,
				Compiler:        compiler,
				CompilerVersion: compilerVersion,
				Architecture:    arch,
				CRuntime:        cRuntime,
				CRuntimeVersion: cRuntimeVersion,
				CppStdlib:       stdlib,
			},
		},
	}

	return rt, nil
}

// RuntimeToID generates a runtime ID string from a *v1.Runtime proto.
// This is the inverse of ParseRuntimeID.
func RuntimeToID(rt *v1.Runtime) string {
	cpp := rt.GetCpp()
	if cpp == nil {
		return rt.GetId()
	}

	prefix := "native"
	if !rt.IsNative {
		prefix = "docker"
	}

	lang := languageToString(cpp.Language)
	compiler := compilerToString(cpp.Compiler)
	version := versionToString(cpp.CompilerVersion)
	crt := cRuntimeToString(cpp.CRuntime)
	crtVersion := versionToString(cpp.CRuntimeVersion)
	arch := architectureToString(cpp.Architecture)

	// Build ID: prefix-lang-compiler-version-cruntime-cruntime_version[-stdlib]-arch
	id := fmt.Sprintf("%s-%s-%s-%s-%s-%s", prefix, lang, compiler, version, crt, crtVersion)

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
