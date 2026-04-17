package native

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	"github.com/Manu343726/buildozer/pkg/logging"
	"github.com/Manu343726/buildozer/pkg/runtime"
)

// NativeCppRuntime implements the runtime.Runtime interface for native C/C++ compilation.
// It provides job execution capabilities by delegating to an Executor with a concrete Toolchain configuration.
// This type acts as a bridge between the generic Runtime interface and concrete C/C++ compilation operations.
type NativeCppRuntime struct {
	*logging.Logger
	// toolchain contains the specific C/C++ compiler configuration (compiler type, version, architecture, etc.).
	toolchain *Toolchain
	// executor handles the actual compilation and linking operations.
	executor *Executor
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
		Logger:    Log().Child(fmt.Sprintf("cpp-native-%s", compilerName)),
		toolchain: toolchain,
		executor:  NewExecutor(toolchain.CompilerPath, toolchain.ArchiverPath, workDir),
	}
}

// Execute executes a C/C++ job according to the runtime.Runtime interface.
// The C++ runtime only works with reference data (no embedded content).
// It expects inputs to be JobDataReference without embedded content, and produces outputs as references.
// This avoids unnecessary copies when using the runtime.
func (r *NativeCppRuntime) Execute(ctx context.Context, req *runtime.ExecutionRequest) (*runtime.ExecutionResult, error) {
	// Validate that all inputs are references (no embedded content)
	if err := r.validateInputsAreReferences(req); err != nil {
		return nil, err
	}

	return r.execute(ctx, req, r.executor, "")
}

// validateInputsAreReferences ensures all inputs are job data references without embedded content.
// The C++ runtime only accepts reference data to avoid copies.
func (r *NativeCppRuntime) validateInputsAreReferences(req *runtime.ExecutionRequest) error {
	if req == nil || req.FullJob == nil {
		return nil // No inputs to validate
	}

	for i, input := range req.FullJob.Inputs {
		if input == nil {
			continue
		}

		// Check that the input is a reference (JobDataReference)
		_, isReference := input.Data.(*v1.JobData_Reference)
		if !isReference {
			return r.Errorf("input[%d] must be a reference (JobDataReference), got %T: C++ runtime only accepts reference data to avoid copies", i, input.Data)
		}
	}

	return nil
}

