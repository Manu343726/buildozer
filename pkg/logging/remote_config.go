package logging

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"connectrpc.com/connect"
	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	"github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1/protov1connect"
)

// RemoteConfigManager implements ConfigManager for remote daemon communication
// It uses a Connect client to communicate with a remote LoggingService
type RemoteConfigManager struct {
	*Logger // Embed Logger for logging within the config manager

	client protov1connect.LoggingServiceClient
}

// NewRemoteConfigManager creates a new remote configuration manager
// It requires an HTTP client and the base URL of the remote daemon
func NewRemoteConfigManager(host string, port int) *RemoteConfigManager {
	baseURL := fmt.Sprintf("http://%s:%d", host, port)

	return &RemoteConfigManager{
		Logger: Log().Child("RemoteConfigManager").With("remoteHost", host, "remotePort", port),
		client: protov1connect.NewLoggingServiceClient(http.DefaultClient, baseURL),
	}
}

// NewRemoteConfigManagerWithClient creates a new remote configuration manager with an existing client
func NewRemoteConfigManagerWithClient(client protov1connect.LoggingServiceClient) *RemoteConfigManager {
	return &RemoteConfigManager{
		Logger: Log().Child("RemoteConfigManager"),
		client: client,
	}
}

// newRequesterInfo creates RequesterInfo for RPC requests
func (m *RemoteConfigManager) newRequesterInfo() *v1.RequesterInfo {
	minor := uint32(1)
	patch := uint32(0)
	return &v1.RequesterInfo{
		RequesterId:   "buildozer-client",
		RequesterType: "cli",
		RequesterVersion: &v1.Version{
			Major: 0,
			Minor: &minor,
			Patch: &patch,
		},
		RequestTimestamp: &v1.TimeStamp{
			UnixMillis: time.Now().UnixMilli(),
		},
	}
}

// GetLoggingStatus returns the current logging configuration from the remote daemon
func (m *RemoteConfigManager) GetLoggingStatus(ctx context.Context) (*LoggingStatusSnapshot, error) {
	m.Debug("Fetching logging status from remote daemon")

	resp, err := m.client.GetLoggingStatus(ctx, &connect.Request[v1.GetLoggingStatusRequest]{
		Msg: &v1.GetLoggingStatusRequest{
			RequesterInfo: m.newRequesterInfo(),
		},
	})
	if err != nil {
		return nil, m.Errorf("failed to get logging status: %w", err)
	}

	m.Debug("Received logging status response", "response", resp.Msg)

	status := resp.Msg.Status
	snapshot := &LoggingStatusSnapshot{
		GlobalLevel:   ProtoLogLevelToSlogLevel(status.GlobalLevel),
		Sinks:         []SinkStatus{},
		Loggers:       []LoggerStatus{},
		ActiveLoggers: []ActiveLoggerInfo{},
	}

	if status.RetrievedAt != nil {
		snapshot.RetrievedAt = timestampToTime(status.RetrievedAt)
	}

	// Convert sinks
	for _, sink := range status.Sinks {
		sinkStatus := SinkStatus{
			Name:                  sink.Name,
			Type:                  sinkTypeToString(sink.Type),
			Level:                 ProtoLogLevelToSlogLevel(sink.Level),
			IncludeSourceLocation: sink.IncludeSourceLocation,
		}

		// Extract file-specific configuration if present
		if sink.FileConfig != nil {
			sinkStatus.Path = sink.FileConfig.Path
			sinkStatus.MaxSize = sink.FileConfig.MaxSizeBytes
			sinkStatus.MaxBackups = sink.FileConfig.MaxBackups
			sinkStatus.MaxAgeDays = sink.FileConfig.MaxAgeDays
			sinkStatus.JSONFormat = sink.FileConfig.JsonFormat
		}

		snapshot.Sinks = append(snapshot.Sinks, sinkStatus)
	}

	// Convert loggers
	for _, logger := range status.Loggers {
		snapshot.Loggers = append(snapshot.Loggers, LoggerStatus{
			Name:      logger.Name,
			SinkNames: logger.SinkNames,
			Level:     ProtoLogLevelToSlogLevel(logger.Level),
		})
	}

	// Convert active loggers
	for _, activeLogger := range status.ActiveLoggers {
		snapshot.ActiveLoggers = append(snapshot.ActiveLoggers, ActiveLoggerInfo{
			Name:              activeLogger.Name,
			ResolvedConfigFor: activeLogger.ResolvedConfigFor,
		})
	}

	return snapshot, nil
}

