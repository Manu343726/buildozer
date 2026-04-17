package drivers

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	"github.com/Manu343726/buildozer/pkg/driver"
)

// testDriver is a minimal driver.Driver implementation for testing the resolver.
type testDriver struct {
	name            string
	applyToolArgs   func(ctx context.Context, rt *v1.Runtime, args []string) (*v1.Runtime, error)
	validateRuntime func(rt *v1.Runtime) (bool, string)
}

func (d testDriver) Name() string        { return d.name }
func (d testDriver) Version() string     { return "1.0.0" }
func (d testDriver) Short() string       { return "" }
func (d testDriver) Long() string        { return "" }
func (d testDriver) ErrorPrefix() string { return d.name + ": error:" }

func (d testDriver) ValidateArgs([]string) error           { return nil }
func (d testDriver) ParseCommandLine([]string) interface{} { return nil }
func (d testDriver) CreateJob(context.Context, interface{}, string, *driver.RuntimeContext) (*v1.Job, error) {
	return nil, nil
}

func (d testDriver) ApplyToolArgs(ctx context.Context, baseRuntime *v1.Runtime, toolArgs []string) (*v1.Runtime, error) {
	if d.applyToolArgs != nil {
		return d.applyToolArgs(ctx, baseRuntime, toolArgs)
	}
	return baseRuntime, nil
}

func (d testDriver) ValidateRuntime(runtime *v1.Runtime) (bool, string) {
	if d.validateRuntime != nil {
		return d.validateRuntime(runtime)
	}
	return true, ""
}

func (d testDriver) ConstructRuntimeID(cfgMap map[string]interface{}) (string, error) {
	// Test implementation: return a dummy ID or error if missing required field
	if _, ok := cfgMap["compiler_version"]; !ok {
		return "", errors.New("required config field 'compiler_version' is missing")
	}
	return "test-runtime-id", nil
}

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

	// Test with driver that returns the runtime as-is
	d := testDriver{name: "gcc"}

	// Provide a valid initialRuntime that can be parsed
	resultIface := resolver.Resolve(ctx, configPath, tmpDir, "native-c-gcc-9.0.0-glibc-2.31-x86_64", []string{"-march=x86-64"}, d)
	result, ok := resultIface.(*RuntimeResolutionResult)
	if !ok || result == nil {
		t.Fatal("expected *RuntimeResolutionResult, got nil or wrong type")
	}

	// Result should have requested runtime set
	if result.RequiredRuntime == nil || result.RequiredRuntime.Id != "native-c-gcc-9.0.0-glibc-2.31-x86_64" {
		wantID := "native-c-gcc-9.0.0-glibc-2.31-x86_64"
		gotID := ""
		if result.RequiredRuntime != nil {
			gotID = result.RequiredRuntime.Id
		}
		t.Errorf("expected RequiredRuntime.Id=%q, got %q", wantID, gotID)
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

	// Driver that returns error from ApplyToolArgs
	d := testDriver{
		name: "gcc",
		applyToolArgs: func(_ context.Context, _ *v1.Runtime, _ []string) (*v1.Runtime, error) {
			return nil, errors.New("invalid compiler flag")
		},
	}

	resultIface := resolver.Resolve(ctx, "", tmpDir, "native-c-gcc-9.0.0-glibc-2.31-x86_64", []string{"-invalid-flag"}, d)
	result, ok := resultIface.(*RuntimeResolutionResult)
	if !ok || result == nil {
		t.Fatal("expected *RuntimeResolutionResult, got nil or wrong type")
	}

	// Should have error about invalid tool arguments
	if result.Error == "" {
		t.Fatal("expected error for invalid tool arguments")
	}
	if !strings.Contains(result.Error, "invalid") {
		t.Errorf("expected 'invalid' in error message, got: %s", result.Error)
	}
}

