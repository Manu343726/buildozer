package logging

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
)

// Logger wraps an slog.Logger with hierarchical name tracking and dynamic sink routing.
// All slog.Logger methods are implemented. Additionally provides:
// - Child(name): Create a child logger with appended hierarchy
// - Errorf(format, args): Log and return an error
// - Panicf(format, args): Log and panic
// - WithAttrs(attrs): Create a logger with fixed attributes added to all messages (delegates to slog.Logger)
// - WithGroup(name): Create a logger with a group added to all messages (delegates to slog.Logger)
type Logger struct {
	*slog.Logger
}

// Sink represents a log sink (destination) with a handler and filtering level
type Sink struct {
	Name    string       // Unique sink name (e.g., "stdout", "file-app")
	Type    string       // Type of sink (stdout, stderr, file, syslog)
	Level   slog.Level   // Minimum level to log to this sink
	Handler slog.Handler // The slog handler for this sink
	mu      sync.RWMutex
}

// Registry manages sinks and logger configurations with hierarchical lookup.
// The registry routes log entries based on logger hierarchy.
type Registry struct {
	sinks         map[string]*Sink            // sink name -> Sink
	loggerConfigs map[string]configuredLogger // logger name -> configuredLogger
	mu            sync.RWMutex
	globalLevel   slog.Level
}

type attributeOrGroup struct {
	isGroup bool
	name    string
	value   slog.Value
}

// registryHandler is a custom slog.Handler that routes to registry.Log()
type registryHandler struct {
	registry     *Registry
	loggerName   string
	attrOrGroups []attributeOrGroup
}

// NewRegistry creates a new logger registry with default level
func NewRegistry() *Registry {
	return &Registry{
		sinks:         make(map[string]*Sink),
		loggerConfigs: make(map[string]configuredLogger),
		globalLevel:   slog.LevelInfo,
	}
}

// GetLogger returns a logger with the given hierarchical name.
// The logger uses dynamic routing to sinks based on current configuration.
func (r *Registry) GetLogger(name string) *Logger {
	return &Logger{
		Logger: slog.New(&registryHandler{
			registry:   r,
			loggerName: name,
		}),
	}
}

const LoggerNameAttr = "logger"

// Log handles a log record by routing it to configured sinks based on logger hierarchy.
// Does hierarchical lookup: exact match first, then parent loggers.
// Example: for "a.b.c", tries "a.b.c", then "a.b", then "a"
func (r *Registry) log(ctx context.Context, record slog.Record, attrsOrGroups []attributeOrGroup) error {
	loggerName := record.Message // This will be overwritten, but we need to extract it first

	// Look for the logger name in the record attributes
	// We'll add it as the first attribute with key "_logger"
	var foundLoggerName string
	record.Attrs(func(a slog.Attr) bool {
		if a.Key == LoggerNameAttr {
			foundLoggerName = a.Value.String()
		}
		return true
	})

	if foundLoggerName == "" {
		foundLoggerName = loggerName
	}

	// Get level and sink names for this logger using hierarchical lookup
	config := r.getLoggerConfig(foundLoggerName)

	// Route to all configured sinks
	if config != nil && config.level <= record.Level {
		for _, sinkName := range config.sinkNames {
			if sink, exists := r.GetSink(sinkName); exists && sink.Handler.Enabled(ctx, record.Level) {
				handler := sink.Handler
				// Simulate WithAttrs and WithGroup calls in order from the registryHandler buffer in order
				//
				// There should be a better way to do this, but since the slog text and JSON handlers implement WithAttrs() and WithGroup()
				// differently there's no homogeneous way to do this while presering order (JSON hanlder for example groups attributes by group, where attributes are tagged with
				// the group name depending on the order of WithAttrs and WithGroup calls)
				for _, ag := range attrsOrGroups {
					if ag.isGroup {
						handler = handler.WithGroup(ag.name)
					} else {
						handler = handler.WithAttrs([]slog.Attr{{Key: ag.name, Value: ag.value}})
					}
				}

				handler.Handle(ctx, record)
			}
		}
	}

	return nil
}

type configuredLogger struct {
	level     slog.Level
	sinkNames []string
}

// getLoggerConfig performs hierarchical lookup for logger configuration.
// Tries exact match first, then parent loggers.
func (r *Registry) getLoggerConfig(name string) *configuredLogger {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Try exact match first
	if loggerConfig, exists := r.loggerConfigs[name]; exists {
		return &loggerConfig
	}

	// Try hierarchical lookup (parent loggers)
	parts := strings.Split(name, ".")
	for i := len(parts) - 1; i > 0; i-- {
		parentName := strings.Join(parts[:i], ".")
		if loggerConfig, exists := r.loggerConfigs[parentName]; exists {
			return &loggerConfig
		}
	}

	return nil
}

// RegisterSink registers a new sink in the registry
func (r *Registry) RegisterSink(sink *Sink) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.sinks[sink.Name]; exists {
		return fmt.Errorf("sink %q already registered", sink.Name)
	}

	r.sinks[sink.Name] = sink
	return nil
}

// GetSink retrieves a sink by name
func (r *Registry) GetSink(name string) (*Sink, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	sink, exists := r.sinks[name]
	return sink, exists
}

// SetLoggerSinks configures which sinks are used by a logger
func (r *Registry) SetLoggerSinks(loggerName string, sinkNames []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Validate that all sinks exist
	for _, sinkName := range sinkNames {
		if _, exists := r.sinks[sinkName]; !exists {
			return fmt.Errorf("sink %q not found", sinkName)
		}
	}

	currentConfig, exists := r.loggerConfigs[loggerName]
	if !exists {
		currentConfig = configuredLogger{
			level:     slog.LevelInfo, // default level
			sinkNames: sinkNames,
		}
	} else {
		currentConfig.sinkNames = sinkNames
	}

	r.loggerConfigs[loggerName] = currentConfig
	return nil
}

