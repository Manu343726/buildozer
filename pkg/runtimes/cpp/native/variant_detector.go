package native

import (
	"context"
	"os/exec"
	"strings"
)

// VariantTester is a function that tests if a specific variant is supported by a compiler.
// It returns true if the variant is available, false otherwise.
type VariantTester func(ctx context.Context, compilerPath string) bool

// testCompilerVariant is a generic helper for testing if a compiler supports a specific feature
// (C runtime, C++ stdlib, architecture, etc.) by attempting to compile a test program with a specific flag.
//
// It uses the provided testProgram and compileFlag to determine if the variant is supported.
// This generic function eliminates duplication across different variant tests.
func testCompilerVariant(ctx context.Context, compilerPath string, testProgram string, compileFlag string, language Language) bool {
	// Build the arguments based on language
	args := []string{}

	if compileFlag != "" {
		args = append(args, compileFlag)
	}

	// Add language specification
	if language == LanguageC {
		args = append(args, "-x", "c")
	} else {
		args = append(args, "-x", "c++")
	}

	// Add test program as stdin and suppress output
	args = append(args, "-", "-o", "/dev/null")

	// Try to compile
	cmd := exec.CommandContext(ctx, compilerPath, args...)
	cmd.Stdin = strings.NewReader(testProgram)

	// Return true if compilation succeeds
	err := cmd.Run()
	return err == nil
}

// testCompilerVariantWithLink is a variant tester that tries to compile AND link with a specific flag.
// This is useful for testing C runtime availability, which may require linking against specific libraries.
func testCompilerVariantWithLink(ctx context.Context, compilerPath string, testProgram string, compileFlag string, linkFlag string, language Language, libs ...string) bool {
	args := []string{}

	if compileFlag != "" {
		args = append(args, compileFlag)
	}

	// Add language specification
	if language == LanguageC {
		args = append(args, "-x", "c")
	} else {
		args = append(args, "-x", "c++")
	}

	// Add test program as stdin
	args = append(args, "-")

	// Add link flag if specified
	if linkFlag != "" {
		args = append(args, linkFlag)
	}

	// Add any additional libraries
	for _, lib := range libs {
		args = append(args, "-l"+lib)
	}

	// Suppress output
	args = append(args, "-o", "/dev/null")

	// Try to compile and link
	cmd := exec.CommandContext(ctx, compilerPath, args...)
	cmd.Stdin = strings.NewReader(testProgram)

	// Return true if compilation and linking succeeds
	err := cmd.Run()
	return err == nil
}

// CollectVariants is a generic helper for detecting available variants by testing each one.
// It takes a list of candidates and a tester function, and returns only those that pass the test.
// If no variants are detected, it returns the defaultVariant.
//
// This pattern is used for detecting:
// - Available C runtimes (glibc, musl)
// - Available C++ standard libraries (libstdc++, libc++)
// - Supported target architectures (x86_64, aarch64, arm)
//
// Type parameter V is the variant type (CRuntime, CppStdlib, Architecture, etc.)
// candidates is a slice of variant candidates to test
// tester is a function that tests if a variant is available
// defaultVariant is the fallback if no variants are detected
func CollectVariants[V comparable](candidates []V, tester func(v V) bool, defaultVariant V) []V {
	var available []V

	for _, candidate := range candidates {
		if tester(candidate) {
			available = append(available, candidate)
		}
	}

	// If no variants detected, use default
	if len(available) == 0 {
		available = append(available, defaultVariant)
	}

	return available
}

// VariantCombinator helps build all combinations of variants from multiple dimensions.
// For example: all C runtimes × all C++ stdlibs × all architectures
// This is a builder pattern that makes it easy to generate the full variant matrix.
type VariantCombinator struct {
	dimensions [][]interface{}
}

// NewVariantCombinator creates a new combinator for building variant matrices.
func NewVariantCombinator() *VariantCombinator {
	return &VariantCombinator{
		dimensions: [][]interface{}{},
	}
}

// AddDimension adds a new dimension of variants to combine.
// For example: AddDimension("C Runtimes", []CRuntime{glibc, musl})
func (vc *VariantCombinator) AddDimension(variants ...interface{}) *VariantCombinator {
	if len(variants) > 0 {
		vc.dimensions = append(vc.dimensions, variants)
	}
	return vc
}

// GenerateCombinations generates all possible combinations of the added dimensions.
// Returns a slice of slices, where each inner slice is one combination.
// For example: [[glibc, libstdc++, x86_64], [glibc, libstdc++, aarch64], ...]
func (vc *VariantCombinator) GenerateCombinations() [][]interface{} {
	if len(vc.dimensions) == 0 {
		return [][]interface{}{}
	}

	// Start with the first dimension
	combinations := [][]interface{}{}
	for _, item := range vc.dimensions[0] {
		combinations = append(combinations, []interface{}{item})
	}

	// For each subsequent dimension, combine with existing combinations
	for dimIdx := 1; dimIdx < len(vc.dimensions); dimIdx++ {
		newCombinations := [][]interface{}{}
		for _, combo := range combinations {
			for _, item := range vc.dimensions[dimIdx] {
				newCombo := append([]interface{}{}, combo...)
				newCombo = append(newCombo, item)
				newCombinations = append(newCombinations, newCombo)
			}
		}
		combinations = newCombinations
	}

	return combinations
}
