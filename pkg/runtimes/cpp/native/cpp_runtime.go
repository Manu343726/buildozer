package native

import (
	"context"
	"fmt"

	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	"github.com/Manu343726/buildozer/internal/logger"
	"github.com/Manu343726/buildozer/pkg/runtime"
)

// NativeCppRuntime implements the runtime.Runtime interface for native C/C++ compilation.
// It provides job execution capabilities by delegating to an Executor with a concrete Toolchain configuration.
// This type acts as a bridge between the generic Runtime interface and concrete C/C++ compilation operations.
type NativeCppRuntime struct {
	// toolchain contains the specific C/C++ compiler configuration (compiler type, version, architecture, etc.).
	toolchain *Toolchain
	// executor handles the actual compilation and linking operations.
	executor *Executor
	// log is the logger for runtime operations.
	log *logger.ComponentLogger
}

// NewNativeCppRuntime creates and returns a new NativeCppRuntime instance.
// Parameters:
// - toolchain: The concrete C/C++ toolchain configuration
// - workDir: The working directory for temporary files and compiler operations
// Returns a fully initialized runtime ready for execution.
func NewNativeCppRuntime(toolchain *Toolchain, workDir string) *NativeCppRuntime {
	return &NativeCppRuntime{
		toolchain: toolchain,
		executor:  NewExecutor(toolchain.CompilerPath, workDir),
		log:       logger.NewComponentLogger(fmt.Sprintf("cpp-native-%s", toolchain.Compiler)),
	}
}

// Execute executes a C/C++ job according to the runtime.Runtime interface.
// It accepts protocol buffer job specifications (CppCompileJob or CppLinkJob),
// converts them to concrete types, delegates to the executor, and returns results.
// Supported job types:
// - *v1.CppCompileJob: Compilation of source files to object files
// - *v1.CppLinkJob: Linking of object files to executables or libraries
// Returns an error if the request is nil or an unsupported job type is encountered.
func (r *NativeCppRuntime) Execute(ctx context.Context, req *runtime.ExecutionRequest) (*runtime.ExecutionResult, error) {
	if req == nil {
		return nil, r.log.Errorf("execution request is nil")
	}

	var stdout, stderr []byte
	var exitCode int
	var err error

	switch job := req.Job.(type) {
	case *v1.CppCompileJob:
		r.log.Info("executing C/C++ compile job", "sources", len(job.SourceFiles), "output", job.OutputFile)
		compileJob := r.protoCompileJobToConcrete(job)
		stdout, stderr, exitCode, err = r.executor.ExecuteCompileJob(ctx, compileJob, req.ProgressCallback)
		if err != nil {
			return nil, err
		}

	case *v1.CppLinkJob:
		r.log.Info("executing C/C++ link job", "objects", len(job.ObjectFiles), "output", job.OutputFile)
		linkJob := r.protoLinkJobToConcrete(job)
		stdout, stderr, exitCode, err = r.executor.ExecuteLinkJob(ctx, linkJob, req.ProgressCallback)
		if err != nil {
			return nil, err
		}

	default:
		return nil, r.log.Errorf("unsupported job type: %T", job)
	}

	return &runtime.ExecutionResult{
		ExitCode:    exitCode,
		Stdout:      stdout,
		Stderr:      stderr,
		Output:      make(map[string][]byte),
		ExecutionID: r.RuntimeID(),
	}, nil
}

// Available checks whether this runtime is available and functional.
// It attempts to run the compiler with the --version flag to verify the compiler exists and is executable.
// Returns true if the compiler is available, false otherwise.
// An error is only returned if there's a fatal system error; compiler unavailability is not an error.
func (r *NativeCppRuntime) Available(ctx context.Context) (bool, error) {
	executor := NewExecutor(r.toolchain.CompilerPath, "/tmp")
	_, _, exitCode, err := executor.executeCommand(ctx, []string{"--version"}, nil)
	if err != nil || exitCode != 0 {
		return false, nil
	}
	r.log.Debug("compiler available", "compiler", r.toolchain.Compiler)
	return true, nil
}

