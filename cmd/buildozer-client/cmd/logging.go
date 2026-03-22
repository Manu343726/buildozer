package cmd

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/Manu343726/buildozer/pkg/cli"
	pkgconfig "github.com/Manu343726/buildozer/pkg/config"
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
  logs add-sink            - Add a new stdout/stderr sink
  logs remove-sink         - Remove a sink
  logs add-logger          - Add a new logger with specified sinks
  logs remove-logger       - Remove a logger
  logs attach-sink         - Attach a sink to an existing logger
  logs detach-sink         - Remove a sink from an existing logger

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
	cmd.AddCommand(newLogsAddSinkCommand())
	cmd.AddCommand(newLogsRemoveSinkCommand())
	cmd.AddCommand(newLogsAddLoggerCommand())
	cmd.AddCommand(newLogsRemoveLoggerCommand())
	cmd.AddCommand(newLogsAttachSinkCommand())
	cmd.AddCommand(newLogsDetachSinkCommand())

	return cmd
}

// newLogsStatusCommand shows the current logging configuration
func newLogsStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show current logging configuration status",
		Long:  "Display the current logging configuration including global level, sinks, and loggers.",
		RunE: func(cmd *cobra.Command, args []string) error {
			commands, err := cli.NewLoggingCommands(pkgconfig.Get())
			if err != nil {
				return err
			}
			return commands.Status()
		},
	}
}

// newLogsTailCommand tails logs
func newLogsTailCommand() *cobra.Command {
	var levels string
	var loggerFilter string
	var historyLines int

	cmd := &cobra.Command{
		Use:   "tail",
		Short: "Tail logs from the running daemon",
		Long:  "Stream logs from the running daemon or in-process daemon with optional filtering.",
		RunE: func(cmd *cobra.Command, args []string) error {
			commands, err := cli.NewLoggingCommands(pkgconfig.Get())
			if err != nil {
				return err
			}
			// Parse log levels from comma-separated string
			var logLevels []slog.Level
			if levels != "" {
				logLevels = cli.ParseLogLevels(levels)
			}
			return commands.Tail(logLevels, loggerFilter, historyLines)
		},
	}

	cmd.Flags().StringVar(&levels, "levels", "", "Comma-separated log levels to filter (error,warn,info,debug,trace)")
	cmd.Flags().StringVar(&loggerFilter, "logger", "", "Filter logs by logger name prefix")
	cmd.Flags().IntVar(&historyLines, "history", 0, "Number of historical log lines to show before streaming (0 = no history)")

	return cmd
}

// newLogsSetGlobalLevelCommand sets the global logging level
func newLogsSetGlobalLevelCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-global-level <level>",
		Short: "Set global logging level",
		Long:  "Set the global logging level (error, warn, info, debug, trace).",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			commands, err := cli.NewLoggingCommands(pkgconfig.Get())
			if err != nil {
				return err
			}
			return commands.SetGlobalLevel(args[0])
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
			commands, err := cli.NewLoggingCommands(pkgconfig.Get())
			if err != nil {
				return err
			}
			return commands.SetLoggerLevel(args[0], args[1])
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
			commands, err := cli.NewLoggingCommands(pkgconfig.Get())
			if err != nil {
				return err
			}
			return commands.SetSinkLevel(args[0], args[1])
		},
	}
}

// newLogsEnableFileSinkCommand enables a logger-specific file sink
func newLogsEnableFileSinkCommand() *cobra.Command {
	var maxSizeMB int
	var maxBackups int
	var maxAgeDays int

	cmd := &cobra.Command{
		Use:   "enable-file-sink <logger-name> <file-path>",
		Short: "Enable logger-specific file sink",
		Long:  "Enable a rotating file sink for a specific logger.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			commands, err := cli.NewLoggingCommands(pkgconfig.Get())
			if err != nil {
				return err
			}
			return commands.EnableFileSink(args[0], args[1], maxSizeMB, maxBackups, maxAgeDays)
		},
	}

	cmd.Flags().IntVar(&maxSizeMB, "max-size", 100, "Maximum size of each log file in MB")
	cmd.Flags().IntVar(&maxBackups, "max-backups", 10, "Maximum number of old log files to keep")
	cmd.Flags().IntVar(&maxAgeDays, "max-age", 0, "Maximum age of log files in days (0 = no limit)")

	return cmd
}

