package drivers

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"connectrpc.com/connect"
	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	"github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1/protov1connect"
	"github.com/Manu343726/buildozer/pkg/staging"
	"google.golang.org/protobuf/encoding/protojson"
)

// Context keys for driver information
const (
	ContextKeyDriverName    = "driver_name"
	ContextKeyDriverVersion = "driver_version"
)

// extractRequesterInfo builds RequesterInfo from context or defaults
func extractRequesterInfo(ctx context.Context) *v1.RequesterInfo {
	requesterInfo := &v1.RequesterInfo{
		RequestTimestamp: &v1.TimeStamp{
			UnixMillis: time.Now().UnixMilli(),
		},
	}

	// Extract driver name from context
	if driverName := ctx.Value(ContextKeyDriverName); driverName != nil {
		if name, ok := driverName.(string); ok && name != "" {
			requesterInfo.RequesterId = name
			requesterInfo.RequesterType = "driver"
			return requesterInfo
		}
	}

	// Extract driver version from context if available
	if driverVersion := ctx.Value(ContextKeyDriverVersion); driverVersion != nil {
		if version, ok := driverVersion.(string); ok && version != "" {
			// For now, just store as prerelease since we have a string version
			requesterInfo.RequesterVersion = &v1.Version{
				Major:      1,
				Prerelease: &version,
			}
		}
	}

	// Fallback if no context values - this means the context wasn't enriched with driver info
	requesterInfo.RequesterId = "driver"
	requesterInfo.RequesterType = "driver"

	return requesterInfo
}

