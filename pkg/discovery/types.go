package discovery

import (
	"net"
	"time"
)

// PeerInfo holds information about a discovered peer daemon
type PeerInfo struct {
	// Unique identifier for the peer (based on hostname + port or UUID)
	ID string

	// Host and port of the peer's gRPC server
	Host string
	Port int

	// Runtimes available on this peer
	AvailableRuntimes []string

	// Status of the peer
	IsAlive bool

	// When this peer was first discovered
	DiscoveredAt time.Time

	// When this peer was last seen
	LastSeenAt time.Time

	// gRPC endpoint
	Endpoint string
}

// PeerRegistry tracks all known peers in the network
type PeerRegistry struct {
	// Map of peer ID to peer info
	peers map[string]*PeerInfo

	// Callback when peer is discovered
	onPeerDiscovered func(*PeerInfo)

	// Callback when peer goes offline
	onPeerOffline func(*PeerInfo)
}

// NewPeerRegistry creates a new peer registry
func NewPeerRegistry() *PeerRegistry {
	return &PeerRegistry{
		peers: make(map[string]*PeerInfo),
	}
}

// OnPeerDiscovered sets the callback for peer discovery
func (pr *PeerRegistry) OnPeerDiscovered(fn func(*PeerInfo)) {
	pr.onPeerDiscovered = fn
}

// OnPeerOffline sets the callback for peer going offline
func (pr *PeerRegistry) OnPeerOffline(fn func(*PeerInfo)) {
	pr.onPeerOffline = fn
}

// RegisterPeer adds or updates a peer in the registry
func (pr *PeerRegistry) RegisterPeer(peer *PeerInfo) {
	if existing, ok := pr.peers[peer.ID]; ok {
		// Peer already exists, update it
		existing.Host = peer.Host
		existing.Port = peer.Port
		existing.AvailableRuntimes = peer.AvailableRuntimes
		existing.IsAlive = true
		existing.LastSeenAt = time.Now()
		existing.Endpoint = peer.Endpoint
	} else {
		// New peer
		peer.DiscoveredAt = time.Now()
		peer.LastSeenAt = time.Now()
		peer.IsAlive = true
		pr.peers[peer.ID] = peer

		if pr.onPeerDiscovered != nil {
			pr.onPeerDiscovered(peer)
		}
	}
}

// GetPeer retrieves a peer by ID
func (pr *PeerRegistry) GetPeer(id string) *PeerInfo {
	return pr.peers[id]
}

// GetAllPeers returns all known peers
func (pr *PeerRegistry) GetAllPeers() []*PeerInfo {
	result := make([]*PeerInfo, 0, len(pr.peers))
	for _, peer := range pr.peers {
		result = append(result, peer)
	}
	return result
}

// RemovePeer removes a peer from the registry
func (pr *PeerRegistry) RemovePeer(id string) {
	if peer, ok := pr.peers[id]; ok {
		delete(pr.peers, id)
		if pr.onPeerOffline != nil {
			pr.onPeerOffline(peer)
		}
	}
}

// MarkPeerOffline marks a peer as offline but keeps it in registry
func (pr *PeerRegistry) MarkPeerOffline(id string) {
	if peer, ok := pr.peers[id]; ok {
		peer.IsAlive = false
		if pr.onPeerOffline != nil {
			pr.onPeerOffline(peer)
		}
	}
}

// PeerCount returns the number of known peers
func (pr *PeerRegistry) PeerCount() int {
	return len(pr.peers)
}

// GetLocalPeerInfo creates a PeerInfo for the local daemon
func GetLocalPeerInfo(host string, port int, runtimes []string) *PeerInfo {
	endpoint := net.JoinHostPort(host, "")
	return &PeerInfo{
		Host:              host,
		Port:              port,
		AvailableRuntimes: runtimes,
		IsAlive:           true,
		DiscoveredAt:      time.Now(),
		LastSeenAt:        time.Now(),
		Endpoint:          endpoint,
	}
}
