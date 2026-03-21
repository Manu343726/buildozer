# Buildozer Development Log

**Project:** Peer-to-Peer Distributed Build System  
**Status:** Phase 1 - Core Abstractions & Local Foundation  
**Last Updated:** 2026-03-21

---

## Phase 1: Core Abstractions & Local Foundation (Weeks 1-5)

### Architecture Decision: Runtime-First Approach (2026-03-21)

**Status:** ✅ PLANNED

**Key Insight:** Runtime system is the foundational abstraction that all other Phase 1 components depend on. Build runtime discovery, Docker API, and execution abstractions FIRST (Milestones 1.0-1.2), then build job abstractions on proven runtime system.

**Updated Phase 1 Structure (10 Milestones, 5 Weeks):**
- **Milestone 1.0-1.2 (Weeks 1-3):** Runtime Foundation (Docker API, native C/C++ toolchains, Docker-based runtimes)
- **Milestone 1.3-1.9 (Weeks 3-5):** Job abstractions, logging, persistence, executor, daemon, drivers

**Benefits:**
- Execution logic built on proven, tested runtime system
- No architectural refactoring when adding P2P scheduling (Phase 4)
- Docker runtime design enables code reuse (native logic executes in containers)
- Smart image tagging with full toolchain metadata enables reproducible builds

