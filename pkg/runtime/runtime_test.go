// Package runtime_test contains tests for the runtime package.
package runtime

import (
	"context"
	"testing"
)

// MockRuntime is a test implementation of the Runtime interface.
type MockRuntime struct {
	id       string
	metadata *Metadata
}

// NewMockRuntime creates a new mock runtime for testing.
func NewMockRuntime(id string, language string) *MockRuntime {
	return &MockRuntime{
		id: id,
		metadata: &Metadata{
			RuntimeID:   id,
			Language:    language,
			RuntimeType: "mock",
			Version:     "1.0.0",
			TargetOS:    "linux",
			TargetArch:  "x86_64",
		},
	}
}

func (m *MockRuntime) Execute(ctx context.Context, req *ExecutionRequest) (*ExecutionResult, error) {
	return &ExecutionResult{ExitCode: 0, Stdout: []byte("mock output")}, nil
}

func (m *MockRuntime) Available(ctx context.Context) (bool, error) {
	return true, nil
}

func (m *MockRuntime) Metadata(ctx context.Context) (*Metadata, error) {
	return m.metadata, nil
}

func (m *MockRuntime) RuntimeID() string {
	return m.id
}

// TestRegistryRegister tests registering runtimes in the registry.
func TestRegistryRegister(t *testing.T) {
	registry := NewRegistry()

	rt := NewMockRuntime("test-runtime-1", "c")
	if err := registry.Register(rt); err != nil {
		t.Fatalf("failed to register runtime: %v", err)
	}

	if registry.Count() != 1 {
		t.Fatalf("expected 1 runtime, got %d", registry.Count())
	}
}

// TestRegistryGet tests retrieving runtimes from the registry.
func TestRegistryGet(t *testing.T) {
	registry := NewRegistry()

	rt := NewMockRuntime("test-runtime-1", "c")
	registry.Register(rt)

	retrieved := registry.Get("test-runtime-1")
	if retrieved == nil {
		t.Fatalf("expected to retrieve runtime, got nil")
	}

	if retrieved.RuntimeID() != "test-runtime-1" {
		t.Fatalf("expected ID 'test-runtime-1', got %q", retrieved.RuntimeID())
	}
}

// TestRegistryDuplicateRegister tests that registering the same runtime twice fails.
func TestRegistryDuplicateRegister(t *testing.T) {
	registry := NewRegistry()

	rt := NewMockRuntime("test-runtime-1", "c")
	if err := registry.Register(rt); err != nil {
		t.Fatalf("first register failed: %v", err)
	}

	rt2 := NewMockRuntime("test-runtime-1", "c")
	if err := registry.Register(rt2); err == nil {
		t.Fatalf("expected error when registering duplicate runtime")
	}
}

// TestRegistryFindByLanguage tests finding runtimes by language.
func TestRegistryFindByLanguage(t *testing.T) {
	registry := NewRegistry()

	rt1 := NewMockRuntime("gcc-11", "c")
	rt2 := NewMockRuntime("gcc-12", "c")
	rt3 := NewMockRuntime("go-1.21", "go")

	registry.Register(rt1)
	registry.Register(rt2)
	registry.Register(rt3)

	matches, err := registry.FindByLanguage(context.Background(), "c")
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if len(matches) != 2 {
		t.Fatalf("expected 2 C runtimes, got %d", len(matches))
	}

	// Check that Go runtime is not in C results
	for _, rt := range matches {
		meta, _ := rt.Metadata(context.Background())
		if meta.Language != "c" {
			t.Fatalf("expected language 'c', got %q", meta.Language)
		}
	}
}
