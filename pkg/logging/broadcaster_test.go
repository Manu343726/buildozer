package logging

import (
	"log/slog"
	"testing"
	"time"
)

// TestLogBroadcasterSubscribe verifies broadcaster subscribe/unsubscribe mechanics
func TestLogBroadcasterSubscribe(t *testing.T) {
	broadcaster := NewLogBroadcaster()

	// Subscribe first stream
	ch1, unsub1 := broadcaster.Subscribe()
	if ch1 == nil {
		t.Fatal("Expected non-nil channel from Subscribe()")
	}

	// Subscribe second stream (verify multiple subscribers work)
	ch2, unsub2 := broadcaster.Subscribe()
	if ch2 == nil {
		t.Fatal("Expected non-nil channel from second Subscribe()")
	}

	// Verify channels are different
	if ch1 == ch2 {
		t.Error("Expected different channels for different subscribers")
	}

	// Test unsubscribe
	unsub1()
	unsub2()
}

// TestLogBroadcasterBroadcast verifies messages are sent to all subscribers
func TestLogBroadcasterBroadcast(t *testing.T) {
	broadcaster := NewLogBroadcaster()

	// Create multiple subscribers
	ch1, unsub1 := broadcaster.Subscribe()
	defer unsub1()
	ch2, unsub2 := broadcaster.Subscribe()
	defer unsub2()

	// Broadcast a message
	entry := &LogEntry{
		Timestamp:  time.Now().UnixNano(),
		LoggerName: "test",
		Level:      slog.LevelInfo,
		Message:    "test message",
		Attributes: map[string]string{"key": "value"},
	}

	broadcaster.Broadcast(entry)

	// Both subscribers should receive the message (non-blocking)
	// Need to do this quickly as broadcast doesn't block
	timeout := time.After(100 * time.Millisecond)

	received1 := false
	received2 := false

	// Check first subscriber
	select {
	case msg := <-ch1:
		if msg.Message != "test message" {
			t.Errorf("Expected 'test message', got %q", msg.Message)
		}
		received1 = true
	case <-timeout:
	}

	// Check second subscriber
	select {
	case msg := <-ch2:
		if msg.Message != "test message" {
			t.Errorf("Expected 'test message', got %q", msg.Message)
		}
		received2 = true
	case <-timeout:
	}

	if !received1 {
		t.Error("First subscriber did not receive message")
	}
	if !received2 {
		t.Error("Second subscriber did not receive message")
	}
}

// TestLogBroadcasterNonBlocking verifies broadcast is non-blocking
func TestLogBroadcasterNonBlocking(t *testing.T) {
	broadcaster := NewLogBroadcaster()

	// Create a slow subscriber that doesn't read from channel
	_ch, unsub := broadcaster.Subscribe()
	defer unsub()
	_ = _ch  // Mark as used (simulating a slow subscriber)

	// Broadcast should not block even though subscriber isn't reading
	entry := &LogEntry{
		Timestamp:  time.Now().UnixNano(),
		LoggerName: "test",
		Level:      slog.LevelInfo,
		Message:    "test",
		Attributes: map[string]string{},
	}

	done := make(chan bool, 1)
	go func() {
		broadcaster.Broadcast(entry)
		done <- true
	}()

	// Should complete quickly
	select {
	case <-done:
		// Success - broadcast completed
	case <-time.After(100 * time.Millisecond):
		t.Error("Broadcast blocked when it should be non-blocking")
	}
}

// TestShouldIncludeLogByLevel verifies level-based filtering
func TestShouldIncludeLogByLevel(t *testing.T) {
	testCases := []struct {
		name       string
		levels     []slog.Level
		entryLevel slog.Level
		included   bool
	}{
		{
			name:       "Empty levels means all levels included",
			levels:     []slog.Level{},
			entryLevel: slog.LevelInfo,
			included:   true,
		},
		{
			name:       "Entry with matching level included",
			levels:     []slog.Level{slog.LevelWarn, slog.LevelError},
			entryLevel: slog.LevelWarn,
			included:   true,
		},
		{
			name:       "Entry with non-matching level excluded",
			levels:     []slog.Level{slog.LevelWarn, slog.LevelError},
			entryLevel: slog.LevelDebug,
			included:   false,
		},
		{
			name:       "Single level filter works",
			levels:     []slog.Level{slog.LevelError},
			entryLevel: slog.LevelError,
			included:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			entry := &LogEntry{
				Timestamp:  time.Now().UnixNano(),
				LoggerName: "test",
				Level:      tc.entryLevel,
				Message:    "test message",
				Attributes: map[string]string{},
			}

			result := shouldIncludeLog(entry, tc.levels, "")
			if result != tc.included {
				t.Errorf("Expected included=%v, got %v", tc.included, result)
			}
		})
	}
}

