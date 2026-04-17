package scheduler

import (
	"github.com/Manu343726/buildozer/pkg/logging"
	"github.com/Manu343726/buildozer/pkg/peers"
	"github.com/Manu343726/buildozer/pkg/runtimes"
	"github.com/Manu343726/buildozer/pkg/scheduler/heuristics"

	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
)

// SchedulingDecision re-exports the heuristics package type for backward compatibility
type SchedulingDecision = heuristics.SchedulingDecision

// SchedulingContext re-exports the heuristics package type for backward compatibility
type SchedulingContext = heuristics.SchedulingContext

// SchedulingHeuristic re-exports the heuristics package interface for backward compatibility
type SchedulingHeuristic = heuristics.SchedulingHeuristic

// RemoteJobExecution tracks the execution of a job on a remote peer
type RemoteJobExecution struct {
	// JobID is the ID of the job being executed remotely
	JobID string

	// RemotePeerID is the ID of the peer executing the job
	RemotePeerID string

	// PreparedJob is the job with all inputs embedded (ready for remote execution)
	PreparedJob *v1.Job

	// LastKnownStatus is the last known status of the remote job
	LastKnownStatus *v1.JobProgress

	// StatusUpdatedAt is the timestamp of the last status update
	StatusUpdatedAt int64 // Unix milliseconds
}

// Log creates a logger for the scheduler package
func Log() *logging.Logger {
	return logging.Log("scheduler")
}

// LogSubsystem creates a logger for the scheduler with a subsystem
func LogSubsystem(subsystem string) *logging.Logger {
	return logging.Log("scheduler").Child(subsystem)
}

// SchedulerConfig holds configuration for the scheduler
type SchedulerConfig struct {
	// Heuristic is the scheduling heuristic to use for placement decisions
	Heuristic SchedulingHeuristic

	// RuntimeManager provides access to all available runtimes (local and remote)
	RuntimeManager runtimes.Manager

	// PeerManager provides access to peer information for remote job execution
	PeerManager *peers.Manager

	// LocalDaemonID is the ID of this daemon for identification
	LocalDaemonID string
}
