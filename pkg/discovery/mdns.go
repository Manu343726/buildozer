package discovery

import (
	"context"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/Manu343726/buildozer/pkg/logging"
)

// mDNS multicast address and port
const (
	mDNSGroup = "224.0.0.251"
	mDNSPort  = 5353
)

// DiscoveryMessage is an mDNS discovery request or response
type DiscoveryMessage struct {
	Type      string   `json:"type"` // "request" or "response"
	DaemonID  string   `json:"daemon_id"`
	Host      string   `json:"host"`
	Port      int      `json:"port"`
	Endpoint  string   `json:"endpoint"`
	Runtimes  []string `json:"runtimes,omitempty"`
	Timestamp int64    `json:"timestamp"`
}

// MDNSDiscoverer handles mDNS-based peer discovery
type MDNSDiscoverer struct {
	*logging.Logger

	// Configuration
	daemonID string
	host     string
	port     int
	interval time.Duration

	// Peer registry
	registry *PeerRegistry

	// mDNS networking
	multicastAddr *net.UDPAddr
	conn          *net.UDPConn

	// State management
	mu                sync.RWMutex
	running           bool
	cancel            context.CancelFunc
	ctx               context.Context
	lastDiscoveryTime time.Time

	// For tracking announced peers
	announcedPeers map[string]time.Time
}

// NewMDNSDiscoverer creates a new mDNS discoverer
func NewMDNSDiscoverer(daemonID string, host string, port int, intervalSecs int) *MDNSDiscoverer {
	return &MDNSDiscoverer{
		Logger:         logging.Log().Child("MDNSDiscoverer"),
		daemonID:       daemonID,
		host:           host,
		port:           port,
		interval:       time.Duration(intervalSecs) * time.Second,
		registry:       NewPeerRegistry(),
		announcedPeers: make(map[string]time.Time),
	}
}

// Registry returns the peer registry
func (md *MDNSDiscoverer) Registry() *PeerRegistry {
	return md.registry
}

// Start begins mDNS discovery and announcement
func (md *MDNSDiscoverer) Start(ctx context.Context) error {
	md.mu.Lock()
	defer md.mu.Unlock()

	if md.running {
		return md.Errorf("discoverer already running")
	}

	md.ctx, md.cancel = context.WithCancel(ctx)
	md.running = true

	// Start announcement goroutine
	go md.announceLoop()

	// Start discovery goroutine
	go md.discoveryLoop()

	md.Info("mDNS discoverer started", "daemon_id", md.daemonID, "host", md.host, "port", md.port)
	return nil
}

// Stop stops the discoverer
func (md *MDNSDiscoverer) Stop() error {
	md.mu.Lock()
	defer md.mu.Unlock()

	if !md.running {
		return nil
	}

	md.running = false
	if md.cancel != nil {
		md.cancel()
	}

	md.Info("mDNS discoverer stopped")
	return nil
}

// announceLoop periodically announces this daemon
func (md *MDNSDiscoverer) announceLoop() {
	ticker := time.NewTicker(md.interval)
	defer ticker.Stop()

	for {
		select {
		case <-md.ctx.Done():
			return
		case <-ticker.C:
			md.announceSelf()
		}
	}
}

// announceSelf announces this daemon (in real implementation, broadcast via mDNS)
func (md *MDNSDiscoverer) announceSelf() {
	md.Debug("announcing daemon presence", "daemon_id", md.daemonID, "endpoint", net.JoinHostPort(md.host, strconv.Itoa(md.port)))

	// Record announcement time
	md.mu.Lock()
	md.announcedPeers[md.daemonID] = time.Now()
	md.mu.Unlock()

	// In a real implementation, this would:
	// 1. Broadcast a UDP multicast message with daemon info
	// 2. Register with mDNS responder
	// For now, this is logged for debugging
}

// discoveryLoop periodically discovers other daemons
func (md *MDNSDiscoverer) discoveryLoop() {
	ticker := time.NewTicker(md.interval)
	defer ticker.Stop()

	for {
		select {
		case <-md.ctx.Done():
			return
		case <-ticker.C:
			md.discoverPeers()
		}
	}
}

// discoverPeers discovers other daemons (in real implementation, via mDNS multicast)
func (md *MDNSDiscoverer) discoverPeers() {
	md.Debug("discovering peers on network")

	// In a real implementation, this would:
	// 1. Send a UDP multicast discovery query
	// 2. Collect responses from other daemons
	// 3. Update peer registry with responses

	// For now, clean up stale peers (peers not seen for more than 2x interval)
	md.cleanupStalePeers()
}

// cleanupStalePeers removes peers that haven't been seen recently
func (md *MDNSDiscoverer) cleanupStalePeers() {
	threshold := time.Now().Add(-2 * md.interval)

	peers := md.registry.GetAllPeers()
	for _, peer := range peers {
		// Skip the local daemon
		if peer.ID == md.daemonID {
			continue
		}

		if peer.LastSeenAt.Before(threshold) {
			md.Debug("removing stale peer", "peer_id", peer.ID, "last_seen", peer.LastSeenAt)
			md.registry.MarkPeerOffline(peer.ID)
		}
	}
}

// AddDiscoveredPeer adds a discovered peer to the registry
func (md *MDNSDiscoverer) AddDiscoveredPeer(peer *PeerInfo) {
	md.registry.RegisterPeer(peer)
	md.Debug("peer discovered", "peer_id", peer.ID, "endpoint", peer.Endpoint)
}

// GetDiscoveredPeers returns all discovered peers
func (md *MDNSDiscoverer) GetDiscoveredPeers() []*PeerInfo {
	peers := md.registry.GetAllPeers()
	// Filter out self
	result := make([]*PeerInfo, 0)
	for _, p := range peers {
		if p.ID != md.daemonID {
			result = append(result, p)
		}
	}
	return result
}

// GetDaemonID returns this daemon's ID
func (md *MDNSDiscoverer) GetDaemonID() string {
	return md.daemonID
}

// IsRunning returns whether the discoverer is running
func (md *MDNSDiscoverer) IsRunning() bool {
	md.mu.RLock()
	defer md.mu.RUnlock()
	return md.running
}
