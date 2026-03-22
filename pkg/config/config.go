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

// Config holds the effective configuration from all sources (flags, env vars, config file)
// It composes config structs from different packages
type Config struct {
	Standalone    bool                  `json:"standalone" yaml:"standalone"`
	Daemon        daemon.DaemonConfig   `json:"daemon" yaml:"daemon"`
	Logging       logging.LoggingConfig `json:"logging" yaml:"logging"`
	Cache         CacheConfig           `json:"cache" yaml:"cache"`
	PeerDiscovery PeerDiscoveryConfig   `json:"peer_discovery" yaml:"peer_discovery"`
}

// DefaultConfig returns the default configuration with all defaults applied from each package
func DefaultConfig() Config {
	return Config{
		Standalone:    false,
		Daemon:        daemon.DefaultConfig(),
		Logging:       logging.DefaultLoggingConfig(),
		Cache:         DefaultCacheConfig(),
		PeerDiscovery: DefaultPeerDiscoveryConfig(),
	}
}
