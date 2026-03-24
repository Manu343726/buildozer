# Compiler CLI Interface Reference

## Overview

This document describes the CLI interfaces for GCC, G++, Clang, and Clang++ to guide the Buildozer driver implementations.

## Compatibility Analysis

### Similarities (All Four Compilers)

All four compilers share the same fundamental interface:
- Basic flags: `-c` (compile only), `-o <file>` (output), `-S` (assembly), `-E` (preprocess)
- Defines: `-D<macro>=<value>`
- Includes: `-I<dir>`
- Libraries: `-l<lib>`, `-L<dir>`
- Warnings: `-W*` flags
- Optimization: `-O0`, `-O1`, `-O2`, `-O3`
- Debugging: `-g`, `-g0`, `-g1`, `-g3`
- Language: `-std=<standard>`
- Generic: `-v` (verbose), `--version`
- Input files: `.c`, `.cpp`, `.cc`, `.cxx`, `.o` files

### Differences

#### GCC/G++
```
Usage: gcc [options] file...
```
- Traditional GNU Compiler Collection
- Uses `-v` and `###` for verbose output
- `-pass-exit-codes` flag (unique to GCC)
- `-print-*` family of inspection flags
- `-save-temps`, `-pipe` flags
- `-specs=<file>` for spec files

#### Clang/Clang++
```
OVERVIEW: clang LLVM compiler
USAGE: clang [options] file...
```
- LLVM-based compiler with different internal representation
- Supports all GCC-compatible flags (mostly)
- Additional flags: `-emit-llvm`, `-emit-ast`
- Additional OpenCL support
- Additional CUDA support
- Configuration: `--config <value>`
- `-fapple-*` flags for macOS-specific features

## Key Findings for Driver Implementation

### For Buildozer Drivers: GCC-Compatible Mode
All four compilers support a GCC-compatible interface sufficient for Buildozer:
- **Compilation:** `-c`, `-o`, source files → object files
- **Flag Handling:** All standard flags pass through
- **Runtime Detection:** Via compiler version and `-v` output parsing
- **Language Detection:** Via command name (gcc/clang = C, g++/clang++ = C++)

### Implementation Strategy
Since all four compilers share the GCC-compatible interface, they can all use the same driver implementation with minimal modification:

1. **Shared Logic:** All use `RunCppDriver` from `gcc_common`
2. **Language Detection:** Via binary name and `LanguageType` enum
3. **Compiler Detection:** Via runtime resolution (will detect gcc, clang, etc.)
4. **No Special Handling Required:** Clang accepts all GCC flags we use

### Runtime Identification
The runtime system (pkg/runtimes/cpp/native/detector.go) already generically detects:
- gcc, g++
- clang, clang++

Both are handled the same way in the runtime detection matrix, no special compilation flags needed.

## References

### GCC Help Output
```
Usage: gcc [options] file...
Options:
  -c                       Compile and assemble, but do not link.
  -o <file>                Place the output into <file>.
  -E                       Preprocess only; do not compile, assemble or link.
  -S                       Compile only; do not assemble or link.
  -D <macro>=<value>       Define <macro> to <value>
  -I <dir>                 Add directory to include search path
  -l<lib>                  Link against library
  -L <dir>                 Add directory to library search path
  -std=<standard>          C/C++ standard (c99, c++17, etc.)
  -v                       Display the programs invoked by the compiler.
  --version                Display compiler version information.
  -g                       Produce debugging information
  -O0, -O1, -O2, -O3       Optimization levels
  -W*                      Warning options
  -f*                      Various feature flags
  -m*                      Machine-specific options
```

### Clang Help Output
```
OVERVIEW: clang LLVM compiler
USAGE: clang [options] file...
OPTIONS:
  -c                      Only run preprocess, compile, and assemble steps
  -o <file>               Place output in <file>
  -E                      Only run the preprocessor
  -D <macro>=<value>      Define <macro> to <value>
  -I <dir>                Add directory to include search path
  -l<lib>                 Link against library
  -L <dir>                Add directory to library search path
  -std=<standard>         C/C++ standard
  -v                      Show commands run and use verbose output
  --version               Print the compiler version
  -g                      Generate source-level debug information
  -O0, -O1, -O2, -O3      Optimization levels
  -W*                     Warning options
  -f*                     Feature flags
  -m*                     Machine-specific options
  --config <value>        Configuration file path
  -emit-llvm              Use LLVM intermediate representation
  -emit-ast               Emit Clang AST files
```

## Conclusion

For Buildozer purposes, **Clang can be treated identically to GCC** from the driver perspective because:
1. All GCC command-line flags work in Clang
2. Both detectable via `-v` output parsing
3. Both produce `.o` object files and executables with same calling conventions
4. Both support the same C/C++ standards
5. Clang is designed to be command-line compatible with GCC

Therefore, the driver implementation can simply add clang/clang++ entry points that use the exact same shared execution path as gcc/g++.
