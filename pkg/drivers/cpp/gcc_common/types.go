// Package gcc_common provides shared types and utilities for GCC/G++ drivers
package gcc_common

import (
	"github.com/Manu343726/buildozer/pkg/config"
)

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
	Libraries     []string // Libraries are the -l libraries to link against
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

// BuildContext holds the execution context for GCC/G++ drivers
type BuildContext struct {
	Config     *config.Config
	Standalone bool
	DaemonHost string // Buildozer daemon hostname or IP address
	DaemonPort int    // Buildozer daemon port number
	StartDir   string
	LogLevel   string // Log level: debug, info, warn, error
	ConfigPath string // Explicit path to .buildozer config file
}
