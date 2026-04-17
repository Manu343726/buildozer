package remote

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"connectrpc.com/connect"
	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	protov1connect "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1/protov1connect"
	"github.com/Manu343726/buildozer/internal/utils"
	"github.com/Manu343726/buildozer/pkg/logging"
	"github.com/Manu343726/buildozer/pkg/peers"
	"github.com/Manu343726/buildozer/pkg/runtime"
)

// Manager wraps remote daemon runtimes as Runtime interface implementations.
// It orchestrates RPC calls to peer RuntimeServices in parallel using fork-join pattern.
type Manager struct {
	*logging.Logger

	peerManager *peers.Manager
}

// NewManager creates a new remote runtimes manager.
func NewManager(peerManager *peers.Manager) *Manager {
	return &Manager{
		Logger:      Log().Child("Manager"),
		peerManager: peerManager,
	}
}

// ListRuntimes queries all remote peers for their runtimes in parallel.
// Merges results from all peers.
func (m *Manager) ListRuntimes(ctx context.Context) ([]*v1.Runtime, string, error) {
	if m.peerManager == nil {
		return nil, "No peer manager configured", nil
	}

	allPeers := m.peerManager.GetAllPeersIncludingLocal()
	remotePeers := filterRemotePeers(allPeers)

	if len(remotePeers) == 0 {
		return nil, "No remote peers available", nil
	}

	// Use Map to query all peers in parallel
	type peerResult struct {
		peerId   string
		runtimes []*v1.Runtime
	}

	results, mapErr := utils.Map(remotePeers, func(peer *peers.PeerInfo) (peerResult, error) {
		// Create RuntimeService client for this peer
		client := protov1connect.NewRuntimeServiceClient(http.DefaultClient, "http://"+peer.Endpoint)

		// Call ListRuntimes via RPC
		resp, err := client.ListRuntimes(ctx, connect.NewRequest(&v1.ListRuntimesRequest{}))
		if err != nil {
			return peerResult{peerId: peer.ID}, err
		}

		return peerResult{
			peerId:   peer.ID,
			runtimes: resp.Msg.Runtimes,
		}, nil
	})

	// Aggregate results - keep duplicates from different peers for scheduler comparison
	var allRuntimes []*v1.Runtime
	successCount := 0

	for _, result := range results {
		if len(result.runtimes) > 0 {
			successCount++
			allRuntimes = append(allRuntimes, result.runtimes...)
		}
	}

	notes := fmt.Sprintf("Retrieved %d runtimes from %d/%d remote peers", len(allRuntimes), successCount, len(remotePeers))
	m.Debug("ListRuntimes completed", "total_runtimes", len(allRuntimes), "peers", successCount)

	if successCount == 0 && mapErr != nil {
		return nil, notes, mapErr
	}

	return allRuntimes, notes, nil
}

// GetRuntimes queries all remote peers for their runtimes and wraps them as Runtime implementations.
// Each returned runtime delegates Execute calls to its remote peer via RPC.
func (m *Manager) GetRuntimes(ctx context.Context) ([]runtime.Runtime, string, error) {
	if m.peerManager == nil {
		return nil, "", errors.New("peer manager not available")
	}

	allPeers := m.peerManager.GetAllPeersIncludingLocal()
	remotePeers := filterRemotePeers(allPeers)

	if len(remotePeers) == 0 {
		return nil, "No remote peers available", nil
	}

	// Use Map to query all peers in parallel
	type peerResult struct {
		peerId   string
		runtimes []*v1.Runtime
	}

	results, mapErr := utils.Map(remotePeers, func(peer *peers.PeerInfo) (peerResult, error) {
		// Create RuntimeService client for this peer
		client := protov1connect.NewRuntimeServiceClient(http.DefaultClient, "http://"+peer.Endpoint)

		// Call ListRuntimes via RPC (GetRuntimes uses ListRuntimes)
		resp, err := client.ListRuntimes(ctx, connect.NewRequest(&v1.ListRuntimesRequest{}))
		if err != nil {
			return peerResult{peerId: peer.ID}, err
		}

		return peerResult{
			peerId:   peer.ID,
			runtimes: resp.Msg.Runtimes,
		}, nil
	})

	// Aggregate results - keep duplicates from different peers for scheduler comparison
	var allRuntimes []runtime.Runtime
	successCount := 0

	for _, result := range results {
		if len(result.runtimes) > 0 {
			successCount++
			for _, rt := range result.runtimes {
				wrapped := NewRemoteRuntime(rt, result.peerId, m.peerManager)
				allRuntimes = append(allRuntimes, wrapped)
			}
		}
	}

	notes := fmt.Sprintf("Created runtime implementations from %d/%d remote peers", len(allRuntimes), successCount)
	m.Debug("GetRuntimes completed", "total_runtimes", len(allRuntimes), "peers", successCount)

	if successCount == 0 && mapErr != nil {
		return nil, notes, mapErr
	}

	return allRuntimes, notes, nil
}

