package sinks

import (
	"fmt"
	"log/slog"
	"os"

	"gopkg.in/natefinch/lumberjack.v2"
)

// StdoutSink creates a handler that writes to stdout
func StdoutSink(opts *slog.HandlerOptions) slog.Handler {
	if opts == nil {
		opts = &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}
	}
	return slog.NewTextHandler(os.Stdout, opts)
}

// StderrSink creates a handler that writes to stderr
func StderrSink(opts *slog.HandlerOptions) slog.Handler {
	if opts == nil {
		opts = &slog.HandlerOptions{
			Level: slog.LevelWarn,
		}
	}
	return slog.NewTextHandler(os.Stderr, opts)
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
		handler = slog.NewTextHandler(lumber, handlerOpts)
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
	})
}
