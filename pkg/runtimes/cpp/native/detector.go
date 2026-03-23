package native

import (
	"context"
	"embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/Manu343726/buildozer/internal/logger"
)

//go:embed testdata/*
var testPrograms embed.FS

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

// isValidVersion checks if a string is a valid version format (contains digits and dots).
func isValidVersion(s string) bool {
	if len(s) == 0 {
		return false
	}
	hasDigit := false
	for _, r := range s {
		if (r >= '0' && r <= '9') || r == '.' || r == '-' {
			if r >= '0' && r <= '9' {
				hasDigit = true
			}
		} else {
			return false
		}
	}
	return hasDigit
}

// getCompilerVersion queries a compiler for its version by running "<compiler> --version"
// and parsing the output. It extracts the first version-like string (containing dots, dashes, and digits).
// Strips trailing parentheses and other trailing punctuation.
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

	// GCC/Clang version extraction strategy
	// Examples:
	//   gcc (Debian 10.2.1-6) 10.2.1 20210110
	//   clang version 11.0.1
	// The actual compiler version is the one WITHOUT a Debian patch suffix (no dash-digit at end)
	firstLine := lines[0]
	parts := strings.Fields(firstLine)

	var versionCandidates []string
	for _, part := range parts {
		if strings.Contains(part, ".") {
			// Remove trailing parentheses and other non-version characters
			part = strings.TrimRight(part, ")")
			// Keep only digits, dots, and dashes
			if isValidVersion(part) {
				versionCandidates = append(versionCandidates, part)
			}
		}
	}

	// Prefer versions without Debian patch suffix (no trailing dash-digit)
	// e.g., prefer "10.2.1" over "10.2.1-6"
	for _, v := range versionCandidates {
		if !strings.Contains(v, "-") {
			return v
		}
	}

	// If all have dashes, return the first one
	if len(versionCandidates) > 0 {
		return versionCandidates[0]
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

// findCompilerPaths finds all instances of a compiler name in the system PATH.
// Returns all matching executable paths, including different versions (e.g., gcc-9, gcc-10, gcc).
// Matches are sorted so that exact name matches (without version suffix) come first.
// Only matches actual compilers, not related tools (e.g., gcc-ar, clang-format).
func (d *Detector) findCompilerPaths(compilerName string) []string {
	pathEnv := os.Getenv("PATH")
	if pathEnv == "" {
		return nil
	}

	// Valid compiler patterns: gcc, gcc-11, g++, g++-11, clang, clang-11, clang++, clang++-11
	isValidCompiler := func(name string) bool {
		if name == compilerName {
			return true
		}
		// Check if it's name with version suffix (e.g., gcc-11)
		if strings.HasPrefix(name, compilerName+"-") {
			// Make sure the suffix after the dash is numeric (a version number)
			suffix := strings.TrimPrefix(name, compilerName+"-")
			// Version suffix should be numeric and not too long (e.g., 11, not a long invalid name)
			if len(suffix) > 0 && len(suffix) <= 3 && strings.TrimFunc(suffix, func(r rune) bool {
				return r >= '0' && r <= '9'
			}) == "" {
				return true
			}
		}
		return false
	}

	var results []string
	seen := make(map[string]bool)

	// Also try LookPath first to get the default PATH result
	if defaultPath, err := exec.LookPath(compilerName); err == nil {
		results = append(results, defaultPath)
		seen[defaultPath] = true
	}

	// Search each directory in PATH
	for _, dir := range strings.Split(pathEnv, string(os.PathListSeparator)) {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			continue
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			name := entry.Name()
			if !isValidCompiler(name) {
				continue
			}

			fullPath := filepath.Join(dir, name)
			if !seen[fullPath] {
				// Check if it's executable
				if fi, err := os.Stat(fullPath); err == nil && (fi.Mode()&0o111) != 0 {
					results = append(results, fullPath)
					seen[fullPath] = true
				}
			}
		}
	}

	return results
}

