# Buildozer Development Plan
## Peer-to-Peer Distributed Build System

**Version:** 2.0  
**Status:** Ready for implementation  
**Last Updated:** 2026-03-17

---

## Conversation Summary & Requirements Gathering

**Date:** March 17, 2026  
**Participant:** Product/Architecture Owner  
**Purpose:** Define comprehensive development plan for Buildozer from scratch

### Product Description (User's Words)

Buildozer is a **distributed build system** with a **peer-to-peer architecture** featuring:

- **No central server** — Nodes auto-discover via mDNS without manual registration
- **Three core components:**
  - **Driver:** CLI mimicking standard build tools (gcc, g++, make) — prepares and submits jobs, collects results
  - **Client:** Daemon running on user machines — accepts submissions, coordinates with peers, manages queue, executes runtimes
  - **Runtime:** Execution environment — defined by toolchain (language, compiler, version, architecture, C runtime), can be native or Docker

- **Generic job model:** Input/output abstraction, could be files, objects, or abstract data
- **Job caching:** If any client in network has output for a job, return it directly; jobs uniquely hashed by (type + required runtime + inputs)
- **Smart input distribution:**
  - Input can be embedded or referenced
  - Referenced input: check local cache first, ask network if not found
  - Multiple clients can send different input parts to execution client in parallel

- **Content-addressed data:** Inputs/outputs uniquely hashed since different clients may have different versions
- **Runtime-tagged outputs:** Outputs tagged with exact runtime that generated them; inputs optionally tagged with runtime
- **Generic JobData type:** Single abstraction for both inputs and outputs
- **gRPC with oneof messages:** Generic messages with oneof fields for concrete types (extensible protocol)

- **Client resource constraints:** Configure max RAM, CPU cores, concurrent jobs per client; shared with network for scheduling
- **Runtime requirements:** Runtime specs can have explicit max cores, RAM, concurrent job limits
- **Error handling:** Job errored if no compatible runtime; if compatible runtimes exist but all busy, job queued at source
- **Network consensus:** Quorum mechanism ensures all peers agree on job scheduling (no two jobs assigned to same client simultaneously)
- **Local execution affinity:** Scheduler favors jobs executing on source client; weight configurable

- **Runtime recipes:** Jobs can include runtime specification/recipe (e.g., Dockerfile) embedded in job definition; recipes are hasheable (part of cache key)
- **Query API:** Access via CLI (non-daemon mode), REST API, and web UI; query client capabilities, config, queue, peer info, scheduling, cache details, etc.
- **Job dependencies:** System auto-detects dependencies by analyzing inputs/outputs; supports both single-job and complete job graph submission

- **C/C++ implementation:** Three drivers (gcc, g++, make); runtimes identified by: language (C/C++), compiler, compiler version, architecture, C runtime, C runtime version; two job types per language (compile: source + args → object file; link: objects + args → executable/library)
- **Runtime implementations:** Native system toolchains OR standard Docker images (Dockerfiles compiled on-the-fly via Docker Go API)

### Requirements Clarifications (User's Answers)

| Aspect | Decision |
|--------|----------|
| **Peer discovery** | mDNS |
| **Error recovery & persistence** | Execution reports back in real-time; source times out if no progress → job errored; error broadcast to network (quorum sync); abrupt client disappearance → job considered lost/errored |
| **Security/authentication** | Include auth protocol as TODO (dummy for now) |
| **Large artifact transfer** | Streaming |
| **Job timeouts** | Handled by real-time progress + timeout (no progress for N time → error) |
| **Cache garbage collection** | Retention policies: size limits + LRU + age-based eviction |
| **Job cancellation** | Any client in network can cancel; if dependent jobs exist, whole DAG cancelled |
| **Protocol versioning** | Google Protobuf backward compat + semantic versioning (major version compatibility) |
| **Logging** | slog with component IDs; levels: error/warning/info/debug/trace; queryable via API (remote tail capable) |
| **Fallback execution** | No fallback; source client always in scheduler pool |

### Key Insights from Conversation

1. **Deterministic hashing is critical:** Jobs hashed by `(type + runtime + inputs)` to ensure reproducibility and cache hits across network
2. **Real-time progress tracking prevents failures:** No need for explicit heartbeats; missing progress = timeout = error
3. **Network consensus (quorum) prevents race conditions:** Two same jobs won't be scheduled to same client simultaneously
4. **Streaming is fundamental design:** Input can be chunked and fetched from multiple peers in parallel
5. **Generic abstractions enable extensibility:** Single JobData, single Job with oneof types, applies to all languages/domains
6. **Logging queryable across network:** Essential for debugging multi-peer issues; users should be able to tail other clients' logs via CLI

---

## Product Overview

**Buildozer** is a **peer-to-peer distributed build system** with no central server. Developers use driver CLIs (gcc, g++, make) that mirror standard build tools, and nodes automatically discover each other via mDNS without manual network setup. The system intelligently caches, schedules, and distributes compilation jobs across a network, enabling faster builds through parallelization and result reuse.

### Three Core Components

1. **Driver** — CLI tool mimicking standard build tools (gcc, g++, gnu make). Prepares jobs, submits to network, collects results.
2. **Client** — Daemon running on user machines. Accepts job submissions, coordinates with peers, manages job queue, executes runtimes.
3. **Runtime** — Execution environment where jobs run. Defined by toolchain (language, compiler, version, architecture, C runtime). Can be native or Docker-based.

---

## TL;DR

Implement a **generic, language-agnostic P2P job distribution system** with:
- **Architecture:** Driver → Client (daemon) → Runtime (execution)
- **Job model:** Abstract input/output with deterministic hashing by (type + runtime + inputs)
- **Caching:** Content-addressed artifact store, queryable across network
- **Scheduling:** Quorum-based, load-aware, favors local execution (configurable)
- **Networking:** gRPC with mDNS discovery, real-time progress reporting
- **Observability:** Web UI, REST API, slog-based logging with remote tail capability
- **C/C++ implementation:** gcc, g++, make drivers + compile/link jobs
- **Resilience:** Timeout-based failure detection, job cancellation with DAG support