// newLogsDisableFileSinkCommand disables a logger-specific file sink
func newLogsDisableFileSinkCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "disable-file-sink <logger-name>",
		Short: "Disable logger-specific file sink",
		Long:  "Disable the file sink for a specific logger.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			commands, err := cli.NewLoggingCommands(pkgconfig.Get())
			if err != nil {
				return err
			}
			return commands.DisableFileSink(args[0])
		},
	}
}

// newLogsAddSinkCommand adds a new sink
func newLogsAddSinkCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "add-sink <type>",
		Short: "Add a new stdout/stderr sink",
		Long:  "Add a new sink of type: stdout or stderr. The sink name is implicitly set to match the type.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			commands, err := cli.NewLoggingCommands(pkgconfig.Get())
			if err != nil {
				return err
			}
			sinkType := args[0]
			// Use the type as the implicit sink name for stdout/stderr
			sinkName := sinkType
			return commands.AddSink(sinkName, sinkType, slog.LevelInfo)
		},
	}
}

// newLogsRemoveSinkCommand removes a sink
func newLogsRemoveSinkCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "remove-sink <sink-name>",
		Short: "Remove a sink",
		Long:  "Remove a sink from the registry and all loggers.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			commands, err := cli.NewLoggingCommands(pkgconfig.Get())
			if err != nil {
				return err
			}
			return commands.RemoveSink(args[0])
		},
	}
}

// newLogsAddLoggerCommand adds a new logger
func newLogsAddLoggerCommand() *cobra.Command {
	var sinks string
	var level string

	cmd := &cobra.Command{
		Use:   "add-logger <logger-name>",
		Short: "Add a new logger with specified sinks",
		Long:  "Add a new logger with specified sinks and logging level.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			commands, err := cli.NewLoggingCommands(pkgconfig.Get())
			if err != nil {
				return err
			}

			// Parse level
			logLevel, err := parseLevel(level)
			if err != nil {
				return err
			}

			// Parse sinks
			var sinkList []string
			if sinks != "" {
				sinkList = strings.Split(sinks, ",")
				for i := range sinkList {
					sinkList[i] = strings.TrimSpace(sinkList[i])
				}
			}

			return commands.AddLogger(args[0], logLevel, sinkList)
		},
	}

	cmd.Flags().StringVar(&sinks, "sinks", "", "Comma-separated list of sink names to attach to this logger")
	cmd.Flags().StringVar(&level, "level", "info", "Logging level (error, warn, info, debug, trace)")

	return cmd
}

// newLogsRemoveLoggerCommand removes a logger
func newLogsRemoveLoggerCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "remove-logger <logger-name>",
		Short: "Remove a logger",
		Long:  "Remove a logger configuration from the registry.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			commands, err := cli.NewLoggingCommands(pkgconfig.Get())
			if err != nil {
				return err
			}
			return commands.RemoveLogger(args[0])
		},
	}
}

// newLogsAttachSinkCommand attaches a sink to an existing logger
func newLogsAttachSinkCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "attach-sink <logger-name> <sink-name>",
		Short: "Attach a sink to an existing logger",
		Long:  "Attach an existing sink to a logger.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			commands, err := cli.NewLoggingCommands(pkgconfig.Get())
			if err != nil {
				return err
			}
			return commands.AttachSink(args[0], args[1])
		},
	}
}

// newLogsDetachSinkCommand removes a sink from a logger
func newLogsDetachSinkCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "detach-sink <logger-name> <sink-name>",
		Short: "Remove a sink from a logger",
		Long:  "Detach a sink from a logger.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			commands, err := cli.NewLoggingCommands(pkgconfig.Get())
			if err != nil {
				return err
			}
			return commands.DetachSink(args[0], args[1])
		},
	}
}

// parseLevel is a helper to convert string to slog.Level
func parseLevel(levelStr string) (slog.Level, error) {
	switch strings.ToLower(levelStr) {
	case "error":
		return slog.LevelError, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "trace":
		return slog.Level(-8), nil
	default:
		return slog.LevelInfo, fmt.Errorf("invalid log level: %s", levelStr)
	}
}
