package daemon

import (
	"context"
	"fmt"
	"sync"
	"time"

	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	"github.com/Manu343726/buildozer/pkg/logging"
	"github.com/Manu343726/buildozer/pkg/runtime"
	"github.com/Manu343726/buildozer/pkg/runtimes"
	"github.com/Manu343726/buildozer/pkg/scheduler"
)

// JobState represents an in-flight job with status tracking
type JobState struct {
	Job       *v1.Job
	Progress  *v1.JobProgress
	Result    *v1.JobResult
	Watchers  []chan *v1.JobProgress // Channels for WatchJobStatus subscribers
	CreatedAt time.Time
	mu        sync.RWMutex
}

// UpdateProgress updates the job's progress and notifies all watchers
func (js *JobState) UpdateProgress(status v1.JobProgress_JobStatus, message string) {
	js.mu.Lock()
	defer js.mu.Unlock()

	js.Progress.Status = status
	js.Progress.UpdatedAt = &v1.TimeStamp{UnixMillis: time.Now().UnixMilli()}
	if message != "" {
		js.Progress.LogOutput += message + "\n"
	}

	// When job reaches terminal status, include the full result
	switch status {
	case v1.JobProgress_JOB_STATUS_COMPLETED, v1.JobProgress_JOB_STATUS_FAILED, v1.JobProgress_JOB_STATUS_CANCELLED:
		js.Progress.Result = js.Result
	}

	// Notify all watchers
	for _, ch := range js.Watchers {
		select {
		case ch <- js.Progress:
		default:
			// Non-blocking send; drop if watcher is slow
		}
	}
}

// GetProgress returns a copy of the current progress
func (js *JobState) GetProgress() *v1.JobProgress {
	js.mu.RLock()
	defer js.mu.RUnlock()

	// Create a shallow copy
	p := *js.Progress
	return &p
}

// AddWatcher adds a watcher channel for status updates
func (js *JobState) AddWatcher(ch chan *v1.JobProgress) {
	js.mu.Lock()
	defer js.mu.Unlock()
	js.Watchers = append(js.Watchers, ch)
}

// RemoveWatcher removes and closes a specific watcher channel
func (js *JobState) RemoveWatcher(targetCh chan *v1.JobProgress) {
	js.mu.Lock()
	defer js.mu.Unlock()

	// Find and remove the watcher
	for i, ch := range js.Watchers {
		if ch == targetCh {
			// Close the channel
			close(ch)
			// Remove from slice
			js.Watchers = append(js.Watchers[:i], js.Watchers[i+1:]...)
			return
		}
	}
}

// CloseWatchers sends final update to all watchers and closes their channels
// This is called when the job reaches terminal state to signal completion
func (js *JobState) CloseWatchers() {
	js.mu.Lock()
	defer js.mu.Unlock()

	// Send final update to all watchers with a small potential wait
	for _, ch := range js.Watchers {
		select {
		case ch <- js.Progress:
		case <-time.After(100 * time.Millisecond):
			// If can't send within 100ms, skip (watcher may be slow/dead)
		}
		// Close the channel after final update
		close(ch)
	}
	js.Watchers = nil // Clear the list
}

// JobManager manages the lifecycle of submitted jobs
// Uses the scheduler for job placement and queueing decisions
type JobManager struct {
	*logging.Logger

	mu         sync.RWMutex
	jobs       map[string]*JobState // job_id → JobState
	runtimeMgr runtimes.Manager     // Manager for accessing available runtimes
	scheduler  *scheduler.Scheduler // Scheduler for job placement decisions and queueing
	daemonID   string               // This daemon's client ID
}

// NewJobManager creates a new job manager
func NewJobManager(daemonID string, runtimeMgr runtimes.Manager) *JobManager {
	return &JobManager{
		Logger:     Log(daemonID).Child("JobManager"),
		jobs:       make(map[string]*JobState),
		runtimeMgr: runtimeMgr,
		daemonID:   daemonID,
	}
}

// SetScheduler sets the scheduler for job placement decisions and queueing
func (jm *JobManager) SetScheduler(sched *scheduler.Scheduler) {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	jm.scheduler = sched
}

