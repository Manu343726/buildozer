package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Manu343726/buildozer/pkg/logging"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Config holds the effective configuration from all sources (flags, env vars, config file)
type Config struct {
	Daemon struct {
		Host              string
		Port              int
		Listen            string
		MaxConcurrentJobs int
		MaxRAMMB          int
		EnableMPOS        bool
	}
	Logging logging.LoggingConfig
	Cache   struct {
		Dir           string
		MaxSizeGB     int
		RetentionDays int
	}
	PeerDiscovery struct {
		Enabled          bool
		MDNSIntervalSecs int
	}
}

// NewRootCommand creates and returns the root command with viper integration
func NewRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "buildozer-client",
		Short: "Buildozer P2P distributed build system client daemon and CLI",
		Long: `buildozer-client is the main entrypoint for the Buildozer distributed build system.

Usage:
  Start daemon:     buildozer-client daemon
  Interactive mode: buildozer-client status           (requires daemon running)
  Standalone mode:  buildozer-client --standalone status (daemon in-process)

Configuration Priority (highest to lowest):
1. CLI flags (e.g., --port 6789)
2. Environment variables (e.g., BUILDOZER_DAEMON_PORT=6789)
3. Configuration file (e.g., ~/.config/buildozer/config.yaml)
4. Hardcoded defaults

Global flag --standalone enables in-process daemon mode for interactive commands,
avoiding the need to run a separate daemon process. Not compatible with the daemon
subcommand or --host/--port flags.`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return initializeViper(cmd)
		},
	}

	// Global flags
	root.PersistentFlags().String("config", "", "config file path (default: ~/.config/buildozer/config.yaml)")
	root.PersistentFlags().Bool("debug", false, "enable debug logging")
	root.PersistentFlags().String("log-level", "info", "logging level: error/warn/info/debug/trace")
	root.PersistentFlags().Bool("standalone", false, "run in standalone mode (in-process daemon for interactive commands)")
	root.PersistentFlags().String("host", "localhost", "daemon host address (not used with --standalone)")
	root.PersistentFlags().Int("port", 6789, "daemon gRPC port (not used with --standalone)")

	// Add subcommands
	daemonCmd := NewDaemonCommand()
	root.AddCommand(daemonCmd)
	root.AddCommand(NewStatusCommand())
	root.AddCommand(NewPeersCommand())
	root.AddCommand(NewLogsCommand())
	root.AddCommand(NewCacheCommand())
	root.AddCommand(NewQueueCommand())
	root.AddCommand(NewConfigCommand())
	root.AddCommand(NewCancelCommand())

	// Validate that daemon command can't be used with --standalone
	root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if err := initializeViper(cmd); err != nil {
			return err
		}
		// Check if --standalone is set and user is trying to run 'daemon' command
		standalone, _ := cmd.Flags().GetBool("standalone")
		if standalone && cmd.Name() != "help" && cmd.Name() != "completion" {
			// Get the actual subcommand being run
			if len(args) > 0 && args[0] == "daemon" {
				return fmt.Errorf("cannot use 'daemon' subcommand with --standalone flag. --standalone enables in-process daemon for interactive commands")
			}
		}
		return nil
	}

	return root
}

// initializeViper sets up viper configuration management with priority order:
// CLI flags > Environment variables > Config file > Defaults
func initializeViper(cmd *cobra.Command) error {
	// Set env prefix
	viper.SetEnvPrefix("BUILDOZER")

	// Use GetEnv to read env variables
	// This allows nested keys like "daemon.port" to map to "BUILDOZER_DAEMON_PORT"
	viper.BindEnv("daemon.host", "BUILDOZER_DAEMON_HOST")
	viper.BindEnv("daemon.port", "BUILDOZER_DAEMON_PORT")
	viper.BindEnv("daemon.listen", "BUILDOZER_DAEMON_LISTEN")
	viper.BindEnv("daemon.max_concurrent_jobs", "BUILDOZER_DAEMON_MAX_CONCURRENT_JOBS")
	viper.BindEnv("daemon.max_ram_mb", "BUILDOZER_DAEMON_MAX_RAM_MB")
	viper.BindEnv("daemon.enable_mpos", "BUILDOZER_DAEMON_ENABLE_MPOS")

	viper.BindEnv("logging.level", "BUILDOZER_LOGGING_LEVEL")
	viper.BindEnv("logging.format", "BUILDOZER_LOGGING_FORMAT")

	viper.BindEnv("cache.dir", "BUILDOZER_CACHE_DIR")
	viper.BindEnv("cache.max_size_gb", "BUILDOZER_CACHE_MAX_SIZE_GB")
	viper.BindEnv("cache.retention_days", "BUILDOZER_CACHE_RETENTION_DAYS")

	viper.BindEnv("peer_discovery.enabled", "BUILDOZER_PEER_DISCOVERY_ENABLED")
	viper.BindEnv("peer_discovery.mDNS_interval_seconds", "BUILDOZER_PEER_DISCOVERY_MDNS_INTERVAL_SECONDS")

	// Set config file paths
	viper.SetConfigName("buildozer")
	viper.SetConfigType("yaml")

	// Config file search paths
	configPath, _ := cmd.Flags().GetString("config")
	if configPath != "" {
		// Explicit config path from --config flag
		viper.SetConfigFile(configPath)
	} else {
		// Standard locations
		viper.AddConfigPath(filepath.Join(os.Getenv("HOME"), ".config", "buildozer"))
		viper.AddConfigPath("/etc/buildozer")
		viper.AddConfigPath(".")
	}

	// Read config file (non-fatal if missing)
	if err := viper.ReadInConfig(); err != nil {
		// Config file is optional
		_ = err
	}

	// Bind CLI flags to viper - these are HIGHEST priority
	// This MUST happen AFTER reading config file so flags override everything
	viper.BindPFlag("debug", cmd.Flags().Lookup("debug"))
	viper.BindPFlag("logging.level", cmd.Flags().Lookup("log-level"))
	viper.BindPFlag("daemon.host", cmd.Flags().Lookup("host"))
	viper.BindPFlag("daemon.port", cmd.Flags().Lookup("port"))

	return nil
}

