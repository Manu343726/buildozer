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
	"github.com/Manu343726/buildozer/pkg/logging"
)

// RuntimeResolutionResult contains the outcome of runtime resolution:
// - The requested runtime specification
// - Whether the daemon has it available
// - Whether it's available natively on this daemon
// - Warning if available remotely/Docker but not natively
// - Error if not available at all
type RuntimeResolutionResult struct {
	// RequiredRuntime is the runtime ID requested after merging config + tool args
	RequiredRuntime string

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

// ToolArgsApplier is a callback function that driver-specific code implements.
// It takes a base runtime (from config) and tool arguments, and returns the
// modified runtime ID that should be requested from the daemon.
// For example, gcc driver would parse -march, compiler version flags, etc.
//
// baseRuntime: the initial runtime from config (may be empty if not configured)
// toolArgs: the tool-specific arguments that may modify the runtime
// return: the final runtime ID to request from daemon, or error if invalid args
type ToolArgsApplier func(ctx context.Context, baseRuntime string, toolArgs []string) (string, error)

// RuntimeValidator is a callback function that driver-specific code implements.
// It checks if a given runtime (returned from daemon) is compatible with the driver.
// For example, gcc driver only accepts C/C++ toolchains that support C language.
//
// runtime: the runtime to validate
// return: (isCompatible, reason)
type RuntimeValidator func(runtime *v1.Runtime) (bool, string)

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
// 4. Validates that a base runtime was found (required)
// 5. Applies tool args via callback to enhance the runtime ID
// 6. Queries daemon for final runtime
// 7. Classifies availability (native, remote, not found)
// 8. Returns result with warnings or errors
//
// configPath: explicit path to .buildozer file (empty = search from cwd)
// startDir: directory to start upward search from (if configPath empty)
// initialRuntime: CLI-provided initial runtime (overrides config)
// toolArgs: driver-specific arguments that may affect runtime
// applier: driver-specific callback to apply tool args to runtime spec
// driverName: name of driver for logging (e.g., "gcc", "g++")
func (rr *RuntimeResolver) Resolve(
	ctx context.Context,
	configPath string,
	startDir string,
	initialRuntime string,
	toolArgs []string,
	applier ToolArgsApplier,
	driverName string,
) *RuntimeResolutionResult {
	rr.InfoContext(ctx, "Starting runtime resolution", "driver", driverName)

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

	// Step 2: Get base runtime from config (depends on driver type)
	// Note: The driver-specific code should extract the appropriate config.
	// Here we pass empty string; driver-specific code handles config parsing.
	// For example, gcc driver would use cfg.Drivers.Gcc runtime settings.
	baseRuntime := ""
	if cfg != nil {
		// Placeholder: driver-specific code extracts from config
		// baseRuntime = cfg.Drivers.Gcc.Runtime (or similar)
	}

	rr.DebugContext(ctx, "Base runtime from config",
		"config", configFile, "baseRuntime", baseRuntime)

	// Step 3: Apply CLI override if provided
	// CLI initialRuntime takes precedence over config
	if initialRuntime != "" {
		rr.InfoContext(ctx, "Using CLI-provided initial runtime", "runtime", initialRuntime)
		baseRuntime = initialRuntime
	}

	// Step 4: Check if we have an initial runtime to work with
	// The applier is for enhancing/modifying an existing runtime, not creating one from scratch
	if baseRuntime == "" {
		rr.ErrorContext(ctx, "No initial runtime found", "config", configFile, "cli", initialRuntime)
		return &RuntimeResolutionResult{
			RequiredRuntime: "",
			Error:           "unable to determine compiler runtime: no configuration file found and no explicit compiler version/architecture specified in command-line flags",
		}
	}

	// Step 5: Apply tool args to enhance the runtime
	rr.DebugContext(ctx, "Applying tool arguments", "baseRuntime", baseRuntime, "toolArgs", toolArgs)
	requestedRuntime, applyErr := applier(ctx, baseRuntime, toolArgs)
	if applyErr != nil {
		rr.ErrorContext(ctx, "Tool args validation failed", "error", applyErr)
		return &RuntimeResolutionResult{
			RequiredRuntime: baseRuntime,
			Error:           fmt.Sprintf("invalid tool arguments: %v", applyErr),
		}
	}

	rr.InfoContext(ctx, "Runtime requested from daemon", "runtime", requestedRuntime)

	// Step 6: Query daemon
	foundRuntime, isNative, daemonErr := rr.queryDaemon(ctx, requestedRuntime)
	if daemonErr != nil {
		rr.ErrorContext(ctx, "Failed to query daemon", "error", daemonErr)
		return &RuntimeResolutionResult{
			RequiredRuntime: requestedRuntime,
			FoundRuntime:    foundRuntime,
			IsNative:        isNative,
			Error:           fmt.Sprintf("daemon query failed: %v", daemonErr),
		}
	}

	// Step 7: Classify result
	if foundRuntime == nil {
		rr.ErrorContext(ctx, "Runtime not found", "runtime", requestedRuntime)
		return &RuntimeResolutionResult{
			RequiredRuntime: requestedRuntime,
			FoundRuntime:    nil,
			IsNative:        false,
			Error:           fmt.Sprintf("runtime '%s' not found on daemon", requestedRuntime),
		}
	}

	if isNative {
		rr.InfoContext(ctx, "Runtime available natively", "runtime", requestedRuntime)
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
		requestedRuntime,
	)
	rr.WarnContext(ctx, "Runtime not available natively", "runtime", requestedRuntime)
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
// based on the provided validator function. Returns only runtimes compatible with the driver.
func (rr *RuntimeResolver) ListCompatibleRuntimes(
	ctx context.Context,
	validator RuntimeValidator,
	driverName string,
) ([]*v1.Runtime, error) {
	daemonURL := fmt.Sprintf("http://%s:%d", rr.daemonHost, rr.daemonPort)
	client := protov1connect.NewRuntimeServiceClient(http.DefaultClient, daemonURL)

	rr.InfoContext(ctx, "Querying daemon for available runtimes", "driver", driverName)

	resp, err := client.ListRuntimes(ctx, connect.NewRequest(&v1.ListRuntimesRequest{}))
	if err != nil {
		rr.ErrorContext(ctx, "Failed to query daemon for runtimes", "error", err)
		return nil, fmt.Errorf("daemon RPC failed: %w", err)
	}

	allRuntimes := resp.Msg.Runtimes
	rr.InfoContext(ctx, "Daemon returned runtimes", "count", len(allRuntimes), "notes", resp.Msg.DetectionNotes)

	// Filter runtimes based on validator
	var compatibleRuntimes []*v1.Runtime
	for _, runtime := range allRuntimes {
		isCompatible, reason := validator(runtime)
		if isCompatible {
			compatibleRuntimes = append(compatibleRuntimes, runtime)
			rr.DebugContext(ctx, "Runtime compatible", "runtimeID", runtime.Id)
		} else {
			rr.DebugContext(ctx, "Runtime incompatible", "runtimeID", runtime.Id, "reason", reason)
		}
	}

	rr.InfoContext(ctx, "Filtered compatible runtimes", "driver", driverName, "compatible", len(compatibleRuntimes), "total", len(allRuntimes))
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
