package runtimes

import (
	"context"
	"net/http"

	"connectrpc.com/connect"
	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	"github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1/protov1connect"
	"github.com/Manu343726/buildozer/pkg/logging"
)

// ServiceHandler implements the RuntimeService gRPC interface.
// It translates gRPC requests to manager calls and converts responses back to proto format.
type ServiceHandler struct {
	*logging.Logger // Embed Logger for logging service operations

	manager *Manager
}

// NewServiceHandler creates a new runtime service handler.
func NewServiceHandler(manager *Manager) *ServiceHandler {
	return &ServiceHandler{
		Logger:  Log().Child("ServiceHandler"),
		manager: manager,
	}
}

// RegisterService creates and registers the runtime service handler.
// Returns the path and HTTP handler for use with an HTTP mux.
func RegisterService(manager *Manager) (string, http.Handler) {
	handler := NewServiceHandler(manager)
	return protov1connect.NewRuntimeServiceHandler(handler)
}

// ListRuntimes implements RuntimeService.ListRuntimes.
// Returns runtimes available on the daemon, optionally including peer runtimes.
// If local_only is true, only returns runtimes from this daemon.
// If local_only is false, also queries peer daemons in the network.
func (sh *ServiceHandler) ListRuntimes(ctx context.Context, req *connect.Request[v1.ListRuntimesRequest]) (*connect.Response[v1.ListRuntimesResponse], error) {
	runtimes, notes, err := sh.manager.ListRuntimes(ctx)
	if err != nil {
		sh.Error("Runtime detection failed", "error", err)
		// Return error but still include any runtimes that were detected before the error
		return connect.NewResponse(&v1.ListRuntimesResponse{
			Runtimes:       runtimes,
			DetectionNotes: notes + " (error during detection: " + err.Error() + ")",
		}), nil
	}

	// TODO: If local_only is false, query peer daemons and merge results
	if !req.Msg.LocalOnly {
		// Peer discovery placeholder - will be implemented when peer coordination is ready
		sh.Debug("Peer runtime discovery not yet implemented", "local_only", req.Msg.LocalOnly)
	}

	return connect.NewResponse(&v1.ListRuntimesResponse{
		Runtimes:       runtimes,
		DetectionNotes: notes,
	}), nil
}

// GetRuntime implements RuntimeService.GetRuntime.
// Returns details about a specific runtime by ID.
func (sh *ServiceHandler) GetRuntime(ctx context.Context, req *connect.Request[v1.GetRuntimeRequest]) (*connect.Response[v1.GetRuntimeResponse], error) {
	runtimeID := req.Msg.RuntimeId

	// Get the actual runtime implementation
	rt, err := sh.manager.GetRuntimeByID(ctx, runtimeID)
	if err != nil {
		errMsg := err.Error()
		return connect.NewResponse(&v1.GetRuntimeResponse{
			Error: &errMsg,
		}), nil
	}

	if rt == nil {
		var msg string
		msg = "runtime not found: " + runtimeID
		return connect.NewResponse(&v1.GetRuntimeResponse{
			Error: &msg,
		}), nil
	}

	// Convert to proto for response
	proto, err := sh.manager.runtimeToProto(ctx, rt)
	if err != nil {
		errMsg := err.Error()
		return connect.NewResponse(&v1.GetRuntimeResponse{
			Error: &errMsg,
		}), nil
	}

	return connect.NewResponse(&v1.GetRuntimeResponse{
		Runtime: proto,
	}), nil
}
