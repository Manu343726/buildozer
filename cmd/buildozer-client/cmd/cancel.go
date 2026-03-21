package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewCancelCommand creates the 'cancel' subcommand
func NewCancelCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cancel JOB_ID",
		Short: "Cancel a running or pending job",
		Long: `Cancel a job by ID. The job can be:
- Pending: Removed from queue immediately
- Running: Execution stopped, job marked as cancelled
- Completed/Failed: No effect (job already finished)

If the job has dependent jobs, the entire dependency DAG is cancelled.

Example:
  buildozer-client cancel job-12345

Use --standalone to cancel jobs in in-process daemon (no separate daemon needed).`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			standalone, _ := IsStandaloneMode(cmd)
			jobID := args[0]

			if standalone {
				fmt.Printf("[STANDALONE] Cancelling job: %s (in-process daemon)\n", jobID)
				return nil
			}

			// TODO: Connect to daemon and call JobService.CancelJob(jobID)
			fmt.Printf("Cancelling job: %s\n", jobID)
			fmt.Println("(Job cancellation not yet implemented)")
			return nil
		},
	}

	return cmd
}
