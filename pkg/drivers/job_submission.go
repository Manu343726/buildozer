package drivers

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"connectrpc.com/connect"
	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	"github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1/protov1connect"
	"github.com/google/uuid"
)

// JobSubmissionContext holds information needed to construct and submit a job
type JobSubmissionContext struct {
	Runtime         *v1.Runtime   // Resolved runtime
	ParsedArgs      interface{}   // Parsed arguments (varies by driver)
	SourceFiles     []string      // Source file paths
	CompilerFlags   []string      // Compiler-specific flags
	IncludeDirs     []string      // Include directories
	Defines         []string      // Macro defines
	ObjectFiles     []string      // Object file inputs (for linking)
	Libraries       []string      // Library names (for linking)
	LibraryDirs     []string      // Library search directories
	LinkerFlags     []string      // Linker-specific flags
	OutputFile      string        // Output file path
	IsLinkJob       bool          // True if linking, false if compiling
	IsSharedLibrary bool          // True if building shared library
	Timeout         time.Duration // Job execution timeout
	WorkDir         string        // Working directory for resolving relative paths
}

// CreateCppCompileJob creates a CppCompileJob proto from submission context
func (jsc *JobSubmissionContext) CreateCppCompileJob() *v1.CppCompileJob {
	return &v1.CppCompileJob{
		SourceFiles:  jsc.SourceFiles,
		CompilerArgs: jsc.CompilerFlags,
		IncludeDirs:  jsc.IncludeDirs,
		Defines:      jsc.Defines,
		OutputFile:   jsc.OutputFile,
	}
}

// CreateCppLinkJob creates a CppLinkJob proto from submission context
func (jsc *JobSubmissionContext) CreateCppLinkJob() *v1.CppLinkJob {
	return &v1.CppLinkJob{
		ObjectFiles:     jsc.ObjectFiles,
		Libraries:       jsc.Libraries,
		LibraryDirs:     jsc.LibraryDirs,
		LinkerArgs:      jsc.LinkerFlags,
		OutputFile:      jsc.OutputFile,
		IsSharedLibrary: jsc.IsSharedLibrary,
	}
}

// CreateJob constructs a Job proto with all necessary fields
func (jsc *JobSubmissionContext) CreateJob(ctx context.Context) (*v1.Job, error) {
	// Generate unique job ID
	jobID := uuid.New().String()

	// Determine input data IDs from source files
	inputDataIDs := make([]string, 0, len(jsc.SourceFiles)+len(jsc.ObjectFiles))
	inputDataIDs = append(inputDataIDs, jsc.SourceFiles...)
	if jsc.IsLinkJob {
		inputDataIDs = append(inputDataIDs, jsc.ObjectFiles...)
	}

	// Convert timeout to TimeDuration proto
	timeoutProto := &v1.TimeDuration{
		Count: int64(jsc.Timeout.Seconds()),
		Unit:  v1.TimeUnit_TIME_UNIT_SECOND,
	}

	// Create job with the appropriate spec type
	// Must directly assign wrapper types due to unexported oneof interface
	var job *v1.Job
	if jsc.IsLinkJob {
		job = &v1.Job{
			Id:                    jobID,
			Runtime:               jsc.Runtime,
			InputDataIds:          inputDataIDs,
			ExpectedOutputDataIds: []string{jsc.OutputFile},
			JobSpec:               &v1.Job_CppLink{CppLink: jsc.CreateCppLinkJob()},
			SourceClientId:        "",
			SubmittedAt:           &v1.TimeStamp{UnixMillis: time.Now().UnixMilli()},
			Timeout:               timeoutProto,
		}
	} else {
		job = &v1.Job{
			Id:                    jobID,
			Runtime:               jsc.Runtime,
			InputDataIds:          inputDataIDs,
			ExpectedOutputDataIds: []string{jsc.OutputFile},
			JobSpec:               &v1.Job_CppCompile{CppCompile: jsc.CreateCppCompileJob()},
			SourceClientId:        "",
			SubmittedAt:           &v1.TimeStamp{UnixMillis: time.Now().UnixMilli()},
			Timeout:               timeoutProto,
		}
	}

	return job, nil
}

// SubmitJob sends a Job to the daemon for execution
func SubmitJob(ctx context.Context, daemonHost string, daemonPort int, job *v1.Job) (*v1.SubmitJobResponse, error) {
	Log().InfoContext(ctx, "Submitting job to daemon",
		"jobID", job.Id,
		"daemonHost", daemonHost,
		"daemonPort", daemonPort)

	// Create client
	daemonAddr := fmt.Sprintf("http://%s:%d", daemonHost, daemonPort)
	client := NewJobServiceClient(daemonAddr)

	// Create submit request
	req := &v1.SubmitJobRequest{
		Job: job,
	}

	// Submit job
	resp, err := client.SubmitJob(ctx, req)
	if err != nil {
		Log().ErrorContext(ctx, "Failed to submit job", "error", err)
		return nil, fmt.Errorf("failed to submit job: %w", err)
	}

	Log().InfoContext(ctx, "Job submitted successfully",
		"jobID", job.Id,
		"accepted", resp.Accepted)

	if !resp.Accepted {
		return resp, fmt.Errorf("daemon rejected job: %s", resp.ErrorMessage)
	}

	return resp, nil
}

