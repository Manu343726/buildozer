package gcc

import "fmt"

// Config holds configuration for the gcc driver
type Config struct {
	CompilerVersion string // e.g., "9", "10", "11", "gcc-9"
	CRuntime        string // e.g., "glibc", "musl"
	CRuntimeVersion string // e.g., "2.31", "1.2.3"
	Architecture    string // e.g., "x86_64", "aarch64", "armv7"
}

// InitConfig initializes a GccConfig from a generic configuration map.
// Returns error if the config map has invalid field types.
// All fields are optional.
func InitConfig(cfgMap map[string]interface{}) (*Config, error) {
	cfg := &Config{}

	if cfgMap == nil {
		return cfg, nil
	}

	if v, ok := cfgMap["compiler_version"].(string); ok {
		cfg.CompilerVersion = v
	} else if cfgMap["compiler_version"] != nil {
		return nil, fmt.Errorf("gcc config: compiler_version must be a string, got %T", cfgMap["compiler_version"])
	}

	if v, ok := cfgMap["c_runtime"].(string); ok {
		cfg.CRuntime = v
	} else if cfgMap["c_runtime"] != nil {
		return nil, fmt.Errorf("gcc config: c_runtime must be a string, got %T", cfgMap["c_runtime"])
	}

	if v, ok := cfgMap["c_runtime_version"].(string); ok {
		cfg.CRuntimeVersion = v
	} else if cfgMap["c_runtime_version"] != nil {
		return nil, fmt.Errorf("gcc config: c_runtime_version must be a string, got %T", cfgMap["c_runtime_version"])
	}

	if v, ok := cfgMap["architecture"].(string); ok {
		cfg.Architecture = v
	} else if cfgMap["architecture"] != nil {
		return nil, fmt.Errorf("gcc config: architecture must be a string, got %T", cfgMap["architecture"])
	}

	return cfg, nil
}
