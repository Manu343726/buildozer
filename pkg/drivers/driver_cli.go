// Package drivers provides utilities for driver command execution and runtime resolution.
package drivers

import (
	"fmt"
	"os"

	"github.com/Manu343726/buildozer/pkg/driver"
	"github.com/spf13/cobra"
)

// CommonDriverConfig contains configuration common to all drivers.
// This is extracted from parsed flags and used to populate the DriverConfig.
type CommonDriverConfig struct {
	DaemonHost     string
	DaemonPort     int
	Standalone     bool
	LogLevel       string
	ConfigPath     string
	InitialRuntime string
}

// ExtractCommonConfig extracts common driver configuration from already-parsed
// buildozer flags. This must be called AFTER StandardDriverFlags.Parse(args).
func ExtractCommonConfig() *CommonDriverConfig {
	daemonHost := "localhost"
	daemonPort := 6789

	if DaemonHostPtr != nil && *DaemonHostPtr != nil {
		daemonHost = **DaemonHostPtr
	}
	if DaemonPortPtr != nil && *DaemonPortPtr != nil {
		daemonPort = **DaemonPortPtr
	}

	return &CommonDriverConfig{
		DaemonHost:     daemonHost,
		DaemonPort:     daemonPort,
		Standalone:     *StandalonePtr,
		LogLevel:       *LogLevelPtr,
		ConfigPath:     *ConfigPathPtr,
		InitialRuntime: *RuntimePtr,
	}
}

// ExecuteDriver is the main entry point for all driver CLIs.
// It sets up a cobra command using the driver's metadata and delegates
// to the generic RunDriver orchestration function.
func ExecuteDriver(d driver.Driver) {
	rootCmd := &cobra.Command{
		Use:     fmt.Sprintf("%s [options] [files...]", d.Name()),
		Short:   d.Short(),
		Long:    d.Long(),
		Version: d.Version(),
		Args:    cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDriver(cmd, args, d)
		},
		DisableFlagParsing: true,
	}

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// runDriver is the cobra RunE handler that delegates to the driver implementation.
func runDriver(cmd *cobra.Command, args []string, d driver.Driver) error {
	// Handle --help and -h flags before parsing driver flags
	for _, arg := range args {
		if arg == "--help" || arg == "-h" || arg == "-help" {
			cmd.Help()
			return nil
		}
	}

	// Parse buildozer driver flags and extract tool-specific flags
	parsedArgs := StandardDriverFlags.Parse(args)

	// Extract generic driver configuration
	commonCfg := ExtractCommonConfig()

	// Validate arguments using driver-specific validator
	if err := d.ValidateArgs(parsedArgs); err != nil {
		fmt.Fprintf(os.Stderr, "%s %v\n", d.ErrorPrefix(), err)
		os.Exit(1)
	}

	// Handle --version flag
	if isVersionFlag(parsedArgs) {
		fmt.Println(d.Version())
		return nil
	}

	// Handle --buildozer-list-runtimes flag
	if *ListRuntimesPtr {
		exitCode := ListCompatibleRuntimes(cmd.Context(), d, &DriverConfig{
			DaemonHost: commonCfg.DaemonHost,
			DaemonPort: commonCfg.DaemonPort,
			LogLevel:   commonCfg.LogLevel,
		})
		os.Exit(exitCode)
		return nil
	}

	// Call the generic driver orchestrator
	driverConfig := &DriverConfig{
		Standalone:     commonCfg.Standalone,
		DaemonHost:     commonCfg.DaemonHost,
		DaemonPort:     commonCfg.DaemonPort,
		LogLevel:       commonCfg.LogLevel,
		StartDir:       "",
		ConfigPath:     commonCfg.ConfigPath,
		InitialRuntime: commonCfg.InitialRuntime,
	}

	exitCode := RunDriver(cmd.Context(), d, parsedArgs, driverConfig)
	os.Exit(exitCode)
	return nil
}
