package logging

import (
	"fmt"
	"log/slog"

	"github.com/Manu343726/buildozer/pkg/logging/sinks"
)

// SinkConfig holds configuration for a single sink
type SinkConfig struct {
	Name  string `json:"name" yaml:"name"`
	Type  string `json:"type" yaml:"type"`   // stdout, stderr, file, syslog
	Level string `json:"level" yaml:"level"` // Level as string for YAML config

	// File-specific configuration
	Path       string `json:"path,omitempty" yaml:"path,omitempty"`
	MaxSizeB   int64  `json:"max_size_b,omitempty" yaml:"max_size_b,omitempty"`     // Max file size
	MaxFiles   int    `json:"max_files,omitempty" yaml:"max_files,omitempty"`       // Max rotated files
	MaxAgeDays int    `json:"max_age_days,omitempty" yaml:"max_age_days,omitempty"` // Max age in days
	JSONFormat bool   `json:"json_format,omitempty" yaml:"json_format,omitempty"`

	// Syslog-specific configuration
	Tag string `json:"tag,omitempty" yaml:"tag,omitempty"`
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
	Sinks       []SinkConfig   `json:"sinks" yaml:"sinks"`
	Loggers     []LoggerConfig `json:"loggers" yaml:"loggers"`
}

// DefaultLoggingConfig returns a default logging configuration
func DefaultLoggingConfig() LoggingConfig {
	return LoggingConfig{
		GlobalLevel: "info",
		Sinks: []SinkConfig{
			{
				Name:  "stdout",
				Type:  "stdout",
				Level: "info",
			},
			{
				Name:  "stderr",
				Type:  "stderr",
				Level: "error",
			},
		},
		Loggers: []LoggerConfig{
			{
				Name:  "buildozer",
				Level: "info",
				Sinks: []string{"stdout", "stderr"},
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
		Level: level,
	}

	switch config.Type {
	case "stdout":
		handler = sinks.StdoutSink(handlerOpts)

	case "stderr":
		handler = sinks.StderrSink(handlerOpts)

	case "file":
		if config.Path == "" {
			return nil, fmt.Errorf("file sink %q requires 'path' configuration", config.Name)
		}

		handler, err = sinks.FileSink(sinks.FileSinkConfig{
			Path:        config.Path,
			MaxSizeB:    config.MaxSizeB,
			MaxFiles:    config.MaxFiles,
			MaxAgeDays:  config.MaxAgeDays,
			JSONFormat:  config.JSONFormat,
			HandlerOpts: handlerOpts,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create file sink %q: %w", config.Name, err)
		}

	default:
		return nil, fmt.Errorf("unknown sink type: %q", config.Type)
	}

	sink := &Sink{
		Name:    config.Name,
		Type:    config.Type,
		Level:   level,
		Handler: handler,
	}

	if err := f.registry.RegisterSink(sink); err != nil {
		return nil, err
	}

	return sink, nil
}

// InitializeFromConfig initializes the registry from a LoggingConfig
func (f *Factory) InitializeFromConfig(config LoggingConfig) error {
	// Set global level
	globalLevel := ParseLevel(config.GlobalLevel)
	f.registry.SetGlobalLevel(globalLevel)

	// Create all sinks
	for _, sinkCfg := range config.Sinks {
		if _, err := f.CreateSink(sinkCfg); err != nil {
			return fmt.Errorf("failed to create sink %q: %w", sinkCfg.Name, err)
		}
	}

	// Configure loggers with their sinks
	for _, loggerCfg := range config.Loggers {
		// Validate that all sinks exist
		for _, sinkName := range loggerCfg.Sinks {
			if _, exists := f.registry.GetSink(sinkName); !exists {
				return fmt.Errorf("sink %q referenced by logger %q not found", sinkName, loggerCfg.Name)
			}
		}

		// Configure which sinks are used by this logger
		if err := f.registry.SetLoggerSinks(loggerCfg.Name, loggerCfg.Sinks); err != nil {
			return fmt.Errorf("failed to configure logger %q: %w", loggerCfg.Name, err)
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