// DetectToolchains discovers all available C/C++ toolchains on the system.
// Returns a list of Toolchain objects representing complete compilation environments.
// This includes detection of C/C++ compilers, runtimes, standard libraries, and ABI details.
// For compilers with multiple available C++ standard libraries, a separate toolchain is created for each.
func (d *Detector) DetectToolchains(ctx context.Context) ([]Toolchain, error) {
	if runtime.GOOS != "linux" {
		d.log.Info("native C/C++ detection only supported on Linux", "os", runtime.GOOS)
		return []Toolchain{}, nil
	}

	var toolchains []Toolchain

	// Compiler specification: name, language, and compiler type
	compilers := []struct {
		name     string
		language Language
		compiler Compiler
	}{
		{"gcc", LanguageC, CompilerGCC},
		{"g++", LanguageCpp, CompilerGCC},
		{"clang", LanguageC, CompilerClang},
		{"clang++", LanguageCpp, CompilerClang},
	}

	for _, spec := range compilers {
		if tcs := d.detectCompilerToolchains(ctx, spec.name, spec.language, spec.compiler); len(tcs) > 0 {
			toolchains = append(toolchains, tcs...)
		}
	}

	return toolchains, nil
}

// detectCompilerToolchains detects a single compiler and returns all available toolchain variants.
// For C: generates one toolchain per available C runtime.
// For C++ (Clang): generates toolchains for each C runtime × C++ stdlib combination.
// Returns nil if the compiler is not found on the system.
// Deduplicates toolchains with identical (version, cruntime, stdlib, arch) combinations.
func (d *Detector) detectCompilerToolchains(ctx context.Context, compilerName string, language Language, compiler Compiler) []Toolchain {
	paths := d.findCompilerPaths(compilerName)
	if len(paths) == 0 {
		return nil
	}

	var allToolchains []Toolchain
	seen := make(map[string]bool) // Deduplicate by version+cruntime+stdlib+arch

	// For each compiler version found
	for _, compilerPath := range paths {
		var toolchains []Toolchain

		// For C program, detect C runtime variants
		if language == LanguageC {
			toolchains = d.detectCRuntimeVariants(ctx, compilerPath, compilerName, language, compiler)
		} else if language == LanguageCpp && compiler == CompilerClang {
			// For C++ Clang, detect all C runtime + C++ stdlib combinations
			toolchains = d.detectClangCppVariants(ctx, compilerPath, compilerName)
		} else {
			// For C++ GCC, detect C runtime variants (GCC always uses libstdc++)
			toolchains = d.detectCRuntimeVariants(ctx, compilerPath, compilerName, language, compiler)
		}

		// Deduplicate: only add toolchains we haven't seen before
		// Key is: version+cruntime+cruntimeversion+language+stdlib+arch
		for _, tc := range toolchains {
			key := fmt.Sprintf("%s|%v|%s|%v|%v|%v",
				tc.CompilerVersion, tc.CRuntime, tc.CRuntimeVersion,
				tc.Language, tc.CppStdlib, tc.Architecture)
			if !seen[key] {
				seen[key] = true
				allToolchains = append(allToolchains, tc)
			}
		}
	}

	return allToolchains
}

// detectCRuntimeVariants detects all available C runtime variants for a compiler.
// For C programs, tests both glibc and musl if available.
// For C++ programs, creates variants for each available C runtime (combined with C++ stdlib).
// Returns a separate Toolchain for each available C runtime.
func (d *Detector) detectCRuntimeVariants(ctx context.Context, compilerPath string, compilerName string, language Language, compiler Compiler) []Toolchain {
	var toolchains []Toolchain

	testProgramData, err := testPrograms.ReadFile("testdata/c_runtime_check.c")
	if err != nil {
		return nil
	}

	testProgram := string(testProgramData)
	var availableRuntimes []CRuntime

	// Test glibc
	if d.testCRuntime(ctx, compilerPath, testProgram, "glibc", "") {
		availableRuntimes = append(availableRuntimes, CRuntimeGlibc)
	}

	// Test musl
	if d.testCRuntime(ctx, compilerPath, testProgram, "musl", "-lc") {
		availableRuntimes = append(availableRuntimes, CRuntimeMusl)
	}

	// If no runtimes detected, try default (glibc)
	if len(availableRuntimes) == 0 {
		availableRuntimes = append(availableRuntimes, CRuntimeGlibc)
	}

	// For each C runtime variant, detect target architecture variants
	for _, cruntime := range availableRuntimes {
		archVariants := d.detectArchitectureVariants(ctx, compilerPath, compilerName, language, compiler, cruntime)
		toolchains = append(toolchains, archVariants...)
	}

	return toolchains
}

