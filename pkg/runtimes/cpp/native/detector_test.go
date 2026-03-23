package native

import (
	"context"
	"testing"
	"time"
)

func TestDetectGCC(t *testing.T) {
	detector := NewDetector()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tc := detector.DetectGCC(ctx)
	if tc == nil {
		t.Skip("GCC not available on this system")
	}

	if tc.Compiler != CompilerGCC {
		t.Errorf("expected compiler=GCC, got %v", tc.Compiler)
	}

	if tc.Language != LanguageC {
		t.Errorf("expected language=C, got %v", tc.Language)
	}

	if tc.CompilerVersion == "unknown" {
		t.Errorf("expected compiler version to be detected, got 'unknown'")
	}

	if tc.CRuntime == CRuntimeUnspecified {
		t.Errorf("expected C runtime to be detected, got Unspecified")
	}
}

func TestDetectGxx(t *testing.T) {
	detector := NewDetector()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tc := detector.DetectGxx(ctx)
	if tc == nil {
		t.Skip("G++ not available on this system")
	}

	if tc.Compiler != CompilerGCC {
		t.Errorf("expected compiler=GCC (for g++), got %v", tc.Compiler)
	}

	if tc.Language != LanguageCpp {
		t.Errorf("expected language=C++, got %v", tc.Language)
	}

	if tc.CompilerVersion == "unknown" {
		t.Errorf("expected compiler version to be detected, got 'unknown'")
	}

	if tc.CppStdlib == CppStdlibUnspecified {
		t.Errorf("expected C++ stdlib to be detected, got Unspecified")
	}

	if tc.CppAbi == CppAbiUnspecified {
		t.Errorf("expected C++ ABI to be detected, got Unspecified")
	}
}

func TestDetectToolchains(t *testing.T) {
	detector := NewDetector()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	toolchains, err := detector.DetectToolchains(ctx)
	if err != nil {
		t.Fatalf("DetectToolchains failed: %v", err)
	}

	if len(toolchains) == 0 {
		t.Skip("No C/C++ toolchains detected on this system")
	}

	for i, tc := range toolchains {
		t.Logf("Toolchain %d: %s", i, tc.String())

		if tc.Compiler == CompilerUnspecified {
			t.Errorf("toolchain %d: compiler not detected", i)
		}

		if tc.Language == LanguageUnspecified {
			t.Errorf("toolchain %d: language not detected", i)
		}

		if tc.CompilerVersion == "unknown" {
			t.Logf("toolchain %d: warning - compiler version 'unknown'", i)
		}

		if tc.CRuntime == CRuntimeUnspecified {
			t.Logf("toolchain %d: warning - C runtime not detected", i)
		}

		if tc.Language == LanguageCpp && tc.CppStdlib == CppStdlibUnspecified {
			t.Logf("toolchain %d: warning - C++ stdlib not detected", i)
		}
	}
}

func TestParseArchitecture(t *testing.T) {
	detector := NewDetector()

	tests := map[string]Architecture{
		"x86_64":  ArchitectureX86_64,
		"aarch64": ArchitectureAArch64,
		"arm":     ArchitectureARM,
		"unknown": ArchitectureUnspecified,
		"mips":    ArchitectureUnspecified,
	}

	for input, expected := range tests {
		result := detector.parseArchitecture(input)
		if result != expected {
			t.Errorf("parseArchitecture(%q): expected %v, got %v", input, expected, result)
		}
	}
}

func TestGetCompilerVersion(t *testing.T) {
	detector := NewDetector()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test with a known compiler if available
	gccPath := "/usr/bin/gcc"
	version := detector.getCompilerVersion(ctx, gccPath)

	// Version should either be a valid version string or "unknown"
	if version == "" {
		t.Errorf("expected non-empty version string, got empty")
	}

	// If version is not "unknown", it should contain a dot
	if version != "unknown" && !containsDot(version) {
		t.Logf("warning: version string %q doesn't look like a version", version)
	}
}

func TestDetectArchitecture(t *testing.T) {
	detector := NewDetector()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	arch := detector.detectArchitecture(ctx, "/usr/bin/gcc")

	// Should return a valid architecture string
	if arch == "" || arch == "unknown" {
		t.Logf("warning: architecture detection returned: %q", arch)
	}

	// Should match GOARCH mapping
	validArches := map[string]bool{
		"x86_64":  true,
		"aarch64": true,
		"arm":     true,
		"unknown": true,
	}

	if !validArches[arch] {
		t.Errorf("unexpected architecture: %q", arch)
	}
}

func TestToolchainString(t *testing.T) {
	tc := Toolchain{
		Language:        LanguageCpp,
		Compiler:        CompilerGCC,
		CompilerPath:    "/usr/bin/g++",
		CompilerVersion: "11.2.0",
		Architecture:    ArchitectureX86_64,
		CRuntime:        CRuntimeGlibc,
		CRuntimeVersion: "2.31",
		CppStdlib:       CppStdlibLibstdcxx,
		CppAbi:          CppAbiItanium,
		AbiModifiers:    []string{"-D_GLIBCXX_USE_CXX11_ABI=1"},
	}

	str := tc.String()

	// Check that the string representation contains expected parts
	expectedParts := []string{
		"GCC",
		"C++",
		"11.2.0",
		"x86_64",
	}

	for _, part := range expectedParts {
		if !contains(str, part) {
			t.Errorf("expected %q in string representation, got: %s", part, str)
		}
	}
}

// Helper functions
func containsDot(s string) bool {
	return len(s) > 0 && s[0] >= '0' && s[0] <= '9' && contains(s, ".")
}

func contains(s, substr string) bool {
	for i := 0; i < len(s)-len(substr)+1; i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