// SetGlobalLevel changes the global logging level on the remote daemon
func (m *RemoteConfigManager) SetGlobalLevel(ctx context.Context, level slog.Level) error {
	m.Debug("Setting global logging level", "level", level.String())

	response, err := m.client.SetGlobalLevel(ctx, &connect.Request[v1.SetGlobalLevelRequest]{
		Msg: &v1.SetGlobalLevelRequest{
			Level:         SlogLevelToProtoLogLevel(level),
			RequesterInfo: m.newRequesterInfo(),
		},
	})
	if err != nil {
		return m.Errorf("failed to set global level: %w", err)
	}

	m.Debug("SetGlobalLevel response", "response", response.Msg)
	return nil
}

// SetLoggerLevel changes the level for a logger on the remote daemon
func (m *RemoteConfigManager) SetLoggerLevel(ctx context.Context, loggerName string, level slog.Level) error {
	m.Debug("Setting logger level", "loggerName", loggerName, "level", level.String())

	response, err := m.client.SetLoggerLevel(ctx, &connect.Request[v1.SetLoggerLevelRequest]{
		Msg: &v1.SetLoggerLevelRequest{
			LoggerName:    loggerName,
			Level:         SlogLevelToProtoLogLevel(level),
			RequesterInfo: m.newRequesterInfo(),
		},
	})
	if err != nil {
		return m.Errorf("failed to set logger level: %w", err)
	}

	m.Debug("SetLoggerLevel response", "response", response.Msg)
	return nil
}

// SetSinkLevel changes the level for a sink on the remote daemon
func (m *RemoteConfigManager) SetSinkLevel(ctx context.Context, sinkName string, level slog.Level) error {
	m.Debug("Setting sink level", "sinkName", sinkName, "level", level.String())

	response, err := m.client.SetSinkLevel(ctx, &connect.Request[v1.SetSinkLevelRequest]{
		Msg: &v1.SetSinkLevelRequest{
			SinkName:      sinkName,
			Level:         SlogLevelToProtoLogLevel(level),
			RequesterInfo: m.newRequesterInfo(),
		},
	})
	if err != nil {
		return m.Errorf("failed to set sink level: %w", err)
	}

	m.Debug("SetSinkLevel response", "response", response.Msg)
	return nil
}

// UpdateSinkConfig updates configuration for an existing sink
func (m *RemoteConfigManager) UpdateSinkConfig(ctx context.Context, sinkName string, changes SinkConfigChange) error {
	m.Debug("Updating sink config", "sinkName", sinkName)

	msg := &v1.UpdateSinkConfigRequest{
		SinkName:      sinkName,
		RequesterInfo: m.newRequesterInfo(),
	}

	if changes.Level != nil {
		protoLevel := SlogLevelToProtoLogLevel(*changes.Level)
		msg.Level = &protoLevel
	}

	if changes.IncludeSourceLocation != nil {
		msg.IncludeSourceLocation = changes.IncludeSourceLocation
	}

	if changes.MaxSizeBytes != nil && *changes.MaxSizeBytes > 0 {
		msg.MaxSizeBytes = changes.MaxSizeBytes
	}

	if changes.MaxAgeDays != nil && *changes.MaxAgeDays > 0 {
		msg.MaxAgeDays = changes.MaxAgeDays
	}

	response, err := m.client.UpdateSinkConfig(ctx, &connect.Request[v1.UpdateSinkConfigRequest]{
		Msg: msg,
	})
	if err != nil {
		return m.Errorf("failed to update sink config: %w", err)
	}

	m.Debug("UpdateSinkConfig response", "response", response.Msg)
	return nil
}

// EnableFileSink enables a file sink on the remote daemon
func (m *RemoteConfigManager) EnableFileSink(ctx context.Context, loggerName, filePath string, maxSizeMB int, maxBackups int, maxAgeDays int) error {
	m.Debug("Enabling file sink", "loggerName", loggerName, "filePath", filePath, "maxSizeMB", maxSizeMB, "maxBackups", maxBackups, "maxAgeDays", maxAgeDays)

	respone, err := m.client.EnableFileSink(ctx, &connect.Request[v1.EnableFileSinkRequest]{
		Msg: &v1.EnableFileSinkRequest{
			LoggerName:    loggerName,
			FilePath:      filePath,
			MaxSizeBytes:  int64(maxSizeMB) * 1024 * 1024,
			MaxBackups:    int32(maxBackups),
			MaxAgeDays:    int32(maxAgeDays),
			JsonFormat:    false,
			RequesterInfo: m.newRequesterInfo(),
		},
	})
	if err != nil {
		return m.Errorf("failed to enable file sink: %w", err)
	}

	m.Debug("EnableFileSink response", "response", respone.Msg)

	return nil
}

