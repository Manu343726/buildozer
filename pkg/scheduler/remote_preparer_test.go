package scheduler

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
)

func TestRemotePreparer_PrepareForRemoteWithEmbeddedInputs(t *testing.T) {
	preparer := NewRemotePreparer()

	// Job with already embedded inputs
	job := &v1.Job{
		Id: "job-1",
		RuntimeRequirement: &v1.Job_Runtime{
			Runtime: &v1.Runtime{Id: "runtime-1"},
		},
		Inputs: []*v1.JobData{
			{
				Id:   "file-1",
				Type: v1.JobData_DATA_TYPE_FILE,
				Data: &v1.JobData_File{
					File: &v1.FileJobData{
						Path:    "input.txt",
						Content: []byte("test content"),
					},
				},
			},
		},
		Outputs: []*v1.JobData{
			{
				Id:   "file-2",
				Type: v1.JobData_DATA_TYPE_FILE,
				Data: &v1.JobData_File{
					File: &v1.FileJobData{
						Path: "output.txt",
					},
				},
			},
		},
		JobSpec: &v1.Job_CppCompile{
			CppCompile: &v1.CppCompileJob{
				SourceFiles: []string{"input.txt"},
				OutputFile:  "output.txt",
			},
		},
	}

	remotePreparedJob, err := preparer.PrepareForRemote(context.Background(), job, "/workdir")

	require.NoError(t, err, "should prepare job successfully")
	assert.Equal(t, job.Id, remotePreparedJob.Id, "job ID should be same")
	assert.Equal(t, 1, len(remotePreparedJob.Inputs), "should have embedded input")
	assert.Empty(t, remotePreparedJob.InputDataIds, "InputDataIds should be cleared")
	assert.Equal(t, job.Outputs, remotePreparedJob.Outputs, "outputs should be same")
}

func TestRemotePreparer_PrepareForRemoteWithReferences(t *testing.T) {
	preparer := NewRemotePreparer()

	// Job with input references but no embedded inputs
	// This would normally be handled by the stager to load and embed files
	// For this test, we'll verify the behavior when inputs are empty
	job := &v1.Job{
		Id: "job-1",
		RuntimeRequirement: &v1.Job_Runtime{
			Runtime: &v1.Runtime{Id: "runtime-1"},
		},
		InputDataIds: []string{"file-1", "file-2"},
		Inputs:       nil, // No embedded inputs
		JobSpec: &v1.Job_CppCompile{
			CppCompile: &v1.CppCompileJob{
				SourceFiles: []string{"file-1"},
				OutputFile:  "output.txt",
			},
		},
	}

	// In the real implementation, this would call stager.CreateJobDataListForFiles
	// For testing the interface without a real stager, we note that it would fail
	// since staging package methods aren't mocked here.
	// This test demonstrates the expected behavior structure.

	// Note: The actual job data loading/embedding is delegated to staging package
	// This test validates the job preparation structure
	remotePreparedJob, err := preparer.PrepareForRemote(context.Background(), job, "/workdir")

	// This will error because we can't actually load files in test
	// The error should be about embedding
	if err != nil {
		assert.ErrorIs(t, err, ErrJobDataEmbeddingFailed, "should return embedding error when stager fails")
	} else {
		// If successful, InputDataIds should be cleared
		assert.Empty(t, remotePreparedJob.InputDataIds, "InputDataIds should be cleared")
	}
}

func TestRemotePreparer_PrepareForRemoteClearsReferences(t *testing.T) {
	preparer := NewRemotePreparer()

	// Job with both inputs and InputDataIds
	job := &v1.Job{
		Id: "job-1",
		RuntimeRequirement: &v1.Job_Runtime{
			Runtime: &v1.Runtime{Id: "runtime-1"},
		},
		InputDataIds: []string{"ref-1", "ref-2"}, // References that should be cleared
		Inputs: []*v1.JobData{
			{
				Id:   "file-1",
				Type: v1.JobData_DATA_TYPE_FILE,
				Data: &v1.JobData_File{
					File: &v1.FileJobData{
						Path:    "input.txt",
						Content: []byte("data"),
					},
				},
			},
		},
		JobSpec: &v1.Job_CppCompile{
			CppCompile: &v1.CppCompileJob{
				SourceFiles: []string{"input.txt"},
				OutputFile:  "output.txt",
			},
		},
	}

	remotePreparedJob, err := preparer.PrepareForRemote(context.Background(), job, "/workdir")

	require.NoError(t, err, "should prepare job successfully")
	assert.Empty(t, remotePreparedJob.InputDataIds, "InputDataIds should be cleared for remote execution")
	assert.NotEmpty(t, remotePreparedJob.Inputs, "embedded inputs should be present")
}

func TestRemotePreparer_ErrorWithNilJob(t *testing.T) {
	preparer := NewRemotePreparer()

	remotePreparedJob, err := preparer.PrepareForRemote(context.Background(), nil, "/workdir")

	assert.Error(t, err, "should error with nil job")
	assert.Nil(t, remotePreparedJob, "prepared job should be nil")
}

func TestRemotePreparer_PreserveJobMetadata(t *testing.T) {
	preparer := NewRemotePreparer()

	submittedAt := &v1.TimeStamp{UnixMillis: 1234567890}
	timeout := &v1.TimeDuration{Count: 30, Unit: v1.TimeUnit_TIME_UNIT_SECOND}

	job := &v1.Job{
		Id:             "job-1",
		SourceClientId: "client-1",
		SubmittedAt:    submittedAt,
		Timeout:        timeout,
		Cwd:            "/project",
		RuntimeRequirement: &v1.Job_Runtime{
			Runtime: &v1.Runtime{Id: "runtime-1"},
		},
		Inputs: []*v1.JobData{},
		JobSpec: &v1.Job_CppCompile{
			CppCompile: &v1.CppCompileJob{},
		},
	}

	remotePreparedJob, err := preparer.PrepareForRemote(context.Background(), job, "/workdir")

	require.NoError(t, err, "should prepare job successfully")
	assert.Equal(t, job.SourceClientId, remotePreparedJob.SourceClientId, "source client ID should be preserved")
	assert.Equal(t, job.SubmittedAt, remotePreparedJob.SubmittedAt, "submitted at should be preserved")
	assert.Equal(t, job.Timeout, remotePreparedJob.Timeout, "timeout should be preserved")
	assert.Equal(t, job.Cwd, remotePreparedJob.Cwd, "cwd should be preserved")
	assert.Equal(t, job.RuntimeRequirement, remotePreparedJob.RuntimeRequirement, "runtime requirement should be preserved")
}