func (r *NativeCppRuntime) execute(ctx context.Context, req *runtime.ExecutionRequest, executor *Executor, workDir string) (*runtime.ExecutionResult, error) {
	if req == nil {
		return nil, r.Errorf("execution request is nil")
	}

	if req.FullJob == nil {
		return nil, r.Errorf("full job is nil")
	}

	// Extract the job spec from the oneof wrapper and execute
	var execResult *runtime.ExecutionResult
	var err error

	switch spec := req.FullJob.JobSpec.(type) {
	case *v1.Job_CppCompile:
		if spec.CppCompile == nil {
			return nil, r.Errorf("cpp compile job is nil")
		}

		// Ensure output directories exist for compile job
		if err := r.ensureOutputDirectories(ctx, spec.CppCompile, workDir); err != nil {
			return nil, err
		}

		r.Info("executing C/C++ compile job", "sources", len(spec.CppCompile.SourceFiles), "output", spec.CppCompile.OutputFile)

		// Debug log detailed compile job information
		r.Debug("Compile job details",
			"sourceFileCount", len(spec.CppCompile.SourceFiles),
			"includeCount", len(spec.CppCompile.IncludeDirs),
			"defineCount", len(spec.CppCompile.Defines),
			"compilerArgCount", len(spec.CppCompile.CompilerArgs),
			"outputFile", spec.CppCompile.OutputFile,
		)
		if len(spec.CppCompile.SourceFiles) > 0 {
			r.Debug("Compile job source files", "files", spec.CppCompile.SourceFiles)
		}
		if len(spec.CppCompile.IncludeDirs) > 0 {
			r.Debug("Compile job include directories", "dirs", spec.CppCompile.IncludeDirs)
		}
		if len(spec.CppCompile.Defines) > 0 {
			r.Debug("Compile job preprocessor defines", "defines", spec.CppCompile.Defines)
		}
		if len(spec.CppCompile.CompilerArgs) > 0 {
			r.Debug("Compile job compiler arguments", "args", spec.CppCompile.CompilerArgs)
		}

		compileJob := r.protoCompileJobToConcrete(spec.CppCompile)
		execResult, err = executor.ExecuteCompileJob(ctx, compileJob, req.ProgressCallback)
		if err != nil {
			return nil, err
		}

	case *v1.Job_CppLink:
		if spec.CppLink == nil {
			return nil, r.Errorf("cpp link job is nil")
		}

		// Ensure output directories exist for link job
		if err := r.ensureOutputDirectories(ctx, spec.CppLink, workDir); err != nil {
			return nil, err
		}

		r.Info("executing C/C++ link job", "objects", len(spec.CppLink.ObjectFiles), "output", spec.CppLink.OutputFile)

		// Debug log detailed link job information
		r.Debug("Link job details",
			"objectFileCount", len(spec.CppLink.ObjectFiles),
			"libraryFileCount", len(spec.CppLink.LibraryFiles),
			"libraryCount", len(spec.CppLink.Libraries),
			"libDirCount", len(spec.CppLink.LibraryDirs),
			"linkerArgCount", len(spec.CppLink.LinkerArgs),
			"isSharedLibrary", spec.CppLink.IsSharedLibrary,
			"outputFile", spec.CppLink.OutputFile,
		)
		if len(spec.CppLink.ObjectFiles) > 0 {
			r.Debug("Link job object files", "files", spec.CppLink.ObjectFiles)
		}
		if len(spec.CppLink.LibraryFiles) > 0 {
			r.Debug("Link job library files (full paths)", "files", spec.CppLink.LibraryFiles)
		}
		if len(spec.CppLink.Libraries) > 0 {
			r.Debug("Link job named libraries (-l flags)", "libs", spec.CppLink.Libraries)
		}
		if len(spec.CppLink.LibraryDirs) > 0 {
			r.Debug("Link job library directories", "dirs", spec.CppLink.LibraryDirs)
		}
		if len(spec.CppLink.LinkerArgs) > 0 {
			r.Debug("Link job linker arguments", "args", spec.CppLink.LinkerArgs)
		}

		linkJob := r.protoLinkJobToConcrete(spec.CppLink)
		execResult, err = executor.ExecuteLinkJob(ctx, linkJob, req.ProgressCallback)
		if err != nil {
			return nil, err
		}

	case *v1.Job_CppArchive:
		if spec.CppArchive == nil {
			return nil, r.Errorf("cpp archive job is nil")
		}

		// Ensure output directories exist for archive job
		if err := r.ensureOutputDirectories(ctx, spec.CppArchive, workDir); err != nil {
			return nil, err
		}

		r.Info("executing C/C++ archive job", "inputs", len(spec.CppArchive.InputFiles), "output", spec.CppArchive.OutputFile)

		// Debug log detailed archive job information
		r.Debug("Archive job details",
			"inputFileCount", len(spec.CppArchive.InputFiles),
			"arFlagCount", len(spec.CppArchive.ArFlags),
			"outputFile", spec.CppArchive.OutputFile,
		)
		if len(spec.CppArchive.InputFiles) > 0 {
			r.Debug("Archive job input files", "files", spec.CppArchive.InputFiles)
		}
		if len(spec.CppArchive.ArFlags) > 0 {
			r.Debug("Archive job ar flags (tool arguments)", "args", spec.CppArchive.ArFlags)
		}

		archiveJob := r.protoArchiveJobToConcrete(spec.CppArchive)
		execResult, err = executor.ExecuteArchiveJob(ctx, archiveJob, req.ProgressCallback)
		if err != nil {
			return nil, err
		}

	default:
		return nil, r.Errorf("unsupported job type: %T", req.FullJob.JobSpec)
	}

	// Executor has already collected output files and populated ExecutionResult.Output
	// Just set the ExecutionID and return directly
	if execResult != nil {
		execResult.ExecutionID = r.RuntimeID()
	}

	return execResult, nil
}

