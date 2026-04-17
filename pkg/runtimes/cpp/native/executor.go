package native

import (
	"context"

	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	"github.com/Manu343726/buildozer/pkg/logging"
	"github.com/Manu343726/buildozer/pkg/runtime"
	"github.com/Manu343726/buildozer/pkg/staging"
)

// Executor manages the execution of C/C++ compilation and linking operations.
// It encapsulates compiler subprocess management, working directory setup, and output capture.
type Executor struct {
	*logging.Logger
	baseExecutor runtime.Executor

	// compilerPath is the filesystem path to the compiler executable.
	compilerPath string
	// archiverPath is the filesystem path to the archiver executable (e.g., ar).
	archiverPath string
	// workDir is the working directory for compiler invocations (where temp files are stored).
	workDir string
}

// NewExecutor creates and returns a new Executor for a specific compiler.
// Parameters:
// - compilerPath: The filesystem path to the compiler executable (e.g., "/usr/bin/gcc")
// - archiverPath: The filesystem path to the archiver executable (e.g., "/usr/bin/ar")
// - workDir: The directory for temporary files and compiler working directory
func NewExecutor(compilerPath, archiverPath, workDir string) *Executor {
	return &Executor{
		Logger:       Log(),
		baseExecutor: runtime.NewExecutor(workDir, Log()),
		compilerPath: compilerPath,
		archiverPath: archiverPath,
		workDir:      workDir,
	}
}

// prepareExecutionResult creates an ExecutionResult with collected output files.
// Parameters:
// - ctx: context for file operations
// - exitCode: the execution exit code (0 = success)
// - stdout: standard output from execution
// - stderr: standard error output from execution
// - outputPaths: list of output file paths to collect (only if exitCode == 0)
// Returns an ExecutionResult with populated Output JobData array (empty if exitCode != 0)
// Returns an error if any output file fails to be converted to JobData (when exitCode == 0)
func (e *Executor) prepareExecutionResult(ctx context.Context, exitCode int, stdout, stderr []byte, outputPaths []string) (*runtime.ExecutionResult, error) {
	var outputList []*v1.JobData

	// Only collect outputs if execution was successful
	if exitCode == 0 && len(outputPaths) > 0 {
		stager := staging.NewJobDataStager(e.workDir)
		var err error
		outputList, err = stager.CreateJobDataListForFiles(ctx, outputPaths, staging.JobDataModeReference)
		if err != nil {
			return nil, e.Errorf("failed to create JobData for output files: %w", err)
		}
	}

	return &runtime.ExecutionResult{
		ExitCode:    exitCode,
		Stdout:      stdout,
		Stderr:      stderr,
		Output:      outputList,
		WorkDir:     e.workDir,
		ExecutionID: "", // Set by caller if needed
	}, nil
}

// ExecuteCompileJob executes a C/C++ compilation job and returns an ExecutionResult.
// It invokes the compiler with the job specifications and captures stdout/stderr.
// Output files are collected and included in the result.
// If progressCallback is provided, progress updates are reported in real-time.
// Returns:
// - *runtime.ExecutionResult with stdout, stderr, exitCode, and output JobData
// - error: An error if the job was nil or other fatal issues occurred
func (e *Executor) ExecuteCompileJob(ctx context.Context, job *CompileJob, progressCallback runtime.ProgressCallback) (*runtime.ExecutionResult, error) {
	if job == nil {
		return nil, e.Errorf("compile job is nil")
	}

	// Validate job has source files
	if len(job.SourceFiles) == 0 {
		return nil, e.Errorf("compile job has no source files")
	}

	// Validate job has output file
	if job.OutputFile == "" {
		return nil, e.Errorf("compile job has no output file")
	}

	// Build compiler arguments: start with -c (compile only, no linking)
	args := []string{"-c"}
	args = append(args, job.SourceFiles...)

	// Add include directories
	for _, dir := range job.IncludeDirs {
		args = append(args, "-I"+dir)
	}

	// Add preprocessor defines
	for key, val := range job.Defines {
		args = append(args, "-D"+key+"="+val)
	}

	// Add custom compiler flags
	args = append(args, job.CompilerFlags...)

	// Specify output file (if provided)
	if job.OutputFile != "" {
		args = append(args, "-o", job.OutputFile)
	}

	// Debug log the compile command that will be executed
	e.Debug("Compiling sources with full command",
		"compiler", e.compilerPath,
		"argCount", len(args),
		"workDir", e.workDir,
	)
	e.Debug("Compile command arguments", "args", args)

	// Execute the compiler
	stdout, stderr, exitCode, err := e.baseExecutor.ExecuteCommand(ctx, e.compilerPath, args, progressCallback)
	if err != nil {
		return nil, err
	}

	// Prepare result with collected output files
	outputPaths := []string{job.OutputFile}
	return e.prepareExecutionResult(ctx, exitCode, stdout, stderr, outputPaths)
}

