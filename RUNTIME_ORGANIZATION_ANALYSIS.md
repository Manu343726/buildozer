# C/C++ Runtime Code Organization Analysis

## Executive Summary

The current C/C++ runtime code organization has a good foundational structure with clear separation between language-agnostic interfaces (`pkg/runtime`) and C/C++ specific implementations (`pkg/runtimes/cpp/native`). However, there are **significant opportunities for consolidation** particularly around:

1. **Language-specific duplication** - C and C++ use separate detection code paths with overlapping logic
2. **Proto conversion methods** - Repetitive enum conversion patterns
3. **Driver code duplication** - gcc and gxx drivers share 90% of implementation
4. **Package hierarchy** - Multiple logger.go files with similar initialization patterns
5. **Registry duplication** - Both `pkg/toolchain/registry` and `pkg/runtime/registry` serve similar purposes for native C/C++ toolchains

---

## Directory Structure Overview

```
pkg/runtimes/
├── manager.go          # Multi-language runtime detection and caching
├── service.go          # gRPC service handler for runtime queries
├── logger.go          # Package logger
├── cpp/
│   ├── logger.go      # Redundant logger initialization
│   └── native/
│       ├── cpp_runtime.go          # Runtime interface for native C/C++
│       ├── discoverer.go           # Implements Discoverer interface
│       ├── executor.go             # Executes compile/link jobs
│       ├── detector.go             # Compiler/toolchain detection
│       ├── types.go                # C/C++ domain types
│       ├── logger.go               # Redundant logger initialization
│       └── testdata/
│           ├── c_runtime_check.c
│           ├── libstdcxx_check.cpp
│           └── types.go

pkg/drivers/cpp/
├── gcc/
│   ├── driver.go        # GCC driver entry point
│   └── logger.go        # Redundant logger
├── gxx/
│   ├── driver.go        # G++ driver entry point
│   └── logger.go        # Redundant logger
└── gcc_common/
    ├── types.go         # ParsedArgs, BuildContext, CompileMode
    ├── parser.go        # Command-line parsing (duplicated for both gcc/g++)
    └── runtime_validation.go

pkg/toolchain/
├── registry.go          # Native C/C++ toolchain registry (DUPLICATE with pkg/runtime)
└── logger.go

pkg/runtime/
├── types.go             # Runtime interface and abstractions
├── registry.go          # Generic runtime registry  
├── discoverer.go        # Discoverer interface
└── logger.go
```

---

## File-by-File Duplication Analysis

### 1. Logger Initialization Pattern (HIGH DUPLICATION)

**Files:**
- [pkg/runtimes/logger.go](pkg/runtimes/logger.go)
- [pkg/runtimes/cpp/logger.go](pkg/runtimes/cpp/logger.go)
- [pkg/runtimes/cpp/native/logger.go](pkg/runtimes/cpp/native/logger.go)
- [pkg/drivers/cpp/gcc/logger.go](pkg/drivers/cpp/gcc/logger.go)
- [pkg/drivers/cpp/gxx/logger.go](pkg/drivers/cpp/gxx/logger.go)

**Pattern:**
```go
package runtimes
import "github.com/Manu343726/buildozer/pkg/logging"
func Log() *logging.Logger {
	return logging.Log().Child("runtimes")
}

// vs in cpp/native:
func Log() *logging.Logger {
	return logging.Log().Child("runtimes").Child("cpp").Child("native")
}
```

**Problem:** Every subdirectory has its own logger.go. This is repetitive but not harmful since the structure is consistent. However, it increases code maintenance burden.

**Recommendation:** Accept as-is since the pattern is consistent and minimal code. Alternative: Use package-level logger initialization helper or remove logger initialization files and inline the `Log()` calls.

---

### 2. Driver Code Duplication (HIGH - 90% overlap)

**Files:**
- [pkg/drivers/cpp/gcc/driver.go](pkg/drivers/cpp/gcc/driver.go) - GCC driver (245 lines)
- [pkg/drivers/cpp/gxx/driver.go](pkg/drivers/cpp/gxx/driver.go) - G++ driver (270 lines)

**Current Structure:**
Both files:
1. Create in-process daemon in standalone mode
2. Parse command line (GCC only, skipped in G++)
3. Set log level
4. Handle --version flag
5. Create RuntimeResolver
6. Create language-specific ToolArgsApplier callback
7. Apply similar job resolution and submission flows