// detectArchitectureVariants detects all available target architectures for a compiler + runtime combo.
// Tests common architectures (x86_64, aarch64, arm) to see which ones this compiler can target.
// Returns a separate Toolchain for each available architecture.
func (d *Detector) detectArchitectureVariants(ctx context.Context, compilerPath string, compilerName string, language Language, compiler Compiler, cruntime CRuntime) []Toolchain {
	var toolchains []Toolchain

	// Determine which language test program to use
	var testProgramPath string
	if language == LanguageC {
		testProgramPath = "testdata/c_runtime_check.c"
	} else {
		testProgramPath = "testdata/libstdcxx_check.cpp"
	}

	testProgramData, err := testPrograms.ReadFile(testProgramPath)
	if err != nil {
		return nil
	}

	testProgram := string(testProgramData)

	// Architecture flags to test: -m64 (x86_64), -m32 (x86), -march=armv7 (ARM), etc.
	architectureTests := []struct {
		arch Architecture
		name string
		flag string
	}{
		{ArchitectureX86_64, "x86_64", "-m64"},
		{ArchitectureAArch64, "aarch64", "-march=armv8-a"},
		{ArchitectureARM, "arm", "-march=armv7-a"},
	}

	var availableArchitectures []Architecture

	// Test which architectures this compiler can target
	for _, archTest := range architectureTests {
		if d.testArchitecture(ctx, compilerPath, testProgram, language, archTest.flag) {
			availableArchitectures = append(availableArchitectures, archTest.arch)
		}
	}

	// If no architectures detected, default to x86_64
	if len(availableArchitectures) == 0 {
		availableArchitectures = append(availableArchitectures, ArchitectureX86_64)
	}

	// For C++, for each architecture variant, detect C++ stdlib variants
	if language == LanguageCpp {
		for _, arch := range availableArchitectures {
			cppVariants := d.detectCppStdlibVariantsForArch(ctx, compilerPath, compilerName, compiler, cruntime, arch)
			toolchains = append(toolchains, cppVariants...)
		}
	} else {
		// For C, create one toolchain per available architecture
		for _, arch := range availableArchitectures {
			tc, err := d.buildToolchain(ctx, compilerPath, compiler, language, cruntime, CppStdlibUnspecified, arch)
			if err == nil {
				d.log.Info("detected toolchain", "compiler", compilerName, "language", languageString(language), "cruntime", cruntimeString(cruntime), "arch", archString(arch), "version", tc.CompilerVersion)
				toolchains = append(toolchains, tc)
			}
		}
	}

	return toolchains
}

// detectCppStdlibVariantsForArch detects all available C++ stdlib variants for a specific C runtime and architecture.
// Returns a separate Toolchain for each available stdlib (e.g., libstdc++, libc++).
func (d *Detector) detectCppStdlibVariantsForArch(ctx context.Context, compilerPath string, compilerName string, compiler Compiler, cruntime CRuntime, arch Architecture) []Toolchain {
	var toolchains []Toolchain

	testProgramData, err := testPrograms.ReadFile("testdata/libstdcxx_check.cpp")
	if err != nil {
		return nil
	}

	testProgram := string(testProgramData)
	var availableStdlibs []CppStdlib

	// Test libstdc++
	if d.testCppStdlib(ctx, compilerPath, testProgram, "libstdc++", "") {
		availableStdlibs = append(availableStdlibs, CppStdlibLibstdcxx)
	}

	// Test libc++ only for Clang (GCC doesn't support it well)
	if compiler == CompilerClang {
		if d.testCppStdlib(ctx, compilerPath, testProgram, "libc++", "-stdlib=libc++") {
			availableStdlibs = append(availableStdlibs, CppStdlibLibcxx)
		}
	}

	// If no stdlib detected, default to libstdc++
	if len(availableStdlibs) == 0 {
		availableStdlibs = append(availableStdlibs, CppStdlibLibstdcxx)
	}

	// Create a toolchain for each available stdlib + C runtime + architecture combination
	for _, stdlib := range availableStdlibs {
		tc, err := d.buildToolchain(ctx, compilerPath, compiler, LanguageCpp, cruntime, stdlib, arch)
		if err == nil {
			d.log.Info("detected toolchain", "compiler", compilerName, "language", "C++", "cruntime", cruntimeString(cruntime), "stdlib", cppstdlibString(stdlib), "arch", archString(arch), "version", tc.CompilerVersion)
			toolchains = append(toolchains, tc)
		}
	}

	return toolchains
}

