package runtimes

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	"github.com/Manu343726/buildozer/pkg/logging"
	"github.com/Manu343726/buildozer/pkg/peers"
	"github.com/Manu343726/buildozer/pkg/runtime"
	"github.com/Manu343726/buildozer/pkg/runtimes/cpp/native"
	"github.com/Manu343726/buildozer/pkg/runtimes/remote"
)

// Manager is an interface for runtime discovery and retrieval.
// Implementations can provide local runtimes, remote peer runtimes, or aggregated runtimes.
type Manager interface {
	// ListRuntimes returns all available runtimes as proto messages.
	// Returns (runtimes, notes, error).
	ListRuntimes(ctx context.Context) ([]*v1.Runtime, string, error)

	// GetRuntimes returns all available runtime.Runtime implementations.
	// Returns (runtimes, notes, error).
	GetRuntimes(ctx context.Context) ([]runtime.Runtime, string, error)

	// GetRuntimeByID returns the runtime implementation with the given ID.
	GetRuntimeByID(ctx context.Context, runtimeID string) (runtime.Runtime, error)

	// Match returns all runtimes matching the given RuntimeMatchQuery.
	Match(ctx context.Context, query *v1.RuntimeMatchQuery) ([]*v1.Runtime, error)
}

// LocalRuntimesManager handles detection and caching of local runtimes.
// Stores the actual runtime.Runtime implementations for later execution.
// Proto conversion happens on-demand when needed for API responses.
type LocalRuntimesManager struct {
	*logging.Logger // Embed Logger for logging module operations

	daemonID         string
	mu               sync.RWMutex
	runtimes         []runtime.Runtime // Store actual runtime objects
	detectionNotes   string
	lastDetectionErr error
	detectedAt       *time.Time
	isDetecting      bool
}

// NewLocalRuntimesManager creates a new local runtime manager.
func NewLocalRuntimesManager(daemonID string) *LocalRuntimesManager {
	return &LocalRuntimesManager{
		Logger:   Log().Child("LocalRuntimesManager"),
		daemonID: daemonID,
	}
}

// ListRuntimes returns all detected runtimes as proto messages.
// Performs detection on first call and caches results.
func (m *LocalRuntimesManager) ListRuntimes(ctx context.Context) ([]*v1.Runtime, string, error) {
	runtimes, notes, err := m.GetRuntimes(ctx)
	if err != nil {
		return nil, notes, err
	}

	// Convert runtime.Runtime objects to proto messages
	var protoRuntimes []*v1.Runtime
	for _, rt := range runtimes {
		proto, err := rt.Proto(ctx)
		if err != nil {
			m.Warn("Failed to convert runtime to proto", "id", rt.RuntimeID(), "error", err)
			continue
		}
		protoRuntimes = append(protoRuntimes, proto)
	}

	return protoRuntimes, notes, nil
}

// GetRuntimes returns all detected runtime.Runtime implementations.
// Performs detection on first call and caches results.
func (m *LocalRuntimesManager) GetRuntimes(ctx context.Context) ([]runtime.Runtime, string, error) {
	m.mu.RLock()

	// If already detected, return cached results
	if m.detectedAt != nil {
		defer m.mu.RUnlock()
		m.Debug("Returning cached runtimes", "count", len(m.runtimes))
		return m.runtimes, m.detectionNotes, m.lastDetectionErr
	}

	// Check if detection is already in progress
	if m.isDetecting {
		m.mu.RUnlock()
		m.Debug("Waiting for in-progress detection")
		return m.waitForDetectionSuper(ctx)
	}

	m.mu.RUnlock()

	// Perform lazy detection
	return m.detectAndCacheSuper(ctx)
}

// detectAndCacheSuper performs runtime detection and caches the results.
// Returns actual runtime.Runtime implementations.
func (m *LocalRuntimesManager) detectAndCacheSuper(ctx context.Context) ([]runtime.Runtime, string, error) {
	m.mu.Lock()
	m.isDetecting = true
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		m.isDetecting = false
		m.mu.Unlock()
	}()

	m.Info("Starting runtime detection")

	runtimes, detectionNotes, err := m.detectRuntimesSuper(ctx)

	m.mu.Lock()
	m.runtimes = runtimes
	m.detectionNotes = detectionNotes
	m.lastDetectionErr = err
	now := time.Now()
	m.detectedAt = &now
	m.mu.Unlock()

	if err != nil {
		m.Error("Runtime detection failed", "error", err)
	} else {
		m.Info("Runtime detection complete", "count", len(runtimes), "notes", detectionNotes)
	}

	return runtimes, detectionNotes, err
}

