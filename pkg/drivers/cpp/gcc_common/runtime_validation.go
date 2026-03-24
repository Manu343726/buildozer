package gcc_common

import (
	"fmt"

	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
)

// RuntimeCompatibility holds information about whether a runtime is compatible with a driver
type RuntimeCompatibility struct {
	IsCompatible bool
	Reason       string // explanation if not compatible
}

// ValidateRuntimeForC checks if a runtime is compatible with a C compiler (gcc).
// A runtime is compatible if:
// - It has C/C++ toolchain metadata
// - It supports C language
// - It uses the GCC compiler
// - It is native
func ValidateRuntimeForC(runtime *v1.Runtime) RuntimeCompatibility {
	if runtime == nil {
		return RuntimeCompatibility{
			IsCompatible: false,
			Reason:       "runtime is nil",
		}
	}

	// Check if it has C/C++ toolchain
	cppToolchain := runtime.GetCpp()
	if cppToolchain == nil {
		return RuntimeCompatibility{
			IsCompatible: false,
			Reason:       fmt.Sprintf("runtime '%s' is not a C/C++ toolchain", runtime.Id),
		}
	}

	// Check if it uses GCC compiler
	compiler := cppToolchain.Compiler
	if compiler != v1.CppCompiler_CPP_COMPILER_GCC {
		return RuntimeCompatibility{
			IsCompatible: false,
			Reason: fmt.Sprintf("runtime '%s' does not use GCC compiler (compiler: %v)",
				runtime.Id, compiler),
		}
	}

	// Check if it supports C language
	language := cppToolchain.Language
	if language != v1.CppLanguage_CPP_LANGUAGE_C {
		return RuntimeCompatibility{
			IsCompatible: false,
			Reason: fmt.Sprintf("runtime '%s' does not support C language (language: %v)",
				runtime.Id, language),
		}
	}

	// For now, we only support native runtimes with C/C++ toolchain
	// Future: extend to support remote/Docker runtimes if needed

	return RuntimeCompatibility{
		IsCompatible: true,
		Reason:       "runtime supports C language with GCC compiler",
	}
}

// ValidateRuntimeForCxx checks if a runtime is compatible with a C++ compiler (g++).
// A runtime is compatible if:
// - It has C/C++ toolchain metadata
// - It supports C++ language
// - It uses the GCC compiler
// - It is native
func ValidateRuntimeForCxx(runtime *v1.Runtime) RuntimeCompatibility {
	if runtime == nil {
		return RuntimeCompatibility{
			IsCompatible: false,
			Reason:       "runtime is nil",
		}
	}

	// Check if it has C/C++ toolchain
	cppToolchain := runtime.GetCpp()
	if cppToolchain == nil {
		return RuntimeCompatibility{
			IsCompatible: false,
			Reason:       fmt.Sprintf("runtime '%s' is not a C/C++ toolchain", runtime.Id),
		}
	}

	// Check if it uses GCC compiler
	compiler := cppToolchain.Compiler
	if compiler != v1.CppCompiler_CPP_COMPILER_GCC {
		return RuntimeCompatibility{
			IsCompatible: false,
			Reason: fmt.Sprintf("runtime '%s' does not use GCC compiler (compiler: %v)",
				runtime.Id, compiler),
		}
	}

	// Check if it supports C++ language
	language := cppToolchain.Language
	if language != v1.CppLanguage_CPP_LANGUAGE_CPP {
		return RuntimeCompatibility{
			IsCompatible: false,
			Reason: fmt.Sprintf("runtime '%s' does not support C++ language (language: %v)",
				runtime.Id, language),
		}
	}

	// For now, we only support native runtimes with C/C++ toolchain
	// Future: extend to support remote/Docker runtimes if needed

	return RuntimeCompatibility{
		IsCompatible: true,
		Reason:       "runtime supports C++ language with GCC compiler",
	}
}

// ValidateRuntimeForClang checks if a runtime is compatible with Clang C compiler.
// A runtime is compatible if:
// - It has C/C++ toolchain metadata
// - It supports C language
// - It uses the CLANG compiler (not GCC)
// - It is native
func ValidateRuntimeForClang(runtime *v1.Runtime) RuntimeCompatibility {
	if runtime == nil {
		return RuntimeCompatibility{
			IsCompatible: false,
			Reason:       "runtime is nil",
		}
	}

	// Check if it has C/C++ toolchain
	cppToolchain := runtime.GetCpp()
	if cppToolchain == nil {
		return RuntimeCompatibility{
			IsCompatible: false,
			Reason:       fmt.Sprintf("runtime '%s' is not a C/C++ toolchain", runtime.Id),
		}
	}

	// Check if it uses CLANG compiler (not GCC)
	compiler := cppToolchain.Compiler
	if compiler != v1.CppCompiler_CPP_COMPILER_CLANG {
		return RuntimeCompatibility{
			IsCompatible: false,
			Reason: fmt.Sprintf("runtime '%s' does not use Clang compiler (compiler: %v)",
				runtime.Id, compiler),
		}
	}

	// Check if it supports C language
	language := cppToolchain.Language
	if language != v1.CppLanguage_CPP_LANGUAGE_C {
		return RuntimeCompatibility{
			IsCompatible: false,
			Reason: fmt.Sprintf("runtime '%s' does not support C language (language: %v)",
				runtime.Id, language),
		}
	}

	return RuntimeCompatibility{
		IsCompatible: true,
		Reason:       "runtime supports C language with Clang compiler",
	}
}

// ValidateRuntimeForClangxx checks if a runtime is compatible with Clang++ C++ compiler.
// A runtime is compatible if:
// - It has C/C++ toolchain metadata
// - It supports C++ language
// - It uses the CLANG compiler (not GCC)
// - It is native
func ValidateRuntimeForClangxx(runtime *v1.Runtime) RuntimeCompatibility {
	if runtime == nil {
		return RuntimeCompatibility{
			IsCompatible: false,
			Reason:       "runtime is nil",
		}
	}

	// Check if it has C/C++ toolchain
	cppToolchain := runtime.GetCpp()
	if cppToolchain == nil {
		return RuntimeCompatibility{
			IsCompatible: false,
			Reason:       fmt.Sprintf("runtime '%s' is not a C/C++ toolchain", runtime.Id),
		}
	}

	// Check if it uses CLANG compiler (not GCC)
	compiler := cppToolchain.Compiler
	if compiler != v1.CppCompiler_CPP_COMPILER_CLANG {
		return RuntimeCompatibility{
			IsCompatible: false,
			Reason: fmt.Sprintf("runtime '%s' does not use Clang compiler (compiler: %v)",
				runtime.Id, compiler),
		}
	}

	// Check if it supports C++ language
	language := cppToolchain.Language
	if language != v1.CppLanguage_CPP_LANGUAGE_CPP {
		return RuntimeCompatibility{
			IsCompatible: false,
			Reason: fmt.Sprintf("runtime '%s' does not support C++ language (language: %v)",
				runtime.Id, language),
		}
	}

	return RuntimeCompatibility{
		IsCompatible: true,
		Reason:       "runtime supports C++ language with Clang compiler",
	}
}