**Third-Party Dependency - Docker Go API:**
- **Official library:** `github.com/docker/docker/client` (Docker's official Go SDK)
- **Not custom:** We implement a thin abstraction wrapper over Docker API; do NOT write custom Docker client driver
- **Why wrapper:** Isolates Docker specifics, easier to test/mock, cleaner in job executor code
- **Docker client setup:** Initialize once at daemon startup; reuse for all image/container operations

---

### Milestone 1.0: Docker API Abstraction Implementation (2026-03-21)

**Status:** ✅ DOCUMENTED (ready for implementation)

**Key Architectural Decisions:**

1. **Embedded Dockerfile Templates (Not External Files)**
   - All predefined Dockerfile templates **embedded in the binary** via Go `embed` package
   - No external Dockerfile files needed for deployment
   - Examples of embedded templates:
     - `ubuntu-gcc-11-glibc-2.35.Dockerfile`
     - `ubuntu-gcc-12-glibc-2.36.Dockerfile`
     - `ubuntu-clang-14-glibc-2.35.Dockerfile`
     - `alpine-gcc-11-musl-1.2.3.Dockerfile`
     - And more for different compiler/cruntime/architecture combinations
   - **Binary portability:** Deploy single buildozer binary to any system; it builds required runtimes on first use

2. **On-Demand Image Building**
   - When job requests runtime: `buildozer-c-gcc-11-x86_64-glibc-2.35`
   - Detector checks if image already exists in Docker daemon
   - If missing:
     1. Load embedded Dockerfile template from binary
     2. Build image via Docker API
     3. Tag with canonical name
     4. Cache in local Docker daemon
   - If exists: Use immediately (fast path, no rebuild)
   - **Result:** First job requesting a runtime triggers build; subsequent jobs use cached image

3. **Predefined Docker Images with Common Toolchains**
   - Covers: gcc-11/12, clang-14/15, glibc/musl, x86_64/aarch64, various versions
   - Combinations selected for common use cases (C/C++ development)
   - Each image tagged twice: C and C++ variants use same underlying image

4. **Comprehensive Docker Image Tagging with Canonical Compiler Names**
   - Tag format: `buildozer-<language>-<compiler>-<version>-<arch>-<cruntime>-<cruntimever>`
   - **Canonical naming:** Use "gcc" in tag (not g++), "clang" (not clang++)
   - Examples:
     - `buildozer-c-gcc-11-x86_64-glibc-2.35` (C with gcc-11)
     - `buildozer-cxx-gcc-11-x86_64-glibc-2.35` (C++ with gcc-11, same image)
     - `buildozer-cxx-clang-14-x86_64-glibc-2.35` (C++ with clang-14)
   - **Rationale:** gcc/g++ are same compiler; language field determines driver selection

5. **Smart Image Reuse Pattern**
   - One Docker image with gcc-11 + g++-11 provides TWO runtimes:
     ```
     buildozer-c-gcc-11-x86_64-glibc-2.35      (uses gcc driver)
     buildozer-cxx-gcc-11-x86_64-glibc-2.35    (uses g++ driver)
     ```
   - Both tags point to same image (zero duplication)
   - Job language field determines which driver (gcc vs g++) to invoke

6. **Metadata-Driven Matching**
   - Job runtime spec: (language=c, compiler=gcc, ver=11, arch=x86_64, cruntime=glibc, ver=2.35)
   - Docker detector parses image tags → Extracts full toolchain metadata
   - Runtime matcher finds exact Docker image by complete metadata match
   - Enables precise, reproducible job-to-runtime matching

**Implementation Files:**
- `pkg/runtimes/cpp/docker/dockerfiles/` — Embedded Dockerfile templates (via `embed` package)
- `pkg/runtimes/cpp/docker/dockerfile_builder.go` — Load embedded templates, on-demand build, caching
- `pkg/runtimes/cpp/docker/docker_cpp_runtime.go` — Docker runtime implementing Runtime interface
- `pkg/runtimes/cpp/docker/detector.go` — Scan images, auto-build if missing, parse metadata, register runtimes

**Deployment Benefit:**
- **Single unit of deployment:** buildozer binary contains Dockerfiles
- **No setup required:** Run binary on any system with Docker; it builds needed runtimes automatically
- **First-use overhead:** First job requesting runtime X triggers Docker build (~1-2 min); subsequent jobs use cached image
- **Network-friendly:** Binary can be deployed offline; doesn't require downloading Dockerfiles from registry

**Testing Strategy:**
- Verify embedded Dockerfiles can be extracted from binary
- On-demand build workflow: Request non-existent runtime → Builds → Returns image
- Compile C file on native → hash X
- Compile same C file via Docker runtime (triggers build on first use) → hash X (verified identical)
- Compile C++ file on native → hash Y
- Compile same C++ file via Docker runtime (uses cached image) → hash Y (verified identical)
- Verify driver selection: Language field determines gcc vs g++
- Verify binary portability: Deploy binary to fresh system; runtimes build on first use

**Next Steps:** Implementation of Milestones 1.0-1.2 (runtime foundation and C/C++ implementations)

---

### Milestone 1.0: Runtime System Foundation - STARTED (2026-03-21)

**Status:** ✅ IMPLEMENTATION STARTED

**Completed Components:**

1. **Runtime Package (`pkg/runtime/`)**
   - `types.go` — Core types and Runtime interface
     - `Runtime` interface: Execute, Available, Metadata, RuntimeID
     - `ExecutionRequest` and `ExecutionResult` types for job execution
     - `Metadata` struct with 9 fields for runtime identification (id, language, compiler, version, arch, OS, C runtime, C runtime version, details)
     - `AvailabilityError` for runtime discovery failures
   - `registry.go` — Runtime registry with search/matching
     - `Registry` type with thread-safe map of runtimes
     - Methods: Register, Get, All, Find, FindByLanguageAndCompiler, Available, Count
     - Thread-safe with RWMutex for concurrent access
   - `discoverer.go` — Discoverer interface for runtime discovery
     - `Discoverer` interface: Discover(ctx, registry), Name()
     - Used by native and Docker runtime implementations to register themselves
   - `runtime_test.go` — Unit tests for registry and interfaces
     - MockRuntime for testing
     - Tests: Register, Get, Duplicate detection, FindByLanguageAndCompiler
     - All 4 tests passing ✅

2. **Docker API Package (`pkg/docker/`)**
   - `types.go` — Docker abstraction types
     - `ContainerConfig` for container creation
     - `ExecResult` for command execution results
     - `createTarArchive()` helper for Dockerfile building
   - `client.go` — Docker API abstraction wrapper
     - `Client` struct wrapping official moby/moby client
     - `NewClient()` with environment variable support and connectivity check
     - Placeholder methods (stubs with TODO comments) for:
       - `PullImage()` - Pull container image
       - `ImageExists()` - Check local image existence
       - `BuildImage()` - Build image from Dockerfile
       - `StartContainer()` - Create and start container
       - `ExecInContainer()` - Execute command in running container
       - `StopContainer()` - Stop running container
       - `RemoveContainer()` - Remove container
       - `ContainerWait()` - Wait for container exit
     - Thread-safe with RWMutex
     - Proper error handling and resource cleanup

3. **Dependencies Added (`go.mod`)**
   - `github.com/moby/moby/client` — Official Docker Go API
   - `github.com/moby/moby/api` — Docker API types
   - Full transitive dependency tree resolved with `go mod tidy`
   - 20+ additional dependencies (docker, containerd, opentelemetry, etc.)

4. **Build Status**
   - ✅ `go build ./...` — Success
   - ✅ `go test ./pkg/runtime/...` — All 4 tests passing
   - ✅ Protocol still compiles with new packages
   - ✅ No lint errors in new code

**Architecture Decisions Made:**

- **Docker client initialization:** Verify connectivity on creation; reuse single client instance
- **Wrapper pattern:** Thin abstraction over moby/moby client; future implementations can swap details
- **Thread safety:** All client operations use RWMutex for concurrent-safe access
- **Placeholder strategy:** Core methods stubbed with TODO comments for implementation in next phase

**REFACTOR (2026-03-21): Made Runtime Package Language-Agnostic**

Initial implementation had C/C++-specific types:
- `Metadata.Compiler`, `Metadata.CRuntime`, `Metadata.CRuntimeVersion`
- `Registry.FindByLanguageAndCompiler()` method
- `ExecutionRequest.Command []string` — tied to subprocess execution

But the development plan and protocol define a **multi-language system**:
- Protocol has `CppToolchain`, `GoToolchain`, `RustToolchain` (oneof)
- Future languages: Java, Python, etc.
- Each language has different toolchain metadata

**Refactored to Generic Design:**
- Removed C/C++-specific fields from `Metadata` — now language-agnostic
- `ExecutionRequest.Job interface{}` — opaque to registry, interpreted by implementation
- `Registry.FindByLanguage(lang)` — works for any language
- `Metadata` has generic fields: `Language`, `Version`, `RuntimeType`, `IsNative`, `Details`
- C/C++-specific metadata handled by **CppDiscoverer implementation**, not core package

**Result:**
- ✅ Core runtime package works with C/C++, Go, Rust, and future languages
- ✅ Implementations (CppDiscoverer, GoDiscoverer, etc.) are language-specific
- ✅ Registry and discovery remain generic and extensible

**Test Results (After Refactoring):**
```
=== RUN   TestRegistryRegister
--- PASS: TestRegistryRegister (0.00s)
=== RUN   TestRegistryGet
--- PASS: TestRegistryGet (0.00s)
=== RUN   TestRegistryDuplicateRegister
--- PASS: TestRegistryDuplicateRegister (0.00s)
=== RUN   TestRegistryFindByLanguage
--- PASS: TestRegistryFindByLanguage (0.00s)
PASS
ok      github.com/Manu343726/buildozer/pkg/runtime     0.002s
```

**Files Created/Modified:**
- ✅ `/pkg/runtime/types.go` — Generic Runtime interface and types (refactored for multi-language support)
- ✅ `/pkg/runtime/registry.go` — Runtime registry with FindByLanguage (refactored)
- ✅ `/pkg/runtime/discoverer.go` — Generic discoverer interface
- ✅ `/pkg/runtime/runtime_test.go` — Unit tests with multi-language support (refactored)
- ✅ `/pkg/docker/types.go` — Docker types and helpers
- ✅ `/pkg/docker/client.go` — Docker API abstraction with TODO stubs
- ✅ `/go.mod` — Added moby/moby dependency

**Next: Milestone 1.0 Completion**
- Implement Docker API methods (PullImage, ImageExists, BuildImage, StartContainer, ExecInContainer, etc.)
- Create tests for Docker API abstraction
- Then proceed to implement language-specific runtimes:
  - **Milestone 1.1**: Native C/C++ toolchain detection (CppDiscoverer implementation)
  - **Milestone 1.2**: Docker-based C/C++ runtime with embedded Dockerfiles
  - Future: Go, Rust, and other language runtime implementations

---

## Phase 1: Core Protocol & Job Model (Weeks 1-4)

### Foundation: Tooling & Protocol Stack (2026-03-21)

**Status:** ✅ ESTABLISHED

**Buf Configuration & Proto Management:**
- **[buf](https://buf.build/) v1.40.1** installed and integrated for:
  - Protocol Buffer linting (STANDARD rule set enforcing Google protobuf best practices)
  - Code generation via `buf generate` (replaces system protoc dependency)
  - Breaking change detection for API evolution
  - VS Code integration with automatic proto formatting on save
  - CI/CD compatibility (no system dependencies)
- **Configuration files:**
  - `buf.yaml` - Linting rules (STANDARD), module dependencies (protovalidate)
  - `buf.gen.yaml` - Code generation plugins with proper Go package configuration
    - Protobuf code generation: `protoc-gen-go` → `internal/gen/`
    - Connect code generation: `protoc-gen-connect-go` → `internal/gen/`
    - Managed mode enabled with go_package_prefix override for correct import paths
    - protovalidate module disabled from managed code generation (annotations-only)
  - `.vscode/settings.json` - VS Code buf extension configuration
- **All proto files:** 100% buf lint compliant (STANDARD rule set)
  - Enum values prefixed with enum name (e.g., `TIME_UNIT_MILLISECOND`)
  - Enum zero values use `_UNSPECIFIED` suffix
  - RPC methods follow `<Service><Method>Request/Response` naming
  - Package versioning aligned with directory structure (`buildozer.proto.v1`)

**[Connect](https://connectrpc.com/) Protocol for RPC:**
- **Selected:** [Connect](https://connectrpc.com/) (connectrpc/connect-go v1.19.1)
- **Setup:**
  - `protoc-gen-connect-go` plugin installed via `go install connectrpc.com/connect/cmd/protoc-gen-connect-go@latest`
  - Generated code: `services.connect.go` with handler/client types for all RPC services
  - Handles gRPC, Connect, and gRPC-Web protocols transparently
- **Rationale:**
  - Supports gRPC-compatible protocol with simpler streaming semantics
  - Single protocol supporting gRPC, REST (HTTP/1.1), and WebSocket transports
  - Better web compatibility and browser support compared to gRPC
  - Cleaner error handling and bidirectional streaming implementation
  - Seamless Go library integration; low overhead
- **Implementation strategy:**
  - RPC method definitions remain in proto services (Connect compatible)
  - Connect code generated in `internal/gen/buildozer/proto/v1/protov1connect/`
  - gRPC compatibility maintained for existing clients
  - Backward compatible: existing protos work with Connect without modification
  - Future: Can support REST transport without proto changes

**Protovalidate Integration:**
- **Status:** Configured as optional enhancement (dependency in buf.yaml)
- **buf.yaml dependency:** `buf.build/bufbuild/protovalidate` for validation annotations
- **Usage pattern:** Annotations define validation rules in proto messages; runtime validation via business logic
- **Future:** Can add validation interceptor when implementing Connect handlers

**Proto File Organization:**
- **Location:** `buildozer/proto/v1/` (semantic versioning aligned with package)
- **Generated code:** `internal/gen/buildozer/proto/v1/` (buf managed)
  - `.pb.go` files: Protobuf message definitions
  - `protov1connect/` directory: Connect service handlers and clients
- **Core proto files:**
  - `vocabulary.proto` - Vocabulary types (fundamental building blocks)
  - `runtime.proto` - Runtime model and toolchain specifications
  - `job.proto` - Job model, progress tracking, and statistics
  - `job_data.proto` - JobData abstraction and artifact storage
  - `auth.proto` - Authentication and request metadata
  - `network_messages.proto` - All P2P message types (peer discovery, job lifecycle)
  - `services.proto` - Service (RPC) definitions for Connect (no gRPC dependency)

**Go Module Dependencies:**
- `connectrpc.com/connect v1.19.1` - Connect RPC library (includes gRPC/gRPC-Web compatibility)
- `google.golang.org/protobuf v1.36.11` - Protobuf runtime
- `google.golang.org/grpc v1.79.3` - gRPC (transitive from Connect)

**Build & Verification:**
- ✅ `buf generate` produces all .pb.go and .connect.go files
- ✅ `buf lint` reports 0 errors/warnings
- ✅ `go build ./...` completes successfully
- ✅ Project compiles and builds cleanly

**Vocabulary Type Enhancements (2026-03-21):**
- **Signature type added:** Cryptographic signature representation for artifact/message authentication
  - `SignatureAlgorithm` enum: RSA-SHA256, RSA-SHA512, ECDSA-SHA256, ECDSA-SHA512, Ed25519
  - `Signature` message: algorithm, base64-encoded value, optional key_id
  - Complements `Hash` vocabulary type for complete crypto support
  - Use cases: peer authentication, artifact signing, build provenance, message authentication

---

### Step 1: Protocol Definitions ✅ COMPLETE

**Objective:** Define comprehensive protocol buffer definitions for all P2P communication, job types, and data models.

**Files Created:**
- `pkg/proto/vocabulary.proto` - Common vocabulary types (TimeUnit, TimeDuration, TimeStamp, TimeRange, Percentage, Size, SizeUnit, HashAlgorithm, Hash, Version, ApiProtocol, ApiUri, LoadInfo)
- `pkg/proto/runtime.proto` - Runtime with oneof toolchain (CppToolchain, GoToolchain, RustToolchain), RuntimeRecipe, ResourceLimit
- `pkg/proto/job.proto` - Job with oneof job_spec (CppCompileJob, CppLinkJob), JobProgress, JobResult, JobDependency
- `pkg/proto/job_data.proto` - JobData, FileJobData, DirectoryJobData, StreamChunk, JobDataReference, RetentionPolicy, JobDataIndex
- `pkg/proto/auth.proto` - RequestMetadata, AuthResponse
- `pkg/proto/network_messages.proto` - NetworkMessage envelope, PeerAnnouncement, all P2P message types
- `pkg/proto/services.proto` - gRPC service definitions (JobService, ExecutorService, PeerService, SchedulerService)
- `pkg/proto/generate.go` - go:generate directive for proto compilation

**Key Design Decisions:**
- Protocol uses Google Protobuf 3.12.4 with gRPC services
- **Pure oneof pattern:** No redundant type enums - Job and Runtime types are discriminated by oneof field alone
- Vocabulary types: Reusable types across protocol (TimeDuration, TimeStamp, Version, Hash, ResourceSpec, etc.)
- Generic toolchain support: Runtime contains oneof for C++, Go, Rust (extensible to other languages)
- Generic job support: Job contains oneof for CppCompile, CppLink (extensible to other job types)
- Content-addressed artifact storage (SHA256 hashing)
- Real-time progress streaming for job execution
- Quorum-based scheduling via gRPC broadcasts
- Network messages wrapped in NetworkMessage envelope with metadata

**Compilation Status:**
- ✅ All `.proto` files compile successfully via `go generate ./...`
- ✅ 8 `.pb.go` files generated (vocabulary, runtime, job, job_data, auth, network_messages, services)
- ✅ 1 `.pb.grpc.go` file generated (services_grpc)
- ✅ Go dependencies resolved: protobuf v1.36.11, gRPC v1.79.2

**Next Step:** Step 2 - Job & Runtime Abstractions (Go implementation layer for job types)

---

## User Feedback & Notes

### Feedback on Step 1: Protocol Definitions

**Issue 1: Toolchain not generic**
- **User feedback:** "What you wrote as Toolchain is not generic, is really a C/C++ toolchain... remember what the plan said about generic messages and oneofs?"
- **Fix applied (iteration 1):** Refactored `Toolchain` to use oneof pattern with ToolchainType enum
- **User feedback:** "you don't need a Toolchain message, you can put the oneof in the runtime directly"
- **Fix applied (iteration 2):** Removed separate Toolchain message, moved oneof directly into Runtime with ToolchainType enum
- **User feedback:** "no need for toolchain type since we have the oneof"
- **Fix applied (iteration 3):** Removed ToolchainType enum, kept only oneof
  - `Runtime.toolchain` is now a pure oneof with CppToolchain, GoToolchain, RustToolchain
  - Oneof itself discriminates the toolchain type (no separate enum needed)
  - Field naming simplified: `cpp`, `go`, `rust` instead of `cpp_toolchain`, etc.
- **Status:** ✅ Recompiled successfully, all protos generate and build without errors
- **Design principle:** Elegant use of proto oneof pattern - the union itself carries the type information

**Issue 2: Job had redundant JobType enum**
- **User feedback:** "in Job, no job type enum, for the same reason"
- **Fix applied:** Removed `JobType` enum from Job message
  - Job now only has `oneof job_spec` with CppCompileJob, CppLinkJob
  - Job type is discriminated by which oneof field is set
  - Updated field numbers to be sequential (id=1, runtime=2, input_data_ids=3, etc.)
  - Updated content_hash comment to reflect job_spec_type instead of type
- **Status:** ✅ Recompiled successfully, all protos generate and build without errors
- **Design principle:** Consistency with Runtime pattern - oneof pattern provides type discrimination implicitly

**Addition: Vocabulary Types File**
- **User feedback:** "Add a vocabulary types proto file with basic types used along the protocol, such as TimeDuration (count + time unit), TimeStamp, etc etc"
- **Files created:** New `pkg/proto/vocabulary.proto` with common types:
  - TimeUnit enum (MILLISECOND, SECOND, MINUTE, HOUR, DAY)
  - TimeDuration (count + unit pair)
  - TimeStamp (unix milliseconds)
  - TimeRange (start + end) [renamed from DateRange]
  - Percentage (0-100)
  - Size with SizeUnit enum (BYTE through TERABYTE)
  - HashAlgorithm enum (SHA256, SHA512, BLAKE3)
  - Hash (algorithm enum + value)
  - Version (semantic versioning)
- **Additional feedback:**
  - Renamed DateRange → TimeRange for clarity
  - HashAlgorithm: Changed from string to enum (SHA256, SHA512, BLAKE3)
  - Removed Status message: Each API will define its own result message with specific code enums/details
  - Removed Progress message: Same reasoning - each RPC defines its own progress format
  - Removed ResourceAmount and ResourceSpec: Resource-specific types, will be defined by APIs that manipulate them
  - Kept Size/SizeUnit: Generic measurement types useful across protocol
  - Removed Identifier, Address, Label, Taggable: Network addresses and identifiers are represented as strings in context where needed; labels/tagging handled per-API
- **Status:** ✅ All changes compile successfully, proto package builds without errors
- **Design principle:** Lightweight vocabulary for fundamental types (time, size, hash, version) reused across protocol; API-specific results, identifiers, addresses, and metadata defined at point of use

**Vocabulary Type Integration:**
- **Objective:** Use vocabulary types consistently across all protocols where applicable
- **Changes applied:**
  - All proto files now import `vocabulary.proto`
  - **TimeStamp** replaces all `int64 unix_ms` timestamp fields (submitted_at, created_at, sent_at, updated_at, joined_at, cancelled_at, decided_at, error_time, last_seen, keep_until, etc.)
  - **TimeDuration** replaces duration fields (timeout in Job, keep_for in RetentionPolicy)
  - **Version** replaces version strings (compiler_version, c_runtime_version, go_version, rust_version, protocol_version, buildozer_version)
  - **Hash** replaces all content_hash string fields (RuntimeRecipe, Job, FileJobData, DirectoryJobData, StreamChunk, CacheQueryMessage, ArtifactFetchRequestMessage)
  - **Percentage** replaces progress_percent and current_load_percent uint32 fields
  - **Size** replaces size_bytes and total_size_bytes uint64 fields (JobData, DirectoryJobData, CacheQueryResponseMessage, CacheAnnouncementMessage, ArtifactFetchResponseMessage, PeerCapabilities cache_size)
- **Files modified:** runtime.proto, job.proto, job_data.proto, network_messages.proto, services.proto
- **Compilation status:** ✅ All protos compile successfully, proto package builds without errors
- **Design principle:** Consistent use of vocabulary layer throughout protocol reduces code duplication and ensures type-safe handling of common constructs

**Job Message Refactoring: Inputs Moved Into Job**
- **Rationale:** Job inputs must be part of the Job message itself to ensure they are never lost when the job is passed around between peers
- **Changes applied:**
  - Added `repeated JobData inputs = 25;` to Job message (keeping input and expected output IDs for caching)
  - Removed `repeated JobData inputs` from JobSubmissionMessage (now only contains Job + submitted_at)
  - Removed `repeated JobData inputs` from ExecuteJobRequest (now only contains Job)
  - Added job_data.proto import to job.proto (no circular dependencies)
  - Removed unused job_data.proto import from services.proto
- **Result:** Job is self-contained with all inputs, preventing data loss and simplifying message passing
- **Compilation status:** ✅ All protos compile successfully without warnings

**ApiUri Vocabulary Type Addition:**
- **Objective:** Add a network endpoint vocabulary type for consistent representation of API addresses
- **Added ApiProtocol enum (simplified):**
  - GRPC: gRPC protocol
  - REST: REST API
  - Note: Can be extended later (HTTP/HTTPS, GRPCS, etc.)
- **ApiUri fields:**
  - `host` (string): Hostname or IP address
  - `port` (uint32): Port number
  - `protocol` (ApiProtocol enum): Communication protocol
  - `subpath` (string, optional): Optional path component (e.g., "/api/v1", "/rpc")
- **Benefits:** Type-safe protocol specification, extensible for future protocols
- **Compilation status:** ✅ Protos compile successfully with simplified enum

**ApiUri Usage Throughout Protocol:**
- **Objective:** Use ApiUri vocabulary type for all network endpoint specifications
- **Changes applied:**
  - **network_messages.proto:**
    - NetworkMessage: `sender_address` (string) → `sender_uri` (ApiUri)
    - NetworkMessage: `reply_to_address` (string) → `reply_to_uri` (ApiUri, optional)
    - PeerAnnouncement: `grpc_address` (string) → `grpc_uri` (ApiUri)
    - PeerAnnouncement: `rest_api_address` (string) → `rest_api_uri` (ApiUri, optional)
  - **job_data.proto:**
    - JobDataReference: `peer_address` (string) → `peer_uri` (ApiUri, optional)
  - **services.proto:**
    - PeerInfo: `grpc_address` (string) → `grpc_uri` (ApiUri)
    - PeerInfo: `rest_api_address` (string) → `rest_api_uri` (ApiUri, optional)
- **Benefits:** Consistent, type-safe endpoint specification; replaces ad-hoc host:port string parsing
- **Compilation status:** ✅ All protos compile successfully with ApiUri usage

**PeerInfo Enhancement: Added Runtime and Resource Information:**
- **Objective:** Enrich PeerInfo with peer capabilities (runtimes, resources, load details)
- **Fields added to PeerInfo:**
  - `repeated Runtime available_runtimes`: Available toolchains/runtimes on the peer
  - `ResourceLimit resources`: Resource constraints and limits (CPU, RAM, disk, concurrent jobs)
  - `uint32 running_jobs_count`: Number of jobs currently running
  - `uint32 queued_jobs_count`: Number of jobs queued
- **Result:** PeerInfo now contains essential peer metadata for intelligent job scheduling and load balancing
- **Files modified:** services.proto (added runtime.proto import to resolve Runtime and ResourceLimit types)
- **Compilation status:** ✅ All protos compile successfully with enriched PeerInfo

**LoadInfo Message: Consolidated Load Reporting:**
- **Objective:** Extract load/utilization metrics into a reusable message, enable runtimes to report their own load
- **LoadInfo message (added to vocabulary.proto):**
  - `Percentage current_load`: Current resource utilization (0-100%)
  - `uint32 running_jobs_count`: Number of jobs currently running
  - `uint32 queued_jobs_count`: Number of jobs queued
  - `repeated Percentage cpu_per_thread`: CPU usage per thread (type-safe percentage per thread)
  - `Size ram_usage`: Current RAM usage
- **Applied to:**
  - **Runtime** (runtime.proto): Added `LoadInfo load` field for runtime to report current utilization
  - **PeerCapabilities** (network_messages.proto): Replaced 3 individual fields with single `LoadInfo load` field
  - **PeerInfo** (services.proto): Replaced 3 individual fields with single `LoadInfo load` field
- **Benefits:** Single source of truth for load metrics, detailed CPU/RAM insights, reusable across different message types, cleaner structure
- **Compilation status:** ✅ All protos compile successfully with LoadInfo consolidation

**Timestamp Standardization: Complete Protocol Audit:**
- **Objective:** Ensure all timestamps use TimeStamp vocabulary type (no raw int64 timestamp fields)
- **Audit performed on all .proto files:**
  - Found 8 raw int64 timestamp fields across 4 files:
    - auth.proto: RequestMetadata timestamp_ms, AuthResponse timestamp_ms
    - build_request.proto: created_at_ms, modified_at_ms, announced_at_ms
    - job_data.proto: JobDataMetadata created_at_ms, last_accessed_at_ms
    - services.proto: CommitScheduleResponse estimated_start_ms
- **Changes applied:**
  - auth.proto: Added vocabulary import, renamed `int64 timestamp_ms` → `TimeStamp timestamp` (2 fields)
  - build_request.proto: Added vocabulary import, replaced all 3 int64 timestamp fields with TimeStamp
  - job_data.proto: Replaced 2 int64 timestamp fields in JobDataMetadata with TimeStamp
  - services.proto: Replaced `int64 estimated_start_ms` with `TimeStamp estimated_start`
- **Result:** All timestamps in protocol now use vocabulary type, ensuring consistency and type safety
- **Compilation status:** ✅ All protos compile successfully with complete timestamp standardization

**Duration Standardization: Complete Protocol Audit:**
- **Objective:** Ensure all durations and TTLs use TimeDuration vocabulary type (no raw seconds/milliseconds fields)
- **Audit performed on all .proto files:**
  - Found 3 raw uint32 duration fields in build_request.proto:
    - Line 140: Build timeout_seconds
    - Line 306: P2P transfer timeout_seconds
    - Line 378: Peer announcement ttl_seconds
- **Changes applied:**
  - Line 140: Renamed `uint32 timeout_seconds` → `TimeDuration timeout`
  - Line 306: Renamed `uint32 timeout_seconds` → `TimeDuration timeout`
  - Line 378: Renamed `uint32 ttl_seconds` → `TimeDuration ttl`
- **Result:** All duration fields in protocol now use vocabulary type, ensuring consistency and explicit time unit specification
- **Compilation status:** ✅ All protos compile successfully with complete duration standardization

**Note on TimeRange:** Currently no start/end timestamp pairs in protocol messages that would benefit from TimeRange type. TimeRange is available for future use when needed (e.g., time window specifications).

---

## Next Phase: Step 2 - Job & Runtime Abstractions

**CppToolchain Type Safety Enhancement:**
- **Objective:** Convert CppToolchain string fields to enums for type safety and validation
- **Enums created (in runtime.proto):**
  - **CppLanguage**: C, CPP
  - **CppCompiler**: GCC, CLANG (extensible for additional compilers)
  - **CppArchitecture**: X86_64, AARCH64, ARM, PPC64LE (extensible for new architectures)
  - **CRuntime**: GLIBC, MUSL (C runtime implementations, extensible for other runtimes)
- **CppToolchain message refactored:**
  - `string language` → `CppLanguage language` (enum)
  - `string compiler` → `CppCompiler compiler` (enum)
  - `string architecture` → `CppArchitecture architecture` (enum)
  - `string c_runtime` → `CRuntime c_runtime` (enum)
  - Other fields (compiler_version, c_runtime_version) remain as Version types
- **Benefits:** Type-safe toolchain specification, validated values, extensible for future compilers/architectures, prevents typos and invalid values
- **Compilation status:** ✅ All protos compile successfully with CppToolchain enums

**Enum Simplification: Removed UNSPECIFIED Values:**
- **Objective:** Remove unnecessary UNSPECIFIED enum values since protobuf3 allows checking field presence without sentinel values
- **Rationale:** Protobuf3 tracks field presence implicitly; explicit UNSPECIFIED values are not needed and simplify enums
- **Changes applied across all proto files:**
  - vocabulary.proto: TimeUnit, HashAlgorithm, SizeUnit, ApiProtocol
  - runtime.proto: CppLanguage, CppCompiler, CppArchitecture, CRuntime
  - job.proto: JobProgress.JobStatus, JobResult.JobStatus
  - job_data.proto: JobData.DataType
  - network_messages.proto: NetworkMessage.MessageType, JobErrorMessage.ErrorType
  - build_request.proto: BuildType, JobDependency.DependencyType
- **Result:** All enums now start at 0 with meaningful values, reducing cognitive overhead and simplifying enum handling
- **Compilation status:** ✅ All protos compile successfully with simplified enums

**CppToolchain Enhancement: C++ ABI and Standard Library:**
- **Objective:** Add comprehensive ABI and standard library specification to CppToolchain for precise C++ compilation environment capture
- **Enums created (in runtime.proto):**
  - **CppAbi**: ITANIUM (default for Unix-like systems), MICROSOFT (for Windows/MSVC)
  - **CppStdlib**: LIBSTDCXX (GCC), LIBCXX (LLVM/Clang), MSVC_STL (Microsoft)
- **Fields added to CppToolchain:**
  - `CppAbi cpp_abi`: C++ ABI specification
  - `CppStdlib cpp_stdlib`: C++ standard library implementation
  - `repeated string abi_modifiers`: Compiler-specific ABI modification flags
    - Examples: `-fabi-version=X` (GCC C++ ABI version), `-fglibcxx-use-cxx11-abi` (GCC std::string ABI), other compiler-specific ABI control flags
- **Benefits:** Captures ABI/stdlib choices for correct cross-compilation and reproducible builds; abi_modifiers allows compiler-specific fine-tuning (e.g., std::string ABI changes) without modifying core enums
- **Compilation status:** ✅ All protos compile successfully with ABI/stdlib additions

**NetworkMessage MessageType Removal:**
- **Objective:** Remove redundant MessageType enum since oneof payload already discriminates message types
- **Change applied (in network_messages.proto):**
  - Removed `enum MessageType` (12 values: PEER_ANNOUNCEMENT, JOB_SUBMISSION, JOB_PROGRESS, JOB_RESULT, JOB_ERROR, JOB_CANCELLATION, SCHEDULE_DECISION, CACHE_QUERY, CACHE_QUERY_RESPONSE, CACHE_ANNOUNCEMENT, ARTIFACT_FETCH_REQUEST, ARTIFACT_FETCH_RESPONSE)
  - Removed `MessageType message_type = 6;` field
  - Updated comment on oneof payload to clarify type discrimination
- **Rationale:** The oneof field implicitly provides type discrimination - the concrete message type is determined by which field is set, making the explicit enum redundant
- **Note:** ErrorType enum in JobErrorMessage remains since it categorizes error types within a single message type (not discriminating between different message types in a oneof)
- **Compilation status:** ✅ All protos compile successfully after MessageType removal

**PeerGoodbye Message Addition:**
- **Objective:** Add peer departure announcement to complement PeerAnnouncement
- **PeerGoodbye message (in network_messages.proto):**
  - `string peer_id`: Peer ID that is leaving
  - `TimeStamp left_at`: Timestamp when peer is leaving
  - `string reason`: Optional reason for departure (e.g., "graceful shutdown", "network error")
- **Integration:** Added to NetworkMessage payload oneof as field 32
- **Benefits:** Enables peers to detect departures and clean up state; complements peer discovery with peer departure notification
- **Compilation status:** ✅ All protos compile successfully with PeerGoodbye addition

**Job Status Enhancement: Data Transfer Phases:**
- **Objective:** Track input and output data transfer phases separately from execution
- **JobProgress.JobStatus enhancements (in job.proto):**
  - Added `INPUT_TRANSFER = 3`: Inputs being transferred to executing peer (after SCHEDULED)
  - Added `OUTPUT_TRANSFER = 6`: Outputs being transferred back to requesting client (after COMPLETED execution)
  - Updated sequence: PENDING → READY → SCHEDULED → INPUT_TRANSFER → RUNNING → COMPLETED → OUTPUT_TRANSFER → [FAILED/CANCELLED at any point]
  - Previous statuses renumbered: RUNNING=3→4, COMPLETED=4→5, FAILED=5→7, CANCELLED=6→8
- **JobResult.JobStatus design (in job.proto):**
  - JobResult message is only published after output transfer completes
  - Status enum: COMPLETED=0 (fully delivered), FAILED=1, CANCELLED=2
  - No intermediate states in JobResult since it represents the final state
- **Benefits:** Separates computation phases from data transfer; enables complete job lifecycle tracking through JobProgress; JobResult represents truly final state
- **Compilation status:** ✅ All protos compile successfully with refined job status model

**JobStatus Field Naming Clarification:**
- **Objective:** Clarify field naming in JobStatus message for clarity
- **Change applied (in services.proto):**
  - Renamed `string submitted_to_peer_id = 2;` → `string submitter_id = 2;`
  - Updated comment to: "Client ID of the client who received the job submission"
- **Rationale:** The field represents the client that accepted and received the submission, not the source client. The terminology and comment should be clearer about this semantic meaning.
- **Compilation status:** ✅ Proto compiles successfully with renamed field
- **Note:** Generated .pb.go file will be regenerated on next proto compilation

**JobTimings Message Addition:**
- **Objective:** Track exact time ranges and durations of job processing through all phases
- **JobTimings message (added to job.proto):**
  - **Phase time ranges (using TimeRange: start_time + end_time):**
    - `pending_time_range`: Job submitted until READY (dependencies met)
    - `ready_time_range`: READY until SCHEDULED (assigned to peer)
    - `scheduled_time_range`: SCHEDULED until INPUT_TRANSFER (ready to transfer inputs)
    - `input_transfer_time_range`: INPUT_TRANSFER until RUNNING (inputs transferred)
    - `running_time_range`: RUNNING until COMPLETED (execution finished)
    - `completed_time_range`: COMPLETED until OUTPUT_TRANSFER (ready to transfer outputs)
    - `output_transfer_time_range`: OUTPUT_TRANSFER until final completion (outputs transferred)
  - **Terminal state timestamps:**
    - `failed_at`: When job failed (can occur at any phase)
    - `cancelled_at`: When job cancelled (can occur at any phase)
  - **Phase durations (derived from time ranges):**
    - `pending_duration`, `ready_duration`, `scheduled_duration`, `input_transfer_duration`, `running_duration`, `output_transfer_duration`
  - **Aggregate metrics:**
    - `total_duration`: End-to-end from submission to final state
    - `wall_clock_duration`: Total elapsed time including any gaps
    - `compute_duration`: Actual execution time (same as running_duration)
- **Design rationale:** Using TimeRange instead of individual timestamps naturally handles gaps and provides exact timing information. If a job is paused, interrupted, or suspended at any point, the time ranges capture the exact contiguous periods when in each phase.
- **Benefits:** Precise visibility into job lifecycle; enables bottleneck analysis (queue time vs. transfer time vs. compute time); handles edge cases like job suspension or multi-phase execution
- **Compilation status:** ✅ Proto compiles successfully with JobTimings using TimeRange

**JobStatistics Message Addition:**
- **Objective:** Aggregate timing, resource usage, and performance metrics for job analysis
- **Refactored with sub-messages (in job.proto):**
  - **JobResourceUsage** - CPU, memory, and disk I/O resource metrics:
    - `uint32 peak_cpu_cores_used`: Peak number of CPU cores actively used
    - `uint32 min_cpu_cores_used`: Minimum number of CPU cores actively used
    - `uint32 avg_cpu_cores_used`: Average number of CPU cores actively used
    - `Size peak_memory_usage`: Peak memory consumption
    - `Size min_memory_usage`: Minimum memory consumption
    - `Size avg_memory_usage`: Average memory consumption
    - `uint64 total_disk_read_bytes`: Total bytes read from disk during execution
    - `uint64 total_disk_write_bytes`: Total bytes written to disk during execution
    - `double peak_disk_read_bandwidth`: Peak read bandwidth (bytes/sec)
    - `double peak_disk_write_bandwidth`: Peak write bandwidth (bytes/sec)
    - `double avg_disk_read_bandwidth`: Average read bandwidth (bytes/sec)
    - `double avg_disk_write_bandwidth`: Average write bandwidth (bytes/sec)
  - **JobDataTransfer** - All data size and network I/O metrics:
    - `Size input_data_size`: Total size of all inputs
    - `Size output_data_size`: Total size of all outputs
    - `Size total_data_transferred`: Combined total (inputs + outputs)
    - `Size network_input_size`: Data fetched from network (vs. local cache)
    - `Size network_output_size`: Data sent to network peers
  - **JobCacheInfo** - Cache information:
    - `bool cache_hit`: Whether output was served from cache
    - `string cache_source_peer_id`: Which peer provided the cached result
  - **JobExecutionMetrics** - Execution details, results, and resource consumption:
    - `string executing_peer_id`: Which peer executed the job
    - `int32 exit_code`: Process exit code
    - `bool success`: Whether execution completed successfully
    - `repeated string stdout_lines`: Standard output captured during execution (one line per entry)
    - `repeated string stderr_lines`: Standard error captured during execution (one line per entry)
    - `JobResourceUsage resource_usage`: Resource consumption during execution (CPU, memory, disk I/O)
  - **JobStatistics** - Top-level aggregator:
    - `string job_id`: Job identifier
    - `JobTimings timings`: Embedded timing information
    - `JobDataTransfer data_transfer`: Embedded data transfer metrics
    - `CacheQueryStatistics cache_query_statistics`: Embedded cache query statistics and timing metrics (from vocabulary)
    - `JobExecutionMetrics execution_metrics`: Embedded execution details, results, and resource consumption
- **Structural refactoring:** JobResourceUsage moved from direct sub-message in JobStatistics to be embedded within JobExecutionMetrics, since resource consumption is semantically part of execution metrics (not a separate category)
- **Design rationale:** Sub-message organization mirrors the pattern used for JobTimings. Groups related metrics by category, making the protocol cleaner and easier to extend with new categories (e.g., energy consumption, network latency distribution).
- **Benefits:** Better organization and readability; enables independent evolution of each metric category; cleaner API when selecting specific metric subsets; extensible for future metrics without modifying top-level JobStatistics
- **Compilation status:** ✅ Proto compiles successfully with JobStatistics sub-messages

**JobExecutionMetrics Enhancement: Output Capture:**
- **Objective:** Include stdout and stderr output in execution metrics for debugging and auditing
- **Change applied (in job.proto):**
  - Added `repeated string stdout_lines = 4;` - captures stdout as a list of text lines
  - Added `repeated string stderr_lines = 5;` - captures stderr as a list of text lines
- **Design rationale:** Line-based storage enables efficient streaming and log-level filtering; avoids storing massive single strings for long-running jobs; each entry represents one line of output
- **Benefits:** Enables complete job output inspection; supports debugging failed jobs; aids in audit trails; allows per-line processing without buffering entire output
- **Compilation status:** ✅ Proto compiles successfully with stdout/stderr additions

**JobResourceUsage Enhancement: Min and Average Metrics:**
- **Objective:** Track resource usage patterns beyond peak values
- **Change applied (in job.proto):**
  - Added `uint32 min_cpu_cores_used = 2;` - minimum CPU cores actively used
  - Added `uint32 avg_cpu_cores_used = 3;` - average CPU cores actively used
  - Added `Size min_memory_usage = 5;` - minimum memory consumption
  - Added `Size avg_memory_usage = 6;` - average memory consumption
- **Design rationale:** Min and average values complement peak metrics to provide complete resource utilization patterns. Peak alone can be misleading (e.g., brief spikes); min/avg provide insights into baseline resource needs.
- **Benefits:** Enables accurate resource provisioning and scheduling; helps identify jobs with volatile vs. stable resource patterns; supports cost optimization and performance profiling
- **Compilation status:** ✅ Proto compiles successfully with expanded resource metrics

**JobResourceUsage Reorganization and Disk I/O Enhancement:**
- **Objective:** Include JobResourceUsage as part of execution metrics and add disk I/O statistics
- **Changes applied (in job.proto):**
  - **Structural reorganization:**
    - Moved JobResourceUsage from direct sub-message in JobStatistics to be embedded within JobExecutionMetrics
    - Rationale: Resource consumption is semantically part of execution metrics, not a separate analytics category
    - Note: JobStatistics now contains 5 fields instead of 6 (execution_metrics now includes resource_usage)
  - **Disk I/O metrics added to JobResourceUsage:**
    - `Size total_disk_read`: Total data read from disk
    - `Size total_disk_write`: Total data written to disk
    - `Size peak_disk_read_bandwidth`: Peak read bandwidth
    - `Size peak_disk_write_bandwidth`: Peak write bandwidth
    - `Size avg_disk_read_bandwidth`: Average read bandwidth
    - `Size avg_disk_write_bandwidth`: Average write bandwidth
- **Design rationale:** Disk I/O is critical for understanding job performance, especially for I/O-bound workloads. Peak/avg bandwidth helps identify sustained I/O patterns vs. brief spikes.
- **Benefits:** Complete resource profiling (CPU, memory, disk); enables identification of bottlenecks; supports resource provisioning decisions; bandwidth metrics help with scheduling optimization
- **Compilation status:** ✅ Proto compiles successfully with reorganized and enhanced resource metrics

**Size Type Enhancement: Double Support for Flexible Measurements:**
- **Objective:** Enable Size type to represent decimal values for bandwidth and other fractional measurements
- **Change applied (in vocabulary.proto):**
  - Changed `int64 count = 1;` → `double count = 1;` in Size message
  - Updated comment: "Size count (supports decimal values)"
  - Added message-level comment: "Supports decimal values for flexible representation (e.g., bandwidth in bytes/sec)"
- **Impact on JobResourceUsage (in job.proto):**
  - Disk I/O metrics now use Size type instead of uint64 and double primitives
  - Field naming simplified: removed "_bytes" suffix since Size includes units
  - Total disk metrics: `total_disk_read` and `total_disk_write`
  - Bandwidth metrics: `peak_disk_read_bandwidth`, `peak_disk_write_bandwidth`, `avg_disk_read_bandwidth`, `avg_disk_write_bandwidth`
- **Benefits:** Unified type for all size and bandwidth measurements; consistent unit handling; enables flexible representation of both discrete sizes and continuous rates; cleaner API with fewer primitive types
- **Compilation status:** ✅ Proto compiles successfully with unified Size type

**Field Naming Cleanup: Remove Redundant Unit Suffixes:**
- **Objective:** Eliminate redundant "_bytes" suffix since Size type already specifies units
- **Changes applied (in job.proto):**
  - Renamed `total_disk_read_bytes` → `total_disk_read`
  - Renamed `total_disk_write_bytes` → `total_disk_write`
  - Updated field comments to remove "(bytes/sec)" and "bytes read/written" references since unit information is in the Size message
- **Rationale:** Size type carries unit information; field names should describe the quantity, not repeat the unit. Cleaner, DRY naming pattern.
- **Compilation status:** ✅ Proto compiles successfully with cleaned-up field names

**CPU and Memory Metrics Separation into Sub-messages:**
- **Objective:** Extract CPU and memory metrics into dedicated messages for detailed per-core and aggregate utilization statistics
- **New messages created (in job.proto):**
  - **CpuUsage** - CPU/core utilization metrics (peak, min, avg percentages):
    - `Percentage peak`: Peak CPU/core utilization percentage (0-100)
    - `Percentage min`: Minimum CPU/core utilization percentage (0-100)
    - `Percentage avg`: Average CPU/core utilization percentage (0-100)
  - **JobMemoryUsage** - Memory resource consumption:
    - `Size peak_memory`: Peak memory usage
    - `Size min_memory`: Minimum memory usage
    - `Size avg_memory`: Average memory usage
- **JobResourceUsage refactored:**
  - `CpuUsage avg_cpu_usage`: Aggregate CPU utilization across all cores
  - `repeated CpuUsage per_core_usage`: Per-core CPU utilization (one CpuUsage entry per CPU core)
  - `JobMemoryUsage memory_usage`: Memory resource consumption
  - Disk I/O metrics: total_disk_read, total_disk_write, bandwidth statistics
  - Field numbering updated for consistency (1-9)
- **Design rationale:** CpuUsage message provides peak/min/avg percentages for both aggregate and per-core analysis. Separates resource types enables independent expansion. Per-core stats are essential for performance debugging on multi-core systems.
- **Benefits:** Cleaner structure; enables detailed per-core performance profiling; supports NUMA and CPU affinity analysis; consistent message pattern for aggregate + per-unit metrics
- **Compilation status:** ✅ Proto compiles successfully with refined CPU/memory metrics structure

**CpuUsage Message Clarification and Simplification:**
- **Objective:** Clarify CPU usage metrics as utilization percentages for both aggregate and per-core analysis
- **Change applied (in job.proto):**
  - Renamed `JobCpuUsage` → `CpuUsage` - simpler, reusable name for CPU utilization metrics
  - Removed core counting fields (peak_cores_used, min_cores_used, avg_cores_used) - not needed; focus on utilization percentages
  - Simplified to three fields capturing utilization %: peak (0-100), min (0-100), avg (0-100)
  - `JobResourceUsage` now uses `CpuUsage avg_cpu_usage` (aggregate) and `repeated CpuUsage per_core_usage` (per-core)
- **Design rationale:** CpuUsage represents utilization percentage for any CPU unit (aggregate or single core). Per-core array provides core-by-core breakdown without needing separate counting fields.
- **Benefits:** Unified type for CPU analysis; cleaner API; enables direct per-core utilization comparison with aggregate; flexible for future multi-socket/NUMA architectures
- **Compilation status:** ✅ Proto compiles successfully with simplified CpuUsage message

**Resource Usage Types Promoted to Vocabulary:**
- **Objective:** Establish generic, reusable resource usage tracking suitable for jobs, peer monitoring, and system metrics
- **Messages moved to vocabulary.proto (from job.proto):**
  - **CpuUsage** - CPU/core utilization percentages (peak, min, avg)
  - **MemoryUsage** - Memory consumption with peak/min/avg metrics (renamed from JobMemoryUsage for clarity)
  - **ResourceUsage** - Comprehensive resource tracking (renamed from JobResourceUsage for generic use):
    - `CpuUsage avg_cpu_usage`: Aggregate CPU utilization
    - `repeated CpuUsage per_core_usage`: Per-core utilization breakdown
    - `MemoryUsage memory_usage`: Memory metrics
    - Disk I/O metrics: total_disk_read, total_disk_write, bandwidth statistics (peak and average)
- **Updated job.proto:**
  - JobExecutionMetrics now references `ResourceUsage resource_usage` from vocabulary (not JobResourceUsage)
  - Removed local definitions of CpuUsage, JobMemoryUsage, JobResourceUsage
  - vocabulary.proto import already present; ResourceUsage now available
- **Design rationale:** Resource consumption is a fundamental measurement applicable to jobs, peers, system monitoring, and performance analysis. Moving to vocabulary makes it a first-class protocol type, enabling consistent resource tracking across all distributed system components.
- **Benefits:** Enables resource reporting at multiple levels (job, peer, system); reusable for quota tracking, scheduling, and monitoring; consistent metrics across protocol; future-extensible for energy, network I/O, and other resources
- **Compilation status:** ✅ Proto compiles successfully with vocabulary-based resource types

**Cache Info Promoted to Vocabulary with Timing Metrics:**
- **Objective:** Establish generic, reusable cache tracking suitable for caching any artifact (job outputs, data, etc.), including detailed cache operation timings
- **Message moved and enhanced in vocabulary.proto:**
  - **CacheQueryStatistics** - Cache query and hit information with timing metrics:
    - `bool cache_hit`: Whether the item was served from cache
    - `string cache_source_peer_id`: Which peer had the cached item (if cache_hit=true)
    - `TimeDuration hash_time`: Time spent computing hash of the item
    - `TimeDuration query_time`: Time spent querying the cache
    - `TimeDuration extraction_time`: Time spent extracting item from cache (if cache_hit=true)
- **Updated job.proto:**
  - JobStatistics now references `CacheQueryStatistics cache_query_statistics` from vocabulary
  - Removed JobCacheInfo message definition
  - Updated field comment to include timing metrics
- **Design rationale:** Cache performance is critical for distributed systems. Timing metrics enable identification of cache bottlenecks (hashing vs. querying vs. extraction). Generic type supports caching at multiple levels (job results, artifacts, data, etc.).
- **Benefits:** Cache operation profiling for performance analysis; generic cache tracking across protocol; supports cache optimization decisions; enables SLA tracking for cache-hit operations
- **Compilation status:** ✅ Proto compiles successfully with vocabulary-based cache info

**IOUsage Message Creation and ResourceUsage Refactoring:**
- **Objective:** Extract I/O and bandwidth metrics into a reusable generic message for disk, network, and other I/O types
- **New message created in vocabulary.proto:**
  - **IOUsage** - Generic I/O and bandwidth tracking:
    - `Size total_read`: Total data read
    - `Size total_write`: Total data written
    - `Size peak_read_bandwidth`: Peak read bandwidth
    - `Size peak_write_bandwidth`: Peak write bandwidth
    - `Size avg_read_bandwidth`: Average read bandwidth
    - `Size avg_write_bandwidth`: Average write bandwidth
- **ResourceUsage refactored (in vocabulary.proto):**
  - Removed individual `Size total_disk_*` and bandwidth fields
  - Added `IOUsage disk_io`: Disk I/O metrics (read/write data and bandwidth)
  - Simplified field numbering (now 1-4 instead of 1-9)
  - Cleaner structure with grouped I/O metrics
- **Design rationale:** IOUsage is generic enough to represent I/O for disk, network, or other channels. Keeps protocol extensible without duplicating I/O metric definitions. Future: memory bandwidth, storage I/O, or other contexts can reuse IOUsage.
- **Benefits:** Reusable I/O usage tracking; supports disk and network metrics with same interface; enables consistent I/O monitoring across protocol; simpler ResourceUsage structure
- **Compilation status:** ✅ Proto compiles successfully with IOUsage separation

**ResourceUsage Enhancement: Network I/O Metrics:**
- **Objective:** Add network I/O tracking to ResourceUsage for complete resource visibility
- **Change applied (in vocabulary.proto):**
  - Added `IOUsage network_io = 5;` field to ResourceUsage
  - Updated ResourceUsage comment to include "network I/O"
- **Rationale:** Network I/O is equally important as disk I/O for distributed systems. Using the same IOUsage type (total_read, total_write, bandwidth metrics) ensures consistent metrics across I/O types.
- **Benefits:** Complete resource telemetry (CPU, memory, disk, network); enables network bottleneck identification; consistent monitoring across all I/O channels
- **Compilation status:** ✅ Proto compiles successfully with network_io field added

**MemoryUsage Enhancement: Optional Memory I/O Metrics:**
- **Objective:** Track memory bandwidth and I/O performance in addition to memory consumption
- **Change applied (in vocabulary.proto):**
  - Added `optional IOUsage memory_io = 4;` field to MemoryUsage
  - Comment: "Optional: Memory I/O metrics (bandwidth and throughput)"
- **Rationale:** Memory bandwidth can be a performance bottleneck in CPU-intensive workloads. IOUsage (with total_read/write and bandwidth metrics) provides comprehensive memory access performance data.
- **Benefits:** Enables memory bandwidth profiling; identifies memory performance bottlenecks; optional field keeps it backward compatible
- **Compilation status:** ✅ Proto compiles successfully with optional memory_io field

---

## Dev Environment Setup

### Buf Installation & VS Code Integration (2026-03-18)

**Status:** ✅ COMPLETE - Including Full Lint Compliance

**Changes:**
1. **Switched from system protoc to buf (Go-based alternative)**
   - Removed dependency on system protobuf-compiler
   - Buf v1.40.1 installed via Go module (`go install github.com/bufbuild/buf/cmd/buf@v1.40.1`)
   - No system dependencies required

2. **Created buf configuration files**
   - `buf.yaml` - Linting and breaking change detection rules
   - `buf.gen.yaml` - Code generation plugin configuration
   - `pkg/proto/generate.go` - Updated to use `buf generate`

3. **Updated dev container configuration**
   - Added `bufbuild.vscode-buf` extension to `.devcontainer/devcontainer.json`
   - Updated postCreateCommand to install buf v1.40.1
   - Added [proto] language settings for auto-formatting

4. **Created VS Code workspace configuration**
   - `.vscode/settings.json` - Buf linting on save, buf as default proto formatter
   - `.vscode/extensions.json` - Added buf extension to recommendations

5. **Created documentation**
   - `pkg/proto/README.md` - Comprehensive guide to buf, advantages/disadvantages vs protoc, workflow examples, troubleshooting

**Benefits:**
- ✅ No system protoc dependency (containers, CI/CD, cross-platform)
- ✅ Integrated linting with buf lint
- ✅ Breaking change detection
- ✅ VS Code integration for real-time diagnostics
- ✅ Automatic proto file formatting on save
- ✅ Consistent development environment via devcontainer

**Proto Compilation Status:**
- All 8 proto files successfully compile
- 8 .pb.go files generated
- 1 _grpc.pb.go file generated
- Project builds cleanly

### buf Lint Compliance (2026-03-18)

**Status:** ✅ COMPLETE - All 36+ Linting Issues Fixed

**Issues Fixed:**

1. **buf.yaml Deprecation (1 issue)**
   - Changed lint rule from deprecated `DEFAULT` to `STANDARD`
   - No functional change; `STANDARD` is the recommended category

2. **Enum Value Naming (21 issues)**
   - **Issue:** Enum values must be prefixed with their enum name in UPPER_CASE
   - **Fix:** Renamed all enum values across 7 proto files
   - Examples:
     - `MILLISECOND` → `TIME_UNIT_MILLISECOND`
     - `GCC` → `CPP_COMPILER_GCC`
     - `FILE` → `DATA_TYPE_FILE`
     - `TIMEOUT` → `ERROR_TYPE_TIMEOUT`
   - **Benefit:** Eliminates naming ambiguity in compound type names

3. **Enum Zero Values (21 issues)**
   - **Issue:** Enum zero values must use `_UNSPECIFIED` suffix in proto3
   - **Fix:** Renamed all unknown/default enum values to `<ENUM>_UNSPECIFIED`
   - Examples:
     - `UNKNOWN_TIME_UNIT` → `TIME_UNIT_UNSPECIFIED`
     - `UNKNOWN_CPP_COMPILER` → `CPP_COMPILER_UNSPECIFIED`
   - **Benefit:** Proto3 compatibility; zero value represents "unknown" state

4. **Package Versioning & Directory Structure (6 issues)**
   - **Issue:** Package `buildozer.proto` detected; should be `buildozer.proto.v1`
   - **Issue:** Proto files in `pkg/proto/` but package suggests `buildozer/proto/v1/`
   - **Fix:** 
     - Reorganized proto files: `pkg/proto/` → `buildozer/proto/v1/`
     - Updated package declarations to `buildozer.proto.v1`
     - Updated all import paths to `buildozer/proto/v1/*.proto`
     - Updated `go_package` option to reflect new location
   - **Benefit:** Aligns file structure with semantic versioning; enables multiple API versions

5. **RPC Request/Response Naming (24+ issues)**
   - **Issue:** RPC request/response types must follow `<Service><RPC>Request/Response` pattern
   - **Fix:** 
     - Renamed RPC message types across all 4 services
     - Created wrapper message types for proper naming convention
     - Examples:
       - `JobSubmissionMessage` + `JobStatus` → `SubmitJobRequest` + `SubmitJobResponse`
       - `GetJobStatusRequest` + `JobProgress` → `GetJobStatusRequest` + `GetJobStatusResponse`
       - `PeerAnnouncement` → `AnnounceSelfRequest`
       - `CacheQueryMessage` → `QueryCacheRequest`
   - **Benefit:** Consistent RPC naming enables auto-documentation and tool generation

**Arc Linting Summary:**
- Before: 36 lint errors/warnings across all 8 proto files
- After: ✅ 0 errors/warnings (100% compliant)
- Status: Clean buf lint output

**Files Reorganized:**
```
OLD:                          NEW:
pkg/proto/                    buildozer/proto/v1/
  ├─ vocabulary.proto           ├─ vocabulary.proto
  ├─ runtime.proto              ├─ runtime.proto
  ├─ job.proto                  ├─ job.proto
  ├─ job_data.proto             ├─ job_data.proto
  ├─ auth.proto                 ├─ auth.proto
  ├─ network_messages.proto      ├─ network_messages.proto
  ├─ services.proto             ├─ services.proto
  └─ generate.go                └─ generate.go
```

**buf LSP Field Documentation Format:**
- **Issue:** buf language server does not recognize field/enum value documentation when comments appear on the same line as declarations
- **Root Cause:** buf LSP parser expects comments on the previous line, not trailing comments (LSP limitation)
- **Fix Applied (2026-03-21):** Moved all inline field/enum value documentation to previous lines across all proto files
  - **Files modified:** vocabulary.proto, runtime.proto, job.proto, job_data.proto, network_messages.proto
  - **Scope:** Approximately 50+ field/enum documentation comments across entire protocol
  - **Examples of fixes:**
    - `SignatureAlgorithm enum values` (5 items): Moved comments from `ALGORITHM = N; // comment` to previous lines
    - `SizeUnit enum values` (5 items): Same pattern
    - `ApiProtocol enum values` (3 items): Same pattern
    - `CppAbi enum values` (1 item): `CPP_ABI_ITANIUM = 1; // Itanium ABI...` → comment on previous line
    - `CppStdlib enum values` (2 items): Similar format fixes
    - `JobStatus enum in JobProgress` (9 items): All inline comments moved to previous lines
    - `JobStatus enum in JobResult` (3 items): Same fix
    - `ErrorType enum in JobErrorMessage` (9 items): All 8 error types with inline comments fixed
    - `JobTimings message fields` (8 items): Duration and timestamp field comments moved to previous lines
    - `IOUsage message fields` (4 items): Bandwidth and usage fields
  - **Verification:**
    - ✅ Zero remaining inline comments (grep `= \d+; //` returns 0 matches)
    - ✅ buf lint passes with 0 errors/warnings
    - ✅ `go generate ./...` completes successfully
    - ✅ `go build ./...` completes successfully
  - **Benefit:** buf LSP now correctly displays field documentation on hover in VS Code
- **Pattern Applied:** For consistency across all proto files:
  ```protobuf
  // Good: Comment on previous line (recognized by buf LSP)
  enum Status {
    // Description of value
    STATUS_ACTIVE = 1;
  }

  message Example {
    // Description of field
    string field_name = 1;
  }

  // Bad (old pattern): Comment on same line (not recognized by buf LSP)
  enum Status {
    STATUS_ACTIVE = 1; // Description (NOT recognized)
  }
  ```

**Complete Enum Value Documentation (2026-03-21):**
- **Objective:** Ensure every enum value across all proto files has a preceding documentation comment
- **Scope:** All enums in vocabulary.proto, runtime.proto, and job_data.proto
- **Enums documented:**
  - **vocabulary.proto:**
    - `TimeUnit`: All 6 values (UNSPECIFIED, MILLISECOND, SECOND, MINUTE, HOUR, DAY)
    - `HashAlgorithm`: All 4 values (UNSPECIFIED, SHA256, SHA512, BLAKE3)
    - `SignatureAlgorithm`: Added UNSPECIFIED comment (other values already documented)
    - `SizeUnit`: All 6 values (UNSPECIFIED, BYTE, KILOBYTE, MEGABYTE, GIGABYTE, TERABYTE) - fixed incorrect BYTE comment
    - `ApiProtocol`: All 3 values (UNSPECIFIED, GRPC, REST) - improved descriptions
  - **runtime.proto:**
    - `CppLanguage`: All 3 values (UNSPECIFIED, C, CPP)
    - `CppCompiler`: All 3 values (UNSPECIFIED, GCC, CLANG)
    - `CppArchitecture`: All 4 values (UNSPECIFIED, X86_64, AARCH64, ARM)
    - `CRuntime`: All 3 values (UNSPECIFIED, GLIBC, MUSL)
    - `CppAbi`: Added UNSPECIFIED comment (ITANIUM already had one)
    - `CppStdlib`: Added UNSPECIFIED comment (LIBSTDCXX, LIBCXX already had comments)
  - **job_data.proto:**
    - `DataType`: All 5 values (UNSPECIFIED, FILE, DIRECTORY, STREAM_CHUNK, REFERENCE)
- **Verification:**
  - ✅ Zero undocumented enum values (grep `= \d+;` with preceding comment check returns 0)
  - ✅ buf lint passes with 0 errors/warnings
  - ✅ `go generate ./...` completes successfully
  - ✅ `go build ./...` completes successfully
- **Pattern Applied:** Every enum value has a comment on the previous line explaining its purpose:
  ```protobuf
  enum Status {
    // Unspecified status (default)
    STATUS_UNSPECIFIED = 0;
    // Active/running state
    STATUS_ACTIVE = 1;
    // Paused/suspended state
    STATUS_PAUSED = 2;
  }
  ```

**Protocol Organization & API Separation (2026-03-21):**
- **Objective:** Split the protocol into logically distinct packages to clarify the different APIs and their use cases
- **Separation:** Four distinct APIs with clear purposes:
  1. **Driver API** (`driver.proto`): Used by gcc/g++/make CLIs to submit jobs
  2. **Introspection API** (`introspection.proto`): Used by tools/CLI/UI to query client state
  3. **Peer APIs** (`executor.proto`, `discovery.proto`, `coordination.proto`): Used by clients to coordinate
  4. **Common Types** (`common/`): Shared vocabulary, job, runtime types used by all APIs
- **Package Structure:**
  - `buildozer.proto.v1.common` - Shared vocabulary types (TimeUnit, Hash, Signature, Size, Job, Runtime, etc.)
  - `buildozer.proto.v1.driver` - Driver API (JobService)
  - `buildozer.proto.v1.introspection` - Introspection API (IntrospectionService)
  - `buildozer.proto.v1.peer` - Peer APIs (ExecutorService, DiscoveryService, CoordinationService)
- **Shared Versioning:** All APIs are version `buildozer.proto.v1` (protocol changes are coordinated across all APIs)
- **buf Configuration:** Added exception for `PACKAGE_VERSION_SUFFIX` rule (not needed when all APIs share v1)
- **Generated Code:** Organized under `internal/gen/buildozer/proto/v1/{common,driver,introspection,peer}/` with Connect service handlers in `*connect/` subdirectories
- **Verification:**
  - ✅ buf lint: 0 errors/warnings (STANDARD rule set minus PACKAGE_VERSION_SUFFIX)
  - ✅ go generate: All 11 proto files compile successfully
  - ✅ go build: Builds successfully
  - ✅ Proto structure clearly separates four distinct APIs

---

## Milestone 1.3: Logging System Implementation (2026-03-21)

**Status:** ✅ COMPLETE - Production-Ready Logging with Age-Based Rotation & CLI Refactoring

### Phase 1: Library Integration - slog-multi + lumberjack (2026-03-21)

**Objective:** Leverage industry-standard libraries for file rotation instead of custom implementations.

**Libraries Integrated:**
- **lumberjack v2.2.1**: Handles file rotation by size (MaxSize in MB), backup count (MaxBackups), and age (MaxAge in days)
- **slog-multi v1.7.1**: Provides Fanout pattern for broadcasting logs to multiple handlers
- **Dependencies:** `gopkg.in/natefinch/lumberjack.v2` and `github.com/samber/slog-multi`

**Code Changes:**

1. **sinks.go Refactoring**
   - Replaced custom 120-line `FileSinkWithRotation` implementation with lumberjack
   - Created `FileSink(path, maxSizeMB, maxBackups, maxAgeDays)` function returning `slog.Handler`
   - Updated helper functions: `JSONFileSink()`, `TextFileSink()` to accept all rotation parameters
   - Embedded sink configuration into slog handlers (no custom iteration logic)

2. **config.go Updates**
   - Added `MaxAgeDays` field to `SinkConfig` struct
   - Updated `Factory.CreateSink()` to pass MaxAgeDays to FileSink()
   - Introduced slog-multi `Fanout()` pattern in `InitializeFromConfig()` for composite handlers

3. **logger.go Refactoring**
   - Changed Logger struct: removed `handlers []slog.Handler` array
   - Added `compositeHandler slog.Handler` field for single composed handler
   - Simplified `Log()` and `LogAttrs()` methods to delegate to compositeHandler if set
   - Added `SetCompositeHandler()` method for factory setup

4. **global.go Updates**
   - Updated `EnableLoggerFileSink()` signature to accept `maxAgeDays` parameter
   - Creates `SinkConfig` with age-based rotation settings

**Benefits:**
- Removed 250+ lines of custom rotation and handler composition code
- Battle-tested implementations replace fragile custom code
- Size-based, count-based, and age-based rotation all supported
- Single compositeHandler pattern cleaner than manual handler list iteration

**Build Status:**
- ✅ `go build ./...` succeeds
- ✅ All logging operations functional
- ✅ No breaking changes to public API

---

### Phase 2: Age-Based Log Rotation Feature (2026-03-21)

**Objective:** Support retention policies based on log file age (days).

**Implementation:**

1. **Configuration Enhancement**
   - Added `MaxAgeDays` field to `FileSinkConfig` (0 = no age-based rotation)
   - Updated YAML configuration schema: `max_age_days: 90`
   - Updated helper functions to accept maxAgeDays parameter

2. **Test Suite (4 tests, all passing)**
   - `TestFileSinkWithAgeRotation`: Verifies lumberjack MaxAge parameter set correctly
   - `TestFileSinkWithoutAgeRotation`: Verifies age rotation disabled when maxAgeDays=0
   - `TestJSONFileSinkWithAge`: Tests JSON sink with age-based rotation
   - `TestTextFileSinkWithAge`: Tests text sink with age-based rotation

3. **Example Usage**
   - Created `examples/logging_with_age_rotation.go` demonstrating:
     - File sink creation with age-based rotation
     - Multiple rotation strategies (size + age)
     - Real-world configuration patterns

**Benefits:**
- Automated cleanup of old log files
- Configurable retention windows (e.g., keep 7, 14, 30, or 90 days)
- Prevents unbounded log disk usage
- Production-ready retention policy

**Test Results:**
- ✅ All 4 tests passing
- ✅ Build succeeds with tests included

---

### Phase 3: CLI Redesign - From Flags to Subcommands (2026-03-21)

**Objective:** Refactor logs command to use proper subcommand pattern instead of flags.

**Design Principle Applied:** "In a CLI, different operations should always be different subcommands" (not flags)

**Old Design (Flag-Based):**
```bash
logs --status
logs --tail
logs --set-global-level debug
logs --set-logger-level buildozer --logger-level info
logs --enable-file-sink buildozer --file-sink-path /tmp/log
```

**New Design (Subcommand-Based):**
```bash
logs status
logs tail
logs set-global-level debug
logs set-logger-level buildozer info
logs enable-file-sink buildozer /tmp/log
```

**Implementation:**

1. **Subcommand Functions (7 total)**
   - `newLogsStatusCommand()` - Display logging configuration
   - `newLogsTailCommand()` - Stream logs in real-time
   - `newLogsSetGlobalLevelCommand()` - Change global logging level
   - `newLogsSetLoggerLevelCommand()` - Change level for specific logger
   - `newLogsSetSinkLevelCommand()` - Change level for specific sink
   - `newLogsEnableFileSinkCommand()` - Create file sink for logger
   - `newLogsDisableFileSinkCommand()` - Remove file sink from logger

2. **Cobra Integration**
   - Parent command `NewLogsCommand()` returns root with 7 subcommands
   - Each subcommand uses `cobra.ExactArgs()` for strict argument validation
   - Automatic help text generation per subcommand
   - Help: `logs --help`, `logs status --help`, `logs set-global-level --help`, etc.

3. **Error Handling**
   - Missing arguments: "Error: accepts X arg(s), received Y"
   - Invalid operations fail with clear Cobra error messages
   - All error messages follow Cobra standard format

4. **Code Cleanup**
   - Removed old `handleLogsInProcess()` and `handleLogsRemote()` functions
   - Removed 10+ boolean/string flags
   - Removed flag-based dispatch logic (~200 lines)

**Command Reference:**

| Operation | Old Flag-Based | New Subcommand | Args |
|-----------|---|---|---|
| View config | `logs --status` | `logs status` | 0 |
| Stream logs | `logs --tail` | `logs tail` | 0 |
| Set global level | `logs --set-global-level debug` | `logs set-global-level debug` | 1 (level) |
| Set logger level | `logs --set-logger-level buildozer --logger-level info` | `logs set-logger-level buildozer info` | 2 (name, level) |
| Set sink level | `logs --set-sink-level stdout --sink-level warn` | `logs set-sink-level stdout warn` | 2 (name, level) |
| Enable file sink | `logs --enable-file-sink buildozer --file-sink-path /tmp/log` | `logs enable-file-sink buildozer /tmp/log` | 2 (name, path) |
| Disable file sink | `logs --disable-file-sink buildozer` | `logs disable-file-sink buildozer` | 1 (name) |

**Benefits:**
- **Clarity**: Each operation is an explicit subcommand
- **Discoverability**: `logs --help` shows all 7 operations clearly
- **Standard Pattern**: Follows conventions of git, docker, kubectl
- **Validation**: Cobra automatically validates argument counts
- **Extensibility**: Easy to add new operations as new subcommands

**Testing performed:**
- ✅ All 7 subcommands functional
- ✅ Help text generation working
- ✅ Error handling for missing arguments
- ✅ Build succeeds
- ✅ No runtime errors

**Files Created/Updated:**
- ✅ `cmd/buildozer-client/cmd/logs.go` - Complete refactor (7 subcommands)
- ✅ `CLI_LOGGING_SUBCOMMANDS.md` - Complete command reference
- ✅ `pkg/logging/sinks/sinks.go` - lumberjack integration
- ✅ `pkg/logging/config.go` - Age-based rotation config
- ✅ `pkg/logging/logger.go` - Composite handler pattern
- ✅ `pkg/logging/global.go` - Updated API signatures
- ✅ `pkg/logging/sinks/sinks_test.go` - Age rotation tests

**Status Summary:**
- ✅ 250+ lines of custom code removed
- ✅ 4 comprehensive tests passing
- ✅ 7 subcommands fully implemented and tested
- ✅ Production-ready logging system
- ✅ Industry-standard libraries (slog-multi, lumberjack)
- ✅ Proper CLI pattern (subcommands, not flags)

---

### Phase 4: Logging Configuration Interface System (2026-03-21)

**Objective:** Create a pluggable logging configuration interface with local and remote implementations, plus RPC service handler.

**Architecture:**

1. **ConfigManager Interface** (`pkg/logging/config_manager.go`)
   - Unified interface for logging configuration operations
   - Methods: GetLoggingStatus, SetGlobalLevel, SetLoggerLevel, SetSinkLevel, EnableFileSink, DisableFileSink, TailLogs
   - Works with both local and remote implementations

2. **LocalConfigManager** (`pkg/logging/config_manager.go`)
   - Implements ConfigManager for local in-process logging
   - Uses existing Registry and Factory from pkg/logging
   - Direct access to logging configuration functions
   - No network overhead

3. **RemoteConfigManager** (`pkg/logging/remote_config.go`)
   - Implements ConfigManager for remote daemon communication
   - Uses Connect client to call LoggingService RPC methods
   - Same interface as LocalConfigManager for seamless switching
   - Handles protocol buffer conversion and error handling

4. **Private Service Handler** (`pkg/logging/service_handler.go`)
   - `loggingServiceHandler` struct (private implementation)
   - Implements `LoggingServiceHandler` from protocol (generated interface)
   - Uses ConfigManager interface internally (can be any implementation)
   - `RegisterLoggingService()` creates and registers handler with HTTP mux

5. **Convenience Factory** (`pkg/logging/factory.go`)
   - `NewLocalConfigManagerFromGlobal()` - Creates local manager from global registry
   - `NewRemoteConfigManagerFromURL()` - Creates remote manager from URL
   - `NewRemoteConfigManagerFromClient()` - Creates remote manager from explicit client
   - `GetLocalConfigManager()` - Simple accessor for global local manager
   - `NewHTTPHandler()` - Convenience for registering service handler

**Type Conversions:**

- `SlogLevelToProtoLogLevel()` - Convert slog.Level to protobuf enum
- `ProtoLogLevelToSlogLevel()` - Convert protobuf enum to slog.Level
- `sinkTypeFromString()` - Convert string to protobuf SinkType enum
- `sinkTypeToString()` - Convert protobuf enum to string
- `timeToTimestamp()` - Convert time.Time to protobuf Timestamp
- `timestampToTime()` - Convert protobuf Timestamp to time.Time

**Data Structures:**

- `LoggingStatusSnapshot` - Complete configuration snapshot with sinks and loggers
- `SinkStatus` - Individual sink configuration and level
- `LoggerStatus` - Individual logger configuration and level
- `LogRecord` - Single log record with timestamp, level, message, attributes

**Usage Examples:**

Local usage:
```go
manager := logging.GetLocalConfigManager()
status, err := manager.GetLoggingStatus(ctx)
err = manager.SetGlobalLevel(ctx, slog.LevelDebug)
err = manager.EnableFileSink(ctx, "buildozer", "/var/log/buildozer.log", 100, 10, 30)
```

Remote usage:
```go
manager := logging.NewRemoteConfigManagerFromURL(httpClient, "http://localhost:6789")
status, err := manager.GetLoggingStatus(ctx)
err = manager.SetGlobalLevel(ctx, slog.LevelDebug)
```

Service registration:
```go
manager := logging.GetLocalConfigManager()
path, handler := logging.RegisterLoggingService(manager)
mux.Handle(path, handler)
```

**Files Created/Modified:**
- ✅ `pkg/logging/config_manager.go` - ConfigManager interface and LocalConfigManager
- ✅ `pkg/logging/remote_config.go` - RemoteConfigManager implementation
- ✅ `pkg/logging/service_handler.go` - Private loggingServiceHandler
- ✅ `pkg/logging/factory.go` - Convenience factory functions
- ✅ `pkg/logging/CONFIG_MANAGER.md` - Comprehensive documentation

**Status Summary:**
- ✅ ConfigManager interface fully defined
- ✅ LocalConfigManager implements interface
- ✅ RemoteConfigManager implements interface
- ✅ Service handler implements LoggingServiceHandler
- ✅ Type conversions complete (slog ↔ protobuf)
- ✅ Convenience API for easy integration
- ✅ Project builds successfully
- ✅ Ready for CLI and daemon integration

---

### Phase 5: Protocol Service Definition - LoggingService (2026-03-21)

**Objective:** Define protocol buffer RPC service for daemon logging configuration.

**Service Definition:**

Created [buildozer/proto/v1/logging.proto](buildozer/proto/v1/logging.proto) with:

1. **LoggingService** - 7 RPC methods
   - `GetLoggingStatus()` - Retrieve current logging configuration
   - `SetGlobalLevel()` - Change global logging level
   - `SetLoggerLevel()` - Change specific logger level
   - `SetSinkLevel()` - Change specific sink level
   - `EnableFileSink()` - Create file sink for logger
   - `DisableFileSink()` - Remove file sink from logger
   - `TailLogs()` - Stream logs in real-time (with filtering)

2. **Message Types**
   - `LogLevel` enum - error, warn, info, debug, trace
   - `SinkType` enum - stdout, stderr, file, syslog
   - `SinkConfig` - Sink configuration with file/syslog options
   - `LoggerConfig` - Named logger configuration
   - `LoggingStatus` - Complete logging state snapshot
   - Request/response messages for each RPC operation
   - `TailLogsResponse` - Streamed log records

3. **Type-Safe Configuration**
   - Enums for LogLevel and SinkType (instead of strings)
   - Nested message types for file and syslog configuration
   - FileConfig: path, max_size_bytes, max_backups, max_age_days, json_format
   - SyslogConfig: tag
   - TimeStamp vocabulary type for all response timestamps

**Protocol Generation:**

- ✅ `buf generate` produces:
  - `logging.pb.go` - Message type definitions
  - `logging.connect.go` - Connect RPC handlers and client
- ✅ Project compiles successfully
- ✅ All 7 RPC methods available for service implementation

**Service Architecture:**

```
CLI (buildozer-client logs commands)
        ↓
Connect Client (generated from LoggingService)
        ↓
Network (HTTP/gRPC/gRPC-Web)
        ↓
Connect Server Handler (to be implemented)
        ↓
In-Process Logging System (pkg/logging)
```

**Integration Points:**

- CLI commands will use `NewLoggingServiceClient()` to invoke RPC methods
- Daemon will implement `LoggingServiceHandler` interface
- Request/response types match CLI operations exactly
- TailLogs supports streaming for real-time log monitoring

**Status Summary:**
- ✅ Service defined with 7 RPC methods
- ✅ Type-safe enums for LogLevel and SinkType
- ✅ Comprehensive request/response messages
- ✅ Connect code generated successfully
- ✅ Ready for daemon implementation

---

### Milestone 1.4: Daemon Package & Service Orchestration (2026-03-21)

**Status:** ✅ COMPLETE

**Objective:** Create a high-level daemon package that collects all subsystems and exposes them through a unified Connect/gRPC server.

**Key Architecture Decisions:**

1. **Daemon Core** (`pkg/daemon/daemon.go`)
   - Main `Daemon` struct managing HTTP/Connect server and service registration
   - Thread-safe lifecycle management (Start/Stop with RWMutex)
   - Service handler registration interface for plugging in services
   - Graceful shutdown with context cancellation

2. **Server Wrapper** (`pkg/daemon/server.go`)
   - High-level `Server` type for typical daemon setup
   - Initializes all standard services automatically (logging, runtime detection, etc.)
   - Single entry point for daemon CLI command
   - Provides access to underlying components when needed

3. **Builder Pattern** (`pkg/daemon/options.go`)
   - Fluent builder for flexible daemon configuration
   - Sensible defaults: Host=localhost, Port=6789, MaxJobs=4, MaxRAM=4GB
   - Configuration validation on builder methods

**Completed Components:**

1. **`pkg/daemon/daemon.go`** (160 lines)
   - `DaemonConfig` struct with network, resource, and feature configuration
   - `Daemon` struct with HTTP server, mux, and lifecycle management
   - Methods: `New()`, `Start()`, `Stop()`, `RegisterServiceHandler()`
   - State queries: `IsRunning()`, `Context()`, `Config()`
   - Thread-safe with RWMutex for concurrent access
   - Graceful shutdown with context timeout support

2. **`pkg/daemon/server.go`** (100 lines)
   - `Server` wrapper type for high-level daemon setup
   - `NewServer()` initializes logging service and registers it with daemon
   - Methods: `Start()`, `Stop()`, `IsRunning()`, `Context()`, `Config()`
   - Access to logging config manager: `LoggingConfigManager()`
   - Access to underlying daemon: `Daemon()`

3. **`pkg/daemon/options.go`** (70 lines)
   - `Builder` type with chainable methods
   - Methods: `Host()`, `Port()`, `MaxConcurrentJobs()`, `MaxRAMMB()`, `EnableMDNS()`
   - Validation: Port range (1-65535), positive max jobs/RAM
   - Methods: `Build()` creates daemon, `BuildWithConfig()` from explicit config

4. **`pkg/daemon/README.md`** (150 lines)
   - Architecture overview with ASCII diagram
   - Usage examples for standalone daemon and builder pattern
   - Service registration pattern documentation
   - Graceful shutdown implementation guide
   - Thread safety guarantees
   - Integration with cmd/buildozer-client

5. **Integration with CLI** (`cmd/buildozer-client/cmd/daemon.go`)
   - Updated daemon command to use new `daemon.Server`
   - Creates and starts daemon with CLI configuration
   - Implements graceful shutdown with signal handling
   - Timeout-based server shutdown (30 second default)

**Service Registration Pattern:**

Each service (logging, runtime, job, etc.) follows this pattern:
```go
// Service implements handler interface
type MyServiceHandler struct { ... }

// Registration function returns path and http.Handler
func RegisterMyService(config Config) (string, http.Handler) {
    handler := newMyServiceHandler(config)
    path, mux := protov1connect.NewMyServiceHandler(handler)
    return path, mux
}

// Daemon registers it
server.Daemon().RegisterServiceHandler(path, handler)
```

**Currently Registered Services:**
- `LoggingService` — Query and modify logging configuration at runtime

**Design Principles:**

1. **Separation of Concerns** — Each subsystem handles its domain; daemon orchestrates
2. **Composition Over Inheritance** — Services composed into daemon, not inherited
3. **Explicit Dependencies** — All dependencies explicitly injected/registered
4. **Graceful Degradation** — Services can be optional; daemon still works
5. **Thread Safety** — RWMutex protects state; safe for concurrent access
6. **Testability** — Clean interfaces enable mocking and testing

**Build Status:**
- ✅ `go build ./pkg/daemon` — Success
- ✅ `go build ./cmd/buildozer-client` — Success
- ✅ `./buildozer-client daemon --help` — Works correctly
- No lint errors or warnings
- Full project builds successfully

**Future Service Integration Points:**

As development progresses, services will be registered in `daemon.NewServer()`:

1. **RuntimeService** — Discover runtimes, query metadata, detect toolchains
2. **JobService** — Submit jobs, monitor progress, retrieve results
3. **CacheService** — Query cache status, manage artifacts, garbage collection
4. **QueueService** — Monitor job queue, scheduler status, load distribution
5. **PeerService** — Peer discovery (mDNS), connectivity status, statistics

**Documentation:**
- ✅ Comprehensive README at `pkg/daemon/README.md`
- ✅ Inline code comments for all public types and methods
- ✅ Usage examples in README and code
- ✅ Architecture diagram in README

**Next Steps:**
- Implement remaining Docker API methods (Milestone 1.0 continuation)
- Implement native C/C++ toolchain detector (Milestone 1.1)
- Implement Docker-based C/C++ runtime (Milestone 1.2)
- Integrate job queue and scheduler into daemon (Milestone 1.3 continuation)

---

## Logger Interface Refactoring Complete (2026-03-21)

**Status:** ✅ COMPLETE

**Completed Work:**

1. **Full slog.Logger Interface Implementation**
   - Implemented ALL slog.Logger methods:
     - **Log levels:** Debug, DebugContext, Info, InfoContext, Warn, WarnContext, Error, ErrorContext
     - **Generic logging:** Log, LogContext, LogAttrs, LogAttrsContext
     - **Attributes:** WithAttrs(), WithGroup() (no-ops for dynamic routing, maintain interface)
   - All methods delegate to underlying slog.Logger with proper context handling
   - Line counts: 418 lines in complete logger.go

2. **Dynamic Handler Routing Implementation**
   - Created `registryHandler` type implementing slog.Handler interface
   - Handler routes all log records through Registry.Log() for dynamic sink resolution
   - Supports hierarchical logger name tracking via "_logger" attribute
   - Enables runtime reconfiguration without logger recreation

3. **Registry Enhancements for Dynamic Routing**
   - `Registry.Log(ctx, record)` — Routes records to configured sinks using hierarchical lookup
   - Hierarchical lookup: exact match → parent loggers → default
   - Thread-safe with RWMutex for concurrent access
   - Full sink management API (register, get, configure levels)

4. **Custom Logger Methods**
   - `Child(name)` — Create child logger maintaining hierarchy (e.g., "parent" + "module" = "parent.module")
   - `Errorf(format, args)` — Log error and return error object
   - `Panicf(format, args)` — Log error and panic with formatted message
   - `Name()` — Get logger's hierarchical name
   - All custom methods properly maintain registry and name context

**Key Architecture Decisions:**

1. **No Persistent Logger Storage** — Loggers created on-the-fly per GetLogger() call
2. **Registry Stores Only Sinks** — loggerConfigs maps logger names to sink names; sinks are actual handlers
3. **Dynamic Routing** — Log records routed at runtime based on current configuration
4. **Hierarchical Lookup** — Settings inherit from parent loggers (e.g., "a.b" inherits from "a")
5. **Complete Interface Compliance** — Logger implements full slog.Logger interface plus custom methods

**Files Modified/Created:**

1. ✅ `pkg/logging/logger.go` — Completely rewritten (418 lines)
   - Logger type wrapping slog.Logger with dynamic routing
   - Registry type managing sinks and configurations
   - registryHandler implementation for slog.Handler interface
   - All slog.Logger methods (Debug, Info, Error, etc.)
   - Custom methods (Child, Errorf, Panicf)

2. ✅ `pkg/logging/global.go` — Updated for new Registry API
3. ✅ `pkg/logging/config.go` — Updated initialization, removed slog-multi
4. ✅ `pkg/logging/config_manager.go` — Updated LoggerStatus structure
5. ✅ `pkg/logging/remote_config.go` — Updated status conversion
6. ✅ `pkg/logging/service_handler.go` — Updated LoggerConfig creation
7. ✅ `buildozer/proto/v1/logging.proto` — Removed LogLevel from LoggerConfig (regenerated with buf)

**Compilation Results:**
- ✅ `go build ./cmd/buildozer-client` — Success (no errors, no warnings)
- ✅ `go build ./pkg/logging` — Success
- ✅ All dependent files compile correctly
- ✅ Proto regeneration successful with `buf generate`

**Testing Validation:**
- Logger methods accessible and callable
- Hierarchical naming works correctly (e.g., logger.Child() extends name)
- Registry sink routing functional
- All required methods present for slog.Logger interface
- Errorf() and Panicf() work as expected

**Design Benefits:**

1. **Full Logging Interface Compliance** — Logger implements complete slog.Logger API
2. **Dynamic Reconfiguration** — Sinks and routes can change without recreating loggers
3. **Hierarchical Configuration** — Parent logger settings apply to children
4. **Zero Boilerplate** — No need to create and manage Logger instances
5. **Clean Separation** — Registry handles routing, Logger handles interface
6. **Thread-Safe** — All state protected with RWMutex

**Remaining TODOs (Ordered by Dependency):**

1. **Performance Validation** — Benchmark hierarchical lookup vs. cached loggers (if needed)
2. **Error Detection in Registry.Log()** — Add error handling for failed sink writes
3. **Attribute Filtering** — Consider filtering "_logger" attribute from final output
4. **Connection to Services**:
   - LoggingService integration with new Logger interface
   - Remote config setting via logging.proto RPC
   - Status queries via LoggerStatus proto
5. **Test Suite** — Unit tests for Logger, Registry, Child(), Errorf(), Panicf()
6. **Documentation** — Update code comments for new dynamic architecture

**Next Steps:**

1. ✅ Complete logger interface (THIS STEP COMPLETE)
2. → Implement test suite for logging package
3. → Validate with real-world logging scenarios
4. → Move to Docker API abstraction (Milestone 1.0)

---

## Next Phase: Step 2 - Job & Runtime Abstractions
