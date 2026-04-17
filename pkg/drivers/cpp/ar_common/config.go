package ar_common

import (
	"fmt"

	"github.com/Manu343726/buildozer/pkg/runtimes/cpp/native"
)

// ArConfig represents configuration for AR (static library archiver).
// AR is ABI-agnostic and only cares about the C runtime and architecture,
// so it doesn't have compiler_version or cpp_stdlib fields.
type ArConfig struct {
	CRuntime        native.CRuntime
	CRuntimeVersion string
	Architecture    native.Architecture
}

// ConfigFromMap converts a configuration map to an ArConfig struct.
// Returns error if required fields are missing or invalid.
func ConfigFromMap(cfgMap map[string]interface{}) (*ArConfig, error) {
	if cfgMap == nil {
		return nil, fmt.Errorf("config map is nil")
	}

	// Extract c_runtime
	cRuntime, err := extractCRuntime(cfgMap)
	if err != nil {
		return nil, err
	}

	// Extract c_runtime_version
	cRuntimeVersion, ok := cfgMap["c_runtime_version"].(string)
	if !ok || cRuntimeVersion == "" {
		return nil, fmt.Errorf("required config field 'c_runtime_version' is missing or not a string")
	}

	// Extract architecture
	architecture, err := extractArchitecture(cfgMap)
	if err != nil {
		return nil, err
	}

	return &ArConfig{
		CRuntime:        cRuntime,
		CRuntimeVersion: cRuntimeVersion,
		Architecture:    architecture,
	}, nil
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