// SubmitJob adds a new job to the scheduler queue and returns a watch handle
// The scheduler will handle queueing if runtimes are busy, or immediately scheduling if available
// The watch handle can be used to stream job progress updates
func (jm *JobManager) SubmitJob(ctx context.Context, job *v1.Job) (*WatchHandle, error) {
	jm.Info("Submitting job", "jobID", job.Id)

	jm.mu.Lock()
	if _, exists := jm.jobs[job.Id]; exists {
		jm.mu.Unlock()
		return nil, fmt.Errorf("job %s already submitted", job.Id)
	}

	// Create initial job state with RECEIVED status
	jobState := &JobState{
		Job: job,
		Progress: &v1.JobProgress{
			JobId:     job.Id,
			Status:    v1.JobProgress_JOB_STATUS_RECEIVED,
			UpdatedAt: &v1.TimeStamp{UnixMillis: time.Now().UnixMilli()},
		},
		Watchers:  make([]chan *v1.JobProgress, 0),
		CreatedAt: time.Now(),
	}

	jm.jobs[job.Id] = jobState
	jm.mu.Unlock()

	// Create watch handle BEFORE any status updates
	// This ensures the caller is subscribed to receive all updates including initial RECEIVED status
	handle, err := jm.WatchJobStatus(job.Id)
	if err != nil {
		return nil, fmt.Errorf("failed to create watch handle: %w", err)
	}

	// Transition job state to READY after subscription
	jobState.UpdateProgress(v1.JobProgress_JOB_STATUS_READY, "Job ready for scheduling")

	// Transition job state to QUEUED
	jobState.UpdateProgress(v1.JobProgress_JOB_STATUS_QUEUED, "Job entered queue")

	// Get the scheduler
	jm.mu.RLock()
	sched := jm.scheduler
	jm.mu.RUnlock()

	if sched == nil {
		jm.Error("Scheduler not available", "jobID", job.Id)
		jobState.UpdateProgress(v1.JobProgress_JOB_STATUS_FAILED, "Scheduler not available")
		jobState.mu.Lock()
		jobState.Result = &v1.JobResult{
			JobId:           job.Id,
			ExecutingPeerId: jm.daemonID,
			StartedAt:       &v1.TimeStamp{UnixMillis: time.Now().UnixMilli()},
			CompletedAt:     &v1.TimeStamp{UnixMillis: time.Now().UnixMilli()},
			Status:          v1.JobResult_JOB_STATUS_FAILED,
			ErrorMessage:    "Scheduler not available",
			ExitCode:        1,
		}
		jobState.mu.Unlock()
		jobState.CloseWatchers()
		return nil, fmt.Errorf("scheduler not available")
	}

	// Submit to scheduler for scheduling and execution decisions
	// Job manager provides callbacks for progress monitoring and completion finalization

	// Create a progress callback that updates watchers
	progressCallback := func(ctx context.Context, progress *runtime.Progress) error {
		jobState.mu.Lock()
		defer jobState.mu.Unlock()

		// Update log output if this is output progress
		if progress.Type == runtime.ProgressTypeOutput {
			jobState.Progress.LogOutput += fmt.Sprintf("[%s] %s", progress.Source, string(progress.Data))
		}

		// Notify watchers of progress update
		for _, ch := range jobState.Watchers {
			select {
			case ch <- jobState.Progress:
			default:
				// Non-blocking send; drop if watcher is slow
			}
		}

		return nil
	}

	// Create a completion callback for output materialization and verification
	completionCallback := func(ctx context.Context, finalJob *v1.Job, execResult *runtime.ExecutionResult) error {
		// Determine final status based on exit code
		finalStatus := v1.JobProgress_JOB_STATUS_COMPLETED
		if execResult.ExitCode != 0 {
			finalStatus = v1.JobProgress_JOB_STATUS_FAILED
		}

		// Convert ExecutionResult to JobResult for finalization
		// Map JobProgress status to JobResult status
		jobResultStatus := v1.JobResult_JOB_STATUS_COMPLETED
		if finalStatus == v1.JobProgress_JOB_STATUS_FAILED {
			jobResultStatus = v1.JobResult_JOB_STATUS_FAILED
		} else if finalStatus == v1.JobProgress_JOB_STATUS_CANCELLED {
			jobResultStatus = v1.JobResult_JOB_STATUS_CANCELLED
		}

		jobResult := &v1.JobResult{
			JobId:           job.Id,
			ExecutingPeerId: jm.daemonID,
			Status:          jobResultStatus,
			ExitCode:        int32(execResult.ExitCode),
			Outputs:         execResult.Output,
		}

		// Call scheduler's finalization to materialize and verify outputs
		finalizedResult, err := sched.FinalizeJobCompletion(ctx, finalJob, jobResult)
		if err != nil {
			jm.Error("Job completion finalization failed", "jobID", job.Id, "error", err)
			// Still update job state even if finalization failed
			jobState.mu.Lock()
			jobState.Result = finalizedResult
			jobState.mu.Unlock()
			// Update status and notify watchers of failure
			jobState.UpdateProgress(v1.JobProgress_JOB_STATUS_FAILED, fmt.Sprintf("Job completion finalization failed: %v", err))
			return err
		}

		// Update job state with finalized result
		jobState.mu.Lock()
		jobState.Result = finalizedResult
		jobState.mu.Unlock()

		// Update status and notify watchers of completion
		jobState.UpdateProgress(finalStatus, "")

		jm.Info("Job completion finalized successfully", "jobID", job.Id)
		return nil
	}

	// Create execution request with callbacks
	execReq := &runtime.ExecutionRequest{
		FullJob:            job,
		ProgressCallback:   progressCallback,
		CompletionCallback: completionCallback,
	}

	execCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	_, err = sched.EnqueueJob(execCtx, execReq)
	cancel()

	if err != nil {
		// Let scheduler handle all error cases
		// Job state remains in jobs map for status queries
		jm.Error("Scheduler failed to process job", "jobID", job.Id, "error", err)
		return nil, fmt.Errorf("scheduler error: %w", err)
	}

	jm.Info("Job submitted to scheduler", "jobID", job.Id)
	return handle, nil
}

