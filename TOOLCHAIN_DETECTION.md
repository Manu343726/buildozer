# C/C++ Toolchain Detection Guide

## Overview

The toolchain detection system enables buildozer drivers and runtimes to automatically discover and validate C/C++ compilation environments on the system. This ensures jobs are only submitted for already-available toolchains, preventing errors after job submission.

## Architecture

### Three-Layer Design

1. **Detection Layer** (`pkg/runtimes/cpp/native/detector.go`)
   - Low-level interaction with filesystem and compiler executables
   - Runs compiler queries to extract version, architecture, runtime info
   - Returns raw `Toolchain` objects with all metadata

2. **Registry Layer** (`pkg/toolchain/registry.go`)
   - Caches toolchains detected at startup
   - Provides high-level query API (GetGCC, GetGxx, CanExecute, etc.)
   - Thread-safe singleton pattern for global access

3. **Driver Layer** (`cmd/gcc/main.go`, `cmd/g++/main.go`)
   - Uses registry to verify compiler availability before job creation
   - Provides user-friendly error messages if compiler missing
   - Optionally logs detected toolchain metadata

## Detected Information

For each discovered compiler, the system detects:

| Property | Example | Purpose |
|----------|---------|---------|
| Compiler Name | gcc, g++, clang, clang++ | Identifies exact executable |
| Compiler Path | /usr/bin/gcc | Full filesystem path for later execution |
| Version | 10.2.1 | Ensures compatibility with requested features |
| Language | C / C++ | C vs C++ mode |
| Architecture | x86_64 | Target hardware platform |
| C Runtime | glibc / musl | Standard C library implementation |
| C Runtime Version | 2.31 | Glibc/musl specific version |
| C++ Stdlib | libstdc++ / libc++ | C++ standard library for C++ jobs |
| C++ ABI | Itanium | Application Binary Interface |
| ABI Modifiers | -D_GLIBCXX_USE_CXX11_ABI=1 | Runtime flags (gcc C++11 ABI) |

## Usage Examples

### For Drivers

```go
import "github.com/Manu343726/buildozer/pkg/toolchain"

func main() {
    ctx := context.Background()
    registry := toolchain.Global()
    
    // Initialize at startup
    if err := registry.Initialize(ctx); err != nil {
        log.Warn("toolchain detection failed", "error", err)
    }
    
    // Verify GCC is available before using
    gcc := registry.GetGCC(ctx)
    if gcc == nil {
        log.Error("gcc not found on system")
        os.Exit(1)
    }
    
    log.Info("verified gcc", "path", gcc.CompilerPath, "version", gcc.CompilerVersion)
}
```

### For Runtimes

```go
import "github.com/Manu343726/buildozer/pkg/toolchain"

// Check if we can execute a specific job
func canHandleJob(jobRequest *v1.Job) bool {
    registry := toolchain.Global()
    
    // Extract compiler and language from proto
    compiler := native.CompilerGCC // or CompilerClang
    language := native.LanguageC    // or LanguageCpp
    
    // Check if available
    return registry.CanExecute(compiler, language, native.ArchitectureUnspecified)
}
```

### For Debugging Users

Run the example utility to see what's available:

```bash
go run examples/toolchain_detection.go
```

Output:
```
Found 4 toolchain(s):

1. GCC C (v10.2.1-6) x86_64 [glibc, unknown@unknown]
   Path: /usr/bin/gcc
   Version: 10.2.1-6

2. GCC C++ (v10.2.1-6) x86_64 [glibc, libstdc++@itanium]
   Path: /usr/bin/g++
   Version: 10.2.1-6

3. Clang C (v11.0.1-2) x86_64 [glibc, unknown@unknown]
   Path: /usr/bin/clang
   Version: 11.0.1-2

4. Clang++ C++ (v11.0.1-2) x86_64 [glibc, libc++@itanium]
   Path: /usr/bin/clang++
   Version: 11.0.1-2
```

## Detection Methods

### Compiler Detection

Checks system PATH for known compiler executables:
- GCC / G++ (gcc, g++)
- Clang / Clang++ (clang, clang++)

```go
gcc := detector.DetectGCC(ctx)           // Returns *Toolchain or nil
toolchains := detector.DetectToolchains(ctx)  // Returns []Toolchain
```

### Version Detection

Runs compiler with `--version` flag and parses output:
```bash
$ gcc --version
gcc (Debian 10.2.1-6) 10.2.1
...
```

Extracting: `10.2.1-6)`

### Architecture Detection

