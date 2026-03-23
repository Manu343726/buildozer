package runtimes

import (
	"context"
	"os"
	"sync"
	"time"

	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	"github.com/Manu343726/buildozer/pkg/logging"
	"github.com/Manu343726/buildozer/pkg/runtime"
	"github.com/Manu343726/buildozer/pkg/runtimes/cpp/native"
)

// Manager handles detection and caching of available runtimes.
// Stores the actual runtime.Runtime implementations for later execution.
// Proto conversion happens on-demand when needed for API responses.
type Manager struct {
	*logging.Logger // Embed Logger for logging module operations

	mu               sync.RWMutex
	runtimes         []runtime.Runtime // Store actual runtime objects
	detectionNotes   string
	lastDetectionErr error
	detectedAt       *time.Time
	isDetecting      bool
}

// NewManager creates a new runtime manager.
func NewManager() *Manager {
	return &Manager{
		Logger: Log().Child("Manager"),
	}
}

// ListRuntimes returns all detected runtimes as proto messages.
// Performs detection on first call and caches results.
func (m *Manager) ListRuntimes(ctx context.Context) ([]*v1.Runtime, string, error) {
	runtimes, notes, err := m.GetRuntimes(ctx)
	if err != nil {
		return nil, notes, err
	}

	// Convert runtime.Runtime objects to proto messages
	var protoRuntimes []*v1.Runtime
	for _, rt := range runtimes {
		proto, err := m.runtimeToProto(ctx, rt)
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
func (m *Manager) GetRuntimes(ctx context.Context) ([]runtime.Runtime, string, error) {
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
func (m *Manager) detectAndCacheSuper(ctx context.Context) ([]runtime.Runtime, string, error) {
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
func (m *Manager) waitForDetectionSuper(ctx context.Context) ([]runtime.Runtime, string, error) {
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

func (m *Manager) detectRuntimesSuper(ctx context.Context) ([]runtime.Runtime, string, error) {
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

// runtimeToProto converts a runtime.Runtime to a v1.Runtime proto message.
// Fully populates all toolchain details for protocol serialization.
func (m *Manager) runtimeToProto(ctx context.Context, rt runtime.Runtime) (*v1.Runtime, error) {
	meta, err := rt.Metadata(ctx)
	if err != nil {
		return nil, err
	}

	protoRT := &v1.Runtime{
		Id:       rt.RuntimeID(),
		IsNative: meta.IsNative,
	}

	if meta.Description != "" {
		protoRT.Description = &meta.Description
	}

	// Handle C/C++ runtimes - fully populate CppToolchain
	if meta.Language == "c" || meta.Language == "cpp" {
		// For NativeCppRuntime, extract full toolchain details
		if nativeRuntime, ok := rt.(*native.NativeCppRuntime); ok {
			toolchain := nativeRuntime.GetToolchain()

			cppToolchain := &v1.CppToolchain{
				Language:        nativeRuntime.ProtoLanguage(),
				Compiler:        nativeRuntime.ProtoCompiler(),
				CompilerVersion: native.ParseVersionString(toolchain.CompilerVersion),
				Architecture:    nativeRuntime.ProtoArchitecture(),
				CRuntime:        nativeRuntime.ProtoCRuntime(),
				CRuntimeVersion: native.ParseVersionString(toolchain.CRuntimeVersion),
				CppAbi:          nativeRuntime.ProtoCppAbi(),
				CppStdlib:       nativeRuntime.ProtoCppStdlib(),
				AbiModifiers:    toolchain.AbiModifiers,
			}

			protoRT.Toolchain = &v1.Runtime_Cpp{
				Cpp: cppToolchain,
			}
		} else {
			// Fallback for non-native C/C++ runtimes: use metadata only
			cppLang := v1.CppLanguage_CPP_LANGUAGE_UNSPECIFIED
			if meta.Language == "c" {
				cppLang = v1.CppLanguage_CPP_LANGUAGE_C
			} else if meta.Language == "cpp" {
				cppLang = v1.CppLanguage_CPP_LANGUAGE_CPP
			}

			protoRT.Toolchain = &v1.Runtime_Cpp{
				Cpp: &v1.CppToolchain{
					Language:        cppLang,
					CompilerVersion: native.ParseVersionString(meta.Version),
				},
			}
		}
	} else if meta.Language == "go" {
		// TODO: Support Go runtimes
	} else if meta.Language == "rust" {
		// TODO: Support Rust runtimes
	}

	return protoRT, nil
}

// GetRuntimeByID returns the runtime implementation with the given ID.
// Returns the actual runtime.Runtime object for execution.
func (m *Manager) GetRuntimeByID(ctx context.Context, runtimeID string) (runtime.Runtime, error) {
	runtimes, _, err := m.GetRuntimes(ctx)
	if err != nil {
		return nil, err
	}

	for _, rt := range runtimes {
		if rt.RuntimeID() == runtimeID {
			return rt, nil
		}
	}

	return nil, nil // Not found, but no error
}
