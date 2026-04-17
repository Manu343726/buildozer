package peers

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	protov1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	"github.com/Manu343726/buildozer/pkg/logging"
)

// RuntimeDiscoveryProvider provides the interface for discovering available runtimes.
// This interface is used by MulticastDiscoverer to avoid circular dependencies with the runtimes package.
type RuntimeDiscoveryProvider interface {
	// ListRuntimes returns all available runtimes as proto messages for announcement.
	ListRuntimes(ctx context.Context) ([]*protov1.Runtime, string, error)
}

// Multicast discovery configuration
const (
	MulticastGroup     = "239.0.0.1"
	MulticastPort      = 5354
	AnnouncementWindow = 90 * time.Second // Flood prevention window
)

// MulticastDiscoverer handles peer discovery via UDP multicast announcements
type MulticastDiscoverer struct {
	*logging.Logger

	// Configuration
	daemonID string
	rpcURI   string
	interval time.Duration

	// Dependencies (injected)
	peerManager     *Manager
	runtimeProvider RuntimeDiscoveryProvider

	// UDP multicast
	listenerConn *net.UDPConn // Connection for receiving announcements
	sendConn     *net.UDPConn // Connection for sending announcements
	addr         *net.UDPAddr
	doneCh       chan struct{}

	// State management
	mu                      sync.RWMutex
	running                 bool
	cancel                  context.CancelFunc
	ctx                     context.Context
	startTime               time.Time            // When discoverer started (for uptime calculation)
	lastAnnouncementTime    map[string]time.Time // Track last announcement per daemon (for flood prevention)
	lastNetworkActivityTime time.Time            // Track when last announcement from any peer was seen
	sequence                uint64
}

// NewMulticastDiscoverer creates a new UDP multicast-based discoverer
func NewMulticastDiscoverer(daemonID string, rpcURI string, intervalSecs int, peerManager *Manager, runtimeProvider RuntimeDiscoveryProvider) *MulticastDiscoverer {
	return &MulticastDiscoverer{
		Logger:               logging.Log(daemonID).Child("MulticastDiscoverer"),
		daemonID:             daemonID,
		rpcURI:               rpcURI,
		interval:             time.Duration(intervalSecs) * time.Second,
		peerManager:          peerManager,
		runtimeProvider:      runtimeProvider,
		doneCh:               make(chan struct{}),
		lastAnnouncementTime: make(map[string]time.Time),
	}
}

// Start begins UDP multicast discovery and announcement
func (md *MulticastDiscoverer) Start(ctx context.Context) error {
	md.mu.Lock()
	defer md.mu.Unlock()

	if md.running {
		return md.Errorf("discoverer already running")
	}

	md.ctx, md.cancel = context.WithCancel(ctx)
	md.running = true

	// Create multicast address
	var err error
	md.addr, err = net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", MulticastGroup, MulticastPort))
	if err != nil {
		return fmt.Errorf("failed to resolve multicast address: %w", err)
	}

	// Create multicast listener connection
	md.listenerConn, err = net.ListenMulticastUDP("udp", nil, md.addr)
	if err != nil {
		return fmt.Errorf("failed to listen on multicast address: %w", err)
	}
	// Set larger buffer for receiving announcements
	md.listenerConn.SetReadBuffer(65535)

	// Create sender connection - bind to local address to send to multicast group
	senderAddr, err := net.ResolveUDPAddr("udp", ":0") // Bind to any local address, OS picks port
	if err != nil {
		md.listenerConn.Close()
		return fmt.Errorf("failed to resolve sender address: %w", err)
	}
	md.sendConn, err = net.DialUDP("udp", senderAddr, md.addr)
	if err != nil {
		md.listenerConn.Close()
		return fmt.Errorf("failed to create sender connection: %w", err)
	}

	// Initialize start time for uptime calculations
	md.startTime = time.Now()

	// Start announcement goroutine
	go md.announcementLoop()

	// Start listener goroutine
	go md.listenerLoop()

	md.Info("UDP multicast discoverer started",
		"daemon_id", md.daemonID,
		"rpc_uri", md.rpcURI,
		"multicast_addr", md.addr.String(),
		"announcement_interval_secs", int(md.interval.Seconds()))
	return nil
}

