package daemon

import (
	"context"
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
func NewJobServiceHandler(manager *JobManager) *JobServiceHandler {
	return &JobServiceHandler{
		Logger:  Log().Child("JobServiceHandler"),
		manager: manager,
	}
}

// RegisterService creates and registers the job service handler
func RegisterJobService(manager *JobManager) (string, http.Handler) {
	handler := NewJobServiceHandler(manager)
	return protov1connect.NewJobServiceHandler(handler)
}

// SubmitJob implements JobService.SubmitJob
// Accepts a job submission and queues it for execution
func (jsh *JobServiceHandler) SubmitJob(ctx context.Context, req *connect.Request[v1.SubmitJobRequest]) (*connect.Response[v1.SubmitJobResponse], error) {
	job := req.Msg.Job

	jsh.Info("Received job submission", "jobID", job.Id)

	// Validate job
	if job.Id == "" {
		return connect.NewResponse(&v1.SubmitJobResponse{
			Accepted:     false,
			ErrorMessage: "job ID cannot be empty",
		}), nil
	}

	if job.Runtime == nil {
		return connect.NewResponse(&v1.SubmitJobResponse{
			Accepted:     false,
			ErrorMessage: "job runtime is required",
		}), nil
	}

	// Check job spec
	if job.JobSpec == nil {
		return connect.NewResponse(&v1.SubmitJobResponse{
			Accepted:     false,
			ErrorMessage: "job spec is required (cpp_compile or cpp_link)",
		}), nil
	}

	// Submit job to manager
	err := jsh.manager.SubmitJob(ctx, job)
	if err != nil {
		jsh.Error("Failed to submit job", "jobID", job.Id, "error", err)
		return connect.NewResponse(&v1.SubmitJobResponse{
			Accepted:     false,
			ErrorMessage: err.Error(),
		}), nil
	}

	// Get initial job status
	jobStatus, err := jsh.manager.GetJobStatus(job.Id)
	if err != nil {
		return connect.NewResponse(&v1.SubmitJobResponse{
			Accepted:     false,
			ErrorMessage: "failed to get job status: " + err.Error(),
		}), nil
	}

	jsh.Info("Job submitted successfully", "jobID", job.Id)

	return connect.NewResponse(&v1.SubmitJobResponse{
		JobStatus:    jobStatus,
		Accepted:     true,
		ErrorMessage: "",
	}), nil
}

// GetJobStatus implements JobService.GetJobStatus
// Returns current status of a submitted job
func (jsh *JobServiceHandler) GetJobStatus(ctx context.Context, req *connect.Request[v1.GetJobStatusRequest]) (*connect.Response[v1.GetJobStatusResponse], error) {
	jobID := req.Msg.JobId

	jsh.Debug("Getting job status", "jobID", jobID)

	jobStatus, err := jsh.manager.GetJobStatus(jobID)
	if err != nil {
		jsh.Warn("Job not found", "jobID", jobID)
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

	jsh.Debug("Watching job status", "jobID", jobID)

	// Subscribe to job progress updates
	progressCh, err := jsh.manager.WatchJobStatus(jobID)
	if err != nil {
		jsh.Warn("Job not found for watch", "jobID", jobID)
		return connect.NewError(connect.CodeNotFound, err)
	}

	// Stream updates until context is cancelled
	for {
		select {
		case <-ctx.Done():
			jsh.Debug("Watch cancelled", "jobID", jobID)
			return ctx.Err()

		case progress := <-progressCh:
			if progress == nil {
				// Channel closed, job done
				return nil
			}

			// Get full job status
			jobStatus, err := jsh.manager.GetJobStatus(jobID)
			if err != nil {
				jsh.Error("Failed to get job status during watch", "jobID", jobID, "error", err)
				return connect.NewError(connect.CodeInternal, err)
			}

			// Send status update to client
			err = stream.Send(&v1.WatchJobStatusResponse{
				JobStatus: jobStatus,
				UpdatedAt: progress.UpdatedAt,
			})
			if err != nil {
				jsh.Debug("Failed to send watch update", "jobID", jobID, "error", err)
				return err
			}
		}
	}
}

// CancelJob implements JobService.CancelJob
// Cancels a submitted job
func (jsh *JobServiceHandler) CancelJob(ctx context.Context, req *connect.Request[v1.CancelJobRequest]) (*connect.Response[v1.CancelJobResponse], error) {
	jobID := req.Msg.JobId

	jsh.Info("Cancelling job", "jobID", jobID, "reason", req.Msg.Reason)

	err := jsh.manager.CancelJob(jobID, req.Msg.Reason)
	if err != nil {
		jsh.Error("Failed to cancel job", "jobID", jobID, "error", err)
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
