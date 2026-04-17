package cli

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"connectrpc.com/connect"
	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	"github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1/protov1connect"
	"github.com/Manu343726/buildozer/pkg/config"
	"github.com/Manu343726/buildozer/pkg/daemon"
	"github.com/Manu343726/buildozer/pkg/logging"
)

// QueueCommands provides command-level implementations for queue CLI operations.
type QueueCommands struct {
	*logging.Logger // Embedded logger for hierarchical logging

	cfg *config.Config
}

// NewQueueCommands creates a new QueueCommands handler.
func NewQueueCommands(cfg *config.Config) (*QueueCommands, error) {
	return &QueueCommands{
		Logger: Log().Child("QueueCommands"),
		cfg:    cfg,
	}, nil
}

// Show displays the job queue status
func (qc *QueueCommands) Show() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create gRPC client
	daemonURL := daemon.RpcURL(qc.cfg.Daemon.Host, qc.cfg.Daemon.Port)
	client := protov1connect.NewIntrospectionServiceClient(
		&http.Client{},
		daemonURL,
	)

	// Call GetJobQueue with CLI requester identification
	resp, err := client.GetJobQueue(ctx, connect.NewRequest(&v1.GetJobQueueRequest{
		RequesterInfo: &v1.RequesterInfo{
			RequesterId:   "cli",
			RequesterType: "cli",
		},
	}))
	if err != nil {
		return fmt.Errorf("failed to query job queue: %w", err)
	}

	// Display queue status
	queueResp := resp.Msg
	fmt.Println("Job Queue Status:")
	fmt.Println("================")
	fmt.Printf("Total Queue Size: %d\n", queueResp.QueueSize)
	fmt.Printf("Running Jobs: %d\n", queueResp.RunningJobsCount)
	fmt.Printf("Pending Jobs: %d\n", len(queueResp.QueuedJobs))

	if len(queueResp.QueuedJobs) == 0 {
		fmt.Println("\nNo jobs in queue")
		return nil
	}

	fmt.Println("\nQueued Jobs:")
	fmt.Println("-----------")
	for i, job := range queueResp.QueuedJobs {
		fmt.Printf("\n%d. Job ID: %s\n", i+1, job.JobId)

		// Display progress if available
		if job.Progress != nil {
			statusStr := formatJobStatus(job.Progress.Status)
			fmt.Printf("   Status: %s\n", statusStr)

			if job.Progress.ProgressPercent != nil {
				fmt.Printf("   Progress: %d%%\n", job.Progress.ProgressPercent.Value)
			}

			if job.Progress.ExecutingPeerId != "" {
				fmt.Printf("   Executing on: %s\n", job.Progress.ExecutingPeerId)
			}
		}

		// Display time in queue
		if job.TimeInQueue != nil {
			fmt.Printf("   Time in Queue: %v\n", job.TimeInQueue)
		}

		// Display queue position
		fmt.Printf("   Queue Position: %d\n", job.QueuePosition)
	}

	return nil
}

// formatJobStatus converts JobProgress_JobStatus enum to human-readable string
func formatJobStatus(status v1.JobProgress_JobStatus) string {
	switch status {
	case v1.JobProgress_JOB_STATUS_UNSPECIFIED:
		return "Unspecified"
	case v1.JobProgress_JOB_STATUS_READY:
		return "Ready"
	case v1.JobProgress_JOB_STATUS_SCHEDULED:
		return "Scheduled"
	case v1.JobProgress_JOB_STATUS_INPUT_TRANSFER:
		return "Input Transfer"
	case v1.JobProgress_JOB_STATUS_RUNNING:
		return "Running"
	case v1.JobProgress_JOB_STATUS_COMPLETED:
		return "Completed"
	case v1.JobProgress_JOB_STATUS_OUTPUT_TRANSFER:
		return "Output Transfer"
	case v1.JobProgress_JOB_STATUS_FAILED:
		return "Failed"
	case v1.JobProgress_JOB_STATUS_CANCELLED:
		return "Cancelled"
	case v1.JobProgress_JOB_STATUS_QUEUED:
		return "Queued"
	default:
		return fmt.Sprintf("Unknown (%d)", status)
	}
}
