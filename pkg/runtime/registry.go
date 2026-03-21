// Package runtime provides abstractions for job execution environments.
package runtime

import (
	"context"
	"fmt"
	"sync"
)

// Registry manages the collection of available runtimes and provides
// lookup and matching capabilities.
type Registry struct {
	mu       sync.RWMutex
	runtimes map[string]Runtime
}

// NewRegistry creates a new empty runtime registry.
func NewRegistry() *Registry {
	return &Registry{
		runtimes: make(map[string]Runtime),
	}
}

// Register adds a runtime to the registry.
// Returns an error if a runtime with the same ID already exists.
func (r *Registry) Register(runtime Runtime) error {
	if runtime == nil {
		return fmt.Errorf("cannot register nil runtime")
	}

	id := runtime.RuntimeID()
	if id == "" {
		return fmt.Errorf("runtime ID cannot be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.runtimes[id]; exists {
		return fmt.Errorf("runtime with ID %q already registered", id)
	}

	r.runtimes[id] = runtime
	return nil
}

// Get returns a runtime by its ID.
// Returns nil if the runtime is not found.
func (r *Registry) Get(id string) Runtime {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.runtimes[id]
}

// All returns all registered runtimes.
func (r *Registry) All() []Runtime {
	r.mu.RLock()
	defer r.mu.RUnlock()

	runtimes := make([]Runtime, 0, len(r.runtimes))
	for _, rt := range r.runtimes {
		runtimes = append(runtimes, rt)
	}
	return runtimes
}

// Find returns all runtimes matching the given filter function.
func (r *Registry) Find(filter func(Runtime) bool) []Runtime {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var matches []Runtime
	for _, rt := range r.runtimes {
		if filter(rt) {
			matches = append(matches, rt)
		}
	}
	return matches
}

// FindByLanguage returns runtimes matching the given language.
func (r *Registry) FindByLanguage(ctx context.Context, language string) ([]Runtime, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var matches []Runtime
	for _, rt := range r.runtimes {
		meta, err := rt.Metadata(ctx)
		if err != nil {
			continue // Skip runtimes we can't query
		}
		if meta.Language == language {
			matches = append(matches, rt)
		}
	}
	return matches, nil
}

// Available returns all available runtimes (those passing the Available() check).
func (r *Registry) Available(ctx context.Context) ([]Runtime, error) {
	r.mu.RLock()
	runtimesCopy := make([]Runtime, 0, len(r.runtimes))
	for _, rt := range r.runtimes {
		runtimesCopy = append(runtimesCopy, rt)
	}
	r.mu.RUnlock()

	var available []Runtime
	for _, rt := range runtimesCopy {
		ok, err := rt.Available(ctx)
		if err == nil && ok {
			available = append(available, rt)
		}
	}
	return available, nil
}

// Count returns the number of registered runtimes.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.runtimes)
}
