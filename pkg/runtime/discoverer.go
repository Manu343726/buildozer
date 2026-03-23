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

// multiDiscoverer wraps multiple discoverers and implements the Discoverer interface.
// It calls each discoverer in sequence and merges their results into a single registry.
type multiDiscoverer struct {
	discoverers []Discoverer
}

// NewMultiDiscoverer creates a new multi-discoverer from a list of discoverers.
// Returns a Discoverer interface to allow for flexibility and encapsulation.
func NewMultiDiscoverer(discoverers ...Discoverer) Discoverer {
	return &multiDiscoverer{
		discoverers: discoverers,
	}
}

// Discover calls each wrapped discoverer and merges their results.
// Returns an error if any discoverer fails; successful discoveries are still registered.
func (md *multiDiscoverer) Discover(ctx context.Context, registry *Registry) error {
	var lastErr error
	for _, d := range md.discoverers {
		if err := d.Discover(ctx, registry); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// Name returns a composite name of all wrapped discoverers.
func (md *multiDiscoverer) Name() string {
	if len(md.discoverers) == 0 {
		return "multi-discoverer-empty"
	}
	if len(md.discoverers) == 1 {
		return md.discoverers[0].Name()
	}
	return "multi-discoverer"
}
