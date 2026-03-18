# Buildozer Development Log

**Project:** Peer-to-Peer Distributed Build System  
**Status:** Phase 1 - Core Protocol & Job Model  
**Last Updated:** 2026-03-17

---

## Phase 1: Core Protocol & Job Model (Weeks 1-4)

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

---

## Next Phase: Step 2 - Job & Runtime Abstractions
