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

// JobManager manages the lifecycle of submitted jobs
type JobManager struct {
	*logging.Logger

	mu         sync.RWMutex
	jobs       map[string]*JobState // job_id → JobState
	queue      []*JobState          // Queue of jobs waiting to execute
	runtimeMgr *runtimes.Manager    // Manager for accessing available runtimes
	daemonID   string               // This daemon's client ID
}

// NewJobManager creates a new job manager
func NewJobManager(daemonID string, runtimeMgr *runtimes.Manager) *JobManager {
	return &JobManager{
		Logger:     Log().Child("JobManager"),
		jobs:       make(map[string]*JobState),
		queue:      make([]*JobState, 0),
		runtimeMgr: runtimeMgr,
		daemonID:   daemonID,
	}
}

// SubmitJob adds a new job to the queue
func (jm *JobManager) SubmitJob(ctx context.Context, job *v1.Job) error {
	jm.Info("Submitting job", "jobID", job.Id)

	jm.mu.Lock()
	defer jm.mu.Unlock()

	// Check if job already exists
	if _, exists := jm.jobs[job.Id]; exists {
		return fmt.Errorf("job %s already submitted", job.Id)
	}

	// Create initial job state
	jobState := &JobState{
		Job: job,
		Progress: &v1.JobProgress{
			JobId:     job.Id,
			Status:    v1.JobProgress_JOB_STATUS_READY,
			UpdatedAt: &v1.TimeStamp{UnixMillis: time.Now().UnixMilli()},
		},
		Watchers:  make([]chan *v1.JobProgress, 0),
		CreatedAt: time.Now(),
	}

	jm.jobs[job.Id] = jobState
	jm.queue = append(jm.queue, jobState)

	jm.Info("Job queued", "jobID", job.Id, "queueSize", len(jm.queue))

	// Start execution in background if not already running
	go jm.processQueue(context.Background())

	return nil
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

// WatchJobStatus subscribes to job status updates
func (jm *JobManager) WatchJobStatus(jobID string) (<-chan *v1.JobProgress, error) {
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

	return ch, nil
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

	jobState.UpdateProgress(v1.JobProgress_JOB_STATUS_CANCELLED, fmt.Sprintf("Cancelled: %s", reason))

	return nil
}

// processQueue continuously processes queued jobs
func (jm *JobManager) processQueue(ctx context.Context) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		jm.mu.Lock()
		if len(jm.queue) == 0 {
			jm.mu.Unlock()
			continue
		}

		// Get next job from queue
		jobState := jm.queue[0]
		jm.queue = jm.queue[1:]
		jm.mu.Unlock()

		// Execute job in background
		go jm.executeJob(jobState)
	}
}

