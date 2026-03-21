package logging

import (
	"context"
	"net/http"
	"time"

	"connectrpc.com/connect"
	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	"github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1/protov1connect"
)

// loggingServiceHandler is the private implementation of LoggingServiceHandler
// It uses a ConfigManager to implement all logging configuration operations
type loggingServiceHandler struct {
	configManager ConfigManager
}

// newLoggingServiceHandler creates a new logging service handler
// This is the private implementation that serves as the RPC handler
func newLoggingServiceHandler(configManager ConfigManager) *loggingServiceHandler {
	return &loggingServiceHandler{
		configManager: configManager,
	}
}

// GetLoggingStatus implements LoggingServiceHandler.GetLoggingStatus
func (h *loggingServiceHandler) GetLoggingStatus(ctx context.Context, req *connect.Request[v1.GetLoggingStatusRequest]) (*connect.Response[v1.GetLoggingStatusResponse], error) {
	snapshot, err := h.configManager.GetLoggingStatus(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Convert snapshot to protobuf
	status := &v1.LoggingStatus{
		GlobalLevel: SlogLevelToProtoLogLevel(snapshot.GlobalLevel),
		RetrievedAt: timeToTimestamp(snapshot.RetrievedAt),
	}

	// Convert sinks
	for _, sinkStatus := range snapshot.Sinks {
		sinkType := sinkTypeFromString(sinkStatus.Type)
		status.Sinks = append(status.Sinks, &v1.SinkConfig{
			Name:  sinkStatus.Name,
			Type:  sinkType,
			Level: SlogLevelToProtoLogLevel(sinkStatus.Level),
		})
	}

	// Convert loggers
	for _, loggerStatus := range snapshot.Loggers {
		status.Loggers = append(status.Loggers, &v1.LoggerConfig{
			Name:      loggerStatus.Name,
			SinkNames: loggerStatus.SinkNames,
		})
	}

	return &connect.Response[v1.GetLoggingStatusResponse]{
		Msg: &v1.GetLoggingStatusResponse{
			Status: status,
		},
	}, nil
}

// SetGlobalLevel implements LoggingServiceHandler.SetGlobalLevel
func (h *loggingServiceHandler) SetGlobalLevel(ctx context.Context, req *connect.Request[v1.SetGlobalLevelRequest]) (*connect.Response[v1.SetGlobalLevelResponse], error) {
	level := ProtoLogLevelToSlogLevel(req.Msg.Level)

	if err := h.configManager.SetGlobalLevel(ctx, level); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return &connect.Response[v1.SetGlobalLevelResponse]{
		Msg: &v1.SetGlobalLevelResponse{
			Level:           req.Msg.Level,
			AffectedLoggers: 0, // TODO: Count affected loggers
			UpdatedAt:       timeNowTimestamp(),
		},
	}, nil
}

// SetLoggerLevel implements LoggingServiceHandler.SetLoggerLevel
func (h *loggingServiceHandler) SetLoggerLevel(ctx context.Context, req *connect.Request[v1.SetLoggerLevelRequest]) (*connect.Response[v1.SetLoggerLevelResponse], error) {
	level := ProtoLogLevelToSlogLevel(req.Msg.Level)

	err := h.configManager.SetLoggerLevel(ctx, req.Msg.LoggerName, level)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return &connect.Response[v1.SetLoggerLevelResponse]{
		Msg: &v1.SetLoggerLevelResponse{
			LoggerName: req.Msg.LoggerName,
			Level:      req.Msg.Level,
			UpdatedAt:  timeNowTimestamp(),
		},
	}, nil
}

// SetSinkLevel implements LoggingServiceHandler.SetSinkLevel
func (h *loggingServiceHandler) SetSinkLevel(ctx context.Context, req *connect.Request[v1.SetSinkLevelRequest]) (*connect.Response[v1.SetSinkLevelResponse], error) {
	level := ProtoLogLevelToSlogLevel(req.Msg.Level)

	err := h.configManager.SetSinkLevel(ctx, req.Msg.SinkName, level)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return &connect.Response[v1.SetSinkLevelResponse]{
		Msg: &v1.SetSinkLevelResponse{
			SinkName:  req.Msg.SinkName,
			Level:     req.Msg.Level,
			UpdatedAt: timeNowTimestamp(),
		},
	}, nil
}

// EnableFileSink implements LoggingServiceHandler.EnableFileSink
func (h *loggingServiceHandler) EnableFileSink(ctx context.Context, req *connect.Request[v1.EnableFileSinkRequest]) (*connect.Response[v1.EnableFileSinkResponse], error) {
	maxSizeMB := int(req.Msg.MaxSizeBytes / (1024 * 1024))
	if maxSizeMB == 0 {
		maxSizeMB = 100 // Default to 100MB
	}

	maxBackups := int(req.Msg.MaxBackups)
	if maxBackups == 0 {
		maxBackups = 10 // Default to 10 backups
	}

	maxAgeDays := int(req.Msg.MaxAgeDays)

	err := h.configManager.EnableFileSink(ctx, req.Msg.LoggerName, req.Msg.FilePath, maxSizeMB, maxBackups, maxAgeDays)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	sinkName := "file-" + req.Msg.LoggerName

	return &connect.Response[v1.EnableFileSinkResponse]{
		Msg: &v1.EnableFileSinkResponse{
			SinkName:   sinkName,
			LoggerName: req.Msg.LoggerName,
			FilePath:   req.Msg.FilePath,
			CreatedAt:  timeNowTimestamp(),
		},
	}, nil
}

// DisableFileSink implements LoggingServiceHandler.DisableFileSink
func (h *loggingServiceHandler) DisableFileSink(ctx context.Context, req *connect.Request[v1.DisableFileSinkRequest]) (*connect.Response[v1.DisableFileSinkResponse], error) {
	err := h.configManager.DisableFileSink(ctx, req.Msg.LoggerName)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	sinkName := "file-" + req.Msg.LoggerName

	return &connect.Response[v1.DisableFileSinkResponse]{
		Msg: &v1.DisableFileSinkResponse{
			LoggerName: req.Msg.LoggerName,
			SinkName:   sinkName,
			RemovedAt:  timeNowTimestamp(),
		},
	}, nil
}

// TailLogs implements LoggingServiceHandler.TailLogs
func (h *loggingServiceHandler) TailLogs(ctx context.Context, req *connect.Request[v1.TailLogsRequest], stream *connect.ServerStream[v1.TailLogsResponse]) error {
	// Convert proto log levels to slog levels
	logLevels := make([]interface{}, len(req.Msg.Levels))
	for i, level := range req.Msg.Levels {
		logLevels[i] = ProtoLogLevelToSlogLevel(level)
	}

	// Note: This is a placeholder as TailLogs requires log buffering infrastructure
	// In a real implementation, logs would be streamed from a circular buffer
	return connect.NewError(connect.CodeUnimplemented, nil)
}

// Helper functions

// sinkTypeFromString converts string sink type to protobuf enum
func sinkTypeFromString(typeStr string) v1.SinkType {
	switch typeStr {
	case "stdout":
		return v1.SinkType_SINK_TYPE_STDOUT
	case "stderr":
		return v1.SinkType_SINK_TYPE_STDERR
	case "file":
		return v1.SinkType_SINK_TYPE_FILE
	case "syslog":
		return v1.SinkType_SINK_TYPE_SYSLOG
	default:
		return v1.SinkType_SINK_TYPE_UNSPECIFIED
	}
}

// timeToTimestamp converts time.Time to protobuf Timestamp
func timeToTimestamp(t time.Time) *v1.TimeStamp {
	return &v1.TimeStamp{
		UnixMillis: t.UnixMilli(),
	}
}

// timeNowTimestamp returns current time as protobuf Timestamp
func timeNowTimestamp() *v1.TimeStamp {
	return timeToTimestamp(time.Now())
}

// RegisterLoggingService creates and registers a Connect handler for LoggingService
// Returns the path and HTTP handler to mount
func RegisterLoggingService(configManager ConfigManager) (string, http.Handler) {
	handler := newLoggingServiceHandler(configManager)
	path, mux := protov1connect.NewLoggingServiceHandler(handler)
	return path, mux
}
