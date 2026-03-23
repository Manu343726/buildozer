package gxx

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	"github.com/Manu343726/buildozer/pkg/config"
	"github.com/Manu343726/buildozer/pkg/drivers"
	gcc_common "github.com/Manu343726/buildozer/pkg/drivers/cpp/gcc_common"
)

// BuildContext is an alias to the shared gcc_common BuildContext
type BuildContext = gcc_common.BuildContext

// ParsedArgs is an alias to the shared gcc_common ParsedArgs
type ParsedArgs = gcc_common.ParsedArgs

// CompileMode is an alias to the shared gcc_common CompileMode
type CompileMode = gcc_common.CompileMode

// ModeCompileOnly is an alias to the shared gcc_common ModeCompileOnly
const ModeCompileOnly = gcc_common.ModeCompileOnly

// RunGxx executes the G++ driver with the given arguments and build context.
// Returns exit code (0 for success, non-zero for failure).
func RunGxx(ctx context.Context, args []string, buildCtx *BuildContext) int {
	parsed := gcc_common.ParseCommandLine(args)

	// Set log level if specified
	if buildCtx.LogLevel != "" {
		Log().Debug("Setting log level", "level", buildCtx.LogLevel)
	}

	// Handle --version flag
	if len(parsed.CompilerFlags) > 0 && parsed.CompilerFlags[0] == "--version" {
		fmt.Println("g++ version 11.2.0 (Buildozer distributed compiler)")
		return 0
	}

	// Check for input files
	if len(parsed.SourceFiles) == 0 && len(parsed.ObjectFiles) == 0 {
		fmt.Fprintln(os.Stderr, "error: no input files specified")
		return 1
	}

	// Load configuration from .buildozer file (upward search) or use provided config
	var cfg *config.Config
	var configFile string
	var err error

	if buildCtx.Config != nil {
		cfg = buildCtx.Config
	} else if buildCtx.ConfigPath != "" {
		// Load from explicit config path
		cfg, configFile, err = config.LoadDriverConfig(buildCtx.ConfigPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to load config file from %s: %v\n", buildCtx.ConfigPath, err)
			defaultCfg := config.DefaultConfig()
			cfg = &defaultCfg
		} else if configFile != "" {
			fmt.Printf("loaded config file: %s\n", configFile)
		}
	} else {
		// Search for config starting from current directory
		cfg, configFile, err = config.LoadDriverConfig(buildCtx.StartDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to load config file: %v\n", err)
			defaultCfg := config.DefaultConfig()
			cfg = &defaultCfg
		}
		if configFile != "" {
			fmt.Printf("loaded config file: %s\n", configFile)
		}
	}

	// Resolve the requested toolchain based on config and command-line arguments
	toolchainResolution := drivers.ResolveGxxToolchain(ctx, &cfg.Drivers.Gxx, args)
	fmt.Printf("resolved toolchain: %s\n", toolchainResolution.String())

	// Query the daemon for available runtimes
	daemonAddr := buildCtx.DaemonAddr
	if daemonAddr == "" {
		daemonAddr = "localhost:6789"
	}

	daemonClient, err := drivers.NewDaemonClient(ctx, daemonAddr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to connect to daemon: %v\n", err)
		// Continue anyway - user may not have daemon running
	} else {
		runtimes, err := daemonClient.ListRuntimes(ctx, true)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to query daemon for runtimes: %v\n", err)
		} else {
			// Find a matching runtime
			matchingRuntime, _ := daemonClient.FindMatchingRuntime(ctx, toolchainResolution, runtimes)
			if matchingRuntime == nil {
				// No matching runtime available
				fmt.Fprint(os.Stderr, drivers.FormatUnavailableRuntimeWarning("g++", toolchainResolution, runtimes))
				return 1
			}

			fmt.Printf("found matching runtime: %s\n", matchingRuntime.GetId())
		}
	}

	// Create the job
	job, err := createJob(parsed, toolchainResolution)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to create job: %v\n", err)
		return 1
	}

	// Execute the job
	result, err := executeJob(ctx, job)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to execute job: %v\n", err)
		return 1
	}

	if len(result) > 0 {
		os.Stdout.Write(result)
	}

	return 0
}

// createJob creates a protobuf Job from parsed arguments and resolved toolchain
func createJob(parsed *ParsedArgs, resolution *drivers.ToolchainResolution) (*v1.Job, error) {
	cppToolchain := drivers.CppToolchainForResolution(resolution, v1.CppLanguage_CPP_LANGUAGE_CPP)

	runtime := &v1.Runtime{
		Toolchain: &v1.Runtime_Cpp{Cpp: cppToolchain},
	}

	outputFile := parsed.OutputFile
	if outputFile == "" {
		if parsed.Mode == ModeCompileOnly && len(parsed.SourceFiles) == 1 {
			outputFile = stripExtension(parsed.SourceFiles[0]) + ".o"
		} else if parsed.Mode == ModeCompileOnly && len(parsed.SourceFiles) > 1 {
			Log().Error("multiple source files with -c requires -o")
			return nil, fmt.Errorf("multiple source files with -c requires -o")
		} else {
			outputFile = "a.out"
		}
	}

	job := &v1.Job{
		Id:      generateJobID(),
		Runtime: runtime,
		Timeout: &v1.TimeDuration{Count: 300, Unit: v1.TimeUnit_TIME_UNIT_SECOND},
	}

	if parsed.Mode == ModeCompileOnly || (len(parsed.ObjectFiles) == 0 && len(parsed.SourceFiles) > 0 && !parsed.IsSharedLibrary) {
		job.JobSpec = &v1.Job_CppCompile{
			CppCompile: &v1.CppCompileJob{
				SourceFiles:  parsed.SourceFiles,
				CompilerArgs: parsed.CompilerFlags,
				IncludeDirs:  parsed.IncludeDirs,
				Defines:      parsed.Defines,
				OutputFile:   outputFile,
			},
		}
	} else if parsed.IsSharedLibrary && len(parsed.SourceFiles) > 0 && len(parsed.ObjectFiles) == 0 {
		// Shared library from sources: compile and link
		// Add -shared flag to compiler args for linking phase
		compilerArgs := append(parsed.CompilerFlags, parsed.LinkerFlags...)
		job.JobSpec = &v1.Job_CppCompile{
			CppCompile: &v1.CppCompileJob{
				SourceFiles:  parsed.SourceFiles,
				CompilerArgs: compilerArgs,
				IncludeDirs:  parsed.IncludeDirs,
				Defines:      parsed.Defines,
				OutputFile:   outputFile,
			},
		}
	} else if len(parsed.ObjectFiles) > 0 {
		job.JobSpec = &v1.Job_CppLink{
			CppLink: &v1.CppLinkJob{
				ObjectFiles:     parsed.ObjectFiles,
				Libraries:       parsed.Libraries,
				LibraryDirs:     parsed.LibraryDirs,
				LinkerArgs:      parsed.LinkerFlags,
				OutputFile:      outputFile,
				IsSharedLibrary: parsed.IsSharedLibrary,
			},
		}
	} else {
		return nil, fmt.Errorf("unable to determine job type")
	}

	return job, nil
}

func stripExtension(path string) string {
	return path[:len(path)-len(filepath.Ext(path))]
}

func generateJobID() string {
	return fmt.Sprintf("gxx-%d", os.Getpid())
}

func executeJob(ctx context.Context, job *v1.Job) ([]byte, error) {
	fmt.Printf("executing job: %s\n", job.Id)
	// TODO: Implement gRPC submission to buildozer-client daemon
	return []byte(""), nil
}
