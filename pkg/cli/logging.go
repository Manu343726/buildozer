package cli

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/Manu343726/buildozer/pkg/config"
	"github.com/Manu343726/buildozer/pkg/logging"
)

// SlogLevel is a type alias for slog.Level used in function signatures
type SlogLevel = slog.Level

// LoggingCommands provides command-level implementations for logging CLI operations.
// Each method corresponds to a CLI subcommand and handles the complete command logic.
// The CLI driver passes configuration and flags; LoggingCommands figures out the rest.
type LoggingCommands struct {
	*logging.Logger // Embedded logger for hierarchical logging

	configManager logging.ConfigManager
}

// NewLoggingCommands creates a new LoggingCommands handler.
// It uses the global logging config manager which is initialized in root command.
// The global manager is either LocalConfigManager (standalone) or RemoteConfigManager (network).
func NewLoggingCommands(cfg *config.Config) (*LoggingCommands, error) {
	manager := logging.NewRemoteConfigManager(cfg.Daemon.Host, cfg.Daemon.Port)

	if manager == nil {
		return nil, fmt.Errorf("global logging config manager not initialized")
	}

	return &LoggingCommands{
		Logger:        Log().Child("LoggingCommands"),
		configManager: manager,
	}, nil
}

// Status shows the current logging configuration status
func (lc *LoggingCommands) Status() error {
	ctx := context.Background()
	status, err := lc.configManager.GetLoggingStatus(ctx)
	if err != nil {
		return lc.Errorf("failed to get logging status: %w", err)
	}

	fmt.Println("global level:", status.GlobalLevel.String())

	fmt.Println("sinks:")
	for _, sink := range status.Sinks {
		fmt.Printf("  - %s:\n", sink.Name)
		fmt.Printf("      type: %s\n", sink.Type)
		fmt.Printf("      level: %s\n", sink.Level.String())
		fmt.Printf("      include_source_location: %v\n", sink.IncludeSourceLocation)

		// Display file-specific configuration
		if sink.Type == "file" && sink.Path != "" {
			fmt.Printf("      path: %s\n", sink.Path)
			fmt.Printf("      json_format: %v\n", sink.JSONFormat)
			if sink.MaxSize > 0 {
				fmt.Printf("      max_size_mb: %d\n", sink.MaxSize/(1024*1024))
			}
			if sink.MaxBackups > 0 {
				fmt.Printf("      max_backups: %d\n", sink.MaxBackups)
			}
			if sink.MaxAgeDays > 0 {
				fmt.Printf("      max_age_days: %d\n", sink.MaxAgeDays)
			}
		}
	}

	fmt.Println("loggers:")
	for _, logger := range status.Loggers {
		fmt.Printf("  - %s:\n", logger.Name)
		fmt.Printf("      level: %s\n", logger.Level.String())
		fmt.Printf("      sinks:\n")
		for _, sinkName := range logger.SinkNames {
			fmt.Printf("        - %s\n", sinkName)
		}
	}

	fmt.Println("active loggers:")
	if len(status.ActiveLoggers) == 0 {
		fmt.Println("  (none)")
	} else {
		for _, activeLogger := range status.ActiveLoggers {
			fmt.Printf("  - %s:\n", activeLogger.Name)
			if activeLogger.ResolvedConfigFor == "" {
				fmt.Printf("      resolves to: (none)\n")
			} else {
				fmt.Printf("      resolves to: %s\n", activeLogger.ResolvedConfigFor)
			}
		}
	}

	return nil
}

// Tail tails logs from the daemon with optional filtering
func (lc *LoggingCommands) Tail(logLevels []slog.Level, loggerFilter string, historyLines int) error {
	ctx := context.Background()
	logChan, err := lc.configManager.TailLogs(ctx, logLevels, loggerFilter, historyLines)
	if err != nil {
		return lc.Errorf("failed to tail logs: %w", err)
	}

	// Stream logs to stdout
	for record := range logChan {
		fmt.Printf("[%s] [%s] [%s] %s\n", record.Timestamp.Format("15:04:05"), record.LoggerName, record.Level.String(), record.Message)
	}
	return nil
}

// ParseLogLevels parses a comma-separated string of log levels
func ParseLogLevels(levelStr string) []slog.Level {
	if levelStr == "" {
		return nil
	}

	parts := strings.Split(levelStr, ",")
	var levels []slog.Level
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if level, err := stringToSlogLevel(part); err == nil {
			levels = append(levels, level)
		}
	}
	return levels
}

// SetGlobalLevel sets the global logging level
func (lc *LoggingCommands) SetGlobalLevel(levelStr string) error {
	level, err := stringToSlogLevel(levelStr)
	if err != nil {
		return err
	}
	ctx := context.Background()
	if err := lc.configManager.SetGlobalLevel(ctx, level); err != nil {
		return err
	}
	lc.Info("Set global logging level", "level", levelStr)
	return nil
}