// Stop stops the discoverer
func (md *MulticastDiscoverer) Stop() error {
	md.mu.Lock()
	defer md.mu.Unlock()

	if !md.running {
		return nil
	}

	md.running = false
	if md.cancel != nil {
		md.cancel()
	}
	if md.listenerConn != nil {
		md.listenerConn.Close()
	}
	if md.sendConn != nil {
		md.sendConn.Close()
	}

	md.Info("UDP multicast discoverer stopped")
	return nil
}

// IsRunning returns whether the discoverer is currently running
func (md *MulticastDiscoverer) IsRunning() bool {
	md.mu.RLock()
	defer md.mu.RUnlock()
	return md.running
}

// PeerManager returns the peer manager
func (md *MulticastDiscoverer) PeerManager() *Manager {
	return md.peerManager
}

// announcementLoop sends announcements when the network goes quiet
// Only announces if no announcements from any peer have been seen in the configured interval
func (md *MulticastDiscoverer) announcementLoop() {
	md.Info("announcement loop started", "quiet_interval_seconds", md.interval.Seconds())

	// Send announcement immediately when daemon comes online
	md.mu.Lock()
	md.lastNetworkActivityTime = time.Now()
	md.mu.Unlock()
	md.sendAnnouncement()

	// Use a shorter ticker to check network activity frequently (every second)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-md.ctx.Done():
			md.Debug("announcement loop stopped")
			return
		case <-ticker.C:
			md.mu.RLock()
			timeSinceLastActivity := time.Since(md.lastNetworkActivityTime)
			md.mu.RUnlock()

			// Send announcement only if network has been quiet for the configured interval
			if timeSinceLastActivity >= md.interval {
				md.sendAnnouncement()
				// Update activity time so we don't spam announcements
				md.mu.Lock()
				md.lastNetworkActivityTime = time.Now()
				md.mu.Unlock()
			}
		}
	}
}

// sendAnnouncement sends a daemon announcement to the multicast group
func (md *MulticastDiscoverer) sendAnnouncement() {
	// Parse RPC URI to extract host and port
	rpcHost, rpcPort := parseRPCURI(md.rpcURI)

	// Get available runtimes from runtime provider
	supportedRuntimes, _, err := md.runtimeProvider.ListRuntimes(md.ctx)
	if err != nil {
		md.Debug("failed to get runtimes for announcement", "error", err)
		// Don't fail announcement - continue with empty runtimes list
		supportedRuntimes = nil
	}

	// Create peer information for this announcement
	isOnline := true
	isLocal := true
	peer := &protov1.Peer{
		PeerId:            md.daemonID,
		ApiUri:            &protov1.ApiUri{Host: rpcHost, Port: rpcPort, Protocol: protov1.ApiProtocol_API_PROTOCOL_GRPC},
		AvailableRuntimes: supportedRuntimes,
		IsOnline:          &isOnline,
		IsLocal:           &isLocal,
	}

	// Collect known peers and peers to ignore (recently announced)
	md.mu.RLock()
	knownPeerInfos := make([]*protov1.Peer, 0)
	ignorePeerIDs := make([]string, 0)
	now := time.Now()
	// Include peers in ignore list if we heard from them recently (within half the AnnouncementWindow)
	recentThreshold := AnnouncementWindow / 2
	for peerID, lastSeen := range md.lastAnnouncementTime {
		if now.Sub(lastSeen) < recentThreshold {
			ignorePeerIDs = append(ignorePeerIDs, peerID)
		}
	}
	md.mu.RUnlock()

	// Get all known peers and convert to proto
	knownPeers := md.peerManager.GetAllPeers()
	for _, peerInfo := range knownPeers {
		isOnline := true
		isLocal := false
		protoPeer := &protov1.Peer{
			PeerId:            peerInfo.ID,
			ApiUri:            &protov1.ApiUri{Host: peerInfo.Host, Port: uint32(peerInfo.Port), Protocol: protov1.ApiProtocol_API_PROTOCOL_GRPC},
			AvailableRuntimes: peerInfo.Runtimes,
			IsOnline:          &isOnline,
			IsLocal:           &isLocal,
		}
		knownPeerInfos = append(knownPeerInfos, protoPeer)
	}

	// Create announcement event as REQUEST with our peer ID as initiator
	event := &protov1.AnnouncementEvent{
		Timestamp:       &protov1.TimeStamp{UnixMillis: time.Now().UnixMilli()},
		Peer:            peer,
		Type:            protov1.AnnouncementType_ANNOUNCEMENT_TYPE_REQUEST,
		InitiatorPeerId: md.daemonID,
		IgnorePeerIds:   ignorePeerIDs,
		KnownPeers:      knownPeerInfos,
	}

	// Encode to protobuf
	data, err := proto.Marshal(event)
	if err != nil {
		md.Debug("failed to marshal announcement event", "error", err)
		return
	}

	// Send to multicast group via dedicated sender connection
	_, err = md.sendConn.Write(data)
	if err != nil {
		md.Debug("failed to send announcement", "error", err)
		return
	}

	md.Debug("announcement sent", "peer_id", md.daemonID, "type", "REQUEST")
}