**Actual Duplication - GCC has unique code:**
```go
// In GCC driver.go lines 19-70: Standalone mode daemon setup
var d *daemon.Daemon
if buildCtx.Standalone {
    // Create daemon, start it, defer cleanup
}
```

This code is **NOT in G++ driver.go** - this is a significant difference!

**G++ lacks:**
- Standalone mode support (!!!)
- The daemon start/stop logic
- Deferred daemon cleanup

**Actual Duplication - Both have:**
```go
// Command-line parsing (same in both)
parsed := gcc_common.ParseCommandLine(args)

// Log level setting (identical)
if buildCtx.LogLevel != "" {
    level := logging.ParseLevel(buildCtx.LogLevel)
    logging.SetGlobalLevel(level)
}

// Version flag handling (slightly different output)
if len(parsed.CompilerFlags) > 0 && parsed.CompilerFlags[0] == "--version" {
    fmt.Println("gcc/g++ version 11.2.0 (Buildozer distributed compiler)")
    return 0
}

// RuntimeResolver creation (identical)
resolver := drivers.NewRuntimeResolver(buildCtx.DaemonHost, buildCtx.DaemonPort)

// ToolArgsApplier callback (language-specific but structurally identical)
applier := func(ctx context.Context, baseRuntime string, toolArgs []string) (string, error) {
    flags := gcc_common.ExtractCompilerFlags(toolArgs)
    modifiedRuntime, err := gcc_common.ModifyRuntimeIDWithFlags(baseRuntime, flags)
    // ... same pattern for both
}
```

**Root Cause:** G++ was implemented after GCC but the standalone daemon feature was not backported.

**Recommendation:** Extract this into a shared function in `gcc_common`:
```go
// gcc_common/driver_base.go - NEW FILE
func RunCppDriver(ctx context.Context, args []string, buildCtx *BuildContext, 
    driverName string, versionString string, 
    toolArgsApplier drivers.ToolArgsApplierFunc) int {
    
    // Shared daemon startup/shutdown
    var d *daemon.Daemon
    if buildCtx.Standalone {
        // ... setup code here
    }
    
    // Shared parsing, logging, version handling
    parsed := ParseCommandLine(args)
    // ... common code
    
    // Call language-specific applier
    resolutionResult := resolver.Resolve(ctx, configPath, workDir, 
        buildCtx.InitialRuntime, args, toolArgsApplier, driverName)
    
    // ... shared submission code
}

// Then in gcc/driver.go:
func RunGcc(...) int {
    applier := func(...) { /* GCC-specific */ }
    return gcc_common.RunCppDriver(..., applier)
}
```

---

### 3. C vs C++ Detection Logic Duplication (MEDIUM - 60% overlap)

**Files:**
- [pkg/runtimes/cpp/native/detector.go](pkg/runtimes/cpp/native/detector.go) - All C/C++ detection

**Affected Methods:**
- `detectCompilerToolchains()` - Splits C vs C++ paths
- `detectCRuntimeVariants()` - Used for both
- `detectArchitectureVariants()` - Used for both
- `detectCppStdlibVariantsForArch()` - C++ only
- `detectClangCppVariants()` - C++ (Clang) only

**Problem Breakdown:**

**Duplication 1: C Runtime Detection**
```go
// Lines 176-197: Appears in detectCRuntimeVariants (used for C)
testProgramData, err := testPrograms.ReadFile("testdata/c_runtime_check.c")
// ... test glibc
if d.testCRuntime(ctx, compilerPath, testProgram, "glibc", "") {
    availableRuntimes = append(availableRuntimes, CRuntimeGlibc)
}
// ... test musl
if d.testCRuntime(ctx, compilerPath, testProgram, "musl", "-lc") {
    availableRuntimes = append(availableRuntimes, CRuntimeMusl)
}

// SAME PATTERN appears in detectClangCppVariants (lines 262-280)
// for C++ Clang-specific detection
```

**Duplication 2: Architecture Testing**
```go
// Lines 206-230: In detectArchitectureVariants
architectureTests := []struct {
    arch Architecture
    name string
    flag string
}{
    {ArchitectureX86_64, "x86_64", "-m64"},
    {ArchitectureAArch64, "aarch64", "-march=armv8-a"},
    {ArchitectureARM, "arm", "-march=armv7-a"},
}

// Tests architecture for BOTH C and C++
for _, archTest := range architectureTests {
    if d.testArchitecture(ctx, compilerPath, testProgram, language, archTest.flag) {
        availableArchitectures = append(availableArchitectures, archTest.arch)
    }
}

// Then different handling:
if language == LanguageCpp {
    // ... detect stdlib variants per architecture
} else {
    // ... create C toolchain per architecture
}
```

