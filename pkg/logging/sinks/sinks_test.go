package sinks

import (
	"log/slog"
	"os"
	"testing"
	"time"
)

// TestFileSinkWithAgeRotation tests that FileSink properly configures age-based rotation
func TestFileSinkWithAgeRotation(t *testing.T) {
	tmpFile := "/tmp/test-age-rotation.log"

	// Test case 1: With age-based rotation enabled (7 days)
	config := FileSinkConfig{
		Path:       tmpFile,
		MaxSizeB:   1024 * 1024, // 1MB
		MaxFiles:   5,
		MaxAgeDays: 7, // 7 days
		JSONFormat: false,
	}

	handler, err := FileSink(config)
	if err != nil {
		t.Fatalf("Failed to create file sink with age rotation: %v", err)
	}

	if handler == nil {
		t.Fatal("FileSink returned nil handler")
	}

	// Verify the handler is properly created
	testRecord := slog.Record{
		Time:    time.Now(),
		Level:   slog.LevelInfo,
		Message: "Test message",
	}

	err = handler.Handle(nil, testRecord)
	if err != nil {
		t.Fatalf("Handler.Handle failed: %v", err)
	}

	// Clean up
	os.Remove(tmpFile)
}

// TestFileSinkWithoutAgeRotation tests default behavior (age rotation disabled)
func TestFileSinkWithoutAgeRotation(t *testing.T) {
	tmpFile := "/tmp/test-no-age.log"

	// Test case: Without age-based rotation (0 = disabled)
	config := FileSinkConfig{
		Path:       tmpFile,
		MaxSizeB:   1024 * 1024, // 1MB
		MaxFiles:   5,
		MaxAgeDays: 0, // Disabled
		JSONFormat: false,
	}

	handler, err := FileSink(config)
	if err != nil {
		t.Fatalf("Failed to create file sink without age rotation: %v", err)
	}

	if handler == nil {
		t.Fatal("FileSink returned nil handler")
	}

	// Verify the handler works
	testRecord := slog.Record{
		Time:    time.Now(),
		Level:   slog.LevelInfo,
		Message: "Test message",
	}

	err = handler.Handle(nil, testRecord)
	if err != nil {
		t.Fatalf("Handler.Handle failed: %v", err)
	}

	// Clean up
	os.Remove(tmpFile)
}

// TestJSONFileSinkWithAge tests JSON file sink with age-based rotation
func TestJSONFileSinkWithAge(t *testing.T) {
	tmpFile := "/tmp/test-json-age.log"

	// Create JSON sink with 14-day age limit
	handler, err := JSONFileSink(tmpFile, 100, 14)
	if err != nil {
		t.Fatalf("Failed to create JSON file sink: %v", err)
	}

	if handler == nil {
		t.Fatal("JSONFileSink returned nil handler")
	}

	// Verify the handler works
	testRecord := slog.Record{
		Time:    time.Now(),
		Level:   slog.LevelInfo,
		Message: "Test JSON message",
	}

	err = handler.Handle(nil, testRecord)
	if err != nil {
		t.Fatalf("Handler.Handle failed: %v", err)
	}

	// Clean up
	os.Remove(tmpFile)
}

// TestTextFileSinkWithAge tests text file sink with age-based rotation
func TestTextFileSinkWithAge(t *testing.T) {
	tmpFile := "/tmp/test-text-age.log"

	// Create text sink with 30-day age limit
	handler, err := TextFileSink(tmpFile, 50, 30)
	if err != nil {
		t.Fatalf("Failed to create text file sink: %v", err)
	}

	if handler == nil {
		t.Fatal("TextFileSink returned nil handler")
	}

	// Verify the handler works
	testRecord := slog.Record{
		Time:    time.Now(),
		Level:   slog.LevelDebug,
		Message: "Test text message",
	}

	err = handler.Handle(nil, testRecord)
	if err != nil {
		t.Fatalf("Handler.Handle failed: %v", err)
	}

	// Clean up
	os.Remove(tmpFile)
}
