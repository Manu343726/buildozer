package logging

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"connectrpc.com/connect"
	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	"github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1/protov1connect"
)

// RemoteConfigManager implements ConfigManager for remote daemon communication
// It uses a Connect client to communicate with a remote LoggingService
type RemoteConfigManager struct {
	client protov1connect.LoggingServiceClient
}

// NewRemoteConfigManager creates a new remote configuration manager
// It requires an HTTP client and the base URL of the remote daemon
func NewRemoteConfigManager(httpClient connect.HTTPClient, baseURL string) *RemoteConfigManager {
	client := protov1connect.NewLoggingServiceClient(httpClient, baseURL)
	return &RemoteConfigManager{
		client: client,
	}
}

// NewRemoteConfigManagerWithClient creates a new remote configuration manager with an existing client
func NewRemoteConfigManagerWithClient(client protov1connect.LoggingServiceClient) *RemoteConfigManager {
	return &RemoteConfigManager{
		client: client,
	}
}

// GetLoggingStatus returns the current logging configuration from the remote daemon
func (m *RemoteConfigManager) GetLoggingStatus(ctx context.Context) (*LoggingStatusSnapshot, error) {
	resp, err := m.client.GetLoggingStatus(ctx, &connect.Request[v1.GetLoggingStatusRequest]{
		Msg: &v1.GetLoggingStatusRequest{},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get logging status: %w", err)
	}

	status := resp.Msg.Status
	snapshot := &LoggingStatusSnapshot{
		GlobalLevel: ProtoLogLevelToSlogLevel(status.GlobalLevel),
		Sinks:       []SinkStatus{},
		Loggers:     []LoggerStatus{},
	}

	if status.RetrievedAt != nil {
		snapshot.RetrievedAt = timestampToTime(status.RetrievedAt)
	}

	// Convert sinks
	for _, sink := range status.Sinks {
		snapshot.Sinks = append(snapshot.Sinks, SinkStatus{
			Name:  sink.Name,
			Type:  sinkTypeToString(sink.Type),
			Level: ProtoLogLevelToSlogLevel(sink.Level),
		})
	}

	// Convert loggers
	for _, logger := range status.Loggers {
		snapshot.Loggers = append(snapshot.Loggers, LoggerStatus{
			Name:      logger.Name,
			SinkNames: logger.SinkNames,
		})
	}

	return snapshot, nil
}

// SetGlobalLevel changes the global logging level on the remote daemon
func (m *RemoteConfigManager) SetGlobalLevel(ctx context.Context, level slog.Level) error {
	_, err := m.client.SetGlobalLevel(ctx, &connect.Request[v1.SetGlobalLevelRequest]{
		Msg: &v1.SetGlobalLevelRequest{
			Level: SlogLevelToProtoLogLevel(level),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to set global level: %w", err)
	}
	return nil
}

// SetLoggerLevel changes the level for a logger on the remote daemon
func (m *RemoteConfigManager) SetLoggerLevel(ctx context.Context, loggerName string, level slog.Level) error {
	_, err := m.client.SetLoggerLevel(ctx, &connect.Request[v1.SetLoggerLevelRequest]{
		Msg: &v1.SetLoggerLevelRequest{
			LoggerName: loggerName,
			Level:      SlogLevelToProtoLogLevel(level),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to set logger level: %w", err)
	}
	return nil
}

// SetSinkLevel changes the level for a sink on the remote daemon
func (m *RemoteConfigManager) SetSinkLevel(ctx context.Context, sinkName string, level slog.Level) error {
	_, err := m.client.SetSinkLevel(ctx, &connect.Request[v1.SetSinkLevelRequest]{
		Msg: &v1.SetSinkLevelRequest{
			SinkName: sinkName,
			Level:    SlogLevelToProtoLogLevel(level),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to set sink level: %w", err)
	}
	return nil
}

// EnableFileSink enables a file sink on the remote daemon
func (m *RemoteConfigManager) EnableFileSink(ctx context.Context, loggerName, filePath string, maxSizeMB int, maxBackups int, maxAgeDays int) error {
	_, err := m.client.EnableFileSink(ctx, &connect.Request[v1.EnableFileSinkRequest]{
		Msg: &v1.EnableFileSinkRequest{
			LoggerName:   loggerName,
			FilePath:     filePath,
			MaxSizeBytes: int64(maxSizeMB) * 1024 * 1024,
			MaxBackups:   int32(maxBackups),
			MaxAgeDays:   int32(maxAgeDays),
			JsonFormat:   false,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to enable file sink: %w", err)
	}
	return nil
}

// DisableFileSink disables a file sink on the remote daemon
func (m *RemoteConfigManager) DisableFileSink(ctx context.Context, loggerName string) error {
	_, err := m.client.DisableFileSink(ctx, &connect.Request[v1.DisableFileSinkRequest]{
		Msg: &v1.DisableFileSinkRequest{
			LoggerName: loggerName,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to disable file sink: %w", err)
	}
	return nil
}

// TailLogs streams log records from the remote daemon
func (m *RemoteConfigManager) TailLogs(ctx context.Context, logLevels []slog.Level, loggerFilter string, historyLines int) (<-chan *LogRecord, error) {
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
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to tail logs: %w", err)
	}

	// Create channel to receive log records
	output := make(chan *LogRecord)

	go func() {
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
				return
			case output <- record:
			}
		}
	}()

	return output, nil
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
