package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Global manager instance (singleton)
var (
	globalManager *Manager
	managerMu     sync.Mutex
)

// ConfigManager returns the global config manager
func ConfigManager() *Manager {
	managerMu.Lock()
	defer managerMu.Unlock()

	if globalManager == nil {
		globalManager = NewManager()
	}

	return globalManager
}

// Manager handles configuration initialization and retrieval
type Manager struct {
	initialized bool
}

// NewManager creates a new configuration manager
func NewManager() *Manager {
	return &Manager{
		initialized: false,
	}
}

// Initialize sets up viper with proper priority order:
// CLI flags > Environment variables > Config file > Defaults
//
// configFile is optional. If empty, standard locations are searched.
func (m *Manager) Initialize(configFile string) error {
	// Set env prefix
	viper.SetEnvPrefix("BUILDOZER")

	// Use reflection to automatically bind all config struct fields to viper
	// This creates environment variable bindings for all fields
	cfg := &Config{}
	if err := bindStructToViper("", reflect.ValueOf(cfg), reflect.TypeOf(cfg)); err != nil {
		return err
	}

	// Set config file paths
	viper.SetConfigName("buildozer")
	viper.SetConfigType("yaml")

	if configFile != "" {
		// Explicit config path
		viper.SetConfigFile(configFile)
	} else {
		// Standard locations
		viper.AddConfigPath(filepath.Join(os.Getenv("HOME"), ".config", "buildozer"))
		viper.AddConfigPath("/etc/buildozer")
		viper.AddConfigPath(".")
	}

	// Read config file (non-fatal if missing)
	_ = viper.ReadInConfig()

	m.initialized = true

	// Store this manager as the global instance
	managerMu.Lock()
	globalManager = m
	managerMu.Unlock()

	return nil
}

// BindFlags binds CLI flags from a cobra command to viper using the provided mappings.
// This must be called after Initialize() to ensure CLI flags have the highest priority.
// The caller provides the flag-to-viper-key mappings, allowing flexibility in what gets bound.
// Priority order remains: CLI flags > env vars > config file > defaults
//
// Example:
//
//	mappings := map[string]string{
//	    "log-level": "logging.level",
//	    "port":      "daemon.port",
//	}
//	manager.BindFlags(cmd.Flags(), mappings)
func (m *Manager) BindFlags(flagSet *pflag.FlagSet, flagMappings map[string]string) error {
	// Bind each flag to its corresponding viper key
	for flagName, viperKey := range flagMappings {
		// Get the flag from the set
		flag := flagSet.Lookup(flagName)
		if flag == nil {
			continue
		}

		// Bind the flag to viper
		if err := viper.BindPFlag(viperKey, flag); err != nil {
			return fmt.Errorf("failed to bind flag %s to viper key %s: %v", flagName, viperKey, err)
		}
	}

	return nil
}

// Get returns the effective configuration after merging all sources.
// Priority order:
// 1. CLI flags (via viper bindings)
// 2. Environment variables (via viper bindings from Initialize)
// 3. Config file values (loaded by Initialize)
// 4. Default values from DefaultConfig() function
func (m *Manager) Get() *Config {
	if !m.initialized {
		m.Initialize("")
	}

	// Start with all defaults from package DefaultConfig() which composes
	// defaults from all sub-packages (daemon, logging, cache, peer discovery)
	cfg := DefaultConfig()

	// Override with any values explicitly set in viper (config file, env vars, CLI flags)
	populateStructFromViper("", reflect.ValueOf(&cfg), reflect.TypeOf(&cfg))

	// Special handling for cache directory - use home-based default if still empty
	if cfg.Cache.Dir == "" {
		cfg.Cache.Dir = filepath.Join(os.Getenv("HOME"), ".cache", "buildozer")
	}

	return &cfg
}

// Get is a convenience function that returns the configuration from the global manager
func Get() *Config {
	return ConfigManager().Get()
}
