package daemon

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"connectrpc.com/connect"
	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	"github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1/protov1connect"
	"github.com/Manu343726/buildozer/pkg/logging"
	"github.com/Manu343726/buildozer/pkg/peers"
	"github.com/Manu343726/buildozer/pkg/runtimes"
)

// DiscoveryServiceHandler implements the DiscoveryService for the daemon
type DiscoveryServiceHandler struct {
	*logging.Logger
	protov1connect.UnimplementedDiscoveryServiceHandler
	daemonID       string // Local daemon ID
	peerManager    *peers.Manager
	runtimeManager runtimes.Manager
	discoverer     *peers.MulticastDiscoverer // For triggering discovery cycles
}

// NewDiscoveryServiceHandler creates a new discovery service handler
func NewDiscoveryServiceHandler(daemonID string, peerManager *peers.Manager, runtimeManager runtimes.Manager, discoverer *peers.MulticastDiscoverer) *DiscoveryServiceHandler {
	return &DiscoveryServiceHandler{
		Logger:         Log(daemonID).Child("DiscoveryServiceHandler"),
		daemonID:       daemonID,
		peerManager:    peerManager,
		runtimeManager: runtimeManager,
		discoverer:     discoverer,
	}
}

// ListPeers returns all known peers discovered via mDNS, including the local daemon
func (h *DiscoveryServiceHandler) ListPeers(ctx context.Context, req *connect.Request[v1.ListPeersRequest]) (*connect.Response[v1.ListPeersResponse], error) {
	requesterID := getRequesterID(req.Msg.RequesterInfo)
	h.Info("ListPeers handler called", "requester", requesterID)
	pbPeers := make([]*v1.Peer, 0)

	// Get current local runtimes (refresh to get latest runtime info)
	localRuntimes, _, err := h.runtimeManager.ListRuntimes(ctx)
	if err != nil {
		h.Debug("failed to get local runtimes for peer list", "requester", requesterID, "error", err)
		localRuntimes = nil
	}

	// Get all peers from peer manager (including local and remote)
	allPeers := h.peerManager.GetAllPeersIncludingLocal()
	for _, peer := range allPeers {
		// For the local peer, use the freshly-retrieved runtimes to ensure current state
		runtimesToUse := peer.Runtimes
		if peer.IsLocal && len(localRuntimes) > 0 {
			runtimesToUse = localRuntimes
			h.Debug("adding local peer to list", "requester", requesterID, "peer_id", peer.ID, "is_local", true, "runtime_count", len(localRuntimes))
		} else if peer.IsLocal {
			h.Debug("adding local peer to list", "requester", requesterID, "peer_id", peer.ID, "is_local", true, "runtime_count", len(runtimesToUse))
		} else {
			h.Debug("adding remote peer to list", "requester", requesterID, "peer_id", peer.ID, "is_local", false, "runtime_count", len(runtimesToUse))
		}

		// Parse endpoint to extract host and port
		var host string
		var port uint32
		parts := parseEndpoint(peer.Endpoint)
		if len(parts) >= 2 {
			host = parts[0]
			if p, err := parsePort(parts[1]); err == nil {
				port = uint32(p)
			}
		}

		isOnline := peer.IsAlive
		isLocal := peer.IsLocal
		pbPeer := &v1.Peer{
			PeerId:            peer.ID,
			ApiUri:            &v1.ApiUri{Host: host, Port: port, Protocol: v1.ApiProtocol_API_PROTOCOL_GRPC},
			IsOnline:          &isOnline,
			IsLocal:           &isLocal,
			AvailableRuntimes: runtimesToUse,
			LastSeen: &v1.TimeStamp{
				UnixMillis: peer.LastSeenAt.UnixMilli(),
			},
		}
		pbPeers = append(pbPeers, pbPeer)
	}

	h.Info("returning peer list", "requester", requesterID, "total_peers", len(pbPeers))
	return connect.NewResponse(&v1.ListPeersResponse{
		Peers: pbPeers,
		Count: int32(len(pbPeers)),
	}), nil
}

// TriggerDiscovery triggers an immediate discovery cycle
func (h *DiscoveryServiceHandler) TriggerDiscovery(ctx context.Context, req *connect.Request[v1.TriggerDiscoveryRequest]) (*connect.Response[v1.TriggerDiscoveryResponse], error) {
	requesterID := getRequesterID(req.Msg.RequesterInfo)
	h.Debug("TriggerDiscovery called", "requester", requesterID)

	if !h.discoverer.IsRunning() {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("discovery service is not running"))
	}

	// In a real implementation, this would trigger an immediate discovery cycle
	// For now, we just acknowledge the request
	h.Info("discovery cycle triggered", "requester", requesterID)

	return connect.NewResponse(&v1.TriggerDiscoveryResponse{
		Message: "Discovery cycle triggered",
		Status:  v1.TriggerDiscoveryResponse_SUCCESS,
	}), nil
}

// RegisterDiscoveryService registers the discovery service handler with the daemon
func RegisterDiscoveryService(daemonID string, d *Daemon) (string, http.Handler) {
	handler := NewDiscoveryServiceHandler(daemonID, d.peerManager, d.runtimes, d.discoverer)
	mux := http.NewServeMux()

	path, discoveryHandler := protov1connect.NewDiscoveryServiceHandler(handler)
	mux.Handle(path, discoveryHandler)

	return path, mux
}

// parseEndpoint parses a "host:port" endpoint string
func parseEndpoint(endpoint string) []string {
	return strings.Split(endpoint, ":")
}

// parsePort parses a port string to uint32
func parsePort(portStr string) (uint32, error) {
	var port uint32
	if p, err := strconv.ParseUint(portStr, 10, 32); err != nil {
		return 0, err
	} else {
		port = uint32(p)
	}
	return port, nil
}
