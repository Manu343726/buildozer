package scheduler

import (
	"context"
	"errors"
	"fmt"
	"time"

	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	"github.com/Manu343726/buildozer/pkg/logging"
	"github.com/Manu343726/buildozer/pkg/runtime"
	"github.com/Manu343726/buildozer/pkg/scheduler/heuristics"
	"github.com/Manu343726/buildozer/pkg/staging"
)

// Scheduler is the main job scheduling coordinator
// It makes placement decisions about where jobs should execute
// and maintains a queue of pending jobs for scheduling when runtimes become available
type Scheduler struct {
	logger *logging.Logger

	config    *SchedulerConfig
	heuristic SchedulingHeuristic
	queue     *JobQueue
}

// NewScheduler creates a new job scheduler with the given configuration
func NewScheduler(config *SchedulerConfig) (*Scheduler, error) {
	if config == nil {
		return nil, fmt.Errorf("scheduler config cannot be nil")
	}

	if config.Heuristic == nil {
		return nil, fmt.Errorf("heuristic cannot be nil")
	}

	if config.RuntimeManager == nil {
		return nil, fmt.Errorf("runtime manager cannot be nil")
	}

	if config.PeerManager == nil {
		return nil, fmt.Errorf("peer manager cannot be nil")
	}

	if config.LocalDaemonID == "" {
		return nil, fmt.Errorf("local daemon ID cannot be empty")
	}

	return &Scheduler{
		logger:    LogSubsystem("Scheduler"),
		config:    config,
		heuristic: config.Heuristic,
		queue:     NewJobQueue(),
	}, nil
}

// scheduleJob makes a scheduling decision for a job and prepares it for execution
// Returns a SchedulingDecision with placement information
// This is an internal method - use EnqueueJob as the public entry point
func (s *Scheduler) scheduleJob(ctx context.Context, execReq *runtime.ExecutionRequest, jobQueueCount int) (*SchedulingDecision, error) {
	if execReq == nil {
		return nil, ErrNilJob
	}

	job := execReq.FullJob
	if job == nil {
		return nil, ErrNilJob
	}

	s.logger.Debug("Scheduling job",
		"jobID", job.Id,
		"queueCount", jobQueueCount,
	)

	// Build scheduling context with access to runtime manager
	// The heuristic will use it to query runtimes directly
	schedCtx := &SchedulingContext{
		Job:            job,
		RuntimeManager: s.config.RuntimeManager,
		LocalDaemonID:  s.config.LocalDaemonID,
		JobCount:       jobQueueCount,
	}

	// Use heuristic to make scheduling decision
	decision, err := s.heuristic.Schedule(ctx, schedCtx)
	if err != nil {
		// Differentiate between temporary and permanent errors
		if errors.Is(err, heuristics.ErrNoMatchingRuntimes) {
			// No compatible runtimes exist - this is a permanent failure
			// The job cannot run anywhere with the given requirements
			s.logger.Error("No compatible runtimes found for job - permanent failure",
				"jobID", job.Id,
				"requirement", fmt.Sprintf("%v", job.RuntimeRequirement),
			)
			return nil, ErrNoCompatibleRuntimes
		}

		// Check for other known temporary errors
		if errors.Is(err, ErrAllRuntimesBusy) {
			// Matching runtimes exist but all are busy - queue for retry
			s.logger.Info("All compatible runtimes are busy - queuing for retry",
				"jobID", job.Id,
			)
			return nil, ErrAllRuntimesBusy
		}

		s.logger.Error("Scheduling heuristic failed", "jobID", job.Id, "error", err)
		return nil, fmt.Errorf("scheduling failed: %w", err)
	}

	s.logger.Debug("Scheduling decision made",
		"jobID", job.Id,
		"runtimeID", decision.SelectedRuntime.Id,
		"reason", decision.Reason,
	)

	// Get the runtime implementation for execution
	runtimeImpl, err := s.config.RuntimeManager.GetRuntimeByID(ctx, decision.SelectedRuntime.Id)
	if err != nil {
		s.logger.Error("Failed to get runtime for execution", "jobID", job.Id, "runtimeID", decision.SelectedRuntime.Id, "error", err)
		return nil, fmt.Errorf("failed to get runtime: %w", err)
	}

	// Execute the job on the selected runtime
	s.logger.Debug("Executing job", "jobID", job.Id, "runtimeID", decision.SelectedRuntime.Id)
	go func() {
		result, err := runtimeImpl.Execute(ctx, execReq)
		if err != nil {
			s.logger.Error("Job execution failed", "jobID", job.Id, "error", err)
			// Create a failed result to pass to completion callback
			failedResult := &runtime.ExecutionResult{
				ExitCode: 1,
				Stderr:   []byte(err.Error()),
			}
			// Call completion callback for failed execution
			if execReq.CompletionCallback != nil {
				if cbErr := execReq.CompletionCallback(ctx, job, failedResult); cbErr != nil {
					s.logger.Error("Completion callback failed after job execution error", "jobID", job.Id, "callbackError", cbErr, "executionError", err)
				}
			}
		} else {
			s.logger.Debug("Job execution completed", "jobID", job.Id, "exitCode", result.ExitCode)
			// Call completion callback for successful execution
			if execReq.CompletionCallback != nil {
				if err := execReq.CompletionCallback(ctx, job, result); err != nil {
					s.logger.Error("Completion callback failed", "jobID", job.Id, "error", err)
				}
			}
		}

		// Try to schedule the next job from queue now that this one is done
		s.logger.Debug("Attempting to schedule next job from queue after completion", "jobID", job.Id)
		s.TryScheduleNext(ctx)
	}()

	return decision, nil
}

