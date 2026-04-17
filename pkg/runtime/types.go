// Package runtime provides abstractions for job execution environments.
// Supports multiple languages and runtime types (native, Docker, etc.).
package runtime

import (
	"context"
	"time"

	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
)

// ProgressType indicates the type of progress update being reported.
type ProgressType string

const (
	// ProgressTypeOutput indicates stdout or stderr data was captured.
	ProgressTypeOutput ProgressType = "output"
	// ProgressTypeLog indicates a structured log message.
	ProgressTypeLog ProgressType = "log"
	// ProgressTypeStatus indicates a status change or milestone.
	ProgressTypeStatus ProgressType = "status"
)

// Progress represents a real-time progress update during job execution.
type Progress struct {
	// Type indicates the kind of progress update.
	Type ProgressType

	// Source indicates where the data came from ("stdout", "stderr", "executor", "runtime", etc.).
	Source string

	// Data contains the actual progress data (output, message, status, etc.).
	// For ProgressTypeOutput: raw bytes output from the job
	// For ProgressTypeLog: formatted log message
	// For ProgressTypeStatus: status description
	Data []byte

	// Timestamp is when this progress was reported.
	Timestamp time.Time

	// ExecutionID is the ID of the execution this progress belongs to (optional).
	ExecutionID string
}

// ProgressCallback is invoked during job execution to report progress in real-time.
// Implementations should process the progress update quickly and not block the execution.
// If the callback returns an error, the execution may be cancelled.
// The callback should be thread-safe if multiple goroutines report progress simultaneously.
type ProgressCallback func(ctx context.Context, progress *Progress) error

// ExecutionRequest represents a generic request to execute a job on a runtime.
// The request contains the full Job proto with all inputs embedded as JobData.
// Each Runtime implementation is responsible for:
// - Creating its own execution context (working directory, temp files)
// - Extracting input files from JobData
// - Setting up output directories
// - Executing the job in the appropriate environment
// - Optionally cleaning up after execution
type ExecutionRequest struct {
	// FullJob is the complete protocol buffer Job message with all inputs embedded.
	// Contains the job specification (CppCompileJob, CppLinkJob) and JobData inputs.
	// The runtime is responsible for extracting inputs and executing the job.
	FullJob *v1.Job

	// ProgressCallback is an optional callback for real-time progress reporting during execution.
	// If provided, the runtime will invoke this callback with Progress updates as they occur.
	// Implementations should report output data, log messages, and status changes.
	// The callback may be nil if progress reporting is not needed.
	ProgressCallback ProgressCallback

	// CompletionCallback is an optional callback invoked when job execution completes.
	// Called after the job finishes (success or failure) to handle finalization like
	// output materialization and verification. Takes the job and execution result.
	// The callback may be nil if completion handling is not needed.
	CompletionCallback func(context.Context, *v1.Job, *ExecutionResult) error
}

// ExecutionResult represents the result of executing a job.
type ExecutionResult struct {
	// ExitCode is the execution exit code (0 = success, non-zero = failure).
	ExitCode int

	// Stdout is the standard output from execution (if applicable).
	Stdout []byte

	// Stderr is the standard error output (if applicable).
	Stderr []byte

	// Output contains output artifacts produced by execution.
	// For sandboxed/remote runtimes: array of JobData with content filled.
	// For local unsandboxed runtimes: empty (files already on disk in WorkDir).
	Output []*v1.JobData

	// WorkDir is the working directory where execution occurred.
	// For local unsandboxed runtimes, this is the temp directory where files were written.
	// Used to collect output files for materialization.
	WorkDir string

	// ExecutionID is an optional identifier for this execution (for logging/debugging).
	ExecutionID string
}

// Metadata represents generic runtime metadata for identification and discovery.
// Each Runtime implementation populates relevant fields; not all fields apply to all runtimes.
type Metadata struct {
	// RuntimeID is the unique identifier for this runtime.
	// Generated as SHA256 of (toolchain || recipe || resource_limits).
	// Examples: "hash-of-cpp-gcc11-x64", "hash-of-go-1.21", "hash-of-docker-image-xyz"
	RuntimeID string

	// RuntimeType describes the type of runtime (e.g., "native-linux-gcc", "docker", "native-go").
	RuntimeType string

	// Language identifies the programming language(s) this runtime supports (e.g., "c", "cpp", "go", "rust").
	Language string

	// Description is a human-readable description of this runtime.
	Description string

	// IsNative indicates whether this runtime is native (true) or Docker-based (false).
	IsNative bool

	// Version represents the version of the main toolchain/component.
	// For C/C++: compiler version (e.g., "11.2.0")
	// For Go: Go version (e.g., "1.21.0")
	// For Rust: Rust version (e.g., "1.75.0")
	Version string

	// TargetOS is the target operating system (e.g., "linux", "darwin", "windows").
	TargetOS string

	// TargetArch is the target CPU architecture (e.g., "x86_64", "aarch64", "arm").
	TargetArch string

	// Details is a free-form string with additional runtime-specific details.
	// For Docker: image digest or name
	// For native: toolchain paths or system info
	Details string
}

// Runtime represents an execution environment for jobs.
// Implementations can be native (system toolchains) or Docker-based.
type Runtime interface {
	// Execute runs a command in this runtime and returns the result.
	// ctx is used for cancellation and timeouts.
	Execute(ctx context.Context, req *ExecutionRequest) (*ExecutionResult, error)

	// Available returns true if this runtime is available and ready for execution.
	Available(ctx context.Context) (bool, error)

	// Metadata returns metadata about this runtime for matching and identification.
	Metadata(ctx context.Context) (*Metadata, error)

	// Proto returns a *v1.Runtime proto representation of this runtime for API responses.
	Proto(ctx context.Context) (*v1.Runtime, error)

	// RuntimeID returns the unique identifier for this runtime.
	RuntimeID() string

	// MatchesQuery checks if this runtime matches the given RuntimeMatchQuery based on its metadata.
	MatchesQuery(ctx context.Context, query *v1.RuntimeMatchQuery) (bool, error)

	// Close releases any resources held by this runtime.
	// For example, temporary directories, network connections, or containers.
	// After Close is called, the runtime should not be used for further operations.
	// Calling Close on an already-closed runtime should not cause an error.
	Close() error
}

// AvailabilityError represents an error when a runtime is not available.
type AvailabilityError struct {
	RuntimeID string
	Reason    string
}

// Error implements the error interface.
func (e *AvailabilityError) Error() string {
	return "runtime " + e.RuntimeID + " not available: " + e.Reason
}
