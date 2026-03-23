package toolchain

import (
	"context"
	"testing"
	"time"

	"github.com/Manu343726/buildozer/pkg/runtimes/cpp/native"
)

func TestRegistryInitialize(t *testing.T) {
	registry := NewRegistry()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := registry.Initialize(ctx)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	toolchains := registry.ListToolchains()
	if len(toolchains) == 0 {
		t.Skip("No toolchains detected on this system")
	}

	for _, tc := range toolchains {
		t.Logf("Found toolchain: %s", tc.String())
	}
}

func TestRegistryGetGCC(t *testing.T) {
	registry := NewRegistry()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := registry.Initialize(ctx)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	gcc := registry.GetGCC(ctx)
	if gcc == nil {
		t.Skip("GCC not available on this system")
	}

	if gcc.Compiler != native.CompilerGCC {
		t.Errorf("expected GCC compiler, got %v", gcc.Compiler)
	}

	if gcc.Language != native.LanguageC {
		t.Errorf("expected C language for GCC, got %v", gcc.Language)
	}

	t.Logf("Found GCC: %s", gcc.String())
}

func TestRegistryGetGxx(t *testing.T) {
	registry := NewRegistry()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := registry.Initialize(ctx)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	gxx := registry.GetGxx(ctx)
	if gxx == nil {
		t.Skip("G++ not available on this system")
	}

	if gxx.Compiler != native.CompilerGCC {
		t.Errorf("expected GCC compiler for G++, got %v", gxx.Compiler)
	}

	if gxx.Language != native.LanguageCpp {
		t.Errorf("expected C++ language for G++, got %v", gxx.Language)
	}

	t.Logf("Found G++: %s", gxx.String())
}

func TestRegistryCanExecute(t *testing.T) {
	registry := NewRegistry()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := registry.Initialize(ctx)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	toolchains := registry.ListToolchains()
	if len(toolchains) == 0 {
		t.Skip("No toolchains detected on this system")
	}

	// Test with the first available toolchain
	tc := toolchains[0]
	canExecute := registry.CanExecute(tc.Compiler, tc.Language, native.ArchitectureUnspecified)
	if !canExecute {
		t.Errorf("expected CanExecute to return true for available toolchain")
	}

	// Test with compiler that shouldn't exist
	canExecute = registry.CanExecute(native.CompilerUnspecified, native.LanguageUnspecified, native.ArchitectureUnspecified)
	if canExecute {
		t.Errorf("expected CanExecute to return false for unspecified compiler/language")
	}
}

func TestRegistrySummary(t *testing.T) {
	registry := NewRegistry()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := registry.Initialize(ctx)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	summary := registry.Summary()
	if summary == "" {
		t.Errorf("expected non-empty summary")
	}

	t.Logf("Toolchain summary: %s", summary)
}

func TestRegistryListToolchains(t *testing.T) {
	registry := NewRegistry()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := registry.Initialize(ctx)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	toolchains := registry.ListToolchains()
	if len(toolchains) == 0 {
		t.Skip("No toolchains detected on this system")
	}

	for _, tc := range toolchains {
		if tc == nil {
			t.Errorf("found nil toolchain in list")
			continue
		}

		if tc.Compiler == native.CompilerUnspecified {
			t.Errorf("toolchain has unspecified compiler")
		}

		if tc.Language == native.LanguageUnspecified {
			t.Errorf("toolchain has unspecified language")
		}
	}
}

func TestGlobalRegistry(t *testing.T) {
	registry := Global()
	if registry == nil {
		t.Fatalf("Global() returned nil")
	}

	// Should return same instance on subsequent calls
	registry2 := Global()
	if registry != registry2 {
		t.Errorf("Global() returned different instances")
	}
}
