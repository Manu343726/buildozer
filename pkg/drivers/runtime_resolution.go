// Package drivers provides shared driver infrastructure and utilities.
package drivers

import (
	"context"
	"fmt"
	"net/http"

	"connectrpc.com/connect"
	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	"github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1/protov1connect"
	"github.com/Manu343726/buildozer/pkg/config"
	"github.com/Manu343726/buildozer/pkg/driver"
	"github.com/Manu343726/buildozer/pkg/logging"
	pkgruntime "github.com/Manu343726/buildozer/pkg/runtime"
)

// RuntimeResolutionResult contains the outcome of runtime resolution:
// - The requested runtime specification
// - Whether the daemon has it available
// - Whether it's available natively on this daemon
// - Warning if available remotely/Docker but not natively
// - Error if not available at all
type RuntimeResolutionResult struct {
	// RequiredRuntime is the runtime descriptor requested after merging config + tool args
	RequiredRuntime *v1.Runtime

	// FoundRuntime is the actual runtime proto returned by daemon (nil if not found)
	FoundRuntime *v1.Runtime

	// IsNative indicates if the runtime is available as a native runtime on this daemon
	IsNative bool

	// Warning is a non-nil string if the runtime is available but not natively
	// (e.g., available on a peer or as a Docker image)
	Warning string

	// Error is a non-nil string if the runtime cannot be resolved at all
	Error string
}

// RuntimeResolver handles the driver-agnostic aspects of runtime resolution:
// 1. Load config from .buildozer file (upward search)
// 2. Merge config runtime + tool-args-modified runtime
// 3. Query daemon for requested runtime
// 4. Classify availability (not found, native, remote/docker)
// 5. Return result with appropriate warnings/errors
type RuntimeResolver struct {
	*logging.Logger

	daemonHost string
	daemonPort int
}

// NewRuntimeResolver creates a new runtime resolver with the given daemon address.
func NewRuntimeResolver(daemonHost string, daemonPort int) *RuntimeResolver {
	return &RuntimeResolver{
		Logger:     logging.Log().Child("RuntimeResolver"),
		daemonHost: daemonHost,
		daemonPort: daemonPort,
	}
}

