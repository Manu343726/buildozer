package ar_common

import (
	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
)

// ValidateRuntimeForArchive checks if a runtime is compatible for archive operations.
func ValidateRuntimeForArchive(runtime *v1.Runtime) (bool, string) {
	if runtime == nil {
		return false, "runtime is nil"
	}

	// Archive operations require a C/C++ runtime (C toolchain)
	if runtime.Toolchain != v1.RuntimeToolchain_RUNTIME_TOOLCHAIN_C &&
		runtime.Toolchain != v1.RuntimeToolchain_RUNTIME_TOOLCHAIN_CPP {
		return false, "archive operations require C or C++ toolchain"
	}

	// Check if it's a CppToolchain
	cppToolchain := runtime.GetCpp()
	if cppToolchain != nil {
		return true, ""
	}

	return false, "runtime does not support archive operations (requires C/C++ toolchain)"
}