// Available checks whether this runtime is available and functional.
// It attempts to run the compiler with the --version flag to verify the compiler exists and is executable.
// Returns true if the compiler is available, false otherwise.
// An error is only returned if there's a fatal system error; compiler unavailability is not an error.
func (r *NativeCppRuntime) Available(ctx context.Context) (bool, error) {
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
		RuntimeType: fmt.Sprintf("native_linux-%s", compilerName),
		Language:    langStr,
		Version:     r.toolchain.CompilerVersion,
		TargetOS:    "linux",
		TargetArch:  string(r.toolchain.Architecture),
		IsNative:    true,
		Description: fmt.Sprintf("Native linux %s %s (%s)", compilerName, r.toolchain.CompilerVersion, r.toolchain.Architecture),
		Details:     fmt.Sprintf("path=%s", r.toolchain.CompilerPath),
	}, nil
}

// RuntimeID returns a unique identifier for this runtime instance.
// Format: native_linux-<language>-<compiler>-<version>-<cruntime>-<cruntimeVersion>-[<stdlib>-]<arch>
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
//	native_linux-c-gcc-10.2.1-glibc-2.31-x86_64 (C with GCC)
//	native_linux-cpp-gcc-10.2.1-glibc-2.31-libstdc++-x86_64 (C++ with GCC)
//	native_linux-cpp-clang-11.0.1-glibc-2.31-libc++-x86_64 (C++ with Clang)
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

	// TODO: detect platform
	platform := "native_linux"

	// Build the ID: native-<language>-<compiler>-<version>-<cruntime>-<cruntimeVersion>
	id := fmt.Sprintf("%s-%s-%s-%s-%s-%s",
		platform,
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

// ensureOutputDirectories creates parent directories for all output files specified in the job.
// This ensures that compilers can write output files to subdirectories without failing
// on "No such file or directory" errors.
// Supports CppCompileJob, CppLinkJob, and CppArchiveJob output files.
func (r *NativeCppRuntime) ensureOutputDirectories(ctx context.Context, job interface{}, workDir string) error {
	switch j := job.(type) {
	case *v1.CppCompileJob:
		if j != nil && j.OutputFile != "" {
			outputPath := j.OutputFile
			if !filepath.IsAbs(outputPath) {
				outputPath = filepath.Join(workDir, outputPath)
			}
			outputDir := filepath.Dir(outputPath)
			if err := os.MkdirAll(outputDir, 0755); err != nil {
				return r.Errorf("failed to create output directory %s: %w", outputDir, err)
			}
		}
	case *v1.CppLinkJob:
		if j != nil && j.OutputFile != "" {
			outputPath := j.OutputFile
			if !filepath.IsAbs(outputPath) {
				outputPath = filepath.Join(workDir, outputPath)
			}
			outputDir := filepath.Dir(outputPath)
			if err := os.MkdirAll(outputDir, 0755); err != nil {
				return r.Errorf("failed to create output directory %s: %w", outputDir, err)
			}
		}
	case *v1.CppArchiveJob:
		if j != nil && j.OutputFile != "" {
			outputPath := j.OutputFile
			if !filepath.IsAbs(outputPath) {
				outputPath = filepath.Join(workDir, outputPath)
			}
			outputDir := filepath.Dir(outputPath)
			if err := os.MkdirAll(outputDir, 0755); err != nil {
				return r.Errorf("failed to create output directory %s: %w", outputDir, err)
			}
		}
	}
	return nil
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
		LibraryFiles:  proto.LibraryFiles,
		Libraries:     proto.Libraries,
		LinkerFlags:   proto.LinkerArgs,
		OutputFile:    proto.OutputFile,
		SharedLibrary: proto.IsSharedLibrary,
	}
}

