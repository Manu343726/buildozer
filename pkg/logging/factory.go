package logging

import (
	"net/http"

	"github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1/protov1connect"
)

// Convenience factory methods for creating ConfigManager implementations

// NewLocalConfigManagerFromGlobal creates a LocalConfigManager using the global registry
// This is a convenience method for common usage patterns
func NewLocalConfigManagerFromGlobal() *LocalConfigManager {
	registry := GetRegistry()
	factory := NewFactory(registry)
	return NewLocalConfigManager(registry, factory)
}

// NewRemoteConfigManagerFromURL creates a RemoteConfigManager pointing to a remote daemon
// httpClient: HTTP client to use for requests (can be http.DefaultClient)
// baseURL: Base URL of the remote daemon (e.g., "http://localhost:6789")
func NewRemoteConfigManagerFromURL(httpClient *http.Client, baseURL string) *RemoteConfigManager {
	return NewRemoteConfigManager(httpClient, baseURL)
}

// NewRemoteConfigManagerFromClient creates a RemoteConfigManager with an explicit client
// This is useful when you need more control over the client configuration
func NewRemoteConfigManagerFromClient(client protov1connect.LoggingServiceClient) *RemoteConfigManager {
	return NewRemoteConfigManagerWithClient(client)
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
