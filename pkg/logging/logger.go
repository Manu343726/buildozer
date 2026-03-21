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
	// The underlying slog.Logger, created with a custom handler for dynamic routing
	slogger *slog.Logger

	// Hierarchical logger name (e.g., "buildozer.runtime.cpp")
	name string

	// Registry for sink lookup and routing
	registry *Registry

	mu sync.RWMutex
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
	sinks         map[string]*Sink    // sink name -> Sink
	loggerConfigs map[string][]string // logger name -> list of sink names
	mu            sync.RWMutex
	globalLevel   slog.Level
}

// registryHandler is a custom slog.Handler that routes to registry.Log()
type registryHandler struct {
	registry   *Registry
	loggerName string
}

// NewRegistry creates a new logger registry with default level
func NewRegistry() *Registry {
	return &Registry{
		sinks:         make(map[string]*Sink),
		loggerConfigs: make(map[string][]string),
		globalLevel:   slog.LevelInfo,
	}
}

// GetLogger returns a logger with the given hierarchical name.
// The logger uses dynamic routing to sinks based on current configuration.
func (r *Registry) GetLogger(name string) *Logger {
	return &Logger{
		slogger: slog.New(&registryHandler{
			registry:   r,
			loggerName: name,
		}),
		name:     name,
		registry: r,
	}
}

// Log handles a log record by routing it to configured sinks based on logger hierarchy.
// Does hierarchical lookup: exact match first, then parent loggers.
// Example: for "a.b.c", tries "a.b.c", then "a.b", then "a"
func (r *Registry) Log(ctx context.Context, record slog.Record) error {
	loggerName := record.Message // This will be overwritten, but we need to extract it first

	// Look for the logger name in the record attributes
	// We'll add it as the first attribute with key "_logger"
	var foundLoggerName string
	record.Attrs(func(a slog.Attr) bool {
		if a.Key == "_logger" {
			foundLoggerName = a.Value.String()
		}
		return true
	})

	if foundLoggerName == "" {
		foundLoggerName = loggerName
	}

	// Get sinks for this logger hierarchy
	sinkNames := r.getLoggerSinks(foundLoggerName)

	// Route to all configured sinks
	for _, sinkName := range sinkNames {
		if sink, exists := r.GetSink(sinkName); exists {
			sink.Handler.Handle(ctx, record)
		}
	}

	return nil
}

// getLoggerSinks performs hierarchical lookup for sinks.
// Tries exact match first, then parent loggers.
func (r *Registry) getLoggerSinks(name string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Try exact match first
	if sinkNames, exists := r.loggerConfigs[name]; exists {
		return sinkNames
	}

	// Try hierarchical lookup (parent loggers)
	parts := strings.Split(name, ".")
	for i := len(parts) - 1; i > 0; i-- {
		parentName := strings.Join(parts[:i], ".")
		if sinkNames, exists := r.loggerConfigs[parentName]; exists {
			return sinkNames
		}
	}

	return []string{}
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

	r.loggerConfigs[loggerName] = sinkNames
	return nil
}

// GetLoggerSinks returns the sink names configured for a logger
func (r *Registry) GetLoggerSinks(loggerName string) ([]string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	sinkNames, exists := r.loggerConfigs[loggerName]
	return sinkNames, exists
}

// SetLoggerLevel sets the level for a specific logger (deprecated for backward compat)
func (r *Registry) SetLoggerLevel(loggerName string, level slog.Level) error {
	r.mu.RLock()
	_, exists := r.loggerConfigs[loggerName]
	r.mu.RUnlock()

	if !exists {
		return fmt.Errorf("logger %q not found", loggerName)
	}
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
	record.AddAttrs(slog.String("_logger", h.loggerName))
	return h.registry.Log(ctx, record)
}

// WithAttrs returns a new handler with the given attributes applied.
// Creates an attributes-wrapping handler that applies attrs then delegates to this handler.
func (h *registryHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &attributesHandler{
		attrs:   attrs,
		next:    h,
		group:   "",
		attrs4: nil,
	}
}

// WithGroup returns a new handler with the given group applied.
// Creates a group-wrapping handler that applies the group then delegates to this handler.
func (h *registryHandler) WithGroup(name string) slog.Handler {
	return &groupHandler{
		group: name,
		next:  h,
	}
}

// Enabled returns whether the handler handles the log level
func (h *registryHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.registry.GetGlobalLevel()
}

// ============ Attribute Handler Wrappers ============

// attributesHandler applies accumulated attributes before delegating to the next handler
type attributesHandler struct {
	attrs   []slog.Attr
	next    slog.Handler
	group   string
	attrs4  []slog.Attr
}

func (a *attributesHandler) Handle(ctx context.Context, record slog.Record) error {
	// Apply accumulated attributes to the record
	record.AddAttrs(a.attrs...)
	return a.next.Handle(ctx, record)
}

func (a *attributesHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	// Accumulate more attributes
	newAttrs := append([]slog.Attr{}, a.attrs...)
	newAttrs = append(newAttrs, attrs...)
	return &attributesHandler{
		attrs:  newAttrs,
		next:   a.next,
		group:  a.group,
		attrs4: a.attrs4,
	}
}

