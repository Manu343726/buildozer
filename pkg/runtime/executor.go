package runtime

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/Manu343726/buildozer/pkg/logging"
)

// Executor provides generic methods for executing jobs on runtimes.
// It provides common logic that runtimes may share, like executing commands, handling progress callbacks, and logging.
type Executor interface {
	// ExecuteCommand executes a command with arguments in the runtime's environment.
	// It captures stdout and stderr, reports progress via the callback, and returns the complete output and exit code.
	ExecuteCommand(ctx context.Context, command string, args []string, progressCallback ProgressCallback) ([]byte, []byte, int, error)
}

type basicExecutor struct {
	*logging.Logger        // Embed Logger for logging execution details
	workDir         string // Current working directory for job execution
}

func NewExecutor(workDir string, parentLogger *logging.Logger) Executor {
	return &basicExecutor{
		Logger:  parentLogger.Child("Executor"),
		workDir: workDir,
	}
}

// It sets up the working directory, captures stdout and stderr with real-time progress reporting,
// executes the compiler subprocess, and returns the complete captured output.
// If progressCallback is provided, it is invoked as output data becomes available.
// Returns:
// - stdout: Complete output captured from stdout
// - stderr: Complete output captured from stderr
// - exitCode: The process exit code (0 for success)
// - error: An error if directory or subprocess operations failed (excludes compiler exit codes)
func (e *basicExecutor) ExecuteCommand(ctx context.Context, command string, args []string, progressCallback ProgressCallback) ([]byte, []byte, int, error) {
	// Ensure work directory exists
	if err := os.MkdirAll(e.workDir, 0755); err != nil {
		return nil, nil, 1, e.Errorf("failed to create work directory: %w", err)
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

	// Configure and run the command
	cmd := exec.CommandContext(ctx, command, args...)
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

	e.Debug("starting execution",
		"command", command,
		"args", args,
		"working_dir", e.workDir)

	exitCode := 0
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	e.Debug("execution completed",
		"exit_code", exitCode,
		"command", command,
		"args", args,
		"working_dir", e.workDir)

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
// Lines are detected by newline characters and sent individually.
// Logs each line of output for observability.
func (e *basicExecutor) reportProgress(ctx context.Context, reader io.Reader, source string, progressCallback ProgressCallback) {
	// Use Scanner to read line by line
	scanner := bufio.NewScanner(reader)

	for scanner.Scan() {
		line := scanner.Bytes()
		// Add newline back since scanner removes it
		lineWithNewline := append(append([]byte{}, line...), '\n')

		// Log the output line
		e.Info("execution output",
			"source", source,
			"output", string(line))

		// Report progress if callback provided
		if progressCallback != nil {
			// Report progress for this line
			progress := &Progress{
				Type:      ProgressTypeOutput,
				Source:    source,
				Data:      lineWithNewline,
				Timestamp: time.Now(),
			}

			if cbErr := progressCallback(ctx, progress); cbErr != nil {
				e.Error("progress callback error", "error", cbErr)
				// Continue reporting even if callback fails
			}
		}
	}

	if err := scanner.Err(); err != nil {
		e.Error("error reading output", "source", source, "error", err)
	}
}
