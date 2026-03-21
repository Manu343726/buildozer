// Package runtime provides abstractions for job execution environments.
package runtime

import "context"

// Discoverer defines the interface for discovering runtimes in the system.
// Implementations can discover native system toolchains, Docker images, or other environments.
type Discoverer interface {
	// Discover finds and registers available runtimes.
	// The runtimes found are added to the registry.
	// ctx is used for cancellation and timeouts.
	Discover(ctx context.Context, registry *Registry) error

	// Name returns the name of this discoverer (e.g., "native-cpp", "docker-cpp").
	Name() string
}
