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

**Tech Stack:** Go, gRPC, Protocol Buffers, mDNS (grandcat/zeroconf), Docker API, slog

**Timeline:** ~26 weeks (6 months) across 7 phases

---

## Architecture

### Design Principles

1. **Generic job abstraction** — Single Job model with oneof for different job types; extensible to any language/domain
2. **Content-addressed caching** — Jobs hashed by (type + runtime + inputs); outputs tagged with runtime that generated them
3. **Decentralized** — No central coordinator; peers communicate via gRPC; scheduling uses quorum consensus
4. **Real-time progress** — Execution client streams progress back to source; timeout-based failure detection
5. **Protocol-first** — All communication via gRPC; Protobuf as contract; backward compatible via semver versioning
6. **Distributed data transfer** — Inputs/outputs streamed; multiple clients can send input parts in parallel
7. **Observability first** — Comprehensive slog-based logging (all levels: error/warning/info/debug/trace) with remote queryability

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

---

## Implementation Plan

### Phase 1: Core Protocol & Job Model (Weeks 1-4)

**Goal:** Define protocol contracts, job abstractions, and basic P2P communication.

#### Step 1: Protocol Buffers v1 (Week 1)

**Affected files:** `pkg/proto/*.proto`

Create:
- `pkg/proto/runtime.proto` — Runtime model (language, compiler, version, architecture, C runtime) + runtime recipe (Dockerfile)
- `pkg/proto/job.proto` — Base `Job` message with oneof for job types (CppCompileJob, CppLinkJob, extensible)
- `pkg/proto/job_data.proto` — `JobData` abstraction with oneof (File, Stream, etc.), hash, runtime tag
- `pkg/proto/client_capability.proto` — CPU cores, RAM, concurrent job limits, supported runtimes
- Update `pkg/proto/network_message.proto` — Job submission, status updates, progress events, error reports
- Create `pkg/proto/auth.proto` — Auth metadata: client_id, token, protocol_version

**Key concepts:**
- Job hash = SHA256(JobType + RuntimeID + InputHashes + RecipeHash)
- JobData streaming with hash verification
- RuntimeRecipe embedding (Dockerfile content hasheable)
- Semantic versioning in metadata

**Verification:** Protoc compilation succeeds; all `.pb.go` generated without warnings.
**Dependencies:** None.

#### Step 2: Job & Runtime Abstractions (Week 1-2)

**Affected files:** `pkg/jobs/`, `pkg/runtime/`

Create:
- `pkg/jobs/job.go` — Base Job interface, hash computation, dependency extraction
- `pkg/jobs/cpp/compile_job.go`, `pkg/jobs/cpp/link_job.go` — C/C++ implementations
- `pkg/runtime/runtime.go` — Runtime model + matching (does this peer support this job's runtime?)
- `pkg/runtime/recipe_parser.go` — Parse RuntimeRecipe (Dockerfile → executable runtime)

**Hashing strategy:** `Hash = SHA256(JobType || RuntimeID || InputHashes || RecipeHash)`

**Verification:** Unit tests for hash computation; different inputs → different hashes; same inputs → same hash.
**Dependencies:** Depends on Step 1.

#### Step 3: Persistent Metadata Store (Week 2-3)

**Affected files:** `pkg/store/`

Create:
- `pkg/store/store.go` — Interface for local metadata persistence
- `pkg/store/boltdb/boltdb_store.go` — BoltDB implementation (embedded, single-file)
- Data models: Client metadata, job metadata, cache index, peer directory
- Operations: Store/retrieve job by hash, cache lookup, peer discovery record

**Verification:** CRUD operations tested; persistence across restarts; concurrent access safe.
**Dependencies:** Depends on Step 2.

#### Step 4: Structured Logging Setup (Week 2)

**Affected files:** `pkg/logging/`

Create:
- `pkg/logging/logger.go` — Component-scoped logger factory using slog
- Log levels: Error, Warn, Info, Debug, Trace (per user spec)
- Structured fields: component, request_id, job_id, client_id, peer_addr
- Remote log querying interface (stub for now; implement in Phase 3)

**Verification:** All log levels work; component identifiers visible in output.
**Dependencies:** None.

#### Step 5: gRPC Services Foundation (Week 3-4)

**Affected files:** `pkg/proto/`, `pkg/services/`

Define three gRPC services in `.proto`:
- `JobService.SubmitJob(Job) → JobStatus` — Client submits a job
- `JobService.GetJobStatus(JobID) → JobStatus` — Query job progress (polling or streaming)
- `JobService.QueryCache(Hash) → CacheLocation[]` — Find which peers have cached result
- `PeerService.ExecuteJob(Job) → stream JobProgress` — Peer executes; streams progress
- `PeerService.Announce() → PeerCapabilities` — Peer announces itself (for discovery)

**Verification:** gRPC services compile; can instantiate servers without implementation.
**Dependencies:** Depends on Step 1.

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

**Plan Version:** 2.0  
**Status:** Ready for implementation  
**Last Updated:** 2026-03-17
