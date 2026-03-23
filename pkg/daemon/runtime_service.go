package daemon

import (
	"context"
	"net/http"

	"connectrpc.com/connect"
	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	"github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1/protov1connect"
	"github.com/Manu343726/buildozer/pkg/logging"
)

// runtimeServiceHandler implements the RuntimeService gRPC interface
type runtimeServiceHandler struct {
	*logging.Logger

	manager *RuntimeManager
}

// newRuntimeServiceHandler creates a new runtime service handler
func newRuntimeServiceHandler(manager *RuntimeManager) *runtimeServiceHandler {
	return &runtimeServiceHandler{
		Logger:  Log().Child("RuntimeServiceHandler"),
		manager: manager,
	}
}

// RegisterRuntimeService creates and registers the runtime service handler
// Returns the path and HTTP handler for use with an HTTP mux
func RegisterRuntimeService(manager *RuntimeManager) (string, http.Handler) {
	handler := newRuntimeServiceHandler(manager)
	return protov1connect.NewRuntimeServiceHandler(handler)
}

// ListRuntimes implements RuntimeService.ListRuntimes
// Returns runtimes available on the daemon, optionally including peer runtimes
// If local_only is true, only returns runtimes from this daemon
// If local_only is false, also queries peer daemons in the network
func (rsh *runtimeServiceHandler) ListRuntimes(ctx context.Context, req *connect.Request[v1.ListRuntimesRequest]) (*connect.Response[v1.ListRuntimesResponse], error) {
	filter := ""
	if req.Msg.ToolchainFilter != nil {
		filter = *req.Msg.ToolchainFilter
	}

	runtimes, notes, err := rsh.manager.ListRuntimes(ctx, filter)
	if err != nil {
		rsh.Error("Runtime detection failed", "error", err)
		// Return error but still include any runtimes that were detected before the error
		return connect.NewResponse(&v1.ListRuntimesResponse{
			Runtimes:       runtimes,
			DetectionNotes: notes + " (error during detection: " + err.Error() + ")",
		}), nil
	}

	// TODO: If local_only is false, query peer daemons and merge results
	if !req.Msg.LocalOnly {
		// Peer discovery placeholder - will be implemented when peer coordination is ready
		rsh.Debug("Peer runtime discovery not yet implemented", "local_only", req.Msg.LocalOnly)
	}

	return connect.NewResponse(&v1.ListRuntimesResponse{
		Runtimes:       runtimes,
		DetectionNotes: notes,
	}), nil
}

// GetRuntime implements RuntimeService.GetRuntime
// Returns details about a specific runtime by ID
func (rsh *runtimeServiceHandler) GetRuntime(ctx context.Context, req *connect.Request[v1.GetRuntimeRequest]) (*connect.Response[v1.GetRuntimeResponse], error) {
	runtimeID := req.Msg.RuntimeId

	// First, ensure we have detected all runtimes
	_, _, err := rsh.manager.ListRuntimes(ctx, "")
	if err != nil {
		rsh.Error("Runtime detection failed", "error", err)
	}

	rt, err := rsh.manager.GetRuntime(ctx, runtimeID)
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

	return connect.NewResponse(&v1.GetRuntimeResponse{
		Runtime: rt,
	}), nil
}
