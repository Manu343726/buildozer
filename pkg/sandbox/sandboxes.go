// Package sandbox provides filesystem isolation for job execution.
// Sandboxes are composable functions that wrap runtimes to add isolation, data materialization, and cleanup.
//
// Each sandbox is a function that takes a runtime.Runtime and returns a wrapped runtime.Runtime.
// Multiple sandboxes can be composed using the Builder pattern.
//
// Sandbox creation is unified with SandboxFactory:
//   - All public factory functions return SandboxFactory
//   - SandboxFactory = func(SandboxParams) SandboxFunc
//   - SandboxParams allows extensibility for future parameters
//
// Example:
//
//	pipeline := sandbox.NewBuilder(logger).
//	    With(sandbox.EmbedInputs()).
//	    With(sandbox.TempDir()).
//	    Build()
//	wrapped, err := pipeline(runtime)
package sandbox

import (
	"context"
	"fmt"
	"os"

	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	"github.com/Manu343726/buildozer/pkg/logging"
	"github.com/Manu343726/buildozer/pkg/runtime"
	"github.com/Manu343726/buildozer/pkg/staging"
)

// SandboxFunc wraps a runtime with additional isolation/transformation logic.
// It takes a runtime and returns a new runtime with the sandbox behavior applied.
type SandboxFunc func(runtime.Runtime) (runtime.Runtime, error)

// SandboxParams contains standard parameters for sandbox creation.
// This struct can be extended in the future with additional fields without changing function signatures.
type SandboxParams struct {
	Logger *logging.Logger
}

// SandboxFactory creates a SandboxFunc given sandbox parameters.
// Factories are composable functions that can be passed to Pipe, Apply, or Builder.
type SandboxFactory func(SandboxParams) SandboxFunc

// ExecuteFunc is a function that implements custom Execute behavior.
// It receives the context, wrapped runtime, and the execution request,
// and can perform any transformations or operations before/after calling the original runtime.
type ExecuteFunc func(context.Context, runtime.Runtime, *runtime.ExecutionRequest) (*runtime.ExecutionResult, error)

// Execute returns a SandboxFactory that wraps a runtime with a custom execution function.
// This is a generic sandbox factory that applies custom execution logic without materializing data.
// Use this to create custom execution sandboxes for specific behavior.
//
// Example:
//
//	customExec := func(ctx context.Context, rt runtime.Runtime, req *runtime.ExecutionRequest) (*runtime.ExecutionResult, error) {
//	    // Custom logic before/after execution
//	    return rt.Execute(ctx, req)
//	}
//	factory := sandbox.Execute(customExec)
//	params := sandbox.SandboxParams{Logger: logger}
//	pipeline := sandbox.Pipe(params, factory)
func Execute(fn ExecuteFunc) SandboxFactory {
	return func(params SandboxParams) SandboxFunc {
		logger := params.Logger
		if logger == nil {
			logger = logging.Log().Child("execute-sandbox")
		}

		return func(rt runtime.Runtime) (runtime.Runtime, error) {
			return &wrappedRuntime{
				Logger:   logger,
				wrapped:  rt,
				execFunc: fn,
			}, nil
		}
	}
}

// EmbedInputs returns a SandboxFactory that materializes all reference inputs to embedded content
// before execution. This ensures remote or distributed runtimes receive self-contained data.
func EmbedInputs() SandboxFactory {
	return Execute(embedInputsExec())
}

func embedInputsExec() ExecuteFunc {
	return func(ctx context.Context, rt runtime.Runtime, req *runtime.ExecutionRequest) (*runtime.ExecutionResult, error) {
		if req == nil || req.FullJob == nil {
			return rt.Execute(ctx, req)
		}

		if len(req.FullJob.Inputs) > 0 {
			stager := staging.NewJobDataStager(req.FullJob.Cwd)
			embeddedInputs, err := stager.EmbedJobDataList(ctx, req.FullJob.Inputs)
			if err != nil {
				return nil, fmt.Errorf("failed to embed inputs: %w", err)
			}
			req.FullJob.Inputs = embeddedInputs
		}

		return rt.Execute(ctx, req)
	}
}

// EmbedOutputs returns a SandboxFactory that materializes all reference outputs to embedded content
// after execution. Useful for consumers that need self-contained output data.
func EmbedOutputs() SandboxFactory {
	return Execute(embedOutputsExec())
}

func embedOutputsExec() ExecuteFunc {
	return func(ctx context.Context, rt runtime.Runtime, req *runtime.ExecutionRequest) (*runtime.ExecutionResult, error) {
		result, err := rt.Execute(ctx, req)

		if err != nil {
			return result, err
		}

		if result == nil || len(result.Output) == 0 {
			return result, err
		}

		if req != nil && req.FullJob != nil {
			stager := staging.NewJobDataStager(req.FullJob.Cwd)
			embeddedOutputs, embedErr := stager.EmbedJobDataList(ctx, result.Output)
			if embedErr != nil {
				return nil, fmt.Errorf("failed to embed outputs: %w", embedErr)
			}
			result.Output = embeddedOutputs
		}

		return result, err
	}
}

// TempDir returns a SandboxFactory that creates a per-job temporary directory for execution.
// On success (exit code 0 and no error), the tmpdir is cleaned up.
// On failure, the tmpdir is kept for debugging with the jobID and location logged.
func TempDir() SandboxFactory {
	return Execute(tempDirExec())
}