**Tech Stack:** Go, Protocol Buffers (via [buf](https://buf.build/)), [Connect](https://connectrpc.com/) (RPC protocol), mDNS (grandcat/zeroconf), Docker API, slog

**Protocol & RPC Tooling:**
- **[buf](https://buf.build/):** Protocol Buffer linting, code generation, and breaking change detection
- **[Connect](https://connectrpc.com/):** Protocol for RPC communication (gRPC-compatible but with REST/WebSocket support and simpler streaming)
- **Protobuf best practices:** enforced via buf lint (STANDARD rule set)

**Timeline:** ~26 weeks (6 months) across 7 phases

---

## Architecture

### Design Principles

1. **Generic job abstraction** — Single Job model with oneof for different job types; extensible to any language/domain
2. **Content-addressed caching** — Jobs hashed by (type + runtime + inputs); outputs tagged with runtime that generated them
3. **Decentralized** — No central coordinator; peers communicate via [Connect](https://connectrpc.com/) (RPC protocol); scheduling uses quorum consensus
4. **Real-time progress** — Execution client streams progress back to source; timeout-based failure detection
5. **Protocol-first** — All communication via [Connect](https://connectrpc.com/); Protobuf as contract; linted via [buf](https://buf.build/) for best practices; backward compatible via semver versioning
6. **Distributed data transfer** — Inputs/outputs streamed; multiple clients can send input parts in parallel
7. **Observability first** — Comprehensive slog-based logging (all levels: error/warning/info/debug/trace) with remote queryability

### Protocol Organization

The buildozer protocol is organized into **four logically separate APIs**, all sharing the same protocol version (`buildozer.proto.v1`):

1. **Driver API** (`buildozer/proto/v1/driver/`) - For external tool integration
   - **Service:** `JobService`
   - **Used by:** gcc, g++, make wrappers and external job submission tools
   - **RPCs:** SubmitJob, GetJobStatus, WatchJobStatus, CancelJob
   - **Purpose:** Submit jobs and track progress from driver CLIs

2. **Introspection API** (`buildozer/proto/v1/introspection/`) - For querying and observability
   - **Service:** `IntrospectionService`
   - **Used by:** CLI tools, web dashboards, monitoring systems
   - **RPCs:** GetClientStatus, ListPeers, GetPeerInfo, QueryLogs, GetCacheStatus, ListCachedArtifacts, GetJobQueue, GetJobHistory
   - **Purpose:** Query client state, peer info, logs, cache, and job history

3. **Peer APIs** (`buildozer/proto/v1/peer/`) - For peer-to-peer coordination
   - **ExecutorService:** ExecuteJob, FetchArtifact (job execution and artifact transfer)
   - **DiscoveryService:** AnnounceSelf, QueryCapabilities (peer discovery and capability advertisement)
   - **CoordinationService:** QueryCache, BroadcastCacheAnnouncement, BroadcastError, ProposeSchedule, CommitSchedule (distributed scheduling, caching, error sync)
   - **Used by:** Buildozer client daemons communicating with each other
   - **Purpose:** Execute jobs remotely, distribute artifacts, discover peers, coordinate scheduling

4. **Common Types** (`buildozer/proto/v1/common/`) - Shared by all APIs
   - **Files:** vocabulary.proto, auth.proto, job.proto, job_data.proto, runtime.proto, network_messages.proto
   - **Content:** TimeUnit, Hash, Signature, Size, ApiProtocol, Job, Runtime, JobData, etc.
   - **Used by:** All three APIs above

**Benefits of this separation:**
- **Clarity:** Each API's purpose is immediately visible from the directory structure
- **Scalability:** New APIs can be added (e.g., admin API, metrics API) without mixing concerns
- **Independent evolution:** Different APIs can be enhanced without entangling logic
- **Type safety:** Common types are clearly separated from service-specific request/response types
- **Documentation:** API docs can clearly explain which API is for what purpose

**Shared versioning:** All APIs are versioned together as `buildozer.proto.v1`. Protocol changes are coordinated across all APIs, and they advance versions together (v1 → v2).

### Layered Architecture (4 Layers)

```
┌──────────────────────────────────────────────────────────┐
│ Layer 4: Client Interfaces (Drivers, REST API, Web UI)   │
│ - gcc, g++, make drivers (mimic standard tools)          │
│ - REST API Gateway (status, logging, management)         │
│ - Web UI Dashboard                                       │
└──────────────────────────────────────────────────────────┘
                         ↑
┌──────────────────────────────────────────────────────────┐
│ Layer 3: Client Orchestration & Scheduling (Daemon)      │
│ - Job queue management (source client)                   │
│ - Quorum-based scheduling                                │
│ - Runtime capability matching                            │
│ - Job dependency resolution (DAG)                        │
│ - Job cancellation logic                                 │
│ - Timeout & error detection                              │
└──────────────────────────────────────────────────────────┘
                         ↑
┌──────────────────────────────────────────────────────────┐
│ Layer 2: Networking & Execution (Peer-to-Peer)           │
│ - gRPC services (job execution, job status)              │
│ - Real-time progress streaming                           │
│ - Artifact distribution (streaming)                      │
│ - mDNS discovery (peer announcement)                     │
│ - Peer health checks                                     │
│ - Runtime executor (native or containerized)             │
│ - Cache manager (local + distributed lookups)            │
└──────────────────────────────────────────────────────────┘
                         ↑
┌──────────────────────────────────────────────────────────┐
│ Layer 1: Job & Data Model (Protocol + Storage)           │
│ - Generic Job message with oneof for job types           │
│ - JobData abstraction (input/output) with streaming      │
│ - RuntimeRecipe model (e.g., Dockerfile)                 │
│ - Client capability advertisement                        │
│ - Local persistent storage (job metadata, cache index)   │
│ - slog-based structured logging                          │
└──────────────────────────────────────────────────────────┘
```

### Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| **Generic Job model** | Supports C/C++, future languages without architectural change |
| **Content-addressed cache** | Deterministic hash (type + runtime + inputs) ensures reproducibility & hits across network |
| **Streaming JobData** | Supports large artifacts; multiple clients can send parts in parallel |
| **Quorum scheduling** | Prevents race conditions (two same jobs not scheduled simultaneously) |
| **Real-time progress** | Source client sees live execution; timeout → error → network broadcast |
| **mDNS discovery** | Zero-config networking; works on local networks without DNS setup |
| **Semver protocol versioning** | Major version compatibility ensures rolling upgrades; Protobuf backward compat handles field additions |
| **slog logging** | Structured, queryable logs with levels; remote tail via API for debugging multi-node issues |
| **[buf](https://buf.build/) for proto management** | Unified linting, code generation, and breaking change detection; enforces STANDARD rule set for Protobuf best practices |
| **[Connect](https://connectrpc.com/) for RPC** | Supports gRPC, REST, and WebSocket with simpler streaming semantics and better web compatibility than gRPC |
| **Pure oneof pattern** | No redundant type enums; message types discriminated by oneof field alone for cleaner protocol |
| **Vocabulary layer** | Reusable types (TimeStamp, TimeDuration, Hash, Size, ResourceUsage, etc.) across all protocol messages |

### Component Logger Embedding Pattern

**Pattern:** All components in Buildozer follow a consistent logging pattern using unnamed embedded loggers for cleaner code and automatic contextual tracking.

**Rationale:**
- **Reduced boilerplate:** Method promotion via unnamed embedding eliminates `c.logger.Method()` prefixes
- **Automatic hierarchy:** Each component's logger is a child of its parent, creating natural hierarchical context visible in logs
- **Consistent error handling:** All errors logged and returned uniformly via `Errorf()` method
- **Remote observability:** Hierarchical structure enables filtering logs by component namespace (e.g., `daemon.httpServer.*`)
- **Easy testing:** Components with embedded loggers easier to mock and verify logging behavior

**Implementation Pattern:**
```go
type MyComponent struct {
    *logging.Logger  // unnamed embedded field
    config ConfigType
    // ... other fields ...
}

func NewMyComponent(cfg ConfigType) *MyComponent {
    return &MyComponent{
        Logger: daemon.Log().Child("MyComponent"),  // parent is daemon logger
        config: cfg,
    }
}

func (c *MyComponent) Operation() error {
    c.Debug("starting operation", "param", value)
    // ... do work ...
    if err != nil {
        return c.Errorf("operation failed: %w", err)  // logs AND returns
    }
    c.Debug("operation completed")
    return nil
}
```

**Logging Hierarchy Example:**
```
root
├── daemon
│   ├── httpServer
│   ├── scheduler
│   └── jobExecutor
└── client
    ├── cacheManager
    └── peerDiscovery
```

**Component Responsibilities:**
- **Log entry and exit:** Use `Debug()` at operation start with relevant parameters
- **Log errors:** Use `Errorf()` for all error paths; this both logs and returns the error
- **Log completion:** Use `Debug()` or `Info()` on successful completion with results
- **Structured attributes:** Include relevant context as key-value pairs: `c.Debug("message", "key1", val1, "key2", val2, ...)`

**All components implement this pattern, including:**
- Daemon HTTP server and services
- CLI command handlers
- Job executors and schedulers
- Cache managers
- Peer discovery and coordination
- Runtime detectors and executors

---

## Implementation Plan

### Phase 1: Core Abstractions & Local Foundation (Weeks 1-5)

**Goal:** Build the foundational runtime system, job abstractions, and local persistence layer. Can discover runtimes (native or Docker), execute jobs locally, and cache results.

**Deliverable:** A buildozer-client daemon that can:
- Discover available runtimes (native C/C++ toolchains and Docker-based runtimes)
- Execute C++ compile jobs on either native or Docker-based runtimes
- Cache output locally
- Query cache status

#### Milestone 1.0: Runtime Foundation Layer (Week 1)

**What - Core Runtime Abstractions:**
- `pkg/runtime/runtime.go` — Runtime interface (abstract)
  - `Execute(command, args, workdir, env) → (stdout, stderr, exitcode)` — Execute within runtime
  - `Available() → bool` — Is this runtime available/functional?
  - `Metadata() → RuntimeMetadata` — Toolchains, versions, architectures, etc.
  - `RuntimeID() → string` — Unique identifier (e.g., "gcc-11-x86_64-glibc-2.35")
  
- `pkg/runtime/discovery.go` — Runtime discovery interface
  - `DiscoverRuntimes() → []Runtime` — Find all available runtimes
  - `FindRuntime(spec RuntimeSpec) → Runtime, error` — Match job to compatible runtime
  
- `pkg/runtime/registry.go` — Runtime registry
  - Load discovered runtimes into registry
  - Query available runtimes
  - Match job specs to available runtimes

**What - Docker API Foundation:**
- `pkg/docker/docker.go` — Docker API abstraction wrapper (using official Docker Go API)
  - **Dependency:** Use official Docker Go API client: `github.com/docker/docker/client`
  - **Not custom:** Do NOT implement custom Docker client; use Docker's official SDK
  - Implement wrapper interface for common operations:
    - `PullImage(imageName) → imageID, error` — Pull Docker image from registry
    - `StartContainer(imageName, command, mounts, env) → Container` — Start container (keep running for long operations)
    - `ExecInContainer(containerID, command, args, workdir, env) → (stdout, stderr, exitcode)` — Execute command in running container
    - `StopContainer(containerID) → error` — Stop and cleanup container
    - `BuildImage(dockerfile, buildContext, imageName) → imageID, error` — Build image from Dockerfile (used for on-demand runtime builds)
    - `ImageExists(imageName) → bool` — Check if image exists locally
    - Handle Docker API errors: connection failures, missing images, permission issues
  - **Why wrapper:** Abstract Docker specifics; easier to test and mock
  
- `pkg/docker/runtime.go` — Docker-based runtime (implements Runtime interface)
  - Wraps Docker API client for runtime abstraction
  - Manages container lifecycle: Start container once, reuse for multiple `Execute()` calls, cleanup on shutdown
  - Handles volume mounting: Map source/output directories into container
  - Handles working directory and environment setup
  - Exposes same `Execute()` interface as native runtimes
  - Delegates actual Docker operations to `docker.go` client wrapper

**Verification:**
- Test Docker API client integration: Pull image, start container, exec command, stop
- Test native runtime execution: Execute command, capture output
- Test Docker container startup, command execution, cleanup
- Test container reuse: Multiple `Execute()` calls on same running container
- Test runtime registry: Add N native runtimes, M Docker runtimes, query all
- Integration: Request "C++/gcc/11" → Registry returns compatible native or Docker runtime
- Performance: Container startup <2s, command execution similar to native
- Error handling: Missing toolchain, Docker unavailable, image pull failure → proper errors

**Acceptance:** Runtime abstraction works (native + Docker); Docker API client integration solid; discovery and registry functional; container lifecycle management correct.

**Effort:** 1.5 weeks (~60 hours) — Medium-high complexity (Docker API client usage, process management, container lifecycle, image handling)

---

#### Milestone 1.1: Native C/C++ Runtime Implementation (Week 2)

**What - Toolchain Discovery & Execution:**
- `pkg/runtimes/cpp/native/detector.go` — Detect native C/C++ toolchains
  - Find gcc, g++, clang, clang++ in PATH
  - Run version detection: `gcc --version`, `-dumpversion`
  - Detect C runtimes: glibc, musl (ldd analysis)
  - Detect architectures: x86_64, aarch64, armv7, etc.
  - Generate `RuntimeID` for each combination
  - Return `[]Runtime` to registry
  
- `pkg/runtimes/cpp/native/executor.go` — Execute C/C++ on native
  - Compile: `gcc -c source.cpp -o output.o <flags>`
  - Link: `gcc -o binary main.o aux.o <flags>`
  - Subprocess management, environment setup
  - Implements `Runtime.Execute()` interface
  
- `pkg/runtimes/cpp/native/native_runtime.go` — Native C/C++ runtime
  - Wraps detector + executor
  - Implements `Runtime` interface
  - Metadata: gcc version, architecture, C runtime, etc.

**Verification:**
- Detect gcc, g++, clang on system → Correct versions and architectures
- Compile simple C++ file → Object file created with correct hash
- Link objects → Executable created
- Output matches native compilation (deterministic)
- Multiple invocations of same command → Same hash (reproducible)

**Acceptance:** Native toolchain detection accurate; compilation/linking deterministic; executor reliable.

**Effort:** 1 week (~40 hours) — Medium complexity (toolchain detection, subprocess management)

---

#### Milestone 1.2: Docker-Based C/C++ Runtime Implementation (Week 2-3)

**What - Predefined Docker Images with Comprehensive Toolchain Tagging:**
- `pkg/runtimes/cpp/docker/dockerfiles/` — **Embedded Dockerfile templates** (in codebase via Go `embed` package)
  - Dockerfile templates for all predefined toolchain combinations
  - Templates embedded in binary so no external files needed
  - Examples of predefined templates:
    - `ubuntu-gcc-11-glibc-2.35.Dockerfile` — ubuntu:22.04 + gcc-11 + g++-11 + glibc-2.35
    - `ubuntu-gcc-12-glibc-2.36.Dockerfile` — ubuntu:22.04 + gcc-12 + g++-12 + glibc-2.36
    - `ubuntu-clang-14-glibc-2.35.Dockerfile` — ubuntu:22.04 + clang-14 + clang++-14 + glibc-2.35
    - `alpine-gcc-11-musl-1.2.3.Dockerfile` — alpine:latest + gcc-11 + g++-11 + musl-1.2.3
    - `ubuntu-gcc-11-glibc-2.35-aarch64.Dockerfile` — ubuntu:22.04 + gcc-11 + g++-11 + glibc-2.35 + aarch64
    - Similar variants with different compiler versions, C runtimes, architectures
  
- `pkg/runtimes/cpp/docker/dockerfile_builder.go` — **On-demand image building**
  - Interface `DockerfileProvider` — Manage Dockerfile templates (embedded)
  - `GetDockerfile(spec RuntimeSpec) → string` — Load embedded Dockerfile for runtime spec
  - `BuildImage(spec RuntimeSpec) → imageID, error` — Build image on demand
    - Check if image already exists locally (tagged with canonical name)
    - If missing: Load template → Build via Docker API → Tag → Cache tags in metadata
    - If exists: Return cached image ID (fast path)
  - **On-demand strategy:** Build image when runtime first requested, reuse for all jobs requiring that runtime
  - **Local image caching:** Once built, image stays in Docker daemon; detector finds via tag scanning
  - **Zero external dependencies:** Dockerfiles embedded; no need for users to provide Dockerfile files
  
- **Comprehensive Docker Image Tagging Strategy with Canonical Compiler Names:**
  - Docker image tags **must include full toolchain specification** for reproducibility
  - **Key insight:** Use canonical compiler name in tag (gcc, clang), not language-specific drivers (g++, clang++)
    - Rationale: gcc/g++ are same compiler; clang/clang++ are same compiler; language specified separately
  - Tag format: `buildozer-<language>-<compiler>-<compiler_version>-<architecture>-<cruntime>-<cruntime_version>`
  - Full metadata examples:
    - `buildozer-c-gcc-11-x86_64-glibc-2.35` (C jobs using gcc-11, glibc 2.35, x86_64)
    - `buildozer-cxx-gcc-11-x86_64-glibc-2.35` (C++ jobs using gcc-11, same base image)
    - `buildozer-c-gcc-12-aarch64-glibc-2.36` (C jobs with gcc-12, glibc 2.36, aarch64)
    - `buildozer-cxx-clang-14-x86_64-glibc-2.35` (C++ jobs with clang-14, glibc)
    - `buildozer-c-gcc-11-x86_64-musl-1.2.3` (C jobs with gcc-11, musl-1.2.3)
  
  - Smart image reuse: Single base image registered under TWO tags (C and C++)
    - Base image: ubuntu:22.04 + gcc-11 + g++-11 + glibc-2.35 + x86_64
      - Tag 1: `buildozer-c-gcc-11-x86_64-glibc-2.35` (for C compilation jobs → uses gcc driver)
      - Tag 2: `buildozer-cxx-gcc-11-x86_64-glibc-2.35` (for C++ compilation jobs → uses g++ driver)
      - Both tags point to same Dockerfile/image (efficient storage, zero image duplication)
  
- `pkg/runtimes/cpp/docker/docker_cpp_runtime.go` — Docker-based C/C++ runtime (implements Runtime interface)
  - Encapsulates single Docker image, but registers as separate runtime for C and C++
  - **Key insight:** Reuses native executor logic inside container via `docker exec`
  - At startup: Mount source, working dir as volumes; select appropriate driver (gcc/g++/clang/clang++)
  - **Driver selection happens at execution based on job language:**
    - C job + gcc compiler → Execute with: `docker exec container gcc -c source.c ...`
    - C++ job + gcc compiler → Execute with: `docker exec container g++ -c source.cpp ...`
    - Both use same underlying compiler, different driver entry point
  - Same output as native execution (because same toolchain, mounted files)
  - **Runtime metadata:** Fully qualified with all components:
    - Language: c or cxx
    - Compiler: gcc or clang (canonical name, not g++ or clang++)
    - Compiler version: 11, 12, 14, etc.
    - Target architecture: x86_64, aarch64, armv7, etc.
    - C runtime: glibc or musl
    - C runtime version: 2.35, 2.36, 1.2.3, etc.
  
- `pkg/runtimes/cpp/docker/detector.go` — Discover and auto-build available Docker runtimes
  - **Discovery phase:** 
    - Scan local Docker images for `buildozer-*` pattern
    - Parse image tags to extract full toolchain metadata
  - **Auto-build phase:**
    - For each known runtime spec (from embedded Dockerfile list), check if image exists
    - If missing: Call `dockerfile_builder.BuildImage()` to build on-demand
    - Once built: Tag appropriately, register both C and C++ variants
  - **Parse image tag to extract full toolchain metadata:**
    - Extract language, compiler, compiler version, architecture, C runtime, C runtime version
    - Example tag: `buildozer-cxx-gcc-11-aarch64-glibc-2.36`
      → Language: cxx, Compiler: gcc, Version: 11, Arch: aarch64, CRuntime: glibc, CRuntimeVer: 2.36
  - Register single image under multiple tags (C and C++ variants)
  - **Implement smart matching:** "Job needs C, gcc, ver=11, x86_64, glibc, ver=2.35" → finds or builds `buildozer-c-gcc-11-x86_64-glibc-2.35`
  - **Implement smart matching:** "Job needs C++, gcc, ver=11, x86_64, glibc, ver=2.35" → finds or builds `buildozer-cxx-gcc-11-x86_64-glibc-2.35` (same image)
  - At runtime: Job language determines which driver (gcc vs g++) to invoke
  - **Caching metadata:** Track which images are built, when, and their base Dockerfile for rebuilds if needed

**Architecture - Smart Image Reuse with Canonical Compiler Names:**
```
Physical Docker Image: ubuntu:22.04 + gcc-11 + g++-11 + glibc-2.35 + x86_64 (built once)
                         ↓
            Two Tag Registrations (same image, different tags)
            ↙                                          ↘
buildozer-c-gcc-11-x86_64-glibc-2.35        buildozer-cxx-gcc-11-x86_64-glibc-2.35
    ↓                                             ↓
C Runtime Metadata:                          C++ Runtime Metadata:
- Language: c                                - Language: cxx
- Compiler: gcc (canonical)                  - Compiler: gcc (canonical)
- Compiler Version: 11                       - Compiler Version: 11
- Target Architecture: x86_64                - Target Architecture: x86_64
- C Runtime: glibc                           - C Runtime: glibc
- C Runtime Version: 2.35                    - C Runtime Version: 2.35
- Driver at execution: gcc                   - Driver at execution: g++
- Same underlying image                      - Same underlying image
```

**Verification:**
- Embedded Dockerfiles can be read from binary (no external files needed)
- `dockerfile_builder.GetDockerfile(spec)` returns embedded Dockerfile content
- **On-demand build workflow:**
  - Request runtime: buildozer-c-gcc-11-x86_64-glibc-2.35 (not yet built)
  - Detector checks Docker daemon → image not found
  - Calls `BuildImage()` → Loads embedded Dockerfile → Builds image → Tags correctly
  - Registers as TWO runtimes (C and C++) with metadata
  - Second request for same runtime → Fast path (image already exists)
- Image discovered and registered as TWO separate runtimes with full metadata
  - Both runtimes share same underlying image (verified by Docker; no duplication)
- C job(language=c, compiler=gcc, ver=11, arch=x86_64, cruntime=glibc, ver=2.35) → Triggers build if needed, then executes
- Docker executor invokes: `docker exec <image> gcc -c ...` (driver selected by job language)
- C++ job(language=cxx, same compiler/version/arch/cruntime) → Finds already-built image, executes
- Docker executor invokes: `docker exec <image> g++ -c ...` (driver selected by job language)
- C job hash matches native gcc-11 compilation
- C++ job hash matches native g++-11 compilation (same underlying gcc-11, different driver)
- Container lifecycle: Start, mount volumes, execute with language-selected driver, cleanup
- Verify driver selection logic: Language field determines gcc vs g++ (or clang vs clang++)
- **Binary portability:** Deploy buildozer binary to any system; embedded Dockerfiles build required runtimes on first use (no external Dockerfile files needed)

**Acceptance:** Docker runtimes with embedded Dockerfile templates provide comprehensive toolchain coverage; on-demand image building simplifies deployment (no external files); C and C++ separated by language tag; compiler name canonical (gcc, clang) not driver-specific (g++, clang++); full metadata enables precise matching; single image efficiently provides both C and C++ runtimes; output identical to native; no duplication; reproducible builds across architectures and C runtime versions; **binary portable** (embedded Dockerfiles → builds runtimes on first use).

**Effort:** 1.5 weeks (~60 hours) — Medium complexity (embedded Dockerfile management, on-demand Docker builds, image caching, comprehensive metadata tagging, driver selection logic)

---

#### Milestone 1.3: Job, Runtime Matching, and Submission Pipeline Abstractions (Week 3)

**What (now that runtimes are proven):**
- `pkg/job/job.go` — Job interface, hash computation (reuses `Runtime.Execute()` to compute output)
- `pkg/job/cpp_compile.go` — CppCompileJob 
  - Specifies: source file, output file, flags, **required runtime spec**
  - Hash = SHA256(job_type + runtime_spec + input_hashes)
  - Uses runtime registry to find compatible `Runtime`
  
- `pkg/runtime/matcher.go` — Job-to-runtime matching (now using proven runtime system)
  - Given job spec → Find compatible runtimes from registry
  - Match C++ version, architecture, C runtime
  
**NEW - Submission Pipeline Abstractions (critical for extensibility to Phase 4):**
- `pkg/queuer/queuer.go` — Interface for job queueing (local to client)
  - `Enqueue(job) → error` — Accept or reject job submission (e.g., queue full)
  - `Dequeue() → Job` — Get next pending job
  - Query queue state (size, oldest job, etc.)
- `pkg/scheduler/scheduler.go` — Interface for job scheduling (can have multiple implementations)
  - `Schedule(job) → ExecutionTarget` — Where should this job execute? (local client ID or peer address)
  - `IsCompatibleRuntimeAvailable(job) → bool` — Does target have compatible runtime?
  - `CanExecuteNow(job) → bool` — Is runtime busy? Ready to execute?
  - Phase 1 impl: Always return local client; Phase 4 impl: Query P2P network, quorum voting
- `pkg/progress/progress.go` — Interface for progress monitoring
  - `Subscribe(jobID) → chan JobProgress` — Stream progress updates
  - `ReportProgress(jobID, progress)` — Execution updates self or remote client
  - Phase 1 impl: Local in-memory updates; Phase 3 impl: Network broadcast
- `pkg/scheduler/local/local_scheduler.go` — Phase 1 implementation: Always execute on this client

**Verification:**
- Create compile job requiring gcc-11 x86_64 glibc
- Query registry for compatible runtimes → returns [gcc-native, gcc-docker]
- Execute on native → output cached
- Execute on Docker → output matches native hash
- Submission pipeline: driver → queue → schedule → execute on compatible runtime → cache → progress

**Acceptance:** Job abstractions work with proven runtime system; job-to-runtime matching reliable; execution deterministic across native/Docker.

**Effort:** 1 week (~40 hours) — Medium complexity (job abstractions, runtime matching)

---

#### Milestone 1.4: Structured Logging Setup (Week 3-4)

**What:**
- `pkg/logging/logger.go` — Component-scoped logger factory using slog
- Five levels: Error, Warn, Info, Debug, Trace
- Structured fields: component, request_id, job_id, client_id, timestamp
- File rotation and retention policies

**Verification:**
- All log levels emitted correctly
- Structured fields visible in output
- Log file rotates at size limit
- No memory leaks from loggers

**Acceptance:** All log levels work; structured output machine-parseable (JSON); file rotation tested.

**Effort:** 4 days (~32 hours) — Low complexity (using Go slog stdlib)

---

#### Milestone 1.5: Persistent Store (Week 4)

**What:**
- `pkg/store/store.go` — Interface for local metadata persistence
- `pkg/store/boltdb/boltdb_store.go` — BoltDB implementation (embedded B+ tree)
- Data models: Client metadata, job metadata, cache index, peer directory
- CRUD operations: Store/retrieve job by hash, update status, query cache

**Verification:**
- Integration tests: Write job metadata, restart daemon, re-read metadata (persist ✓)
- Concurrent access: Multiple goroutines write/read simultaneously (race condition free ✓)
- Performance: Lookup cache entry <1ms

**Acceptance:** Persistence tests pass; concurrent access deadlock-free; 1000s of cache entries retrievable <100ms.

**Effort:** 1 week (~40 hours) — Medium complexity (database indices, schema versioning)

---

#### Milestone 1.6: Cache Manager (Week 4)

**What:**
- `pkg/cache/cache.go` — Cache interface
- `pkg/cache/local/local_cache.go` — Local disk-based artifact cache
  - Directory structure: `cache/<hash>/<artifact_name>`
  - Metadata file: hash, size, timestamp, runtime_tag
  - Operations: Store artifact, retrieve, delete
  - Retention: Size limits + LRU + age-based eviction
- Hash verification: Verify retrieved artifact matches stored hash

**Verification:**
- Store artifact for job hash X
- Retrieve artifact for job hash X → same bytes
- Query non-existent hash → nil
- Cache eviction: Add 1000 small artifacts exceeding size limit → LRU eviction works

**Acceptance:** Cache CRUD works; eviction strategies tested; hash verification correct.

**Effort:** 1.5 weeks (~60 hours) — Medium complexity (eviction algorithms, file management)

---

#### Milestone 1.7: Job Executor (Refactored for Runtime System) (Week 4-5)

**What - Now using the proven runtime system:**
- `pkg/executor/executor.go` — Executor interface
- `pkg/executor/executor_impl.go` — Implementation using runtime registry
  - Validate job inputs present
  - Query runtime registry for compatible runtime based on job spec
  - Invoke `runtime.Execute()` command via discovered runtime (native or Docker)
  - Capture stdout/stderr/exit code
  - Handle resource limits (CPU, memory) - passed to runtime

**Key Change:** Executor now delegates to discovered `Runtime` instances instead of hardcoding subprocess logic. This supports both native and Docker-based runtimes seamlessly.

**Verification:**
- Execute C++ compile job on native runtime → object file created
- Execute same job on Docker runtime → identical output hash
- Stderr captured on compile error
- Output file hash computed correctly across native and Docker execution
- Resource limits enforced (CPU, memory)

**Acceptance:** Execution works on both native and Docker runtimes; output deterministic; resource limits functional.

**Effort:** 1.5 weeks (~60 hours) — Medium complexity (runtime invocation, error handling)

---

#### Milestone 1.8: Local Client Daemon (Week 5)

**What:**
- `cmd/buildozer-client/main.go` — Daemon entry point
- `pkg/client/client.go` — Client coordinator orchestrating job submission pipeline
  - Initialize: logging, store, executor, cache, queuer, local scheduler, progress tracker
  - Initialize runtime discovery & registry (from Phase 1.0-1.2)
  - Accept job submission via Connect RPC → `JobService.SubmitJob(job)`
  - **Submission Pipeline:**
    1. **Queueing:** `queuer.Enqueue(job)` — Accept or reject (queue full?)
    2. **Scheduling:** `scheduler.Schedule(job)` → ExecutionTarget (Phase 1: always local)
    3. **Check Runtime:** `scheduler.IsCompatibleRuntimeAvailable(target, job)` → error if no match
    4. **Wait for Capacity:** `scheduler.CanExecuteNow(target, job)` → may block if runtime busy
    5. **Execute:** Pass to executor (which uses discovered runtime)
    6. **Progress Reporting:** `progressMonitor.Subscribe(jobID)` streams updates back to caller
  - Submission RPC is **blocking**: Returns when job completes or error occurs
  - Driver receives `JobStatus` stream (progress updates) during execution

**Verification:**
- Start daemon without errors
- Submit local compile job via Connect RPC → RPC blocks until completion
- During execution, progress streamed (every 5s)
- On completion, client receives final result
- Query job status → shows history and progress events
- Query cache → shows cached artifact
- Second submission of same job → Queued immediately, scheduled, **cache hit** (no re-execution)
- Daemon handles queue full → Returns error, driver can retry
- Runtime registry working: Discover native + Docker runtimes, match jobs to compatible runtimes

**Acceptance:** Daemon starts/stops cleanly; submission pipeline works; queueing + scheduling + progress tracking integrated; cache hit on re-execution; runtime discovery functional.

**Effort:** 2 weeks (~80 hours) — Medium-high complexity (pipeline orchestration, Connect streaming, state management, runtime coordination)

**Note:** Scheduler interface is abstract; Phase 1 uses `LocalScheduler`. Phase 4 swaps in `P2PScheduler` without changing `Client` code.

---

#### Milestone 1.9: Minimal Test Drivers (Week 5)

**What:** Basic driver implementations for end-to-end testing (full drivers deferred to Phase 5)
- `cmd/buildozer-gcc/main.go` — Minimal gcc wrapper
  - Parse: input file, output file, basic flags
  - Create CppCompileJob proto
  - Submit via Connect RPC to local daemon (which discovers native/Docker runtimes)
  - Stream progress + write output file
  - **Limitation:** Only basic flags; no optimization flags, includes, etc. (added in Phase 5)
  - **Fallback:** If daemon unavailable, invoke native gcc
- `cmd/buildozer-g++/main.go` — Minimal g++ wrapper (same)
- `cmd/buildozer-make/main.go` — Minimal make wrapper (basic invocation, no recursive resolution)

**Verification:**
- `buildozer-gcc -c hello.cpp -o hello.o` → Compiles locally (on native runtime) → hello.o created
- `buildozer-gcc --version` → Returns gcc version (from underlying compiler)
- Daemon unavailable → Falls back to native gcc
- Multiple submissions of same job → Cache hit (no re-execution)
- Integration test: E2E pipeline from driver submission through runtime discovery to cached result

**Acceptance:** Minimal drivers work for testing; fallback mode functional; E2E tests pass; drivers work with discovered runtimes (native and Docker).

**Effort:** 1 week (~40 hours) — Low-medium complexity (basic argument parsing, simple fallback logic)

**Note:** These are deliberate **MVP drivers** for testing Phase 1 functionality. Full drivers with all flags, recursion, include path handling, etc. come in Phase 5.

---

**Phase 1 Exit Criteria:**
- ✅ Runtime Foundation: Docker API wrapper, native & Docker runtime implementations
- ✅ Job abstractions with deterministic hashing
- ✅ Abstract submission pipeline (Queuer, Scheduler, ProgressMonitor interfaces)
- ✅ Local persistence working (BoltDB)
- ✅ Logger integrated across codebase
- ✅ Local executor can compile C++ files on native or Docker runtimes
- ✅ Cache manager working (CRUD + eviction)
- ✅ Single-client daemon with full submission pipeline (queue → schedule → execute → cache)
- ✅ Minimal test drivers (gcc/g++/make) with fallback mode
- ✅ All unit + integration tests passing
- ✅ Reproducible builds: same job hash = same output (native and Docker identical)
- ✅ **E2E validation: Driver submission → queueing → scheduling → execution on discovered runtime → caching → progress streaming**

**Key Architectural Achievement:** 
- Runtime system proven before building execution abstractions
- Job submission pipeline is **abstract and extensible**
- Phase 1 uses local implementations; Phase 4 swaps scheduler to P2P network without changing client code
- Native and Docker executions produce identical outputs (code reuse via `docker exec`)

**Cumulative Time:** 5 weeks

---

### Phase 2: Networking & Discovery (Weeks 5-7)

**Goal:** Implement P2P networking, mDNS discovery, and peer communication.

#### Step 6: mDNS Discovery Implementation (Week 5)

Use `grandcat/zeroconf` for mDNS service registration/listening.
Each peer registers: `_buildozer._tcp` service with gRPC address, protocol version, capabilities.

**Verification:** Multi-peer setup; peers discover each other within 5 seconds.
**Dependencies:** Depends on Step 5.

#### Step 7: gRPC Services Implementation (Week 5-6)

Implement gRPC service endpoints:
- `SubmitJob()` — Validate job, store in client queue, trigger scheduling
- `ExecuteJob()` — Start executor, stream progress back
- `GetJobStatus()` — Lookup in store, return current status
- `QueryCache()` — Search local cache, query peers in parallel
- `Announce()` — Return current client capabilities

**Verification:** Tests for each service; mock peers; end-to-end job submission → execution.
**Dependencies:** Depends on Steps 3, 5.

#### Step 8: Real-Time Progress & Timeout Handling (Week 6-7)

Source client tracks `last_progress_time` per job.
Execution client streams progress every N seconds (configurable).
Timeout: if no progress for `job_timeout` (default 10min), source client marks job as **errored**.

**Verification:** Simulate timeout scenario; verify error broadcast; other peers see error.
**Dependencies:** Depends on Step 7.

---

### Phase 3: Job Execution & Caching (Weeks 8-12)

**Goal:** Implement job execution, artifact caching, and distributed data transfer.

#### Step 9: Job Executor (Week 8)

Create `pkg/executor/`:
- Local execution: subprocess with environment setup, capture stdout/stderr
- Container execution: Use Docker Go API to spin container from RuntimeRecipe image
- Handle resource limits: CPU cores, RAM

**Verification:** Execute C++ compile job; capture output; hash matches expected.
**Dependencies:** Depends on Step 2.

#### Step 10: Artifact Cache Manager (Week 8-9)

Create `pkg/cache/`:
- Local cache: Directory structure `cache/<hash>` with metadata + artifact data
- `Store(hash, data)` — Persist artifact locally
- `Get(hash)` → data or nil — Check local cache
- `QueryNetwork(hash)` — Ask peers if they have artifact via `QueryCache()` RPC
- `FetchRemote(hash, peer_addr)` → stream artifact chunks from peer
- Retention policies: Size limits, LRU, age-based

**Verification:** Store artifact, retrieve, verify hash; network query returns correct peer.
**Dependencies:** Depends on Steps 7, 9.

#### Step 11: Streaming Artifact Distribution (Week 9-10)

Add gRPC service: `ArtifactService.StreamArtifact(hash) → stream Data`

**Verification:** Stream large artifact successfully; parallel chunk reception works.
**Dependencies:** Depends on Steps 7, 10.

#### Step 12: Cache Coherence & Distributed Lookup (Week 10-11)

When job finishes, add to local cache and announce to network.
Global cache query: Source client queries all known peers for artifact hash.

**Verification:** Job on peer A, source client on peer B queries cache → finds artifact on peer A.
**Dependencies:** Depends on Steps 7, 10, 11.

#### Step 13: Input Preparation & Dependency Staging (Week 11-12)

Before execution: Resolve all input references → fetch from cache (local or network).
Parallel fetching: Multiple inputs fetched concurrently from different peers.

**Verification:** Job with 3 input files from 3 peers executes successfully.
**Dependencies:** Depends on Steps 10, 11.

---

### Phase 4: Job Orchestration & Scheduling (Weeks 13-16)

**Goal:** Implement job queuing, scheduling, and dependency resolution.

#### Step 14: Dependency Resolution & DAG (Week 13)

Create `pkg/graph/`:
- Extract dependencies: Analyze job inputs/outputs → build edges
- Detect cycles: Return error if DAG has cycles
- Topological sort: Compute execution order

**Verification:** Submit graph with dependencies; verify topological order correct; cycle detection works.
**Dependencies:** Depends on Steps 2, 3.

#### Step 15: Job Queue & Quorum Scheduling (Week 13-14)

Create `pkg/scheduler/`:
- Source client maintains job queue (pending + ready jobs)
- Quorum mechanism: Broadcast `ScheduleDecision` to all peers before scheduling
- Scoring: `(peer_load + job_cpu_cost) / peer_capacity` (least-load bin-packing)
- Affinity: Prefer source client (configurable weight)
- Job states: `Pending → Ready → Scheduled → Running → Completed/Failed`

**Verification:** Schedule jobs in correct order; quorum prevents double-booking; affinity favors local peer.
**Dependencies:** Depends on Steps 6, 14.

#### Step 16: Job Cancellation (Week 14-15)

Any client can cancel any job via `CancelJob(JobID)` RPC.
If job has dependent jobs: Cancel entire DAG (transitive closure).

**Verification:** Cancel queued job; cancel running job; verify cancellation propagates to dependent jobs.
**Dependencies:** Depends on Steps 7, 14, 15.

#### Step 17: Error Handling & Retries (Week 15-16)

Job error sources: Timeout, executor crash, network failure, resource exhaustion.
Retry policy: Exponential backoff (base 2s, max 5 attempts).

**Verification:** Simulate peer crash mid-execution; job retried; eventually completes or fails.
**Dependencies:** Depends on Steps 8, 15.

---

### Phase 5: Client & Driver Implementation (Weeks 17-20)

**Goal:** Implement daemon, drivers, and query API.

#### Step 17.5: Client CLI Architecture with Cobra & Viper (Week 17)

**What - Build buildozer-client CLI with cobra + viper for enterprise-grade configuration management:**

**Configuration Priority (Cobra + Viper Pattern):**
- **Priority 1 (Highest):** CLI flags (e.g., `--port 6789`)
- **Priority 2:** Environment variables (e.g., `BUILDOZER_DAEMON_PORT=6789`)
- **Priority 3:** Configuration file (e.g., `~/.config/buildozer/config.yaml`)
- **Priority 4 (Lowest):** Hardcoded defaults in code

Viper library automatically merges these sources in priority order. Configuration file is optional (won't error if missing). Environment variables are prefixed with `BUILDOZER_`.

**buildozer-client CLI Structure - Two Operating Modes:**

1. **Daemon Mode** — `buildozer-client daemon [FLAGS]`
   - Launches persistent client daemon
   - Registers on mDNS for peer discovery
   - Accepts job submissions via gRPC
   - Optionally `--standalone` flag: If set, runs daemon in-process within CLI (for dev/testing)
   
2. **Interactive Mode** — Commands to interact with running daemon via API
   - `buildozer-client status` — Query client status (load, capabilities)
   - `buildozer-client peers` — List connected peers
   - `buildozer-client logs [FLAGS]` — Tail client logs (local or remote)
   - `buildozer-client cache` — View cached artifacts and stats
   - `buildozer-client queue` — View current job queue
   - `buildozer-client config` — Show effective configuration (after merging all sources)
   - `buildozer-client cancel JOB_ID` — Cancel a running job

**Global Flags (apply to all commands):**
- `--config` — Config file path (default: `~/.config/buildozer/config.yaml`)
- `--standalone` — Run daemon in-process (only with `daemon` command)
- `--debug` — Enable debug logging
- `--log-level` — Set logging level: error/warn/info/debug/trace
- `--host` — Daemon address (default: localhost)
- `--port` — Daemon gRPC port (default: 6789)

**Configuration File Structure (YAML):**
```yaml
# ~/.config/buildozer/config.yaml
daemon:
  port: 6789
  listen: "0.0.0.0"
  max_concurrent_jobs: 4
  max_ram_mb: 8192

logging:
  level: "info"
  format: "text"  # or "json"

cache:
  dir: "~/.cache/buildozer"
  max_size_gb: 100
  retention_days: 30

peer_discovery:
  enabled: true
  mDNS_interval_seconds: 30
```

**Environment Variables (with prefix BUILDOZER_):**
- `BUILDOZER_DAEMON_PORT` → daemon.port
- `BUILDOZER_DAEMON_LISTEN` → daemon.listen
- `BUILDOZER_DAEMON_MAX_CONCURRENT_JOBS` → daemon.max_concurrent_jobs
- `BUILDOZER_LOGGING_LEVEL` → logging.level
- `BUILDOZER_CACHE_DIR` → cache.dir
- `BUILDOZER_DEBUG` → global --debug flag

**Implementation Details:**
- Use `spf13/cobra` for command framework
- Use `spf13/viper` for configuration management
- Bind cobra flags to viper: `viper.BindPFlag("daemon.port", cmd.Flags().Lookup("port"))`
- Auto-load config file if exists: `viper.ReadInConfig()` (non-fatal if missing)
- Set env prefix: `viper.SetEnvPrefix("BUILDOZER")`
- Enable auto-env: `viper.AutomaticEnv()`
- Read config from standard locations: `$HOME/.config/buildozer/config.yaml`, `/etc/buildozer/config.yaml`

**buildozer-client daemon flow:**
1. Parse cobra flags
2. Load config file (if exists) + env vars + CLI flags → merged config via viper
3. Initialize logger with effective log level
4. If `--standalone`: Create daemon in-process, skip port/host setup
5. If not standalone: Start gRPC server on (host, port) from config
6. Register on mDNS with capabilities
7. Main loop: Accept job submissions, trigger scheduling, stream progress
8. Graceful shutdown: Finish in-flight jobs, close connections

**buildozer-client status flow:**
1. Parse cobra flags
2. Load config (host, port)
3. Connect to daemon via gRPC
4. Call `IntrospectionService.GetClientStatus()`
5. Display results (load, queue size, peer count, cache stats)

**Verification:**
- CLI help: `buildozer-client help` shows commands + flags
- Daemon mode: `buildozer-client daemon` starts server on default port 6789
- Standalone mode: `buildozer-client daemon --standalone` runs in-process (no port binding)
- Config file usage: Create `~/.config/buildozer/config.yaml` with custom port → daemon starts on custom port
- Env var override: `BUILDOZER_DAEMON_PORT=7890 buildozer-client daemon` → overrides config file (runs on 7890)
- CLI flag override: `buildozer-client daemon --port 8890` → overrides both config and env vars (runs on 8890)
- Config command: `buildozer-client config` shows merged configuration from all sources
- Interactive commands: `buildozer-client status`, `buildozer-client peers`, etc. connect to daemon and display info
- Graceful shutdown: `Ctrl+C` or `buildozer-client cancel JOB_ID` works cleanly

**Acceptance:**
- All commands work with cobra + viper priority order
- Configuration file optional (daemon starts with defaults if missing)
- Env vars override config file, CLI flags override everything
- Daemon and interactive modes fully functional
- E2E: Start daemon with config file → Submit job via interactive command → Stream progress → Job completes
- Standalone flag enables in-process daemon for testing without port conflicts

**Effort:** 1 week (~40 hours) — Low-medium complexity (cobra/viper setup, command structure, config merging)

**Note:** Cobra + Viper is standard Go CLI pattern; viper handling the inheritance/override logic automatically once configured correctly.

---

#### Step 18: Client Daemon (Week 17)

Create `cmd/buildozer-client/main.go`, `pkg/client/`:
- Initialize: Log setup, config load, storage init, mDNS registration, gRPC server start
- Main loop: Accept job submissions, trigger scheduling, stream progress
- Configuration: Resource limits, local weight, protocol version, auth token (TODO)

**Verification:** Start daemon; register on mDNS; accept jobs; shut down cleanly.
**Dependencies:** Depends on Steps 5-17.

#### Step 19: Driver Implementation (Week 18)

Create `cmd/buildozer-gcc/`, `cmd/buildozer-g++/`, `cmd/buildozer-make/`:
- Parse command-line arguments → create Job
- Connect to local client daemon (RPC) → submit job
- Stream progress + results back to stdout/stderr

**Verification:** `buildozer-gcc --version`; compile job submission; output file created.
**Dependencies:** Depends on Step 18.

#### Step 20: REST API & Web UI (Weeks 19-20)

Create `cmd/buildozer-api/`, `cmd/dashboard/`, `web/`:
- REST API: HTTP wrapper around gRPC services
  - `GET /api/client/status` — Client capabilities, load, queue size
  - `GET /api/peers` — List all discovered peers
  - `GET /api/cache/stats` — Cache hit rate, size, eviction rate
  - `GET /api/logs?client_id=...&level=info&tail=100` — Remote log tail
  - `POST /api/jobs/{id}/cancel` — Cancel job
- Web UI: Real-time dashboard showing job queue, network topology, cache stats, live logs

**Verification:** Start daemon, access http://localhost:9090; see live status + logs.
**Dependencies:** Depends on Step 18.

---

### Phase 6: C/C++ Integration (Weeks 21-23)

**Goal:** Implement C/C++ runtime detection and compilation job execution.

#### Step 21: Toolchain Detection (Week 21)

Create `pkg/toolchain/cpp/`:
- `gcc_detector.go` — Detect gcc version, path, supported architectures
- `gxx_detector.go` — Detect g++ version, path
- Detect C runtimes (glibc, musl, etc.) installed on system
- Client announces supported runtimes on registration

**Verification:** Run detector; identify all toolchains on system.
**Dependencies:** Depends on Step 2.

#### Step 22: C/C++ Compile Job Executor (Week 22)

Create `pkg/executor/cpp/compile_executor.go`:
- Job: source file, compiler flags, include paths, runtime spec
- Executor: Invoke compiler with args; capture object file output

**Verification:** Compile source to object file; verify hash; re-run hits cache.
**Dependencies:** Depends on Steps 9, 21.

#### Step 23: C/C++ Link Job Executor (Week 23)

Create `pkg/executor/cpp/link_executor.go`:
- Job: object files, linker flags, output file, runtime spec
- Executor: Invoke linker (ld/g++/gcc -o); handle static/dynamic/shared

**Verification:** Link object files → executable; hash correct; execute successfully.
**Dependencies:** Depends on Steps 9, 21.

---

### Phase 7: Testing & Production Hardening (Weeks 24-26)

**Goal:** Comprehensive testing, security, and production readiness.

#### Step 24: Unit & Integration Tests (Week 24)

- Unit tests: Job hashing, scheduling, cache operations (>75% coverage)
- Integration tests: Multi-peer setup, job execution, cache hit, scheduling
- Docker Compose: 3-peer network, submit 10+ jobs, verify correct execution

**Verification:** `go test ./...` passes; coverage report.
**Dependencies:** Depends on Phases 1-6.

#### Step 25: Security & Authentication (Week 25)

Create `pkg/auth/`:
- Implement dummy auth as placeholder (TODO per user spec)
- Client sends `auth_token` in `RequestMetadata`; server validates (always passes for now)

**Verification:** Auth token validation; error on invalid token (stubbed).
**Dependencies:** Depends on Phase 1.

#### Step 26: Documentation & Deployment (Week 26)

Create `docs/`, update `Makefile`, `docker-compose.yml`:
- Architecture doc: Explain 4-layer model, job execution flow, caching strategy
- User guide: Install driver, run daemon, submit jobs
- API reference: gRPC services, Proto definitions
- Deployment: Docker images, Kubernetes manifests (future)

**Verification:** Docs build; deployment tested; CI passes.
**Dependencies:** Depends on Phases 1-6.

---

## Verification & Quality Gates

### Phase-by-Phase Verification

**Phase 1:**
- Proto compilation without errors ✓
- Job hashing deterministic ✓
- Metadata store CRUD operations ✓
- Logging all levels working ✓
- gRPC services compile ✓

**Phase 2:**
- mDNS discovery: 3+ peers discover each other < 5 sec ✓
- gRPC services bidirectional ✓
- Progress streaming 5+ events/sec ✓
- Timeout detection (simulate 10 min silence) ✓

**Phase 3:**
- Execute local C/C++ compile job ✓
- Store artifact in cache ✓
- Retrieve from cache (hash match) ✓
- Stream artifact from peer ✓
- Parallel input fetch (3 inputs from 3 peers) ✓

**Phase 4:**
- DAG topological sort correct ✓
- Job quorum scheduling prevents double-booking ✓
- Affinity favors local peer ✓
- Cancel job + dependent DAG ✓
- Retry with exponential backoff ✓

**Phase 5:**
- Client daemon starts + registers ✓
- `buildozer-gcc` compiles source → object ✓
- REST API returns correct status ✓
- Web UI shows live job queue ✓

**Phase 6:**
- Detect GCC/G++/toolchains ✓
- Compile job → object file ✓
- Link job → executable ✓
- Multi-job project (compile + link sequence) ✓

**Phase 7:**
- Unit test coverage >75% ✓
- Integration: 3 peers, 10 jobs, all execute ✓
- Auth token validation ✓
- Docs complete + CI passing ✓

### Integration Test Scenarios

1. **Single peer, single job:** Driver submits compile job → peer executes → result returned
2. **Multi-peer cache hit:** Peer A compiles, peer B queries result from A
3. **Job dependencies:** 3 compiles + 1 link; compiles execute, then link
4. **Scheduling affinity:** 2 peers, job from peer A → scheduled on peer A
5. **Timeout & recovery:** Execution peer silent for 10 min → source times out → job marked failed → network notified
6. **Peer crash:** Execution peer crashes mid-job → source detects via no progress → job rerouted

### Test Metrics
- Coverage: `go test -cover ./...` (target >75%)
- Performance: Scheduling latency <100ms p99, cache lookup <50ms p99
- Reliability: Multi-peer setup, 100 jobs, 99%+ completion rate

---

## Critical Files & Structure

### Proto Definitions (Layer 1 - Protocol Contracts)
- `pkg/proto/job.proto` — Job abstraction with oneof for job types
- `pkg/proto/job_data.proto` — JobData with streaming support and hashing
- `pkg/proto/runtime.proto` — Runtime model + recipes
- `pkg/proto/client_capability.proto` — Peer capabilities (CPU, RAM, concurrent jobs, runtimes)
- `pkg/proto/network_message.proto` — Job submission, status, progress, errors
- `pkg/proto/auth.proto` — Auth metadata (client_id, token, protocol_version)

### Core Services (Layer 2 - Networking)
- `pkg/services/job_service.grpc.pb.go` — Submit, query, cancel jobs
- `pkg/services/peer_service.grpc.pb.go` — Execute jobs, announce capabilities
- `pkg/services/artifact_service.grpc.pb.go` — Stream artifacts, query cache
- `pkg/discovery/mdns.go` — mDNS registration + listening

### Job Execution & Caching (Layer 2-3 - Execution)
- `pkg/jobs/job.go` — Base Job interface, hashing logic
- `pkg/jobs/cpp/compile_job.go` — C/C++ compile job
- `pkg/jobs/cpp/link_job.go` — C/C++ link job
- `pkg/executor/executor.go` — Job executor interface
- `pkg/executor/local_executor.go` — Execute on host
- `pkg/executor/container_executor.go` — Execute in Docker container
- `pkg/cache/cache.go` — Cache interface (local + network queries)
- `pkg/cache/retention_policy.go` — Size/LRU/age-based eviction

### Orchestration & Scheduling (Layer 3-4 - Coordination)
- `pkg/graph/dag.go` — Dependency DAG, topological sort, cycle detection
- `pkg/queue/queue.go` — Per-client job queue (pending + ready)
- `pkg/scheduler/scheduler.go` — Quorum-based scheduling, bin-packing
- `pkg/resilience/timeout.go` — Timeout detection, progress tracking
- `pkg/resilience/cancellation.go` — Job cancellation logic

### Client & Storage (Layer 1-4 Infrastructure)
- `pkg/store/store.go` — Metadata persistence interface
- `pkg/store/boltdb/store.go` — BoltDB implementation
- `pkg/logging/logger.go` — Component-scoped slog logger
- `pkg/toolchain/cpp/detector.go` — GCC/G++ detection

### Command-Line Tools & API (Layer 4)
- `cmd/buildozer-client/main.go` — Client daemon entry
- `cmd/buildozer-gcc/main.go` — GCC driver
- `cmd/buildozer-g++/main.go` — G++ driver  
- `cmd/buildozer-make/main.go` — Make driver
- `cmd/api/main.go` — REST API gateway
- `cmd/dashboard/main.go` — Web UI server

---

## Final Schedule & Dependencies

### Critical Path

```
Week 1-4:   Phase 1 (Foundation)
  ↓
Week 5-7:   Phase 2 (Networking) — *depends on Phase 1*
Week 8-12:  Phase 3 (Caching) — *parallel with rest, depends on Phase 2*
Week 13-16: Phase 4 (Scheduling) — *depends on Phase 1-3*
Week 17-20: Phase 5 (Drivers) — *depends on Phase 1-4*
Week 21-23: Phase 6 (C/C++) — *depends on Phase 5*
Week 24-26: Phase 7 (Testing) — *depends on Phase 1-6*

Total: ~26 weeks (~6 months)
```

### Parallel Work Opportunities
- Phase 3 (caching) can start once Phase 2 is complete
- Phase 5 (drivers) can start once Phase 1 is complete
- Phase 6 (C/C++) can start once Phase 5 is complete
- Phase 7 (testing) should span Phases 5-26

---

## Decisions & Rationale

1. **Start from conceptual redesign:** Current implementation incomplete; full restart allows clean architecture aligned with actual requirements (quorum scheduling, streaming, DAGs, remote logging).

2. **Generic Job model with oneof:** Supports C/C++, future languages (Go, Rust, etc.) without changing core protocols.

3. **Content-addressed caching:** Deterministic hash = reproducibility + cross-network reuse + safe concurrency.

4. **Quorum scheduling:** Prevents race conditions; agreement before scheduling prevents double-booking.

5. **Real-time progress + timeout:** Operator visibility + automatic failure detection without explicit handshakes.

6. **mDNS discovery:** Zero-config, works offline; suffices for local networks; scalable via Consul/etcd if needed later.

7. **BoltDB for metadata:** Embedded, single-file, no external dependencies; sufficient for client-side state.

8. **Streaming artifacts:** Handles large outputs (100+ MB); multiple sources in parallel.

9. **slog logging with remote tail:** Structured, queryable logs; essential for multi-peer debugging.

10. **Dummy auth placeholder:** Meet user spec (mark TODO) without blocking MVP; real auth (TLS/tokens) in future.

---

## Open Questions & Future Expansion

### Clarifications (if needed)
1. **Cross-language support:** C/C++ first (Phase 6); Go/Rust deferred to Phase 8+ (beyond initial scope)
2. **Distributed metadata store:** Current plan uses local BoltDB per client; global state via quorum broadcast
3. **Cache replication:** Retention policies handle local eviction; optional replication via peer sync (Phase 7+)

### Future Phases (Post-GA)
- **Phase 8:** Go/Rust language support
- **Phase 9:** Vault/credential management
- **Phase 10:** Kubernetes integration, distributed metadata DB
- **Phase 11:** Performance optimization (compressed artifacts, delta sync)
- **Phase 12:** Multi-region federation

---

## Development Policy

### Backwards Compatibility
- **Default:** Do NOT maintain backwards compatibility. Remove deprecated APIs and functions without hesitation.
- **Exception:** Only maintain backwards compatibility if explicitly requested by the user in the task description.
- **Rationale:** Clean code, reduced maintenance burden, no duplicate deprecated implementations. Clear migration path is better than legacy compatibility.

### Logger Hierarchy
- **Root logger:** `logging.Log()` returns the main `"buildozer"` logger, set as default slog logger.
- **Package loggers:** Every package must have a `logger.go` file with a `Log()` function.
- **Hierarchy rule:** Package loggers are created as children of the root logger (or parent package logger if applicable):
  - Top-level packages: `logging.Log().Child("package_name")`
  - Nested packages: Build full path from root: `logging.Log().Child("parent").Child("child")`
  - Example: `pkg/runtimes/cpp/native/Log()` → `logging.Log().Child("runtimes").Child("cpp").Child("native")`
- **Component loggers:** Each component (struct, service, handler) within a package should have its own logger as a child of the package logger:
  - Component loggers are **embedded unnamed** in component structs for direct method access
  - Embed as `*logging.Logger` so methods are promoted to component type
  - Components can call logging methods directly: `.Debug()`, `.Info()`, `.Error()`
  - Components return errors using `.Errorf(format, args...)` which both logs AND returns the error (like `fmt.Errorf()`)
  - Example:
    ```go
    type httpServer struct {
        *logging.Logger  // unnamed embedded field
        config DaemonConfig
        // other fields...
    }
    
    func newHTTPServer(config DaemonConfig) *httpServer {
        return &httpServer{
            Logger: daemon.Log().Child("httpServer"),  // or use field name if not embedded
            config: config,
        }
    }
    
    // Now httpServer can call logging and error methods directly:
    func (hs *httpServer) start() error {
        hs.Info("starting HTTP server")  // promoted method
        if err := listen(); err != nil {
            return hs.Errorf("failed to listen: %w", err)  // logs error and returns it
        }
    }
    ```
- **Rationale:** Maintains proper logger hierarchy for structured logging and remote log queries by component path. Embedding provides clean, direct access to logging methods without verbose `.logger.` prefixes, and `.Errorf()` integration ensures all errors are logged.

---

**Plan Version:** 2.0  
**Status:** Ready for implementation  
**Last Updated:** 2026-03-17  
**Policy Updates:** 2026-03-22 (Backwards compatibility, Logger hierarchy with component loggers)
