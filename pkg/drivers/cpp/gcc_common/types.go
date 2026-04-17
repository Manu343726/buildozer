// Package gcc_common provides shared types and utilities for GCC/G++ drivers
package gcc_common

// CompileMode represents whether the command is compile-only, link-only, or both
type CompileMode int

const (
	ModeUnknown        CompileMode = iota // ModeUnknown indicates mode could not be determined
	ModeCompileOnly                       // ModeCompileOnly means -c flag is present (no linking)
	ModeLink                              // ModeLink means linking without intermediate compilation
	ModeCompileAndLink                    // ModeCompileAndLink is the default
)

// ParsedArgs represents parsed command-line arguments for gcc/g++
type ParsedArgs struct {
	// Compilation/linking inputs
	SourceFiles []string // SourceFiles are the input .c or .cpp files
	ObjectFiles []string // ObjectFiles are the input .o files
	OutputFile  string   // OutputFile is the -o value

	// Compilation/linking flags
	Defines       []string // Defines are the -D macro definitions
	IncludeDirs   []string // IncludeDirs are the -I directories
	Libraries     []string // Libraries are the -l (named) libraries to link against (e.g., "m", "pthread")
	LibraryFiles  []string // LibraryFiles are full-path library files (e.g., "lib/libmath.a")
	LibraryDirs   []string // LibraryDirs are the -L search directories
	CompilerFlags []string // CompilerFlags are other compiler-specific flags
	LinkerFlags   []string // LinkerFlags are linker-specific flags

	// Mode indicates compile-only, link-only, or both
	Mode CompileMode

	// IsSharedLibrary indicates -shared flag (for creating shared libraries)
	IsSharedLibrary bool

	// OriginalArgs stores the original command-line arguments
	OriginalArgs []string
}
