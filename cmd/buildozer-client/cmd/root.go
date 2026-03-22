package cmd

import (
	"fmt"
	"os"
	"strings"

	pkgconfig "github.com/Manu343726/buildozer/pkg/config"
	"github.com/Manu343726/buildozer/pkg/logging"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// NewRootCommand creates and returns the root command with config manager integration
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
			return initializeConfig(cmd)
		},
	}

	// Global flags
	root.PersistentFlags().String("settings", "", "custom settings file path (default: ~/.config/buildozer/buildozer.yaml)")
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

	return root
}

// initializeConfig sets up configuration from all sources using the config manager
// and initializes logging with the appropriate config for this command.
func initializeConfig(cmd *cobra.Command) error {
	// Get the settings file path from the --settings flag
	settingsFile, _ := cmd.Flags().GetString("settings")

	// Initialize the global config manager with the settings file
	configMgr := pkgconfig.ConfigManager()
	if err := configMgr.Initialize(settingsFile); err != nil {
		return err
	}

	// Define which flags map to which config values
	// The CLI owns this decision about what flags override what config keys
	flagMappings := map[string]string{
		"log-level":  "logging.global_level",
		"host":       "daemon.host",
		"port":       "daemon.port",
		"standalone": "standalone",
	}

	// Bind the selected flags to viper (highest priority)
	if err := configMgr.BindFlags(cmd.Flags(), flagMappings); err != nil {
		return err
	}

	// Get the effective configuration
	cfg := pkgconfig.Get()

	// Determine if we're running the daemon command by checking os.Args
	// (Look for "daemon" in the command line, skipping flag names and values)
	isDaemonCommand := isDaemonBeingRun()

	// Initialize logging with appropriate config for this command
	var loggingConfig logging.LoggingConfig
	if isDaemonCommand {
		// Daemon uses its own logging config (may have file sinks, etc)
		loggingConfig = cfg.Daemon.Logging
	} else {
		// Client commands use the general logging config
		loggingConfig = cfg.Logging
	}

	if err := logging.InitializeGlobal(loggingConfig); err != nil {
		return err
	}

	return nil
}

// isDaemonBeingRun checks if the "daemon" subcommand is being executed
// by looking for "daemon" in the command line arguments
func isDaemonBeingRun() bool {
	for _, arg := range os.Args[1:] {
		// Skip flag names and their values
		if strings.HasPrefix(arg, "-") {
			continue
		}
		// First non-flag argument should be the subcommand
		return arg == "daemon"
	}
	return false
}

// PrintConfigSummary prints the effective configuration (useful for debugging)
func PrintConfigSummary() {
	cfg := pkgconfig.Get()
	fmt.Println("Effective Configuration:")
	fmt.Println("======================")
	fmt.Printf("Daemon:\n")
	fmt.Printf("  Host: %s\n", cfg.Daemon.Host)
	fmt.Printf("  Port: %d\n", cfg.Daemon.Port)
	fmt.Printf("  Max Concurrent Jobs: %d\n", cfg.Daemon.MaxConcurrentJobs)
	fmt.Printf("  Max RAM: %d MB\n", cfg.Daemon.MaxRAMMB)
	fmt.Printf("  Enable mDNS: %v\n", cfg.Daemon.EnableMDNS)
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

// IsStandaloneMode returns true if --standalone flag is set globally
func IsStandaloneMode(cmd *cobra.Command) (bool, error) {
	root := cmd
	for root.Parent() != nil {
		root = root.Parent()
	}
	return root.PersistentFlags().GetBool("standalone")
}
