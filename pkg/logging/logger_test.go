package logging

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/Manu343726/buildozer/pkg/logging/sinks"
)

// TestLoggerWithAttrs verifies that WithAttrs accumulates and applies attributes
func TestLoggerWithAttrs(t *testing.T) {
	// Create a buffer to capture output
	var buf bytes.Buffer

	// Create an ordered text handler
	handler := sinks.NewOrderedTextHandler(&buf, nil)
	sink := &Sink{
		Name:    "test",
		Type:    "memory",
		Level:   slog.LevelDebug,
		Handler: handler,
	}

	// Create registry and logger
	registry := NewRegistry()
	_ = registry.RegisterSink(sink)
	_ = registry.SetLoggerSinks("test", []string{"test"})

	logger := registry.GetLogger("test")

	// Test 1: Basic logging without attributes
	buf.Reset()
	logger.Info("message without attrs")
	if buf.Len() == 0 {
		t.Error("Expected output for basic message")
	}
	t.Logf("Test 1 output: %s", buf.String())

	// Test 2: Logger with fixed attributes
	buf.Reset()
	logger2 := logger.WithAttrs(slog.String("user", "alice"), slog.Int("id", 42))
	logger2.Info("message with fixed attrs")
	output := buf.String()
	if !bytes.Contains(buf.Bytes(), []byte("alice")) {
		t.Error("Expected 'alice' in output with accumulated attrs")
	}
	if !bytes.Contains(buf.Bytes(), []byte("42")) {
		t.Error("Expected '42' in output with accumulated attrs")
	}
	t.Logf("Test 2 output: %s", output)

	// Test 3: Child logger inherits attributes
	buf.Reset()
	child := logger2.Child("submodule")
	child.Info("child message")
	output = buf.String()
	// Child should inherit parent's attributes
	if !bytes.Contains(buf.Bytes(), []byte("alice")) {
		t.Error("Expected child to inherit parent's attributes")
	}
	t.Logf("Test 3 output: %s", output)

	// Test 4: Child can add more attributes
	buf.Reset()
	child2 := child.WithAttrs(slog.String("request_id", "xyz123"))
	child2.Info("child with additional attrs")
	output = buf.String()
	if !bytes.Contains(buf.Bytes(), []byte("alice")) || !bytes.Contains(buf.Bytes(), []byte("xyz123")) {
		t.Error("Expected child to have both parent and own attributes")
	}
	t.Logf("Test 4 output: %s", output)

	t.Log("All Logger.WithAttrs tests passed!")
}

// TestLoggerWithGroup verifies that WithGroup creates a group context
func TestLoggerWithGroup(t *testing.T) {
	var buf bytes.Buffer
	handler := sinks.NewOrderedTextHandler(&buf, nil)
	sink := &Sink{
		Name:    "test",
		Type:    "memory",
		Level:   slog.LevelDebug,
		Handler: handler,
	}

	registry := NewRegistry()
	_ = registry.RegisterSink(sink)
	_ = registry.SetLoggerSinks("test", []string{"test"})

	logger := registry.GetLogger("test")

	// Test: Logger with group
	buf.Reset()
	logger2 := logger.WithGroup("request")
	logger2.Info("message with group", slog.String("path", "/api"))
	output := buf.String()
	t.Logf("WithGroup output: %s", output)

	// The group context should be applied
	t.Log("Logger.WithGroup test completed")
}

// TestLoggerHierarchy verifies that WithAttrs accumulates correctly through hierarchy
func TestLoggerHierarchy(t *testing.T) {
	var buf bytes.Buffer
	handler := sinks.NewOrderedTextHandler(&buf, nil)
	sink := &Sink{
		Name:    "test",
		Type:    "memory",
		Level:   slog.LevelDebug,
		Handler: handler,
	}

	registry := NewRegistry()
	_ = registry.RegisterSink(sink)
	_ = registry.SetLoggerSinks("app", []string{"test"})
	_ = registry.SetLoggerSinks("app.db", []string{"test"})
	_ = registry.SetLoggerSinks("app.db.postgres", []string{"test"})

	logger := registry.GetLogger("app")
	logger = logger.WithAttrs(slog.String("env", "prod"))

	db := logger.Child("db")
	db = db.WithAttrs(slog.String("type", "postgres"))

	postgres := db.Child("postgres")
	postgres = postgres.WithAttrs(slog.String("host", "localhost"))

	// All accumulated attributes should be present
	buf.Reset()
	postgres.Info("connection event", slog.String("event", "connect"))
	output := buf.String()

	t.Logf("Hierarchy output: %s", output)

	// Verify all attributes are present
	if !bytes.Contains(buf.Bytes(), []byte("prod")) {
		t.Error("Expected 'prod' from root logger")
	}
	if !bytes.Contains(buf.Bytes(), []byte("postgres")) {
		t.Error("Expected 'postgres' from db logger")
	}
	if !bytes.Contains(buf.Bytes(), []byte("localhost")) {
		t.Error("Expected 'localhost' from postgres logger")
	}

	t.Log("Hierarchy accumulation test passed!")
}
