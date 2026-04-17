package daemon

import (
	"context"
	"sync"
	"time"

	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	"github.com/Manu343726/buildozer/pkg/logging"
)

// RuntimeManager handles detection, caching, and querying of available runtimes.
// It lazy-loads runtimes on first request and caches the results.
type RuntimeManager struct {
	*logging.Logger

	mu               sync.RWMutex
	runtimes         []*v1.Runtime
	detectionNotes   string
	lastDetectionErr error
	detectedAt       *time.Time
	isDetecting      bool
}

// NewRuntimeManager creates a new runtime manager with lazy detection.
func NewRuntimeManager(daemonID ...string) *RuntimeManager {
	return &RuntimeManager{
		Logger: Log(daemonID...).Child("RuntimeManager"),
	}
}

// ListRuntimes returns all detected runtimes, performing detection on first call.
// On subsequent calls, returns cached results.
func (rm *RuntimeManager) ListRuntimes(ctx context.Context, filter string) ([]*v1.Runtime, string, error) {
	rm.mu.RLock()

	// If already detected, return cached results
	if rm.detectedAt != nil {
		defer rm.mu.RUnlock()
		rm.Debug("Returning cached runtimes", "count", len(rm.runtimes), "filter", filter)
		return rm.filterRuntimes(rm.runtimes, filter), rm.detectionNotes, rm.lastDetectionErr
	}

	// Check if detection is already in progress
	if rm.isDetecting {
		rm.mu.RUnlock()
		rm.Debug("Waiting for in-progress detection")
		return rm.waitForDetection(ctx, filter)
	}

	rm.mu.RUnlock()

	// Perform lazy detection
	return rm.detectAndCache(ctx, filter)
}

// detectAndCache performs runtime detection and caches the results.
func (rm *RuntimeManager) detectAndCache(ctx context.Context, filter string) ([]*v1.Runtime, string, error) {
	rm.mu.Lock()
	rm.isDetecting = true
	rm.mu.Unlock()

	defer func() {
		rm.mu.Lock()
		rm.isDetecting = false
		rm.mu.Unlock()
	}()

	rm.Info("Starting runtime detection")

	runtimes, detectionNotes, err := rm.detectRuntimes(ctx)

	rm.mu.Lock()
	rm.runtimes = runtimes
	rm.detectionNotes = detectionNotes
	rm.lastDetectionErr = err
	now := time.Now()
	rm.detectedAt = &now
	rm.mu.Unlock()

	if err != nil {
		rm.Error("Runtime detection failed", "error", err)
	} else {
		rm.Info("Runtime detection complete", "count", len(runtimes), "notes", detectionNotes)
	}

	return rm.filterRuntimes(runtimes, filter), detectionNotes, err
}

// waitForDetection waits for an in-progress detection to complete.
func (rm *RuntimeManager) waitForDetection(ctx context.Context, filter string) ([]*v1.Runtime, string, error) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, "", ctx.Err()
		case <-ticker.C:
			rm.mu.RLock()
			if rm.detectedAt != nil {
				defer rm.mu.RUnlock()
				return rm.filterRuntimes(rm.runtimes, filter), rm.detectionNotes, rm.lastDetectionErr
			}
			rm.mu.RUnlock()
		}
	}
}

// detectRuntimes detects all available runtimes.
func (rm *RuntimeManager) detectRuntimes(ctx context.Context) ([]*v1.Runtime, string, error) {
	// For now, this is a placeholder that will call the cpp native discoverer
	// when the daemon is properly integrated with runtime detection
	rm.Debug("Detecting C/C++ runtimes")

	// TODO: Integrate with pkg/runtimes/cpp/native discoverer
	// This requires moving detector/registry logic from CLI to shared package
	// or creating a discoverer that can be called from the daemon

	return []*v1.Runtime{}, "C/C++ runtime detection not yet implemented in daemon", nil
}

// filterRuntimes filters runtimes by toolchain type.
func (rm *RuntimeManager) filterRuntimes(runtimes []*v1.Runtime, filter string) []*v1.Runtime {
	if filter == "" {
		return runtimes
	}

	var filtered []*v1.Runtime
	for _, rt := range runtimes {
		if rm.matchesFilter(rt, filter) {
			filtered = append(filtered, rt)
		}
	}
	return filtered
}

// matchesFilter checks if a runtime matches the filter criteria.
func (rm *RuntimeManager) matchesFilter(rt *v1.Runtime, filter string) bool {
	switch filter {
	case "cpp", "c++":
		return rt.ToolchainSpec == (*v1.Runtime_Cpp)(nil) && rt.GetCpp() != nil
	case "go":
		return rt.ToolchainSpec == (*v1.Runtime_Go)(nil) && rt.GetGo() != nil
	case "rust":
		return rt.ToolchainSpec == (*v1.Runtime_Rust)(nil) && rt.GetRust() != nil
	default:
		return false
	}
}

// GetRuntime returns the runtime with the given ID.
func (rm *RuntimeManager) GetRuntime(ctx context.Context, runtimeID string) (*v1.Runtime, error) {
	runtimes, _, err := rm.ListRuntimes(ctx, "")
	if err != nil {
		return nil, err
	}

	for _, rt := range runtimes {
		if rt.Id == runtimeID {
			return rt, nil
		}
	}

	return nil, nil // Not found, but no error
}
