package logger

import (
	"context"
	"log/slog"
	"runtime"
	"time"
)

// ComponentLogger wraps slog and adds a 'component' tag to all logs
type ComponentLogger struct {
	component string
	handler   slog.Handler
}

// NewComponentLogger creates a new logger for a specific component
func NewComponentLogger(componentName string) *ComponentLogger {
	return &ComponentLogger{
		component: componentName,
		handler:   slog.Default().Handler(),
	}
}

// NewComponentLoggerWithHandler creates a logger with a specific handler
func NewComponentLoggerWithHandler(componentName string, handler slog.Handler) *ComponentLogger {
	return &ComponentLogger{
		component: componentName,
		handler:   handler,
	}
}

// Debug logs a debug message
func (l *ComponentLogger) Debug(msg string, args ...any) {
	l.logAtCallSite(1, slog.LevelDebug, msg, args...)
}

// DebugCtx logs a debug message with context
func (l *ComponentLogger) DebugCtx(ctx context.Context, msg string, args ...any) {
	l.logAtCallSiteCtx(ctx, 1, slog.LevelDebug, msg, args...)
}

// Info logs an info message
func (l *ComponentLogger) Info(msg string, args ...any) {
	l.logAtCallSite(1, slog.LevelInfo, msg, args...)
}

// InfoCtx logs an info message with context
func (l *ComponentLogger) InfoCtx(ctx context.Context, msg string, args ...any) {
	l.logAtCallSiteCtx(ctx, 1, slog.LevelInfo, msg, args...)
}

// Warn logs a warning message
func (l *ComponentLogger) Warn(msg string, args ...any) {
	l.logAtCallSite(1, slog.LevelWarn, msg, args...)
}

// WarnCtx logs a warning message with context
func (l *ComponentLogger) WarnCtx(ctx context.Context, msg string, args ...any) {
	l.logAtCallSiteCtx(ctx, 1, slog.LevelWarn, msg, args...)
}

// Error logs an error message
func (l *ComponentLogger) Error(msg string, args ...any) {
	l.logAtCallSite(1, slog.LevelError, msg, args...)
}

// ErrorCtx logs an error message with context
func (l *ComponentLogger) ErrorCtx(ctx context.Context, msg string, args ...any) {
	l.logAtCallSiteCtx(ctx, 1, slog.LevelError, msg, args...)
}

// Fatal logs a fatal message and panics
func (l *ComponentLogger) Fatal(msg string, args ...any) {
	l.logAtCallSite(1, slog.LevelError, msg, args...)
	panic(msg)
}

// FatalCtx logs a fatal message with context and panics
func (l *ComponentLogger) FatalCtx(ctx context.Context, msg string, args ...any) {
	l.logAtCallSiteCtx(ctx, 1, slog.LevelError, msg, args...)
	panic(msg)
}

// Errorf logs a formatted error message and returns an error
func (l *ComponentLogger) Errorf(msg string, args ...any) error {
	l.logAtCallSite(1, slog.LevelError, msg, args...)
	return &wrappedError{message: msg}
}

// ErrorfCtx logs a formatted error message with context and returns an error
func (l *ComponentLogger) ErrorfCtx(ctx context.Context, msg string, args ...any) error {
	l.logAtCallSiteCtx(ctx, 1, slog.LevelError, msg, args...)
	return &wrappedError{message: msg}
}

// Panicf logs a formatted message and panics
func (l *ComponentLogger) Panicf(msg string, args ...any) {
	l.logAtCallSite(1, slog.LevelError, msg, args...)
	panic(msg)
}

// PanicfCtx logs a formatted message with context and panics
func (l *ComponentLogger) PanicfCtx(ctx context.Context, msg string, args ...any) {
	l.logAtCallSiteCtx(ctx, 1, slog.LevelError, msg, args...)
	panic(msg)
}

// logAtCallSite logs with the actual caller's location
func (l *ComponentLogger) logAtCallSite(skipFrames int, level slog.Level, msg string, args ...any) {
	if !l.handler.Enabled(context.Background(), level) {
		return
	}

	var pcs [1]uintptr
	runtime.Callers(2+skipFrames, pcs[:])
	r := slog.NewRecord(time.Now(), level, msg, pcs[0])

	// Add component attribute
	r.AddAttrs(slog.String("component", l.component))

	// Add other attributes
	if len(args) > 0 {
		r.AddAttrs(slog.Group("", args...))
	}

	l.handler.Handle(context.Background(), r)
}

// logAtCallSiteCtx logs with context and the actual caller's location
func (l *ComponentLogger) logAtCallSiteCtx(ctx context.Context, skipFrames int, level slog.Level, msg string, args ...any) {
	if !l.handler.Enabled(ctx, level) {
		return
	}

	var pcs [1]uintptr
	runtime.Callers(2+skipFrames, pcs[:])
	r := slog.NewRecord(time.Now(), level, msg, pcs[0])

	// Add component attribute
	r.AddAttrs(slog.String("component", l.component))

	// Add other attributes
	if len(args) > 0 {
		r.AddAttrs(slog.Group("", args...))
	}

	l.handler.Handle(ctx, r)
}

// InitDefault initializes the default slog handler
func InitDefault() {
	// Initialize slog with default settings
	// This is called at daemon startup
}

// InitText initializes slog with text output
func InitText() {
	// Initialize slog with text format
	// This is called at daemon startup
}

// wrappedError is a simple error wrapper for Errorf
type wrappedError struct {
	message string
}

func (e *wrappedError) Error() string {
	return e.message
}