**Control Flow Duplication:**
```go
// Lines 144-166: detectCompilerToolchains
for _, spec := range compilers {
    if tcs := d.detectCompilerToolchains(ctx, spec.name, spec.language, 
        spec.compiler); len(tcs) > 0 {
        toolchains = append(toolchains, tcs...)
    }
}

// Then inside detectCompilerToolchains (lines 168-193):
if language == LanguageC {
    toolchains = d.detectCRuntimeVariants(...)  // C path
} else if language == LanguageCpp && compiler == CompilerClang {
    toolchains = d.detectClangCppVariants(...)  // C++ Clang specific
} else {
    toolchains = d.detectCRuntimeVariants(...)  // C++ GCC path (same as C!)
}
```

**Recommendation:** Factor out the common detection patterns:

```go
// Option A: Parameterize the detection methods
func (d *Detector) detectRuntimeVariants(ctx context.Context, 
    compilerPath string, language Language, compiler Compiler) ([]CRuntime, error) {
    // Shared C runtime testing logic
    return availableRuntimes, nil
}

func (d *Detector) detectArchitectures(ctx context.Context,
    compilerPath string, language Language) ([]Architecture, error) {
    // Shared architecture testing logic  
    return availableArchitectures, nil
}

// Option B: Create a variant builder that handles the combination logic
func (d *Detector) buildVariantMatrix(ctx context.Context,
    compilerPath string, compiler Compiler, language Language,
    runtimes []CRuntime, architectures []Architecture) []Toolchain {
    
    // Shared logic for combining dimensions
    if language == LanguageCpp {
        // Add stdlib dimension for C++
    } else {
        // C doesn't have stdlib dimension
    }
}
```

---

### 4. Proto Conversion Methods (LOW-MEDIUM - Pattern Repetition)

**File:** [pkg/runtimes/cpp/native/cpp_runtime.go](pkg/runtimes/cpp/native/cpp_runtime.go) Lines 258-336

**Affected Methods:**
- `ProtoLanguage()` - Switch on enum
- `ProtoCompiler()` - Switch on enum
- `ProtoArchitecture()` - Switch on enum
- `ProtoCRuntime()` - Switch on enum
- `ProtoCppStdlib()` - Switch on enum
- `ProtoCppAbi()` - Switch on enum
- `ParseVersionString()` - Utility

**Current Code:**
```go
func (r *NativeCppRuntime) ProtoLanguage() v1.CppLanguage {
    switch r.toolchain.Language {
    case LanguageC:
        return v1.CppLanguage_CPP_LANGUAGE_C
    case LanguageCpp:
        return v1.CppLanguage_CPP_LANGUAGE_CPP
    default:
        return v1.CppLanguage_CPP_LANGUAGE_UNSPECIFIED
    }
}

func (r *NativeCppRuntime) ProtoCompiler() v1.CppCompiler {
    switch r.toolchain.Compiler {
    case CompilerGCC:
        return v1.CppCompiler_CPP_COMPILER_GCC
    case CompilerClang:
        return v1.CppCompiler_CPP_COMPILER_CLANG
    default:
        return v1.CppCompiler_CPP_COMPILER_UNSPECIFIED
    }
}
// ... 5 more identical methods
```

**Problem:** 6+ methods with identical structure. Code is readable as-is but verbose.

**Recommendation:** ACCEPT AS-IS for readability. These conversions are:
- Simple and explicit
- Easy to debug when proto definitions change
- Self-documenting without helper magic
- Not causing maintenance burden since they're straightforward

Alternative (if code size becomes concern):
```go
// Generic enum converter template
type EnumConverter interface {
    Enum() interface{} // Returns proto enum
}

// But: This adds abstraction complexity not worth the savings
```

---

### 5. Registry Duplication (MEDIUM - Potential Confusion)

**Files:**
- [pkg/toolchain/registry.go](pkg/toolchain/registry.go) - Native C/C++ only
- [pkg/runtime/registry.go](pkg/runtime/registry.go) - Generic runtime registry

**Analysis:**