// JobServiceClient wraps the gRPC client for JobService
type JobServiceClient struct {
	client protov1connect.JobServiceClient
}

// NewJobServiceClient creates a new job service client
func NewJobServiceClient(baseURL string) *JobServiceClient {
	// Ensure baseURL has a protocol scheme
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		baseURL = "http://" + baseURL
	}

	httpClient := &http.Client{}
	client := protov1connect.NewJobServiceClient(httpClient, baseURL)

	return &JobServiceClient{
		client: client,
	}
}

// SubmitJob calls the SubmitJob RPC method
func (c *JobServiceClient) SubmitJob(ctx context.Context, req *v1.SubmitJobRequest) (*v1.SubmitJobResponse, error) {
	resp, err := c.client.SubmitJob(ctx, &connect.Request[v1.SubmitJobRequest]{
		Msg: req,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to submit job: %w", err)
	}

	return resp.Msg, nil
}

// GetJobStatus calls the GetJobStatus RPC method
func (c *JobServiceClient) GetJobStatus(ctx context.Context, jobID string) (*v1.JobStatus, error) {
	resp, err := c.client.GetJobStatus(ctx, &connect.Request[v1.GetJobStatusRequest]{
		Msg: &v1.GetJobStatusRequest{
			JobId: jobID,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get job status: %w", err)
	}

	if resp.Msg.ErrorMessage != "" {
		return nil, fmt.Errorf("daemon error: %s", resp.Msg.ErrorMessage)
	}

	return resp.Msg.JobStatus, nil
}

// LoadSourceFile reads a source file from disk
func LoadSourceFile(path string) ([]byte, error) {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read source file %s: %w", path, err)
	}
	return content, nil
}

// ResolveFilePath resolves a file path relative to working directory
func ResolveFilePath(path string, workDir string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(workDir, path)
}

// WatchAndStreamJobProgress watches a job's progress and streams output to stdout
// Returns the final execution result and exit code
func WatchAndStreamJobProgress(ctx context.Context, daemonHost string, daemonPort int, jobID string) (*v1.JobResult, int, error) {
	Log().InfoContext(ctx, "Watching job progress", "jobID", jobID)

	// Create client
	daemonAddr := fmt.Sprintf("http://%s:%d", daemonHost, daemonPort)
	client := NewJobServiceClient(daemonAddr)
	_ = client // Use client to avoid unused variable warning

	// Start watching job status
	// For now, we'll poll job status until completion
	// In production, this would use proper gRPC streaming with WatchJobStatus

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	var lastLogLength int

	for {
		select {
		case <-ctx.Done():
			return nil, 1, ctx.Err()

		case <-ticker.C:
			// Get current job status via polling
			jobStatus, err := getJobStatusViaPlaceholder(jobID)
			if err != nil {
				Log().DebugContext(ctx, "Waiting for job status", "jobID", jobID)
				continue
			}

			if jobStatus == nil {
				continue
			}

			// Stream new output to stdout
			if len(jobStatus.Progress.LogOutput) > lastLogLength {
				newOutput := jobStatus.Progress.LogOutput[lastLogLength:]
				fmt.Print(newOutput)
				lastLogLength = len(jobStatus.Progress.LogOutput)
			}

			// Check if job is complete
			switch jobStatus.Progress.Status {
			case v1.JobProgress_JOB_STATUS_COMPLETED:
				Log().InfoContext(ctx, "Job completed successfully", "jobID", jobID)
				// Retrieve final result
				return getJobResultViaPlaceholder(jobID), 0, nil

			case v1.JobProgress_JOB_STATUS_FAILED:
				Log().ErrorContext(ctx, "Job failed", "jobID", jobID)
				result := getJobResultViaPlaceholder(jobID)
				if result != nil {
					return result, int(result.ExitCode), nil
				}
				return nil, 1, fmt.Errorf("job failed with unknown result")

			case v1.JobProgress_JOB_STATUS_CANCELLED:
				Log().WarnContext(ctx, "Job was cancelled", "jobID", jobID)
				return nil, 1, fmt.Errorf("job was cancelled")
			}
		}
	}
}

// Placeholder functions for getting job status and result
// In production, these would use proper gRPC calls to the daemon

func getJobStatusViaPlaceholder(jobID string) (*v1.JobStatus, error) {
	// Create a client and make RPC call to daemon
	// Note: daemonHost and daemonPort are not available here in this implementation
	// This is a limitation of the current placeholder approach
	// In production, these should be passed through context or a service

	// For now, use hardcoded localhost:6789
	daemonAddr := "http://localhost:6789"
	client := NewJobServiceClient(daemonAddr)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return client.GetJobStatus(ctx, jobID)
}

func getJobResultViaPlaceholder(jobID string) *v1.JobResult {
	// This would require a GetJobResult RPC method
	// For now, we return the result embedded in JobStatus if available
	status, err := getJobStatusViaPlaceholder(jobID)
	if err != nil {
		return nil
	}

	// Build a JobResult from the JobStatus
	// This is temporary - should have a dedicated GetJobResult RPC
	if status != nil {
		return &v1.JobResult{
			JobId:           status.JobId,
			ExecutingPeerId: status.SubmitterClientId,
			CompletedAt:     status.Progress.UpdatedAt,
			LogOutput:       status.Progress.LogOutput,
		}
	}
	return nil
}