// DisableFileSink disables a file sink on the remote daemon
func (m *RemoteConfigManager) DisableFileSink(ctx context.Context, loggerName string) error {
	m.Debug("Disabling file sink", "loggerName", loggerName)

	repsonse, err := m.client.DisableFileSink(ctx, &connect.Request[v1.DisableFileSinkRequest]{
		Msg: &v1.DisableFileSinkRequest{
			LoggerName:    loggerName,
			RequesterInfo: m.newRequesterInfo(),
		},
	})
	if err != nil {
		return m.Errorf("failed to disable file sink: %w", err)

	}
	m.Debug("DisableFileSink response", "response", repsonse.Msg)
	return nil
}

// TailLogs streams log records from the remote daemon
func (m *RemoteConfigManager) TailLogs(ctx context.Context, logLevels []slog.Level, loggerFilter string, historyLines int) (<-chan *LogRecord, error) {
	m.Debug("Tailing logs", "logLevels", logLevels, "loggerFilter", loggerFilter, "historyLines", historyLines)

	// Convert slog levels to proto levels
	protoLevels := make([]v1.LogLevel, len(logLevels))
	for i, level := range logLevels {
		protoLevels[i] = SlogLevelToProtoLogLevel(level)
	}

	stream, err := m.client.TailLogs(ctx, &connect.Request[v1.TailLogsRequest]{
		Msg: &v1.TailLogsRequest{
			Levels:        protoLevels,
			LoggerFilter:  loggerFilter,
			HistoryLines:  int32(historyLines),
			Follow:        true,
			RequesterInfo: m.newRequesterInfo(),
		},
	})
	if err != nil {
		return nil, m.Errorf("failed to tail logs: %w", err)
	}

	m.Debug("Started tailing logs stream")

	// Create channel to receive log records
	output := make(chan *LogRecord)

	go func() {
		m.Debug("tail logs stream goroutine started")
		defer m.Debug("tail logs stream goroutine exiting")

		defer close(output)
		for stream.Receive() {
			msg := stream.Msg()
			record := &LogRecord{
				LoggerName: msg.LoggerName,
				Level:      ProtoLogLevelToSlogLevel(msg.Level),
				Message:    msg.Message,
				Attributes: msg.Attributes,
			}
			if msg.Timestamp != nil {
				record.Timestamp = timestampToTime(msg.Timestamp)
			}
			select {
			case <-ctx.Done():
				m.Debug("tail logs stream goroutine exiting due to context cancellation")
				return
			case output <- record:
			}
		}
	}()

	return output, nil
}

// AddSink creates and registers a new sink on the remote daemon
func (m *RemoteConfigManager) AddSink(ctx context.Context, sinkName, sinkType string, level slog.Level) error {
	m.Debug("Adding sink on remote daemon", "sink", sinkName, "type", sinkType, "level", level.String())

	protoType := v1.SinkType_SINK_TYPE_STDOUT
	switch sinkType {
	case "stdout", "text":
		protoType = v1.SinkType_SINK_TYPE_STDOUT
	case "stderr":
		protoType = v1.SinkType_SINK_TYPE_STDERR
	case "json", "json-stderr":
		protoType = v1.SinkType_SINK_TYPE_STDOUT // Both stdout types use STDOUT in proto
	default:
		return m.Errorf("unsupported sink type: %s", sinkType)
	}

	response, err := m.client.AddSink(ctx, &connect.Request[v1.AddSinkRequest]{
		Msg: &v1.AddSinkRequest{
			Name:          sinkName,
			Type:          protoType,
			Level:         SlogLevelToProtoLogLevel(level),
			RequesterInfo: m.newRequesterInfo(),
		},
	})
	if err != nil {
		return m.Errorf("failed to add sink: %w", err)
	}

	m.Debug("Sink added successfully", "sink", sinkName, "message", response.Msg.Message)
	return nil
}

