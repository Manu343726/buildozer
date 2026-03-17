package logger

import (
	"context"
	"log/slog"
	"os"
)

// ComponentLogger wraps logging functionality with a component tag
type ComponentLogger struct {
	component string
}

// NewComponentLogger creates a new logger for a specific component
func NewComponentLogger(component string) *ComponentLogger {
	return &ComponentLogger{
		component: component,
	}
}

// InitDefault initializes the default slog logger with JSON output and level
func InitDefault(level slog.Level) {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})
	slog.SetDefault(slog.New(handler))
}

// InitText initializes the default slog logger with text output (for development)
func InitText(level slog.Level) {
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})
	slog.SetDefault(slog.New(handler))
}

// Debug logs a debug message with component tag
func (cl *ComponentLogger) Debug(msg string, args ...any) {
	slog.Default().Debug(msg, append([]any{"component", cl.component}, args...)...)
}

// DebugCtx logs a debug message with context and component tag
func (cl *ComponentLogger) DebugCtx(ctx context.Context, msg string, args ...any) {
	slog.Default().DebugContext(ctx, msg, append([]any{"component", cl.component}, args...)...)
}

// Info logs an info message with component tag
func (cl *ComponentLogger) Info(msg string, args ...any) {
	slog.Default().Info(msg, append([]any{"component", cl.component}, args...)...)
}

// InfoCtx logs an info message with context and component tag
func (cl *ComponentLogger) InfoCtx(ctx context.Context, msg string, args ...any) {
	slog.Default().InfoContext(ctx, msg, append([]any{"component", cl.component}, args...)...)
}

// Warn logs a warning message with component tag
func (cl *ComponentLogger) Warn(msg string, args ...any) {
	slog.Default().Warn(msg, append([]any{"component", cl.component}, args...)...)
}

// WarnCtx logs a warning message with context and component tag
func (cl *ComponentLogger) WarnCtx(ctx context.Context, msg string, args ...any) {
	slog.Default().WarnContext(ctx, msg, append([]any{"component", cl.component}, args...)...)
}

// Error logs an error message with component tag
func (cl *ComponentLogger) Error(msg string, args ...any) {
	slog.Default().Error(msg, append([]any{"component", cl.component}, args...)...)
}

// ErrorCtx logs an error message with context and component tag
func (cl *ComponentLogger) ErrorCtx(ctx context.Context, msg string, args ...any) {
	slog.Default().ErrorContext(ctx, msg, append([]any{"component", cl.component}, args...)...)
}

// Fatal logs a fatal message and exits with code 1
func (cl *ComponentLogger) Fatal(msg string, args ...any) {
	slog.Default().Error(msg, append([]any{"component", cl.component}, args...)...)
	os.Exit(1)
}

// FatalCtx logs a fatal message with context and exits with code 1
func (cl *ComponentLogger) FatalCtx(ctx context.Context, msg string, args ...any) {
	slog.Default().ErrorContext(ctx, msg, append([]any{"component", cl.component}, args...)...)
	os.Exit(1)
}

// WithAttrs returns a new logger with additional attributes
func (cl *ComponentLogger) WithAttrs(attrs ...slog.Attr) *ComponentLogger {
	// Note: With the new design, we always use slog.Default() so attributes
	// passed here would apply globally. This method now just returns a new
	// logger for the same component.
	return &ComponentLogger{
		component: cl.component,
	}
}

// WithGroup returns a new logger with a group name
func (cl *ComponentLogger) WithGroup(name string) *ComponentLogger {
	_ = slog.Default().WithGroup(name)
	return &ComponentLogger{
		component: cl.component,
	}
}
