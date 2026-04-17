package cmd

import (
	"github.com/Manu343726/buildozer/pkg/cli"
	pkgconfig "github.com/Manu343726/buildozer/pkg/config"
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

For each queued job, shows:
- Job ID
- Current status
- Time in queue
- Queue position/priority

Use --standalone to query in-process daemon queue (no separate daemon needed).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			commands, err := cli.NewQueueCommands(pkgconfig.Get())
			if err != nil {
				return err
			}
			return commands.Show()
		},
	}
}