// detectClangCppVariants detects all available C++ toolchain variants for Clang by testing
// all combinations of available C runtimes, C++ standard libraries, and target architectures.
// This creates a full matrix of all possible compilation environments.
func (d *Detector) detectClangCppVariants(ctx context.Context, compilerPath string, compilerName string) []Toolchain {
	testProgramData, err := testPrograms.ReadFile("testdata/c_runtime_check.c")
	if err != nil {
		return nil
	}

	testProgram := string(testProgramData)
	var availableRuntimes []CRuntime

	// Test glibc
	if d.testCRuntime(ctx, compilerPath, testProgram, "glibc", "") {
		availableRuntimes = append(availableRuntimes, CRuntimeGlibc)
	}

	// Test musl
	if d.testCRuntime(ctx, compilerPath, testProgram, "musl", "-lc") {
		availableRuntimes = append(availableRuntimes, CRuntimeMusl)
	}

	// If no runtimes detected, default to glibc
	if len(availableRuntimes) == 0 {
		availableRuntimes = append(availableRuntimes, CRuntimeGlibc)
	}

	// For each C runtime, detect architecture variants
	var allToolchains []Toolchain
	for _, cruntime := range availableRuntimes {
		archVariants := d.detectArchitectureVariants(ctx, compilerPath, compilerName, LanguageCpp, CompilerClang, cruntime)
		allToolchains = append(allToolchains, archVariants...)
	}

	return allToolchains
}

// buildToolchain constructs a complete Toolchain object from a compiler executable path.
// It detects compiler version, target architecture, C runtime, C++ standard library, and ABI.
// For C runtime, the provided runtime is used; for target architecture and C++, overrides can specify variants.
func (d *Detector) buildToolchain(ctx context.Context, compilerPath string, compiler Compiler, language Language, cruntime CRuntime, stdlibOverride CppStdlib, arch Architecture) (Toolchain, error) {
	tc := Toolchain{
		Language:     language,
		Compiler:     compiler,
		CompilerPath: compilerPath,
	}

	// Detect version
	tc.CompilerVersion = d.getCompilerVersion(ctx, compilerPath)

	// Set architecture
	tc.Architecture = arch

	// Set C runtime and detect its version
	tc.CRuntime = cruntime
	tc.CRuntimeVersion = d.detectCRuntimeVersion(ctx, cruntime)

	// For C++ compilers, set C++ stdlib and ABI
	if language == LanguageCpp {
		if stdlibOverride != CppStdlibUnspecified {
			// Use the specified stdlib (used when building multiple variants)
			tc.CppStdlib = stdlibOverride
		} else {
			// Auto-detect stdlib (used for GCC)
			tc.CppStdlib, _ = d.detectCppStdlib(ctx, compilerPath, tc.Compiler)
		}
		tc.AbiModifiers = d.detectAbiModifiers(ctx, compilerPath, tc.Compiler)
		tc.CppAbi = CppAbiItanium // Standard for UNIX-like systems
	}

	return tc, nil
}

// detectCRuntimeVersion returns the version of a specific C runtime.
func (d *Detector) detectCRuntimeVersion(ctx context.Context, cruntime CRuntime) string {
	switch cruntime {
	case CRuntimeGlibc:
		return d.getGlibcVersion(ctx)
	case CRuntimeMusl:
		return d.getMuslVersion(ctx)
	default:
		return "unknown"
	}
}

// testArchitecture attempts to compile a test program targeting a specific architecture.
// Returns true if the compiler can successfully target that architecture, false otherwise.
func (d *Detector) testArchitecture(ctx context.Context, compilerPath string, testProgram string, language Language, archFlag string) bool {
	// For C
	if language == LanguageC {
		args := []string{archFlag, "-x", "c", "-", "-o", "/dev/null"}
		cmd := exec.CommandContext(ctx, compilerPath, args...)
		cmd.Stdin = strings.NewReader(testProgram)
		return cmd.Run() == nil
	}

	// For C++
	args := []string{archFlag, "-x", "c++", "-", "-o", "/dev/null"}
	cmd := exec.CommandContext(ctx, compilerPath, args...)
	cmd.Stdin = strings.NewReader(testProgram)
	return cmd.Run() == nil
}

