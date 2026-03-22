package logging

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockConfigManager is a mock implementation of ConfigManager for testing
type MockConfigManager struct {
	mock.Mock
}

func (m *MockConfigManager) UpdateSinkConfig(ctx context.Context, sinkName string, change *SinkConfigChange) error {
	args := m.Called(ctx, sinkName, change)
	return args.Error(0)
}

func (m *MockConfigManager) EnableFileSink(ctx context.Context, loggerName, filePath string, maxSizeMB int32, maxBackups int32, maxAgeDays int32) error {
	args := m.Called(ctx, loggerName, filePath, maxSizeMB, maxBackups, maxAgeDays)
	return args.Error(0)
}

func (m *MockConfigManager) DisableFileSink(ctx context.Context, loggerName string) error {
	args := m.Called(ctx, loggerName)
	return args.Error(0)
}

func (m *MockConfigManager) AddSink(ctx context.Context, sinkName, sinkType string, level slog.Level) error {
	args := m.Called(ctx, sinkName, sinkType, level)
	return args.Error(0)
}

func (m *MockConfigManager) RemoveSink(ctx context.Context, sinkName string) error {
	args := m.Called(ctx, sinkName)
	return args.Error(0)
}

func (m *MockConfigManager) AddLogger(ctx context.Context, loggerName string, level slog.Level, sinkNames []string) error {
	args := m.Called(ctx, loggerName, level, sinkNames)
	return args.Error(0)
}

func (m *MockConfigManager) GetLoggingStatus(ctx context.Context) (*LoggingStatusSnapshot, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*LoggingStatusSnapshot), args.Error(1)
}

// TestMockConfigManager_GetLoggingStatus tests the mock can return status
func TestMockConfigManager_GetLoggingStatus(t *testing.T) {
	mockMgr := new(MockConfigManager)
	status := &LoggingStatusSnapshot{
		GlobalLevel:   slog.LevelInfo,
		Sinks:         []SinkStatus{},
		Loggers:       []LoggerStatus{},
		ActiveLoggers: []ActiveLoggerInfo{},
	}

	mockMgr.On("GetLoggingStatus", mock.Anything).Return(status, nil)

	ctx := context.Background()
	result, err := mockMgr.GetLoggingStatus(ctx)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, slog.LevelInfo, result.GlobalLevel)
	mockMgr.AssertCalled(t, "GetLoggingStatus", mock.Anything)
}

// TestMockConfigManager_UpdateSinkConfig tests the mock can track UpdateSinkConfig calls
func TestMockConfigManager_UpdateSinkConfig(t *testing.T) {
	mockMgr := new(MockConfigManager)
	mockMgr.On("UpdateSinkConfig", mock.Anything, "test_sink", mock.Anything).Return(nil)

	ctx := context.Background()
	change := &SinkConfigChange{
		Level: levelPtr(slog.LevelWarn),
	}

	err := mockMgr.UpdateSinkConfig(ctx, "test_sink", change)
	require.NoError(t, err)
	mockMgr.AssertCalled(t, "UpdateSinkConfig", mock.Anything, "test_sink", mock.Anything)
}

// TestMockConfigManager_AddSink tests the mock can track AddSink calls
func TestMockConfigManager_AddSink(t *testing.T) {
	mockMgr := new(MockConfigManager)
	mockMgr.On("AddSink", mock.Anything, "new_sink", "file", slog.LevelError).Return(nil)

	ctx := context.Background()
	err := mockMgr.AddSink(ctx, "new_sink", "file", slog.LevelError)
	require.NoError(t, err)
	mockMgr.AssertCalled(t, "AddSink", mock.Anything, "new_sink", "file", slog.LevelError)
}

// TestMockConfigManager_AddLogger tests the mock can track AddLogger calls
func TestMockConfigManager_AddLogger(t *testing.T) {
	mockMgr := new(MockConfigManager)
	mockMgr.On("AddLogger", mock.Anything, "app", slog.LevelWarn, []string{"sink1"}).Return(nil)

	ctx := context.Background()
	err := mockMgr.AddLogger(ctx, "app", slog.LevelWarn, []string{"sink1"})
	require.NoError(t, err)
	mockMgr.AssertCalled(t, "AddLogger", mock.Anything, "app", slog.LevelWarn, []string{"sink1"})
}

// TestMockConfigManager_EnableFileSink tests the mock can track EnableFileSink calls
func TestMockConfigManager_EnableFileSink(t *testing.T) {
	mockMgr := new(MockConfigManager)
	mockMgr.On(
		"EnableFileSink",
		mock.Anything,
		"app.logger",
		"/var/log/app.log",
		int32(10),
		int32(5),
		int32(7),
	).Return(nil)

	ctx := context.Background()
	err := mockMgr.EnableFileSink(ctx, "app.logger", "/var/log/app.log", 10, 5, 7)
	require.NoError(t, err)
	mockMgr.AssertCalled(t, "EnableFileSink", mock.Anything, "app.logger", "/var/log/app.log", int32(10), int32(5), int32(7))
}
