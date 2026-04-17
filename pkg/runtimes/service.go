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

	manager Manager
}

// NewServiceHandler creates a new runtime service handler.
// The manager should handle runtime aggregation (local + remote) if needed.
func NewServiceHandler(manager Manager) *ServiceHandler {
	return &ServiceHandler{
		Logger:  Log().Child("ServiceHandler"),
		manager: manager,
	}
}

// RegisterService creates and registers the runtime service handler.
// The manager should handle both local and remote runtimes via the Manager interface.
// For including peer runtimes, pass an AggregatedRuntimesManager.
// For local-only runtimes, pass a LocalRuntimesManager.
func RegisterService(manager Manager) (string, http.Handler) {
	handler := NewServiceHandler(manager)
	return protov1connect.NewRuntimeServiceHandler(handler)
}

// ListRuntimes implements RuntimeService.ListRuntimes.
// Returns runtimes available via the manager.
// If the manager is an AggregatedRuntimesManager with local_only=false,
// it will include runtimes from remote peers.
// If local_only=true, only returns runtimes from this daemon.
func (sh *ServiceHandler) ListRuntimes(ctx context.Context, req *connect.Request[v1.ListRuntimesRequest]) (*connect.Response[v1.ListRuntimesResponse], error) {
	// If local_only flag is set, use only local runtime manager
	var mgr Manager
	if req.Msg.LocalOnly {
		// For local-only queries, we need the local manager
		// If the current manager is aggregated, extract the local one
		if agg, ok := sh.manager.(*AggregatedRuntimesManager); ok {
			mgr = agg.localManager
		} else {
			mgr = sh.manager
		}
	} else {
		// Use the full manager (which may be aggregated)
		mgr = sh.manager
	}

	runtimes, notes, err := mgr.ListRuntimes(ctx)
	if err != nil {
		sh.Error("Runtime discovery failed", "error", err, "local_only", req.Msg.LocalOnly)
		// Return error but still include any runtimes that were discovered before the error
		return connect.NewResponse(&v1.ListRuntimesResponse{
			Runtimes:       runtimes,
			DetectionNotes: notes + " (error during discovery: " + err.Error() + ")",
		}), nil
	}

	sh.Debug("Runtime discovery complete", "runtime_count", len(runtimes), "local_only", req.Msg.LocalOnly)

	return connect.NewResponse(&v1.ListRuntimesResponse{
		Runtimes:       runtimes,
		DetectionNotes: notes,
	}), nil
}

// GetRuntime implements RuntimeService.GetRuntime.
// Returns details about a specific runtime by ID.
func (sh *ServiceHandler) GetRuntime(ctx context.Context, req *connect.Request[v1.GetRuntimeRequest]) (*connect.Response[v1.GetRuntimeResponse], error) {
	runtimeID := req.Msg.RuntimeId

	// Get the actual runtime implementation (must be local/executable)
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

	// Get the local manager to access runtimeToProto conversion
	var localMgr *LocalRuntimesManager

	if agg, ok := sh.manager.(*AggregatedRuntimesManager); ok {
		// Extract local manager from aggregated manager and assert it's LocalRuntimesManager
		if local, ok := agg.localManager.(*LocalRuntimesManager); ok {
			localMgr = local
		}
	} else if local, ok := sh.manager.(*LocalRuntimesManager); ok {
		localMgr = local
	}

	if localMgr == nil {
		// Manager doesn't support proto conversion
		errMsg := "runtime manager does not support runtime details conversion"
		return connect.NewResponse(&v1.GetRuntimeResponse{
			Error: &errMsg,
		}), nil
	}

	// Convert to proto for response
	proto, err := rt.Proto(ctx)
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

// Match implements RuntimeService.Match.
// Returns runtimes matching the given query.
// If local_only=false, includes runtimes from remote peers.
// If local_only=true, only returns local runtimes.
func (sh *ServiceHandler) Match(ctx context.Context, req *connect.Request[v1.MatchRuntimesRequest]) (*connect.Response[v1.MatchRuntimesResponse], error) {
	if req.Msg.Query == nil {
		return connect.NewResponse(&v1.MatchRuntimesResponse{
			Runtimes: nil,
			Notes:    "invalid request: query cannot be nil",
			Error:    ptrString("query is required"),
		}), nil
	}

	// If local_only flag is set, use only local runtime manager
	var mgr Manager
	if req.Msg.LocalOnly {
		// For local-only queries, we need the local manager
		// If the current manager is aggregated, extract the local one
		if agg, ok := sh.manager.(*AggregatedRuntimesManager); ok {
			mgr = agg.localManager
		} else {
			mgr = sh.manager
		}
	} else {
		// Use the full manager (which may be aggregated)
		mgr = sh.manager
	}

	matches, err := mgr.Match(ctx, req.Msg.Query)
	if err != nil {
		sh.Error("Runtime matching failed", "error", err, "local_only", req.Msg.LocalOnly)
		return connect.NewResponse(&v1.MatchRuntimesResponse{
			Runtimes: matches,
			Notes:    "matching completed with errors",
			Error:    ptrString(err.Error()),
		}), nil
	}

	sh.Debug("Runtime matching complete", "match_count", len(matches), "local_only", req.Msg.LocalOnly)

	return connect.NewResponse(&v1.MatchRuntimesResponse{
		Runtimes: matches,
		Notes:    "matching completed successfully",
	}), nil
}

// ptrString returns a pointer to a string value
func ptrString(s string) *string {
	return &s
}
