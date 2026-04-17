package scheduler

import (
	"github.com/Manu343726/buildozer/pkg/scheduler/heuristics"
)

// NewSimpleLocalFirstHeuristic creates a new simple local-first heuristic
// This is a convenience export from the heuristics subpackage
func NewSimpleLocalFirstHeuristic() *heuristics.SimpleLocalFirstHeuristic {
	return heuristics.NewSimpleLocalFirstHeuristic()
}