// TestShouldIncludeLogByLoggerName verifies logger name filtering
func TestShouldIncludeLogByLoggerName(t *testing.T) {
	testCases := []struct {
		name        string
		loggerName  string
		filter      string
		included    bool
	}{
		{
			name:       "Empty filter includes all loggers",
			loggerName: "com.example.service",
			filter:     "",
			included:   true,
		},
		{
			name:       "Exact logger name match",
			loggerName: "com.example.service",
			filter:     "com.example.service",
			included:   true,
		},
		{
			name:       "Exact logger name no match",
			loggerName: "com.example.service",
			filter:     "com.other.service",
			included:   false,
		},
		{
			name:       "Prefix wildcard match",
			loggerName: "com.example.service",
			filter:     "com.example.*",
			included:   true,
		},
		{
			name:       "Prefix wildcard no match",
			loggerName: "com.example.service",
			filter:     "com.other.*",
			included:   false,
		},
		{
			name:       "Single component prefix match",
			loggerName: "myservice",
			filter:     "my*",
			included:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			entry := &LogEntry{
				Timestamp:  time.Now().UnixNano(),
				LoggerName: tc.loggerName,
				Level:      slog.LevelInfo,
				Message:    "test message",
				Attributes: map[string]string{},
			}

			result := shouldIncludeLog(entry, []slog.Level{}, tc.filter)
			if result != tc.included {
				t.Errorf("Expected included=%v, got %v", tc.included, result)
			}
		})
	}
}

// TestShouldIncludeLogCombined verifies combined level + logger name filtering
func TestShouldIncludeLogCombined(t *testing.T) {
	entry := &LogEntry{
		Timestamp:  time.Now().UnixNano(),
		LoggerName: "com.example.service",
		Level:      slog.LevelWarn,
		Message:    "test message",
		Attributes: map[string]string{},
	}

	// Both level and logger match
	if !shouldIncludeLog(entry, []slog.Level{slog.LevelWarn}, "com.example.*") {
		t.Error("Expected true when both level and logger match")
	}

	// Level matches but logger doesn't
	if shouldIncludeLog(entry, []slog.Level{slog.LevelWarn}, "com.other.*") {
		t.Error("Expected false when logger doesn't match")
	}

	// Logger matches but level doesn't
	if shouldIncludeLog(entry, []slog.Level{slog.LevelError}, "com.example.*") {
		t.Error("Expected false when level doesn't match")
	}
}

// TestMatchesFilter verifies wildcard pattern matching
func TestMatchesFilter(t *testing.T) {
	testCases := []struct {
		loggerName string
		filter     string
		matches    bool
	}{
		{"myservice", "", true},
		{"myservice", "myservice", true},
		{"myservice", "yourservice", false},
		{"com.example.service", "com.example.*", true},
		{"com.example.subservice", "com.example.*", true},
		{"com.other.service", "com.example.*", false},
		{"io", "io*", true},
		{"ioutil", "io*", true},
		{"os", "io*", false},
	}

	for _, tc := range testCases {
		result := matchesFilter(tc.loggerName, tc.filter)
		if result != tc.matches {
			t.Errorf("matchesFilter(%q, %q): expected %v, got %v", tc.loggerName, tc.filter, tc.matches, result)
		}
	}
}

// TestBroadcasterHandlerFiltering verifies the broadcaster handler
func TestBroadcasterHandlerFiltering(t *testing.T) {
	broadcaster := NewLogBroadcaster()
	handler := NewBroadcasterHandler(broadcaster, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})

	if handler == nil {
		t.Fatal("Expected non-nil handler from NewBroadcasterHandler()")
	}
}