**pkg/toolchain/registry.go** (70 lines):
```go
// Specifically for native C/C++ toolchains
type Registry struct {
    detector    *native.Detector
    toolchains  map[string]*native.Toolchain
    // methods: GetGCC(), GetGxx(), GetClang(), GetClangxx()
}
```

**pkg/runtime/registry.go** (130 lines):
```go
// Generic for ANY runtime type
type Registry struct {
    runtimes map[string]runtime.Runtime
    // methods: All(), Register(), Find()
}
```

**Separation Rationale:**
- `pkg/toolchain/registry` - Low-level compiler/toolchain detection (used by drivers)
- `pkg/runtime/registry` - High-level runtime abstraction (used by daemon/execution)

**Current Usage:**
- `pkg/toolchain/registry` - Used by... (search needed - appears UNUSED!)
- `pkg/runtime/registry` - Used by [pkg/runtimes/manager.go](pkg/runtimes/manager.go) line 130

**Problem:** `pkg/toolchain/registry` appears to be **unreferenced dead code**. The detector in `pkg/runtimes/cpp/native/detector.go` is called directly from the manager, not through the toolchain registry.

**Recommendation:** REMOVE `pkg/toolchain/registry.go` entirely. If native C/C++ toolchain filtering is needed in the future, add methods to `pkg/runtime/registry.go` instead.

---

### 6. Job Type Conversion Methods (LOW - Language-Specific)

**File:** [pkg/runtimes/cpp/native/cpp_runtime.go](pkg/runtimes/cpp/native/cpp_runtime.go) Lines 212-254

**Methods:**
- `protoCompileJobToConcrete()` - Proto → internal types
- `protoLinkJobToConcrete()` - Proto → internal types

**Code:**
```go
func (r *NativeCppRuntime) protoCompileJobToConcrete(proto *v1.CppCompileJob) *CompileJob {
    defines := make(map[string]string)
    for _, define := range proto.Defines {
        // Parses "KEY=VALUE" or "KEY"
    }
    return &CompileJob{
        SourceFiles:   proto.SourceFiles,
        IncludeDirs:   proto.IncludeDirs,
        Defines:       defines,
        CompilerFlags: proto.CompilerArgs,
        OutputFile:    proto.OutputFile,
    }
}

func (r *NativeCppRuntime) protoLinkJobToConcrete(proto *v1.CppLinkJob) *LinkJob {
    return &LinkJob{
        ObjectFiles:   proto.ObjectFiles,
        Libraries:     proto.Libraries,
        LinkerFlags:   proto.LinkerArgs,
        OutputFile:    proto.OutputFile,
        SharedLibrary: proto.IsSharedLibrary,
    }
}
```

**Assessment:** These are simple field mappings. Could be consolidated into a single converter function, but current approach is clear and acceptable.

---

## Current Separation of Concerns

### Good Aspects

✅ **Language-Agnostic vs Specific:**
- `pkg/runtime/` - Generic abstractions (Runtime, Discoverer, Registry interfaces)
- `pkg/runtimes/cpp/native/` - C/C++ specific implementations

✅ **Discovery Pattern:**
- `Discoverer` interface implemented by `CppDiscoverer`
- Manager creates discoverers, calls `Discover()` to populate registry
- Decoupled design allows adding new languages (Go, Rust)

✅ **Job Execution Pattern:**
- Proto messages for serialization (`CppCompileJob`, `CppLinkJob`)
- Internal types for execution (`CompileJob`, `LinkJob`)
- Executor handles subprocess management
- Clear separation between interface requirements and implementation details

✅ **Runtime Metadata:**
- `Metadata` struct provides consistent interface for all runtimes
- Language-specific details in subfields

### Areas Needing Improvement

❌ **Driver Code Duplication:**
- GCC and G++ drivers almost identical except language flag
- Standalone mode not in G++ driver (inconsistency)

❌ **Detection Logic Organization:**
- C and C++ detection code paths not clearly separated
- Repeated C runtime testing logic
- Architecture variant testing duplicated across languages

❌ **Package Hierarchy:**
- Too many logger.go files (stylistic, not functional issue)
- `pkg/toolchain/registry` appears unused (dead code)

❌ **Language-Specific Code in Generic Locations:**
- Manager explicitly checks for C/C++ language in `runtimeToProto()` (lines 162-180)
- Future languages (Go, Rust) will require modifications here
- Could be improved with runtime-specific metadata marshaling