// listenerLoop receives announcements from other daemons on the multicast group
func (md *MulticastDiscoverer) listenerLoop() {
	md.Info("listener loop started")
	buffer := make([]byte, 65535)

	for {
		select {
		case <-md.ctx.Done():
			md.Debug("listener loop stopped")
			return
		default:
		}

		// Set read deadline to allow context checking
		md.listenerConn.SetReadDeadline(time.Now().Add(5 * time.Second))

		n, remoteAddr, err := md.listenerConn.ReadFromUDP(buffer)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				// Read timeout - expected, try again
				continue
			}
			// Log other errors
			md.Warn("UDP read failed", "error", err.Error())
			continue
		}

		// Parse announcement event
		event := &protov1.AnnouncementEvent{}
		err = proto.Unmarshal(buffer[:n], event)
		if err != nil {
			md.Debug("failed to unmarshal announcement from peer",
				"error", err,
				"packet_size", n,
				"from_addr", remoteAddr.String())
			continue
		}

		// Skip own announcements
		if event.Peer != nil && event.Peer.PeerId == md.daemonID {
			continue
		}

		// Log announcement reception
		peerID := ""
		var runtimeCount int64
		if event.Peer != nil {
			peerID = event.Peer.PeerId
			runtimeCount = int64(len(event.Peer.AvailableRuntimes))
		}

		md.Info("announcement received from peer",
			"from_peer_id", peerID,
			"from_addr", remoteAddr.String(),
			"runtimes_count", runtimeCount)

		// Handle the announcement
		md.handleAnnouncement(event, remoteAddr.String())
	}
}

