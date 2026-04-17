package peers

import (
	"time"

	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
)

// PeerInfo holds information about a discovered peer daemon in the network
type PeerInfo struct {
	// Unique identifier for the peer (e.g., hostname:port, UUID, etc.)
	ID string

	// Host address of the peer's gRPC server
	Host string

	// Port the peer's gRPC server is listening on
	Port int

	// Full runtime objects available on this peer (includes complete toolchain info)
	Runtimes []*v1.Runtime

	// Whether this peer is currently online/alive
	IsAlive bool

	// When this peer was first discovered
	DiscoveredAt time.Time

	// When this peer was last seen/heard from
	LastSeenAt time.Time

	// gRPC endpoint for connecting to this peer (e.g., hostname:port)
	Endpoint string

	// IsLocal indicates if this is the local daemon itself
	IsLocal bool
}

// Copy returns a deep copy of the PeerInfo
func (p *PeerInfo) Copy() *PeerInfo {
	if p == nil {
		return nil
	}
	runtimes := make([]*v1.Runtime, len(p.Runtimes))
	copy(runtimes, p.Runtimes)
	return &PeerInfo{
		ID:           p.ID,
		Host:         p.Host,
		Port:         p.Port,
		Runtimes:     runtimes,
		IsAlive:      p.IsAlive,
		DiscoveredAt: p.DiscoveredAt,
		LastSeenAt:   p.LastSeenAt,
		Endpoint:     p.Endpoint,
		IsLocal:      p.IsLocal,
	}
}
