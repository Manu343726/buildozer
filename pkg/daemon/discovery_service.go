package daemon

import (
	"context"
	"fmt"
	"net/http"

	"connectrpc.com/connect"
	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	"github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1/protov1connect"
	"github.com/Manu343726/buildozer/pkg/discovery"
	"github.com/Manu343726/buildozer/pkg/logging"
)

// DiscoveryServiceHandler implements the DiscoveryService for the daemon
type DiscoveryServiceHandler struct {
	*logging.Logger
	discoverer *discovery.MDNSDiscoverer
}

// NewDiscoveryServiceHandler creates a new discovery service handler
func NewDiscoveryServiceHandler(discoverer *discovery.MDNSDiscoverer) *DiscoveryServiceHandler {
	return &DiscoveryServiceHandler{
		Logger:     logging.Log().Child("DiscoveryServiceHandler"),
		discoverer: discoverer,
	}
}

// ListPeers returns all known peers discovered via mDNS
func (h *DiscoveryServiceHandler) ListPeers(ctx context.Context, req *connect.Request[v1.ListPeersRequest]) (*connect.Response[v1.ListPeersResponse], error) {
	h.Debug("ListPeers called")

	peers := h.discoverer.GetDiscoveredPeers()
	pbPeers := make([]*v1.PeerInfo, 0, len(peers))

	for _, peer := range peers {
		// Convert available runtimes from strings to Runtime objects
		// TODO: In a real implementation, this should populate full Runtime objects
		// For now, we create minimal Runtime objects with just the ID
		runtimes := make([]*v1.Runtime, 0, len(peer.AvailableRuntimes))
		for _, runtimeName := range peer.AvailableRuntimes {
			runtimes = append(runtimes, &v1.Runtime{
				Id: runtimeName,
			})
		}

		pbPeer := &v1.PeerInfo{
			PeerId:            peer.ID,
			Hostname:          peer.Endpoint,
			IsOnline:          peer.IsAlive,
			AvailableRuntimes: runtimes,
			LastSeen: &v1.TimeStamp{
				UnixMillis: peer.LastSeenAt.UnixMilli(),
			},
		}
		pbPeers = append(pbPeers, pbPeer)
	}

	return connect.NewResponse(&v1.ListPeersResponse{
		Peers: pbPeers,
		Count: int32(len(pbPeers)),
	}), nil
}

// TriggerDiscovery triggers an immediate discovery cycle
func (h *DiscoveryServiceHandler) TriggerDiscovery(ctx context.Context, req *connect.Request[v1.TriggerDiscoveryRequest]) (*connect.Response[v1.TriggerDiscoveryResponse], error) {
	h.Debug("TriggerDiscovery called")

	if !h.discoverer.IsRunning() {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("discovery service is not running"))
	}

	// In a real implementation, this would trigger an immediate discovery cycle
	// For now, we just acknowledge the request
	h.Info("discovery cycle triggered")

	return connect.NewResponse(&v1.TriggerDiscoveryResponse{
		Message: "Discovery cycle triggered",
		Status:  v1.TriggerDiscoveryResponse_SUCCESS,
	}), nil
}

// RegisterDiscoveryService registers the discovery service handler with the daemon
func RegisterDiscoveryService(d *Daemon) (string, http.Handler) {
	handler := NewDiscoveryServiceHandler(d.discoverer)
	mux := http.NewServeMux()

	path, discoveryHandler := protov1connect.NewDiscoveryServiceHandler(handler)
	mux.Handle(path, discoveryHandler)

	return path, mux
}
