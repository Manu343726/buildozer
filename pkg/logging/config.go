package logging

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/Manu343726/buildozer/pkg/logging/sinks"
)

// ExpandHome expands the ~ prefix in paths to the user's home directory
// Examples:
//   - "~" → "/home/user"
//   - "~/.cache/logs" → "/home/user/.cache/logs"
//   - "/var/log" → "/var/log" (unchanged)
func ExpandHome(path string) string {
	if path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return home
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

// SinkConfig holds configuration for a single sink
type SinkConfig struct {
	Name  string `json:"name" yaml:"name"`
	Type  string `json:"type" yaml:"type"`   // stdout, stderr, file, syslog
	Level string `json:"level" yaml:"level"` // Level as string for YAML config

	// File-specific configuration
	Filename   string `json:"filename,omitempty" yaml:"filename,omitempty"`         // Filename (relative to logging dir)
	MaxSizeB   int64  `json:"max_size_b,omitempty" yaml:"max_size_b,omitempty"`     // Max file size
	MaxFiles   int    `json:"max_files,omitempty" yaml:"max_files,omitempty"`       // Max rotated files
	MaxAgeDays int    `json:"max_age_days,omitempty" yaml:"max_age_days,omitempty"` // Max age in days
	JSONFormat bool   `json:"json_format,omitempty" yaml:"json_format,omitempty"`

	// Syslog-specific configuration
	Tag string `json:"tag,omitempty" yaml:"tag,omitempty"`

	// Include source location (file:line) in log output (default: false)
	IncludeSourceLocation bool `json:"include_source_location,omitempty" yaml:"include_source_location,omitempty"`

	// Omit logger name if source location is enabled (default: true)
	// Only applies when IncludeSourceLocation is true
	OmitLoggerNameIfSourceEnabled bool `json:"omit_logger_name_if_source_enabled,omitempty" yaml:"omit_logger_name_if_source_enabled,omitempty"`
}

// LoggerConfig holds configuration for a single logger
type LoggerConfig struct {
	Name  string   `json:"name" yaml:"name"`
	Level string   `json:"level" yaml:"level"` // Level as string for YAML config
	Sinks []string `json:"sinks" yaml:"sinks"` // List of sink names to attach
}

// LoggingConfig holds the overall logging configuration
type LoggingConfig struct {
	GlobalLevel string         `json:"global_level" yaml:"global_level"` // Level as string
	LoggingDir  string         `json:"logging_dir" yaml:"logging_dir"`   // Base directory for file sinks
	Sinks       []SinkConfig   `json:"sinks" yaml:"sinks"`
	Loggers     []LoggerConfig `json:"loggers" yaml:"loggers"`
}

// DefaultLoggingConfig returns the default logging configuration for CLI client
// The buildozer logger captures all messages at debug level.
// The stdout sink filters based on CLI flag, while file sink is always debug.
func DefaultLoggingConfig() LoggingConfig {
	return LoggingConfig{
		GlobalLevel: "debug",
		LoggingDir:  "~/.cache/buildozer/logs", // Default to user cache directory
		Sinks: []SinkConfig{
			{
				Name:                          "stdout",
				Type:                          "stdout",
				Level:                         "warn", // Default to warn, changed by CLI flag
				IncludeSourceLocation:         true,   // Include source location by default
				OmitLoggerNameIfSourceEnabled: true,   // Omit logger name by default when source is enabled
			},
		},
		Loggers: []LoggerConfig{
			{
				Name:  "buildozer",
				Level: "debug", // Logger captures all debug messages
				Sinks: []string{"stdout"},
			},
		},
	}
}

// Factory creates slog.Handler instances from sink configurations
type Factory struct {
	registry *Registry
}

// NewFactory creates a new handler factory
func NewFactory(registry *Registry) *Factory {
	return &Factory{
		registry: registry,
	}
}

// ParseLevel converts a string to slog.Level
func ParseLevel(levelStr string) slog.Level {
	switch levelStr {
	case "error":
		return slog.LevelError
	case "warn":
		return slog.LevelWarn
	case "info":
		return slog.LevelInfo
	case "debug":
		return slog.LevelDebug
	case "trace":
		return slog.Level(-8) // Trace is 2 below Debug
	default:
		return slog.LevelInfo
	}
}

// LevelToString converts slog.Level to string
func LevelToString(level slog.Level) string {
	switch level {
	case slog.LevelError:
		return "error"
	case slog.LevelWarn:
		return "warn"
	case slog.LevelInfo:
		return "info"
	case slog.LevelDebug:
		return "debug"
	default:
		if level < slog.LevelDebug {
			return "trace"
		}
		return "info"
	}
}

// CreateSink creates a sink from configuration and registers it
func (f *Factory) CreateSink(config SinkConfig) (*Sink, error) {
	var handler slog.Handler
	var err error

	level := ParseLevel(config.Level)
	handlerOpts := &slog.HandlerOptions{
		Level:     level,
		AddSource: config.IncludeSourceLocation,
	}

	switch config.Type {
	case "stdout":
		handler = sinks.StdoutSinkWithOmitLogger(handlerOpts, config.OmitLoggerNameIfSourceEnabled)

	case "stderr":
		handler = sinks.StderrSinkWithOmitLogger(handlerOpts, config.OmitLoggerNameIfSourceEnabled)

	case "file":
		if config.Filename == "" {
			return nil, fmt.Errorf("file sink %q requires 'filename' configuration", config.Name)
		}

		// Construct full path using logging directory
		fullPath := filepath.Join(f.registry.loggingDir, config.Filename)

		handler, err = sinks.FileSink(sinks.FileSinkConfig{
			Path:                          fullPath,
			MaxSizeB:                      config.MaxSizeB,
			MaxFiles:                      config.MaxFiles,
			MaxAgeDays:                    config.MaxAgeDays,
			JSONFormat:                    config.JSONFormat,
			IncludeSourceLocation:         config.IncludeSourceLocation,
			OmitLoggerNameIfSourceEnabled: config.OmitLoggerNameIfSourceEnabled,
			HandlerOpts:                   handlerOpts,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create file sink %q: %w", config.Name, err)
		}

	default:
		return nil, fmt.Errorf("unknown sink type: %q", config.Type)
	}

	sink := &Sink{
		Name:             config.Name,
		Type:             config.Type,
		Level:            level,
		Handler:          handler,
		FilePath:         config.Filename,
		MaxSize:          config.MaxSizeB,
		MaxBackups:       int32(config.MaxFiles),
		MaxAgeDays:       int32(config.MaxAgeDays),
		JSONFormat:       config.JSONFormat,
		IncludeSourceLoc: config.IncludeSourceLocation,
	}

	if err := f.registry.RegisterSink(sink); err != nil {
		return nil, err
	}

	return sink, nil
}

// InitializeFromConfig initializes the registry from a LoggingConfig
// The logging directory from the config is used as the base path for all file sinks
func (f *Factory) InitializeFromConfig(config LoggingConfig) error {
	// Set global level
	globalLevel := ParseLevel(config.GlobalLevel)
	f.registry.SetGlobalLevel(globalLevel)

	// Expand and set logging directory
	loggingDir := ExpandHome(config.LoggingDir)
	f.registry.loggingDir = loggingDir

	// Create all sinks
	for _, sinkCfg := range config.Sinks {
		if _, err := f.CreateSink(sinkCfg); err != nil {
			return fmt.Errorf("failed to create sink %q: %w", sinkCfg.Name, err)
		}
	}

	// Configure loggers with their sinks AND levels
	for _, loggerCfg := range config.Loggers {
		// Validate that all sinks exist
		for _, sinkName := range loggerCfg.Sinks {
			if _, exists := f.registry.GetSink(sinkName); !exists {
				return fmt.Errorf("sink %q referenced by logger %q not found", sinkName, loggerCfg.Name)
			}
		}

		// Configure which sinks are used by this logger (this creates the logger config entry)
		if err := f.registry.SetLoggerSinks(loggerCfg.Name, loggerCfg.Sinks); err != nil {
			return fmt.Errorf("failed to configure logger %q: %w", loggerCfg.Name, err)
		}

		// Now set the logger level from config (after the logger config entry exists)
		level := ParseLevel(loggerCfg.Level)
		if err := f.registry.SetLoggerLevel(loggerCfg.Name, level); err != nil {
			return fmt.Errorf("failed to set logger %q level: %w", loggerCfg.Name, err)
		}
	}

	return nil
}

// GetLoggerStatus returns the current configuration status of all loggers
func (r *Registry) GetLoggerStatus() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	status := make(map[string]interface{})

	for name, sinkNames := range r.loggerConfigs {
		status[name] = map[string]interface{}{
			"sinks": sinkNames,
		}
	}

	return status
}

// GetSinkStatus returns the current configuration status of all sinks
func (r *Registry) GetSinkStatus() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	status := make(map[string]interface{})

	for name, sink := range r.sinks {
		status[name] = map[string]interface{}{
			"type":  sink.Type,
			"level": LevelToString(sink.Level),
		}
	}

	return status
}