// GetRuntimeByID searches all remote peers for a runtime with the given ID in parallel.
// Returns the runtime wrapped as a RemoteRuntime for transparent RPC execution.
func (m *Manager) GetRuntimeByID(ctx context.Context, runtimeID string) (runtime.Runtime, error) {
	if m.peerManager == nil {
		return nil, errors.New("peer manager not available")
	}

	allPeers := m.peerManager.GetAllPeersIncludingLocal()
	remotePeers := filterRemotePeers(allPeers)

	// Use Map to search all peers in parallel
	type searchResult struct {
		peerId  string
		runtime *v1.Runtime
		found   bool
	}

	results, _ := utils.Map(remotePeers, func(peer *peers.PeerInfo) (searchResult, error) {
		// Create RuntimeService client for this peer
		client := protov1connect.NewRuntimeServiceClient(http.DefaultClient, "http://"+peer.Endpoint)

		// Call GetRuntime via RPC
		resp, err := client.GetRuntime(ctx, connect.NewRequest(&v1.GetRuntimeRequest{RuntimeId: runtimeID}))
		if err != nil {
			// Not found is not an error, just means this peer doesn't have it
			return searchResult{peerId: peer.ID, found: false}, nil
		}

		return searchResult{
			peerId:  peer.ID,
			runtime: resp.Msg.Runtime,
			found:   true,
		}, nil
	})

	// Return first match found
	for _, result := range results {
		if result.found {
			wrapped := NewRemoteRuntime(result.runtime, result.peerId, m.peerManager)
			m.Debug("Found remote runtime", "runtimeID", runtimeID, "peerId", result.peerId)
			return wrapped, nil
		}
	}

	m.Debug("Remote runtime not found", "runtimeID", runtimeID)
	return nil, fmt.Errorf("remote runtime not found: %s", runtimeID)
}

// Match queries all remote peers for runtimes matching the given query in parallel.
// Merges matching runtimes from all peers.
func (m *Manager) Match(ctx context.Context, query *v1.RuntimeMatchQuery) ([]*v1.Runtime, error) {
	if query == nil {
		return nil, fmt.Errorf("match query cannot be nil")
	}

	if m.peerManager == nil {
		return nil, errors.New("peer manager not available")
	}

	allPeers := m.peerManager.GetAllPeersIncludingLocal()
	remotePeers := filterRemotePeers(allPeers)

	if len(remotePeers) == 0 {
		return nil, nil
	}

	// Use Map to query all peers in parallel
	type peerResult struct {
		runtimes []*v1.Runtime
	}

	results, mapErr := utils.Map(remotePeers, func(peer *peers.PeerInfo) (peerResult, error) {
		// Create RuntimeService client for this peer
		client := protov1connect.NewRuntimeServiceClient(http.DefaultClient, "http://"+peer.Endpoint)

		// Call Match via RPC
		resp, err := client.Match(ctx, connect.NewRequest(&v1.MatchRuntimesRequest{Query: query}))
		if err != nil {
			return peerResult{}, err
		}

		return peerResult{
			runtimes: resp.Msg.Runtimes,
		}, nil
	})

	// Aggregate results - keep duplicates from different peers for scheduler comparison
	var matches []*v1.Runtime
	successCount := 0

	for _, result := range results {
		if len(result.runtimes) > 0 {
			successCount++
			matches = append(matches, result.runtimes...)
		}
	}

	m.Debug("Match completed", "peers_queried", successCount, "matches_found", len(matches))

	if successCount == 0 && mapErr != nil {
		return nil, mapErr
	}

	return matches, nil
}

// filterRemotePeers returns only non-local peers from the peer list.
func filterRemotePeers(peerList []*peers.PeerInfo) []*peers.PeerInfo {
	var remote []*peers.PeerInfo
	for _, p := range peerList {
		if !p.IsLocal {
			remote = append(remote, p)
		}
	}
	return remote
}