func tempDirExec() ExecuteFunc {
	return func(ctx context.Context, rt runtime.Runtime, req *runtime.ExecutionRequest) (*runtime.ExecutionResult, error) {
		if req == nil || req.FullJob == nil {
			return rt.Execute(ctx, req)
		}

		jobID := req.FullJob.Id
		prefix := fmt.Sprintf("buildozer-job-%s-", jobID)
		tempDir, err := os.MkdirTemp("", prefix)
		if err != nil {
			return nil, fmt.Errorf("failed to create temp directory: %w", err)
		}

		// Wrap with Workdir to execute in the temp directory
		workdirFactory := Workdir(tempDir)
		workdirSandbox := workdirFactory(SandboxParams{})
		workdirWrapped, err := workdirSandbox(rt)
		if err != nil {
			os.RemoveAll(tempDir)
			return nil, fmt.Errorf("failed to wrap with workdir: %w", err)
		}

		// Execute in the temp directory
		result, execErr := workdirWrapped.Execute(ctx, req)

		// Clean up on success (no error and exit code 0)
		// Keep on failure (error or non-zero exit) for debugging
		if execErr == nil && result != nil && result.ExitCode == 0 {
			os.RemoveAll(tempDir)
		} else {
			if execErr != nil {
				// Keep temp directory for debugging
			}
		}

		return result, execErr
	}
}

// Workdir returns a SandboxFactory for executing in a fixed directory.
// The directory must exist, or wrapping will fail.
func Workdir(workdir string) SandboxFactory {
	return func(params SandboxParams) SandboxFunc {
		logger := params.Logger
		if logger == nil {
			logger = logging.Log().Child("workdir")
		}

		return func(rt runtime.Runtime) (runtime.Runtime, error) {
			// Validate directory exists
			stat, err := os.Stat(workdir)
			if err != nil {
				return nil, fmt.Errorf("workdir does not exist or is not accessible: %w", err)
			}
			if !stat.IsDir() {
				return nil, fmt.Errorf("workdir path is not a directory: %s", workdir)
			}

			return &wrappedRuntime{
				Logger:   logger,
				wrapped:  rt,
				execFunc: workdirExec(workdir),
			}, nil
		}
	}
}

func workdirExec(workdir string) ExecuteFunc {
	return func(ctx context.Context, rt runtime.Runtime, req *runtime.ExecutionRequest) (*runtime.ExecutionResult, error) {
		if req == nil || req.FullJob == nil {
			return rt.Execute(ctx, req)
		}

		// Materialize all input files to the workdir and convert to references
		if len(req.FullJob.Inputs) > 0 {
			stager := staging.NewJobDataStager(workdir)

			// Write all inputs to workdir
			if err := stager.WriteJobDataListToFiles(ctx, req.FullJob.Inputs, workdir); err != nil {
				return nil, fmt.Errorf("failed to materialize inputs to workdir: %w", err)
			}

			// Convert inputs to references (after materialization, they're on disk)
			refInputs, err := stager.CreateJobDataListForFiles(ctx, getInputPaths(req.FullJob.Inputs), staging.JobDataModeReference)
			if err != nil {
				return nil, fmt.Errorf("failed to convert inputs to references: %w", err)
			}
			req.FullJob.Inputs = refInputs
		}

		// Update the job's working directory
		req.FullJob.Cwd = workdir

		return rt.Execute(ctx, req)
	}
}

// getInputPaths extracts file paths from JobData inputs
func getInputPaths(inputs []*v1.JobData) []string {
	var paths []string
	for _, input := range inputs {
		if file := input.GetFile(); file != nil {
			paths = append(paths, file.Path)
		} else if dir := input.GetDirectory(); dir != nil {
			paths = append(paths, dir.Path)
		}
	}
	return paths
}

// wrappedRuntime is a generic runtime wrapper that applies a custom execute function.
type wrappedRuntime struct {
	*logging.Logger
	wrapped  runtime.Runtime
	execFunc func(context.Context, runtime.Runtime, *runtime.ExecutionRequest) (*runtime.ExecutionResult, error)
}

// Execute applies the custom logic.
func (w *wrappedRuntime) Execute(ctx context.Context, req *runtime.ExecutionRequest) (*runtime.ExecutionResult, error) {
	return w.execFunc(ctx, w.wrapped, req)
}

// Available delegates to wrapped.
func (w *wrappedRuntime) Available(ctx context.Context) (bool, error) {
	return w.wrapped.Available(ctx)
}

// Metadata delegates to wrapped.
func (w *wrappedRuntime) Metadata(ctx context.Context) (*runtime.Metadata, error) {
	return w.wrapped.Metadata(ctx)
}

// Proto delegates to wrapped.
func (w *wrappedRuntime) Proto(ctx context.Context) (*v1.Runtime, error) {
	return w.wrapped.Proto(ctx)
}

// RuntimeID delegates to wrapped.
func (w *wrappedRuntime) RuntimeID() string {
	return w.wrapped.RuntimeID()
}

// MatchesQuery delegates to wrapped.
func (w *wrappedRuntime) MatchesQuery(ctx context.Context, query *v1.RuntimeMatchQuery) (bool, error) {
	return w.wrapped.MatchesQuery(ctx, query)
}

// Close delegates to wrapped.
func (w *wrappedRuntime) Close() error {
	return w.wrapped.Close()
}