// GetLoggerSinks returns the sink names configured for a logger
func (r *Registry) GetLoggerSinks(loggerName string) ([]string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	sinkNames, exists := r.loggerConfigs[loggerName]
	if !exists {
		return nil, false
	}

	return sinkNames.sinkNames, true
}

// SetLoggerLevel sets the level for a specific logger (deprecated for backward compat)
func (r *Registry) SetLoggerLevel(loggerName string, level slog.Level) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	currentConfig, exists := r.loggerConfigs[loggerName]

	if !exists {
		return fmt.Errorf("logger %q not found", loggerName)
	}

	currentConfig.level = level
	r.loggerConfigs[loggerName] = currentConfig

	return nil
}

// SetGlobalLevel sets the global logging level
func (r *Registry) SetGlobalLevel(level slog.Level) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.globalLevel = level
}

// GetGlobalLevel returns the global logging level
func (r *Registry) GetGlobalLevel() slog.Level {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.globalLevel
}

// SetSinkLevel sets the level for a specific sink
func (r *Registry) SetSinkLevel(sinkName string, level slog.Level) error {
	r.mu.RLock()
	sink, exists := r.sinks[sinkName]
	r.mu.RUnlock()

	if !exists {
		return fmt.Errorf("sink %q not found", sinkName)
	}

	sink.SetLevel(level)
	return nil
}

// ============ Sink Methods ============

// SetLevel sets the log level for this sink
func (s *Sink) SetLevel(level slog.Level) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Level = level
}

// GetLevel returns the log level for this sink
func (s *Sink) GetLevel() slog.Level {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Level
}

// ============ registryHandler Implementation ============

// Handle implements slog.Handler
func (h *registryHandler) Handle(ctx context.Context, record slog.Record) error {
	// Add the logger name as an attribute
	record.AddAttrs(slog.Attr{Key: LoggerNameAttr, Value: slog.StringValue(h.loggerName)})

	// Route to registry for handling
	return h.registry.log(ctx, record, h.attrOrGroups)
}

// WithAttrs returns a new handler with the given attributes applied.
// Creates an attributes-wrapping handler that applies attrs then delegates to this handler.
func (h *registryHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := append([]attributeOrGroup{}, h.attrOrGroups...)
	for _, attr := range attrs {
		newAttrs = append(newAttrs, attributeOrGroup{
			isGroup: false,
			name:    attr.Key,
			value:   attr.Value,
		})
	}
	return &registryHandler{
		registry:     h.registry,
		loggerName:   h.loggerName,
		attrOrGroups: newAttrs,
	}
}

// WithGroup returns a new handler with the given group applied.
// Creates a group-wrapping handler that applies the group then delegates to this handler.
func (h *registryHandler) WithGroup(name string) slog.Handler {
	return &registryHandler{
		registry:   h.registry,
		loggerName: h.loggerName,
		attrOrGroups: append(h.attrOrGroups, attributeOrGroup{
			isGroup: true,
			name:    name,
		}),
	}
}

// Child creates a handler for a child logger with an appended hierarchical name.
func (h *registryHandler) Child(name string) slog.Handler {
	childName := h.loggerName + "." + name
	return &registryHandler{
		registry:     h.registry,
		loggerName:   childName,
		attrOrGroups: h.attrOrGroups, // Inherit attributes and groups from parent
	}
}

// Enabled returns whether the handler handles the log level
func (h *registryHandler) Enabled(ctx context.Context, level slog.Level) bool {
	// The registryHandler itself doesn't filter by level; filtering is done in the registry.Log() method based on configuration.
	return true
}

// ============ Logger Methods - Full slog.Logger Interface ============

// Child creates a child logger with an appended hierarchical name.
// Example: logger.Child("module") creates "parent.module"
// All attributes and groups from parent are inherited via slog.Logger
func (l *Logger) Child(name string) *Logger {
	handler := l.Handler().(*registryHandler)
	childHandler := handler.Child(name)
	return &Logger{
		Logger: slog.New(childHandler),
	}
}

// WithAttrs returns a logger with the given attributes added to all messages.
// Returns a new Logger wrapping the result of slog.Logger.With().
func (l *Logger) WithAttrs(attrs ...slog.Attr) *Logger {
	handler := l.Handler().(*registryHandler)
	childHandler := handler.WithAttrs(attrs)
	return &Logger{
		Logger: slog.New(childHandler),
	}
}

// WithGroup returns a logger with a group added to all messages.
// Returns a new Logger wrapping the result of slog.Logger.WithGroup().
func (l *Logger) WithGroup(name string) *Logger {
	handler := l.Handler().(*registryHandler)
	childHandler := handler.WithGroup(name)
	return &Logger{
		Logger: slog.New(childHandler),
	}
}

// Errorf logs an error and returns it
// Format: Errorf(format, args...)
// Example: if err := doSomething(); err != nil { return logger.Errorf("failed: %w", err) }
func (l *Logger) Errorf(format string, args ...any) error {
	msg := fmt.Sprintf(format, args...)
	l.Error(msg)
	return errors.New(msg)
}

// Panicf logs a message and panics
// Format: Panicf(format, args...)
// Example: if critical := check(); !critical { logger.Panicf("critical check failed: %v", reason) }
func (l *Logger) Panicf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	l.Error(msg)
	panic(msg)
}
