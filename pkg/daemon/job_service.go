package daemon

import (
	"context"
	"fmt"
	"net/http"

	"connectrpc.com/connect"
	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	"github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1/protov1connect"
	"github.com/Manu343726/buildozer/pkg/logging"
)

// JobServiceHandler implements the JobService gRPC interface
type JobServiceHandler struct {
	*logging.Logger
	manager *JobManager
}

// NewJobServiceHandler creates a new job service handler
func NewJobServiceHandler(daemonID string, manager *JobManager) *JobServiceHandler {
	return &JobServiceHandler{
		Logger:  Log(daemonID).Child("JobServiceHandler"),
		manager: manager,
	}
}

// RegisterService creates and registers the job service handler
func RegisterJobService(daemonID string, manager *JobManager) (string, http.Handler) {
	handler := NewJobServiceHandler(daemonID, manager)
	return protov1connect.NewJobServiceHandler(handler)
}

// SubmitJob implements JobService.SubmitJob
// Accepts a job submission and streams status updates
// The stream is established before scheduling to prevent race conditions
func (jsh *JobServiceHandler) SubmitJob(ctx context.Context, req *connect.Request[v1.SubmitJobRequest], stream *connect.ServerStream[v1.SubmitJobResponse]) error {
	job := req.Msg.Job
	requesterID := getRequesterID(req.Msg.RequesterInfo)

	jsh.Info("Received job submission", "jobID", job.Id, "requester", requesterID)

	// Validate job
	if job.Id == "" {
		err := stream.Send(&v1.SubmitJobResponse{
			Response: &v1.SubmitJobResponse_Confirmation{
				Confirmation: &v1.SubmissionConfirmation{
					Accepted:     false,
					ErrorMessage: "job ID cannot be empty",
				},
			},
		})
		return err
	}

	// Check that runtime requirement is set (either explicit runtime or runtime match query)
	if job.RuntimeRequirement == nil {
		err := stream.Send(&v1.SubmitJobResponse{
			Response: &v1.SubmitJobResponse_Confirmation{
				Confirmation: &v1.SubmissionConfirmation{
					Accepted:     false,
					ErrorMessage: "job runtime is required (either runtime or runtime_match_query must be set)",
				},
			},
		})
		return err
	}

	// Check job spec
	if job.JobSpec == nil {
		err := stream.Send(&v1.SubmitJobResponse{
			Response: &v1.SubmitJobResponse_Confirmation{
				Confirmation: &v1.SubmissionConfirmation{
					Accepted:     false,
					ErrorMessage: "job spec is required (cpp_compile or cpp_link)",
				},
			},
		})
		return err
	}

	// Create initial job state and send confirmation IMMEDIATELY to establish connection
	// This ensures the client is listening before we submit to the scheduler
	initialJobStatus := &v1.JobStatus{
		JobId: job.Id,
		Progress: &v1.JobProgress{
			JobId:  job.Id,
			Status: v1.JobProgress_JOB_STATUS_RECEIVED, // Initial state: received
		},
	}

	confirmMsg := &v1.SubmitJobResponse{
		Response: &v1.SubmitJobResponse_Confirmation{
			Confirmation: &v1.SubmissionConfirmation{
				JobStatus:    initialJobStatus,
				Accepted:     true,
				ErrorMessage: "",
			},
		},
	}
	if err := stream.Send(confirmMsg); err != nil {
		jsh.Error("Failed to send confirmation", "jobID", job.Id, "error", err)
		return err
	}

	jsh.Info("Job submission confirmed, streaming status updates", "jobID", job.Id, "requester", requesterID)

	// Submit job to manager AFTER confirmation is sent to ensure client is listening
	// SubmitJob returns the watch handle, so we don't need to call WatchJobStatus separately
	handle, err := jsh.manager.SubmitJob(ctx, job)
	if err != nil {
		jsh.Error("Failed to submit job to scheduler", "jobID", job.Id, "requester", requesterID, "error", err)
		return err
	}

	jsh.Info("Job submitted to scheduler", "jobID", job.Id, "requester", requesterID)

	// Stream updates until context is cancelled or job reaches terminal state
	for {
		select {
		case <-ctx.Done():
			// Client cancelled the stream, unsubscribe and return
			jsh.manager.StopWatching(handle)
			jsh.Info("Stream cancelled by client", "jobID", job.Id, "requester", requesterID)
			return ctx.Err()

		case progress, ok := <-handle.Channel:
			if !ok {
				// Channel closed by manager after job completion - job is done
				jsh.Info("Stream ended, watching channel closed", "jobID", job.Id, "requester", requesterID)
				return nil
			}

			// Get full job status
			jobStatus, err := jsh.manager.GetJobStatus(job.Id)
			if err != nil {
				jsh.Error("Failed to get job status during watch", "jobID", job.Id, "error", err)
				jsh.manager.StopWatching(handle)
				return fmt.Errorf("failed to get job status: %w", err)
			}

			// Send status update
			updateMsg := &v1.SubmitJobResponse{
				Response: &v1.SubmitJobResponse_StatusUpdate{
					StatusUpdate: &v1.StatusUpdate{
						JobStatus: jobStatus,
						UpdatedAt: progress.UpdatedAt,
					},
				},
			}
			if err := stream.Send(updateMsg); err != nil {
				jsh.Debug("Failed to send update", "jobID", job.Id, "error", err)
				jsh.manager.StopWatching(handle)
				return err
			}
		}
	}
}