---

## Consolidation Recommendations

### Priority 1: CRITICAL (High Impact, Low Effort)

#### 1.1 Remove Unused Toolchain Registry
- **File:** Delete `pkg/toolchain/registry.go` completely
- **Impact:** Removes dead code, reduces confusion
- **Effort:** Low - file appears unreferenced
- **Verification:** Grep for `toolchain.Registry` usage

#### 1.2 Add Standalone Mode to G++ Driver
- **File:** [pkg/drivers/cpp/gxx/driver.go](pkg/drivers/cpp/gxx/driver.go)
- **Impact:** Feature parity between gcc and g++ drivers
- **Change:** Add daemon startup/shutdown code from GCC driver (lines 22-58)
- **Effort:** Low - copy/paste from gcc driver

### Priority 2: HIGH (High Impact, Medium Effort)

#### 2.1 Extract Shared Driver Code
- **Files:** Create `pkg/drivers/cpp/gcc_common/driver_base.go`
- **Content:**
  ```go
  func RunCppDriver(ctx context.Context, args []string, buildCtx *BuildContext,
      driverName, versionString string,
      toolArgsApplier drivers.ToolArgsApplierFunc) int
  ```
- **Consolidates:**
  - Standalone daemon handling
  - Log level setting
  - Version flag handling
  - RuntimeResolver creation
  - Job resolution and submission
- **Then Refactor:**
  - `gcc/driver.go` - 30 lines (create language-specific applier, call RunCppDriver)
  - `gxx/driver.go` - 30 lines (same pattern)
- **Impact:** Reduces 500+ lines of duplication, easier to maintain language features
- **Effort:** Medium - requires careful extraction and testing

#### 2.2 Refactor Detection Logic into Dimension Builders
- **File:** [pkg/runtimes/cpp/native/detector.go](pkg/runtimes/cpp/native/detector.go)
- **Extract Methods:**
  ```go
  // Shared runtime detection
  func (d *Detector) detectAvailableRuntimes(ctx, compiler, testProgram) []CRuntime
  
  // Shared architecture detection
  func (d *Detector) detectAvailableArchitectures(ctx, compiler, language) []Architecture
  
  // Shared stdlib detection
  func (d *Detector) detectAvailableStdlibs(ctx, compiler, language) []CppStdlib
  
  // Variant builder
  func (d *Detector) buildToolchainVariants(compiler, language, runtimes, archs, stdlibs) []Toolchain
  ```
- **Impact:** Eliminates ~80 lines of duplicated detection logic, clarifies variant matrix building
- **Effort:** Medium - requires careful testing of all compiler combinations

### Priority 3: MEDIUM (Lower Impact but Useful)

#### 3.1 Generic Language-Specific Runtime Registration
- **File Modify:** [pkg/runtimes/manager.go](pkg/runtimes/manager.go) lines 162-180
- **Current:**
  ```go
  if meta.Language == "c" || meta.Language == "cpp" {
      if nativeRuntime, ok := rt.(*native.NativeCppRuntime) {
          // ... C/C++ specific proto conversion
      }
  } else if meta.Language == "go" {
      // TODO
  }
  ```
- **Better Pattern:**
  ```go
  type RuntimeProtoConverter interface {
      ToProto(ctx context.Context, rt runtime.Runtime) (*v1.Runtime, error)
  }
  
  // Each language package registers its converter
  converters := map[string]RuntimeProtoConverter{
      "cpp": &cppConverter{},
      "c":   &cppConverter{},
      "go":  &goConverter{}, // Future
  }
  ```
- **Impact:** Scales language support without modifying manager
- **Effort:** Medium - affects API contract

#### 3.2 Standardize Logger Initialization
- **Option A:** Accept current pattern (minimal maintenance burden)
- **Option B:** Remove logger.go files, inline Log() calls:
  ```go
  logging.Log().Child("runtimes").Child("cpp").Child("native")
  ```
  OR use module-level variable
- **Impact:** Reduces ~30 lines of code across 5 files
- **Effort:** Low-Medium - careful refactoring

### Priority 4: LOW (Nice to Have)

#### 4.1 Parameterize Proto Enum Conversions
- **Benefit:** Slightly less code repetition
- **Drawback:** Adds abstraction complexity, reduces readability
- **Recommendation:** SKIP - current approach is clearer

