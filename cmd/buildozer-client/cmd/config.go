package cmd

import (
"github.com/spf13/cobra"
)

// NewConfigCommand creates the 'config' subcommand
func NewConfigCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Show effective configuration",
		Long: `Display the effective configuration after merging all sources:
1. CLI flags (highest priority)
2. Environment variables (BUILDOZER_*)
3. Configuration file (~/.config/buildozer/config.yaml)
4. Hardcoded defaults (lowest priority)

This helps verify that configuration is being loaded correctly from all sources.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			PrintConfigSummary()
			return nil
		},
	}
}