// executeJob executes a single job using the appropriate runtime
func (jm *JobManager) executeJob(jobState *JobState) {
	job := jobState.Job
	jm.Info("Executing job", "jobID", job.Id, "runtimeID", job.Runtime.Id)

	// Update status to SCHEDULED
	jobState.UpdateProgress(v1.JobProgress_JOB_STATUS_SCHEDULED, "Job scheduled for execution")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get the runtime from the manager
	rt, err := jm.runtimeMgr.GetRuntimeByID(ctx, job.Runtime.Id)
	if err != nil {
		jm.Error("Runtime not found", "jobID", job.Id, "runtimeID", job.Runtime.Id, "error", err)
		jobState.UpdateProgress(v1.JobProgress_JOB_STATUS_FAILED, fmt.Sprintf("Runtime not found: %v", err))

		jobState.mu.Lock()
		jobState.Result = &v1.JobResult{
			JobId:           job.Id,
			ExecutingPeerId: jm.daemonID,
			StartedAt:       &v1.TimeStamp{UnixMillis: time.Now().UnixMilli()},
			CompletedAt:     &v1.TimeStamp{UnixMillis: time.Now().UnixMilli()},
			Status:          v1.JobResult_JOB_STATUS_FAILED,
			ErrorMessage:    err.Error(),
			ExitCode:        1,
			RuntimeUsed:     job.Runtime,
		}
		jobState.mu.Unlock()
		return
	}

	// Update status to INPUT_TRANSFER (preparing for execution)
	jobState.UpdateProgress(v1.JobProgress_JOB_STATUS_INPUT_TRANSFER, "Preparing job inputs")

	// Create progress callback to stream output in real-time
	progressCallback := func(ctx context.Context, progress *runtime.Progress) error {
		switch progress.Type {
		case runtime.ProgressTypeOutput:
			// Capture stdout/stderr output and append to job log
			jobState.mu.Lock()
			jobState.Progress.LogOutput += fmt.Sprintf("[%s] %s", progress.Source, string(progress.Data))
			jobState.mu.Unlock()

			// Notify watchers of the new output
			if jobState.Progress.Status != v1.JobProgress_JOB_STATUS_RUNNING {
				// Update to RUNNING status on first output
				jobState.UpdateProgress(v1.JobProgress_JOB_STATUS_RUNNING, "")
			} else {
				// Send current progress to watchers without updating status
				jobState.mu.RLock()
				p := *jobState.Progress
				jobState.mu.RUnlock()

				for _, ch := range jobState.Watchers {
					select {
					case ch <- &p:
					default:
						// Non-blocking send
					}
				}
			}
		}
		return nil
	}

	// Create execution request with progress callback
	// Extract the actual job from the oneof wrapper (Job_CppCompile or Job_CppLink)
	var execJob interface{}
	switch jobSpec := job.JobSpec.(type) {
	case *v1.Job_CppCompile:
		execJob = jobSpec.CppCompile
	case *v1.Job_CppLink:
		execJob = jobSpec.CppLink
	default:
		jm.Error("unknown job spec type", "jobID", job.Id, "type", fmt.Sprintf("%T", job.JobSpec))
		jobState.UpdateProgress(v1.JobProgress_JOB_STATUS_FAILED, "Unknown job spec type")

		jobState.mu.Lock()
		jobState.Result = &v1.JobResult{
			JobId:           job.Id,
			ExecutingPeerId: jm.daemonID,
			StartedAt:       &v1.TimeStamp{UnixMillis: time.Now().UnixMilli()},
			CompletedAt:     &v1.TimeStamp{UnixMillis: time.Now().UnixMilli()},
			Status:          v1.JobResult_JOB_STATUS_FAILED,
			ErrorMessage:    fmt.Sprintf("Unknown job spec type: %T", job.JobSpec),
			ExitCode:        1,
			RuntimeUsed:     job.Runtime,
		}
		jobState.mu.Unlock()
		return
	}

	execReq := &runtime.ExecutionRequest{
		Job:              execJob,
		ProgressCallback: progressCallback,
	}

	// Execute the job using the runtime
	execResult, err := rt.Execute(ctx, execReq)

	startTime := time.Now().UnixMilli()
	endTime := time.Now().UnixMilli()

	// Update progress based on result
	if err != nil {
		jm.Error("Job execution failed", "jobID", job.Id, "error", err)
		jobState.UpdateProgress(v1.JobProgress_JOB_STATUS_FAILED, fmt.Sprintf("Execution failed: %v", err))

		jobState.mu.Lock()
		jobState.Result = &v1.JobResult{
			JobId:           job.Id,
			ExecutingPeerId: jm.daemonID,
			StartedAt:       &v1.TimeStamp{UnixMillis: startTime},
			CompletedAt:     &v1.TimeStamp{UnixMillis: endTime},
			Status:          v1.JobResult_JOB_STATUS_FAILED,
			ErrorMessage:    err.Error(),
			ExitCode:        1,
			RuntimeUsed:     job.Runtime,
		}
		jobState.mu.Unlock()
		return
	}

	// Create result from execution result
	result := &v1.JobResult{
		JobId:           job.Id,
		ExecutingPeerId: jm.daemonID,
		StartedAt:       &v1.TimeStamp{UnixMillis: startTime},
		CompletedAt:     &v1.TimeStamp{UnixMillis: endTime},
		LogOutput:       string(execResult.Stdout) + string(execResult.Stderr),
		ExitCode:        int32(execResult.ExitCode),
		RuntimeUsed:     job.Runtime,
	}

	jobState.mu.Lock()
	jobState.Result = result
	jobState.mu.Unlock()

	// Update progress based on exit code
	if execResult.ExitCode != 0 {
		jm.Error("Job execution returned non-zero exit code", "jobID", job.Id, "exitCode", execResult.ExitCode)
		jobState.UpdateProgress(v1.JobProgress_JOB_STATUS_FAILED, fmt.Sprintf("Exit code: %d", execResult.ExitCode))
		result.Status = v1.JobResult_JOB_STATUS_FAILED
		result.ErrorMessage = fmt.Sprintf("Exit code %d", execResult.ExitCode)
	} else {
		jm.Info("Job execution completed successfully", "jobID", job.Id)
		jobState.UpdateProgress(v1.JobProgress_JOB_STATUS_COMPLETED, "Execution completed successfully")
		result.Status = v1.JobResult_JOB_STATUS_UNSPECIFIED // Success
	}
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
