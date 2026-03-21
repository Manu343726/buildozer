package logging

import (
	"log/slog"
	"sync"
)

// GlobalLogger holds the global logger registry
var (
	globalRegistry *Registry
	globalFactory  *Factory
	mu             sync.Once
)

// InitializeGlobal initializes the global logging system
func InitializeGlobal(config LoggingConfig) error {
	var initErr error
	mu.Do(func() {
		globalRegistry = NewRegistry()
		globalFactory = NewFactory(globalRegistry)
		initErr = globalFactory.InitializeFromConfig(config)
	})
	return initErr
}

// GetRegistry returns the global logger registry
func GetRegistry() *Registry {
	if globalRegistry == nil {
		// Initialize with default config if not yet initialized
		globalRegistry = NewRegistry()
		globalFactory = NewFactory(globalRegistry)
		globalFactory.InitializeFromConfig(DefaultLoggingConfig())
	}
	return globalRegistry
}

// GetLogger returns a logger from the global registry
func GetLogger(name string) *Logger {
	return GetRegistry().GetLogger(name)
}

// SetLoggerLevel sets the level for a logger globally
func SetLoggerLevel(loggerName string, level slog.Level) error {
	return GetRegistry().SetLoggerLevel(loggerName, level)
}

// SetGlobalLevel sets the global logging level
func SetGlobalLevel(level slog.Level) {
	GetRegistry().SetGlobalLevel(level)
}

// SetSinkLevel sets the level for a sink globally
func SetSinkLevel(sinkName string, level slog.Level) error {
	return GetRegistry().SetSinkLevel(sinkName, level)
}

// GetLoggerStatus returns status of all loggers
func GetLoggerStatus() map[string]interface{} {
	return GetRegistry().GetLoggerStatus()
}

// GetSinkStatus returns status of all sinks
func GetSinkStatus() map[string]interface{} {
	return GetRegistry().GetSinkStatus()
}

// GetGlobalLevel returns the current global logging level
func GetGlobalLevel() slog.Level {
	return GetRegistry().GetGlobalLevel()
}

// EnableLoggerFileSink enables a logger-specific rotating file sink
// maxSizeMB: maximum file size in MB (default: 100)
// maxAgeDays: maximum age of log files in days before cleanup (0 = disabled)
func EnableLoggerFileSink(loggerName, filePath string, maxSizeMB int, maxAgeDays int) error {
	registry := GetRegistry()
	factory := globalFactory

	sinkName := "file-" + loggerName

	config := SinkConfig{
		Name:       sinkName,
		Type:       "file",
		Level:      "trace", // Capture everything at trace level
		Path:       filePath,
		MaxSizeB:   int64(maxSizeMB) * 1024 * 1024,
		MaxFiles:   5,
		MaxAgeDays: maxAgeDays,
	}

	_, err := factory.CreateSink(config)
	if err != nil {
		return err
	}

	// Get current sinks for this logger
	sinkNames, exists := registry.GetLoggerSinks(loggerName)
	if !exists {
		sinkNames = []string{}
	}

	// Add new sink to the list
	sinkNames = append(sinkNames, sinkName)

	return registry.SetLoggerSinks(loggerName, sinkNames)
}

// DisableLoggerFileSink disables a logger-specific file sink
func DisableLoggerFileSink(loggerName string) error {
	registry := GetRegistry()
	sinkName := "file-" + loggerName

	// Get current sinks for this logger
	sinkNames, exists := registry.GetLoggerSinks(loggerName)
	if !exists {
		return nil // Logger not configured
	}

	// Remove the sink
	newSinkNames := []string{}
	for _, s := range sinkNames {
		if s != sinkName {
			newSinkNames = append(newSinkNames, s)
		}
	}

	return registry.SetLoggerSinks(loggerName, newSinkNames)
}

// GetAvailableSinks returns a list of all registered sinks
func GetAvailableSinks() []string {
	registry := GetRegistry()
	registry.mu.RLock()
	defer registry.mu.RUnlock()

	sinks := make([]string, 0, len(registry.sinks))
	for name := range registry.sinks {
		sinks = append(sinks, name)
	}
	return sinks
}

// GetAvailableLoggers returns a list of all registered loggers
func GetAvailableLoggers() []string {
	registry := GetRegistry()
	registry.mu.RLock()
	defer registry.mu.RUnlock()

	loggers := make([]string, 0, len(registry.loggerConfigs))
	for name := range registry.loggerConfigs {
		loggers = append(loggers, name)
	}
	return loggers
}