// handleAnnouncement processes an announcement from another daemon
// Implements the gossip protocol:
// - Resets the network activity timer (so we don't announce while network is active)
// - If we haven't heard from this daemon recently, send back our announcement (flood prevention)
func (md *MulticastDiscoverer) handleAnnouncement(event *protov1.AnnouncementEvent, fromAddr string) {
	if event.Peer == nil {
		md.Debug("skipping announcement: missing peer information", "from_addr", fromAddr)
		return
	}

	// Reset network activity timer - we just heard from a peer
	md.mu.Lock()
	md.lastNetworkActivityTime = time.Now()

	// Check per-peer flood prevention window (separate from network quiet time)
	lastSeen, exists := md.lastAnnouncementTime[event.Peer.PeerId]
	now := time.Now()

	// Check if we should send a response (outside announcement window)
	shouldRespond := !exists || now.Sub(lastSeen) > AnnouncementWindow

	// Update last seen time for this peer
	md.lastAnnouncementTime[event.Peer.PeerId] = now
	md.mu.Unlock()

	// Register the peer
	md.registerPeerFromAnnouncement(event)

	// Process known peers from the announcement (gossip protocol)
	// Register peers we learn about transitively
	if len(event.KnownPeers) > 0 {
		md.Debug("processing known peers from announcement",
			"peer_id", event.Peer.PeerId,
			"known_peers_count", len(event.KnownPeers))
		for _, knownPeer := range event.KnownPeers {
			// Skip if it's our own peer ID
			if knownPeer.PeerId == md.daemonID {
				continue
			}

			// Convert proto Peer to PeerInfo and register
			alreadyKnown := md.peerManager.HasPeer(knownPeer.PeerId)

			if knownPeer.ApiUri != nil {
				endpoint := fmt.Sprintf("%s:%d", knownPeer.ApiUri.Host, knownPeer.ApiUri.Port)
				peerInfo := &PeerInfo{
					ID:       knownPeer.PeerId,
					Host:     knownPeer.ApiUri.Host,
					Port:     int(knownPeer.ApiUri.Port),
					Endpoint: endpoint,
					Runtimes: knownPeer.AvailableRuntimes,
					IsAlive:  true,
					IsLocal:  false,
				}

				md.peerManager.RegisterPeer(peerInfo)

				if !alreadyKnown {
					md.Debug("discovered new peer via gossip",
						"new_peer_id", knownPeer.PeerId,
						"via_peer_id", event.Peer.PeerId)
				}
			}
		}
	}

	// Check if we should respond to REQUEST announcements
	// Skip responding if our peer_id is in the ignore list
	isIgnored := false
	for _, ignorePeerID := range event.IgnorePeerIds {
		if ignorePeerID == md.daemonID {
			isIgnored = true
			break
		}
	}

	// Only respond to REQUEST type announcements, not RESPONSE type
	if event.Type == protov1.AnnouncementType_ANNOUNCEMENT_TYPE_REQUEST && shouldRespond && !isIgnored {
		md.Info("peer announcement has triggered response",
			"peer_id", event.Peer.PeerId,
			"initiator_peer_id", event.InitiatorPeerId,
			"from_addr", fromAddr,
			"reason", func() string {
				if !exists {
					return "new_peer"
				}
				return fmt.Sprintf("outside_window (%.1fs since last)", now.Sub(lastSeen).Seconds())
			}())
		md.sendAnnouncementResponses(event)
	} else if event.Type == protov1.AnnouncementType_ANNOUNCEMENT_TYPE_REQUEST && isIgnored {
		md.Debug("announcement response suppressed (peer in ignore list)",
			"peer_id", event.Peer.PeerId,
			"from_addr", fromAddr)
	} else if event.Type == protov1.AnnouncementType_ANNOUNCEMENT_TYPE_RESPONSE {
		md.Debug("received announcement response, no further response needed",
			"peer_id", event.Peer.PeerId,
			"initiator_peer_id", event.InitiatorPeerId)
	} else {
		md.Debug("announcement response suppressed (flood prevention)",
			"peer_id", event.Peer.PeerId,
			"from_addr", fromAddr,
			"time_since_last", fmt.Sprintf("%.1fs", now.Sub(lastSeen).Seconds()),
			"window_seconds", AnnouncementWindow.Seconds())
	}
}