// SetLoggerLevel sets a specific logger's level
func (lc *LoggingCommands) SetLoggerLevel(loggerName, levelStr string) error {
	level, err := stringToSlogLevel(levelStr)
	if err != nil {
		return err
	}
	ctx := context.Background()
	if err := lc.configManager.SetLoggerLevel(ctx, loggerName, level); err != nil {
		return err
	}
	lc.Info("Set logger level", "logger", loggerName, "level", levelStr)
	return nil
}

// SetSinkLevel sets a specific sink's level
func (lc *LoggingCommands) SetSinkLevel(sinkName, levelStr string) error {
	level, err := stringToSlogLevel(levelStr)
	if err != nil {
		return err
	}
	ctx := context.Background()
	if err := lc.configManager.SetSinkLevel(ctx, sinkName, level); err != nil {
		return err
	}
	lc.Info("Set sink level", "sink", sinkName, "level", levelStr)
	return nil
}

// EnableFileSink enables a logger-specific file sink
func (lc *LoggingCommands) EnableFileSink(loggerName, filePath string, maxSizeMB, maxBackups, maxAgeDays int) error {
	ctx := context.Background()
	if err := lc.configManager.EnableFileSink(ctx, loggerName, filePath, maxSizeMB, maxBackups, maxAgeDays); err != nil {
		return err
	}
	lc.Info("Enabled file sink for logger", "logger", loggerName, "path", filePath, "max_size_mb", maxSizeMB, "max_backups", maxBackups, "max_age_days", maxAgeDays)
	return nil
}

// DisableFileSink disables a logger-specific file sink
func (lc *LoggingCommands) DisableFileSink(loggerName string) error {
	ctx := context.Background()
	if err := lc.configManager.DisableFileSink(ctx, loggerName); err != nil {
		return err
	}
	lc.Info("Disabled file sink for logger", "logger", loggerName)
	return nil
}

// AddSink adds a new sink
func (lc *LoggingCommands) AddSink(sinkName, sinkType string, level slog.Level) error {
	ctx := context.Background()
	if err := lc.configManager.AddSink(ctx, sinkName, sinkType, level); err != nil {
		return err
	}
	lc.Info("Added sink", "sink", sinkName, "type", sinkType, "level", level.String())
	return nil
}

// RemoveSink removes a sink
func (lc *LoggingCommands) RemoveSink(sinkName string) error {
	ctx := context.Background()
	if err := lc.configManager.RemoveSink(ctx, sinkName); err != nil {
		return err
	}
	lc.Info("Removed sink", "sink", sinkName)
	return nil
}

// AddLogger adds a new logger with specified sinks
func (lc *LoggingCommands) AddLogger(loggerName string, level slog.Level, sinkNames []string) error {
	ctx := context.Background()
	if err := lc.configManager.AddLogger(ctx, loggerName, level, sinkNames); err != nil {
		return err
	}
	lc.Info("Added logger", "logger", loggerName, "level", level.String(), "sinks", sinkNames)
	return nil
}

// RemoveLogger removes a logger
func (lc *LoggingCommands) RemoveLogger(loggerName string) error {
	ctx := context.Background()
	if err := lc.configManager.RemoveLogger(ctx, loggerName); err != nil {
		return err
	}
	lc.Info("Removed logger", "logger", loggerName)
	return nil
}

// AttachSink attaches a sink to an existing logger
func (lc *LoggingCommands) AttachSink(loggerName, sinkName string) error {
	ctx := context.Background()
	if err := lc.configManager.AttachSink(ctx, loggerName, sinkName); err != nil {
		return err
	}
	lc.Info("Attached sink to logger", "logger", loggerName, "sink", sinkName)
	return nil
}

// DetachSink removes a sink from a logger
func (lc *LoggingCommands) DetachSink(loggerName, sinkName string) error {
	ctx := context.Background()
	if err := lc.configManager.DetachSink(ctx, loggerName, sinkName); err != nil {
		return err
	}
	lc.Info("Detached sink from logger", "logger", loggerName, "sink", sinkName)
	return nil
}

// stringToSlogLevel converts a string log level to slog.Level
func stringToSlogLevel(levelStr string) (slog.Level, error) {
	switch levelStr {
	case "error", "ERROR":
		return slog.LevelError, nil
	case "warn", "warning", "WARN", "WARNING":
		return slog.LevelWarn, nil
	case "info", "INFO":
		return slog.LevelInfo, nil
	case "debug", "DEBUG":
		return slog.LevelDebug, nil
	case "trace", "TRACE":
		return slog.Level(-8), nil // Trace is below Debug
	default:
		return slog.LevelInfo, fmt.Errorf("invalid log level: %s", levelStr)
	}
}