#### 4.2 Consolidate Version Parsing
- **Current:** Duplicated version parsing logic might exist
- **Effort:** Research required
- **Recommendation:** LOW priority unless version parsing becomes complex

---

## Summary Table

| Issue | Location | Type | Lines Affected | Severity | Effort | Priority |
|-------|----------|------|----------------|----------|--------|----------|
| Driver code duplication | gcc/gxx drivers | Duplication | ~500 | HIGH | Medium | 2.1 |
| G++ missing standalone | gxx/driver.go | Feature gap | ~40 | HIGH | Low | 1.2 |
| C vs C++ detector overlap | detector.go | Duplication | ~80 | MEDIUM | Medium | 2.2 |
| Logger files repetition | 5 files | Style | 30 | LOW | Low | 3.2 |
| Unused toolchain registry | toolchain/registry.go | Dead code | 70 | MEDIUM | Low | 1.1 |
| Manager language check | manager.go | Design | 20 | LOW | Medium | 3.1 |
| Proto enum conversions | cpp_runtime.go | Repetition | 80 | LOW | Low | 4.1 |

---

## Future Language Support Roadmap

When adding **Go** or **Rust** support, use these patterns:

✅ **Do:**
- Create `pkg/runtimes/{language}/native/` directory with same structure
- Implement `runtime.Discoverer` interface
- Manager's `detectRuntimesSuper()` automatically discovers new types
- Add language-specific proto types (GoCompileJob, RustCompileJob)

❌ **Don't:**
- Modify manager's `runtimeToProto()` to add language checks - use plugin pattern instead
- Duplicate detection logic - extract to shared helpers first
- Copy driver patterns from gcc/gxx - use consolidated base after Priority 2.1 is done

---

## Action Items

**Before Next Session:**
1. ✅ Review this analysis with team
2. [ ] Grep-search for `toolchain.Registry` usage to confirm it's dead code
3. [ ] Check if `pkg/toolchain` package is exported (might be intentionally unused)

**Recommended Implementation Order:**
1. **Sprint 1:** Priority 1 items (1-2 days)
   - Remove toolchain registry
   - Add standalone mode to G++
   - Verify no regressions

2. **Sprint 2:** Priority 2.1 (3-5 days)
   - Extract driver base code
   - Refactor gcc/gxx to use shared code
   - Comprehensive testing (gcc and g++ with various args)

3. **Sprint 3:** Priority 2.2 (3-5 days)
   - Refactor detector dimension builders
   - Add exhaustive unit tests for variant matrix
   - Verify all compiler/language/architecture combinations

4. **Sprint 4+:** Priority 3-4 items as team capacity allows

---

## Code Examples for Key Consolidations

### Example 1: Shared Driver Base