// waitForDetectionSuper waits for an in-progress detection to complete.
func (m *LocalRuntimesManager) waitForDetectionSuper(ctx context.Context) ([]runtime.Runtime, string, error) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, "", ctx.Err()
		case <-ticker.C:
			m.mu.RLock()
			if m.detectedAt != nil {
				defer m.mu.RUnlock()
				return m.runtimes, m.detectionNotes, m.lastDetectionErr
			}
			m.mu.RUnlock()
		}
	}
}

func (m *LocalRuntimesManager) detectRuntimesSuper(ctx context.Context) ([]runtime.Runtime, string, error) {
	m.Debug("Detecting runtimes using registered discoverers")

	// Create array of discoverers for different runtime types
	// Each discoverer is responsible for finding and registering runtimes of its type
	discoverers := []runtime.Discoverer{
		native.NewCppDiscoverer(os.TempDir()),
		// TODO: Add discoverers for other languages
		// docker.NewDockerCppDiscoverer(),
		// golang.NewGoDiscoverer(),
		// rust.NewRustDiscoverer(),
	}

	// Combine discoverers into a multi-discoverer
	multiDiscoverer := runtime.NewMultiDiscoverer(discoverers...)

	// Create a registry for discovered runtimes
	reg := runtime.NewRegistry()

	// Run discovery
	if err := multiDiscoverer.Discover(ctx, reg); err != nil {
		m.Warn("Runtime discovery had errors", "error", err)
		// Continue - some discoverers may have succeeded
	}

	// Return actual runtime objects
	discoveredRuntimes := reg.All()

	if len(discoveredRuntimes) == 0 {
		return discoveredRuntimes, "No runtimes detected on system", nil
	}

	return discoveredRuntimes, "Runtime detection complete", nil
}

// GetRuntimeByID returns the runtime implementation with the given ID.
// Returns the actual runtime.Runtime object for execution.
func (m *LocalRuntimesManager) GetRuntimeByID(ctx context.Context, runtimeID string) (runtime.Runtime, error) {
	runtimes, _, err := m.GetRuntimes(ctx)
	if err != nil {
		return nil, err
	}

	for _, rt := range runtimes {
		id := rt.RuntimeID()
		if id == runtimeID {
			m.Debug("Found runtime by ID", "id", runtimeID)
			return rt, nil
		} else {
			m.Debug("Runtime ID does not match", "expected_id", runtimeID, "runtime_id", id)
		}
	}

	return nil, nil // Not found, but no error
}

// Match returns all runtimes matching the given RuntimeMatchQuery.
// Filters runtimes by platform, toolchain, and specific parameters.
func (m *LocalRuntimesManager) Match(ctx context.Context, query *v1.RuntimeMatchQuery) ([]*v1.Runtime, error) {
	runtimes, _, err := m.GetRuntimes(ctx)
	if err != nil {
		return nil, err
	}

	if query == nil {
		return nil, fmt.Errorf("match query cannot be nil")
	}

	var result []*v1.Runtime

	for _, rt := range runtimes {
		id := rt.RuntimeID()
		if matches, err := rt.MatchesQuery(ctx, query); err != nil {
			m.Warn("Error checking runtime against query", "runtime_id", id, "error", err)
		} else if matches {
			if proto, err := rt.Proto(ctx); err != nil {
				m.Warn("Failed to convert matching runtime to proto", "id", id, "error", err)
			} else {
				result = append(result, proto)
			}
		}
	}

	m.Debug("Runtime match query completed", "query_platforms", query.Platforms, "query_toolchains", query.Toolchains, "matches_found", len(result))

	return result, nil
}

// AggregatedRuntimesManager combines local and remote runtimes.
// It provides unified access to runtime discovery from both local and peer daemons.
// GetRuntimes and GetRuntimeByID only return local executable runtimes.
// ListRuntimes aggregates runtimes from both local and remote sources for discovery.
type AggregatedRuntimesManager struct {
	*logging.Logger

	localManager  Manager
	remoteManager Manager
}

// NewAggregatedRuntimesManager creates a new aggregated runtime manager.
// It combines runtime discovery from local and remote managers.
func NewAggregatedRuntimesManager(localManager, remoteManager Manager) *AggregatedRuntimesManager {
	return &AggregatedRuntimesManager{
		Logger:        Log().Child("AggregatedRuntimesManager"),
		localManager:  localManager,
		remoteManager: remoteManager,
	}
}