func (a *attributesHandler) WithGroup(name string) slog.Handler {
	return &groupHandler{
		group: name,
		next:  a,
	}
}

func (a *attributesHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return a.next.Enabled(ctx, level)
}

// groupHandler applies a group context before delegating to the next handler
type groupHandler struct {
	group string
	next  slog.Handler
}

func (g *groupHandler) Handle(ctx context.Context, record slog.Record) error {
	// Add group context to the record
	if g.group != "" {
		record.AddAttrs(slog.Group(g.group))
	}
	return g.next.Handle(ctx, record)
}

func (g *groupHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &attributesHandler{
		attrs:  attrs,
		next:   g.next,
		group:  g.group,
		attrs4: nil,
	}
}

func (g *groupHandler) WithGroup(name string) slog.Handler {
	// Nested groups - just create a new groupHandler
	return &groupHandler{
		group: name,
		next:  g,
	}
}

func (g *groupHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return g.next.Enabled(ctx, level)
}

// ============ Logger Methods - Full slog.Logger Interface ============

// Name returns the logger's hierarchical name
func (l *Logger) Name() string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.name
}

// Child creates a child logger with an appended hierarchical name.
// Example: logger.Child("module") creates "parent.module"
// All attributes and groups from parent are inherited via slog.Logger
func (l *Logger) Child(name string) *Logger {
	l.mu.RLock()
	parentName := l.name
	registry := l.registry
	slogger := l.slogger
	l.mu.RUnlock()

	childName := parentName + "." + name
	// Create child with same slogger (which carries inherited attrs/groups)
	return &Logger{
		slogger:  slogger,
		name:     childName,
		registry: registry,
	}
}

// Debug logs at LevelDebug
func (l *Logger) Debug(msg string, args ...any) {
	l.log(context.Background(), slog.LevelDebug, msg, args...)
}

// DebugContext logs at LevelDebug with context
func (l *Logger) DebugContext(ctx context.Context, msg string, args ...any) {
	l.log(ctx, slog.LevelDebug, msg, args...)
}

// Info logs at LevelInfo
func (l *Logger) Info(msg string, args ...any) {
	l.log(context.Background(), slog.LevelInfo, msg, args...)
}

// InfoContext logs at LevelInfo with context
func (l *Logger) InfoContext(ctx context.Context, msg string, args ...any) {
	l.log(ctx, slog.LevelInfo, msg, args...)
}

// Warn logs at LevelWarn
func (l *Logger) Warn(msg string, args ...any) {
	l.log(context.Background(), slog.LevelWarn, msg, args...)
}

// WarnContext logs at LevelWarn with context
func (l *Logger) WarnContext(ctx context.Context, msg string, args ...any) {
	l.log(ctx, slog.LevelWarn, msg, args...)
}

// Error logs at LevelError
func (l *Logger) Error(msg string, args ...any) {
	l.log(context.Background(), slog.LevelError, msg, args...)
}

// ErrorContext logs at LevelError with context
func (l *Logger) ErrorContext(ctx context.Context, msg string, args ...any) {
	l.log(ctx, slog.LevelError, msg, args...)
}

// Log logs at the specified level
func (l *Logger) Log(level slog.Level, msg string, args ...any) {
	l.log(context.Background(), level, msg, args...)
}

// LogContext logs at the specified level with context
func (l *Logger) LogContext(ctx context.Context, level slog.Level, msg string, args ...any) {
	l.log(ctx, level, msg, args...)
}

// LogAttrs logs at the specified level with attributes
func (l *Logger) LogAttrs(level slog.Level, msg string, attrs ...slog.Attr) {
	l.logAttrs(context.Background(), level, msg, attrs...)
}

// LogAttrsContext logs at the specified level with attributes and context
func (l *Logger) LogAttrsContext(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr) {
	l.logAttrs(ctx, level, msg, attrs...)
}

// WithAttrs returns a logger with the given attributes added to all messages.
// Returns a new Logger wrapping the result of slog.Logger.With().
func (l *Logger) WithAttrs(attrs ...slog.Attr) *Logger {
	l.mu.RLock()
	name := l.name
	registry := l.registry
	// Convert attrs to interface{} for With() call
	args := make([]interface{}, len(attrs))
	for i, attr := range attrs {
		args[i] = attr
	}
	slogger := l.slogger.With(args...)
	l.mu.RUnlock()

	return &Logger{
		slogger:  slogger,
		name:     name,
		registry: registry,
	}
}

// WithGroup returns a logger with a group added to all messages.
// Returns a new Logger wrapping the result of slog.Logger.WithGroup().
func (l *Logger) WithGroup(name string) *Logger {
	l.mu.RLock()
	loggerName := l.name
	registry := l.registry
	slogger := l.slogger.WithGroup(name)
	l.mu.RUnlock()

	return &Logger{
		slogger:  slogger,
		name:     loggerName,
		registry: registry,
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

// ============ Internal logging methods ============

// log is the internal method that performs the actual logging
func (l *Logger) log(ctx context.Context, level slog.Level, msg string, args ...any) {
	l.slogger.Log(ctx, level, msg, args...)
}

// logAttrs is the internal method for logging with attributes
func (l *Logger) logAttrs(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr) {
	l.slogger.LogAttrs(ctx, level, msg, attrs...)
}
