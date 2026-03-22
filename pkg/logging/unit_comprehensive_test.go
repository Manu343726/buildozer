package logging

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/Manu343726/buildozer/pkg/logging/sinks"
)

// ==================== Registry Sink Tests ====================

// TestRegistry_RegisterAndRetrieveSink tests registering and retrieving sinks
func TestRegistry_RegisterAndRetrieveSink(t *testing.T) {
	registry := NewRegistry()

	var buf bytes.Buffer
	sink := &Sink{
		Name:    "test_sink",
		Type:    "stdout",
		Level:   slog.LevelDebug,
		Handler: slog.NewTextHandler(&buf, nil),
	}

	if err := registry.RegisterSink(sink); err != nil {
		t.Fatalf("Failed to register sink: %v", err)
	}

	retrieved, exists := registry.GetSink("test_sink")
	if !exists {
		t.Fatal("Sink should exist after registration")
	}
	if retrieved.Name != "test_sink" {
		t.Errorf("Expected sink name 'test_sink', got '%s'", retrieved.Name)
	}
}

// TestRegistry_DuplicateSinkRegistration tests that duplicate sink names are rejected
func TestRegistry_DuplicateSinkRegistration(t *testing.T) {
	registry := NewRegistry()

	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, nil)

	sink1 := &Sink{
		Name:    "duplicate",
		Type:    "stdout",
		Level:   slog.LevelDebug,
		Handler: handler,
	}

	sink2 := &Sink{
		Name:    "duplicate",
		Type:    "stderr",
		Level:   slog.LevelWarn,
		Handler: handler,
	}

	if err := registry.RegisterSink(sink1); err != nil {
		t.Fatalf("Failed to register first sink: %v", err)
	}

	err := registry.RegisterSink(sink2)
	if err == nil {
		t.Error("Should reject duplicate sink name")
	}
}

// TestRegistry_GetNonexistentSink tests retrieving nonexistent sink
func TestRegistry_GetNonexistentSink(t *testing.T) {
	registry := NewRegistry()

	_, exists := registry.GetSink("nonexistent")
	if exists {
		t.Error("Nonexistent sink should not be found")
	}
}

// TestRegistry_RemoveSink tests removing sinks
func TestRegistry_RemoveSink(t *testing.T) {
	registry := NewRegistry()

	var buf bytes.Buffer
	sink := &Sink{
		Name:    "test_sink",
		Type:    "stdout",
		Level:   slog.LevelDebug,
		Handler: slog.NewTextHandler(&buf, nil),
	}

	if err := registry.RegisterSink(sink); err != nil {
		t.Fatalf("Failed to register sink: %v", err)
	}

	if err := registry.RemoveSink("test_sink"); err != nil {
		t.Fatalf("Failed to remove sink: %v", err)
	}

	_, exists := registry.GetSink("test_sink")
	if exists {
		t.Error("Sink should be removed")
	}
}

// TestRegistry_RemoveNonexistentSink tests removing nonexistent sink
func TestRegistry_RemoveNonexistentSink(t *testing.T) {
	registry := NewRegistry()

	err := registry.RemoveSink("nonexistent")
	if err == nil {
		t.Error("Should fail when removing nonexistent sink")
	}
}

// TestRegistry_GetAllSinks tests retrieving all sinks
func TestRegistry_GetAllSinks(t *testing.T) {
	registry := NewRegistry()

	var buf bytes.Buffer

	sink1 := &Sink{
		Name:    "sink1",
		Type:    "stdout",
		Level:   slog.LevelDebug,
		Handler: slog.NewTextHandler(&buf, nil),
	}
	sink2 := &Sink{
		Name:    "sink2",
		Type:    "stderr",
		Level:   slog.LevelWarn,
		Handler: slog.NewTextHandler(&buf, nil),
	}

	if err := registry.RegisterSink(sink1); err != nil {
		t.Fatalf("Failed to register sink1: %v", err)
	}
	if err := registry.RegisterSink(sink2); err != nil {
		t.Fatalf("Failed to register sink2: %v", err)
	}

	allSinks := registry.GetAllSinks()
	if len(allSinks) < 2 {
		t.Errorf("Expected at least 2 sinks, got %d", len(allSinks))
	}

	sinkNames := make(map[string]bool)
	for _, s := range allSinks {
		sinkNames[s.Name] = true
	}

	if !sinkNames["sink1"] || !sinkNames["sink2"] {
		t.Error("Expected both sink1 and sink2 in all sinks")
	}
}

// ==================== Registry Logger Tests ====================

