package gcc_common

import (
	"fmt"

	"github.com/Manu343726/buildozer/pkg/runtimes/cpp/native"
)

// GccConfig represents configuration for GCC (C compiler)
type GccConfig struct {
	CompilerVersion string
	CRuntime        native.CRuntime
	CRuntimeVersion string
	Architecture    native.Architecture
}

// GxxConfig represents configuration for G++ (C++ compiler)
type GxxConfig struct {
	CompilerVersion string
	CRuntime        native.CRuntime
	CRuntimeVersion string
	Architecture    native.Architecture
	CppStdlib       native.CppStdlib
}

// ClangConfig represents configuration for Clang (C compiler)
type ClangConfig struct {
	CompilerVersion string
	CRuntime        native.CRuntime
	CRuntimeVersion string
	Architecture    native.Architecture
}

// ClangxxConfig represents configuration for Clang++ (C++ compiler)
type ClangxxConfig struct {
	CompilerVersion string
	CRuntime        native.CRuntime
	CRuntimeVersion string
	Architecture    native.Architecture
	CppStdlib       native.CppStdlib
}

// ConfigFromMap converts a configuration map to a typed config struct.
// The configType parameter should be one of: "gcc", "g++", "clang", "clang++"
// For "ar", use ar_common.ConfigFromMap instead.
// Returns error if required fields are missing or invalid.
func ConfigFromMap(configType string, cfgMap map[string]interface{}) (interface{}, error) {
	if cfgMap == nil {
		return nil, fmt.Errorf("config map is nil")
	}

	// Common field extraction
	cRuntime, err := extractCRuntime(cfgMap)
	if err != nil {
		return nil, err
	}

	cRuntimeVersion, ok := cfgMap["c_runtime_version"].(string)
	if !ok || cRuntimeVersion == "" {
		return nil, fmt.Errorf("required config field 'c_runtime_version' is missing or not a string")
	}

	architecture, err := extractArchitecture(cfgMap)
	if err != nil {
		return nil, err
	}

	switch configType {
	case "gcc":
		compilerVersion, ok := cfgMap["compiler_version"].(string)
		if !ok || compilerVersion == "" {
			return nil, fmt.Errorf("required config field 'compiler_version' is missing or not a string for gcc")
		}
		return GccConfig{
			CompilerVersion: compilerVersion,
			CRuntime:        cRuntime,
			CRuntimeVersion: cRuntimeVersion,
			Architecture:    architecture,
		}, nil

	case "g++":
		compilerVersion, ok := cfgMap["compiler_version"].(string)
		if !ok || compilerVersion == "" {
			return nil, fmt.Errorf("required config field 'compiler_version' is missing or not a string for g++")
		}
		cppStdlib, err := extractCppStdlib(cfgMap)
		if err != nil {
			return nil, err
		}
		return GxxConfig{
			CompilerVersion: compilerVersion,
			CRuntime:        cRuntime,
			CRuntimeVersion: cRuntimeVersion,
			Architecture:    architecture,
			CppStdlib:       cppStdlib,
		}, nil

	case "clang":
		compilerVersion, ok := cfgMap["compiler_version"].(string)
		if !ok || compilerVersion == "" {
			return nil, fmt.Errorf("required config field 'compiler_version' is missing or not a string for clang")
		}
		return ClangConfig{
			CompilerVersion: compilerVersion,
			CRuntime:        cRuntime,
			CRuntimeVersion: cRuntimeVersion,
			Architecture:    architecture,
		}, nil

	case "clang++":
		compilerVersion, ok := cfgMap["compiler_version"].(string)
		if !ok || compilerVersion == "" {
			return nil, fmt.Errorf("required config field 'compiler_version' is missing or not a string for clang++")
		}
		cppStdlib, err := extractCppStdlib(cfgMap)
		if err != nil {
			return nil, err
		}
		return ClangxxConfig{
			CompilerVersion: compilerVersion,
			CRuntime:        cRuntime,
			CRuntimeVersion: cRuntimeVersion,
			Architecture:    architecture,
			CppStdlib:       cppStdlib,
		}, nil

	default:
		return nil, fmt.Errorf("unknown config type: %s", configType)
	}
}

// extractCRuntime extracts and validates the c_runtime field from a config map
func extractCRuntime(cfgMap map[string]interface{}) (native.CRuntime, error) {
	cRuntimeStr, ok := cfgMap["c_runtime"].(string)
	if !ok || cRuntimeStr == "" {
		return native.CRuntimeUnspecified, fmt.Errorf("required config field 'c_runtime' is missing or not a string")
	}

	switch cRuntimeStr {
	case "glibc":
		return native.CRuntimeGlibc, nil
	case "musl":
		return native.CRuntimeMusl, nil
	default:
		return native.CRuntimeUnspecified, fmt.Errorf("invalid c_runtime value: %q (valid values: glibc, musl)", cRuntimeStr)
	}
}

// extractArchitecture extracts and validates the architecture field from a config map
func extractArchitecture(cfgMap map[string]interface{}) (native.Architecture, error) {
	archStr, ok := cfgMap["architecture"].(string)
	if !ok || archStr == "" {
		return native.ArchitectureUnspecified, fmt.Errorf("required config field 'architecture' is missing or not a string")
	}

	switch archStr {
	case "x86_64":
		return native.ArchitectureX86_64, nil
	case "aarch64":
		return native.ArchitectureAArch64, nil
	case "arm":
		return native.ArchitectureARM, nil
	default:
		return native.ArchitectureUnspecified, fmt.Errorf("invalid architecture value: %q (valid values: x86_64, aarch64, arm)", archStr)
	}
}

// extractCppStdlib extracts and validates the cpp_stdlib field from a config map
func extractCppStdlib(cfgMap map[string]interface{}) (native.CppStdlib, error) {
	cppStdlibStr, ok := cfgMap["cpp_stdlib"].(string)
	if !ok || cppStdlibStr == "" {
		return native.CppStdlibUnspecified, fmt.Errorf("required config field 'cpp_stdlib' is missing or not a string for C++ compiler")
	}

	switch cppStdlibStr {
	case "libstdc++":
		return native.CppStdlibLibstdcxx, nil
	case "libc++":
		return native.CppStdlibLibcxx, nil
	default:
		return native.CppStdlibUnspecified, fmt.Errorf("invalid cpp_stdlib value: %q (valid values: libstdc++, libc++)", cppStdlibStr)
	}
}