// GetJobStatus implements JobService.GetJobStatus
// Returns current status of a submitted job
func (jsh *JobServiceHandler) GetJobStatus(ctx context.Context, req *connect.Request[v1.GetJobStatusRequest]) (*connect.Response[v1.GetJobStatusResponse], error) {
	jobID := req.Msg.JobId
	requesterID := getRequesterID(req.Msg.RequesterInfo)

	jsh.Debug("Getting job status", "jobID", jobID, "requester", requesterID)

	jobStatus, err := jsh.manager.GetJobStatus(jobID)
	if err != nil {
		jsh.Warn("Job not found", "jobID", jobID, "requester", requesterID)
		return connect.NewResponse(&v1.GetJobStatusResponse{
			ErrorMessage: err.Error(),
		}), nil
	}

	return connect.NewResponse(&v1.GetJobStatusResponse{
		JobStatus: jobStatus,
	}), nil
}

// WatchJobStatus implements JobService.WatchJobStatus
// Streams job status updates in real-time
func (jsh *JobServiceHandler) WatchJobStatus(ctx context.Context, req *connect.Request[v1.WatchJobStatusRequest], stream *connect.ServerStream[v1.WatchJobStatusResponse]) error {
	jobID := req.Msg.JobId
	requesterID := getRequesterID(req.Msg.RequesterInfo)

	jsh.Debug("Watching job status", "jobID", jobID, "requester", requesterID)

	// Subscribe to job progress updates
	handle, err := jsh.manager.WatchJobStatus(jobID)
	if err != nil {
		jsh.Warn("Job not found for watch", "jobID", jobID, "requester", requesterID)
		return connect.NewError(connect.CodeNotFound, err)
	}

	// Stream updates until context is cancelled or job reaches terminal state
	for {
		select {
		case <-ctx.Done():
			// Client cancelled the stream, unsubscribe and return
			jsh.manager.StopWatching(handle)
			jsh.Info("Stream cancelled by client", "jobID", jobID, "requester", requesterID)
			return ctx.Err()

		case progress, ok := <-handle.Channel:
			if !ok {
				// Channel closed by manager after job completion - job is done
				jsh.Info("Stream ended, watching channel closed", "jobID", jobID, "requester", requesterID)
				return nil
			}

			// Get full job status
			jobStatus, err := jsh.manager.GetJobStatus(jobID)
			if err != nil {
				jsh.Error("Failed to get job status during watch", "jobID", jobID, "requester", requesterID, "error", err)
				return connect.NewError(connect.CodeInternal, err)
			}

			// Send status update to client
			err = stream.Send(&v1.WatchJobStatusResponse{
				JobStatus: jobStatus,
				UpdatedAt: progress.UpdatedAt,
			})
			if err != nil {
				jsh.Debug("Failed to send watch update", "jobID", jobID, "requester", requesterID, "error", err)
				return err
			}
		}
	}
}

// CancelJob implements JobService.CancelJob
// Cancels a submitted job
func (jsh *JobServiceHandler) CancelJob(ctx context.Context, req *connect.Request[v1.CancelJobRequest]) (*connect.Response[v1.CancelJobResponse], error) {
	jobID := req.Msg.JobId
	requesterID := getRequesterID(req.Msg.RequesterInfo)

	jsh.Info("Cancelling job", "jobID", jobID, "requester", requesterID, "reason", req.Msg.Reason)

	err := jsh.manager.CancelJob(jobID, req.Msg.Reason)
	if err != nil {
		jsh.Error("Failed to cancel job", "jobID", jobID, "requester", requesterID, "error", err)
		return connect.NewResponse(&v1.CancelJobResponse{
			Success:      false,
			ErrorMessage: err.Error(),
		}), nil
	}

	return connect.NewResponse(&v1.CancelJobResponse{
		Success:       true,
		JobsCancelled: 1,
	}), nil
}
