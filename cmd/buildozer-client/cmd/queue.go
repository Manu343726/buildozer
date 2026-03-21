package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewQueueCommand creates the 'queue' subcommand
func NewQueueCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "queue",
		Short: "View the job queue",
		Long: `Display the current job queue status:

- Pending jobs: Waiting to be scheduled
- Running jobs: Currently executing
- Completed jobs: Recently finished jobs
- Failed jobs: Jobs that failed to execute

For each job, shows:
- Job ID
- Status (pending/running/completed/failed)
- Time submitted
- Expected completion time
- Assigned executor/peer

Use --standalone to query in-process daemon queue (no separate daemon needed).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			standalone, _ := IsStandaloneMode(cmd)

			if standalone {
				fmt.Println("[STANDALONE] Job Queue (in-process daemon):")
				fmt.Println("============================================")
				fmt.Println("Pending: 0 jobs")
				fmt.Println("Running: 0 jobs")
				fmt.Println("Completed: 0 jobs")
				fmt.Println("Failed: 0 jobs")
				return nil
			}

			// TODO: Connect to daemon and call IntrospectionService.GetJobQueue()
			fmt.Println("Job Queue:")
			fmt.Println("==========")
			fmt.Println("Pending: 0 jobs")
			fmt.Println("Running: 0 jobs")
			fmt.Println("Completed: 0 jobs")
			fmt.Println("Failed: 0 jobs")
			fmt.Println("(Queue stats not yet implemented)")
			return nil
		},
	}
}
