package heuristics

import (
	"context"

	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	"github.com/Manu343726/buildozer/pkg/runtimes"
)

// SchedulingHeuristic defines the interface for scheduling decision logic
type SchedulingHeuristic interface {
	// Schedule makes a scheduling decision for a job
	// Returns a SchedulingDecision indicating where the job should execute
	Schedule(ctx context.Context, decision *SchedulingContext) (*SchedulingDecision, error)

	// Name returns a human-readable name for this heuristic
	Name() string
}

// SchedulingDecision contains the decision made by a scheduling heuristic
type SchedulingDecision struct {
	// PeerId is the ID of the peer daemon where the job should be executed
	// This can be the local daemon ID (from SchedulingContext.LocalDaemonID) for local execution
	// or a remote peer's ID for remote execution
	PeerId string

	// SelectedRuntime is the selected runtime for execution
	// The runtime manager will return a Runtime interface implementation that handles
	// both local and remote execution transparently through RPC
	SelectedRuntime *v1.Runtime

	// Reason is a human-readable explanation of why this decision was made
	Reason string
}

// SchedulingContext contains all information needed for scheduling decisions
type SchedulingContext struct {
	Job *v1.Job

	// RuntimeManager provides access to all available runtimes (local and remote)
	// The heuristic queries this directly to make scheduling decisions
	RuntimeManager runtimes.Manager

	// LocalDaemonID is the ID of the local daemon
	LocalDaemonID string

	// JobCount is the current number of jobs in the queue
	JobCount int
}
