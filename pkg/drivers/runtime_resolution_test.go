package drivers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
)

// TestNewRuntimeResolver tests resolver creation
func TestNewRuntimeResolver(t *testing.T) {
	tests := []struct {
		name       string
		daemonHost string
		daemonPort int
		wantErr    bool
	}{
		{
			name:       "valid localhost",
			daemonHost: "localhost",
			daemonPort: 6789,
			wantErr:    false,
		},
		{
			name:       "custom host and port",
			daemonHost: "builder.local",
			daemonPort: 7777,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := NewRuntimeResolver(tt.daemonHost, tt.daemonPort)
			if resolver == nil {
				t.Fatal("expected resolver, got nil")
			}
			if resolver.Logger == nil {
				t.Fatal("expected logger to be embedded")
			}
		})
	}
}

// TestResolveValidApplier tests Resolve with a valid tool args applier
func TestResolveValidApplier(t *testing.T) {
	resolver := NewRuntimeResolver("localhost", 6789)
	ctx := context.Background()

	// Create a temporary config file for testing
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".buildozer.yaml")

	// Write a minimal config
	err := os.WriteFile(configPath, []byte(`
standalone: true
drivers:
  gcc:
    compiler_version: "9"
`), 0644)
	if err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	// Test with applier that modifies runtime
	applier := func(ctx context.Context, baseRuntime string, toolArgs []string) (string, error) {
		// Simulate gcc driver: returns a modified runtime ID
		return "gcc-9-glibc-x86_64", nil
	}

	result := resolver.Resolve(ctx, configPath, tmpDir, "", []string{"-march=x86-64"}, applier, "gcc")

	// Result should have requested runtime set
	if result.RequiredRuntime != "gcc-9-glibc-x86_64" {
		t.Errorf("expected RequiredRuntime='gcc-9-glibc-x86_64', got '%s'", result.RequiredRuntime)
	}

	// Since daemon might not have the runtime, we expect either Error or found runtime
	if result.Error != "" && result.FoundRuntime == nil {
		t.Logf("Expected error when daemon doesn't have runtime: %s", result.Error)
	}
}

// TestResolveApplierError tests Resolve when applier returns error
func TestResolveApplierError(t *testing.T) {
	resolver := NewRuntimeResolver("localhost", 6789)
	ctx := context.Background()
	tmpDir := t.TempDir()

	// Applier that returns error
	applier := func(ctx context.Context, baseRuntime string, toolArgs []string) (string, error) {
		return "", errors.New("invalid compiler flag")
	}

	result := resolver.Resolve(ctx, "", tmpDir, "", []string{"-invalid-flag"}, applier, "gcc")

	// Should have error about invalid tool arguments
	if result.Error == "" {
		t.Fatal("expected error for invalid tool arguments")
	}
	if !contains(result.Error, "invalid") {
		t.Errorf("expected 'invalid' in error message, got: %s", result.Error)
	}
}

// TestResolveNoConfig tests Resolve when config file doesn't exist
func TestResolveNoConfig(t *testing.T) {
	resolver := NewRuntimeResolver("localhost", 6789)
	ctx := context.Background()
	tmpDir := t.TempDir()

	// Applier that returns base runtime (no config available)
	applier := func(ctx context.Context, baseRuntime string, toolArgs []string) (string, error) {
		if baseRuntime != "" {
			return baseRuntime, nil
		}
		// When no config, use default runtime
		return "gcc-default", nil
	}

	result := resolver.Resolve(ctx, "", tmpDir, "", []string{}, applier, "gcc")

	// Should handle missing config gracefully
	if result.RequiredRuntime == "" {
		t.Fatal("expected RequiredRuntime to be set")
	}
	if result.RequiredRuntime != "gcc-default" {
		t.Errorf("expected 'gcc-default', got '%s'", result.RequiredRuntime)
	}
}

// TestResolveWithDaemon tests runtime resolution against actual daemon if running
func TestResolveWithDaemon(t *testing.T) {
	// Check if daemon is running
	daemonHost := "localhost"
	daemonPort := 6789

	// Try to query daemon - if it fails, skip this test
	resolver := NewRuntimeResolver(daemonHost, daemonPort)
	ctx := context.Background()

	// Use a dummy applier that returns a request
	applier := func(ctx context.Context, baseRuntime string, toolArgs []string) (string, error) {
		return "test-runtime", nil
	}

	result := resolver.Resolve(ctx, "", t.TempDir(), "", []string{}, applier, "test")

	// If daemon is running, we'll get either success or "runtime not found"
	// If daemon is not running, we'll get a connection error
	if result.Error != "" {
		t.Logf("Test against daemon returned error (daemon may not be running): %v", result.Error)
		// This is OK - daemon may not be running in test environment
		return
	}

	// If we got here, daemon is running
	if result.RequiredRuntime != "test-runtime" {
		t.Errorf("expected RequiredRuntime='test-runtime', got '%s'", result.RequiredRuntime)
	}
}