// protoArchiveJobToConcrete converts a protocol buffer CppArchiveJob to a concrete ArchiveJob.
// This method translates between the protocol representation and the internal representation.
func (r *NativeCppRuntime) protoArchiveJobToConcrete(proto *v1.CppArchiveJob) *ArchiveJob {
	return &ArchiveJob{
		InputFiles: proto.InputFiles,
		ArFlags:    proto.ArFlags,
		OutputFile: proto.OutputFile,
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

func (r *NativeCppRuntime) ProtoCompilerVersion() *v1.Version {
	return ParseVersionString(r.toolchain.CompilerVersion)
}

func (r *NativeCppRuntime) ProtoCRuntimeVersion() *v1.Version {
	return ParseVersionString(r.toolchain.CRuntimeVersion)
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

func (r *NativeCppRuntime) ProtoCppToolchain() v1.RuntimeToolchain {
	switch r.toolchain.Language {
	case LanguageC:
		return v1.RuntimeToolchain_RUNTIME_TOOLCHAIN_C
	case LanguageCpp:
		return v1.RuntimeToolchain_RUNTIME_TOOLCHAIN_CPP
	default:
		return v1.RuntimeToolchain_RUNTIME_TOOLCHAIN_UNSPECIFIED
	}
}

func (r *NativeCppRuntime) Proto(ctx context.Context) (*v1.Runtime, error) {
	return &v1.Runtime{
		Id:        r.RuntimeID(),
		Platform:  v1.RuntimePlatform_RUNTIME_PLATFORM_NATIVE_LINUX,
		Toolchain: r.ProtoCppToolchain(),
		ToolchainSpec: &v1.Runtime_Cpp{
			Cpp: &v1.CppToolchain{
				Language:        r.ProtoLanguage(),
				Compiler:        r.ProtoCompiler(),
				CompilerVersion: r.ProtoCompilerVersion(),
				Architecture:    r.ProtoArchitecture(),
				CRuntime:        r.ProtoCRuntime(),
				CRuntimeVersion: r.ProtoCRuntimeVersion(),
				CppStdlib:       r.ProtoCppStdlib(),
				CppAbi:          r.ProtoCppAbi(),
				AbiModifiers:    r.toolchain.AbiModifiers,
			},
		},
	}, nil
}

func (r *NativeCppRuntime) MatchesQuery(ctx context.Context, query *v1.RuntimeMatchQuery) (bool, error) {
	if query == nil {
		return false, fmt.Errorf("runtime query is nil")
	}

	if len(query.Platforms) > 0 && !slices.Contains(query.Platforms, v1.RuntimePlatform_RUNTIME_PLATFORM_NATIVE_LINUX) {
		r.Debug("query does not match platform", "queryPlatforms", query.Platforms)
		return false, nil
	}

	if len(query.Toolchains) > 0 && !slices.Contains(query.Toolchains, r.ProtoCppToolchain()) {
		r.Debug("query does not match toolchain", "queryToolchains", query.Toolchains, "runtimeToolchain", r.ProtoCppToolchain())
		return false, nil
	}

	return r.matchesRuntimeParameters(query.Params)
}

func (r *NativeCppRuntime) matchesRuntimeParameters(params map[string]*v1.StringArray) (bool, error) {
	for key, acceptedValues := range params {
		if matches, err := r.matchesRuntimeParameter(key, acceptedValues.Values); err != nil {
			return false, err
		} else if !matches {
			return false, nil
		}
	}

	return true, nil
}

func (r *NativeCppRuntime) matchesRuntimeParameter(key string, acceptedValues []string) (bool, error) {
	switch key {
	case "compiler":
		return r.matchesCompiler(acceptedValues)
	case "compiler_version":
		return r.matchesCompilerVersion(acceptedValues)
	case "cruntime":
		fallthrough
	case "c_runtime":
		return r.matchesCRuntime(acceptedValues)
	case "cruntime_version":
		fallthrough
	case "c_runtime_version":
		return r.matchesCRuntimeVersion(acceptedValues)
	case "cpp_stdlib":
		return r.matchesCppStdlib(acceptedValues)
	case "cpp_abi":
		return r.matchesCppAbi(acceptedValues)
	case "architecture":
		return r.matchesArchitecture(acceptedValues)
	}

	return false, fmt.Errorf("unknown runtime parameter: %s", key)
}

func (r *NativeCppRuntime) matchesCompiler(acceptedValues []string) (bool, error) {
	protoCompiler := r.ProtoCompiler().String()
	compiler := r.toolchain.Compiler.String()
	matches := len(acceptedValues) == 0 || slices.Contains(acceptedValues, protoCompiler) || slices.Contains(acceptedValues, compiler)
	r.Debug("matching compiler", "protoCompiler", protoCompiler, "compiler", compiler, "acceptedValues", acceptedValues, "acceptedValuesCount", len(acceptedValues), "matches", matches)
	return matches, nil
}

func (r *NativeCppRuntime) matchesCompilerVersion(acceptedValues []string) (bool, error) {
	protoCompilerVersion := r.ProtoCompilerVersion()
	compilerVersion := r.toolchain.CompilerVersion
	matches := len(acceptedValues) == 0 || slices.Contains(acceptedValues, protoCompilerVersion.String()) || slices.Contains(acceptedValues, compilerVersion)
	r.Debug("matching compiler version", "protoCompilerVersion", protoCompilerVersion, "compilerVersion", compilerVersion, "acceptedValues", acceptedValues, "acceptedValuesCount", len(acceptedValues), "matches", matches)
	return matches, nil
}

func (r *NativeCppRuntime) matchesCRuntime(acceptedValues []string) (bool, error) {
	protoCRuntime := r.ProtoCRuntime().String()
	cruntime := r.toolchain.CRuntime.String()
	matches := len(acceptedValues) == 0 || slices.Contains(acceptedValues, protoCRuntime) || slices.Contains(acceptedValues, cruntime)
	r.Debug("matching C runtime", "protoCRuntime", protoCRuntime, "cruntime", cruntime, "acceptedValues", acceptedValues, "acceptedValuesCount", len(acceptedValues), "matches", matches)
	return matches, nil
}

func (r *NativeCppRuntime) matchesCRuntimeVersion(acceptedValues []string) (bool, error) {
	protoCRuntimeVersion := r.ProtoCRuntimeVersion()
	cruntimeVersion := r.toolchain.CRuntimeVersion
	matches := len(acceptedValues) == 0 || slices.Contains(acceptedValues, protoCRuntimeVersion.String()) || slices.Contains(acceptedValues, cruntimeVersion)
	r.Debug("matching C runtime version", "protoCRuntimeVersion", protoCRuntimeVersion, "cruntimeVersion", cruntimeVersion, "acceptedValues", acceptedValues, "acceptedValuesCount", len(acceptedValues), "matches", matches)
	return matches, nil
}

func (r *NativeCppRuntime) matchesCppStdlib(acceptedValues []string) (bool, error) {
	protoCppStdlib := r.ProtoCppStdlib().String()
	cppStdlib := r.toolchain.CppStdlib.String()
	matches := len(acceptedValues) == 0 || slices.Contains(acceptedValues, protoCppStdlib) || slices.Contains(acceptedValues, cppStdlib)
	r.Debug("matching C++ stdlib", "protoCppStdlib", protoCppStdlib, "cppStdlib", cppStdlib, "acceptedValues", acceptedValues, "acceptedValuesCount", len(acceptedValues), "matches", matches)
	return matches, nil
}

func (r *NativeCppRuntime) matchesCppAbi(acceptedValues []string) (bool, error) {
	protoCppAbi := r.ProtoCppAbi().String()
	cppAbi := r.toolchain.CppAbi.String()
	matches := len(acceptedValues) == 0 || slices.Contains(acceptedValues, protoCppAbi) || slices.Contains(acceptedValues, cppAbi)
	r.Debug("matching C++ ABI", "protoCppAbi", protoCppAbi, "cppAbi", cppAbi, "acceptedValues", acceptedValues, "acceptedValuesCount", len(acceptedValues), "matches", matches)
	return matches, nil
}

func (r *NativeCppRuntime) matchesArchitecture(acceptedValues []string) (bool, error) {
	protoArchitecture := r.ProtoArchitecture().String()
	architecture := r.toolchain.Architecture.String()
	matches := len(acceptedValues) == 0 || slices.Contains(acceptedValues, protoArchitecture) || slices.Contains(acceptedValues, architecture)
	r.Debug("matching architecture", "protoArchitecture", protoArchitecture, "architecture", architecture, "acceptedValues", acceptedValues, "acceptedValuesCount", len(acceptedValues), "matches", matches)
	return matches, nil
}

// Close releases any resources held by this runtime.
// For the native C/C++ runtime, this is a no-op as there are no persistent resources.
func (r *NativeCppRuntime) Close() error {
	return nil
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
