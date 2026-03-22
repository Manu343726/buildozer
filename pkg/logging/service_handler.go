package logging

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"connectrpc.com/connect"
	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	"github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1/protov1connect"
)

// loggingServiceHandler is the private implementation of LoggingServiceHandler
// It uses a ConfigManager to implement all logging configuration operations
type loggingServiceHandler struct {
	*Logger // Embed Logger for logging within the service handler

	configManager ConfigManager
}

// newLoggingServiceHandler creates a new logging service handler
// This is the private implementation that serves as the RPC handler
func newLoggingServiceHandler(configManager ConfigManager) *loggingServiceHandler {
	return &loggingServiceHandler{
		Logger:        Log().Child("LoggingServiceHandler"),
		configManager: configManager,
	}
}

// logRequesterInfo extracts and logs requester info fields
func (h *loggingServiceHandler) logRequesterInfo(msg string, info *v1.RequesterInfo) {
	if info == nil {
		h.Debug(msg)
		return
	}
	h.Debug(msg, "requester_id", info.RequesterId, "requester_type", info.RequesterType)
}

// GetLoggingStatus implements LoggingServiceHandler.GetLoggingStatus
func (h *loggingServiceHandler) GetLoggingStatus(ctx context.Context, req *connect.Request[v1.GetLoggingStatusRequest]) (*connect.Response[v1.GetLoggingStatusResponse], error) {
	h.logRequesterInfo("Received GetLoggingStatus request", req.Msg.RequesterInfo)

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
		sinkConfig := &v1.SinkConfig{
			Name:  sinkStatus.Name,
			Type:  sinkType,
			Level: SlogLevelToProtoLogLevel(sinkStatus.Level),
		}
		
		// Add file-specific configuration if this is a file sink
		if sinkType == v1.SinkType_SINK_TYPE_FILE && sinkStatus.Path != "" {
			sinkConfig.FileConfig = &v1.SinkConfig_FileConfig{
				Path:         sinkStatus.Path,
				MaxSizeBytes: sinkStatus.MaxSize,
				MaxBackups:   sinkStatus.MaxBackups,
				MaxAgeDays:   sinkStatus.MaxAgeDays,
				JsonFormat:   sinkStatus.JSONFormat,
			}
		}
		
		status.Sinks = append(status.Sinks, sinkConfig)
	}

	// Convert loggers
	for _, loggerStatus := range snapshot.Loggers {
		status.Loggers = append(status.Loggers, &v1.LoggerConfig{
			Name:      loggerStatus.Name,
			SinkNames: loggerStatus.SinkNames,
			Level:     SlogLevelToProtoLogLevel(loggerStatus.Level),
		})
	}

	// Convert active loggers
	for _, activeLogger := range snapshot.ActiveLoggers {
		status.ActiveLoggers = append(status.ActiveLoggers, &v1.ActiveLoggerInfo{
			Name:              activeLogger.Name,
			ResolvedConfigFor: activeLogger.ResolvedConfigFor,
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
	h.logRequesterInfo("Received SetGlobalLevel request", req.Msg.RequesterInfo)

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
	h.logRequesterInfo("Received SetLoggerLevel request", req.Msg.RequesterInfo)
	h.Debug("details", "logger_name", req.Msg.LoggerName)

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
	h.logRequesterInfo("Received SetSinkLevel request", req.Msg.RequesterInfo)
	h.Debug("details", "sink_name", req.Msg.SinkName)

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
	h.logRequesterInfo("Received EnableFileSink request", req.Msg.RequesterInfo)
	h.Debug("details", "logger_name", req.Msg.LoggerName, "file_path", req.Msg.FilePath)

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
	h.logRequesterInfo("Received DisableFileSink request", req.Msg.RequesterInfo)
	h.Debug("details", "logger_name", req.Msg.LoggerName)

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

// AddSink implements LoggingServiceHandler.AddSink
// Creates stdout or stderr sinks. File sinks must use EnableFileSink.
// Returns error if there's an overlap with an existing sink of the same type.
func (h *loggingServiceHandler) AddSink(ctx context.Context, req *connect.Request[v1.AddSinkRequest]) (*connect.Response[v1.AddSinkResponse], error) {
	h.logRequesterInfo("Received AddSink request", req.Msg.RequesterInfo)
	h.Debug("details", "sink_name", req.Msg.Name)

	sinkType := sinkTypeToString(req.Msg.Type)
	level := ProtoLogLevelToSlogLevel(req.Msg.Level)

	// Only allow stdout and stderr via RPC - file sinks must use EnableFileSink
	if sinkType != "stdout" && sinkType != "stderr" {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("invalid sink type %q; only 'stdout' and 'stderr' are supported via AddSink (use EnableFileSink for file sinks)", sinkType))
	}

	err := h.configManager.AddSink(ctx, req.Msg.Name, sinkType, level)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	return &connect.Response[v1.AddSinkResponse]{
		Msg: &v1.AddSinkResponse{
			Name:      req.Msg.Name,
			Type:      req.Msg.Type,
			Level:     req.Msg.Level,
			CreatedAt: timeNowTimestamp(),
			Message:   "Sink created successfully",
		},
	}, nil
}

// RemoveSink implements LoggingServiceHandler.RemoveSink
func (h *loggingServiceHandler) RemoveSink(ctx context.Context, req *connect.Request[v1.RemoveSinkRequest]) (*connect.Response[v1.RemoveSinkResponse], error) {
	h.logRequesterInfo("Received RemoveSink request", req.Msg.RequesterInfo)
	h.Debug("details", "sink_name", req.Msg.Name)

	err := h.configManager.RemoveSink(ctx, req.Msg.Name)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return &connect.Response[v1.RemoveSinkResponse]{
		Msg: &v1.RemoveSinkResponse{
			Name:            req.Msg.Name,
			LoggersAffected: 0, // TODO: Count affected loggers
			RemovedAt:       timeNowTimestamp(),
			Message:         "Sink removed successfully",
		},
	}, nil
}

// AddLogger implements LoggingServiceHandler.AddLogger
func (h *loggingServiceHandler) AddLogger(ctx context.Context, req *connect.Request[v1.AddLoggerRequest]) (*connect.Response[v1.AddLoggerResponse], error) {
	h.logRequesterInfo("Received AddLogger request", req.Msg.RequesterInfo)
	h.Debug("details", "logger_name", req.Msg.Name)

	level := ProtoLogLevelToSlogLevel(req.Msg.Level)

	err := h.configManager.AddLogger(ctx, req.Msg.Name, level, req.Msg.SinkNames)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return &connect.Response[v1.AddLoggerResponse]{
		Msg: &v1.AddLoggerResponse{
			Name:      req.Msg.Name,
			Level:     req.Msg.Level,
			SinkNames: req.Msg.SinkNames,
			CreatedAt: timeNowTimestamp(),
			Message:   "Logger created successfully",
		},
	}, nil
}

// RemoveLogger implements LoggingServiceHandler.RemoveLogger
func (h *loggingServiceHandler) RemoveLogger(ctx context.Context, req *connect.Request[v1.RemoveLoggerRequest]) (*connect.Response[v1.RemoveLoggerResponse], error) {
	h.logRequesterInfo("Received RemoveLogger request", req.Msg.RequesterInfo)
	h.Debug("details", "logger_name", req.Msg.Name)

	err := h.configManager.RemoveLogger(ctx, req.Msg.Name)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return &connect.Response[v1.RemoveLoggerResponse]{
		Msg: &v1.RemoveLoggerResponse{
			Name:      req.Msg.Name,
			RemovedAt: timeNowTimestamp(),
			Message:   "Logger removed successfully",
		},
	}, nil
}

// TailLogs implements LoggingServiceHandler.TailLogs
func (h *loggingServiceHandler) TailLogs(ctx context.Context, req *connect.Request[v1.TailLogsRequest], stream *connect.ServerStream[v1.TailLogsResponse]) error {
	h.logRequesterInfo("Received TailLogs request", req.Msg.RequesterInfo)

	// Convert proto log levels to slog levels
	logLevels := make([]interface{}, len(req.Msg.Levels))
	for i, level := range req.Msg.Levels {
		logLevels[i] = ProtoLogLevelToSlogLevel(level)
	}

	// Note: This is a placeholder as TailLogs requires log buffering infrastructure
	// In a real implementation, logs would be streamed from a circular buffer
	return connect.NewError(connect.CodeUnimplemented, nil)
}

// AttachSink implements LoggingServiceHandler.AttachSink
func (h *loggingServiceHandler) AttachSink(ctx context.Context, req *connect.Request[v1.AttachSinkRequest]) (*connect.Response[v1.AttachSinkResponse], error) {
	h.logRequesterInfo("Received AttachSink request", req.Msg.RequesterInfo)
	h.Debug("details", "logger_name", req.Msg.LoggerName, "sink_name", req.Msg.SinkName)

	err := h.configManager.AttachSink(ctx, req.Msg.LoggerName, req.Msg.SinkName)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Get updated logger config to return sink names
	snapshot, err := h.configManager.GetLoggingStatus(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var sinkNames []string
	for _, loggerStatus := range snapshot.Loggers {
		if loggerStatus.Name == req.Msg.LoggerName {
			sinkNames = loggerStatus.SinkNames
			break
		}
	}

	return &connect.Response[v1.AttachSinkResponse]{
		Msg: &v1.AttachSinkResponse{
			LoggerName: req.Msg.LoggerName,
			SinkName:   req.Msg.SinkName,
			SinkNames:  sinkNames,
			AttachedAt: timeNowTimestamp(),
			Message:    "Sink attached successfully",
		},
	}, nil
}

// DetachSink implements LoggingServiceHandler.DetachSink
func (h *loggingServiceHandler) DetachSink(ctx context.Context, req *connect.Request[v1.DetachSinkRequest]) (*connect.Response[v1.DetachSinkResponse], error) {
	h.logRequesterInfo("Received DetachSink request", req.Msg.RequesterInfo)
	h.Debug("details", "logger_name", req.Msg.LoggerName, "sink_name", req.Msg.SinkName)

	err := h.configManager.DetachSink(ctx, req.Msg.LoggerName, req.Msg.SinkName)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Get updated logger config to return remaining sink names
	snapshot, err := h.configManager.GetLoggingStatus(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var sinkNames []string
	for _, loggerStatus := range snapshot.Loggers {
		if loggerStatus.Name == req.Msg.LoggerName {
			sinkNames = loggerStatus.SinkNames
			break
		}
	}

	return &connect.Response[v1.DetachSinkResponse]{
		Msg: &v1.DetachSinkResponse{
			LoggerName: req.Msg.LoggerName,
			SinkName:   req.Msg.SinkName,
			SinkNames:  sinkNames,
			DetachedAt: timeNowTimestamp(),
			Message:    "Sink detached successfully",
		},
	}, nil
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
