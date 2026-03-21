package native

import (
	"context"
	"os/exec"
	"runtime"
	"strings"

	"github.com/Manu343726/buildozer/internal/logger"
)

// CompilerInfo contains metadata about a detected C/C++ compiler on the system.
type CompilerInfo struct {
	// Name is the compiler name (e.g., "gcc", "clang").
	Name string
	// Path is the filesystem path to the compiler executable.
	Path string
	// Version is the version string of the compiler (e.g., "11.2.0").
	Version string
	// Architecture is the target CPU architecture that the compiler targets (e.g., "x86_64").
	Architecture string
}

// Detector finds available C/C++ compilers and toolchains on the system.
// It scans the system PATH for known compilers (gcc, clang) and extracts their metadata.
type Detector struct {
	// log is the logger for detector operations.
	log *logger.ComponentLogger
}

// NewDetector creates and returns a new C/C++ compiler detector.
func NewDetector() *Detector {
	return &Detector{
		log: logger.NewComponentLogger("cpp-native-detector"),
	}
}

// DetectCompilers scans the system to find all available C/C++ compilers.
// On non-Linux systems, it returns an empty slice with no error.
// It checks for both GCC and Clang compilers using PATH lookup.
// For each discovered compiler, it extracts version and architecture information.
func (d *Detector) DetectCompilers(ctx context.Context) ([]CompilerInfo, error) {
	if runtime.GOOS != "linux" {
		d.log.Info("native C/C++ detection only supported on Linux", "os", runtime.GOOS)
		return []CompilerInfo{}, nil
	}

	var compilers []CompilerInfo

	// Check for GCC
	if path, err := exec.LookPath("gcc"); err == nil {
		version := d.getCompilerVersion(ctx, path)
		arch := d.detectArchitecture(ctx, path)
		compilers = append(compilers, CompilerInfo{
			Name:         "gcc",
			Path:         path,
			Version:      version,
			Architecture: arch,
		})
	}

	// Check for Clang
	if path, err := exec.LookPath("clang"); err == nil {
		version := d.getCompilerVersion(ctx, path)
		arch := d.detectArchitecture(ctx, path)
		compilers = append(compilers, CompilerInfo{
			Name:         "clang",
			Path:         path,
			Version:      version,
			Architecture: arch,
		})
	}

	if len(compilers) > 0 {
		d.log.Info("detected compilers", "count", len(compilers))
	}

	return compilers, nil
}

// getCompilerVersion queries a compiler for its version by running "<compiler> --version"
// and parsing the output. It extracts the first version-like string (containing dots).
// If version detection fails, it returns "unknown".
func (d *Detector) getCompilerVersion(ctx context.Context, compilerPath string) string {
	cmd := exec.CommandContext(ctx, compilerPath, "--version")
	output, err := cmd.Output()
	if err != nil {
		return "unknown"
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) == 0 {
		return "unknown"
	}

	// Simple heuristic: look for a version pattern (string containing dots)
	firstLine := lines[0]
	parts := strings.Fields(firstLine)
	for _, part := range parts {
		if strings.Contains(part, ".") {
			return part
		}
	}

	return "unknown"
}

// detectArchitecture determines the target CPU architecture based on the Go runtime GOARCH.
// This is a simplification that assumes the compiler targets the same architecture as the
// Go runtime it's running on. Future improvements could query the compiler directly.
func (d *Detector) detectArchitecture(ctx context.Context, compilerPath string) string {
	switch runtime.GOARCH {
	case "amd64":
		return "x86_64"
	case "arm64":
		return "aarch64"
	case "arm":
		return "arm"
	default:
		return "unknown"
	}
}
