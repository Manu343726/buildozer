package logging

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	"github.com/Manu343726/buildozer/pkg/logging/sinks"
)

// SinkConfigChange represents changes to sink configuration.
// All fields are optional pointers - only non-nil fields will be updated.
type SinkConfigChange struct {
	// Logging level (nil = no change)
	Level *slog.Level

	// Include source location in logs (nil = no change)
	IncludeSourceLocation *bool

	// Maximum file size in bytes (nil or 0 = no change, only for file sinks)
	MaxSizeBytes *int64

	// Maximum age of log files in days (nil or 0 = no change, only for file sinks)
	MaxAgeDays *int32
}

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

	// UpdateSinkConfig updates configuration for an existing sink with optional changes.
	// Uses SinkConfigChange with pointer fields - only non-nil fields are updated.
	UpdateSinkConfig(ctx context.Context, sinkName string, changes SinkConfigChange) error

	// EnableFileSink enables a rotating file sink for a logger
	// The filePath parameter is a filename relative to the logging directory
	EnableFileSink(ctx context.Context, loggerName, filePath string, maxSizeMB int, maxBackups int, maxAgeDays int) error

	// DisableFileSink disables a file-based sink for a logger
	DisableFileSink(ctx context.Context, loggerName string) error

	// AddSink creates and registers a new stdout/stderr sink
	AddSink(ctx context.Context, sinkName, sinkType string, level slog.Level) error

	// RemoveSink removes a sink from the registry and all loggers
	RemoveSink(ctx context.Context, sinkName string) error

	// AddLogger creates a new logger with specified sinks and level
	AddLogger(ctx context.Context, loggerName string, level slog.Level, sinkNames []string) error

	// RemoveLogger removes a logger configuration from the registry
	RemoveLogger(ctx context.Context, loggerName string) error

	// AttachSink attaches an existing sink to a logger
	AttachSink(ctx context.Context, loggerName, sinkName string) error

	// DetachSink removes a sink from a logger
	DetachSink(ctx context.Context, loggerName, sinkName string) error

	// TailLogs streams log records in real-time with optional filtering
	TailLogs(ctx context.Context, logLevels []slog.Level, loggerFilter string, historyLines int) (<-chan *LogRecord, error)
}

// LoggingStatusSnapshot represents a snapshot of the current logging configuration
type LoggingStatusSnapshot struct {
	GlobalLevel   slog.Level
	Sinks         []SinkStatus
	Loggers       []LoggerStatus
	ActiveLoggers []ActiveLoggerInfo // List of all active loggers with their resolved config
	RetrievedAt   time.Time
}

// ActiveLoggerInfo represents an active logger and the configured logger it resolves to
type ActiveLoggerInfo struct {
	Name              string // The active logger name (e.g., "app.db.postgres")
	ResolvedConfigFor string // The configured logger this resolves to via hierarchical lookup
}

// SinkStatus represents the status of a sink
type SinkStatus struct {
	Name                  string
	Type                  string
	Level                 slog.Level
	Path                  string // Only for file sinks
	JSONFormat            bool   // Only for file sinks
	MaxSize               int64  // Only for file sinks - in bytes
	MaxBackups            int32  // Only for file sinks
	MaxAgeDays            int32  // Only for file sinks
	IncludeSourceLocation bool   // Include source location in logs
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
	*Logger
	registry *Registry
}

// NewLocalConfigManager creates a new local configuration manager
func NewLocalConfigManager(registry *Registry) *LocalConfigManager {
	return &LocalConfigManager{
		Logger:   Log().Child("LocalConfigManager"),
		registry: registry,
	}
}