// parseArchitecture converts string architecture to Architecture enum.
func (d *Detector) parseArchitecture(arch string) Architecture {
	switch arch {
	case "x86_64":
		return ArchitectureX86_64
	case "aarch64":
		return ArchitectureAArch64
	case "arm":
		return ArchitectureARM
	default:
		return ArchitectureUnspecified
	}
}

// getGlibcVersion queries the glibc version from the system.
func (d *Detector) getGlibcVersion(ctx context.Context) string {
	// Try to get version from ldd
	cmd := exec.CommandContext(ctx, "ldd", "--version")
	output, err := cmd.Output()
	if err != nil {
		return "unknown"
	}

	// Parse version from output like "ldd (GNU libc) 2.31"
	lines := strings.Split(string(output), "\n")
	if len(lines) > 0 {
		re := regexp.MustCompile(`\d+\.\d+(\.\d+)?`)
		matches := re.FindStringSubmatch(lines[0])
		if len(matches) > 0 {
			return matches[0]
		}
	}

	return "unknown"
}

// getMuslVersion queries the musl version from the system.
func (d *Detector) getMuslVersion(ctx context.Context) string {
	// musl libc doesn't have a standard version query command
	// Try to detect from ld-musl-*.so.1 version
	cmd := exec.CommandContext(ctx, "sh", "-c", "find /lib -name 'ld-musl-*.so.1' -o -name 'libc.musl-*.so.1' | head -1")
	output, err := cmd.Output()
	if err != nil {
		return "unknown"
	}

	// Extract version from filename
	filename := strings.TrimSpace(string(output))
	if filename != "" {
		re := regexp.MustCompile(`\d+\.\d+(\.\d+)?`)
		matches := re.FindStringSubmatch(filename)
		if len(matches) > 0 {
			return matches[0]
		}
	}

	return "unknown"
}

// detectCppStdlib determines the default C++ standard library for GCC (always libstdc++).
// Returns the default stdlib and empty abiModifiers (use detectAbiModifiers for those).
func (d *Detector) detectCppStdlib(ctx context.Context, compilerPath string, compiler Compiler) (CppStdlib, []string) {
	switch compiler {
	case CompilerGCC:
		// GCC uses libstdc++ by default
		return CppStdlibLibstdcxx, nil
	default:
		return CppStdlibUnspecified, nil
	}
}

// detectAbiModifiers detects C++11 ABI modifiers for the given compiler and language.
func (d *Detector) detectAbiModifiers(ctx context.Context, compilerPath string, compiler Compiler) []string {
	var abiModifiers []string

	if compiler != CompilerGCC {
		return abiModifiers
	}

	// GCC: Check for C++11 ABI
	abiCheckData, err := testPrograms.ReadFile("testdata/gcc_cxx11_abi_check.hpp")
	if err == nil {
		cmd := exec.CommandContext(ctx, compilerPath, "-E", "-dM", "-")
		cmd.Stdin = strings.NewReader(string(abiCheckData))
		output, err := cmd.Output()
		if err == nil && strings.Contains(string(output), "_GLIBCXX_USE_CXX11_ABI") {
			abiModifiers = append(abiModifiers, "-D_GLIBCXX_USE_CXX11_ABI=1")
		}
	}

	return abiModifiers
}

// testCRuntime attempts to compile a test C program with a specific C runtime.
// Returns true if compilation succeeds, false otherwise.
func (d *Detector) testCRuntime(ctx context.Context, compilerPath string, testProgram string, libName string, extraFlag string) bool {
	var args []string
	if extraFlag != "" {
		args = []string{"-x", "c", "-", extraFlag, "-o", "/dev/null"}
	} else {
		args = []string{"-x", "c", "-", "-o", "/dev/null"}
	}

	cmd := exec.CommandContext(ctx, compilerPath, args...)
	cmd.Stdin = strings.NewReader(testProgram)
	return cmd.Run() == nil
}