// TestCreateBroadcasterSink verifies sink creation with registry
func TestCreateBroadcasterSink(t *testing.T) {
	// Initialize the global registry
	InitializeGlobal(DefaultLoggingConfig())

	sinkName := "test-broadcaster-sink"
	broadcaster, err := CreateBroadcasterSink(sinkName)
	if err != nil {
		t.Fatalf("CreateBroadcasterSink() error: %v", err)
	}

	if broadcaster == nil {
		t.Fatal("Expected non-nil broadcaster")
	}

	// Verify sink is registered
	registry := GetRegistry()
	sink, exists := registry.GetSink(sinkName)
	if !exists {
		t.Errorf("Expected sink %q to be registered", sinkName)
	}
	if sink == nil {
		t.Errorf("Expected non-nil sink %q", sinkName)
	}
}

// TestAddRemoveSinkFromLoggers verifies sink attachment/detachment
func TestAddRemoveSinkFromLoggers(t *testing.T) {
	InitializeGlobal(DefaultLoggingConfig())
	registry := GetRegistry()

	// Create a test sink
	sinkName := "test-attach-sink"
	broadcaster, err := CreateBroadcasterSink(sinkName)
	if err != nil {
		t.Fatalf("CreateBroadcasterSink() error: %v", err)
	}

	// Create a test logger
	loggerName := "test.logger"

	// Initially logger should have no sinks
	_, existsBefore := registry.GetLoggerSinks(loggerName)
	if existsBefore {
		t.Logf("Logger %q already has sinks configured", loggerName)
	}

	// Add sink to logger
	err = AddSinkToLoggers(sinkName, []string{loggerName})
	if err != nil {
		t.Fatalf("AddSinkToLoggers() error: %v", err)
	}

	// Verify sink was added
	sinks, exists := registry.GetLoggerSinks(loggerName)
	if !exists {
		t.Errorf("Logger %q should have sinks after AddSinkToLoggers()", loggerName)
	}
	foundSink := false
	for _, s := range sinks {
		if s == sinkName {
			foundSink = true
			break
		}
	}
	if !foundSink {
		t.Errorf("Expected sink %q in logger %q, got %v", sinkName, loggerName, sinks)
	}

	// Remove sink from logger
	err = RemoveSinkFromLoggers(sinkName, []string{loggerName})
	if err != nil {
		t.Fatalf("RemoveSinkFromLoggers() error: %v", err)
	}

	// Verify sink was removed
	sinksAfter, _ := registry.GetLoggerSinks(loggerName)
	for _, s := range sinksAfter {
		if s == sinkName {
			t.Errorf("Expected sink %q to be removed from logger %q", sinkName, loggerName)
		}
	}

	_ = broadcaster
}

// TestMultipleBroadcasterSinks verifies multiple concurrent broadcaster sinks
func TestMultipleBroadcasterSinks(t *testing.T) {
	InitializeGlobal(DefaultLoggingConfig())

	// Create multiple broadcaster sinks (simulating multiple concurrent tail streams)
	sink1, err1 := CreateBroadcasterSink("broadcaster-1")
	sink2, err2 := CreateBroadcasterSink("broadcaster-2")

	if err1 != nil || err2 != nil {
		t.Fatalf("CreateBroadcasterSink() errors: %v, %v", err1, err2)
	}

	if sink1 == nil || sink2 == nil {
		t.Fatal("Expected non-nil broadcasters")
	}

	// Verify they are different instances
	if sink1 == sink2 {
		t.Error("Expected different broadcaster instances for different sessions")
	}

	// Test that one broadcaster doesn't interfere with the other
	ch1, unsub1 := sink1.Subscribe()
	ch2, unsub2 := sink2.Subscribe()
	defer unsub1()
	defer unsub2()

	entry := &LogEntry{
		Timestamp:  time.Now().UnixNano(),
		LoggerName: "test",
		Level:      slog.LevelInfo,
		Message:    "test",
		Attributes: map[string]string{},
	}

	sink1.Broadcast(entry)

	// sink1 subscriber should receive, sink2 should not
	received1 := false
	received2 := false

	select {
	case <-ch1:
		received1 = true
	case <-time.After(50 * time.Millisecond):
	}

	select {
	case <-ch2:
		received2 = true
	case <-time.After(50 * time.Millisecond):
	}

	if !received1 {
		t.Error("sink1 subscriber should have received broadcast")
	}
	if received2 {
		t.Error("sink2 subscriber should not have received broadcast from sink1")
	}
}