// GetLoggingStatus returns the current logging configuration
func (m *LocalConfigManager) GetLoggingStatus(ctx context.Context) (*LoggingStatusSnapshot, error) {
	m.Debug("retrieving logging status")

	snapshot := &LoggingStatusSnapshot{
		GlobalLevel:   m.registry.GetGlobalLevel(),
		RetrievedAt:   time.Now(),
		Sinks:         []SinkStatus{},
		Loggers:       []LoggerStatus{},
		ActiveLoggers: []ActiveLoggerInfo{},
	}

	// Get all sinks
	m.registry.mu.RLock()
	for _, sink := range m.registry.sinks {
		sinkStatus := SinkStatus{
			Name:                  sink.Name,
			Type:                  sink.Type,
			Level:                 sink.Level,
			Path:                  sink.FilePath,
			MaxSize:               sink.MaxSize,
			MaxBackups:            sink.MaxBackups,
			MaxAgeDays:            sink.MaxAgeDays,
			JSONFormat:            sink.JSONFormat,
			IncludeSourceLocation: sink.IncludeSourceLoc,
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

	// Get active loggers with their resolved configurations
	for _, resolution := range m.registry.GetActiveLoggersWithResolution() {
		snapshot.ActiveLoggers = append(snapshot.ActiveLoggers, ActiveLoggerInfo{
			Name:              resolution.Name,
			ResolvedConfigFor: resolution.ResolvedConfigFor,
		})
	}

	m.Debug("logging status retrieved", "sinks", len(snapshot.Sinks), "loggers", len(snapshot.Loggers), "active_loggers", len(snapshot.ActiveLoggers), "global_level", snapshot.GlobalLevel.String())
	return snapshot, nil
}

// SetGlobalLevel changes the global logging level
func (m *LocalConfigManager) SetGlobalLevel(ctx context.Context, level slog.Level) error {
	m.Debug("setting global logging level", "level", level.String())
	m.registry.SetGlobalLevel(level)
	m.Debug("global logging level changed", "level", level.String())
	return nil
}

// SetLoggerLevel changes the level for a specific logger
func (m *LocalConfigManager) SetLoggerLevel(ctx context.Context, loggerName string, level slog.Level) error {
	m.Debug("setting logger level", "logger", loggerName, "level", level.String())
	err := m.registry.SetLoggerLevel(loggerName, level)
	if err != nil {
		return m.Errorf("failed to set logger level: %w", err)
	}
	m.Debug("logger level changed", "logger", loggerName, "level", level.String())
	return nil
}

// SetSinkLevel changes the level for a specific sink
func (m *LocalConfigManager) SetSinkLevel(ctx context.Context, sinkName string, level slog.Level) error {
	m.Debug("setting sink level", "sink", sinkName, "level", level.String())
	err := m.registry.SetSinkLevel(sinkName, level)
	if err != nil {
		return m.Errorf("failed to set sink level: %w", err)
	}
	m.Debug("sink level changed", "sink", sinkName, "level", level.String())
	return nil
}

// UpdateSinkConfig updates configuration for an existing sink
// All fields except sinkName are optional (pass nil/0 to skip updating)
func (m *LocalConfigManager) UpdateSinkConfig(ctx context.Context, sinkName string, changes SinkConfigChange) error {
	m.Debug("updating sink config", "sink", sinkName, "changes", changes)

	sink, exists := m.registry.GetSink(sinkName)
	if !exists {
		return m.Errorf("sink %q not found", sinkName)
	}

	if changes.Level != nil {
		if err := m.registry.SetSinkLevel(sinkName, *changes.Level); err != nil {
			return m.Errorf("failed to update level: %w", err)
		}
		m.Debug("updated sink level", "sink", sinkName, "level", changes.Level.String())
	}

	if changes.IncludeSourceLocation != nil {
		m.registry.mu.Lock()
		sink.IncludeSourceLoc = *changes.IncludeSourceLocation
		m.registry.mu.Unlock()
		m.Debug("updated sink include_source_location", "sink", sinkName, "include_source_location", *changes.IncludeSourceLocation)
	}

	// Update file-specific configuration if provided (only for file sinks)
	if changes.MaxSizeBytes != nil || changes.MaxAgeDays != nil {
		if sink.Type != "file" {
			return m.Errorf("sink %q is not a file sink; cannot update file-specific configuration", sinkName)
		}

		m.registry.mu.Lock()
		if changes.MaxSizeBytes != nil && *changes.MaxSizeBytes > 0 {
			sink.MaxSize = *changes.MaxSizeBytes
			m.Debug("updated sink max_size", "sink", sinkName, "max_size_bytes", sink.MaxSize)
		}

		if changes.MaxAgeDays != nil && *changes.MaxAgeDays > 0 {
			sink.MaxAgeDays = *changes.MaxAgeDays
			m.Debug("updated sink max_age_days", "sink", sinkName, "max_age_days", sink.MaxAgeDays)
		}
		m.registry.mu.Unlock()
	}

	return nil
}

// EnableFileSink is a shorthand for creating a file sink and attaching it to a logger
// Creates a sink named "file-{loggerName}" and attaches only to that logger
// If the logger already has a file sink attached, it removes the old one first
func (m *LocalConfigManager) EnableFileSink(ctx context.Context, loggerName, filePath string, maxSizeMB int, maxBackups int, maxAgeDays int) error {
	m.Debug("enabling file sink", "logger", loggerName, "path", filePath, "max_size_mb", maxSizeMB, "max_age_days", maxAgeDays)

	sinkName := "file-" + loggerName
	defaultLevel := slog.LevelDebug

	// Check if logger already has a file sink attached and detach it
	if sinkNames, exists := m.registry.GetLoggerSinks(loggerName); exists {
		for _, name := range sinkNames {
			if sink, exists := m.registry.GetSink(name); exists && sink.Type == "file" {
				// Detach the old file sink
				if err := m.registry.DetachSink(loggerName, name); err != nil {
					m.Error("failed to detach old file sink", "logger", loggerName, "sink", name)
				}
				// Remove the old sink from registry
				if err := m.registry.RemoveSink(name); err != nil {
					m.Error("failed to remove old file sink", "sink", name)
				}
				m.Debug("detached and removed old file sink", "logger", loggerName, "sink", name)
			}
		}
	}

	// Set defaults
	if maxSizeMB == 0 {
		maxSizeMB = 100 // Default to 100MB
	}
	if maxBackups == 0 {
		maxBackups = 10 // Default to 10 backups
	}

	// Create the new file sink using the internal method with file path
	if err := m.addSinkInternal(ctx, sinkName, "file", filePath, defaultLevel); err != nil {
		return m.Errorf("failed to add file sink: %w", err)
	}

	// Attach the new sink to the logger
	if err := m.registry.AttachSink(loggerName, sinkName); err != nil {
		// Clean up the sink we just created
		_ = m.registry.RemoveSink(sinkName)
		return m.Errorf("failed to attach sink to logger: %w", err)
	}

	m.Debug("file sink enabled", "logger", loggerName, "path", filePath, "sink", sinkName)
	return nil
}

// DisableFileSink is a shorthand for removing a file sink from a logger
// Detaches and removes the file sink associated with the logger
func (m *LocalConfigManager) DisableFileSink(ctx context.Context, loggerName string) error {
	m.Debug("disabling file sink", "logger", loggerName)

	// Find all file sinks attached to this logger and remove them
	sinkNames, exists := m.registry.GetLoggerSinks(loggerName)
	if !exists {
		return m.Errorf("logger %q not found", loggerName)
	}

	var foundFileSink string
	for _, name := range sinkNames {
		if sink, exists := m.registry.GetSink(name); exists && sink.Type == "file" {
			foundFileSink = name
			break
		}
	}

	if foundFileSink == "" {
		return m.Errorf("no file sink attached to logger %q", loggerName)
	}

	// Detach the sink from the logger
	if err := m.registry.DetachSink(loggerName, foundFileSink); err != nil {
		return m.Errorf("failed to detach sink: %w", err)
	}

	// Remove the sink from the registry
	if err := m.registry.RemoveSink(foundFileSink); err != nil {
		return m.Errorf("failed to remove sink: %w", err)
	}

	m.Debug("file sink disabled", "logger", loggerName, "sink", foundFileSink)
	return nil
}

// AddSink creates and registers a new sink (file, stdout, or stderr)
// Returns error if there's an overlap with an existing sink:
// - New stdout and existing stdout already exists
// - New stderr and existing stderr already exists
// - New file and existing file pointing to same path
func (m *LocalConfigManager) AddSink(ctx context.Context, sinkName, sinkType string, level slog.Level) error {
	return m.addSinkInternal(ctx, sinkName, sinkType, "", level)
}

// addSinkInternal is the internal implementation that handles file path for file sinks
// For file sinks, filePath is a filename relative to the logging directory
func (m *LocalConfigManager) addSinkInternal(ctx context.Context, sinkName, sinkType, filePath string, level slog.Level) error {
	m.Debug("adding sink", "sink", sinkName, "type", sinkType, "level", level.String())

	// Validate sink type
	switch sinkType {
	case "stdout", "stderr", "file":
		// Valid types
	default:
		return m.Errorf("unsupported sink type: %s (must be 'stdout', 'stderr', or 'file')", sinkType)
	}

	// Check for overlaps with existing sinks
	for _, sink := range m.registry.GetAllSinks() {
		if sinkType == "stdout" && sink.Type == "stdout" {
			return m.Errorf("a stdout sink already exists; cannot create another (overlap with %q)", sink.Name)
		}
		if sinkType == "stderr" && sink.Type == "stderr" {
			return m.Errorf("a stderr sink already exists; cannot create another (overlap with %q)", sink.Name)
		}
		if sinkType == "file" && sink.Type == "file" && sink.FilePath == filePath {
			return m.Errorf("a file sink for %q already exists (overlap with %q); cannot create another", filePath, sink.Name)
		}
	}

	var handler slog.Handler
	switch sinkType {
	case "stdout":
		handler = slog.NewTextHandler(os.Stdout, nil)
	case "stderr":
		handler = slog.NewTextHandler(os.Stderr, nil)
	case "file":
		if filePath == "" {
			return m.Errorf("file sink requires file path")
		}
		// Construct full path using logging directory
		fullPath := filepath.Join(m.registry.loggingDir, filePath)
		var err error
		handler, err = sinks.FileSink(sinks.FileSinkConfig{
			Path:                          fullPath,
			MaxSizeB:                      100 * 1024 * 1024, // Default to 100MB
			MaxFiles:                      10,                // Default to 10 backups
			MaxAgeDays:                    0,                 // No age-based rotation by default
			JSONFormat:                    false,
			OmitLoggerNameIfSourceEnabled: true, // Omit logger name by default when source is enabled
			HandlerOpts: &slog.HandlerOptions{
				Level: level,
			},
		})
		if err != nil {
			return m.Errorf("failed to create file sink: %w", err)
		}
	}

	sink := &Sink{
		Name:     sinkName,
		Type:     sinkType,
		FilePath: filePath,
		Level:    level,
		Handler:  handler,
	}

	if err := m.registry.RegisterSink(sink); err != nil {
		return m.Errorf("failed to register sink: %w", err)
	}

	m.Debug("sink added", "sink", sinkName, "type", sinkType)
	return nil
}

// RemoveSink removes a sink from the registry and all loggers
func (m *LocalConfigManager) RemoveSink(ctx context.Context, sinkName string) error {
	m.Debug("removing sink", "sink", sinkName)
	err := m.registry.RemoveSink(sinkName)
	if err != nil {
		return m.Errorf("failed to remove sink: %w", err)
	}
	m.Debug("sink removed", "sink", sinkName)
	return nil
}

// AddLogger creates a new logger with specified sinks and level
// If no sinks are specified, automatically attaches default sinks (stdout, buildozer-daemon.log) if they exist
func (m *LocalConfigManager) AddLogger(ctx context.Context, loggerName string, level slog.Level, sinkNames []string) error {
	m.Debug("adding logger", "logger", loggerName, "level", level.String(), "sinks", sinkNames)

	// If no sinks specified, use default sinks
	if len(sinkNames) == 0 {
		sinkNames = m.getDefaultSinks()
		if len(sinkNames) == 0 {
			m.Warn("no sinks specified and no default sinks available for new logger", "logger", loggerName)
		} else {
			m.Debug("applying default sinks to new logger", "logger", loggerName, "sinks", sinkNames)
		}
	}

	// Set logger level
	if err := m.registry.SetLoggerLevel(loggerName, level); err != nil {
		return m.Errorf("failed to set logger level: %w", err)
	}

	// Set logger sinks (if any available)
	if len(sinkNames) > 0 {
		if err := m.registry.SetLoggerSinks(loggerName, sinkNames); err != nil {
			return m.Errorf("failed to set logger sinks: %w", err)
		}
	}

	m.Debug("logger added", "logger", loggerName, "level", level.String(), "sinks", sinkNames)
	return nil
}

// getDefaultSinks returns the list of default sink names that should be attached to new loggers
// Returns sinks that exist in the registry from the standard default list
func (m *LocalConfigManager) getDefaultSinks() []string {
	defaultSinkNames := []string{"stdout", "buildozer-daemon.log"}
	var availableSinks []string

	allSinks := m.registry.GetAllSinks()
	sinkMap := make(map[string]bool)
	for _, sink := range allSinks {
		sinkMap[sink.Name] = true
	}

	for _, sinkName := range defaultSinkNames {
		if sinkMap[sinkName] {
			availableSinks = append(availableSinks, sinkName)
		}
	}

	return availableSinks
}

// RemoveLogger removes a logger configuration from the registry
func (m *LocalConfigManager) RemoveLogger(ctx context.Context, loggerName string) error {
	m.Debug("removing logger", "logger", loggerName)
	err := m.registry.RemoveLogger(loggerName)
	if err != nil {
		return m.Errorf("failed to remove logger: %w", err)
	}
	m.Debug("logger removed", "logger", loggerName)
	return nil
}

// AttachSink attaches a sink to an existing logger
func (m *LocalConfigManager) AttachSink(ctx context.Context, loggerName, sinkName string) error {
	m.Debug("attaching sink to logger", "logger", loggerName, "sink", sinkName)
	err := m.registry.AttachSink(loggerName, sinkName)
	if err != nil {
		return m.Errorf("failed to attach sink: %w", err)
	}
	m.Debug("sink attached to logger", "logger", loggerName, "sink", sinkName)
	return nil
}

// DetachSink removes a sink from a logger
func (m *LocalConfigManager) DetachSink(ctx context.Context, loggerName, sinkName string) error {
	m.Debug("detaching sink from logger", "logger", loggerName, "sink", sinkName)
	err := m.registry.DetachSink(loggerName, sinkName)
	if err != nil {
		return m.Errorf("failed to detach sink: %w", err)
	}
	m.Debug("sink detached from logger", "logger", loggerName, "sink", sinkName)
	return nil
}

// TailLogs is not implemented for local manager (would require buffering)
func (m *LocalConfigManager) TailLogs(ctx context.Context, logLevels []slog.Level, loggerFilter string, historyLines int) (<-chan *LogRecord, error) {
	m.Debug("TailLogs requested", "filter", loggerFilter, "history_lines", historyLines)
	return nil, m.Errorf("TailLogs not supported for local configuration manager")
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
