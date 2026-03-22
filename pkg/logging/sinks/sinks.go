package sinks

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
)

// StdoutSink creates a handler that writes to stdout
func StdoutSink(opts *slog.HandlerOptions) slog.Handler {
	if opts == nil {
		opts = &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}
	}
	return NewOrderedTextHandler(os.Stdout, opts)
}

// StderrSink creates a handler that writes to stderr
func StderrSink(opts *slog.HandlerOptions) slog.Handler {
	if opts == nil {
		opts = &slog.HandlerOptions{
			Level: slog.LevelWarn,
		}
	}
	return NewOrderedTextHandler(os.Stderr, opts)
}

// FileSinkConfig holds configuration for file sinks
type FileSinkConfig struct {
	Path        string // File path
	MaxSizeB    int64  // Max file size in bytes (default: 100MB)
	MaxFiles    int    // Max number of rotated files (default: 5)
	MaxAgeDays  int    // Max age of log files in days (default: 0 = no limit)
	JSONFormat  bool   // Use JSON format instead of text
	HandlerOpts *slog.HandlerOptions
}

// FileSink creates a rotating file sink using lumberjack
func FileSink(config FileSinkConfig) (slog.Handler, error) {
	// Create parent directory if needed
	dir := ""
	for i := len(config.Path) - 1; i >= 0; i-- {
		if config.Path[i] == '/' {
			dir = config.Path[:i]
			break
		}
	}
	if dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create log directory: %w", err)
		}
	}

	// Set defaults
	maxSizeMB := 100
	maxBackups := 5
	maxAgeDays := 0
	if config.MaxSizeB > 0 {
		maxSizeMB = int(config.MaxSizeB / (1024 * 1024))
	}
	if config.MaxFiles > 0 {
		maxBackups = config.MaxFiles
	}
	if config.MaxAgeDays > 0 {
		maxAgeDays = config.MaxAgeDays
	}

	// Create lumberjack logger for file rotation
	lumber := &lumberjack.Logger{
		Filename:   config.Path,
		MaxSize:    maxSizeMB,
		MaxBackups: maxBackups,
		MaxAge:     maxAgeDays,
		Compress:   false,
	}

	// Create handler options if not provided
	handlerOpts := config.HandlerOpts
	if handlerOpts == nil {
		handlerOpts = &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}
	}

	// Create slog handler with lumberjack writer
	var handler slog.Handler
	if config.JSONFormat {
		handler = slog.NewJSONHandler(lumber, handlerOpts)
	} else {
		handler = NewOrderedTextHandler(lumber, handlerOpts)
	}

	return handler, nil
}

// JSONFileSink creates a JSON file sink
// maxSizeMB: maximum file size in MB before rotation (default: 100)
// maxAgeDays: maximum age of log files in days before cleanup (0 = disabled)
func JSONFileSink(path string, maxSizeMB int, maxAgeDays int) (slog.Handler, error) {
	return FileSink(FileSinkConfig{
		Path:       path,
		MaxSizeB:   int64(maxSizeMB) * 1024 * 1024,
		MaxFiles:   5,
		MaxAgeDays: maxAgeDays,
		JSONFormat: true,
		HandlerOpts: &slog.HandlerOptions{
			Level:     slog.LevelDebug,
			AddSource: true,
		},
	})
}

// TextFileSink creates a text file sink
// maxSizeMB: maximum file size in MB before rotation (default: 100)
// maxAgeDays: maximum age of log files in days before cleanup (0 = disabled)
func TextFileSink(path string, maxSizeMB int, maxAgeDays int) (slog.Handler, error) {
	return FileSink(FileSinkConfig{
		Path:       path,
		MaxSizeB:   int64(maxSizeMB) * 1024 * 1024,
		MaxFiles:   5,
		MaxAgeDays: maxAgeDays,
		JSONFormat: false,
		HandlerOpts: &slog.HandlerOptions{
			Level:     slog.LevelDebug,
			AddSource: true,
		},
	})
}

// ============ Ordered Text Handler ============

// OrderedTextHandler wraps a TextHandler and reorders attributes to:
// time, level, logger, msg, other attributes
type OrderedTextHandler struct {
	underlying io.Writer
	opts       *slog.HandlerOptions
	attrs      []slog.Attr // Accumulated attributes
	group      string      // Current group
}

// NewOrderedTextHandler creates a new handler with custom attribute ordering
func NewOrderedTextHandler(w io.Writer, opts *slog.HandlerOptions) *OrderedTextHandler {
	if opts == nil {
		opts = &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}
	}
	return &OrderedTextHandler{
		underlying: w,
		opts:       opts,
		attrs:      []slog.Attr{},
	}
}

func (h *OrderedTextHandler) Handle(ctx context.Context, record slog.Record) error {
	// Check if should log at this level
	if h.opts.Level != nil && record.Level < h.opts.Level.Level() {
		return nil
	}

	// Collect all attributes: accumulated + record attributes
	var loggerName string
	var otherAttrs []slog.Attr

	// Add accumulated attributes first
	otherAttrs = append(otherAttrs, h.attrs...)

	// Add record attributes
	record.Attrs(func(a slog.Attr) bool {
		if a.Key == "logger" {
			loggerName = a.Value.String()
		} else {
			otherAttrs = append(otherAttrs, a)
		}
		return true
	})

	// Build output with desired order: time, level, logger, msg, other attributes
	output := fmt.Sprintf("time=%s level=%s", record.Time.Format(time.RFC3339Nano), record.Level)

	if loggerName != "" {
		output += fmt.Sprintf(" logger=%s", loggerName)
	}

	output += fmt.Sprintf(" msg=%q", record.Message)

	for _, attr := range otherAttrs {
		output += fmt.Sprintf(" %s=%v", attr.Key, attr.Value.Any())
	}

	output += "\n"
	_, err := io.WriteString(h.underlying, output)
	return err
}

func (h *OrderedTextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	// Create new handler with accumulated attributes
	newAttrs := append([]slog.Attr{}, h.attrs...)
	newAttrs = append(newAttrs, attrs...)
	return &OrderedTextHandler{
		underlying: h.underlying,
		opts:       h.opts,
		attrs:      newAttrs,
		group:      h.group,
	}
}

func (h *OrderedTextHandler) WithGroup(name string) slog.Handler {
	return &OrderedTextHandler{
		underlying: h.underlying,
		opts:       h.opts,
		attrs:      h.attrs,
		group:      name,
	}
}

func (h *OrderedTextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.opts.Level == nil || level >= h.opts.Level.Level()
}