// GetJobStatus returns the current status of a job
func (jm *JobManager) GetJobStatus(jobID string) (*v1.JobStatus, error) {
	jm.mu.RLock()
	jobState, exists := jm.jobs[jobID]
	jm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("job %s not found", jobID)
	}

	progress := jobState.GetProgress()
	return &v1.JobStatus{
		JobId:             jobID,
		SubmitterClientId: jm.daemonID,
		SubmittedAt:       jobState.Job.SubmittedAt,
		Progress:          progress,
	}, nil
}

// WatchHandle is a handle for watching job status updates
type WatchHandle struct {
	JobID   string
	Channel <-chan *v1.JobProgress
	// Internal reference to the actual channel for cleanup
	actualCh chan *v1.JobProgress
}

// WatchJobStatus subscribes to job status updates
// Returns a handle that contains a receive channel and can be used for cleanup
func (jm *JobManager) WatchJobStatus(jobID string) (*WatchHandle, error) {
	jm.mu.RLock()
	jobState, exists := jm.jobs[jobID]
	jm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("job %s not found", jobID)
	}

	// Create a watcher channel with buffer
	ch := make(chan *v1.JobProgress, 10)

	// Add to job state's watchers
	jobState.AddWatcher(ch)

	// Send current status immediately
	ch <- jobState.GetProgress()

	return &WatchHandle{
		JobID:    jobID,
		Channel:  ch,
		actualCh: ch,
	}, nil
}

// StopWatching removes a watcher from a job
// Call this when unsubscribing from job status updates
func (jm *JobManager) StopWatching(handle *WatchHandle) error {
	if handle == nil {
		return fmt.Errorf("watch handle is nil")
	}

	jm.mu.RLock()
	jobState, exists := jm.jobs[handle.JobID]
	jm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("job %s not found", handle.JobID)
	}

	jobState.RemoveWatcher(handle.actualCh)
	return nil
}

// CancelJob marks a job as cancelled
func (jm *JobManager) CancelJob(jobID string, reason string) error {
	jm.mu.Lock()
	jobState, exists := jm.jobs[jobID]
	jm.mu.Unlock()

	if !exists {
		return fmt.Errorf("job %s not found", jobID)
	}

	jm.Info("Cancelling job", "jobID", jobID, "reason", reason)

	// Mark as cancelled
	jobState.UpdateProgress(v1.JobProgress_JOB_STATUS_CANCELLED, fmt.Sprintf("Cancelled: %s", reason))
	jobState.CloseWatchers()

	return nil
}

// GetResult returns the result of a completed job
func (jm *JobManager) GetResult(jobID string) (*v1.JobResult, error) {
	jm.mu.RLock()
	jobState, exists := jm.jobs[jobID]
	jm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("job %s not found", jobID)
	}

	jobState.mu.RLock()
	defer jobState.mu.RUnlock()

	if jobState.Result == nil {
		return nil, fmt.Errorf("job %s has no result (still executing)", jobID)
	}

	return jobState.Result, nil
}
