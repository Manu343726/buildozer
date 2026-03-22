package logging

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExpandHome tests the ExpandHome function
func TestExpandHome(t *testing.T) {
	home := os.Getenv("HOME")
	if home == "" {
		t.Skip("HOME not set")
	}

	// Test tilde expansion
	result := ExpandHome("~/buildozer")
	expected := filepath.Join(home, "buildozer")
	assert.Equal(t, expected, result)

	// Test non-tilde path
	result = ExpandHome("/var/log")
	assert.Equal(t, "/var/log", result)

	// Test path with tilde not at start (should not expand)
	result = ExpandHome("/path/~other")
	assert.Equal(t, "/path/~other", result)
}

// TestDefaultLoggingConfig tests the default logging configuration
func TestDefaultLoggingConfig(t *testing.T) {
	config := DefaultLoggingConfig()

	// Should have some default sinks
	assert.NotEmpty(t, config.Sinks)

	// Should have some default loggers
	assert.NotEmpty(t, config.Loggers)

	// Global level should be set
	assert.NotZero(t, config.GlobalLevel)
}

// TestParseLevel tests parsing log levels from strings
func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected slog.Level
	}{
		{"error", slog.LevelError},
		{"ERROR", slog.LevelError},
		{"warn", slog.LevelWarn},
		{"WARNING", slog.LevelWarn},
		{"info", slog.LevelInfo},
		{"INFO", slog.LevelInfo},
		{"debug", slog.LevelDebug},
		{"DEBUG", slog.LevelDebug},
		{"invalid", slog.LevelInfo}, // Should default to info
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			result := ParseLevel(test.input)
			assert.Equal(t, test.expected, result)
		})
	}
}

// TestLevelToString tests converting log levels to strings
func TestLevelToString(t *testing.T) {
	tests := []struct {
		level    slog.Level
		expected string
	}{
		{slog.LevelError, "error"},
		{slog.LevelWarn, "warn"},
		{slog.LevelInfo, "info"},
		{slog.LevelDebug, "debug"},
	}

	for _, test := range tests {
		t.Run(test.expected, func(t *testing.T) {
			result := LevelToString(test.level)
			assert.Equal(t, test.expected, result)
		})
	}
}

// TestNewFactory tests creating a new logging factory
func TestNewFactory(t *testing.T) {
	registry := NewRegistry()
	factory := NewFactory(registry)

	assert.NotNil(t, factory)
	// Factory should be able to create sinks
	config := SinkConfig{
		Name:  "test_sink",
		Type:  "memory",
		Level: "info",
	}
	sink, err := factory.CreateSink(config)
	require.NoError(t, err)
	assert.NotNil(t, sink)
	assert.Equal(t, "test_sink", sink.Name)
}

// TestFactory_CreateSink tests creating sinks with various configurations
func TestFactory_CreateSink(t *testing.T) {
	registry := NewRegistry()
	factory := NewFactory(registry)

	tests := []struct {
		name   string
		config SinkConfig
		valid  bool
	}{
		{
			name: "memory sink",
			config: SinkConfig{
				Name:  "memory",
				Type:  "memory",
				Level: "info",
			},
			valid: true,
		},
		{
			name: "file sink",
			config: SinkConfig{
				Name:  "file",
				Type:  "file",
				Level: "debug",
			},
			valid: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sink, err := factory.CreateSink(test.config)
			if test.valid {
				require.NoError(t, err)
				assert.NotNil(t, sink)
				assert.Equal(t, test.config.Name, sink.Name)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

// TestRegistry_GetLoggerStatus tests getting logger status
func TestRegistry_GetLoggerStatus(t *testing.T) {
	registry := NewRegistry()

	// Set up a logger
	sink := &Sink{
		Name:    "test",
		Type:    "memory",
		Level:   slog.LevelInfo,
		Handler: slog.NewTextHandler(&bytes.Buffer{}, nil),
	}
	err := registry.RegisterSink(sink)
	require.NoError(t, err)

	err = registry.SetLoggerSinks("app", []string{"test"})
	require.NoError(t, err)

	// Get status
	status := registry.GetLoggerStatus()
	assert.NotNil(t, status)
	assert.Greater(t, len(status), 0)
}

// TestRegistry_GetSinkStatus tests getting sink status
func TestRegistry_GetSinkStatus(t *testing.T) {
	registry := NewRegistry()

	sink := &Sink{
		Name:    "test",
		Type:    "memory",
		Level:   slog.LevelInfo,
		Handler: slog.NewTextHandler(&bytes.Buffer{}, nil),
	}
	err := registry.RegisterSink(sink)
	require.NoError(t, err)

	// Get status
	status := registry.GetSinkStatus()
	assert.NotNil(t, status)
	// Should have at least the test sink
	assert.Greater(t, len(status), 0)
}