// EnqueueJob adds a job to the queue and attempts to schedule it
// If scheduling succeeds, the decision is returned and job is not queued
// If scheduling fails because all runtimes are busy, the job is queued for later retry
// Returns the scheduling decision if immediately scheduled, or error if failed
func (s *Scheduler) EnqueueJob(ctx context.Context, execReq *runtime.ExecutionRequest) (*SchedulingDecision, error) {
	if execReq == nil {
		return nil, ErrNilJob
	}

	job := execReq.FullJob
	if job == nil {
		return nil, ErrNilJob
	}

	// Try to schedule the job immediately
	decision, err := s.scheduleJob(ctx, execReq, s.queue.Len())

	// If scheduling succeeded, return the decision
	if err == nil {
		s.logger.Debug("Job scheduled immediately", "jobID", job.Id)
		return decision, nil
	}

	// If all runtimes are busy, queue the job for later retry
	if err == ErrAllRuntimesBusy {
		s.logger.Debug("All runtimes busy, queuing job for later retry", "jobID", job.Id)
		queuedJob := &QueuedJob{
			ExecReq:    execReq,
			EnqueuedAt: getUnixMillis(),
		}
		s.queue.Enqueue(queuedJob)
		return nil, ErrAllRuntimesBusy
	}

	// If no compatible runtimes exist, this is a permanent failure - don't queue
	if err == ErrNoCompatibleRuntimes {
		s.logger.Error("Job cannot execute - no compatible runtimes found", "jobID", job.Id)
		return nil, ErrNoCompatibleRuntimes
	}

	// For other errors, don't queue - return the error
	s.logger.Error("Job scheduling failed", "jobID", job.Id, "error", err)
	return nil, err
}

// TryScheduleNext attempts to schedule the next job in the queue
// This is called when a job completes or runtimes become available
// Returns true if a job was successfully scheduled, false otherwise
func (s *Scheduler) TryScheduleNext(ctx context.Context) bool {
	queuedJob := s.queue.Peek()
	if queuedJob == nil {
		return false
	}

	s.logger.Debug("Attempting to schedule next queued job",
		"jobID", queuedJob.ExecReq.FullJob.Id,
		"queueLen", s.queue.Len(),
	)

	// Try to schedule the job
	_, err := s.scheduleJob(ctx, queuedJob.ExecReq, s.queue.Len()-1)

	// If successful, remove from queue
	if err == nil {
		s.queue.Dequeue()
		s.logger.Debug("Queued job scheduled successfully",
			"jobID", queuedJob.ExecReq.FullJob.Id,
			"remainingInQueue", s.queue.Len(),
		)
		return true
	}

	// If still busy, keep in queue for next retry
	if err == ErrAllRuntimesBusy {
		s.logger.Debug("Queued job still blocked - runtimes still busy",
			"jobID", queuedJob.ExecReq.FullJob.Id,
		)
		return false
	}

	// For other errors, remove from queue and log
	s.queue.Dequeue()
	s.logger.Error("Queued job scheduling failed persistently, removing from queue",
		"jobID", queuedJob.ExecReq.FullJob.Id,
		"error", err,
	)
	return false
}