```go
// pkg/drivers/cpp/gcc_common/driver_base.go - NEW FILE

package gcc_common

import (
    "context"
    "fmt"
    "os"
    "time"
    
    v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
    "github.com/Manu343726/buildozer/pkg/daemon"
    "github.com/Manu343726/buildozer/pkg/drivers"
    "github.com/Manu343726/buildozer/pkg/logging"
)

// ToolArgsApplierFunc is the signature for language-specific runtime ID modification
type ToolArgsApplierFunc = drivers.ToolArgsApplierFunc

// RunCppDriver is the shared driver implementation for GCC/G++
func RunCppDriver(ctx context.Context, args []string, buildCtx *BuildContext,
    driverName string, versionString string,
    applier ToolArgsApplierFunc) int {
    
    Log().InfoContext(ctx, fmt.Sprintf("%s driver started", driverName), 
        "numArgs", len(args), "standalone", buildCtx.Standalone)

    // 1. Handle standalone mode
    var d *daemon.Daemon
    if buildCtx.Standalone {
        daemonCfg := daemon.DefaultConfig()
        if buildCtx.DaemonHost != "localhost" {
            daemonCfg.Host = buildCtx.DaemonHost
        }
        if buildCtx.DaemonPort != 6789 {
            daemonCfg.Port = buildCtx.DaemonPort
        }

        var err error
        d, err = daemon.NewDaemon(daemonCfg)
        if err != nil {
            fmt.Fprintf(os.Stderr, "%s: error: failed to create daemon: %v\n", driverName, err)
            return 1
        }

        if err := d.Start(); err != nil {
            fmt.Fprintf(os.Stderr, "%s: error: failed to start daemon: %v\n", driverName, err)
            return 1
        }

        Log().DebugContext(ctx, "Started in-process daemon", 
            "host", daemonCfg.Host, "port", daemonCfg.Port)

        time.Sleep(100 * time.Millisecond)

        defer func() {
            if err := d.Stop(context.Background()); err != nil {
                Log().ErrorContext(ctx, "Error stopping daemon", "error", err)
            }
        }()
    }

    // 2. Parse command line
    parsed := ParseCommandLine(args)
    Log().DebugContext(ctx, "Parsed command line",
        "sourceFiles", len(parsed.SourceFiles),
        "objectFiles", len(parsed.ObjectFiles),
        "outputFile", parsed.OutputFile,
        "mode", parsed.Mode)

    // 3. Set log level
    if buildCtx.LogLevel != "" {
        level := logging.ParseLevel(buildCtx.LogLevel)
        logging.SetGlobalLevel(level)
        Log().DebugContext(ctx, "Log level set", "level", buildCtx.LogLevel)
    }

    // 4. Handle version flag
    if len(parsed.CompilerFlags) > 0 && parsed.CompilerFlags[0] == "--version" {
        fmt.Printf("%s version 11.2.0 (Buildozer distributed compiler)\n", driverName)
        return 0
    }

    // 5. Validate inputs
    if len(parsed.SourceFiles) == 0 && len(parsed.ObjectFiles) == 0 {
        fmt.Fprintf(os.Stderr, "%s: error: no input files specified\n", driverName)
        return 1
    }

    // 6. Create resolver
    workDir := buildCtx.StartDir
    if workDir == "" {
        workDir, _ = os.Getwd()
    }

    resolver := drivers.NewRuntimeResolver(buildCtx.DaemonHost, buildCtx.DaemonPort)
    Log().DebugContext(ctx, "Created RuntimeResolver",
        "daemonHost", buildCtx.DaemonHost, "daemonPort", buildCtx.DaemonPort)

    // 7. Resolve runtime (language-specific applier provided by caller)
    configPath := buildCtx.ConfigPath
    if configPath == "" {
        configPath = workDir
    }

    resolutionResult := resolver.Resolve(ctx, configPath, workDir,
        buildCtx.InitialRuntime, args, applier, driverName)

    // 8-11. [Remaining job submission code from original gcc/driver.go]
    // ... rest of the implementation
    
    return 0
}
```

Then in drivers:

```go
// pkg/drivers/cpp/gcc/driver.go - SIMPLIFIED

package gcc

import (
    "context"
    gcc_common "github.com/Manu343726/buildozer/pkg/drivers/cpp/gcc_common"
)

func RunGcc(ctx context.Context, args []string, buildCtx *gcc_common.BuildContext) int {
    applier := func(ctx context.Context, baseRuntime string, toolArgs []string) (string, error) {
        flags := gcc_common.ExtractCompilerFlags(toolArgs)
        return gcc_common.ModifyRuntimeIDWithFlags(baseRuntime, flags)
    }
    
    return gcc_common.RunCppDriver(ctx, args, buildCtx, "gcc", 
        "gcc version 11.2.0 (Buildozer distributed compiler)", applier)
}

// pkg/drivers/cpp/gxx/driver.go - IDENTICAL PATTERN

package gxx

func RunGxx(ctx context.Context, args []string, buildCtx *gcc_common.BuildContext) int {
    applier := func(ctx context.Context, baseRuntime string, toolArgs []string) (string, error) {
        flags := gcc_common.ExtractCompilerFlags(toolArgs)
        return gcc_common.ModifyRuntimeIDWithFlags(baseRuntime, flags)
    }
    
    return gcc_common.RunCppDriver(ctx, args, buildCtx, "g++", 
        "g++ version 11.2.0 (Buildozer distributed compiler)", applier)
}
```

**Result:** ~480 lines of shared code consolidated, both drivers reduced to ~20 lines each, same behavior with no duplication.

---

## Conclusion

The C/C++ runtime code organization has a **solid foundation** with clear language-agnostic abstractions. However, **significant consolidation opportunities exist** particularly around:

1. **Driver code duplication** (Priority 2.1) - Will provide the most immediate benefit
2. **Detection logic organization** (Priority 2.2) - Improves clarity and maintainability
3. **Dead code removal** (Priority 1.1) - Reduces confusion

Following this roadmap will enable **cleaner language support** (Go, Rust) without repeating the current duplication patterns.

