package main

import (
	"fmt"
	"os"

	"github.com/Manu343726/buildozer/pkg/drivers"
	gxxdriver "github.com/Manu343726/buildozer/pkg/drivers/cpp/gxx"
	"github.com/spf13/cobra"
)

/*
G++ wrapper for Buildozer distributed build system.

Buildozer-specific flags (not passed to g++):
  --buildozer-log-level <level>    Set driver log level (debug, info, warn, error)
  --buildozer-config <path>         Explicit path to .buildozer config file

All other flags are passed directly to g++.
Example:
  g++ --buildozer-log-level debug --buildozer-config /etc/buildozer.conf -O2 -c main.cpp -o main.o

G++ 10.2.1 --help reference for valid flags and syntax:
Same as GCC with additional C++ specific options:
  -std=<standard>          C++ standard (c++98, c++03, c++0x, c++11, c++1y, c++14, c++1z, c++17, c++2a, c++20, etc)
  -stdlib=<lib>            C++ standard library to use (libstdc++, libc++)
Options starting with -g, -f, -m, -O, -W, or --param are automatically passed to sub-processes.
Full compatibility with GCC flags plus C++ extensions.
*/

func main() {
	rootCmd := &cobra.Command{
		Use:                "g++ [options] [files...]",
		Short:              "G++ C++ compiler",
		Long:               "G++ - the GNU Compiler Collection for C++. Fully compatible with standard G++.",
		Version:            "10.2.1",
		Args:               cobra.ArbitraryArgs,
		RunE:               runGxx,
		DisableFlagParsing: true, // Allow all g++ flags without cobra validation
	}

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runGxx(cmd *cobra.Command, args []string) error {
	// Handle --help and -h flags before parsing driver flags
	for _, arg := range args {
		if arg == "--help" || arg == "-h" || arg == "-help" {
			cmd.Help()
			return nil
		}
	}

	// Parse buildozer driver flags and extract tool-specific flags
	gxxArgs := drivers.StandardDriverFlags.Parse(args)

	// Validate arguments using shared CLI validator
	cliCfg := &drivers.CLIConfig{
		Name:           "g++",
		Type:           drivers.GXX,
		SupportsStdlib: true,
	}

	_, err := drivers.ValidateAndParseArgs(gxxArgs, cliCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "g++: error: %v\n", err)
		os.Exit(1)
	}

	// Create build context and run driver
	// Build daemon address from host and port flags
	daemonHost := "localhost"
	daemonPort := 6789
	
	if drivers.DaemonHostPtr != nil && *drivers.DaemonHostPtr != nil {
		daemonHost = **drivers.DaemonHostPtr
	}
	if drivers.DaemonPortPtr != nil && *drivers.DaemonPortPtr != nil {
		daemonPort = **drivers.DaemonPortPtr
	}
	
	daemonAddr := fmt.Sprintf("%s:%d", daemonHost, daemonPort)

	buildCtx := &gxxdriver.BuildContext{
		Config:     nil,
		DaemonAddr: daemonAddr,
		Standalone: false,
		StartDir:   "",
		LogLevel:   *drivers.LogLevelPtr,
		ConfigPath: *drivers.ConfigPathPtr,
	}

	exitCode := gxxdriver.RunGxx(cmd.Context(), gxxArgs, buildCtx)
	os.Exit(exitCode)
	return nil
}
