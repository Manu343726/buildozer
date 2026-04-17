package native

// Language represents the C/C++ language variant.
type Language string

const (
	// LanguageUnspecified indicates an unspecified language.
	LanguageUnspecified Language = "unspecified"
	// LanguageC represents the C programming language.
	LanguageC Language = "c"
	// LanguageCpp represents the C++ programming language.
	LanguageCpp Language = "cpp"
)

func (l Language) String() string {
	return string(l)
}

// Compiler represents the C/C++ compiler implementation.
type Compiler string

const (
	// CompilerUnspecified indicates an unspecified compiler.
	CompilerUnspecified Compiler = "unspecified"
	// CompilerGCC represents the GNU Compiler Collection (GCC).
	CompilerGCC Compiler = "gcc"
	// CompilerClang represents the Clang/LLVM compiler.
	CompilerClang Compiler = "clang"
)

func (c Compiler) String() string {
	return string(c)
}

// Architecture represents the target CPU architecture that the compiler targets.
type Architecture string

const (
	// ArchitectureUnspecified indicates an unspecified architecture.
	ArchitectureUnspecified Architecture = "unspecified"
	// ArchitectureX86_64 represents 64-bit Intel/AMD x86 architecture.
	ArchitectureX86_64 Architecture = "x86_64"
	// ArchitectureAArch64 represents 64-bit ARM (ARMv8) architecture.
	ArchitectureAArch64 Architecture = "aarch64"
	// ArchitectureARM represents 32-bit ARM architecture.
	ArchitectureARM Architecture = "arm"
)

func (a Architecture) String() string {
	return string(a)
}

// CRuntime represents the C standard library implementation.
type CRuntime string

const (
	// CRuntimeUnspecified indicates an unspecified C runtime.
	CRuntimeUnspecified CRuntime = "unspecified"
	// CRuntimeGlibc represents the GNU C Library (glibc), standard on Linux.
	CRuntimeGlibc CRuntime = "glibc"
	// CRuntimeMusl represents the musl C library, used in Alpine Linux and embedded systems.
	CRuntimeMusl CRuntime = "musl"
)

func (c CRuntime) String() string {
	return string(c)
}

// CppStdlib represents the C++ standard library implementation.
type CppStdlib string

const (
	// CppStdlibUnspecified indicates an unspecified C++ standard library.
	CppStdlibUnspecified CppStdlib = "unspecified"
	// CppStdlibLibstdcxx represents GCC's libstdc++ standard library.
	CppStdlibLibstdcxx CppStdlib = "libstdc++"
	// CppStdlibLibcxx represents LLVM/Clang's libc++ standard library.
	CppStdlibLibcxx CppStdlib = "libc++"
)

func (c CppStdlib) String() string {
	return string(c)
}

// CppAbi represents the C++ ABI (Application Binary Interface) specification.
type CppAbi string

const (
	// CppAbiUnspecified indicates an unspecified C++ ABI.
	CppAbiUnspecified CppAbi = "unspecified"
	// CppAbiItanium represents the Itanium C++ ABI (used on UNIX-like systems by GCC and Clang).
	CppAbiItanium CppAbi = "itanium"
)

func (a CppAbi) String() string {
	return string(a)
}

// Toolchain represents a complete C/C++ compilation environment specifying the compiler,
// target architecture, and runtime libraries.
type Toolchain struct {
	// Language is the C/C++ language variant (C or C++).
	Language Language
	// Compiler is the compiler implementation (GCC or Clang).
	Compiler Compiler
	// CompilerPath is the filesystem path to the compiler executable.
	CompilerPath string
	// ArchiverPath is the filesystem path to the archiver executable (e.g., ar).
	ArchiverPath string
	// CompilerVersion is the version string of the compiler (e.g., "11.2.0").
	CompilerVersion string
	// Architecture is the target CPU architecture.
	Architecture Architecture
	// CRuntime is the C standard library implementation.
	CRuntime CRuntime
	// CRuntimeVersion is the version string of the C runtime (e.g., "2.31").
	CRuntimeVersion string
	// CppAbi is the C++ ABI specification.
	CppAbi CppAbi
	// CppStdlib is the C++ standard library implementation.
	CppStdlib CppStdlib
	// AbiModifiers is a list of compiler-specific flags that modify the ABI.
	// For example, gcc C++11 ABI is enabled with the flag "-D_GLIBCXX_USE_CXX11_ABI=1".
	AbiModifiers []string
}

// CompileJob specifies a C/C++ source compilation operation.
type CompileJob struct {
	// SourceFiles is a list of source files to compile.
	SourceFiles []string
	// IncludeDirs is a list of directories to search for include files.
	IncludeDirs []string
	// Defines is a map of preprocessor defines and their values (e.g., "DEBUG" -> "1").
	Defines map[string]string
	// CompilerFlags is a list of additional compiler command-line flags.
	CompilerFlags []string
	// OutputFile is the path where the compiled object file will be written.
	OutputFile string
}

// LinkJob specifies a C/C++ linking operation to produce an executable or library.
type LinkJob struct {
	// ObjectFiles is a list of object files to link.
	ObjectFiles []string
	// LibraryFiles is a list of full-path library files to link (e.g., "lib/libmath.a").
	LibraryFiles []string
	// Libraries is a list of library names to link against (e.g., "m", "pthread").
	Libraries []string
	// LinkerFlags is a list of additional linker command-line flags.
	LinkerFlags []string
	// OutputFile is the path where the linked executable or library will be written.
	OutputFile string
	// SharedLibrary indicates whether to produce a shared library instead of an executable.
	SharedLibrary bool
}

// ArchiveJob specifies a C/C++ static library archive operation.
type ArchiveJob struct {
	// InputFiles is a list of object files to add to the archive.
	InputFiles []string
	// ArFlags is a list of ar command flags (e.g., "r", "u", "c", "v").
	ArFlags []string
	// OutputFile is the path where the archive file (.a) will be written.
	OutputFile string
}

// ExecutionOutput contains the result of executing a compilation or linking job.
type ExecutionOutput struct {
	// ExitCode is the exit code returned by the compiler or linker (0 for success).
	ExitCode int
	// Stdout contains the standard output from the compiler or linker.
	Stdout []byte
	// Stderr contains the standard error output from the compiler or linker.
	Stderr []byte
}
