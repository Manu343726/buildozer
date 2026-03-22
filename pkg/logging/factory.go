package logging

import (
	"net/http"
)

// Convenience factory methods for creating ConfigManager implementations

// NewLocalConfigManagerFromGlobal creates a LocalConfigManager using the global registry
// This is a convenience method for common usage patterns
func NewLocalConfigManagerFromGlobal() *LocalConfigManager {
	registry := GetRegistry()
	return NewLocalConfigManager(registry)
}

// GetLocalConfigManager returns a LocalConfigManager using the global registry
// This is the simplest way to get a config manager for local operations
func GetLocalConfigManager() *LocalConfigManager {
	return NewLocalConfigManagerFromGlobal()
}

// Convenience export for registering the service with a standard HTTP mux
// Returns a function that can be used as an HTTP handler
// Usage:
//
//	mux := http.NewServeMux()
//	path, handler := logging.NewHTTPHandler(logging.GetLocalConfigManager())
//	mux.Handle(path, handler)
func NewHTTPHandler(configManager ConfigManager) (string, http.Handler) {
	return RegisterLoggingService(configManager)
}