// ListRuntimes returns all runtimes (local and remote) as proto messages.
// Aggregates proto runtimes from both local discovery and remote peers.
// Deduplicates by runtime ID.
func (m *AggregatedRuntimesManager) ListRuntimes(ctx context.Context) ([]*v1.Runtime, string, error) {
	// Get local proto runtimes
	localProtos, localNotes, localErr := m.localManager.ListRuntimes(ctx)
	if localErr != nil {
		m.Warn("Error getting local runtimes", "error", localErr)
	}

	// Get remote proto runtimes from remote manager
	remoteProtos, _, remoteErr := m.remoteManager.ListRuntimes(ctx)
	if remoteErr != nil {
		m.Warn("Error getting remote runtimes", "error", remoteErr)
	}

	// Deduplicate and combine runtimes, merging peer_ids for each runtime
	runtimeMap := make(map[string]*v1.Runtime)

	// Add local runtimes to map
	for _, rt := range localProtos {
		if rt != nil && rt.Id != "" {
			runtimeMap[rt.Id] = rt
		}
	}

	// Add remote runtimes, merging peer_ids if the same runtime exists locally
	for _, rt := range remoteProtos {
		if rt != nil && rt.Id != "" {
			if existing, exists := runtimeMap[rt.Id]; exists {
				// Merge peer_ids lists - avoid duplicates
				peerIDSet := make(map[string]bool)
				for _, pid := range existing.PeerIds {
					peerIDSet[pid] = true
				}
				for _, pid := range rt.PeerIds {
					if !peerIDSet[pid] {
						existing.PeerIds = append(existing.PeerIds, pid)
						peerIDSet[pid] = true
					}
				}
			} else {
				// First time seeing this runtime, add it
				runtimeMap[rt.Id] = rt
			}
		}
	}

	// Convert map back to slice
	allRuntimes := make([]*v1.Runtime, 0, len(runtimeMap))
	for _, rt := range runtimeMap {
		allRuntimes = append(allRuntimes, rt)
	}

	notes := localNotes
	if len(runtimeMap) > len(localProtos) {
		notes += " (includes " + string(rune(len(allRuntimes)-len(localProtos))) + " peer runtimes)"
	}

	m.Debug("Aggregated runtime discovery", "local_count", len(localProtos), "remote_count", len(remoteProtos), "total_count", len(allRuntimes))
	return allRuntimes, notes, localErr
}

// GetRuntimes returns only local executable runtime.Runtime implementations.
// Remote runtimes cannot be executed locally, only discovered via ListRuntimes.
func (m *AggregatedRuntimesManager) GetRuntimes(ctx context.Context) ([]runtime.Runtime, string, error) {
	// Only return local runtimes (remote runtimes cannot be executed)
	return m.localManager.GetRuntimes(ctx)
}

// GetRuntimeByID returns a local runtime by ID.
// Remote runtimes cannot be executed locally, only discovered via ListRuntimes.
func (m *AggregatedRuntimesManager) GetRuntimeByID(ctx context.Context, runtimeID string) (runtime.Runtime, error) {
	// Only return local runtime (remote runtimes cannot be executed)
	return m.localManager.GetRuntimeByID(ctx, runtimeID)
}

// Match returns all local runtimes matching the given RuntimeMatchQuery.
// Only returns local runtimes (remote runtimes cannot be executed locally).
func (m *AggregatedRuntimesManager) Match(ctx context.Context, query *v1.RuntimeMatchQuery) ([]*v1.Runtime, error) {
	// For discovery purposes, aggregate matches from both local and remote runtimes.
	// Get matches from local manager
	localMatches, localErr := m.localManager.Match(ctx, query)
	if localErr != nil {
		m.Warn("Local runtime matching failed", "error", localErr)
	}

	// Get matches from remote manager
	remoteMatches, remoteErr := m.remoteManager.Match(ctx, query)
	if remoteErr != nil {
		m.Warn("Remote runtime matching failed", "error", remoteErr)
	}

	// Aggregate and deduplicate by runtime ID
	matchMap := make(map[string]*v1.Runtime)
	for _, rt := range localMatches {
		matchMap[rt.Id] = rt
	}
	for _, rt := range remoteMatches {
		if _, exists := matchMap[rt.Id]; !exists {
			matchMap[rt.Id] = rt
		}
	}

	var aggregated []*v1.Runtime
	for _, rt := range matchMap {
		aggregated = append(aggregated, rt)
	}

	m.Debug("Runtime match aggregation complete", "local_count", len(localMatches), "remote_count", len(remoteMatches), "total_count", len(aggregated))

	// Return aggregated results, reporting any errors
	if localErr != nil && remoteErr != nil {
		return aggregated, localErr // Return an error if both failed
	}
	return aggregated, nil
}

// NewRemoteRuntimesManager creates a new remote runtime manager.
// It wraps remote daemon runtimes with RPC implementations for transparent remote execution.
func NewRemoteRuntimesManager(peerManager *peers.Manager) Manager {
	return remote.NewManager(peerManager)
}