// testCppStdlib attempts to compile a test program with a specific C++ standard library.
// Returns true if compilation succeeds, false otherwise.
func (d *Detector) testCppStdlib(ctx context.Context, compilerPath string, testProgram string, libName string, stdlibFlag string) bool {
	var args []string
	if stdlibFlag != "" {
		args = []string{stdlibFlag, "-x", "c++", "-", "-o", "/dev/null"}
	} else {
		args = []string{"-x", "c++", "-", "-o", "/dev/null"}
	}

	cmd := exec.CommandContext(ctx, compilerPath, args...)
	cmd.Stdin = strings.NewReader(testProgram)
	return cmd.Run() == nil
}

// DetectGCC detects the GCC toolchain for C compilation.
// Returns nil if GCC is not found on the system.
func (d *Detector) DetectGCC(ctx context.Context) *Toolchain {
	tcs := d.detectCompilerToolchains(ctx, "gcc", LanguageC, CompilerGCC)
	if len(tcs) > 0 {
		return &tcs[0]
	}
	return nil
}

// DetectGxx detects the G++ toolchain for C++ compilation.
// Returns nil if G++ is not found on the system.
func (d *Detector) DetectGxx(ctx context.Context) *Toolchain {
	tcs := d.detectCompilerToolchains(ctx, "g++", LanguageCpp, CompilerGCC)
	if len(tcs) > 0 {
		return &tcs[0]
	}
	return nil
}

// DetectClang detects the Clang toolchain for C compilation.
// Returns nil if Clang is not found on the system.
func (d *Detector) DetectClang(ctx context.Context) *Toolchain {
	tcs := d.detectCompilerToolchains(ctx, "clang", LanguageC, CompilerClang)
	if len(tcs) > 0 {
		return &tcs[0]
	}
	return nil
}

// DetectClangxx detects the Clang++ toolchain for C++ compilation.
// Returns nil if Clang++ is not found on the system.
// Note: If multiple C++ standard libraries are available, returns the first one.
func (d *Detector) DetectClangxx(ctx context.Context) *Toolchain {
	tcs := d.detectCompilerToolchains(ctx, "clang++", LanguageCpp, CompilerClang)
	if len(tcs) > 0 {
		return &tcs[0]
	}
	return nil
}

// compilerString returns the string representation of a Compiler enum.
func compilerString(c Compiler) string {
	switch c {
	case CompilerGCC:
		return "GCC"
	case CompilerClang:
		return "Clang"
	default:
		return "unknown"
	}
}

// languageString returns the string representation of a Language enum.
func languageString(l Language) string {
	switch l {
	case LanguageC:
		return "C"
	case LanguageCpp:
		return "C++"
	default:
		return "unknown"
	}
}

// cruntimeString returns the string representation of a CRuntime enum.
func cruntimeString(c CRuntime) string {
	switch c {
	case CRuntimeGlibc:
		return "glibc"
	case CRuntimeMusl:
		return "musl"
	default:
		return "unknown"
	}
}

// cppstdlibString returns the string representation of a CppStdlib enum.
func cppstdlibString(c CppStdlib) string {
	switch c {
	case CppStdlibLibstdcxx:
		return "libstdc++"
	case CppStdlibLibcxx:
		return "libc++"
	default:
		return "unknown"
	}
}

// cppAbiString returns the string representation of a CppAbi enum.
func cppAbiString(c CppAbi) string {
	switch c {
	case CppAbiItanium:
		return "itanium"
	default:
		return "unknown"
	}
}

// archString returns the string representation of an Architecture enum.
func archString(a Architecture) string {
	switch a {
	case ArchitectureX86_64:
		return "x86_64"
	case ArchitectureAArch64:
		return "aarch64"
	case ArchitectureARM:
		return "arm"
	default:
		return "unknown"
	}
}

// String returns a human-readable representation of the Toolchain.
func (tc *Toolchain) String() string {
	return fmt.Sprintf(
		"%s %s (v%s) %s [%s, %s@%s]",
		compilerString(tc.Compiler),
		languageString(tc.Language),
		tc.CompilerVersion,
		archString(tc.Architecture),
		cruntimeString(tc.CRuntime),
		cppstdlibString(tc.CppStdlib),
		cppAbiString(tc.CppAbi),
	)
}
