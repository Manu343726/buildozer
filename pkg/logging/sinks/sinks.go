package sinks

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/natefinch/lumberjack.v2"
)

// ANSI color codes for log levels and components
const (
	colorBlue    = "\033[34m"
	colorGreen   = "\033[32m"
	colorYellow  = "\033[33m"
	colorRed     = "\033[31m"
	colorCyan    = "\033[36m" // For timestamp and logger name
	colorMagenta = "\033[35m" // For attributes
	colorReset   = "\033[0m"
)

// ColorMode controls whether log output includes ANSI color codes
type ColorMode int

const (
	ColorModeDisabled ColorMode = iota
	ColorModeEnabled
)

// StdoutSink creates a handler that writes to stdout with colored output
func StdoutSink(opts *slog.HandlerOptions) slog.Handler {
	return StdoutSinkWithOmitLogger(opts, false)
}

// StdoutSinkWithOmitLogger creates a handler that writes to stdout with optional omit logger name behavior
func StdoutSinkWithOmitLogger(opts *slog.HandlerOptions, omitLoggerNameIfSourceEnabled bool) slog.Handler {
	if opts == nil {
		opts = &slog.HandlerOptions{
			Level:     slog.LevelInfo,
			AddSource: true, // Include source location by default
		}
	} else if !opts.AddSource {
		opts.AddSource = true // Always enable AddSource for stdout sink
	}
	return NewColoredTextHandlerWithOmitLogger(os.Stdout, opts, ColorModeEnabled, omitLoggerNameIfSourceEnabled)
}

// StderrSink creates a handler that writes to stderr with colored output
func StderrSink(opts *slog.HandlerOptions) slog.Handler {
	return StderrSinkWithOmitLogger(opts, false)
}

// StderrSinkWithOmitLogger creates a handler that writes to stderr with optional omit logger name behavior
func StderrSinkWithOmitLogger(opts *slog.HandlerOptions, omitLoggerNameIfSourceEnabled bool) slog.Handler {
	if opts == nil {
		opts = &slog.HandlerOptions{
			Level:     slog.LevelWarn,
			AddSource: true, // Include source location by default
		}
	} else if !opts.AddSource {
		opts.AddSource = true // Always enable AddSource for stderr sink
	}
	return NewColoredTextHandlerWithOmitLogger(os.Stderr, opts, ColorModeEnabled, omitLoggerNameIfSourceEnabled)
}

// FileSinkConfig holds configuration for file sinks
type FileSinkConfig struct {
	Path                          string // File path
	MaxSizeB                      int64  // Max file size in bytes (default: 100MB)
	MaxFiles                      int    // Max number of rotated files (default: 5)
	MaxAgeDays                    int    // Max age of log files in days (default: 0 = no limit)
	JSONFormat                    bool   // Use JSON format instead of text
	IncludeSourceLocation         bool   // Include source location in logs (file:line)
	OmitLoggerNameIfSourceEnabled bool   // Omit logger name if source location is enabled
	HandlerOpts                   *slog.HandlerOptions
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
		// File output doesn't use colors
		handler = NewColoredTextHandlerWithOmitLogger(lumber, handlerOpts, ColorModeDisabled, config.OmitLoggerNameIfSourceEnabled)
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

// ============ Colored Text Handler ============

// ColoredTextHandler writes log entries as formatted text with optional colored level output.
// Format: [timestamp][level][logger][source] message attribute=value attribute2=value2 ...
// Timestamp format: dd/mm/yy hh:mm:ss.ms
// Level colors: DEBUG=blue, INFO=green, WARN=yellow, ERROR=red
// Source location (file:line) in green, printed only if AddSource is enabled in HandlerOptions
type ColoredTextHandler struct {
	underlying                    io.Writer
	opts                          *slog.HandlerOptions
	attrs                         []slog.Attr // Accumulated attributes
	group                         string      // Current group
	colorMode                     ColorMode   // Whether to colorize level output
	omitLoggerNameIfSourceEnabled bool        // Omit logger name if source location is enabled
}

// NewColoredTextHandler creates a new handler with formatted output and optional coloring
func NewColoredTextHandler(w io.Writer, opts *slog.HandlerOptions, colorMode ColorMode) *ColoredTextHandler {
	return NewColoredTextHandlerWithOmitLogger(w, opts, colorMode, false)
}

// NewColoredTextHandlerWithOmitLogger creates a new handler with optional omitLoggerName behavior
func NewColoredTextHandlerWithOmitLogger(w io.Writer, opts *slog.HandlerOptions, colorMode ColorMode, omitLoggerNameIfSourceEnabled bool) *ColoredTextHandler {
	if opts == nil {
		opts = &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}
	}
	return &ColoredTextHandler{
		underlying:                    w,
		opts:                          opts,
		attrs:                         []slog.Attr{},
		colorMode:                     colorMode,
		omitLoggerNameIfSourceEnabled: omitLoggerNameIfSourceEnabled,
	}
}

// getLevelColor returns the ANSI color code for the given log level
func getLevelColor(level slog.Level) string {
	switch {
	case level < slog.LevelInfo: // DEBUG
		return colorBlue
	case level < slog.LevelWarn: // INFO
		return colorGreen
	case level < slog.LevelError: // WARN
		return colorYellow
	default: // ERROR and above
		return colorRed
	}
}

// getLevelString returns the uppercase string representation of the log level
func getLevelString(level slog.Level) string {
	switch {
	case level < slog.LevelInfo:
		return "DEBUG"
	case level < slog.LevelWarn:
		return "INFO"
	case level < slog.LevelError:
		return "WARN"
	default:
		return "ERROR"
	}
}

