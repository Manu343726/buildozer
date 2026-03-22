package cli

import (
	"fmt"

	pkgconfig "github.com/Manu343726/buildozer/pkg/config"
	"github.com/Manu343726/buildozer/pkg/logging"
)

// ConfigCommands provides command-level implementations for config CLI operations.
type ConfigCommands struct {
	*logging.Logger // Embedded logger for hierarchical logging

	configMgr *pkgconfig.Manager
}

// NewConfigCommands creates a new ConfigCommands handler.
func NewConfigCommands(configMgr *pkgconfig.Manager) *ConfigCommands {
	return &ConfigCommands{
		Logger:    Log().Child("ConfigCommands"),
		configMgr: configMgr,
	}
}

// ShowConfig logs and displays the effective configuration.
func (cc *ConfigCommands) ShowConfig() {
	cfg := cc.configMgr.Get()

	output := "Effective Configuration:\n"
	output += "======================\n"
	output += "Daemon:\n"
	output += fmt.Sprintf("  Host: %s\n", cfg.Daemon.Host)
	output += fmt.Sprintf("  Port: %d\n", cfg.Daemon.Port)
	output += fmt.Sprintf("  Max Concurrent Jobs: %d\n", cfg.Daemon.MaxConcurrentJobs)
	output += fmt.Sprintf("  Max RAM: %d MB\n", cfg.Daemon.MaxRAMMB)
	output += fmt.Sprintf("  Enable mDNS: %v\n", cfg.Daemon.EnableMDNS)

	output += "\nLogging:\n"
	output += fmt.Sprintf("  Global Level: %s\n", cfg.Logging.GlobalLevel)
	output += fmt.Sprintf("  Sinks: %d\n", len(cfg.Logging.Sinks))
	for _, sink := range cfg.Logging.Sinks {
		output += fmt.Sprintf("    - %s (%s, level: %s)\n", sink.Name, sink.Type, sink.Level)
	}
	output += fmt.Sprintf("  Loggers: %d\n", len(cfg.Logging.Loggers))
	for _, logger := range cfg.Logging.Loggers {
		output += fmt.Sprintf("    - %s (level: %s, sinks: %v)\n", logger.Name, logger.Level, logger.Sinks)
	}

	output += "\nCache:\n"
	output += fmt.Sprintf("  Directory: %s\n", cfg.Cache.Dir)
	output += fmt.Sprintf("  Max Size: %d GB\n", cfg.Cache.MaxSizeGB)
	output += fmt.Sprintf("  Retention: %d days\n", cfg.Cache.RetentionDays)

	output += "\nPeer Discovery:\n"
	output += fmt.Sprintf("  Enabled: %v\n", cfg.PeerDiscovery.Enabled)
	output += fmt.Sprintf("  mDNS Interval: %d seconds\n", cfg.PeerDiscovery.MDNSIntervalSecs)

	cc.Info(output)
}
