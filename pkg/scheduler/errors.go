package scheduler

import "fmt"

// Scheduler-specific errors
var (
	ErrNilSchedulingContext     = fmt.Errorf("scheduling context cannot be nil")
	ErrNilJob                   = fmt.Errorf("job cannot be nil")
	ErrNilRuntime               = fmt.Errorf("runtime cannot be nil")
	ErrNilRuntimeManager        = fmt.Errorf("runtime manager cannot be nil")
	ErrNilRuntimeMatchQuery     = fmt.Errorf("runtime match query cannot be nil")
	ErrNoRuntimeRequirement     = fmt.Errorf("job runtime requirement not set")
	ErrNoMatchingRuntimes       = fmt.Errorf("no runtimes matched the job requirements")
	ErrNoCompatibleRuntimes     = fmt.Errorf("no compatible runtimes exist for this job - permanent failure")
	ErrAllRuntimesBusy          = fmt.Errorf("all compatible runtimes are busy - job queued for retry")
	ErrRemotePeerNotFound       = fmt.Errorf("remote peer not found")
	ErrRemoteJobFailed          = fmt.Errorf("remote job execution failed")
	ErrJobDataEmbeddingFailed   = fmt.Errorf("failed to embed job data for remote execution")
	ErrRemoteStatusUpdateFailed = fmt.Errorf("failed to update remote job status")
)
