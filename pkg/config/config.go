package config

import (
	"github.com/Manu343726/buildozer/pkg/daemon"
	"github.com/Manu343726/buildozer/pkg/logging"
)

// CacheConfig holds cache-related configuration
type CacheConfig struct {
	Dir           string `json:"dir" yaml:"dir"`                       // Cache directory path
	MaxSizeGB     int    `json:"max_size_gb" yaml:"max_size_gb"`       // Maximum cache size in GB
	RetentionDays int    `json:"retention_days" yaml:"retention_days"` // Cache retention period in days
}

// DefaultCacheConfig returns the default cache configuration
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		Dir:           "", // Will use home directory default in manager
		MaxSizeGB:     100,
		RetentionDays: 30,
	}
}

// PeerDiscoveryConfig holds peer discovery configuration
type PeerDiscoveryConfig struct {
	Enabled          bool `json:"enabled" yaml:"enabled"`                             // Enable peer discovery
	MDNSIntervalSecs int  `json:"mDNS_interval_seconds" yaml:"mDNS_interval_seconds"` // mDNS discovery interval in seconds
}

// DefaultPeerDiscoveryConfig returns the default peer discovery configuration
func DefaultPeerDiscoveryConfig() PeerDiscoveryConfig {
	return PeerDiscoveryConfig{
		Enabled:          true,
		MDNSIntervalSecs: 30,
	}
}

// CppDriverConfig holds C/C++ compiler driver configuration
type CppDriverConfig struct {
	// CompilerVersion specifies the preferred compiler version (e.g., "9", "10", "11", "gcc-9")
	CompilerVersion string `json:"compiler_version" yaml:"compiler_version"`

	// CompilerType specifies the preferred compiler type if multiple are available (e.g., "gcc", "clang")
	CompilerType string `json:"compiler_type" yaml:"compiler_type"`

	// CRuntime specifies the C runtime to use (e.g., "glibc", "musl")
	CRuntime string `json:"c_runtime" yaml:"c_runtime"`

	// CppStdLib specifies the C++ standard library (e.g., "libstdc++", "libc++")
	CppStdLib string `json:"cpp_stdlib" yaml:"cpp_stdlib"`

	// Architecture specifies the target architecture (e.g., "x86_64", "aarch64", "armv7")
	Architecture string `json:"architecture" yaml:"architecture"`
}

// DriversConfig holds driver-specific configuration
type DriversConfig struct {
	// Gcc configuration for gcc driver
	Gcc CppDriverConfig `json:"gcc" yaml:"gcc"`

	// Gxx configuration for g++ driver
	Gxx CppDriverConfig `json:"g++" yaml:"g++"`

	// Make configuration for make driver (future)
	// Make *MakeDriverConfig `json:"make" yaml:"make"`
}

// DefaultDriversConfig returns default driver configuration
func DefaultDriversConfig() DriversConfig {
	return DriversConfig{
		Gcc: CppDriverConfig{},
		Gxx: CppDriverConfig{},
	}
}

// Config holds the effective configuration from all sources (flags, env vars, config file)
// It composes config structs from different packages
type Config struct {
	Standalone    bool                  `json:"standalone" yaml:"standalone"`
	Daemon        daemon.DaemonConfig   `json:"daemon" yaml:"daemon"`
	Logging       logging.LoggingConfig `json:"logging" yaml:"logging"`
	Cache         CacheConfig           `json:"cache" yaml:"cache"`
	PeerDiscovery PeerDiscoveryConfig   `json:"peer_discovery" yaml:"peer_discovery"`
	Drivers       DriversConfig         `json:"drivers" yaml:"drivers"`
}

// DefaultConfig returns the default configuration with all defaults applied from each package
func DefaultConfig() Config {
	return Config{
		Standalone:    false,
		Daemon:        daemon.DefaultConfig(),
		Logging:       logging.DefaultLoggingConfig(),
		Cache:         DefaultCacheConfig(),
		PeerDiscovery: DefaultPeerDiscoveryConfig(),
		Drivers:       DefaultDriversConfig(),
	}
}

// Copy returns a shallow copy of the Config struct
func (c *Config) Copy() *Config {
	if c == nil {
		return nil
	}
	cfg := *c
	return &cfg
}
