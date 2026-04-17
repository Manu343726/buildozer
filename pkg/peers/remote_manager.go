package peers

import (
	"context"
	"errors"
	"fmt"

	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	"github.com/Manu343726/buildozer/pkg/logging"
	"github.com/Manu343726/buildozer/pkg/runtime"
)

// RemoteRuntimesManager provides runtimes available from remote peer daemons.
// It queries the peer manager to retrieve runtime information from other nodes in the network.
// Note: RemoteRuntimesManager only supports ListRuntimes for discovery.
// GetRuntimes and GetRuntimeByID return errors since remote runtimes cannot be executed locally.
type RemoteRuntimesManager struct {
	*logging.Logger

	peerManager *Manager
}

// NewRemoteRuntimesManager creates a new remote runtime manager.
func NewRemoteRuntimesManager(peerManager *Manager) *RemoteRuntimesManager {
	return &RemoteRuntimesManager{
		Logger:      logging.Log("peers").Child("RemoteRuntimesManager"),
		peerManager: peerManager,
	}
}

// ListRuntimes returns all runtimes from remote peers as proto messages.
// Returns from remote peers only, excluding local daemon runtimes.
func (m *RemoteRuntimesManager) ListRuntimes(ctx context.Context) ([]*v1.Runtime, string, error) {
	if m.peerManager == nil {
		return nil, "No peer manager configured", nil
	}

	// Get all peers and collect their runtimes
	allPeers := m.peerManager.GetAllPeersIncludingLocal()

	remotePeerCount := 0
	var allRuntimes []*v1.Runtime

	for _, peer := range allPeers {
		if !peer.IsLocal && len(peer.Runtimes) > 0 {
			remotePeerCount++
			allRuntimes = append(allRuntimes, peer.Runtimes...)
		}
	}

	notes := "Retrieved runtimes from " + string(rune(remotePeerCount)) + " remote peers"
	m.Debug("Queried remote peers", "peer_count", remotePeerCount, "runtime_count", len(allRuntimes))

	return allRuntimes, notes, nil
}

// GetRuntimes returns error - remote runtimes cannot be executed locally.
// Use ListRuntimes for remote runtime discovery.
func (m *RemoteRuntimesManager) GetRuntimes(ctx context.Context) ([]runtime.Runtime, string, error) {
	return nil, "", errorRemoteRuntimeExecution("GetRuntimes")
}

// GetRuntimeByID returns error - remote runtimes cannot be executed locally.
// Use ListRuntimes for remote runtime discovery.
func (m *RemoteRuntimesManager) GetRuntimeByID(ctx context.Context, runtimeID string) (runtime.Runtime, error) {
	return nil, errorRemoteRuntimeExecution("GetRuntimeByID")
}

// Match returns runtimes from remote peers matching the given query.
// Remote runtimes can be discovered and matched for scheduling purposes,
// even though they cannot be executed locally.
func (m *RemoteRuntimesManager) Match(ctx context.Context, query *v1.RuntimeMatchQuery) ([]*v1.Runtime, error) {
	if query == nil {
		return nil, fmt.Errorf("match query cannot be nil")
	}

	// List all remote runtimes
	runtimes, _, err := m.ListRuntimes(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list remote runtimes for matching: %w", err)
	}

	var matches []*v1.Runtime

	for _, rt := range runtimes {
		if m.runtimeMatchesQuery(rt, query) {
			matches = append(matches, rt)
		}
	}

	m.Debug("Remote runtime match query completed", "query_platforms", query.Platforms, "query_toolchains", query.Toolchains, "matches_found", len(matches))

	return matches, nil
}

// runtimeMatchesQuery checks if a runtime matches all constraints in the query.
func (m *RemoteRuntimesManager) runtimeMatchesQuery(rt *v1.Runtime, query *v1.RuntimeMatchQuery) bool {
	// Check platform constraint
	if len(query.Platforms) > 0 {
		platformMatches := false
		for _, qp := range query.Platforms {
			if rt.Platform == qp {
				platformMatches = true
				break
			}
		}
		if !platformMatches {
			return false
		}
	}

	// Check toolchain constraint
	if len(query.Toolchains) > 0 {
		toolchainMatches := false
		for _, qt := range query.Toolchains {
			if rt.Toolchain == qt {
				toolchainMatches = true
				break
			}
		}
		if !toolchainMatches {
			return false
		}
	}

	// Check specific parameters if provided
	if len(query.Params) > 0 {
		runtimeParams := m.extractRuntimeParameters(rt)
		if !m.parametersMatch(runtimeParams, query.Params) {
			return false
		}
	}

	return true
}

// extractRuntimeParameters extracts key runtime parameters from a Runtime.
// Returns a map of parameter name -> list of values.
func (m *RemoteRuntimesManager) extractRuntimeParameters(rt *v1.Runtime) map[string][]string {
	params := make(map[string][]string)

	// Add platform parameter
	params["platform"] = []string{rt.Platform.String()}

	// Add toolchain parameter
	params["toolchain"] = []string{rt.Toolchain.String()}

	// Extract toolchain-specific parameters
	switch ts := rt.ToolchainSpec.(type) {
	case *v1.Runtime_Cpp:
		if ts.Cpp != nil {
			// C runtime
			params["c_runtime"] = []string{ts.Cpp.CRuntime.String()}

			// C runtime version
			if ts.Cpp.CRuntimeVersion != nil {
				params["c_runtime_version"] = []string{ts.Cpp.CRuntimeVersion.String()}
			}

			// Architecture
			params["architecture"] = []string{ts.Cpp.Architecture.String()}

			// C++ stdlib (only if not unspecified)
			if ts.Cpp.CppStdlib != v1.CppStdlib_CPP_STDLIB_UNSPECIFIED {
				params["cpp_stdlib"] = []string{ts.Cpp.CppStdlib.String()}
			}

			// Compiler and version (if available)
			if ts.Cpp.Compiler != v1.CppCompiler_CPP_COMPILER_UNSPECIFIED {
				params["compiler"] = []string{ts.Cpp.Compiler.String()}
			}
			if ts.Cpp.CompilerVersion != nil {
				params["compiler_version"] = []string{ts.Cpp.CompilerVersion.String()}
			}
		}
	}

	return params
}

// parametersMatch checks if runtime parameters match the query parameters.
// All query parameters must match at least one runtime parameter value.
func (m *RemoteRuntimesManager) parametersMatch(runtimeParams map[string][]string, queryParams map[string]*v1.StringArray) bool {
	for paramName, queryValues := range queryParams {
		// Empty query values means "match any"
		if len(queryValues.Values) == 0 {
			continue
		}

		runtimeValues, exists := runtimeParams[paramName]
		if !exists && len(queryValues.Values) > 0 {
			// Parameter doesn't exist in runtime but query requires specific values
			return false
		}

		// Check if any runtime value matches any query value
		matched := false
		for _, qv := range queryValues.Values {
			for _, rv := range runtimeValues {
				if qv == rv {
					matched = true
					break
				}
			}
			if matched {
				break
			}
		}

		if !matched && len(queryValues.Values) > 0 {
			return false
		}
	}

	return true
}

// errorRemoteRuntimeExecution returns a descriptive error for remote runtime execution attempts.
func errorRemoteRuntimeExecution(method string) error {
	return errors.New("RemoteRuntimesManager." + method + " - remote runtimes cannot be executed locally; use ListRuntimes or Match for discovery only")
}
