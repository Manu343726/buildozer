package native

import (
	"context"
	"fmt"
	"strconv"
	"strings"

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
	compilerName := "unknown"
	switch toolchain.Compiler {
	case CompilerGCC:
		compilerName = "gcc"
	case CompilerClang:
		compilerName = "clang"
	}
	return &NativeCppRuntime{
		toolchain: toolchain,
		executor:  NewExecutor(toolchain.CompilerPath, workDir),
		log:       logger.NewComponentLogger(fmt.Sprintf("cpp-native-%s", compilerName)),
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

// GetToolchain returns the internal Toolchain configuration for this runtime.
// This is used for protocol serialization and detailed inspection.
func (r *NativeCppRuntime) GetToolchain() *Toolchain {
	return r.toolchain
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
// Format: native-<language>-<compiler>-<version>-<cruntime>-<cruntimeVersion>-[<stdlib>-]<arch>
// The ID is deterministic and includes:
// - Language: "c" or "cpp"
// - Compiler name: "gcc", "clang"
// - Compiler version (e.g., "10.2.1")
// - C runtime: "glibc", "musl"
// - C runtime version (e.g., "2.31")
// - C++ stdlib (only for C++): "libstdc++", "libc++"
// - Target architecture: "x86_64", "aarch64", "arm"
// Examples:
//
//	native-c-gcc-10.2.1-glibc-2.31-x86_64 (C with GCC)
//	native-cpp-gcc-10.2.1-glibc-2.31-libstdc++-x86_64 (C++ with GCC)
//	native-cpp-clang-11.0.1-glibc-2.31-libc++-x86_64 (C++ with Clang)
func (r *NativeCppRuntime) RuntimeID() string {
	// Language: "c" or "cpp"
	languageName := "c"
	if r.toolchain.Language == LanguageCpp {
		languageName = "cpp"
	}

	// Compiler name: "gcc" or "clang"
	compilerName := "unknown"
	switch r.toolchain.Compiler {
	case CompilerGCC:
		compilerName = "gcc"
	case CompilerClang:
		compilerName = "clang"
	}

	// C runtime name: "glibc" or "musl"
	cruntimeName := "unknown"
	switch r.toolchain.CRuntime {
	case CRuntimeGlibc:
		cruntimeName = "glibc"
	case CRuntimeMusl:
		cruntimeName = "musl"
	}

	// Build the ID: native-<language>-<compiler>-<version>-<cruntime>-<cruntimeVersion>
	id := fmt.Sprintf("native-%s-%s-%s-%s-%s",
		languageName,
		compilerName,
		r.toolchain.CompilerVersion,
		cruntimeName,
		r.toolchain.CRuntimeVersion)

	// For C++, include the C++ stdlib before architecture
	if r.toolchain.Language == LanguageCpp {
		stdlibName := "unknown"
		switch r.toolchain.CppStdlib {
		case CppStdlibLibstdcxx:
			stdlibName = "libstdc++"
		case CppStdlibLibcxx:
			stdlibName = "libc++"
		}
		id = fmt.Sprintf("%s-%s", id, stdlibName)
	}

	// Finally, append architecture
	id = fmt.Sprintf("%s-%s", id, r.toolchain.Architecture)

	return id
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

// ProtoLanguage converts internal Language to protocol buffer enum
func (r *NativeCppRuntime) ProtoLanguage() v1.CppLanguage {
	switch r.toolchain.Language {
	case LanguageC:
		return v1.CppLanguage_CPP_LANGUAGE_C
	case LanguageCpp:
		return v1.CppLanguage_CPP_LANGUAGE_CPP
	default:
		return v1.CppLanguage_CPP_LANGUAGE_UNSPECIFIED
	}
}

// ProtoCompiler converts internal Compiler to protocol buffer enum
func (r *NativeCppRuntime) ProtoCompiler() v1.CppCompiler {
	switch r.toolchain.Compiler {
	case CompilerGCC:
		return v1.CppCompiler_CPP_COMPILER_GCC
	case CompilerClang:
		return v1.CppCompiler_CPP_COMPILER_CLANG
	default:
		return v1.CppCompiler_CPP_COMPILER_UNSPECIFIED
	}
}

// ProtoArchitecture converts internal Architecture to protocol buffer enum
func (r *NativeCppRuntime) ProtoArchitecture() v1.CpuArchitecture {
	switch r.toolchain.Architecture {
	case ArchitectureX86_64:
		return v1.CpuArchitecture_CPU_ARCHITECTURE_X86_64
	case ArchitectureAArch64:
		return v1.CpuArchitecture_CPU_ARCHITECTURE_AARCH64
	case ArchitectureARM:
		return v1.CpuArchitecture_CPU_ARCHITECTURE_ARM
	default:
		return v1.CpuArchitecture_CPU_ARCHITECTURE_UNSPECIFIED
	}
}

// ProtoCRuntime converts internal CRuntime to protocol buffer enum
func (r *NativeCppRuntime) ProtoCRuntime() v1.CRuntime {
	switch r.toolchain.CRuntime {
	case CRuntimeGlibc:
		return v1.CRuntime_C_RUNTIME_GLIBC
	case CRuntimeMusl:
		return v1.CRuntime_C_RUNTIME_MUSL
	default:
		return v1.CRuntime_C_RUNTIME_UNSPECIFIED
	}
}

// ProtoCppStdlib converts internal CppStdlib to protocol buffer enum
func (r *NativeCppRuntime) ProtoCppStdlib() v1.CppStdlib {
	switch r.toolchain.CppStdlib {
	case CppStdlibLibstdcxx:
		return v1.CppStdlib_CPP_STDLIB_LIBSTDCXX
	case CppStdlibLibcxx:
		return v1.CppStdlib_CPP_STDLIB_LIBCXX
	default:
		return v1.CppStdlib_CPP_STDLIB_UNSPECIFIED
	}
}

// ProtoCppAbi converts internal CppAbi to protocol buffer enum
func (r *NativeCppRuntime) ProtoCppAbi() v1.CppAbi {
	switch r.toolchain.CppAbi {
	case CppAbiItanium:
		return v1.CppAbi_CPP_ABI_ITANIUM
	default:
		return v1.CppAbi_CPP_ABI_UNSPECIFIED
	}
}

// ParseVersionString parses a version string like "10.2.1" into a proto Version message
func ParseVersionString(versionStr string) *v1.Version {
	// Handle empty or unknown versions
	if versionStr == "" || versionStr == "unknown" {
		return &v1.Version{Major: 0}
	}

	pv := &v1.Version{}

	// Split by dots and dashes
	// Examples: "10.2.1", "11.0.1-2", "10.2.1-rc1"
	parts := strings.FieldsFunc(versionStr, func(r rune) bool {
		return r == '.' || r == '-'
	})

	// Parse major version
	if len(parts) > 0 {
		if major, err := strconv.ParseUint(parts[0], 10, 32); err == nil {
			pv.Major = uint32(major)
		}
	}

	// Parse minor version
	if len(parts) > 1 {
		if isNumeric(parts[1]) {
			if minor, err := strconv.ParseUint(parts[1], 10, 32); err == nil {
				m := uint32(minor)
				pv.Minor = &m
			}
		} else {
			// Non-numeric part is prerelease
			pv.Prerelease = &parts[1]
		}
	}

	// Parse patch version
	if len(parts) > 2 {
		if isNumeric(parts[2]) {
			if patch, err := strconv.ParseUint(parts[2], 10, 32); err == nil {
				p := uint32(patch)
				pv.Patch = &p
			}
		} else {
			// Non-numeric part is prerelease or metadata
			if parts[2] != "" {
				pv.Prerelease = &parts[2]
			}
		}
	}

	return pv
}

// isNumeric returns true if the string contains only digits
func isNumeric(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
