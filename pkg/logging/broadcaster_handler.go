package logging

import (
	"context"
	"log/slog"
)

// NewBroadcasterHandler creates a handler that broadcasts logs to RPC streams
func NewBroadcasterHandler(broadcaster *LogBroadcaster, opts *slog.HandlerOptions) slog.Handler {
	if opts == nil {
		opts = &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}
	}
	return &broadcasterHandler{
		broadcaster: broadcaster,
		opts:        opts,
	}
}

type broadcasterHandler struct {
	broadcaster *LogBroadcaster
	opts        *slog.HandlerOptions
}

func (h *broadcasterHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.opts.Level == nil || level >= h.opts.Level.Level()
}

func (h *broadcasterHandler) Handle(ctx context.Context, record slog.Record) error {
	if !h.Enabled(ctx, record.Level) {
		return nil
	}

	// Extract attributes and component name
	attrs := make(map[string]string)
	var loggerName string

	record.Attrs(func(attr slog.Attr) bool {
		// Component attribute is added by ComponentLogger
		if attr.Key == "component" {
			loggerName = attr.Value.String()
		}
		attrs[attr.Key] = attr.Value.String()
		return true
	})

	entry := &LogEntry{
		Timestamp:  record.Time.UnixNano(),
		LoggerName: loggerName,
		Level:      record.Level,
		Message:    record.Message,
		Attributes: attrs,
	}

	// Broadcast to all RPC stream subscribers
	h.broadcaster.Broadcast(entry)
	return nil
}

func (h *broadcasterHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h // No state to maintain
}

func (h *broadcasterHandler) WithGroup(name string) slog.Handler {
	return h // No state to maintain
}
