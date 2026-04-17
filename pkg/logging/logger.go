package logging

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
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
	Name             string       // Unique sink name (e.g., "stdout", "file-app")
	Type             string       // Type of sink (stdout, stderr, file, syslog)
	FilePath         string       // File path for file sinks (empty for stdout/stderr)
	Level            slog.Level   // Minimum level to log to this sink
	Handler          slog.Handler // The slog handler for this sink
	MaxSize          int64        // Max file size in bytes (file sinks only)
	MaxBackups       int32        // Max rotated files to keep (file sinks only)
	MaxAgeDays       int32        // Max age in days (file sinks only)
	JSONFormat       bool         // Whether to use JSON format (file sinks only)
	IncludeSourceLoc bool         // Include source location in logs (file:line)
	mu               sync.RWMutex
}

// Activity-based logger tracking constants
const (
	// activeLoggerThreshold is how long a logger can be inactive before it's considered stale
	activeLoggerThreshold = 30 * time.Second
)

// Registry manages sinks and logger configurations with hierarchical lookup.
// The registry routes log entries based on logger hierarchy.
// It also tracks active logger names and their activity timestamps for status queries.
type Registry struct {
	sinks          map[string]*Sink            // sink name -> Sink
	loggerConfigs  map[string]configuredLogger // logger name -> configuredLogger
	activeLoggers  map[string]struct{}         // logger name -> (set of active logger names)
	loggerActivity map[string]time.Time        // logger name -> last activity time, for cleanup of temporary loggers
	mu             sync.RWMutex
	globalLevel    slog.Level
	loggingDir     string // Base directory for file sinks
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
		sinks:          make(map[string]*Sink),
		loggerConfigs:  make(map[string]configuredLogger),
		activeLoggers:  make(map[string]struct{}),
		loggerActivity: make(map[string]time.Time),
		globalLevel:    slog.LevelInfo,
	}
}

// GetLogger returns a logger with the given hierarchical name.
// The logger uses dynamic routing to sinks based on current configuration.
// The logger name is registered in the active loggers registry with activity tracking.
// A finalizer is registered to clean up the logger when it's garbage collected.
func (r *Registry) GetLogger(name string) *Logger {
	logger := &Logger{
		Logger: slog.New(&registryHandler{
			registry:   r,
			loggerName: name,
		}),
	}

	// Set up finalizer for automatic cleanup
	setupLoggerFinalizer(logger)

	return logger
}