// Resolve performs the complete runtime resolution workflow:
// 1. Loads .buildozer config from cwd upward (or explicit path)
// 2. Extracts base runtime from config (driver-specific)
// 3. Merges config runtime with CLI initialRuntime (CLI overrides config)
// 4. Parses the base runtime ID to a *v1.Runtime descriptor
// 5. Applies tool args via the driver's ApplyToolArgs method
// 6. Queries daemon for final runtime
// 7. Classifies availability (native, remote, not found)
// 8. Returns result with warnings or errors
//
// configPath: explicit path to .buildozer file (empty = search from cwd)
// startDir: directory to start upward search from (if configPath empty)
// initialRuntime: CLI-provided initial runtime ID (overrides config)
// toolArgs: driver-specific arguments that may affect runtime
// d: the driver implementation (used for ApplyToolArgs and Name)
func (rr *RuntimeResolver) Resolve(
	ctx context.Context,
	configPath string,
	startDir string,
	initialRuntime string,
	toolArgs []string,
	d driver.Driver,
) *RuntimeResolutionResult {
	rr.InfoContext(ctx, "Starting runtime resolution", "driver", d.Name())

	// Step 1: Load config
	var cfg *config.Config
	var configFile string
	var configErr error

	if configPath != "" {
		rr.DebugContext(ctx, "Loading explicit config", "path", configPath)
		cfg, configFile, configErr = config.LoadDriverConfig(configPath)
	} else {
		rr.DebugContext(ctx, "Searching for config from directory", "startDir", startDir)
		cfg, configFile, configErr = config.LoadDriverConfig(startDir)
	}

	if configErr != nil {
		rr.WarnContext(ctx, "Failed to load config, using defaults", "error", configErr)
		cfg = nil
	}

	if configFile != "" {
		rr.InfoContext(ctx, "Config loaded", "path", configFile)
	}

	// Step 2: Get base runtime ID from config (depends on driver type)
	baseRuntimeID := ""
	if cfg != nil {
		// Placeholder: driver-specific code extracts from config
		// baseRuntimeID = cfg.Drivers.Gcc.Runtime (or similar)
	}

	rr.DebugContext(ctx, "Base runtime from config",
		"config", configFile, "baseRuntime", baseRuntimeID)

	// Step 3: Apply CLI override if provided
	if initialRuntime != "" {
		rr.InfoContext(ctx, "Using CLI-provided initial runtime", "runtime", initialRuntime)
		baseRuntimeID = initialRuntime
	}

	// Step 4: Check if we have an initial runtime to work with
	if baseRuntimeID == "" {
		rr.ErrorContext(ctx, "No initial runtime found", "config", configFile, "cli", initialRuntime)
		return &RuntimeResolutionResult{
			RequiredRuntime: nil,
			Error:           "unable to determine compiler runtime: no configuration file found and no explicit compiler version/architecture specified in command-line flags",
		}
	}

	// Step 5: Parse runtime ID string into a *v1.Runtime descriptor
	baseRuntime, parseErr := pkgruntime.ParseRuntimeID(baseRuntimeID)
	if parseErr != nil {
		rr.ErrorContext(ctx, "Failed to parse runtime ID", "runtimeID", baseRuntimeID, "error", parseErr)
		return &RuntimeResolutionResult{
			RequiredRuntime: nil,
			Error:           fmt.Sprintf("invalid runtime ID %q: %v", baseRuntimeID, parseErr),
		}
	}

	// Step 6: Apply tool args via driver method
	rr.DebugContext(ctx, "Applying tool arguments", "baseRuntime", baseRuntime.Id, "toolArgs", toolArgs)
	requestedRuntime, applyErr := d.ApplyToolArgs(ctx, baseRuntime, toolArgs)
	if applyErr != nil {
		rr.ErrorContext(ctx, "Tool args validation failed", "error", applyErr)
		return &RuntimeResolutionResult{
			RequiredRuntime: baseRuntime,
			Error:           fmt.Sprintf("invalid tool arguments: %v", applyErr),
		}
	}

	rr.InfoContext(ctx, "Runtime requested from daemon", "runtime", requestedRuntime.Id)

	// Step 7: Query daemon using the runtime ID
	foundRuntime, isNative, daemonErr := rr.queryDaemon(ctx, requestedRuntime.Id)
	if daemonErr != nil {
		rr.ErrorContext(ctx, "Failed to query daemon", "error", daemonErr)
		return &RuntimeResolutionResult{
			RequiredRuntime: requestedRuntime,
			FoundRuntime:    foundRuntime,
			IsNative:        isNative,
			Error:           fmt.Sprintf("daemon query failed: %v", daemonErr),
		}
	}

	// Step 8: Classify result
	if foundRuntime == nil {
		rr.ErrorContext(ctx, "Runtime not found", "runtime", requestedRuntime.Id)
		return &RuntimeResolutionResult{
			RequiredRuntime: requestedRuntime,
			FoundRuntime:    nil,
			IsNative:        false,
			Error:           fmt.Sprintf("runtime '%s' not found on daemon", requestedRuntime.Id),
		}
	}

	if isNative {
		rr.InfoContext(ctx, "Runtime available natively", "runtime", requestedRuntime.Id)
		return &RuntimeResolutionResult{
			RequiredRuntime: requestedRuntime,
			FoundRuntime:    foundRuntime,
			IsNative:        true,
		}
	}

	// Runtime available but not natively
	warning := fmt.Sprintf(
		"runtime '%s' is available on a peer or as a Docker image, but not natively on this machine. "+
			"Job execution will use remote runtime.",
		requestedRuntime.Id,
	)
	rr.WarnContext(ctx, "Runtime not available natively", "runtime", requestedRuntime.Id)
	return &RuntimeResolutionResult{
		RequiredRuntime: requestedRuntime,
		FoundRuntime:    foundRuntime,
		IsNative:        false,
		Warning:         warning,
	}
}

