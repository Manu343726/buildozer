package scheduler

import (
	"context"
	"fmt"

	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	"github.com/Manu343726/buildozer/pkg/logging"
	"github.com/Manu343726/buildozer/pkg/staging"
)

// RemotePreparer prepares jobs for remote execution by embedding all input data
type RemotePreparer struct {
	logger *logging.Logger
}

// NewRemotePreparer creates a new remote job preparer
func NewRemotePreparer() *RemotePreparer {
	return &RemotePreparer{
		logger: LogSubsystem("RemotePreparer"),
	}
}

// PrepareForRemote takes a job and embeds all its input data, making it ready for remote execution
// The returned job has no input references - all data is embedded in the Inputs field
func (rp *RemotePreparer) PrepareForRemote(ctx context.Context, job *v1.Job, workDir string) (*v1.Job, error) {
	if job == nil {
		return nil, fmt.Errorf("job cannot be nil")
	}

	rp.logger.Debug("Preparing job for remote execution",
		"jobID", job.Id,
		"inputCount", len(job.Inputs),
		"inputDataIdsCount", len(job.InputDataIds),
	)

	// Create a copy of the job
	remotePreparedJob := &v1.Job{
		Id:                 job.Id,
		RuntimeRequirement: job.RuntimeRequirement,
		Inputs:             job.Inputs,
		Outputs:            job.Outputs,
		JobSpec:            job.JobSpec,
		SourceClientId:     job.SourceClientId,
		SubmittedAt:        job.SubmittedAt,
		Timeout:            job.Timeout,
		Cwd:                job.Cwd,
	}

	// For remote execution, we must embed all input data
	// Check if inputs are already embedded or need to be loaded
	if len(job.Inputs) == 0 && len(job.InputDataIds) > 0 {
		rp.logger.Debug("No inputs embedded - need to load and embed them",
			"jobID", job.Id,
			"inputDataIdsCount", len(job.InputDataIds),
		)

		// Load all input files and create JobData with embedded content
		stager := staging.NewJobDataStager(workDir)
		embeddedInputs, err := stager.CreateJobDataListForFiles(ctx, job.InputDataIds, staging.JobDataModeContent)
		if err != nil {
			rp.logger.Error("Failed to embed job inputs", "jobID", job.Id, "error", err)
			return nil, fmt.Errorf("%w: %v", ErrJobDataEmbeddingFailed, err)
		}

		remotePreparedJob.Inputs = embeddedInputs
		rp.logger.Debug("Embedded job inputs",
			"jobID", job.Id,
			"embeddedCount", len(embeddedInputs),
		)
	}

	// Clear InputDataIds since we're embedding everything
	// Remote daemon should not use references for remote jobs
	remotePreparedJob.InputDataIds = nil

	rp.logger.Debug("Job prepared for remote execution",
		"jobID", job.Id,
		"embeddedInputCount", len(remotePreparedJob.Inputs),
	)

	return remotePreparedJob, nil
}