// SubmitJob sends a Job to the daemon for execution and returns the submission confirmation and streaming response
func SubmitJob(ctx context.Context, daemonHost string, daemonPort int, job *v1.Job) (*v1.SubmissionConfirmation, *connect.ServerStreamForClient[v1.SubmitJobResponse], error) {
	Log().InfoContext(ctx, "Submitting job to daemon",
		"jobID", job.Id,
		"daemonHost", daemonHost,
		"daemonPort", daemonPort)

	// Create client
	daemonAddr := fmt.Sprintf("http://%s:%d", daemonHost, daemonPort)
	client := NewJobServiceClient(daemonAddr)

	// Create submit request with requester info from context
	req := &v1.SubmitJobRequest{
		Job:           job,
		RequesterInfo: extractRequesterInfo(ctx),
	}

	// Debug log: show full job submission proto
	protoJSON, _ := protojson.MarshalOptions{Multiline: true}.Marshal(req)
	Log().DebugContext(ctx, "Full job submission proto", "proto", string(protoJSON))

	// Submit job and get streaming response
	confirmation, stream, err := client.SubmitJobStreamingWithStream(ctx, req)
	if err != nil {
		Log().ErrorContext(ctx, "Failed to submit job", "error", err)
		return nil, nil, fmt.Errorf("failed to submit job: %w", err)
	}

	Log().InfoContext(ctx, "Job submitted successfully",
		"jobID", job.Id,
		"accepted", confirmation.Accepted)

	if !confirmation.Accepted {
		return confirmation, stream, fmt.Errorf("daemon rejected job: %s", confirmation.ErrorMessage)
	}

	return confirmation, stream, nil
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

// SubmitJobStreaming calls the SubmitJob RPC method (streaming) and returns the first confirmation message
func (c *JobServiceClient) SubmitJobStreaming(ctx context.Context, req *v1.SubmitJobRequest) (*v1.SubmissionConfirmation, error) {
	stream, err := c.client.SubmitJob(ctx, &connect.Request[v1.SubmitJobRequest]{
		Msg: req,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to submit job stream: %w", err)
	}

	// Read the first message from the stream - should be SubmissionConfirmation
	if !stream.Receive() {
		err := stream.Err()
		if err != nil {
			return nil, fmt.Errorf("failed to receive submission confirmation: %w", err)
		}
		return nil, fmt.Errorf("unexpected end of stream before receiving confirmation")
	}

	resp := stream.Msg()
	confirmation := resp.GetConfirmation()
	if confirmation == nil {
		return nil, fmt.Errorf("expected SubmissionConfirmation but got %T", resp.Response)
	}

	return confirmation, nil
}

// SubmitJobStreamingWithStream calls the SubmitJob RPC method (streaming) and returns both confirmation and the open stream
func (c *JobServiceClient) SubmitJobStreamingWithStream(ctx context.Context, req *v1.SubmitJobRequest) (*v1.SubmissionConfirmation, *connect.ServerStreamForClient[v1.SubmitJobResponse], error) {
	stream, err := c.client.SubmitJob(ctx, &connect.Request[v1.SubmitJobRequest]{
		Msg: req,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to submit job stream: %w", err)
	}

	// Read the first message from the stream - should be SubmissionConfirmation
	if !stream.Receive() {
		err := stream.Err()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to receive submission confirmation: %w", err)
		}
		return nil, nil, fmt.Errorf("unexpected end of stream before receiving confirmation")
	}

	resp := stream.Msg()
	confirmation := resp.GetConfirmation()
	if confirmation == nil {
		return nil, nil, fmt.Errorf("expected SubmissionConfirmation but got %T", resp.Response)
	}

	return confirmation, stream, nil
}

// GetJobStatus calls the GetJobStatus RPC method
func (c *JobServiceClient) GetJobStatus(ctx context.Context, jobID string) (*v1.JobStatus, error) {
	resp, err := c.client.GetJobStatus(ctx, &connect.Request[v1.GetJobStatusRequest]{
		Msg: &v1.GetJobStatusRequest{
			JobId:         jobID,
			RequesterInfo: extractRequesterInfo(ctx),
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

// WatchAndStreamJobProgressFromStream processes status updates from a streaming submission response,
// streams output to stdout, and writes output files to the specified directory.
// This function reads StatusUpdate messages from the open submission stream.
func WatchAndStreamJobProgressFromStream(ctx context.Context, stream *connect.ServerStreamForClient[v1.SubmitJobResponse], daemonHost string, daemonPort int, jobID string, outputDir string) (*v1.JobResult, int, error) {
	Log().DebugContext(ctx, "Watching job progress from submission stream", "jobID", jobID, "outputDir", outputDir)

	var lastLogLength int
	var finalStatus *v1.JobStatus

	// Process stream updates from the submission response
	for stream.Receive() {
		resp := stream.Msg()
		if resp == nil {
			continue
		}

		// Extract status from either oneof variant
		var status *v1.JobStatus

		// Check for StatusUpdate variant (the normal case for progress updates)
		if statusUpdate := resp.GetStatusUpdate(); statusUpdate != nil {
			status = statusUpdate.JobStatus
		} else if confirmation := resp.GetConfirmation(); confirmation != nil {
			// Skip confirmation messages (we already got the first one)
			status = confirmation.JobStatus
		}

		if status == nil {
			continue
		}

		finalStatus = status

		// Log status update with details
		statusStr := status.Progress.Status.String()
		Log().DebugContext(ctx, "Job status update",
			"jobID", jobID,
			"status", statusStr,
			"progress", status.Progress.ProgressPercent,
			"executingPeer", status.Progress.ExecutingPeerId,
		)

		// Process new output: separate job status messages from compiler output
		if len(status.Progress.LogOutput) > lastLogLength {
			newOutput := status.Progress.LogOutput[lastLogLength:]

			// Only log non-compiler output through the logger
			// Compiler output (lines starting with [stdout] or [stderr]) goes directly to stderr
			lines := strings.Split(newOutput, "\n")
			for _, line := range lines {
				if line == "" {
					continue
				}

				if strings.HasPrefix(line, "[stdout]") || strings.HasPrefix(line, "[stderr]") {
					// Print compiler output directly to stderr
					fmt.Fprintln(os.Stderr, line)
				} else if line != "" {
					// Log job status messages through the logger
					Log().InfoContext(ctx, line)
				}
			}

			lastLogLength = len(status.Progress.LogOutput)
		}

		// Check terminal status and exit if job is done
		switch status.Progress.Status {
		case v1.JobProgress_JOB_STATUS_COMPLETED,
			v1.JobProgress_JOB_STATUS_FAILED,
			v1.JobProgress_JOB_STATUS_CANCELLED:
			// Stream is done, exit the loop
			goto streamDone
		}
	}

streamDone:

	// Check for stream errors
	if err := stream.Err(); err != nil {
		Log().ErrorContext(ctx, "Stream error while watching job", "error", err)
		return nil, 1, fmt.Errorf("stream error: %w", err)
	}

	if finalStatus == nil {
		return nil, 1, fmt.Errorf("no job status received")
	}

	// Extract result from final status progress (only populated on terminal state)
	result := finalStatus.Progress.Result
	if result == nil {
		// Fallback: build minimal result from progress if not included
		result = buildJobResultFromStatus(finalStatus)
	}

	// Determine exit code based on final status
	exitCode := 0
	switch finalStatus.Progress.Status {
	case v1.JobProgress_JOB_STATUS_COMPLETED:
		Log().DebugContext(ctx, "Job completed successfully",
			"jobID", jobID,
			"logOutputLength", len(finalStatus.Progress.LogOutput),
		)
		exitCode = 0

	case v1.JobProgress_JOB_STATUS_FAILED:
		Log().ErrorContext(ctx, "Job failed",
			"jobID", jobID,
			"logOutputLength", len(finalStatus.Progress.LogOutput),
		)
		exitCode = 1

	case v1.JobProgress_JOB_STATUS_CANCELLED:
		Log().WarnContext(ctx, "Job was cancelled",
			"jobID", jobID,
		)
		exitCode = 1
	}

	// Materialize output files transparently
	// The daemon includes output files in the result with the following signaling:
	//   - For sandboxed runtimes: outputs have content filled (driver must extract/write)
	//   - For local unsandboxed: outputs have empty content (driver validates hash only)
	// The JobDataStager handles both cases transparently via WriteJobDataListToFiles,
	// which checks if files already exist (hash validation) or writes new (and validates).
	if outputDir != "" && result != nil && exitCode == 0 {
		Log().DebugContext(ctx, "Result object details",
			"jobID", result.JobId,
			"resultOutputsCount", len(result.Outputs),
			"resultStatus", result.Status,
		)
		if err := writeJobOutputs(ctx, result, result.Outputs, outputDir); err != nil {
			Log().WarnContext(ctx, "Failed to materialize output files", "error", err, "jobID", jobID)
			// Don't fail the job over output materialization failure - outputs may have been written by runtime
		}
	}

	return result, exitCode, nil
}

// buildJobResultFromStatus constructs a JobResult from a JobStatus response
func buildJobResultFromStatus(status *v1.JobStatus) *v1.JobResult {
	if status == nil {
		return nil
	}

	// Map JobProgress status to JobResult status
	jobResultStatus := v1.JobResult_JOB_STATUS_UNSPECIFIED
	switch status.Progress.Status {
	case v1.JobProgress_JOB_STATUS_COMPLETED:
		jobResultStatus = v1.JobResult_JOB_STATUS_COMPLETED
	case v1.JobProgress_JOB_STATUS_FAILED:
		jobResultStatus = v1.JobResult_JOB_STATUS_FAILED
	case v1.JobProgress_JOB_STATUS_CANCELLED:
		jobResultStatus = v1.JobResult_JOB_STATUS_CANCELLED
	}

	return &v1.JobResult{
		JobId:           status.JobId,
		ExecutingPeerId: status.SubmitterClientId,
		Status:          jobResultStatus,
		CompletedAt:     status.Progress.UpdatedAt,
		LogOutput:       status.Progress.LogOutput,
	}
}

// writeJobOutputs materializes output files using the common JobDataStager implementation.
// This function handles output materialization transparently through JobDataStager.WriteJobDataListToFiles():
//
// For each output file:
//   - If file exists locally: validates hash matches (no re-write). Supports case where
//     daemon or runtime already wrote outputs to the local filesystem.
//   - If file doesn't exist but JobData.Content is filled: writes file to disk and
//     validates hash. Supports future remote execution where daemon returns output
//     content in JobResult.
//   - If file doesn't exist and no content: assumes runtime wrote it directly
//     (sandboxed/remote execution). Just validates hash on final file.
//
// writeJobOutputs materializes output files from job result, then verifies them on disk.
// This handles both sandboxed and unsandboxed runtimes transparently.
//
// This design allows drivers, daemon, and runtimes to work together regardless of
// who actually writes the files. Hash verification ensures correctness in all cases.
func writeJobOutputs(ctx context.Context, result *v1.JobResult, outputData []*v1.JobData, outputDir string) error {
	if result == nil || outputDir == "" {
		return nil
	}

	if len(outputData) == 0 {
		Log().DebugContext(ctx, "No output files to materialize", "jobID", result.JobId)
		return nil
	}

	// Use the common JobDataStager implementation to write files
	stager := staging.NewJobDataStager(outputDir)
	if err := stager.WriteJobDataListToFiles(ctx, outputData, outputDir); err != nil {
		return fmt.Errorf("failed to write output files: %w", err)
	}

	Log().DebugContext(ctx, "Output files materialized from daemon response",
		"jobID", result.JobId,
		"fileCount", len(outputData))

	// Verify materialized outputs are on disk with correct hashes
	if err := stager.VerifyJobDataList(ctx, outputData, outputDir, staging.VerificationModeSaved); err != nil {
		return fmt.Errorf("output verification failed after materialization: %w", err)
	}

	Log().DebugContext(ctx, "Output files verified on disk",
		"jobID", result.JobId,
		"fileCount", len(outputData))

	return nil
}
