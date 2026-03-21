package logging

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
)

// ConfigManager is the interface for managing logging configuration.
// It provides a unified API for both local and remote logging management.
type ConfigManager interface {
	// GetLoggingStatus returns the current logging configuration including all sinks and loggers
	GetLoggingStatus(ctx context.Context) (*LoggingStatusSnapshot, error)

	// SetGlobalLevel changes the global logging level for all loggers and sinks
	SetGlobalLevel(ctx context.Context, level slog.Level) error

	// SetLoggerLevel changes the logging level for a specific logger
	SetLoggerLevel(ctx context.Context, loggerName string, level slog.Level) error

	// SetSinkLevel changes the logging level for a specific sink
	SetSinkLevel(ctx context.Context, sinkName string, level slog.Level) error

	// EnableFileSink enables a rotating file sink for a logger
	EnableFileSink(ctx context.Context, loggerName, filePath string, maxSizeMB int, maxBackups int, maxAgeDays int) error

	// DisableFileSink disables a file-based sink for a logger
	DisableFileSink(ctx context.Context, loggerName string) error

	// TailLogs streams log records in real-time with optional filtering
	TailLogs(ctx context.Context, logLevels []slog.Level, loggerFilter string, historyLines int) (<-chan *LogRecord, error)
}

// LoggingStatusSnapshot represents a snapshot of the current logging configuration
type LoggingStatusSnapshot struct {
	GlobalLevel slog.Level
	Sinks       []SinkStatus
	Loggers     []LoggerStatus
	RetrievedAt time.Time
}

// SinkStatus represents the status of a sink
type SinkStatus struct {
	Name       string
	Type       string
	Level      slog.Level
	Path       string // Only for file sinks
	JSONFormat bool   // Only for file sinks
}

// LoggerStatus represents the status of a logger
type LoggerStatus struct {
	Name      string
	SinkNames []string
	Level     slog.Level
}

// LogRecord represents a single log record
type LogRecord struct {
	Timestamp  time.Time
	LoggerName string
	Level      slog.Level
	Message    string
	Attributes map[string]string
}

// LocalConfigManager implements ConfigManager using the local logging package functions
type LocalConfigManager struct {
	registry *Registry
	factory  *Factory
}

// NewLocalConfigManager creates a new local configuration manager
func NewLocalConfigManager(registry *Registry, factory *Factory) *LocalConfigManager {
	return &LocalConfigManager{
		registry: registry,
		factory:  factory,
	}
}

// GetLoggingStatus returns the current logging configuration
func (m *LocalConfigManager) GetLoggingStatus(ctx context.Context) (*LoggingStatusSnapshot, error) {
	snapshot := &LoggingStatusSnapshot{
		GlobalLevel: m.registry.GetGlobalLevel(),
		RetrievedAt: time.Now(),
		Sinks:       []SinkStatus{},
		Loggers:     []LoggerStatus{},
	}

	// Get all sinks
	m.registry.mu.RLock()
	for _, sink := range m.registry.sinks {
		sinkStatus := SinkStatus{
			Name:  sink.Name,
			Type:  sink.Type,
			Level: sink.Level,
		}
		snapshot.Sinks = append(snapshot.Sinks, sinkStatus)
	}

	// Get all loggers from configuration
	for name, config := range m.registry.loggerConfigs {
		loggerStatus := LoggerStatus{
			Name:      name,
			SinkNames: config.sinkNames,
			Level:     config.level,
		}
		snapshot.Loggers = append(snapshot.Loggers, loggerStatus)
	}
	m.registry.mu.RUnlock()

	return snapshot, nil
}

// SetGlobalLevel changes the global logging level
func (m *LocalConfigManager) SetGlobalLevel(ctx context.Context, level slog.Level) error {
	m.registry.SetGlobalLevel(level)
	return nil
}

// SetLoggerLevel changes the level for a specific logger
func (m *LocalConfigManager) SetLoggerLevel(ctx context.Context, loggerName string, level slog.Level) error {
	return m.registry.SetLoggerLevel(loggerName, level)
}

// SetSinkLevel changes the level for a specific sink
func (m *LocalConfigManager) SetSinkLevel(ctx context.Context, sinkName string, level slog.Level) error {
	return m.registry.SetSinkLevel(sinkName, level)
}

// EnableFileSink enables a file sink for a logger
func (m *LocalConfigManager) EnableFileSink(ctx context.Context, loggerName, filePath string, maxSizeMB int, maxBackups int, maxAgeDays int) error {
	return EnableLoggerFileSink(loggerName, filePath, maxSizeMB, maxAgeDays)
}

// DisableFileSink disables a file sink for a logger
func (m *LocalConfigManager) DisableFileSink(ctx context.Context, loggerName string) error {
	return DisableLoggerFileSink(loggerName)
}

// TailLogs is not implemented for local manager (would require buffering)
func (m *LocalConfigManager) TailLogs(ctx context.Context, logLevels []slog.Level, loggerFilter string, historyLines int) (<-chan *LogRecord, error) {
	return nil, fmt.Errorf("TailLogs not supported for local configuration manager")
}

// ProtoLogLevelToSlogLevel converts protobuf LogLevel to slog.Level
func ProtoLogLevelToSlogLevel(protoLevel v1.LogLevel) slog.Level {
	switch protoLevel {
	case v1.LogLevel_LOG_LEVEL_ERROR:
		return slog.LevelError
	case v1.LogLevel_LOG_LEVEL_WARN:
		return slog.LevelWarn
	case v1.LogLevel_LOG_LEVEL_INFO:
		return slog.LevelInfo
	case v1.LogLevel_LOG_LEVEL_DEBUG:
		return slog.LevelDebug
	case v1.LogLevel_LOG_LEVEL_TRACE:
		return slog.Level(-8) // Trace is 2 below Debug
	default:
		return slog.LevelInfo
	}
}

// SlogLevelToProtoLogLevel converts slog.Level to protobuf LogLevel
func SlogLevelToProtoLogLevel(level slog.Level) v1.LogLevel {
	switch level {
	case slog.LevelError:
		return v1.LogLevel_LOG_LEVEL_ERROR
	case slog.LevelWarn:
		return v1.LogLevel_LOG_LEVEL_WARN
	case slog.LevelInfo:
		return v1.LogLevel_LOG_LEVEL_INFO
	case slog.LevelDebug:
		return v1.LogLevel_LOG_LEVEL_DEBUG
	default:
		if level < slog.LevelDebug {
			return v1.LogLevel_LOG_LEVEL_TRACE
		}
		return v1.LogLevel_LOG_LEVEL_INFO
	}
}
