package gcc_common

import (
	"fmt"
	"strings"

	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	pkgruntime "github.com/Manu343726/buildozer/pkg/runtime"
	"google.golang.org/protobuf/proto"
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

// ModifyRuntimeWithFlags takes a base runtime descriptor and compiler flag details,
// and returns a modified runtime descriptor with the extracted flag values applied.
// baseRuntime is guaranteed to be non-nil (checked by RuntimeResolver).
// Returns an error only if the runtime is not a C/C++ runtime.
func ModifyRuntimeWithFlags(baseRuntime *v1.Runtime, flags *CompilerFlagDetails) (*v1.Runtime, error) {
	cpp := baseRuntime.GetCpp()
	if cpp == nil {
		return nil, fmt.Errorf("not a C/C++ runtime: %s", baseRuntime.GetId())
	}

	// If no flags affect the runtime, return as-is
	if flags.Version == "" && flags.Architecture == "" && flags.StdLib == "" {
		return baseRuntime, nil
	}

	// Clone the proto so we don't mutate the original
	modified := proto.Clone(baseRuntime).(*v1.Runtime)
	modCpp := modified.GetCpp()

	if flags.Version != "" {
		modCpp.CompilerVersion = pkgruntime.ParseVersionString(flags.Version)
	}
	if flags.Architecture != "" {
		modCpp.Architecture = pkgruntime.ParseArchitectureString(flags.Architecture)
	}
	if flags.StdLib != "" {
		modCpp.CppStdlib = pkgruntime.ParseStdlibString(flags.StdLib)
	}

	// Regenerate the ID from updated fields
	modified.Id = pkgruntime.RuntimeToID(modified)

	return modified, nil
}
