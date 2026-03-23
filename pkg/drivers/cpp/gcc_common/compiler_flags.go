package gcc_common

import (
	"strings"
)

// CompilerFlagDetails holds extracted compiler configuration details from command-line flags
type CompilerFlagDetails struct {
	// Version is the compiler version requested via command-line (e.g., "11" from -v11)
	Version string
	// Architecture is the target architecture (e.g., "x86_64" from -march=native or -m64)
	Architecture string
	// CStandard is the C standard (e.g., "c99" from -std=c99)
	CStandard string
	// CppStandard is the C++ standard (e.g., "c++17" from -std=c++17)
	CppStandard string
	// StdLib is the C++ standard library (e.g., "libstdc++" or "libc++")
	StdLib string
	// Optimization is the optimization level (e.g., "2" from -O2)
	Optimization string
}

// ExtractCompilerFlags parses command-line arguments and extracts compiler-specific flags.
// Returns a CompilerFlagDetails struct with extracted values.
func ExtractCompilerFlags(args []string) *CompilerFlagDetails {
	details := &CompilerFlagDetails{}

	for i := 0; i < len(args); i++ {
		arg := args[i]

		// Extract compiler version: -v9, -v10, -v11
		if strings.HasPrefix(arg, "-v") && len(arg) > 2 {
			if isVersion(arg[2:]) {
				details.Version = arg[2:]
				continue
			}
		}

		// Extract architecture: -march=value, -mtune=value, -m64, -m32
		if strings.HasPrefix(arg, "-march=") {
			details.Architecture = arg[7:]
			continue
		}
		if strings.HasPrefix(arg, "-mtune=") {
			// -mtune affects CPU tuning but we map it to architecture for runtime selection
			details.Architecture = arg[7:]
			continue
		}
		if arg == "-m64" {
			details.Architecture = "x86_64"
			continue
		}
		if arg == "-m32" {
			details.Architecture = "i386"
			continue
		}
		if arg == "-march" && i+1 < len(args) {
			details.Architecture = args[i+1]
			i++
			continue
		}

		// Extract C/C++ standard: -std=c99, -std=c++17, etc.
		if strings.HasPrefix(arg, "-std=") {
			std := arg[5:]
			if strings.HasPrefix(std, "c++") {
				details.CppStandard = std
			} else if strings.HasPrefix(std, "c") {
				details.CStandard = std
			}
			continue
		}
		if arg == "-std" && i+1 < len(args) {
			std := args[i+1]
			if strings.HasPrefix(std, "c++") {
				details.CppStandard = std
			} else if strings.HasPrefix(std, "c") {
				details.CStandard = std
			}
			i++
			continue
		}

		// Extract C++ standard library: -stdlib=libc++, -stdlib=libstdc++
		if strings.HasPrefix(arg, "-stdlib=") {
			details.StdLib = arg[8:]
			continue
		}
		if arg == "-stdlib" && i+1 < len(args) {
			details.StdLib = args[i+1]
			i++
			continue
		}

		// Extract optimization level: -O0, -O1, -O2, -O3, -Os, -Oz, -Ofast
		if strings.HasPrefix(arg, "-O") && len(arg) > 2 {
			details.Optimization = arg[2:]
			continue
		}
	}

	return details
}

// isVersion checks if a string is a valid compiler version number (without leading v)
func isVersion(s string) bool {
	if len(s) == 0 {
		return false
	}
	// Accept single digit or dot-separated versions (e.g., "9", "11", "10.2")
	for i, ch := range s {
		if ch >= '0' && ch <= '9' {
			continue
		} else if ch == '.' && i > 0 && i < len(s)-1 {
			continue
		} else {
			return false
		}
	}
	return true
}

// ModifyRuntimeIDWithFlags takes a base runtime ID and compiler flag details,
// and returns a modified runtime ID that includes the extracted flag values.
// Example: "gcc-glibc-x86_64" + version "11" + arch "armv7-a" = "gcc-9-glibc-armv7-a"
func ModifyRuntimeIDWithFlags(baseRuntime string, flags *CompilerFlagDetails) string {
	if baseRuntime == "" {
		baseRuntime = "gcc-default"
	}

	// Parse the base runtime ID structure: "compiler-version-cruntime-architecture"
	// Examples: "gcc-default", "gcc-9-glibc-x86_64", "clang-10-glibc-libstdcxx-x86_64"

	// If flags specify version or architecture, inject them into the runtime ID
	if flags.Version != "" || flags.Architecture != "" {
		// Split the base runtime into parts
		parts := strings.Split(baseRuntime, "-")

		// Handle default case
		if len(parts) == 2 && parts[1] == "default" {
			// Start fresh with just the compiler
			if flags.Version != "" {
				baseRuntime = parts[0] + "-" + flags.Version
			}
			if flags.Architecture != "" {
				baseRuntime = baseRuntime + "-default-" + flags.Architecture
			}
		} else if len(parts) >= 2 {
			// Modify existing runtime ID
			if flags.Version != "" {
				// Replace version component (parts[1])
				parts[1] = flags.Version
			}
			if flags.Architecture != "" && len(parts) > 0 {
				// Replace or add architecture component (last element)
				parts[len(parts)-1] = flags.Architecture
			}
			baseRuntime = strings.Join(parts, "-")
		}
	}

	return baseRuntime
}
