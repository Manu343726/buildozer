package gcc_common

import "fmt"

// RuntimeCompilerType represents the compiler (gcc or clang family) for runtime ID construction
type RuntimeCompilerType string

const (
	RuntimeCompilerGCC   RuntimeCompilerType = "gcc"
	RuntimeCompilerClang RuntimeCompilerType = "clang"
)

// RuntimeLanguage represents the programming language (C or C++) for runtime ID construction
type RuntimeLanguage string

const (
	RuntimeLanguageC   RuntimeLanguage = "c"
	RuntimeLanguageCxx RuntimeLanguage = "cpp"
)

// ConstructRuntimeIDFromGccConfig constructs a runtime ID from a GccConfig
func ConstructRuntimeIDFromGccConfig(cfg GccConfig) string {
	return fmt.Sprintf("native_linux-c-%s-%s-%s-%s-%s",
		RuntimeCompilerGCC, cfg.CompilerVersion, cfg.CRuntime, cfg.CRuntimeVersion, cfg.Architecture)
}

// ConstructRuntimeIDFromGxxConfig constructs a runtime ID from a GxxConfig
func ConstructRuntimeIDFromGxxConfig(cfg GxxConfig) string {
	return fmt.Sprintf("native_linux-cpp-%s-%s-%s-%s-%s-%s",
		RuntimeCompilerGCC, cfg.CompilerVersion, cfg.CRuntime, cfg.CRuntimeVersion, cfg.CppStdlib, cfg.Architecture)
}

// ConstructRuntimeIDFromClangConfig constructs a runtime ID from a ClangConfig
func ConstructRuntimeIDFromClangConfig(cfg ClangConfig) string {
	return fmt.Sprintf("native_linux-c-%s-%s-%s-%s-%s",
		RuntimeCompilerClang, cfg.CompilerVersion, cfg.CRuntime, cfg.CRuntimeVersion, cfg.Architecture)
}

// ConstructRuntimeIDFromClangxxConfig constructs a runtime ID from a ClangxxConfig
func ConstructRuntimeIDFromClangxxConfig(cfg ClangxxConfig) string {
	return fmt.Sprintf("native_linux-cpp-%s-%s-%s-%s-%s-%s",
		RuntimeCompilerClang, cfg.CompilerVersion, cfg.CRuntime, cfg.CRuntimeVersion, cfg.CppStdlib, cfg.Architecture)
}