// ExecuteLinkJob executes a C/C++ linking job and returns an ExecutionResult.
// It invokes the compiler in linker mode to produce an executable or shared library.
// Output files are collected and included in the result.
// For shared libraries, the -shared flag is prepended before object files.
// If progressCallback is provided, progress updates are reported in real-time.
// Returns:
// - *runtime.ExecutionResult with stdout, stderr, exitCode, and output JobData
// - error: An error if the job was nil or other fatal issues occurred
func (e *Executor) ExecuteLinkJob(ctx context.Context, job *LinkJob, progressCallback runtime.ProgressCallback) (*runtime.ExecutionResult, error) {
	if job == nil {
		return nil, e.Errorf("link job is nil")
	}

	// Validate job has object files
	if len(job.ObjectFiles) == 0 {
		return nil, e.Errorf("link job has no object files")
	}

	// Validate job has output file
	if job.OutputFile == "" {
		return nil, e.Errorf("link job has no output file")
	}

	args := []string{}

	// Add -shared flag for shared libraries before object files
	if job.SharedLibrary {
		args = append(args, "-shared")
	}

	// Add object files
	args = append(args, job.ObjectFiles...)

	// Add library files directly (full paths, e.g., lib/libmath.a)
	args = append(args, job.LibraryFiles...)

	// Add named libraries to link against with -l flag (e.g., "m" becomes "-lm")
	for _, lib := range job.Libraries {
		args = append(args, "-l"+lib)
	}

	// Add custom linker flags
	args = append(args, job.LinkerFlags...)

	// Specify output file (if provided)
	if job.OutputFile != "" {
		args = append(args, "-o", job.OutputFile)
	}

	// Debug log the link command that will be executed
	e.Debug("Linking objects with full command",
		"compiler", e.compilerPath,
		"argCount", len(args),
		"isSharedLibrary", job.SharedLibrary,
		"objectFileCount", len(job.ObjectFiles),
		"libraryFileCount", len(job.LibraryFiles),
		"libraryCount", len(job.Libraries),
		"workDir", e.workDir,
	)
	e.Debug("Link command arguments", "args", args)

	// Execute the linker
	stdout, stderr, exitCode, err := e.baseExecutor.ExecuteCommand(ctx, e.compilerPath, args, progressCallback)
	if err != nil {
		return nil, err
	}

	// Prepare result with collected output files
	outputPaths := []string{job.OutputFile}
	return e.prepareExecutionResult(ctx, exitCode, stdout, stderr, outputPaths)
}

// ExecuteArchiveJob executes a C/C++ archive job using the ar tool and returns an ExecutionResult.
// It invokes ar to create or update a static library archive from object files.
// Output files are collected and included in the result.
// If progressCallback is provided, progress updates are reported in real-time.
// Returns:
// - *runtime.ExecutionResult with stdout, stderr, exitCode, and output JobData
// - error: An error if the job was nil or other fatal issues occurred
func (e *Executor) ExecuteArchiveJob(ctx context.Context, job *ArchiveJob, progressCallback runtime.ProgressCallback) (*runtime.ExecutionResult, error) {
	if job == nil {
		return nil, e.Errorf("archive job is nil")
	}

	// Validate job has input files
	if len(job.InputFiles) == 0 {
		return nil, e.Errorf("archive job has no input files")
	}

	// Validate job has output file
	if job.OutputFile == "" {
		return nil, e.Errorf("archive job has no output file")
	}

	// Build ar arguments
	// ar command syntax: ar [command+modifiers] [options] archive-file input-files...
	// The parser splits combined flags like "qc" into separate strings ["q", "c"]
	// We need to reconstruct them back into a single command+modifiers string.
	args := []string{}

	// Reconstruct command+modifiers from individual flags
	// All flags go into a single string: "qc" instead of ["q", "c"]
	if len(job.ArFlags) > 0 {
		commandStr := ""
		for _, flag := range job.ArFlags {
			commandStr += flag
		}
		args = append(args, commandStr)
	}

	// Add output archive file
	args = append(args, job.OutputFile)

	// Add input files
	args = append(args, job.InputFiles...)

	// Debug log the archive command that will be executed
	e.Debug("Creating archive with full command",
		"archiver", e.archiverPath,
		"argCount", len(args),
		"inputFileCount", len(job.InputFiles),
		"outputFile", job.OutputFile,
		"workDir", e.workDir,
	)
	e.Debug("Archive command arguments", "args", args)

	// Execute the archiver
	stdout, stderr, exitCode, err := e.baseExecutor.ExecuteCommand(ctx, e.archiverPath, args, progressCallback)
	if err != nil {
		return nil, err
	}

	// Prepare result with collected output files
	outputPaths := []string{job.OutputFile}
	return e.prepareExecutionResult(ctx, exitCode, stdout, stderr, outputPaths)
}