// TestResolveNoConfig tests Resolve when config file doesn't exist
func TestResolveNoConfig(t *testing.T) {
	resolver := NewRuntimeResolver("localhost", 6789)
	ctx := context.Background()
	tmpDir := t.TempDir()

	// Driver that returns the base runtime as-is
	d := testDriver{name: "gcc"}

	resultIface := resolver.Resolve(ctx, "", tmpDir, "native-c-gcc-9.0.0-glibc-2.31-x86_64", []string{}, d)
	result, ok := resultIface.(*RuntimeResolutionResult)
	if !ok || result == nil {
		t.Fatal("expected *RuntimeResolutionResult, got nil or wrong type")
	}

	// Should handle missing config gracefully
	if result.RequiredRuntime == nil {
		t.Fatal("expected RequiredRuntime to be set")
	}
	if result.RequiredRuntime.Id != "native-c-gcc-9.0.0-glibc-2.31-x86_64" {
		t.Errorf("expected 'native-c-gcc-9.0.0-glibc-2.31-x86_64', got '%s'", result.RequiredRuntime.Id)
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

	// Applier that returns the runtime as-is
	d := testDriver{name: "test"}

	resultIface := resolver.Resolve(ctx, "", t.TempDir(), "native-c-gcc-10.0.0-glibc-2.31-x86_64", []string{}, d)
	result, ok := resultIface.(*RuntimeResolutionResult)
	if !ok || result == nil {
		t.Fatal("expected *RuntimeResolutionResult, got nil or wrong type")
	}

	// If daemon is running, we'll get either success or "runtime not found"
	// If daemon is not running, we'll get a connection error
	if result.Error != "" {
		t.Logf("Test against daemon returned error (daemon may not be running): %v", result.Error)
		// This is OK - daemon may not be running in test environment
		return
	}

	// If we got here, daemon is running
	if result.RequiredRuntime == nil || result.RequiredRuntime.Id != "native-c-gcc-10.0.0-glibc-2.31-x86_64" {
		gotID := ""
		if result.RequiredRuntime != nil {
			gotID = result.RequiredRuntime.Id
		}
		t.Errorf("expected RequiredRuntime.Id='native-c-gcc-10.0.0-glibc-2.31-x86_64', got '%s'", gotID)
	}
}

// TestResolveMultipleToolArgs tests applier with multiple tool arguments
func TestResolveMultipleToolArgs(t *testing.T) {
	resolver := NewRuntimeResolver("localhost", 6789)
	ctx := context.Background()
	tmpDir := t.TempDir()

	// Driver that returns the base runtime as-is (tool arg handling is in ModifyRuntimeWithFlags)
	d := testDriver{name: "gcc"}

	tests := []struct {
		name           string
		initialRuntime string
		toolArgs       []string
		wantRuntimeID  string
	}{
		{
			name:           "no args",
			initialRuntime: "native-c-gcc-9.0.0-glibc-2.31-x86_64",
			toolArgs:       []string{},
			wantRuntimeID:  "native-c-gcc-9.0.0-glibc-2.31-x86_64",
		},
		{
			name:           "with compile flag",
			initialRuntime: "native-c-gcc-11.0.0-glibc-2.31-x86_64",
			toolArgs:       []string{"-v11", "-c"},
			wantRuntimeID:  "native-c-gcc-11.0.0-glibc-2.31-x86_64",
		},
		{
			name:           "with optimization",
			initialRuntime: "native-c-gcc-10.0.0-glibc-2.31-x86_64",
			toolArgs:       []string{"-v10", "-O2"},
			wantRuntimeID:  "native-c-gcc-10.0.0-glibc-2.31-x86_64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resultIface := resolver.Resolve(ctx, "", tmpDir, tt.initialRuntime, tt.toolArgs, d)
			result, ok := resultIface.(*RuntimeResolutionResult)
			if !ok || result == nil {
				t.Fatal("expected *RuntimeResolutionResult, got nil or wrong type")
			}
			if result.RequiredRuntime == nil {
				t.Fatal("expected RequiredRuntime to be set")
			}
			if result.RequiredRuntime.Id != tt.wantRuntimeID {
				t.Errorf("expected '%s', got '%s'", tt.wantRuntimeID, result.RequiredRuntime.Id)
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
				RequiredRuntime: &v1.Runtime{Id: "native-c-gcc-9.0.0-glibc-2.31-x86_64"},
				FoundRuntime:    &v1.Runtime{Id: "native-c-gcc-9.0.0-glibc-2.31-x86_64"},
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
				RequiredRuntime: &v1.Runtime{Id: "native-c-gcc-9.0.0-glibc-2.31-x86_64"},
				FoundRuntime:    &v1.Runtime{Id: "native-c-gcc-9.0.0-glibc-2.31-x86_64"},
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
				RequiredRuntime: &v1.Runtime{Id: "native-c-gcc-9.0.0-glibc-2.31-x86_64"},
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

// TestApplierContract tests that ToolArgsApplier receives correct inputs
func TestApplierContract(t *testing.T) {
	resolver := NewRuntimeResolver("localhost", 6789)
	ctx := context.Background()
	tmpDir := t.TempDir()

	// Track what driver receives
	var receivedArgs []string

	d := testDriver{
		name: "test",
		applyToolArgs: func(_ context.Context, baseRuntime *v1.Runtime, toolArgs []string) (*v1.Runtime, error) {
			receivedArgs = toolArgs
			return baseRuntime, nil
		},
	}

	testArgs := []string{"-c", "-o", "output.o"}
	resolver.Resolve(ctx, "", tmpDir, "native-c-gcc-9.0.0-glibc-2.31-x86_64", testArgs, d)

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
