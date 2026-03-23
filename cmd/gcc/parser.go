// Package main provides shared utilities for C/C++ driver command-line parsing.
package main

import (
	"path/filepath"
	"strings"
)

// CompileMode represents whether the command is compile-only, link-only, or both.
type CompileMode int

const (
	// ModeUnknown indicates mode could not be determined.
	ModeUnknown CompileMode = iota
	// ModeCompileOnly means -c flag is present (no linking).
	ModeCompileOnly
	// ModeLink means linking without intermediate compilation (rare in driver usage).
	ModeLink
	// ModeCompileAndLink is the default (both compile and link).
	ModeCompileAndLink
)

// ParsedArgs represents parsed command-line arguments for gcc/g++.
type ParsedArgs struct {
	// Mode indicates compile-only, link-only, or both.
	Mode CompileMode

	// SourceFiles are the input .c or .cpp files.
	SourceFiles []string

	// ObjectFiles are the input .o files (for linking).
	ObjectFiles []string

	// OutputFile is the -o value (output executable or object file).
	OutputFile string

	// IncludeDirs are the -I directories.
	IncludeDirs []string

	// Defines are the -D macro definitions.
	Defines []string

	// Libraries are the -l libraries to link against.
	Libraries []string

	// LibraryDirs are the -L search directories.
	LibraryDirs []string

	// CompilerFlags are other compiler-specific flags preserved as-is.
	CompilerFlags []string

	// LinkerFlags are other linker-specific flags preserved as-is.
	LinkerFlags []string

	// IsSharedLibrary indicates -shared flag (for g++ driver).
	IsSharedLibrary bool

	// OriginalArgs are the raw command-line arguments for debugging.
	OriginalArgs []string
}

// ParseCommandLine parses gcc/g++ command-line arguments.
// It extracts source files, output file, include directories, defines, libraries, and flags.
func ParseCommandLine(args []string) *ParsedArgs {
	parsed := &ParsedArgs{
		Mode:          ModeCompileAndLink,
		SourceFiles:   []string{},
		ObjectFiles:   []string{},
		IncludeDirs:   []string{},
		Defines:       []string{},
		Libraries:     []string{},
		LibraryDirs:   []string{},
		CompilerFlags: []string{},
		LinkerFlags:   []string{},
		OriginalArgs:  args,
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]

		switch {
		case arg == "-c":
			// Compile-only mode
			parsed.Mode = ModeCompileOnly

		case arg == "-shared":
			// Shared library flag
			parsed.IsSharedLibrary = true
			parsed.LinkerFlags = append(parsed.LinkerFlags, arg)

		case arg == "-o":
			// Output file (next argument)
			if i+1 < len(args) {
				parsed.OutputFile = args[i+1]
				i++
			}

		case strings.HasPrefix(arg, "-I"):
			// Include directory
			if len(arg) > 2 {
				parsed.IncludeDirs = append(parsed.IncludeDirs, arg[2:])
			} else if i+1 < len(args) {
				parsed.IncludeDirs = append(parsed.IncludeDirs, args[i+1])
				i++
			}

		case strings.HasPrefix(arg, "-D"):
			// Define
			if len(arg) > 2 {
				parsed.Defines = append(parsed.Defines, arg[2:])
			} else if i+1 < len(args) {
				parsed.Defines = append(parsed.Defines, args[i+1])
				i++
			}

		case strings.HasPrefix(arg, "-l"):
			// Library
			if len(arg) > 2 {
				parsed.Libraries = append(parsed.Libraries, arg[2:])
			} else if i+1 < len(args) {
				parsed.Libraries = append(parsed.Libraries, args[i+1])
				i++
			}

		case strings.HasPrefix(arg, "-L"):
			// Library directory
			if len(arg) > 2 {
				parsed.LibraryDirs = append(parsed.LibraryDirs, arg[2:])
			} else if i+1 < len(args) {
				parsed.LibraryDirs = append(parsed.LibraryDirs, args[i+1])
				i++
			}

		case arg == "--version", arg == "-v", arg == "-print-version":
			// Version flags - handled specially by driver
			parsed.CompilerFlags = append(parsed.CompilerFlags, arg)

		case strings.HasPrefix(arg, "-"):
			// Other flags (compiler or linker)
			// These are preserved as-is
			if isCompilerOnlyFlag(arg) {
				parsed.CompilerFlags = append(parsed.CompilerFlags, arg)
			} else {
				// Linker-related flag
				parsed.LinkerFlags = append(parsed.LinkerFlags, arg)
			}

		default:
			// Input file (source or object)
			if strings.HasSuffix(arg, ".c") || strings.HasSuffix(arg, ".cpp") ||
				strings.HasSuffix(arg, ".cc") || strings.HasSuffix(arg, ".cxx") ||
				strings.HasSuffix(arg, ".C") || strings.HasSuffix(arg, ".c++") {
				parsed.SourceFiles = append(parsed.SourceFiles, arg)
			} else if strings.HasSuffix(arg, ".o") {
				parsed.ObjectFiles = append(parsed.ObjectFiles, arg)
			}
		}
	}

	return parsed
}

// isCompilerOnlyFlag determines if a flag is compiler-specific (not linker-related).
func isCompilerOnlyFlag(flag string) bool {
	// -Wl is linker pass-through, not a compiler flag
	if strings.HasPrefix(flag, "-Wl,") || flag == "-Wl" {
		return false
	}

	compilerFlags := map[string]bool{
		"-O0": true, "-O1": true, "-O2": true, "-O3": true, "-Os": true,
		"-Wall": true, "-Wextra": true, "-Werror": true, "-Wno-all": true,
		"-pedantic": true, "-std": true, "-stdlib": true,
		"-fPIC": true, "-fPIE": true, "-fno-exceptions": true,
		"-finline-functions": true, "-foptimize-sibling-calls": true,
		"-g": true, "-ggdb": true, "-gstabs": true,
		"-E": true, "-M": true, "-MM": true, "-fsyntax-only": true,
		"-fvisibility": true, "-fvisibility-inlines-hidden": true,
		"-nostdinc": true, "-nostdinc++": true,
		"-B": true, "-V": true,
	}

	// Check exact match or prefix match for flags with values
	for cf := range compilerFlags {
		if flag == cf || strings.HasPrefix(flag, cf+"=") {
			return true
		}
	}

	// Flags starting with -f, -W (except -Wl), -std are compiler flags
	if strings.HasPrefix(flag, "-f") ||
		(strings.HasPrefix(flag, "-W") && !strings.HasPrefix(flag, "-Wl")) ||
		strings.HasPrefix(flag, "-std") || strings.HasPrefix(flag, "-stdlib") ||
		strings.HasPrefix(flag, "-isystem") || strings.HasPrefix(flag, "-isysroot") {
		return true
	}

	// Optimization flags
	if strings.HasPrefix(flag, "-O") {
		return true
	}

	// Debug flags
	if strings.HasPrefix(flag, "-g") {
		return true
	}

	return false
}

// DetectLanguage determines if this is a C or C++ compilation based on source file extensions.
func DetectLanguage(sourceFiles []string) bool {
	// Returns true for C++, false for C
	for _, file := range sourceFiles {
		ext := filepath.Ext(file)
		switch ext {
		case ".cpp", ".cc", ".cxx", ".c++":
			return true // C++
		}
	}
	return false // Default to C
}
