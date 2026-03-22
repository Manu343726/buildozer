package logging

import (
	"log/slog"
	"sync"
)

// GlobalLogger holds the global logger registry
var (
	globalRegistry *Registry
	globalFactory  *Factory
	mu             sync.Mutex
	initialized    bool
)

// InitializeGlobal initializes the global logging system.
// Must be called before any loggers are created to ensure correct configuration is used.
// If already initialized, this call is a no-op and returns nil.
func InitializeGlobal(config LoggingConfig) error {
	mu.Lock()
	// If already initialized, do nothing
	if initialized {
		mu.Unlock()
		return nil
	}

	// Check if we need to initialize
	needsInit := globalRegistry == nil
	mu.Unlock()

	if needsInit {
		// Do initialization without holding the lock
		// (initialization may trigger logger calls which would deadlock)
		registry := NewRegistry()
		factory := NewFactory(registry)

		if err := factory.InitializeFromConfig(config); err != nil {
			return err
		}

		// Now lock to store and mark as initialized
		mu.Lock()
		// Double-check that it wasn't initialized by another goroutine
		if !initialized {
			globalRegistry = registry
			globalFactory = factory
			slog.SetDefault(registry.GetLogger("buildozer").Logger)
			initialized = true
		}
		mu.Unlock()
	}

	return nil
}

// GetRegistry returns the global logger registry.
// If not yet initialized by InitializeGlobal(), initializes with default config.
func GetRegistry() *Registry {
	// First check without lock (read-only check)
	if globalRegistry != nil {
		return globalRegistry
	}

	// Need to initialize - do it without holding the lock
	mu.Lock()
	// Double-check pattern: verify again under lock
	if globalRegistry != nil {
		mu.Unlock()
		return globalRegistry
	}

	// Unlock before initialization (to avoid deadlock when code calls GetLogger)
	mu.Unlock()

	registry := NewRegistry()
	factory := NewFactory(registry)

	if err := factory.InitializeFromConfig(DefaultLoggingConfig()); err != nil {
		panic("failed to initialize global logging: " + err.Error())
	}

	// Lock again to store
	mu.Lock()
	// Double-check: another goroutine might have already initialized
	if globalRegistry == nil {
		globalRegistry = registry
		globalFactory = factory
		slog.SetDefault(registry.GetLogger("buildozer").Logger)
		initialized = true
	}
	mu.Unlock()

	return globalRegistry
}

// GetLogger returns a logger from the global registry
func GetLogger(name string) *Logger {
	return GetRegistry().GetLogger(name)
}

// Log returns the main "buildozer" logger
// This is the root logger for the application and is set as the default slog logger
func Log() *Logger {
	return GetLogger("buildozer")
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
		Filename:   filePath,
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

// CreateBroadcasterSink creates a new broadcaster sink with the given name
// This is typically called per RPC streaming session with a unique name
func CreateBroadcasterSink(sinkName string) (*LogBroadcaster, error) {
	registry := GetRegistry()

	broadcaster := NewLogBroadcaster()
	handler := NewBroadcasterHandler(broadcaster, &slog.HandlerOptions{Level: slog.LevelDebug})

	sink := &Sink{
		Name:    sinkName,
		Type:    "broadcaster",
		Level:   slog.LevelDebug,
		Handler: handler,
	}

	if err := registry.RegisterSink(sink); err != nil {
		return nil, err
	}

	return broadcaster, nil
}

// AddSinkToLoggers adds a sink to specified loggers
func AddSinkToLoggers(sinkName string, loggerNames []string) error {
	registry := GetRegistry()

	for _, loggerName := range loggerNames {
		sinkNames, _ := registry.GetLoggerSinks(loggerName)

		// Check if sink already added
		alreadyAdded := false
		for _, s := range sinkNames {
			if s == sinkName {
				alreadyAdded = true
				break
			}
		}

		if !alreadyAdded {
			sinkNames = append(sinkNames, sinkName)
			if err := registry.SetLoggerSinks(loggerName, sinkNames); err != nil {
				return err
			}
		}
	}

	return nil
}

// RemoveSinkFromLoggers removes a sink from specified loggers
func RemoveSinkFromLoggers(sinkName string, loggerNames []string) error {
	registry := GetRegistry()

	for _, loggerName := range loggerNames {
		sinkNames, _ := registry.GetLoggerSinks(loggerName)

		newSinkNames := []string{}
		for _, s := range sinkNames {
			if s != sinkName {
				newSinkNames = append(newSinkNames, s)
			}
		}

		if err := registry.SetLoggerSinks(loggerName, newSinkNames); err != nil {
			return err
		}
	}

	return nil
}
