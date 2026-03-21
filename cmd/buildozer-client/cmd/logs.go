package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/Manu343726/buildozer/pkg/logging"
	"github.com/spf13/cobra"
)

// NewLogsCommand creates the 'logs' parent command with subcommands
func NewLogsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Manage client logging configuration and view logs",
		Long: `Manage client logging configuration, including viewing logs and dynamically configuring loggers and sinks.

Use different subcommands for different operations:
  logs status              - View current logging configuration
  logs tail                - Tail logs from the running daemon
  logs set-global-level    - Set global logging level
  logs set-logger-level    - Set specific logger level
  logs set-sink-level      - Set specific sink level
  logs enable-file-sink    - Enable logger-specific file sink
  logs disable-file-sink   - Disable logger-specific file sink

The --standalone flag works with all subcommands to manipulate in-process daemon logging.`,
	}

	// Add subcommands
	cmd.AddCommand(newLogsStatusCommand())
	cmd.AddCommand(newLogsTailCommand())
	cmd.AddCommand(newLogsSetGlobalLevelCommand())
	cmd.AddCommand(newLogsSetLoggerLevelCommand())
	cmd.AddCommand(newLogsSetSinkLevelCommand())
	cmd.AddCommand(newLogsEnableFileSinkCommand())
	cmd.AddCommand(newLogsDisableFileSinkCommand())

	return cmd
}

// newLogsStatusCommand shows the current logging configuration
func newLogsStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show current logging configuration status",
		Long:  "Display the current logging configuration including global level, sinks, and loggers.",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Initialize logging if not already done
			if logging.GetRegistry() == nil {
				if err := InitializeLogging(); err != nil {
					return fmt.Errorf("failed to initialize logging: %w", err)
				}
			}
			return showLoggingStatus()
		},
	}
}

// newLogsTailCommand tails logs
func newLogsTailCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "tail",
		Short: "Tail logs from the running daemon",
		Long:  "Stream logs from the running daemon or in-process daemon.",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Initialize logging if not already done
			if logging.GetRegistry() == nil {
				if err := InitializeLogging(); err != nil {
					return fmt.Errorf("failed to initialize logging: %w", err)
				}
			}
			fmt.Println("[LOGGING] Tailing in-process daemon logs...")
			fmt.Println("(Log streaming not yet implemented)")
			return nil
		},
	}
}

// newLogsSetGlobalLevelCommand sets the global logging level
func newLogsSetGlobalLevelCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-global-level <level>",
		Short: "Set global logging level",
		Long:  "Set the global logging level (error, warn, info, debug, trace).",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Initialize logging if not already done
			if logging.GetRegistry() == nil {
				if err := InitializeLogging(); err != nil {
					return fmt.Errorf("failed to initialize logging: %w", err)
				}
			}

			level := logging.ParseLevel(args[0])
			logging.SetGlobalLevel(level)
			fmt.Printf("Set global logging level to: %s\n", args[0])
			return nil
		},
	}
}

// newLogsSetLoggerLevelCommand sets a specific logger's level
func newLogsSetLoggerLevelCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-logger-level <logger-name> <level>",
		Short: "Set specific logger level",
		Long:  "Set the logging level for a specific logger (error, warn, info, debug, trace).",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Initialize logging if not already done
			if logging.GetRegistry() == nil {
				if err := InitializeLogging(); err != nil {
					return fmt.Errorf("failed to initialize logging: %w", err)
				}
			}

			loggerName := args[0]
			levelStr := args[1]

			level := logging.ParseLevel(levelStr)
			if err := logging.SetLoggerLevel(loggerName, level); err != nil {
				return err
			}
			fmt.Printf("Set logger %q level to: %s\n", loggerName, levelStr)
			return nil
		},
	}
}

// newLogsSetSinkLevelCommand sets a specific sink's level
func newLogsSetSinkLevelCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-sink-level <sink-name> <level>",
		Short: "Set specific sink level",
		Long:  "Set the logging level for a specific sink (error, warn, info, debug, trace).",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Initialize logging if not already done
			if logging.GetRegistry() == nil {
				if err := InitializeLogging(); err != nil {
					return fmt.Errorf("failed to initialize logging: %w", err)
				}
			}

			sinkName := args[0]
			levelStr := args[1]

			level := logging.ParseLevel(levelStr)
			if err := logging.SetSinkLevel(sinkName, level); err != nil {
				return err
			}
			fmt.Printf("Set sink %q level to: %s\n", sinkName, levelStr)
			return nil
		},
	}
}

// newLogsEnableFileSinkCommand enables a logger-specific file sink
func newLogsEnableFileSinkCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "enable-file-sink <logger-name> <file-path>",
		Short: "Enable logger-specific file sink",
		Long:  "Enable a rotating file sink for a specific logger.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Initialize logging if not already done
			if logging.GetRegistry() == nil {
				if err := InitializeLogging(); err != nil {
					return fmt.Errorf("failed to initialize logging: %w", err)
				}
			}

			loggerName := args[0]
			filePath := args[1]

			if err := logging.EnableLoggerFileSink(loggerName, filePath, 100, 0); err != nil {
				return err
			}
			fmt.Printf("Enabled file sink for logger %q at: %s\n", loggerName, filePath)
			return nil
		},
	}
}

// newLogsDisableFileSinkCommand disables a logger-specific file sink
func newLogsDisableFileSinkCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "disable-file-sink <logger-name>",
		Short: "Disable logger-specific file sink",
		Long:  "Disable the file sink for a specific logger.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Initialize logging if not already done
			if logging.GetRegistry() == nil {
				if err := InitializeLogging(); err != nil {
					return fmt.Errorf("failed to initialize logging: %w", err)
				}
			}

			loggerName := args[0]

			if err := logging.DisableLoggerFileSink(loggerName); err != nil {
				return err
			}
			fmt.Printf("Disabled file sink for logger %q\n", loggerName)
			return nil
		},
	}
}

// showLoggingStatus displays the current logging configuration
func showLoggingStatus() error {
	registry := logging.GetRegistry()

	fmt.Println("[LOGGING STATUS]")
	fmt.Println("================")

	globalLevel := logging.GetGlobalLevel()
	fmt.Printf("Global Level: %s\n\n", globalLevel)

	// Show sinks
	sinkStatus := registry.GetSinkStatus()
	fmt.Println("Sinks:")
	sinkJSON, _ := json.MarshalIndent(sinkStatus, "  ", "  ")
	fmt.Printf("%s\n\n", string(sinkJSON))

	// Show loggers
	loggerStatus := registry.GetLoggerStatus()
	fmt.Println("Loggers:")
	loggerJSON, _ := json.MarshalIndent(loggerStatus, "  ", "  ")
	fmt.Printf("%s\n", string(loggerJSON))

	return nil
}
