package daemon

import (
	"context"
	"net/http"

	"connectrpc.com/connect"
	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	"github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1/protov1connect"
	"github.com/Manu343726/buildozer/pkg/logging"
)

// IntrospectionServiceHandler implements the IntrospectionService for the daemon
type IntrospectionServiceHandler struct {
	*logging.Logger // Embedded logger for hierarchical logging

	protov1connect.UnimplementedIntrospectionServiceHandler

	daemon *Daemon
}

// NewIntrospectionServiceHandler creates a new introspection service handler
func NewIntrospectionServiceHandler(daemonID string, daemon *Daemon) *IntrospectionServiceHandler {
	return &IntrospectionServiceHandler{
		Logger: logging.Log(daemonID).Child("IntrospectionServiceHandler"),
		daemon: daemon,
	}
}

// GetJobQueue returns the current job queue state and statistics
func (h *IntrospectionServiceHandler) GetJobQueue(ctx context.Context, req *connect.Request[v1.GetJobQueueRequest]) (*connect.Response[v1.GetJobQueueResponse], error) {
	requesterID := getRequesterID(req.Msg.RequesterInfo)
	h.Debug("Getting job queue", "requester", requesterID)

	if h.daemon == nil {
		h.Error("daemon is nil", "requester", requesterID)
		return nil, connect.NewError(connect.CodeInternal, nil)
	}

	// Build queued jobs response
	queuedJobs := make([]*v1.QueuedJobInfo, 0)

	// TODO: Get queue size from scheduler if needed
	// For now, queue is managed internally by scheduler
	// TODO: Implement detailed queued job info once scheduler exposes queue details
	resp := &v1.GetJobQueueResponse{
		QueuedJobs:       queuedJobs,
		RunningJobsCount: 0, // TODO: Track running jobs
		QueueSize:        0, // Queue now managed by scheduler internally
	}

	h.Info("returning job queue info", "requester", requesterID)
	return connect.NewResponse(resp), nil
}

// RegisterIntrospectionService registers the introspection service with the daemon
func RegisterIntrospectionService(daemonID string, d *Daemon) (string, http.Handler) {
	handler := NewIntrospectionServiceHandler(daemonID, d)
	return protov1connect.NewIntrospectionServiceHandler(handler)
}

// GetQueueDetails returns detailed information about queued jobs
// This is a helper function for IntrospectionService to access scheduler internals
func (h *IntrospectionServiceHandler) getQueueDetails() []*v1.QueuedJobInfo {
	// TODO: Implement once scheduler exposes queue iteration
	// For now, return empty list
	return make([]*v1.QueuedJobInfo, 0)
}