// OnJobCompletion should be called when a job completes (success or failure)
// This triggers scheduling of the next queued job
func (s *Scheduler) OnJobCompletion(ctx context.Context) {
	if s.TryScheduleNext(ctx) {
		// Successfully scheduled the next job
		// Continue trying to schedule more jobs
		for s.TryScheduleNext(ctx) {
			// Keep scheduling while jobs are available and runtimes allow
		}
	}
}

// GetQueuedJobs returns a copy of all currently queued jobs
func (s *Scheduler) GetQueuedJobs() []*QueuedJob {
	return s.queue.GetAll()
}

// GetQueueLength returns the number of jobs currently waiting in the queue
func (s *Scheduler) GetQueueLength() int {
	return s.queue.Len()
}

// RemoveQueuedJob removes a specific job from the queue by ID
// Returns true if the job was found and removed
func (s *Scheduler) RemoveQueuedJob(jobID string) bool {
	return s.queue.Remove(jobID)
}

// FinalizeJobCompletion materializes and verifies job outputs after execution completes.
// This should be called when a job execution finishes (success or failure).
// For successful jobs, this materializes output files and verifies their integrity.
// Returns the finalized job result with populated outputs.
func (s *Scheduler) FinalizeJobCompletion(ctx context.Context, job *v1.Job, jobResult *v1.JobResult) (*v1.JobResult, error) {
	if job == nil || jobResult == nil {
		return jobResult, nil
	}

	s.logger.Debug("Finalizing job completion with output materialization",
		"jobID", job.Id,
		"exitCode", jobResult.ExitCode,
	)

	// Only materialize outputs for successful jobs
	if jobResult.ExitCode != 0 {
		s.logger.Debug("Skipping output materialization for failed job",
			"jobID", job.Id,
		)
		return jobResult, nil
	}

	// Early exit if no outputs to materialize
	if len(job.Outputs) == 0 && len(jobResult.Outputs) == 0 {
		s.logger.Debug("No outputs to materialize",
			"jobID", job.Id,
		)
		return jobResult, nil
	}

	stager := staging.NewJobDataStager(job.Cwd)

	// Materialize outputs combining job specification and execution result
	materializedOutputs, err := stager.MaterializeJobOutputs(ctx, job, jobResult, job.Cwd)
	if err != nil {
		s.logger.Error("Failed to materialize job outputs",
			"jobID", job.Id,
			"error", err,
		)
		jobResult.Status = v1.JobResult_JOB_STATUS_FAILED
		jobResult.ErrorMessage = fmt.Sprintf("Output materialization failed: %v", err)
		jobResult.ExitCode = 1
		return jobResult, fmt.Errorf("output materialization failed: %w", err)
	}

	s.logger.Debug("Materializing outputs complete",
		"jobID", job.Id,
		"outputCount", len(materializedOutputs),
	)

	// Update result with materialized outputs
	jobResult.Outputs = materializedOutputs

	// Verify all materialized outputs using the stager's batch verify function
	err = stager.VerifyJobResultOutputs(ctx, jobResult, job.Cwd, staging.VerificationModeSaved)
	if err != nil {
		s.logger.Error("Output verification failed",
			"jobID", job.Id,
			"error", err,
		)
		jobResult.Status = v1.JobResult_JOB_STATUS_FAILED
		jobResult.ErrorMessage = fmt.Sprintf("Output verification failed: %v", err)
		jobResult.ExitCode = 1
		return jobResult, fmt.Errorf("output verification failed: %w", err)
	}

	s.logger.Debug("Output verification complete",
		"jobID", job.Id,
		"outputCount", len(materializedOutputs),
	)

	// Set final status to completed for successful jobs
	jobResult.Status = v1.JobResult_JOB_STATUS_COMPLETED

	return jobResult, nil
}

// getUnixMillis returns the current time in Unix milliseconds
func getUnixMillis() int64 {
	return time.Now().UnixMilli()
}
