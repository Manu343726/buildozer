package runtime

import (
	"fmt"

	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
)

// CppRuntimeParser handles parsing of native C/C++ runtime IDs
// Format for C:  compiler-version-cruntime-cruntime_version-arch
// Format for C++: compiler-version-cruntime-cruntime_version-stdlib-arch
type CppRuntimeParser struct{}

func init() {
	// Register parsers for C/C++ toolchains on native Linux platform
	RegisterRuntimeParser("native_linux-c", &CppRuntimeParser{})
	RegisterRuntimeParser("native_linux-cpp", &CppRuntimeParser{})
	// Note: AR is not a separate toolchain - it's part of the C toolchain and uses C runtimes (CppArchiveJob jobs)
}

// Parse implements RuntimeParser for C/C++ runtimes
// idParts contains: [compiler, version, cruntime, cruntime_version, ...]
// For C: [compiler, version, cruntime, cruntime_version, arch]
// For C++: [compiler, version, cruntime, cruntime_version, stdlib, arch]
func (p *CppRuntimeParser) Parse(idParts []string) (*v1.Runtime, error) {
	if len(idParts) < 5 {
		return nil, fmt.Errorf("C/C++ runtime ID requires at least 5 parts, got %d", len(idParts))
	}

	// Determine language based on runtime kind
	// This will be set by the caller after determining the kind
	compiler := parseCompiler(idParts[0])
	if compiler == v1.CppCompiler_CPP_COMPILER_UNSPECIFIED {
		return nil, fmt.Errorf("invalid compiler: %q", idParts[0])
	}

	compilerVersion := parseVersion(idParts[1])

	cRuntime := parseCRuntime(idParts[2])
	if cRuntime == v1.CRuntime_C_RUNTIME_UNSPECIFIED {
		return nil, fmt.Errorf("invalid c_runtime: %q", idParts[2])
	}

	cRuntimeVersion := parseVersion(idParts[3])

	// Determine if C or C++ based on remaining parts
	var lang v1.CppLanguage
	var arch v1.CpuArchitecture
	var stdlib v1.CppStdlib

	if len(idParts) == 5 {
		// C format: compiler-version-cruntime-cruntime_version-arch
		lang = v1.CppLanguage_CPP_LANGUAGE_C
		arch = parseArchitecture(idParts[4])
		if arch == v1.CpuArchitecture_CPU_ARCHITECTURE_UNSPECIFIED {
			return nil, fmt.Errorf("invalid architecture: %q", idParts[4])
		}
	} else if len(idParts) == 6 {
		// C++ format: compiler-version-cruntime-cruntime_version-stdlib-arch
		lang = v1.CppLanguage_CPP_LANGUAGE_CPP
		stdlib = parseStdlib(idParts[4])
		if stdlib == v1.CppStdlib_CPP_STDLIB_UNSPECIFIED {
			return nil, fmt.Errorf("invalid cpp_stdlib: %q", idParts[4])
		}
		arch = parseArchitecture(idParts[5])
		if arch == v1.CpuArchitecture_CPU_ARCHITECTURE_UNSPECIFIED {
			return nil, fmt.Errorf("invalid architecture: %q", idParts[5])
		}
	} else {
		return nil, fmt.Errorf("C/C++ runtime ID must have 5 or 6 parts, got %d", len(idParts))
	}

	rt := &v1.Runtime{
		Toolchain: v1.RuntimeToolchain_RUNTIME_TOOLCHAIN_C,
		ToolchainSpec: &v1.Runtime_Cpp{
			Cpp: &v1.CppToolchain{
				Language:        lang,
				Compiler:        compiler,
				CompilerVersion: compilerVersion,
				Architecture:    arch,
				CRuntime:        cRuntime,
				CRuntimeVersion: cRuntimeVersion,
				CppStdlib:       stdlib,
			},
		},
	}

	return rt, nil
}