// queryDaemon queries the daemon for a specific runtime and determines if it's native.
// Returns (runtime, isNative, error)
func (rr *RuntimeResolver) queryDaemon(
	ctx context.Context,
	runtimeID string,
) (*v1.Runtime, bool, error) {
	daemonURL := fmt.Sprintf("http://%s:%d", rr.daemonHost, rr.daemonPort)
	client := protov1connect.NewRuntimeServiceClient(http.DefaultClient, daemonURL)

	rr.DebugContext(ctx, "Querying daemon for runtime", "url", daemonURL, "runtimeID", runtimeID)

	resp, err := client.GetRuntime(ctx, connect.NewRequest(&v1.GetRuntimeRequest{
		RuntimeId: runtimeID,
	}))
	if err != nil {
		return nil, false, fmt.Errorf("daemon RPC failed: %w", err)
	}

	if resp.Msg.Error != nil {
		return nil, false, fmt.Errorf("daemon error: %s", *resp.Msg.Error)
	}

	runtime := resp.Msg.Runtime
	if runtime == nil {
		return nil, false, nil // Runtime not found
	}

	// Determine if runtime is native
	// A runtime is native if it's not a Docker runtime and not a peer runtime
	// For now, we check if the runtime doesn't have Docker-specific markers
	// (This is simplified; actual implementation depends on how remote/docker runtimes are marked)
	isNative := isNativeRuntime(runtime)

	rr.DebugContext(ctx, "Daemon returned runtime", "runtimeID", runtimeID, "isNative", isNative)

	return runtime, isNative, nil
}

// ListCompatibleRuntimes queries the daemon for all available runtimes and filters them
// using the driver's ValidateRuntime method. Returns only runtimes compatible with the driver.
func (rr *RuntimeResolver) ListCompatibleRuntimes(
	ctx context.Context,
	d driver.Driver,
) ([]*v1.Runtime, error) {
	daemonURL := fmt.Sprintf("http://%s:%d", rr.daemonHost, rr.daemonPort)
	client := protov1connect.NewRuntimeServiceClient(http.DefaultClient, daemonURL)

	rr.InfoContext(ctx, "Querying daemon for available runtimes", "driver", d.Name())

	resp, err := client.ListRuntimes(ctx, connect.NewRequest(&v1.ListRuntimesRequest{}))
	if err != nil {
		rr.ErrorContext(ctx, "Failed to query daemon for runtimes", "error", err)
		return nil, fmt.Errorf("daemon RPC failed: %w", err)
	}

	allRuntimes := resp.Msg.Runtimes
	rr.InfoContext(ctx, "Daemon returned runtimes", "count", len(allRuntimes), "notes", resp.Msg.DetectionNotes)

	// Filter runtimes based on driver's ValidateRuntime method
	var compatibleRuntimes []*v1.Runtime
	for _, runtime := range allRuntimes {
		isCompatible, reason := d.ValidateRuntime(runtime)
		if isCompatible {
			compatibleRuntimes = append(compatibleRuntimes, runtime)
			rr.DebugContext(ctx, "Runtime compatible", "runtimeID", runtime.Id)
		} else {
			rr.DebugContext(ctx, "Runtime incompatible", "runtimeID", runtime.Id, "reason", reason)
		}
	}

	rr.InfoContext(ctx, "Filtered compatible runtimes", "driver", d.Name(), "compatible", len(compatibleRuntimes), "total", len(allRuntimes))
	return compatibleRuntimes, nil
}

// isNativeRuntime determines if a runtime is native (not Docker, not remote peer)
// This is a helper function that encodes the logic for determining native runtimes.
// Currently simplified; actual implementation may need more sophisticated detection.
func isNativeRuntime(runtime *v1.Runtime) bool {
	if runtime == nil {
		return false
	}

	// TODO: Implement proper detection based on runtime type
	// For now, assume all runtimes are native (placeholder)
	// Future: Check if runtime has Docker marker, peer marker, etc.
	return true
}

// LoadDriverConfig is a convenience function that wraps config.LoadDriverConfig
// with proper upward search from startDir.
func LoadDriverConfig(startDir string) (*config.Config, string, error) {
	// If empty startDir, use current directory
	if startDir == "" {
		startDir = "."
	}

	// This wraps the existing config.LoadDriverConfig which does upward search
	return config.LoadDriverConfig(startDir)
}