// getRelativeSourcePath returns a relative source path based on buildozer directory structure
// Returns paths like: pkg/drivers/cpp/job.go, internal/gen/proto.go, cmd/buildozer-client/main.go
func getRelativeSourcePath(fullPath string) string {
	// Check for /pkg/ directory (public packages)
	if idx := strings.Index(fullPath, "/pkg/"); idx >= 0 {
		return fullPath[idx+1:] // +1 to skip the leading /
	}
	// Check for /internal/ directory (private packages)
	if idx := strings.Index(fullPath, "/internal/"); idx >= 0 {
		return fullPath[idx+1:]
	}
	// Check for /cmd/ directory (CLI tools)
	if idx := strings.Index(fullPath, "/cmd/"); idx >= 0 {
		return fullPath[idx+1:]
	}
	// Fallback to just the filename if not found in standard locations
	return filepath.Base(fullPath)
}

func (h *ColoredTextHandler) Handle(ctx context.Context, record slog.Record) error {
	// Check if should log at this level
	if h.opts.Level != nil && record.Level < h.opts.Level.Level() {
		return nil
	}

	// Collect all attributes: accumulated + record attributes
	var loggerName string
	var sourceLocation string
	var otherAttrs []slog.Attr

	// Add accumulated attributes first
	otherAttrs = append(otherAttrs, h.attrs...)

	// Extract source location from record PC if AddSource is enabled
	if h.opts != nil && h.opts.AddSource && record.PC != 0 {
		fs := runtime.CallersFrames([]uintptr{record.PC})
		f, _ := fs.Next()
		if f.File != "" {
			relativePath := getRelativeSourcePath(f.File)
			sourceLocation = fmt.Sprintf("%s:%d", relativePath, f.Line)
		}
	}

	// Add record attributes
	record.Attrs(func(a slog.Attr) bool {
		if a.Key == "logger" {
			loggerName = a.Value.String()
		} else {
			otherAttrs = append(otherAttrs, a)
		}
		return true
	})

	// Format timestamp as dd/mm/yy hh:mm:ss.ms
	timestamp := record.Time.Format("02/01/06 15:04:05.000")

	// Get level string and color
	levelStr := getLevelString(record.Level)
	levelColor := ""
	timestampColor := ""
	sourceColor := ""
	attrColor := ""
	resetColor := ""
	if h.colorMode == ColorModeEnabled {
		levelColor = getLevelColor(record.Level)
		timestampColor = colorCyan
		sourceColor = colorGreen // Source location in green
		attrColor = colorMagenta
		resetColor = colorReset
	}

	// Build output with format: [timestamp][level][logger][source] message attr=value ...
	// Timestamp and logger name in cyan, level in its own color, source location in green, attributes in magenta
	var output string
	if h.colorMode == ColorModeEnabled {
		output = fmt.Sprintf("[%s%s%s][%s%s%s]", timestampColor, timestamp, resetColor, levelColor, levelStr, resetColor)
	} else {
		output = fmt.Sprintf("[%s][%s]", timestamp, levelStr)
	}

	// Add logger name if present (colored in cyan like timestamp)
	// Skip if omitLoggerNameIfSourceEnabled is true and source location is present
	if loggerName != "" {
		shouldOmit := h.omitLoggerNameIfSourceEnabled && sourceLocation != ""
		if !shouldOmit {
			if h.colorMode == ColorModeEnabled {
				output += fmt.Sprintf("[%s%s%s]", timestampColor, loggerName, resetColor)
			} else {
				output += fmt.Sprintf("[%s]", loggerName)
			}
		}
	}

	// Add source location if present (colored in green)
	if sourceLocation != "" {
		if h.colorMode == ColorModeEnabled {
			output += fmt.Sprintf("[%s%s%s]", sourceColor, sourceLocation, resetColor)
		} else {
			output += fmt.Sprintf("[%s]", sourceLocation)
		}
	}

	// Add message
	output += fmt.Sprintf(" %s", record.Message)

	// Add attributes (colored in magenta)
	for _, attr := range otherAttrs {
		if h.colorMode == ColorModeEnabled {
			output += fmt.Sprintf(" %s%s=%v%s", attrColor, attr.Key, attr.Value.Any(), resetColor)
		} else {
			output += fmt.Sprintf(" %s=%v", attr.Key, attr.Value.Any())
		}
	}

	output += "\n"
	_, err := io.WriteString(h.underlying, output)
	return err
}

func (h *ColoredTextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	// Create new handler with accumulated attributes
	newAttrs := append([]slog.Attr{}, h.attrs...)
	newAttrs = append(newAttrs, attrs...)
	return &ColoredTextHandler{
		underlying:                    h.underlying,
		opts:                          h.opts,
		attrs:                         newAttrs,
		group:                         h.group,
		colorMode:                     h.colorMode,
		omitLoggerNameIfSourceEnabled: h.omitLoggerNameIfSourceEnabled,
	}
}

func (h *ColoredTextHandler) WithGroup(name string) slog.Handler {
	return &ColoredTextHandler{
		underlying:                    h.underlying,
		opts:                          h.opts,
		attrs:                         h.attrs,
		group:                         name,
		colorMode:                     h.colorMode,
		omitLoggerNameIfSourceEnabled: h.omitLoggerNameIfSourceEnabled,
	}
}

func (h *ColoredTextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.opts.Level == nil || level >= h.opts.Level.Level()
}
