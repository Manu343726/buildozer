package drivers

import (
	"fmt"
	"strings"
)

// CompilerType defines which compiler driver to use
type CompilerType int

const (
	GCC CompilerType = iota
	GXX
	Clang
	ClangCxx
)

// CLIConfig holds configuration for compiler CLI drivers
type CLIConfig struct {
	Name           string       // "gcc" or "g++"
	Type           CompilerType // GCC or GXX
	SupportsStdlib bool         // True for G++
}

// ParsedCLIArgs holds parsed command-line information
type ParsedCLIArgs struct {
	Language string // From -x flag
	Standard string // From -std flag
	StdLib   string // From -stdlib flag (G++ only)
}

// ValidateAndParseArgs validates arguments and extracts runtime information
func ValidateAndParseArgs(args []string, cfg *CLIConfig) (*ParsedCLIArgs, error) {
	parsed := &ParsedCLIArgs{}

	for i := 0; i < len(args); i++ {
		arg := args[i]

		// Check if flag is valid
		if !IsValidCompilerFlag(arg, cfg.Type) {
			// Check if it looks like a file (doesn't start with -)
			if !strings.HasPrefix(arg, "-") {
				continue // Valid file argument
			}
			return nil, fmt.Errorf("unrecognized command-line option '%s'", arg)
		}

		// Extract language from -x flag
		if arg == "-x" && i+1 < len(args) {
			parsed.Language = args[i+1]
			i++
		}

		// Extract standard from -std flag
		if (arg == "-std" || strings.HasPrefix(arg, "-std=")) && i+1 < len(args) {
			if arg == "-std" {
				parsed.Standard = args[i+1]
				i++
			} else if strings.HasPrefix(arg, "-std=") {
				parsed.Standard = strings.TrimPrefix(arg, "-std=")
			}
		}

		// Extract stdlib from -stdlib flag (G++/Clang++ only)
		if (cfg.SupportsStdlib || cfg.Type == ClangCxx) && (arg == "-stdlib" || strings.HasPrefix(arg, "-stdlib=")) && i+1 < len(args) {
			if arg == "-stdlib" {
				parsed.StdLib = args[i+1]
				i++
			} else if strings.HasPrefix(arg, "-stdlib=") {
				parsed.StdLib = strings.TrimPrefix(arg, "-stdlib=")
			}
		}
	}

	return parsed, nil
}

// IsValidCompilerFlag checks if a flag is valid for the given compiler
func IsValidCompilerFlag(flag string, ct CompilerType) bool {
	// Single character flags (common to both)
	if flag == "-c" || flag == "-S" || flag == "-E" || flag == "-v" || flag == "-###" {
		return true
	}

	// Flags requiring arguments
	if flag == "-o" || flag == "-x" || flag == "-B" || flag == "-std" || flag == "-specs" ||
		flag == "-Xassembler" || flag == "-Xpreprocessor" || flag == "-Xlinker" {
		return true
	}

	// G++/Clang++ specific flags
	if (ct == GXX || ct == ClangCxx) && (flag == "-stdlib") {
		return true
	}

	// Flags with = format
	if strings.HasPrefix(flag, "-std=") || strings.HasPrefix(flag, "--sysroot=") ||
		strings.HasPrefix(flag, "-specs=") || strings.HasPrefix(flag, "-print-file-name=") ||
		strings.HasPrefix(flag, "-print-prog-name=") || strings.HasPrefix(flag, "-save-temps=") {
		return true
	}

	// G++ specific flags with =
	if ct == GXX && strings.HasPrefix(flag, "-stdlib=") {
		return true
	}

	// Common single-dash flags (both GCC and G++)
	if strings.HasPrefix(flag, "-O") || // -O0, -O1, -O2, -O3, -Os, -Oz
		strings.HasPrefix(flag, "-g") || // -g, -g0, -g1, -g2, -g3, -ggdb, etc.
		strings.HasPrefix(flag, "-f") || // -fPIC, -fno-exceptions, etc.
		strings.HasPrefix(flag, "-W") || // -Wall, -Wextra, -Werror, etc.
		strings.HasPrefix(flag, "-m") || // -march, -mtune, -m32, -m64, etc.
		strings.HasPrefix(flag, "-D") || // -DDEFINE
		strings.HasPrefix(flag, "-I") || // -I/path/to/include
		strings.HasPrefix(flag, "-L") || // -L/path/to/lib
		strings.HasPrefix(flag, "-l") || // -lm (link math library)
		strings.HasPrefix(flag, "-Wl") || // -Wl,--as-needed
		strings.HasPrefix(flag, "-Wp") || // -Wp,-D_FORTIFY_SOURCE=2
		strings.HasPrefix(flag, "-Wa") || // -Wa,--noexecstack
		strings.HasPrefix(flag, "--param") { // --param=key=value
		return true
	}

	// Long flags (common to both)
	if flag == "--version" || flag == "--help" || flag == "--target-help" ||
		flag == "-pass-exit-codes" || flag == "-dumpspecs" || flag == "-dumpversion" ||
		flag == "-dumpmachine" || flag == "-print-search-dirs" ||
		flag == "-print-libgcc-file-name" || flag == "-print-multiarch" ||
		flag == "-print-multi-directory" || flag == "-print-multi-lib" ||
		flag == "-print-multi-os-directory" || flag == "-print-sysroot" ||
		flag == "-print-sysroot-headers-suffix" || flag == "-pipe" ||
		flag == "-time" || flag == "-no-canonical-prefixes" ||
		flag == "-save-temps" || flag == "-shared" || flag == "-pie" {
		return true
	}

	// Long flags with = format (common to both)
	if strings.HasPrefix(flag, "--help=") {
		return true
	}

	return false
}