// setupLoggerFinalizer sets up a finalizer on a logger instance.
// Extracts the registry and logger name from the handler and registers the finalizer.
// Used by all logger creation methods: GetLogger, Child, With, WithGroup.
func setupLoggerFinalizer(logger *Logger) {
	handler := logger.Handler().(*registryHandler)
	registry := handler.registry
	loggerName := handler.loggerName

	registry.mu.Lock()
	registry.activeLoggers[loggerName] = struct{}{}
	registry.loggerActivity[loggerName] = time.Now()
	registry.mu.Unlock()

	// Set finalizer to clean up when logger is garbage collected
	runtime.SetFinalizer(logger, func(l *Logger) {
		registry.mu.Lock()
		delete(registry.activeLoggers, loggerName)
		delete(registry.loggerActivity, loggerName)
		registry.mu.Unlock()
	})
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

	// Determine effective level: global level is a FLOOR that cannot be lowered
	// Use the MORE restrictive (higher) of global vs logger-specific level
	// Examples:
	//   Global=INFO, Logger=DEBUG  → use DEBUG (logger can be more verbose than global)
	//   Global=INFO, Logger=ERROR  → use ERROR (logger can't override to be less verbose)
	//   Global=ERROR, Logger=INFO  → use ERROR (can't lower the global floor)
	effectiveLevel := r.globalLevel

	if config != nil {
		// Logger has explicit configuration - use whichever is MORE restrictive (higher value)
		// In slog, higher values = more restrictive (ERROR=8 > INFO=0 > DEBUG=-4 > TRACE=-8)
		if config.level > r.globalLevel {
			effectiveLevel = config.level // Logger is more restrictive, use it
		}
		// else: global is more restrictive or equal, keep global level as the floor
	}

	// Route to configured sinks if level allows
	if config != nil && effectiveLevel <= record.Level {
		for _, sinkName := range config.sinkNames {
			if sink, exists := r.GetSink(sinkName); exists && sink.Handler.Enabled(ctx, record.Level) {
				handler := sink.Handler

				// Extract daemon attribute if present - add it first via WithAttrs() so it appears early
				// (daemon should appear right after level in output, before other attributes)
				for _, ag := range attrsOrGroups {
					if !ag.isGroup && ag.name == "daemon" {
						handler = handler.WithAttrs([]slog.Attr{{Key: "daemon", Value: ag.value}})
						break
					}
				}

				// Simulate WithAttrs and WithGroup calls in order from the registryHandler buffer in order
				//
				// There should be a better way to do this, but since the slog text and JSON handlers implement WithAttrs() and WithGroup()
				// differently there's no homogeneous way to do this while presering order (JSON hanlder for example groups attributes by group, where attributes are tagged with
				// the group name depending on the order of WithAttrs and WithGroup calls)
				for _, ag := range attrsOrGroups {
					if ag.isGroup {
						handler = handler.WithGroup(ag.name)
					} else if ag.name != "daemon" {
						// Skip daemon - already added above
						handler = handler.WithAttrs([]slog.Attr{{Key: ag.name, Value: ag.value}})
					}
				}

				_ = handler.Handle(ctx, record)
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

// GetActiveLoggers returns a sorted list of all active logger names.
// Active loggers are those that have been created via GetLogger() or have explicit configuration.
// Results are sorted for consistent output.
func (r *Registry) GetActiveLoggers() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Collect both active loggers (created via GetLogger) and configured loggers
	loggerSet := make(map[string]struct{})
	for name := range r.activeLoggers {
		loggerSet[name] = struct{}{}
	}
	for name := range r.loggerConfigs {
		loggerSet[name] = struct{}{}
	}

	// Convert to sorted slice
	loggers := make([]string, 0, len(loggerSet))
	for name := range loggerSet {
		loggers = append(loggers, name)
	}

	// Sort for consistent output
	sort.Strings(loggers)
	return loggers
}

// IsLoggerActive checks if a logger is currently active.
// A logger is active if it has been created via GetLogger() or has explicit configuration.
func (r *Registry) IsLoggerActive(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, isActive := r.activeLoggers[name]
	_, hasConfig := r.loggerConfigs[name]
	return isActive || hasConfig
}

// ActiveLoggerResolution represents an active logger and the configured logger it resolves to
type ActiveLoggerResolution struct {
	Name              string // The active logger name
	ResolvedConfigFor string // The configured logger this logger resolves to (empty if none)
}

// GetActiveLoggersWithResolution returns all active loggers with information about
// which configured logger each one resolves to via hierarchical lookup.
// Excludes loggers that have been inactive for longer than activeLoggerThreshold.
// Results are sorted by logger name for consistent output.
func (r *Registry) GetActiveLoggersWithResolution() []ActiveLoggerResolution {
	r.mu.RLock()
	defer r.mu.RUnlock()

	now := time.Now()

	// Collect both active loggers and configured loggers
	// But exclude temporary loggers that have been inactive > threshold
	loggerSet := make(map[string]struct{})
	for name := range r.activeLoggers {
		// Check if this logger is stale (inactive for too long)
		if lastActivity, exists := r.loggerActivity[name]; exists {
			if now.Sub(lastActivity) > activeLoggerThreshold {
				// Skip stale loggers (temporary loggers that are no longer in use)
				continue
			}
		}
		loggerSet[name] = struct{}{}
		// Also add parent loggers of active child loggers
		// e.g., if "app.db.postgres" is active, also add "app.db" and "app"
		parts := strings.Split(name, ".")
		for i := len(parts) - 1; i > 0; i-- {
			parent := strings.Join(parts[:i], ".")
			loggerSet[parent] = struct{}{}
		}
	}
	for name := range r.loggerConfigs {
		loggerSet[name] = struct{}{}
	}

	// Convert to sorted slice
	loggerNames := make([]string, 0, len(loggerSet))
	for name := range loggerSet {
		loggerNames = append(loggerNames, name)
	}
	sort.Strings(loggerNames)

	// Build resolution info for each logger
	resolutions := make([]ActiveLoggerResolution, 0, len(loggerNames))
	for _, loggerName := range loggerNames {
		// Find what configured logger this logger resolves to
		resolvedName := r.findResolvedLoggerName(loggerName)
		resolutions = append(resolutions, ActiveLoggerResolution{
			Name:              loggerName,
			ResolvedConfigFor: resolvedName,
		})
	}

	return resolutions
}

// findResolvedLoggerName finds the configured logger name that a logger resolves to
// via hierarchical lookup. Returns empty string if no configuration exists.
// Must be called with r.mu held for reading.
func (r *Registry) findResolvedLoggerName(name string) string {
	// Try exact match first
	if _, exists := r.loggerConfigs[name]; exists {
		return name
	}

	// Try hierarchical lookup (parent loggers)
	parts := strings.Split(name, ".")
	for i := len(parts) - 1; i > 0; i-- {
		parentName := strings.Join(parts[:i], ".")
		if _, exists := r.loggerConfigs[parentName]; exists {
			return parentName
		}
	}

	// No configuration found - logger uses defaults
	return ""
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

// GetAllSinks returns all registered sinks
func (r *Registry) GetAllSinks() []*Sink {
	r.mu.RLock()
	defer r.mu.RUnlock()

	sinks := make([]*Sink, 0, len(r.sinks))
	for _, sink := range r.sinks {
		sinks = append(sinks, sink)
	}
	return sinks
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

// SetLoggingDir sets the base directory for file sinks
func (r *Registry) SetLoggingDir(dir string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.loggingDir = dir
}

// GetLoggingDir returns the base directory for file sinks
func (r *Registry) GetLoggingDir() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.loggingDir
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

// RemoveSink removes a sink from the registry and all loggers
func (r *Registry) RemoveSink(sinkName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.sinks[sinkName]; !exists {
		return fmt.Errorf("sink %q not found", sinkName)
	}

	// Check if any loggers are using this sink
	var loggersUsingSink []string
	for loggerName, config := range r.loggerConfigs {
		for _, s := range config.sinkNames {
			if s == sinkName {
				loggersUsingSink = append(loggersUsingSink, loggerName)
				break
			}
		}
	}

	if len(loggersUsingSink) > 0 {
		return fmt.Errorf("sink %q is in use by loggers: %v", sinkName, loggersUsingSink)
	}

	// Remove sink from registry
	delete(r.sinks, sinkName)

	return nil
}

// RemoveLogger removes a logger configuration from the registry
func (r *Registry) RemoveLogger(loggerName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.loggerConfigs[loggerName]; !exists {
		return fmt.Errorf("logger %q not found", loggerName)
	}

	delete(r.loggerConfigs, loggerName)
	return nil
}

// AttachSink adds a sink to an existing logger
func (r *Registry) AttachSink(loggerName, sinkName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check sink exists
	if _, exists := r.sinks[sinkName]; !exists {
		return fmt.Errorf("sink %q not found", sinkName)
	}

	// Check logger exists
	config, exists := r.loggerConfigs[loggerName]
	if !exists {
		return fmt.Errorf("logger %q not found", loggerName)
	}

	// Check sink is not already attached
	for _, s := range config.sinkNames {
		if s == sinkName {
			return fmt.Errorf("sink %q is already attached to logger %q", sinkName, loggerName)
		}
	}

	// Add sink to logger
	config.sinkNames = append(config.sinkNames, sinkName)
	r.loggerConfigs[loggerName] = config
	return nil
}

// DetachSink removes a sink from a logger
func (r *Registry) DetachSink(loggerName, sinkName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check logger exists
	config, exists := r.loggerConfigs[loggerName]
	if !exists {
		return fmt.Errorf("logger %q not found", loggerName)
	}

	// Find and remove sink
	newSinkNames := make([]string, 0, len(config.sinkNames))
	found := false
	for _, s := range config.sinkNames {
		if s != sinkName {
			newSinkNames = append(newSinkNames, s)
		} else {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("sink %q is not attached to logger %q", sinkName, loggerName)
	}

	config.sinkNames = newSinkNames
	r.loggerConfigs[loggerName] = config
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

	// Update activity timestamp for this logger
	h.registry.mu.Lock()
	h.registry.loggerActivity[h.loggerName] = time.Now()
	h.registry.mu.Unlock()

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
	// Register the child logger as active with activity tracking
	h.registry.mu.Lock()
	h.registry.activeLoggers[childName] = struct{}{}
	h.registry.loggerActivity[childName] = time.Now()
	h.registry.mu.Unlock()
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
	childLogger := &Logger{
		Logger: slog.New(childHandler),
	}
	// Set up finalizer for automatic cleanup of child logger
	setupLoggerFinalizer(childLogger)
	return childLogger
}

const badKey = "!BADKEY"

// argsToAttr turns a prefix of the nonempty args slice into an Attr
// and returns the unconsumed portion of the slice.
// If args[0] is an Attr, it returns it.
// If args[0] is a string, it treats the first two elements as
// a key-value pair.
// Otherwise, it treats args[0] as a value with a missing key.
func argsToAttr(args []any) (slog.Attr, []any) {
	switch x := args[0].(type) {
	case string:
		if len(args) == 1 {
			return slog.String(badKey, x), nil
		}
		return slog.Any(x, args[1]), args[2:]

	case slog.Attr:
		return x, args[1:]

	default:
		return slog.Any(badKey, x), args[1:]
	}
}

func argsToAttrSlice(args []any) []slog.Attr {
	var (
		attr  slog.Attr
		attrs []slog.Attr
	)
	for len(args) > 0 {
		attr, args = argsToAttr(args)
		attrs = append(attrs, attr)
	}
	return attrs
}

// WithAttrs returns a logger with the given attributes added to all messages.
// Returns a new Logger wrapping the result of slog.Logger.With().
func (l *Logger) With(args ...any) *Logger {
	handler := l.Handler().(*registryHandler)
	attrs := argsToAttrSlice(args)
	childHandler := handler.WithAttrs(attrs)
	childLogger := &Logger{
		Logger: slog.New(childHandler),
	}
	// Set up finalizer for automatic cleanup of derived logger
	setupLoggerFinalizer(childLogger)
	return childLogger
}

// WithGroup returns a logger with a group added to all messages.
// Returns a new Logger wrapping the result of slog.Logger.WithGroup().
func (l *Logger) WithGroup(name string) *Logger {
	handler := l.Handler().(*registryHandler)
	childHandler := handler.WithGroup(name)
	childLogger := &Logger{
		Logger: slog.New(childHandler),
	}
	// Set up finalizer for automatic cleanup of derived logger
	setupLoggerFinalizer(childLogger)
	return childLogger
}

// Errorf logs an error and returns it
// Format: Errorf(format, args...)
// Example: if err := doSomething(); err != nil { return logger.Errorf("failed: %w", err) }
// Note: Reports the caller of Errorf(), not Errorf() itself, via runtime.Caller()
// Supports %w directive for error wrapping like fmt.Errorf
func (l *Logger) Errorf(format string, args ...any) error {
	// Use fmt.Errorf to support %w directive for error wrapping
	err := fmt.Errorf(format, args...)
	// Log the error at the call site with correct PC
	l.logAtCallSite(1, slog.LevelError, err.Error())
	return err
}

// Panicf logs a message and panics
// Format: Panicf(format, args...)
// Example: if critical := check(); !critical { logger.Panicf("critical check failed: %v", reason) }
// Note: Reports the caller of Panicf(), not Panicf() itself, via runtime.Caller()
func (l *Logger) Panicf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	l.logAtCallSite(1, slog.LevelError, msg)
	panic(msg)
}

// logAtCallSite logs at the correct call site by skipping the specified number of stack frames.
// Follows the slog documentation pattern for wrapping output methods.
// skipFrames is the number of frames to skip (0 = caller of logAtCallSite, 1 = caller's caller, etc.)
func (l *Logger) logAtCallSite(skipFrames int, level slog.Level, msg string) {
	var pcs [1]uintptr
	// runtime.Callers skips frames: 0=logAtCallSite, 1=wrapper (Errorf/Panicf), 2+=user code
	// We skip 2+skipFrames to land on the actual caller
	runtime.Callers(2+skipFrames, pcs[:])

	r := slog.NewRecord(time.Now(), level, msg, pcs[0])
	_ = l.Handler().Handle(context.Background(), r)
}
