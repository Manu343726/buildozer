package native

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/Manu343726/buildozer/pkg/logging"
	"github.com/Manu343726/buildozer/pkg/runtime"
)

// Executor manages the execution of C/C++ compilation and linking operations.
// It encapsulates compiler subprocess management, working directory setup, and output capture.
type Executor struct {
	// compilerPath is the filesystem path to the compiler executable.
	compilerPath string
	// workDir is the working directory for compiler invocations (where temp files are stored).
	workDir string
	// log is the logger for executor operations.
	log *logging.Logger
}

// NewExecutor creates and returns a new Executor for a specific compiler.
// Parameters:
// - compilerPath: The filesystem path to the compiler executable (e.g., "/usr/bin/gcc")
// - workDir: The directory for temporary files and compiler working directory
func NewExecutor(compilerPath, workDir string) *Executor {
	return &Executor{
		compilerPath: compilerPath,
		workDir:      workDir,
		log:          Log(),
	}
}

// ExecuteCompileJob executes a C/C++ compilation job.
// It invokes the compiler with the job specifications and captures stdout/stderr.
// If progressCallback is provided, progress updates are reported in real-time.
// The callback is invoked as output data becomes available.
// Returns:
// - stdout: The standard output from the compiler
// - stderr: The standard error output from the compiler
// - exitCode: The exit code (0 for success)
// - error: An error if the job was nil or other fatal issues occurred
func (e *Executor) ExecuteCompileJob(ctx context.Context, job *CompileJob, progressCallback runtime.ProgressCallback) ([]byte, []byte, int, error) {
	if job == nil {
		return nil, nil, 1, e.log.Errorf("compile job is nil")
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

	// Specify output file
	args = append(args, "-o", job.OutputFile)

	return e.executeCommand(ctx, args, progressCallback)
}

// ExecuteLinkJob executes a C/C++ linking job.
// It invokes the compiler in linker mode to produce an executable or shared library.
// For shared libraries, the -shared flag is prepended before object files.
// If progressCallback is provided, progress updates are reported in real-time.
// The callback is invoked as output data becomes available.
// Returns:
// - stdout: The standard output from the linker
// - stderr: The standard error output from the linker
// - exitCode: The exit code (0 for success)
// - error: An error if the job was nil or other fatal issues occurred
func (e *Executor) ExecuteLinkJob(ctx context.Context, job *LinkJob, progressCallback runtime.ProgressCallback) ([]byte, []byte, int, error) {
	if job == nil {
		return nil, nil, 1, e.log.Errorf("link job is nil")
	}

	args := []string{}

	// Add -shared flag for shared libraries before object files
	if job.SharedLibrary {
		args = append(args, "-shared")
	}

	// Add object files
	args = append(args, job.ObjectFiles...)

	// Add libraries to link against
	for _, lib := range job.Libraries {
		args = append(args, "-l"+lib)
	}

	// Add custom linker flags
	args = append(args, job.LinkerFlags...)

	// Specify output file
	args = append(args, "-o", job.OutputFile)

	return e.executeCommand(ctx, args, progressCallback)
}

// executeCommand runs the compiler with the given arguments.
// It sets up the working directory, captures stdout and stderr with real-time progress reporting,
// executes the compiler subprocess, and returns the complete captured output.
// If progressCallback is provided, it is invoked as output data becomes available.
// Returns:
// - stdout: Complete output captured from stdout
// - stderr: Complete output captured from stderr
// - exitCode: The process exit code (0 for success)
// - error: An error if directory or subprocess operations failed (excludes compiler exit codes)
func (e *Executor) executeCommand(ctx context.Context, args []string, progressCallback runtime.ProgressCallback) ([]byte, []byte, int, error) {
	// Ensure work directory exists
	if err := os.MkdirAll(e.workDir, 0755); err != nil {
		return nil, nil, 1, e.log.Errorf("failed to create work directory: %w", err)
	}

	// Create pipes for real-time output capture
	stdoutReader, stdoutWriter := io.Pipe()
	stderrReader, stderrWriter := io.Pipe()

	// Buffers to accumulate complete output
	stdoutBuf := &bytes.Buffer{}
	stderrBuf := &bytes.Buffer{}

	// Create multi-writers to capture output while also reporting progress
	stdoutMulti := io.MultiWriter(stdoutWriter, stdoutBuf)
	stderrMulti := io.MultiWriter(stderrWriter, stderrBuf)

	// Configure and run the compiler command
	cmd := exec.CommandContext(ctx, e.compilerPath, args...)
	cmd.Dir = e.workDir
	cmd.Stdout = stdoutMulti
	cmd.Stderr = stderrMulti

	// Launch goroutines to read from pipes and report progress
	done := make(chan error, 2)

	go func() {
		e.reportProgress(ctx, stdoutReader, "stdout", progressCallback)
		done <- nil
	}()

	go func() {
		e.reportProgress(ctx, stderrReader, "stderr", progressCallback)
		done <- nil
	}()

	// Execute the compiler
	exitCode := 0
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	// Close writers to signal EOF to readers
	stdoutWriter.Close()
	stderrWriter.Close()

	// Wait for progress reporting goroutines to complete
	<-done
	<-done

	return stdoutBuf.Bytes(), stderrBuf.Bytes(), exitCode, nil
}

// reportProgress reads from a pipe and reports progress updates.
// It sends ProgressTypeOutput updates for each line of output.
// Data is sent in chunks; large output streams are split into manageable pieces.
func (e *Executor) reportProgress(ctx context.Context, reader io.Reader, source string, progressCallback runtime.ProgressCallback) {
	if progressCallback == nil {
		// If no callback, just discard the data
		io.Copy(io.Discard, reader)
		return
	}

	// Buffer for reading chunks
	buf := make([]byte, 4096)

	for {
		n, err := reader.Read(buf)
		if n > 0 {
			// Report progress for this chunk of output
			progress := &runtime.Progress{
				Type:      runtime.ProgressTypeOutput,
				Source:    source,
				Data:      append([]byte{}, buf[:n]...), // Copy data
				Timestamp: time.Now(),
			}

			if cbErr := progressCallback(ctx, progress); cbErr != nil {
				e.log.Errorf("progress callback error: %w", cbErr)
				// Continue reporting even if callback fails
			}
		}

		if err != nil {
			if err != io.EOF {
				e.log.Errorf("error reading %s: %w", source, err)
			}
			break
		}
	}
}
