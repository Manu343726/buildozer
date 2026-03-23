package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	"github.com/Manu343726/buildozer/internal/logger"
	"github.com/Manu343726/buildozer/pkg/toolchain"
)

var log = logger.NewComponentLogger("gxx-driver")

func main() {
	ctx := context.Background()
	parsed := ParseCommandLine(os.Args[1:])

	if len(parsed.CompilerFlags) > 0 && parsed.CompilerFlags[0] == "--version" {
		fmt.Println("g++ version 11.2.0 (Buildozer distributed compiler)")
		os.Exit(0)
	}

	if len(parsed.SourceFiles) == 0 && len(parsed.ObjectFiles) == 0 {
		log.ErrorCtx(ctx, "no input files specified")
		os.Exit(1)
	}

	// Initialize toolchain registry to detect available compilers
	registry := toolchain.Global()
	if err := registry.Initialize(ctx); err != nil {
		log.WarnCtx(ctx, "failed to detect toolchains", "error", err)
		// Continue anyway - we'll create the job assuming G++ is available
	} else {
		// Verify G++ is available on this system
		gxx := registry.GetGxx(ctx)
		if gxx == nil {
			log.ErrorCtx(ctx, "g++ not found on this system")
			os.Exit(1)
		}
		log.InfoCtx(ctx, "using detected G++ toolchain", "version", gxx.CompilerVersion, "path", gxx.CompilerPath)
	}

	job, err := createJob(parsed)
	if err != nil {
		log.ErrorCtx(ctx, "failed to create job", "error", err)
		os.Exit(1)
	}

	result, err := executeJob(ctx, job)
	if err != nil {
		log.ErrorCtx(ctx, "failed to execute job", "error", err)
		os.Exit(1)
	}

	if len(result) > 0 {
		os.Stdout.Write(result)
	}

	os.Exit(0)
}

func createJob(parsed *ParsedArgs) (*v1.Job, error) {
	cppToolchain := &v1.CppToolchain{
		Language: v1.CppLanguage_CPP_LANGUAGE_CPP,
		Compiler: v1.CppCompiler_CPP_COMPILER_GCC,
	}

	runtime := &v1.Runtime{
		Toolchain: &v1.Runtime_Cpp{Cpp: cppToolchain},
	}

	outputFile := parsed.OutputFile
	if outputFile == "" {
		if parsed.Mode == ModeCompileOnly && len(parsed.SourceFiles) == 1 {
			outputFile = stripExtension(parsed.SourceFiles[0]) + ".o"
		} else if parsed.Mode == ModeCompileOnly && len(parsed.SourceFiles) > 1 {
			log.Error("multiple source files with -c requires -o")
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
	log.InfoCtx(ctx, "executing job", "jobID", job.Id)
	// TODO: Implement gRPC submission to buildozer-client daemon
	return []byte(""), nil
}