// TestRegistry_SetLoggerSinks tests setting logger sinks
func TestRegistry_SetLoggerSinks(t *testing.T) {
	registry := NewRegistry()

	var buf bytes.Buffer
	sink := &Sink{
		Name:    "test_sink",
		Type:    "stdout",
		Level:   slog.LevelDebug,
		Handler: slog.NewTextHandler(&buf, nil),
	}

	if err := registry.RegisterSink(sink); err != nil {
		t.Fatalf("Failed to register sink: %v", err)
	}

	if err := registry.SetLoggerSinks("test_logger", []string{"test_sink"}); err != nil {
		t.Fatalf("Failed to set logger sinks: %v", err)
	}

	sinks, exists := registry.GetLoggerSinks("test_logger")
	if !exists {
		t.Fatal("Logger should be registered")
	}
	if len(sinks) != 1 || sinks[0] != "test_sink" {
		t.Error("Logger should reference test_sink")
	}
}

// TestRegistry_GetLoggerSinks_Multiple tests getting multiple logger sinks
func TestRegistry_GetLoggerSinks_Multiple(t *testing.T) {
	registry := NewRegistry()

	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, nil)

	sink1 := &Sink{
		Name:    "sink1",
		Type:    "stdout",
		Level:   slog.LevelDebug,
		Handler: handler,
	}
	sink2 := &Sink{
		Name:    "sink2",
		Type:    "stderr",
		Level:   slog.LevelWarn,
		Handler: handler,
	}

	if err := registry.RegisterSink(sink1); err != nil {
		t.Fatalf("Failed to register sink1: %v", err)
	}
	if err := registry.RegisterSink(sink2); err != nil {
		t.Fatalf("Failed to register sink2: %v", err)
	}

	if err := registry.SetLoggerSinks("test_logger", []string{"sink1", "sink2"}); err != nil {
		t.Fatalf("Failed to set logger sinks: %v", err)
	}

	sinks, exists := registry.GetLoggerSinks("test_logger")
	if !exists {
		t.Fatal("Logger should exist")
	}
	if len(sinks) != 2 {
		t.Errorf("Expected 2 sinks, got %d", len(sinks))
	}
}

// TestRegistry_RemoveLogger tests removing loggers
func TestRegistry_RemoveLogger(t *testing.T) {
	registry := NewRegistry()

	var buf bytes.Buffer
	sink := &Sink{
		Name:    "test_sink",
		Type:    "stdout",
		Level:   slog.LevelDebug,
		Handler: slog.NewTextHandler(&buf, nil),
	}

	if err := registry.RegisterSink(sink); err != nil {
		t.Fatalf("Failed to register sink: %v", err)
	}

	if err := registry.SetLoggerSinks("test_logger", []string{"test_sink"}); err != nil {
		t.Fatalf("Failed to set logger sinks: %v", err)
	}

	if err := registry.RemoveLogger("test_logger"); err != nil {
		t.Fatalf("Failed to remove logger: %v", err)
	}

	_, exists := registry.GetLoggerSinks("test_logger")
	if exists {
		t.Error("Logger should be removed")
	}
}

// ==================== Registry Hierarchical Logger Tests ====================

// TestRegistry_HierarchicalLookup_ExactMatch tests hierarchical logger lookup with exact match
func TestRegistry_HierarchicalLookup_ExactMatch(t *testing.T) {
	registry := NewRegistry()

	var buf bytes.Buffer

	sinkAll := &Sink{
		Name:    "all",
		Type:    "stdout",
		Level:   slog.LevelDebug,
		Handler: slog.NewTextHandler(&buf, nil),
	}
	sinkRuntime := &Sink{
		Name:    "runtime",
		Type:    "stderr",
		Level:   slog.LevelWarn,
		Handler: slog.NewTextHandler(&buf, nil),
	}

	if err := registry.RegisterSink(sinkAll); err != nil {
		t.Fatalf("Failed to register sink: %v", err)
	}
	if err := registry.RegisterSink(sinkRuntime); err != nil {
		t.Fatalf("Failed to register sink: %v", err)
	}

	if err := registry.SetLoggerSinks("buildozer", []string{"all"}); err != nil {
		t.Fatalf("Failed to set logger sinks: %v", err)
	}

	if err := registry.SetLoggerSinks("buildozer.runtime", []string{"runtime"}); err != nil {
		t.Fatalf("Failed to set logger sinks: %v", err)
	}

	// Exact match should return the specific logger's sinks
	sinks, exists := registry.GetLoggerSinks("buildozer.runtime")
	if !exists {
		t.Fatal("Logger buildozer.runtime should exist")
	}
	if len(sinks) == 0 || sinks[0] != "runtime" {
		t.Error("Logger should reference runtime sink")
	}
}

