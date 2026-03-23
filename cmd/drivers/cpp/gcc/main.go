package main

import (
	"fmt"
	"os"

	"github.com/Manu343726/buildozer/pkg/drivers"
	gccdriver "github.com/Manu343726/buildozer/pkg/drivers/cpp/gcc"
	"github.com/spf13/cobra"
)

/*
GCC wrapper for Buildozer distributed build system.

Buildozer-specific flags (not passed to gcc):
  --buildozer-log-level <level>    Set driver log level (debug, info, warn, error)
  --buildozer-config <path>         Explicit path to .buildozer config file

All other flags are passed directly to gcc.
Example:
  gcc --buildozer-log-level debug --buildozer-config /etc/buildozer.conf -c main.c -o main.o

GCC 10.2.1 --help reference for valid flags and syntax:
Usage: gcc [options] file...
Options:
  -pass-exit-codes         Exit with highest error code from a phase.
  --help                   Display this information.
  --target-help            Display target specific command line options.
  --help={common|optimizers|params|target|warnings|[^]{joined|separate|undocumented}}[,...].
  (Use '-v --help' to display command line options of sub-processes).
  --version                Display compiler version information.
  -dumpspecs               Display all of the built in spec strings.
  -dumpversion             Display the version of the compiler.
  -dumpmachine             Display the compiler's target processor.
  -print-search-dirs       Display the directories in the compiler's search path.
  -print-libgcc-file-name  Display the name of the compiler's companion library.
  -print-file-name=<lib>   Display the full path to library <lib>.
  -print-prog-name=<prog>  Display the full path to compiler component <prog>.
  -print-multiarch         Display the target's normalized GNU triplet.
  -print-multi-directory   Display the root directory for versions of libgcc.
  -print-multi-lib         Display the mapping between command line options and multiple library search directories.
  -print-multi-os-directory Display the relative path to OS libraries.
  -print-sysroot           Display the target libraries directory.
  -print-sysroot-headers-suffix Display the sysroot suffix used to find headers.
  -Wa,<options>            Pass comma-separated <options> on to the assembler.
  -Wp,<options>            Pass comma-separated <options> on to the preprocessor.
  -Wl,<options>            Pass comma-separated <options> on to the linker.
  -Xassembler <arg>        Pass <arg> on to the assembler.
  -Xpreprocessor <arg>     Pass <arg> on to the preprocessor.
  -Xlinker <arg>           Pass <arg> on to the linker.
  -save-temps              Do not delete intermediate files.
  -save-temps=<arg>        Do not delete intermediate files.
  -no-canonical-prefixes   Do not canonicalize paths when building relative prefixes.
  -pipe                    Use pipes rather than intermediate files.
  -time                    Time the execution of each subprocess.
  -specs=<file>            Override built-in specs with the contents of <file>.
  -std=<standard>          Assume that the input sources are for <standard>.
  --sysroot=<directory>    Use <directory> as the root directory for headers and libraries.
  -B <directory>           Add <directory> to the compiler's search paths.
  -v                       Display the programs invoked by the compiler.
  -###                     Like -v but options quoted and commands not executed.
  -E                       Preprocess only; do not compile, assemble or link.
  -S                       Compile only; do not assemble or link.
  -c                       Compile and assemble, but do not link.
  -o <file>                Place the output into <file>.
  -pie                     Create a dynamically linked position independent executable.
  -shared                  Create a shared library.
  -x <language>            Specify the language of the following input files (c, c++, assembler, none, etc).
Options starting with -g, -f, -m, -O, -W, or --param are automatically passed to sub-processes.
*/

func main() {
	rootCmd := &cobra.Command{
		Use:                "gcc [options] [files...]",
		Short:              "GCC C compiler",
		Long:               "GCC - the GNU Compiler Collection for C. Fully compatible with standard GCC.",
		Version:            "10.2.1",
		Args:               cobra.ArbitraryArgs,
		RunE:               runGcc,
		DisableFlagParsing: true, // Allow all gcc flags without cobra validation
	}

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runGcc(cmd *cobra.Command, args []string) error {
	// Handle --help and -h flags before parsing driver flags
	for _, arg := range args {
		if arg == "--help" || arg == "-h" || arg == "-help" {
			cmd.Help()
			return nil
		}
	}

	// Parse buildozer driver flags and extract tool-specific flags
	gccArgs := drivers.StandardDriverFlags.Parse(args)

	// Validate arguments using shared CLI validator
	cliCfg := &drivers.CLIConfig{
		Name:           "gcc",
		Type:           drivers.GCC,
		SupportsStdlib: false,
	}

	_, err := drivers.ValidateAndParseArgs(gccArgs, cliCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gcc: error: %v\n", err)
		os.Exit(1)
	}

	// Create build context and run driver
	// Get daemon address from flags (already parsed by StandardDriverFlags)
	daemonHost := "localhost"
	daemonPort := 6789
	
	if drivers.DaemonHostPtr != nil && *drivers.DaemonHostPtr != nil {
		daemonHost = **drivers.DaemonHostPtr
	}
	if drivers.DaemonPortPtr != nil && *drivers.DaemonPortPtr != nil {
		daemonPort = **drivers.DaemonPortPtr
	}

	buildCtx := &gccdriver.BuildContext{
		Config:     nil,
		DaemonHost: daemonHost,
		DaemonPort: daemonPort,
		Standalone: false,
		StartDir:   "",
		LogLevel:   *drivers.LogLevelPtr,
		ConfigPath: *drivers.ConfigPathPtr,
	}

	exitCode := gccdriver.RunGcc(cmd.Context(), gccArgs, buildCtx)
	os.Exit(exitCode)
	return nil
}