Maps Go runtime GOARCH to compiler target:
- amd64 → x86_64
- arm64 → aarch64  
- arm → arm

### C Runtime Detection

1. Queries compiler for libc path:
   ```bash
   $ gcc -print-file-name=libc.so.6
   /lib64/libc.so.6
   ```

2. Detects glibc version:
   ```bash
   $ ldd --version | head -1
   ldd (GNU libc) 2.31
   ```

3. Fallback: Searches filesystem for musl:
   ```bash
   find /lib -name 'ld-musl-*.so.1' | head -1
   ```

### C++ Stdlib Detection

For GCC:
- Default: libstdc++
- Detects C++11 ABI via predefine check

For Clang:
- Tests `-stdlib=libc++` support
- Falls back to libstdc++ on Linux

## API Reference

### Registry Methods

```go
// Initialization
registry := toolchain.Global()
err := registry.Initialize(ctx)

// Query specific toolchains
gcc := registry.GetGCC(ctx)           // Returns *native.Toolchain
gxx := registry.GetGxx(ctx)
clang := registry.GetClang(ctx)
clangxx := registry.GetClangxx(ctx)

// Query by enum
tc := registry.GetByCompilerAndLanguage(
    native.CompilerGCC, 
    native.LanguageC,
)

// List all
toolchains := registry.ListToolchains()  // Returns []*native.Toolchain

// Execution check
canRun := registry.CanExecute(
    native.CompilerGCC,
    native.LanguageC,
    native.ArchitectureUnspecified,
)

// Summary
summary := registry.Summary()  // Returns string
```

### Toolchain Fields

```go
type Toolchain struct {
    Language             native.Language    // C or C++
    Compiler             native.Compiler    // GCC or Clang
    CompilerPath         string            // /usr/bin/gcc
    CompilerVersion      string            // 10.2.1-6
    Architecture         native.Architecture // x86_64, aarch64, arm
    CRuntime             native.CRuntime     // Glibc, Musl
    CRuntimeVersion      string            // 2.31
    CppStdlib            native.CppStdlib    // Libstdc++, Libc++
    CppAbi               native.CppAbi       // Itanium
    AbiModifiers         []string          // ["-D_GLIBCXX_USE_CXX11_ABI=1"]
}

// String representation
fmt.Println(tc.String())  // "GCC C++ (v10.2.1-6) x86_64 [glibc, libstdc++@itanium]"
```

## Testing

Run toolchain detection tests:

```bash
# Detector tests
go test ./pkg/runtimes/cpp/native -v

# Registry tests
go test ./pkg/toolchain -v

# All toolchain tests
go test ./pkg/runtimes/cpp/native ./pkg/toolchain -v
```

Example test output:
```
=== RUN   TestDetectGCC
--- PASS: TestDetectGCC (0.00s)
=== RUN   TestDetectToolchains
--- PASS: TestDetectToolchains (0.08s)
=== RUN   TestRegistryInitialize
--- PASS: TestRegistryInitialize (0.08s)
...
PASS
ok      github.com/Manu343726/buildozer/pkg/runtimes/cpp/native  (cached)
ok      github.com/Manu343726/buildozer/pkg/toolchain            0.480s
```

## Error Handling

### System Without Compilers

When no compilers are detected:
```
Detecting available C/C++ toolchains...
No C/C++ toolchains detected on this system
```

### Graceful Degradation

If detection fails during driver startup:
```go
if err := registry.Initialize(ctx); err != nil {
    log.Warn("failed to detect toolchains", "error", err)
    // Continue - will fail later if compiler actually missing
}
```

### Driver-Level Errors

Clear user feedback when compiler missing:
```
gcc: error: gcc not found on this system
```

## Performance

- **Detection Time:** ~100-500ms depending on system (cached after first run)
- **Memory:** Minimal (single Toolchain object per detected compiler)
- **Cache:** Global singleton maintains single Toolchain list

## Current Limitations

1. **Linux-Only:** Currently supports Linux. Windows/macOS require different detection methods
2. **GCC/Clang Only:** Doesn't detect other compilers (MSVC, Intel ICC, etc.)
3. **Static Architecture:** Uses Go runtime GOARCH, doesn't query compiler directly
4. **Basic Glibc Detection:** Simplified detection (doesn't parse all versions)

## Future Improvements

1. Query compiler directly for target architecture
2. Detect compiler-specific capabilities (C++17, C++20 support)
3. Windows/macOS support (MSVC, XCode Clang)
4. Extended runtime detection (musl version accuracy)
5. Caching to disk for faster startup
6. Network distribution (publish available toolchains to peers)

EOF
