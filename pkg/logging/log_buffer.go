package logging

import (
	"log/slog"
	"sync"
)

// LogEntry represents a single log entry
type LogEntry struct {
	Timestamp  int64 // Unix nanoseconds
	LoggerName string
	Level      slog.Level
	Message    string
	Attributes map[string]string
}

// LogBroadcaster manages broadcasting logs to multiple subscribers
// Each subscriber receives a channel that gets new log entries
type LogBroadcaster struct {
	mu          sync.RWMutex
	subscribers map[int]chan *LogEntry // map of subscriber ID to channel
	nextSubID   int
}

// NewLogBroadcaster creates a new log broadcaster
func NewLogBroadcaster() *LogBroadcaster {
	return &LogBroadcaster{
		subscribers: make(map[int]chan *LogEntry),
	}
}

// Broadcast sends a log entry to all active subscribers
func (lb *LogBroadcaster) Broadcast(entry *LogEntry) {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	for _, ch := range lb.subscribers {
		select {
		case ch <- entry:
		default:
			// Skip if channel is full (non-blocking, don't block the logger)
		}
	}
}

// Subscribe creates a new subscription channel for live log entries
// Returns a channel and a function to unsubscribe.
func (lb *LogBroadcaster) Subscribe() (<-chan *LogEntry, func()) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	subID := lb.nextSubID
	lb.nextSubID++

	ch := make(chan *LogEntry, 100) // buffer size for slower consumers
	lb.subscribers[subID] = ch

	unsubscribe := func() {
		lb.mu.Lock()
		defer lb.mu.Unlock()
		if subCh, exists := lb.subscribers[subID]; exists {
			close(subCh)
			delete(lb.subscribers, subID)
		}
	}

	return ch, unsubscribe
}

// CloseAllSubscribers closes all active subscriber channels
func (lb *LogBroadcaster) CloseAllSubscribers() {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	for _, ch := range lb.subscribers {
		close(ch)
	}
	lb.subscribers = make(map[int]chan *LogEntry)
}
