package peers

import (
	"sync"
	"time"
)

// Manager tracks all known peers in the network, including the local daemon
type Manager struct {
	mu sync.RWMutex

	// Map of peer ID to peer info
	peers map[string]*PeerInfo

	// Local daemon peer info
	localPeer *PeerInfo

	// Callback when peer is discovered
	onPeerDiscovered func(*PeerInfo)

	// Callback when peer goes offline
	onPeerOffline func(*PeerInfo)
}

// NewManager creates a new peer manager
func NewManager() *Manager {
	return &Manager{
		peers: make(map[string]*PeerInfo),
	}
}

// SetLocalPeer sets the local daemon's peer info
func (pm *Manager) SetLocalPeer(peer *PeerInfo) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if peer != nil {
		peer.IsLocal = true
		peer.IsAlive = true
		peer.DiscoveredAt = time.Now()
		peer.LastSeenAt = time.Now()
	}
	pm.localPeer = peer
}

// GetLocalPeer returns the local daemon's peer info
func (pm *Manager) GetLocalPeer() *PeerInfo {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	if pm.localPeer == nil {
		return nil
	}
	return pm.localPeer.Copy()
}

// OnPeerDiscovered sets the callback for peer discovery
func (pm *Manager) OnPeerDiscovered(fn func(*PeerInfo)) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.onPeerDiscovered = fn
}

// OnPeerOffline sets the callback for peer going offline
func (pm *Manager) OnPeerOffline(fn func(*PeerInfo)) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.onPeerOffline = fn
}

// RegisterPeer adds or updates a peer in the manager
// Returns true if the peer was newly discovered, false if it was updated
func (pm *Manager) RegisterPeer(peer *PeerInfo) bool {
	if peer == nil {
		return false
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()

	if existing, ok := pm.peers[peer.ID]; ok {
		// Peer already exists, update it
		existing.Host = peer.Host
		existing.Port = peer.Port
		existing.Runtimes = peer.Runtimes
		existing.IsAlive = true
		existing.LastSeenAt = time.Now()
		existing.Endpoint = peer.Endpoint
		existing.IsLocal = peer.IsLocal
		return false
	}

	// New peer
	peer.DiscoveredAt = time.Now()
	peer.LastSeenAt = time.Now()
	peer.IsAlive = true
	pm.peers[peer.ID] = peer

	if pm.onPeerDiscovered != nil {
		pm.onPeerDiscovered(peer)
	}

	return true
}

// GetPeer retrieves a peer by ID
func (pm *Manager) GetPeer(id string) *PeerInfo {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if peer, ok := pm.peers[id]; ok {
		return peer.Copy()
	}
	return nil
}

// GetAllPeers returns all known peers (excluding local peer, which should be accessed via GetLocalPeer)
func (pm *Manager) GetAllPeers() []*PeerInfo {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	result := make([]*PeerInfo, 0, len(pm.peers))
	for _, peer := range pm.peers {
		result = append(result, peer.Copy())
	}
	return result
}

// GetAllPeersIncludingLocal returns all known peers including the local peer
func (pm *Manager) GetAllPeersIncludingLocal() []*PeerInfo {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	result := make([]*PeerInfo, 0)

	// Add local peer first if it exists
	if pm.localPeer != nil {
		result = append(result, pm.localPeer.Copy())
	}

	// Add remote peers
	for _, peer := range pm.peers {
		result = append(result, peer.Copy())
	}
	return result
}

// RemovePeer removes a peer from the manager
func (pm *Manager) RemovePeer(id string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if peer, ok := pm.peers[id]; ok {
		delete(pm.peers, id)
		if pm.onPeerOffline != nil {
			pm.onPeerOffline(peer)
		}
	}
}

// MarkPeerOffline marks a peer as offline but keeps it in the manager
func (pm *Manager) MarkPeerOffline(id string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if peer, ok := pm.peers[id]; ok {
		peer.IsAlive = false
		if pm.onPeerOffline != nil {
			pm.onPeerOffline(peer)
		}
	}
}

// PeerCount returns the number of known peers (excluding local peer)
func (pm *Manager) PeerCount() int {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return len(pm.peers)
}

// TotalPeerCount returns the number of known peers including the local peer
func (pm *Manager) TotalPeerCount() int {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	count := len(pm.peers)
	if pm.localPeer != nil {
		count++
	}
	return count
}

// HasPeer checks if a peer with the given ID exists
func (pm *Manager) HasPeer(id string) bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	_, exists := pm.peers[id]
	return exists
}

// Clear removes all peers from the manager (except local peer)
func (pm *Manager) Clear() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.peers = make(map[string]*PeerInfo)
}