// Metadata returns runtime.Metadata describing this C/C++ runtime.
// The metadata includes:
// - RuntimeID: Unique identifier for this runtime
// - Language: "c" or "cpp"
// - Compiler: Compiler name and version
// - Architecture: Target CPU architecture
// - Description: Human-readable summary
// - Details: Additional technical information (compiler path)
func (r *NativeCppRuntime) Metadata(ctx context.Context) (*runtime.Metadata, error) {
	langStr := "c"
	if r.toolchain.Language == LanguageCpp {
		langStr = "cpp"
	}

	compilerName := "unknown"
	switch r.toolchain.Compiler {
	case CompilerGCC:
		compilerName = "gcc"
	case CompilerClang:
		compilerName = "clang"
	}

	return &runtime.Metadata{
		RuntimeID:   r.RuntimeID(),
		RuntimeType: fmt.Sprintf("native-%s", compilerName),
		Language:    langStr,
		Version:     r.toolchain.CompilerVersion,
		TargetOS:    "linux",
		TargetArch:  string(r.toolchain.Architecture),
		IsNative:    true,
		Description: fmt.Sprintf("Native %s %s (%s)", compilerName, r.toolchain.CompilerVersion, r.toolchain.Architecture),
		Details:     fmt.Sprintf("path=%s", r.toolchain.CompilerPath),
	}, nil
}

// RuntimeID returns a unique identifier for this runtime instance.
// The ID is deterministic and includes:
// - The compiler name (gcc, clang, or unknown)
// - The compiler version
// - The target architecture
// Format: "native-cpp-<compiler>-<version>-<arch>"
func (r *NativeCppRuntime) RuntimeID() string {
	compilerName := "unknown"
	switch r.toolchain.Compiler {
	case CompilerGCC:
		compilerName = "gcc"
	case CompilerClang:
		compilerName = "clang"
	}
	return fmt.Sprintf("native-cpp-%s-%s-%s", compilerName, r.toolchain.CompilerVersion, r.toolchain.Architecture)
}

// protoCompileJobToConcrete converts a protocol buffer CppCompileJob to a concrete CompileJob.
// This method translates between the protocol representation (used for network serialization)
// and the internal representation (used by the executor).
// Note: Preprocessor defines in the proto are assumed to be in "KEY" or "KEY=VALUE" format.
func (r *NativeCppRuntime) protoCompileJobToConcrete(proto *v1.CppCompileJob) *CompileJob {
	defines := make(map[string]string)
	for _, define := range proto.Defines {
		// Parse: "KEY=VALUE" or just "KEY"
		if idx := -1; idx < 0 {
			for i := 0; i < len(define); i++ {
				if define[i] == '=' {
					idx = i
					break
				}
			}
			if idx > 0 {
				defines[define[:idx]] = define[idx+1:]
			} else {
				defines[define] = ""
			}
		}
	}

	return &CompileJob{
		SourceFiles:   proto.SourceFiles,
		IncludeDirs:   proto.IncludeDirs,
		Defines:       defines,
		CompilerFlags: proto.CompilerArgs,
		OutputFile:    proto.OutputFile,
	}
}

// protoLinkJobToConcrete converts a protocol buffer CppLinkJob to a concrete LinkJob.
// This method translates between the protocol representation and the internal representation.
func (r *NativeCppRuntime) protoLinkJobToConcrete(proto *v1.CppLinkJob) *LinkJob {
	return &LinkJob{
		ObjectFiles:   proto.ObjectFiles,
		Libraries:     proto.Libraries,
		LinkerFlags:   proto.LinkerArgs,
		OutputFile:    proto.OutputFile,
		SharedLibrary: proto.IsSharedLibrary,
	}
}
