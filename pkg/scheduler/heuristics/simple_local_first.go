package heuristics

import (
	"context"
	"fmt"

	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	"github.com/Manu343726/buildozer/pkg/logging"
)

// SimpleLocalFirstHeuristic implements a simple scheduling heuristic that prefers local execution
type SimpleLocalFirstHeuristic struct {
	logger *logging.Logger
}

// NewSimpleLocalFirstHeuristic creates a new simple local-first heuristic
func NewSimpleLocalFirstHeuristic() *SimpleLocalFirstHeuristic {
	return &SimpleLocalFirstHeuristic{
		logger: Log().Child("SimpleLocalFirstHeuristic"),
	}
}

// Schedule implements SchedulingHeuristic.Schedule()
// Returns local execution if a compatible local runtime exists, otherwise selects remote runtime
func (h *SimpleLocalFirstHeuristic) Schedule(ctx context.Context, decision *SchedulingContext) (*SchedulingDecision, error) {
	if decision == nil {
		return nil, ErrNilSchedulingContext
	}

	if decision.Job == nil {
		return nil, ErrNilJob
	}

	if decision.RuntimeManager == nil {
		return nil, ErrNilRuntimeManager
	}

	// Extract the runtime requirement from the job
	var query *v1.RuntimeMatchQuery
	var explicitRuntime *v1.Runtime

	switch req := decision.Job.RuntimeRequirement.(type) {
	case *v1.Job_Runtime:
		// Explicit runtime provided
		if req.Runtime == nil {
			return nil, ErrNilRuntime
		}
		explicitRuntime = req.Runtime
	case *v1.Job_RuntimeMatchQuery:
		// Runtime query provided
		if req.RuntimeMatchQuery == nil {
			return nil, ErrNilRuntimeMatchQuery
		}
		query = req.RuntimeMatchQuery
	default:
		return nil, ErrNoRuntimeRequirement
	}

	// If explicit runtime was provided, use it
	if explicitRuntime != nil {
		// For explicit runtimes, try to execute locally if available
		_, err := decision.RuntimeManager.GetRuntimeByID(ctx, explicitRuntime.Id)
		if err == nil {
			// Runtime is available locally
			return &SchedulingDecision{
				PeerId:          decision.LocalDaemonID,
				SelectedRuntime: explicitRuntime,
				Reason:          "Using explicitly specified local runtime",
			}, nil
		}

		// Runtime is not local - schedule on the peer that has it
		// Extract peer ID from the runtime's peer_ids
		var peerId string
		if len(explicitRuntime.PeerIds) > 0 {
			peerId = explicitRuntime.PeerIds[0]
		}

		if peerId == "" {
			return nil, fmt.Errorf("explicit runtime %s not available locally and no remote peer found", explicitRuntime.Id)
		}

		return &SchedulingDecision{
			PeerId:          peerId,
			SelectedRuntime: explicitRuntime,
			Reason:          "Explicit runtime available on remote peer",
		}, nil
	}

	// Query-based scheduling - find matching runtimes
	matches, err := decision.RuntimeManager.Match(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to find matching runtimes: %w", err)
	}

	if len(matches) == 0 {
		return nil, ErrNoMatchingRuntimes
	}

	// Prefer local runtimes first
	for _, rt := range matches {
		// Check if this runtime is available locally
		_, err := decision.RuntimeManager.GetRuntimeByID(ctx, rt.Id)
		if err == nil {
			// Runtime is available locally
			return &SchedulingDecision{
				PeerId:          decision.LocalDaemonID,
				SelectedRuntime: rt,
				Reason:          "Selected available local runtime matching query",
			}, nil
		}
	}

	// No local runtime matches - use first remote runtime
	selectedRuntime := matches[0]
	var peerId string
	if len(selectedRuntime.PeerIds) > 0 {
		peerId = selectedRuntime.PeerIds[0]
	}

	if peerId == "" {
		return nil, fmt.Errorf("no local match and selected remote runtime %s has no peer ID", selectedRuntime.Id)
	}

	return &SchedulingDecision{
		PeerId:          peerId,
		SelectedRuntime: selectedRuntime,
		Reason:          "Selected remote runtime - no local match available",
	}, nil
}

// Name returns the name of this heuristic
func (h *SimpleLocalFirstHeuristic) Name() string {
	return "SimpleLocalFirst"
}
