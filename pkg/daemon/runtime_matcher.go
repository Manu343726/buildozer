package daemon

import (
	"context"
	"fmt"

	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
)

// Match finds all runtimes matching the given RuntimeMatchQuery.
// It filters by platform and toolchain first, then checks specific parameters.
//
// Example usage for AR (archiver):
//
//	query := &v1.RuntimeMatchQuery{
//	    Platforms:  []v1.RuntimePlatform{v1.RUNTIME_PLATFORM_NATIVE_LINUX},
//	    Toolchains: []v1.RuntimeToolchain{v1.RUNTIME_TOOLCHAIN_C, v1.RUNTIME_TOOLCHAIN_CPP},
//	    Params: map[string]*v1.StringArray{
//	        "c_runtime":         & v1.StringArray{Values: []string{"glibc"}},
//	        "c_runtime_version": &v1.StringArray{Values: []string{"2.31"}},
//	        "architecture":      &v1.StringArray{Values: []string{"x86_64"}},
//	        "cpp_stdlib":        &v1.StringArray{Values: []string{}},  // empty = don't care
//	    },
//	}
//	matches, err := rm.Match(ctx, query)
func (rm *RuntimeManager) Match(ctx context.Context, query *v1.RuntimeMatchQuery) ([]*v1.Runtime, error) {
	if query == nil {
		return nil, fmt.Errorf("match query cannot be nil")
	}

	// List all runtimes (triggering detection if needed)
	runtimes, _, err := rm.ListRuntimes(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("failed to list runtimes for matching: %w", err)
	}

	var matches []*v1.Runtime

	for _, rt := range runtimes {
		if rm.runtimeMatchesQuery(rt, query) {
			matches = append(matches, rt)
		}
	}

	rm.Debug("Runtime match query completed", "query_platforms", query.Platforms, "query_toolchains", query.Toolchains, "matches_found", len(matches))

	return matches, nil
}

// runtimeMatchesQuery checks if a runtime matches all constraints in the query.
func (rm *RuntimeManager) runtimeMatchesQuery(rt *v1.Runtime, query *v1.RuntimeMatchQuery) bool {
	// Check platform constraint
	if len(query.Platforms) > 0 {
		platformMatches := false
		for _, qp := range query.Platforms {
			if rt.Platform == qp {
				platformMatches = true
				break
			}
		}
		if !platformMatches {
			return false
		}
	}

	// Check toolchain constraint
	if len(query.Toolchains) > 0 {
		toolchainMatches := false
		for _, qt := range query.Toolchains {
			if rt.Toolchain == qt {
				toolchainMatches = true
				break
			}
		}
		if !toolchainMatches {
			return false
		}
	}

	// Check specific parameters based on toolchain type
	if len(query.Params) > 0 {
		runtimeParams := rm.extractRuntimeParameters(rt)
		if !rm.parametersMatch(runtimeParams, query.Params) {
			return false
		}
	}

	return true
}

// extractRuntimeParameters extracts key runtime parameters as strings from a Runtime.
// The result is a map of parameter name -> value (as string).
//
// For C/C++ (CppToolchain):
//   - c_runtime: enum name (e.g., "GLIBC", "MUSL")
//   - c_runtime_version: version string (e.g., "2.31")
//   - architecture: enum name (e.g., "X86_64", "AARCH64")
//   - cpp_stdlib: enum name (e.g., "LIBSTDCXX", "LIBCXX") or empty if not specified
//   - compiler: enum name (e.g., "GCC", "CLANG") or empty if not applicable
//   - compiler_version: version string
//
// For other toolchains, adds language-specific parameters as needed.
func (rm *RuntimeManager) extractRuntimeParameters(rt *v1.Runtime) map[string]string {
	params := make(map[string]string)

	// Add platform parameter
	params["platform"] = rt.Platform.String()

	// Add toolchain parameter
	params["toolchain"] = rt.Toolchain.String()

	// Extract toolchain-specific parameters
	switch ts := rt.ToolchainSpec.(type) {
	case *v1.Runtime_Cpp:
		if ts.Cpp != nil {
			// C runtime and version
			params["c_runtime"] = ts.Cpp.CRuntime.String()
			if ts.Cpp.CRuntimeVersion != nil {
				params["c_runtime_version"] = ts.Cpp.CRuntimeVersion.String()
			}

			// Architecture
			params["architecture"] = ts.Cpp.Architecture.String()

			// C++ stdlib (only if not unspecified)
			if ts.Cpp.CppStdlib != v1.CppStdlib_CPP_STDLIB_UNSPECIFIED {
				params["cpp_stdlib"] = ts.Cpp.CppStdlib.String()
			}

			// Compiler and version (if available)
			if ts.Cpp.Compiler != v1.CppCompiler_CPP_COMPILER_UNSPECIFIED {
				params["compiler"] = ts.Cpp.Compiler.String()
			}
			if ts.Cpp.CompilerVersion != nil {
				params["compiler_version"] = ts.Cpp.CompilerVersion.String()
			}
		}

	case *v1.Runtime_Go:
		if ts.Go != nil {
			params["goarch"] = ts.Go.Goarch
			params["goos"] = ts.Go.Goos
			if ts.Go.GoVersion != nil {
				params["go_version"] = ts.Go.GoVersion.String()
			}
		}

	case *v1.Runtime_Rust:
		if ts.Rust != nil {
			params["target_triple"] = ts.Rust.TargetTriple
			params["edition"] = ts.Rust.Edition
			if ts.Rust.RustVersion != nil {
				params["rust_version"] = ts.Rust.RustVersion.String()
			}
		}
	}

	return params
}

// parametersMatch checks if runtime parameters satisfy query constraints.
// For each parameter in query:
//   - If the query parameter value list is empty: the parameter is optional (matches any/none)
//   - If the query parameter value list is non-empty: runtime value must be in the list
//
// Query params are normalized: enum strings like "RUNTIME_PLATFORM_NATIVE_LINUX" are compared as-is.
func (rm *RuntimeManager) parametersMatch(runtimeParams map[string]string, queryParams map[string]*v1.StringArray) bool {
	for paramName, paramConstraint := range queryParams {
		// Empty constraint means "don't care", so any value is acceptable
		if paramConstraint == nil || len(paramConstraint.Values) == 0 {
			continue
		}

		// Get runtime's value for this parameter
		runtimeValue, exists := runtimeParams[paramName]
		if !exists {
			// Runtime doesn't have this parameter but query requires it
			// This is typically not a match, but could depend on semantics
			// For now, treat missing parameters conservatively:
			// If the runtime doesn't have the parameter, it can't satisfy the constraint
			return false
		}

		// Check if runtime value is in the list of acceptable values
		if !rm.stringInList(runtimeValue, paramConstraint.Values) {
			return false
		}
	}

	return true
}

// stringInList checks if a string is in a list of strings.
func (rm *RuntimeManager) stringInList(s string, list []string) bool {
	for _, item := range list {
		if s == item {
			return true
		}
	}
	return false
}