// GetConfig returns the effective configuration from viper
// using defaults only when values are not found in any source
func GetConfig() *Config {
	cfg := &Config{}

	// Daemon - get from viper with inline defaults
	cfg.Daemon.Host = getStringOrDefault("daemon.host", "localhost")
	cfg.Daemon.Port = getIntOrDefault("daemon.port", 6789)
	cfg.Daemon.Listen = getStringOrDefault("daemon.listen", "0.0.0.0")
	cfg.Daemon.MaxConcurrentJobs = getIntOrDefault("daemon.max_concurrent_jobs", 4)
	cfg.Daemon.MaxRAMMB = getIntOrDefault("daemon.max_ram_mb", 8192)
	cfg.Daemon.EnableMPOS = getBoolOrDefault("daemon.enable_mpos", true)

	// Logging - get from viper, or use defaults
	if viper.IsSet("logging") {
		if err := viper.UnmarshalKey("logging", &cfg.Logging); err != nil {
			// Fall back to default if unmarshal fails
			cfg.Logging = logging.DefaultLoggingConfig()
		}
	} else {
		cfg.Logging = logging.DefaultLoggingConfig()
	}

	// Cache
	cacheDir := getStringOrDefault("cache.dir", filepath.Join(os.Getenv("HOME"), ".cache", "buildozer"))
	cfg.Cache.Dir = cacheDir
	cfg.Cache.MaxSizeGB = getIntOrDefault("cache.max_size_gb", 100)
	cfg.Cache.RetentionDays = getIntOrDefault("cache.retention_days", 30)

	// Peer Discovery
	cfg.PeerDiscovery.Enabled = getBoolOrDefault("peer_discovery.enabled", true)
	cfg.PeerDiscovery.MDNSIntervalSecs = getIntOrDefault("peer_discovery.mDNS_interval_seconds", 30)

	return cfg
}

// Helper functions to get values with fallback defaults
func getStringOrDefault(key, defaultVal string) string {
	if viper.IsSet(key) {
		return viper.GetString(key)
	}
	return defaultVal
}

func getIntOrDefault(key string, defaultVal int) int {
	if viper.IsSet(key) {
		return viper.GetInt(key)
	}
	return defaultVal
}

func getBoolOrDefault(key string, defaultVal bool) bool {
	if viper.IsSet(key) {
		return viper.GetBool(key)
	}
	return defaultVal
}

// PrintConfigSummary prints the effective configuration (useful for debugging)
func PrintConfigSummary() {
	cfg := GetConfig()
	fmt.Println("Effective Configuration:")
	fmt.Println("======================")
	fmt.Printf("Daemon:\n")
	fmt.Printf("  Host: %s\n", cfg.Daemon.Host)
	fmt.Printf("  Port: %d\n", cfg.Daemon.Port)
	fmt.Printf("  Listen: %s\n", cfg.Daemon.Listen)
	fmt.Printf("  Max Concurrent Jobs: %d\n", cfg.Daemon.MaxConcurrentJobs)
	fmt.Printf("  Max RAM: %d MB\n", cfg.Daemon.MaxRAMMB)
	fmt.Printf("  Enable MPOS: %v\n", cfg.Daemon.EnableMPOS)
	fmt.Printf("\nLogging:\n")
	fmt.Printf("  Global Level: %s\n", cfg.Logging.GlobalLevel)
	fmt.Printf("  Sinks: %d\n", len(cfg.Logging.Sinks))
	for _, sink := range cfg.Logging.Sinks {
		fmt.Printf("    - %s (%s, level: %s)\n", sink.Name, sink.Type, sink.Level)
	}
	fmt.Printf("  Loggers: %d\n", len(cfg.Logging.Loggers))
	for _, logger := range cfg.Logging.Loggers {
		fmt.Printf("    - %s (level: %s, sinks: %v)\n", logger.Name, logger.Level, logger.Sinks)
	}
	fmt.Printf("\nCache:\n")
	fmt.Printf("  Directory: %s\n", cfg.Cache.Dir)
	fmt.Printf("  Max Size: %d GB\n", cfg.Cache.MaxSizeGB)
	fmt.Printf("  Retention: %d days\n", cfg.Cache.RetentionDays)
	fmt.Printf("\nPeer Discovery:\n")
	fmt.Printf("  Enabled: %v\n", cfg.PeerDiscovery.Enabled)
	fmt.Printf("  mDNS Interval: %d seconds\n", cfg.PeerDiscovery.MDNSIntervalSecs)

	// Show config sources
	if viper.ConfigFileUsed() != "" {
		fmt.Printf("\nConfig file used: %s\n", viper.ConfigFileUsed())
	}
}

// InitializeLogging initializes the logging system from the current configuration
func InitializeLogging() error {
	cfg := GetConfig()
	return logging.InitializeGlobal(cfg.Logging)
}

// IsStandaloneMode returns true if --standalone flag is set globally
func IsStandaloneMode(cmd *cobra.Command) (bool, error) {
	root := cmd
	for root.Parent() != nil {
		root = root.Parent()
	}
	return root.PersistentFlags().GetBool("standalone")
}