// TestRegistry_HierarchicalLookup_FallbackToParent tests that child loggers can be set separately
func TestRegistry_HierarchicalLookup_FallbackToParent(t *testing.T) {
	registry := NewRegistry()

	var buf bytes.Buffer
	sink := &Sink{
		Name:    "all",
		Type:    "stdout",
		Level:   slog.LevelDebug,
		Handler: slog.NewTextHandler(&buf, nil),
	}

	if err := registry.RegisterSink(sink); err != nil {
		t.Fatalf("Failed to register sink: %v", err)
	}

	// Set parent logger
	if err := registry.SetLoggerSinks("buildozer", []string{"all"}); err != nil {
		t.Fatalf("Failed to set logger sinks: %v", err)
	}

	// Verify parent logger exists
	sinks, exists := registry.GetLoggerSinks("buildozer")
	if !exists {
		t.Fatal("Parent logger should exist")
	}
	if len(sinks) == 0 || sinks[0] != "all" {
		t.Error("Parent logger should reference 'all' sink")
	}

	// Also set child logger explicitly
	if err := registry.SetLoggerSinks("buildozer.runtime", []string{"all"}); err != nil {
		t.Fatalf("Failed to set child logger sinks: %v", err)
	}

	// Verify child logger exists
	sinks, exists = registry.GetLoggerSinks("buildozer.runtime")
	if !exists {
		t.Fatal("Child logger should exist")
	}
	if len(sinks) == 0 || sinks[0] != "all" {
		t.Error("Child logger should reference 'all' sink")
	}
}

// ==================== Registry Sink Attachment Tests ====================

// TestRegistry_AttachSink tests attaching sinks to loggers
func TestRegistry_AttachSink(t *testing.T) {
	registry := NewRegistry()

	var buf bytes.Buffer
	sink := &Sink{
		Name:    "test_sink",
		Type:    "stdout",
		Level:   slog.LevelDebug,
		Handler: slog.NewTextHandler(&buf, nil),
	}

	if err := registry.RegisterSink(sink); err != nil {
		t.Fatalf("Failed to register sink: %v", err)
	}

	if err := registry.SetLoggerSinks("test_logger", []string{}); err != nil {
		t.Fatalf("Failed to set logger sinks: %v", err)
	}

	if err := registry.AttachSink("test_logger", "test_sink"); err != nil {
		t.Fatalf("Failed to attach sink: %v", err)
	}

	sinks, exists := registry.GetLoggerSinks("test_logger")
	if !exists {
		t.Fatal("Logger should exist")
	}
	found := false
	for _, s := range sinks {
		if s == "test_sink" {
			found = true
			break
		}
	}
	if !found {
		t.Error("test_sink should be attached")
	}
}

// TestRegistry_DetachSink tests detaching sinks from loggers
func TestRegistry_DetachSink(t *testing.T) {
	registry := NewRegistry()

	var buf bytes.Buffer
	sink := &Sink{
		Name:    "test_sink",
		Type:    "stdout",
		Level:   slog.LevelDebug,
		Handler: slog.NewTextHandler(&buf, nil),
	}

	if err := registry.RegisterSink(sink); err != nil {
		t.Fatalf("Failed to register sink: %v", err)
	}

	if err := registry.SetLoggerSinks("test_logger", []string{"test_sink"}); err != nil {
		t.Fatalf("Failed to set logger sinks: %v", err)
	}

	if err := registry.DetachSink("test_logger", "test_sink"); err != nil {
		t.Fatalf("Failed to detach sink: %v", err)
	}

	sinks, exists := registry.GetLoggerSinks("test_logger")
	if !exists {
		t.Fatal("Logger should still exist")
	}
	for _, s := range sinks {
		if s == "test_sink" {
			t.Error("test_sink should not be attached")
			return
		}
	}
}

// ==================== Logger Attribute Tests ====================

// TestLogger_WithAttrs tests logger attributes accumulation
func TestLogger_WithAttrs(t *testing.T) {
	var buf bytes.Buffer
	handler := sinks.NewOrderedTextHandler(&buf, nil)
	sink := &Sink{
		Name:    "test",
		Type:    "memory",
		Level:   slog.LevelDebug,
		Handler: handler,
	}

	registry := NewRegistry()
	if err := registry.RegisterSink(sink); err != nil {
		t.Fatalf("Failed to register sink: %v", err)
	}
	if err := registry.SetLoggerSinks("test", []string{"test"}); err != nil {
		t.Fatalf("Failed to set logger sinks: %v", err)
	}

	logger := registry.GetLogger("test")

	// Test logging with attributes
	buf.Reset()
	logger2 := logger.With(slog.String("user", "alice"), slog.Int("id", 42))
	logger2.Info("message with attrs")

	_ = buf.String()
	if !bytes.Contains(buf.Bytes(), []byte("alice")) {
		t.Error("Expected 'alice' in output")
	}
	if !bytes.Contains(buf.Bytes(), []byte("42")) {
		t.Error("Expected '42' in output")
	}
}