// sendAnnouncementResponses sends a single RESPONSE announcement with all known peer information
// This implements efficient peer discovery where one response contains the responder's info
// plus all known peers in the network
func (md *MulticastDiscoverer) sendAnnouncementResponses(requestEvent *protov1.AnnouncementEvent) {
	if requestEvent == nil || requestEvent.Peer == nil {
		md.Debug("cannot send announcement response: missing request event or peer")
		return
	}

	requestingPeerID := requestEvent.Peer.PeerId
	ignoreSet := make(map[string]bool)
	for _, peerID := range requestEvent.IgnorePeerIds {
		ignoreSet[peerID] = true
	}

	// Skip response if requesting peer is in ignore list
	if ignoreSet[requestingPeerID] {
		md.Debug("announcement response suppressed (peer in ignore list)",
			"requesting_peer_id", requestingPeerID)
		return
	}

	// Parse our RPC URI to create our peer info
	rpcHost, rpcPort := parseRPCURI(md.rpcURI)

	// Get our available runtimes
	supportedRuntimes, _, err := md.runtimeProvider.ListRuntimes(md.ctx)
	if err != nil {
		md.Debug("failed to get runtimes for response announcement", "error", err)
		supportedRuntimes = nil
	}

	isOnline := true
	isLocal := true
	responderPeer := &protov1.Peer{
		PeerId:            md.daemonID,
		ApiUri:            &protov1.ApiUri{Host: rpcHost, Port: rpcPort, Protocol: protov1.ApiProtocol_API_PROTOCOL_GRPC},
		AvailableRuntimes: supportedRuntimes,
		IsOnline:          &isOnline,
		IsLocal:           &isLocal,
	}

	// Get all known peers and convert to proto
	knownPeers := md.peerManager.GetAllPeers()
	knownPeerInfos := make([]*protov1.Peer, 0)

	for _, peerInfo := range knownPeers {
		// Skip if it's the requesting peer or in the ignore list
		if peerInfo.ID == requestingPeerID || ignoreSet[peerInfo.ID] {
			continue
		}

		isOnline := true
		isLocal := false
		protoPeer := &protov1.Peer{
			PeerId:            peerInfo.ID,
			ApiUri:            &protov1.ApiUri{Host: peerInfo.Host, Port: uint32(peerInfo.Port), Protocol: protov1.ApiProtocol_API_PROTOCOL_GRPC},
			AvailableRuntimes: peerInfo.Runtimes,
			IsOnline:          &isOnline,
			IsLocal:           &isLocal,
		}
		knownPeerInfos = append(knownPeerInfos, protoPeer)
	}

	// Create single response announcement with responder as Peer and all known peers in known_peers
	event := &protov1.AnnouncementEvent{
		Timestamp:       &protov1.TimeStamp{UnixMillis: time.Now().UnixMilli()},
		Peer:            responderPeer,
		Type:            protov1.AnnouncementType_ANNOUNCEMENT_TYPE_RESPONSE,
		InitiatorPeerId: requestEvent.InitiatorPeerId,
		KnownPeers:      knownPeerInfos,
	}

	// Encode to protobuf
	data, err := proto.Marshal(event)
	if err != nil {
		md.Debug("failed to marshal response announcement event", "error", err)
		return
	}

	// Send to multicast group via dedicated sender connection
	_, err = md.sendConn.Write(data)
	if err != nil {
		md.Debug("failed to send response announcement", "error", err)
		return
	}

	md.Info("announcement response sent",
		"responder_peer_id", md.daemonID,
		"type", "RESPONSE",
		"initiator_peer_id", requestEvent.InitiatorPeerId,
		"known_peers_count", len(knownPeerInfos))
}

// registerPeerFromAnnouncement converts an announcement event to a peer registration
func (md *MulticastDiscoverer) registerPeerFromAnnouncement(event *protov1.AnnouncementEvent) {
	if event.Peer == nil || event.Peer.ApiUri == nil {
		md.Debug("skipping peer registration: missing peer or API URI in announcement")
		return
	}

	peer := event.Peer

	// Reconstruct endpoint from host and port
	endpoint := fmt.Sprintf("%s:%d", peer.ApiUri.Host, peer.ApiUri.Port)

	// Store full Runtime proto objects from the announcement
	// Add this peer's ID to each runtime's peer_ids list
	runtimes := make([]*protov1.Runtime, 0, len(peer.AvailableRuntimes))
	if peer.AvailableRuntimes != nil {
		for _, rt := range peer.AvailableRuntimes {
			if rt != nil {
				// Add this peer to the runtime's peer_ids list
				rt.PeerIds = append(rt.PeerIds, peer.PeerId)
				runtimes = append(runtimes, rt)
			}
		}
	}

	peerInfo := &PeerInfo{
		ID:       peer.PeerId,
		Host:     peer.ApiUri.Host,
		Port:     int(peer.ApiUri.Port),
		Endpoint: endpoint,
		Runtimes: runtimes,
		IsAlive:  true,
		IsLocal:  false,
	}

	md.peerManager.RegisterPeer(peerInfo)

	md.Info("peer registered in peer manager from announcement",
		"peer_id", peer.PeerId,
		"endpoint", endpoint,
		"runtime_count", len(runtimes))
}

// parseRPCURI parses a "host:port" URI and returns the components
func parseRPCURI(uri string) (string, uint32) {
	// Try to parse as host:port
	parts := strings.Split(uri, ":")
	if len(parts) == 2 {
		host := parts[0]
		port := uint32(0)
		if p, err := strconv.ParseUint(parts[1], 10, 32); err == nil {
			port = uint32(p)
		}
		return host, port
	}
	return uri, 0
}

// GetDiscoveredPeers returns all discovered peers from the peer manager
func (md *MulticastDiscoverer) GetDiscoveredPeers() []*PeerInfo {
	return md.peerManager.GetAllPeers()
}
