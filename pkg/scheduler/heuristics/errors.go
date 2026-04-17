package heuristics

import "fmt"

// Heuristics-specific errors
var (
	ErrNilSchedulingContext = fmt.Errorf("scheduling context cannot be nil")
	ErrNilJob              = fmt.Errorf("job cannot be nil")
	ErrNilRuntime          = fmt.Errorf("runtime cannot be nil")
	ErrNilRuntimeManager   = fmt.Errorf("runtime manager cannot be nil")
	ErrNilRuntimeMatchQuery = fmt.Errorf("runtime match query cannot be nil")
	ErrNoRuntimeRequirement = fmt.Errorf("job runtime requirement not set")
	ErrNoMatchingRuntimes  = fmt.Errorf("no runtimes matched the job requirements")
)
