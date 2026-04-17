package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	"github.com/Manu343726/buildozer/pkg/logging"
)

// RemoteTracker tracks the execution progress of remote jobs
type RemoteTracker struct {
	logger *logging.Logger

	mu               sync.RWMutex
	remoteExecutions map[string]*RemoteJobExecution // job ID -> remote execution info
}

// NewRemoteTracker creates a new remote job tracker
func NewRemoteTracker() *RemoteTracker {
	return &RemoteTracker{
		logger:           LogSubsystem("RemoteTracker"),
		remoteExecutions: make(map[string]*RemoteJobExecution),
	}
}

// TrackRemoteExecution registers a job for remote execution tracking
// Job IDs are universal across peers, so no mapping is needed
func (rt *RemoteTracker) TrackRemoteExecution(ctx context.Context, jobID string, remotePeerID string, preparedJob *v1.Job) error {
	if jobID == "" {
		return fmt.Errorf("job ID cannot be empty")
	}

	if remotePeerID == "" {
		return fmt.Errorf("remote peer ID cannot be empty")
	}

	rt.mu.Lock()
	defer rt.mu.Unlock()

	// Check if already tracking
	if _, exists := rt.remoteExecutions[jobID]; exists {
		return fmt.Errorf("job %s already being tracked", jobID)
	}

	execution := &RemoteJobExecution{
		JobID:        jobID,
		RemotePeerID: remotePeerID,
		PreparedJob:  preparedJob,
		LastKnownStatus: &v1.JobProgress{
			JobId:     jobID,
			Status:    v1.JobProgress_JOB_STATUS_QUEUED,
			UpdatedAt: &v1.TimeStamp{UnixMillis: time.Now().UnixMilli()},
		},
		StatusUpdatedAt: time.Now().UnixMilli(),
	}

	rt.remoteExecutions[jobID] = execution

	rt.logger.Debug("Started tracking remote job execution",
		"jobID", jobID,
		"remotePeerID", remotePeerID,
	)

	return nil
}

// UpdateRemoteStatus updates the known status of a remote job
func (rt *RemoteTracker) UpdateRemoteStatus(ctx context.Context, jobID string, status *v1.JobProgress) error {
	if jobID == "" {
		return fmt.Errorf("job ID cannot be empty")
	}

	if status == nil {
		return fmt.Errorf("status cannot be nil")
	}

	rt.mu.Lock()
	defer rt.mu.Unlock()

	execution, exists := rt.remoteExecutions[jobID]
	if !exists {
		return fmt.Errorf("job %s not being tracked", jobID)
	}

	execution.LastKnownStatus = status
	execution.StatusUpdatedAt = time.Now().UnixMilli()

	rt.logger.Debug("Updated remote job status",
		"jobID", jobID,
		"status", status.Status.String(),
	)

	return nil
}

// GetRemoteExecution returns the remote execution tracking for a job
func (rt *RemoteTracker) GetRemoteExecution(jobID string) (*RemoteJobExecution, error) {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	execution, exists := rt.remoteExecutions[jobID]
	if !exists {
		return nil, fmt.Errorf("job %s not being tracked", jobID)
	}

	// Return a copy to prevent external modification
	executionCopy := *execution
	return &executionCopy, nil
}

// GetRemoteStatus returns the last known status of a remote job
func (rt *RemoteTracker) GetRemoteStatus(jobID string) (*v1.JobProgress, error) {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	execution, exists := rt.remoteExecutions[jobID]
	if !exists {
		return nil, fmt.Errorf("job %s not being tracked", jobID)
	}

	// Return a copy of the status
	statusCopy := *execution.LastKnownStatus
	return &statusCopy, nil
}

// StopTracking removes a job from remote execution tracking
func (rt *RemoteTracker) StopTracking(jobID string) error {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	execution, exists := rt.remoteExecutions[jobID]
	if !exists {
		return fmt.Errorf("job %s not being tracked", jobID)
	}

	delete(rt.remoteExecutions, jobID)

	rt.logger.Debug("Stopped tracking remote job",
		"jobID", jobID,
		"remotePeerID", execution.RemotePeerID,
	)

	return nil
}

// IsRemoteJob returns true if a job is being executed remotely
func (rt *RemoteTracker) IsRemoteJob(jobID string) bool {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	_, exists := rt.remoteExecutions[jobID]
	return exists
}

// GetAllRemoteExecutions returns all currently tracked remote executions
func (rt *RemoteTracker) GetAllRemoteExecutions() []*RemoteJobExecution {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	executions := make([]*RemoteJobExecution, 0, len(rt.remoteExecutions))
	for _, exec := range rt.remoteExecutions {
		execCopy := *exec
		executions = append(executions, &execCopy)
	}

	return executions
}

// trackLocalJobProgress creates a tracker entry for a locally-executed job
// This allows the tracker to handle both local and remote jobs transparently
// The caller will use the returned channel to report progress
func (rt *RemoteTracker) trackLocalJobProgress(jobID string, progressChan <-chan *v1.JobProgress) {
	// For local jobs, we don't need to store anything in remoteExecutions
	// The job manager directly uses the progress channel to report updates
	// This method exists for symmetry and future tracking needs
	rt.logger.Debug("Tracking local job progress", "jobID", jobID)
}