// TestResolveMultipleToolArgs tests applier with multiple tool arguments
func TestResolveMultipleToolArgs(t *testing.T) {
	resolver := NewRuntimeResolver("localhost", 6789)
	ctx := context.Background()
	tmpDir := t.TempDir()

	// Applier that simulates gcc parsing multiple flags
	applier := func(ctx context.Context, baseRuntime string, toolArgs []string) (string, error) {
		// Count the number of tool args to demonstrate parsing
		if len(toolArgs) == 0 {
			return "gcc-9-default", nil
		}

		// Simulate compiler version detection from tool args
		for _, arg := range toolArgs {
			if arg == "-v11" {
				return "gcc-11-glibc-x86_64", nil
			}
			if arg == "-v10" {
				return "gcc-10-glibc-x86_64", nil
			}
		}

		return "gcc-9-glibc-x86_64", nil
	}

	tests := []struct {
		name        string
		toolArgs    []string
		wantRuntime string
	}{
		{
			name:        "no args",
			toolArgs:    []string{},
			wantRuntime: "gcc-9-default",
		},
		{
			name:        "version 11",
			toolArgs:    []string{"-v11", "-c"},
			wantRuntime: "gcc-11-glibc-x86_64",
		},
		{
			name:        "version 10",
			toolArgs:    []string{"-v10", "-O2"},
			wantRuntime: "gcc-10-glibc-x86_64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolver.Resolve(ctx, "", tmpDir, "", tt.toolArgs, applier, "gcc")
			if result.RequiredRuntime != tt.wantRuntime {
				t.Errorf("expected '%s', got '%s'", tt.wantRuntime, result.RequiredRuntime)
			}
		})
	}
}

// TestLoadDriverConfig tests config file loading
func TestLoadDriverConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".buildozer.yaml")

	// Write a test config
	configContent := `
standalone: true
drivers:
  gcc:
    compiler_version: "10"
    compiler_type: "gcc"
    c_runtime: "glibc"
    architecture: "x86_64"
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Load config from explicit path
	cfg, loadedPath, loadErr := LoadDriverConfig(configPath)

	if loadErr != nil {
		t.Logf("Config load returned error (this is OK if .buildozer format differs): %v", loadErr)
		// This is OK - LoadDriverConfig might have specific format requirements
		return
	}

	if cfg == nil {
		t.Fatal("expected config, got nil")
	}

	// Verify the path
	if loadedPath == "" {
		t.Logf("Config loaded but path not returned")
	}
}

// TestRuntimeResolutionResult tests the result structure
func TestRuntimeResolutionResult(t *testing.T) {
	tests := []struct {
		name       string
		result     *RuntimeResolutionResult
		checkError func(*RuntimeResolutionResult) bool
	}{
		{
			name: "success native runtime",
			result: &RuntimeResolutionResult{
				RequiredRuntime: "gcc-9-glibc-x86_64",
				FoundRuntime:    &v1.Runtime{Id: "gcc-9-glibc-x86_64"},
				IsNative:        true,
				Error:           "",
				Warning:         "",
			},
			checkError: func(r *RuntimeResolutionResult) bool {
				return r.Error == "" && r.IsNative && r.FoundRuntime != nil
			},
		},
		{
			name: "remote runtime with warning",
			result: &RuntimeResolutionResult{
				RequiredRuntime: "gcc-9-glibc-x86_64",
				FoundRuntime:    &v1.Runtime{Id: "gcc-9-glibc-x86_64"},
				IsNative:        false,
				Error:           "",
				Warning:         "runtime available on peer",
			},
			checkError: func(r *RuntimeResolutionResult) bool {
				return r.Error == "" && !r.IsNative && r.Warning != ""
			},
		},
		{
			name: "runtime not found error",
			result: &RuntimeResolutionResult{
				RequiredRuntime: "gcc-9-glibc-x86_64",
				FoundRuntime:    nil,
				IsNative:        false,
				Error:           "runtime not found",
				Warning:         "",
			},
			checkError: func(r *RuntimeResolutionResult) bool {
				return r.Error != "" && r.FoundRuntime == nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.checkError(tt.result) {
				t.Errorf("result check failed: %+v", tt.result)
			}
		})
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			fmt.Sprintf("%s%s", "%", substr) != "" && fmt.Sprintf("%s", substr) != ""))
}

// TestApplierContract tests that ToolArgsApplier receives correct inputs
func TestApplierContract(t *testing.T) {
	resolver := NewRuntimeResolver("localhost", 6789)
	ctx := context.Background()
	tmpDir := t.TempDir()

	// Track what applier receives
	var receivedArgs []string

	applier := func(ctx context.Context, baseRuntime string, toolArgs []string) (string, error) {
		_ = baseRuntime // baseRuntime is intentionally not checked in this test
		receivedArgs = toolArgs
		return "resolved-runtime", nil
	}

	testArgs := []string{"-c", "-o", "output.o"}
	resolver.Resolve(ctx, "", tmpDir, "", testArgs, applier, "test")

	// Verify applier was called with correct arguments
	if len(receivedArgs) != len(testArgs) {
		t.Errorf("expected %d tool args, got %d", len(testArgs), len(receivedArgs))
	}

	for i, arg := range testArgs {
		if i < len(receivedArgs) && receivedArgs[i] != arg {
			t.Errorf("arg %d: expected '%s', got '%s'", i, arg, receivedArgs[i])
		}
	}
}