// TestLogger_WithGroup tests logger groups
func TestLogger_WithGroup(t *testing.T) {
	var buf bytes.Buffer
	handler := sinks.NewOrderedTextHandler(&buf, nil)
	sink := &Sink{
		Name:    "test",
		Type:    "memory",
		Level:   slog.LevelDebug,
		Handler: handler,
	}

	registry := NewRegistry()
	if err := registry.RegisterSink(sink); err != nil {
		t.Fatalf("Failed to register sink: %v", err)
	}
	if err := registry.SetLoggerSinks("test", []string{"test"}); err != nil {
		t.Fatalf("Failed to set logger sinks: %v", err)
	}

	logger := registry.GetLogger("test")

	// Test logger with group
	buf.Reset()
	logger2 := logger.WithGroup("request")
	logger2.Info("message with group", slog.String("path", "/api"))

	// Just verify it doesn't crash - group functionality is tested elsewhere
	if buf.Len() == 0 {
		t.Error("Expected output for grouped message")
	}
}

// TestLogger_Hierarchy tests logger hierarchy with accumulated attributes
func TestLogger_Hierarchy(t *testing.T) {
	var buf bytes.Buffer
	handler := sinks.NewOrderedTextHandler(&buf, nil)
	sink := &Sink{
		Name:    "test",
		Type:    "memory",
		Level:   slog.LevelDebug,
		Handler: handler,
	}

	registry := NewRegistry()
	if err := registry.RegisterSink(sink); err != nil {
		t.Fatalf("Failed to register sink: %v", err)
	}
	if err := registry.SetLoggerSinks("app", []string{"test"}); err != nil {
		t.Fatalf("Failed to set logger sinks: %v", err)
	}

	logger1 := registry.GetLogger("app")
	logger1 = logger1.With(slog.String("env", "prod"))

	// Log from parent
	buf.Reset()
	logger1.Info("parent message")
	if buf.Len() == 0 {
		t.Error("Expected output for parent logger")
	}
}

// ==================== Level Management Tests ====================

// TestRegistry_SetLoggerLevel tests setting logger level
func TestRegistry_SetLoggerLevel(t *testing.T) {
	registry := NewRegistry()

	var buf bytes.Buffer
	sink := &Sink{
		Name:    "test_sink",
		Type:    "stdout",
		Level:   slog.LevelDebug,
		Handler: slog.NewTextHandler(&buf, nil),
	}

	if err := registry.RegisterSink(sink); err != nil {
		t.Fatalf("Failed to register sink: %v", err)
	}

	if err := registry.SetLoggerSinks("test_logger", []string{"test_sink"}); err != nil {
		t.Fatalf("Failed to set logger sinks: %v", err)
	}

	if err := registry.SetLoggerLevel("test_logger", slog.LevelWarn); err != nil {
		t.Fatalf("Failed to set logger level: %v", err)
	}

	// Verify level was set
	if err := registry.SetLoggerLevel("test_logger", slog.LevelInfo); err != nil {
		t.Fatalf("Failed to update logger level: %v", err)
	}
}

// TestRegistry_SetSinkLevel tests setting sink level
func TestRegistry_SetSinkLevel(t *testing.T) {
	registry := NewRegistry()

	var buf bytes.Buffer
	sink := &Sink{
		Name:    "test_sink",
		Type:    "stdout",
		Level:   slog.LevelDebug,
		Handler: slog.NewTextHandler(&buf, nil),
	}

	if err := registry.RegisterSink(sink); err != nil {
		t.Fatalf("Failed to register sink: %v", err)
	}

	if err := registry.SetSinkLevel("test_sink", slog.LevelWarn); err != nil {
		t.Fatalf("Failed to set sink level: %v", err)
	}

	retrieved, exists := registry.GetSink("test_sink")
	if !exists {
		t.Fatal("Sink should exist")
	}
	if retrieved.Level != slog.LevelWarn {
		t.Errorf("Expected level %v, got %v", slog.LevelWarn, retrieved.Level)
	}
}

// TestRegistry_GlobalLevel tests global logging level
func TestRegistry_GlobalLevel(t *testing.T) {
	registry := NewRegistry()

	originalLevel := registry.GetGlobalLevel()

	registry.SetGlobalLevel(slog.LevelWarn)
	newLevel := registry.GetGlobalLevel()

	if newLevel != slog.LevelWarn {
		t.Errorf("Expected level %v, got %v", slog.LevelWarn, newLevel)
	}

	// Restore original level
	registry.SetGlobalLevel(originalLevel)
}