// RemoveSink removes a sink from the remote daemon
func (m *RemoteConfigManager) RemoveSink(ctx context.Context, sinkName string) error {
	m.Debug("Removing sink on remote daemon", "sink", sinkName)

	response, err := m.client.RemoveSink(ctx, &connect.Request[v1.RemoveSinkRequest]{
		Msg: &v1.RemoveSinkRequest{
			Name:          sinkName,
			RequesterInfo: m.newRequesterInfo(),
		},
	})
	if err != nil {
		return m.Errorf("failed to remove sink: %w", err)
	}

	m.Debug("Sink removed successfully", "sink", sinkName, "loggers_affected", response.Msg.LoggersAffected)
	return nil
}

// AddLogger creates a new logger on the remote daemon
func (m *RemoteConfigManager) AddLogger(ctx context.Context, loggerName string, level slog.Level, sinkNames []string) error {
	m.Debug("Adding logger on remote daemon", "logger", loggerName, "level", level.String(), "sinks", sinkNames)

	response, err := m.client.AddLogger(ctx, &connect.Request[v1.AddLoggerRequest]{
		Msg: &v1.AddLoggerRequest{
			Name:          loggerName,
			Level:         SlogLevelToProtoLogLevel(level),
			SinkNames:     sinkNames,
			RequesterInfo: m.newRequesterInfo(),
		},
	})
	if err != nil {
		return m.Errorf("failed to add logger: %w", err)
	}

	m.Debug("Logger added successfully", "logger", loggerName, "message", response.Msg.Message)
	return nil
}

// RemoveLogger removes a logger from the remote daemon
func (m *RemoteConfigManager) RemoveLogger(ctx context.Context, loggerName string) error {
	m.Debug("Removing logger on remote daemon", "logger", loggerName)

	response, err := m.client.RemoveLogger(ctx, &connect.Request[v1.RemoveLoggerRequest]{
		Msg: &v1.RemoveLoggerRequest{
			Name:          loggerName,
			RequesterInfo: m.newRequesterInfo(),
		},
	})
	if err != nil {
		return m.Errorf("failed to remove logger: %w", err)
	}

	m.Debug("Logger removed successfully", "logger", loggerName, "message", response.Msg.Message)
	return nil
}

// AttachSink attaches a sink to a logger on the remote daemon
func (m *RemoteConfigManager) AttachSink(ctx context.Context, loggerName, sinkName string) error {
	m.Debug("Attaching sink to logger on remote daemon", "logger", loggerName, "sink", sinkName)

	response, err := m.client.AttachSink(ctx, &connect.Request[v1.AttachSinkRequest]{
		Msg: &v1.AttachSinkRequest{
			LoggerName:    loggerName,
			SinkName:      sinkName,
			RequesterInfo: m.newRequesterInfo(),
		},
	})
	if err != nil {
		return m.Errorf("failed to attach sink: %w", err)
	}

	m.Debug("Sink attached successfully", "logger", loggerName, "sink", sinkName, "message", response.Msg.Message)
	return nil
}

// DetachSink removes a sink from a logger on the remote daemon
func (m *RemoteConfigManager) DetachSink(ctx context.Context, loggerName, sinkName string) error {
	m.Debug("Detaching sink from logger on remote daemon", "logger", loggerName, "sink", sinkName)

	response, err := m.client.DetachSink(ctx, &connect.Request[v1.DetachSinkRequest]{
		Msg: &v1.DetachSinkRequest{
			LoggerName:    loggerName,
			SinkName:      sinkName,
			RequesterInfo: m.newRequesterInfo(),
		},
	})
	if err != nil {
		return m.Errorf("failed to detach sink: %w", err)
	}

	m.Debug("Sink detached successfully", "logger", loggerName, "sink", sinkName, "message", response.Msg.Message)
	return nil
}

// Helper functions

// sinkTypeToString converts protobuf enum sink type to string
func sinkTypeToString(t v1.SinkType) string {
	switch t {
	case v1.SinkType_SINK_TYPE_STDOUT:
		return "stdout"
	case v1.SinkType_SINK_TYPE_STDERR:
		return "stderr"
	case v1.SinkType_SINK_TYPE_FILE:
		return "file"
	case v1.SinkType_SINK_TYPE_SYSLOG:
		return "syslog"
	default:
		return "unknown"
	}
}

// timestampToTime converts protobuf Timestamp to time.Time
func timestampToTime(ts *v1.TimeStamp) time.Time {
	if ts == nil {
		return time.Time{}
	}
	return time.UnixMilli(ts.UnixMillis)
}
