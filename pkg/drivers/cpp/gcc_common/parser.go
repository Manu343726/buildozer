package gcc_common

import (
	"path/filepath"
	"strings"
)

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
		LibraryFiles:  []string{},
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
				i++
				parsed.IncludeDirs = append(parsed.IncludeDirs, args[i])
			}
		case strings.HasPrefix(arg, "-D"):
			// Define
			if len(arg) > 2 {
				parsed.Defines = append(parsed.Defines, arg[2:])
			} else if i+1 < len(args) {
				i++
				parsed.Defines = append(parsed.Defines, args[i])
			}
		case strings.HasPrefix(arg, "-l"):
			// Library
			if len(arg) > 2 {
				parsed.Libraries = append(parsed.Libraries, arg[2:])
			} else if i+1 < len(args) {
				i++
				parsed.Libraries = append(parsed.Libraries, args[i])
			}
		case strings.HasPrefix(arg, "-L"):
			// Library directory
			if len(arg) > 2 {
				parsed.LibraryDirs = append(parsed.LibraryDirs, arg[2:])
			} else if i+1 < len(args) {
				i++
				parsed.LibraryDirs = append(parsed.LibraryDirs, args[i])
			}
		case arg == "--version", arg == "-v", arg == "-print-version":
			// Version flags - handled specially by driver
			parsed.CompilerFlags = append(parsed.CompilerFlags, arg)
		case strings.HasPrefix(arg, "-"):
			// Other flags - determine if compiler or linker
			if isCompilerOnlyFlag(arg) {
				parsed.CompilerFlags = append(parsed.CompilerFlags, arg)
			} else {
				parsed.LinkerFlags = append(parsed.LinkerFlags, arg)
			}
		default:
			// Input file (source, object, or library)
			if strings.HasSuffix(arg, ".c") || strings.HasSuffix(arg, ".cpp") ||
				strings.HasSuffix(arg, ".cc") || strings.HasSuffix(arg, ".cxx") ||
				strings.HasSuffix(arg, ".C") || strings.HasSuffix(arg, ".c++") {
				// Source file
				parsed.SourceFiles = append(parsed.SourceFiles, arg)
			} else if strings.HasSuffix(arg, ".o") {
				// Object file
				parsed.ObjectFiles = append(parsed.ObjectFiles, arg)
			} else if strings.HasSuffix(arg, ".a") || strings.HasSuffix(arg, ".so") ||
				strings.HasSuffix(arg, ".lib") || strings.HasSuffix(arg, ".dll") {
				// Library file with full path - add to LibraryFiles, not Libraries
				// Libraries contains named libraries from -l flags
				parsed.LibraryFiles = append(parsed.LibraryFiles, arg)
			}
		}
	}

	// Post-process to detect link-only operations
	// If we have no source files but have object files, and no -c flag, this is a link-only job
	if parsed.Mode != ModeCompileOnly && // Not explicitly compile-only
		len(parsed.SourceFiles) == 0 && // No source files
		len(parsed.ObjectFiles) > 0 { // But we have object files
		parsed.Mode = ModeLink
	}

	return parsed
}

// isCompilerOnlyFlag returns true if the flag is compiler-only (not linker)
func isCompilerOnlyFlag(flag string) bool {
	compilerOnlyFlags := map[string]bool{
		"-c":           true,
		"-S":           true,
		"-E":           true,
		"-M":           true,
		"-MM":          true,
		"-fPIC":        true,
		"-fPIE":        true,
		"-fno-PIC":     true,
		"-fno-PIE":     true,
		"-nostdinc":    true,
		"-nostdinc++":  true,
		"-trigraphs":   true,
		"-notrigraphs": true,
	}

	// Check exact match
	if compilerOnlyFlags[flag] {
		return true
	}

	// Check prefix matches for compiler-specific flags
	if strings.HasPrefix(flag, "-std=") ||
		strings.HasPrefix(flag, "-march=") ||
		strings.HasPrefix(flag, "-mtune=") ||
		strings.HasPrefix(flag, "-m64") ||
		strings.HasPrefix(flag, "-m32") ||
		strings.HasPrefix(flag, "-O") ||
		strings.HasPrefix(flag, "-g") ||
		strings.HasPrefix(flag, "-W") {
		return true
	}

	return false
}

// DetectLanguage determines if this is a C or C++ compilation based on source file extensions.
// Returns true for C++, false for C.
func DetectLanguage(sourceFiles []string) bool {
	for _, file := range sourceFiles {
		ext := filepath.Ext(file)
		switch ext {
		case ".cpp", ".cc", ".cxx", ".c++":
			return true // C++
		}
	}
	return false // Default to C
}
