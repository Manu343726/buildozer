// Integration Guide: Scheduler with Job Manager
//
// The new scheduler package is designed to be seamlessly integrated into pkg/daemon/job_manager.go
// without changing the driver-daemon interface. This document outlines the integration approach.
//
// ## Current Job Manager Architecture
//
// JobManager has two main responsibilities:
// 1. Queue-based job submission and tracking (JobQueue, JobState, status streaming)
// 2. Sequential job execution (processQueueLoop -> executeJob)
//
// ## Integration Design
//
// The scheduler will be added to JobManager as a new component that makes placement decisions
// without modifying external interfaces:
//
// ```go
// type JobManager struct {
//    // ... existing fields ...
//    scheduler *scheduler.Scheduler  // New: placement decision maker
// }
// ```
//
// ## Execution Flow Changes
//
// Before integration:
//      SubmitJob -> Enqueue -> processQueueLoop -> executeJob -> local execution
//
// After integration:
//      SubmitJob -> Enqueue -> processQueueLoop -> executeJob:
//                   ├─ Call scheduler.EnqueueJob() (queues or immediately schedules)
//                   ├─ If immediately scheduled and local: run local execution
//                   ├─ If immediately scheduled and remote: run remote execution + track
//                   └─ If queued: wait for runtimes to become available
//
// ## Key Integration Points
//
// ### 1. Initialize Scheduler (in NewJobManager or Start)
// ```go
// jobMgr.scheduler, err = scheduler.NewScheduler(&scheduler.SchedulerConfig{
//     Heuristic:      scheduler.NewSimpleLocalFirstHeuristic(),
//     RuntimeManager: jobMgr.runtimeMgr, // Existing runtime manager
//     LocalDaemonID:  jobMgr.daemonID,
// })
// ```
//
// ### 2. Decision Point in executeJob (use EnqueueJob - the public entry point)
// ```go
// func (jm *JobManager) executeJob(jobState *JobState) {
//     defer jm.jobCompleted()
//     
//     job := jobState.Job
//     
//     // NEW: Use scheduler's queue system (EnqueueJob is the public entry point)
//     decision, err := jm.scheduler.EnqueueJob(ctx, job, job.Cwd)
//     
//     // Handle three outcomes:
//     if err == scheduler.ErrAllRuntimesBusy {
//         // Job was queued - will be retried when runtimes available
//         return
//     }
//     if err == scheduler.ErrNoCompatibleRuntimes {
//         // Permanent failure - no execution environment for this job
//         // Mark job as failed and return
//     }
//     if err != nil {
//         // Other errors - handle and fail job
//     }
//     
//     // Scheduling successful - check if local or remote
//     if decision.PeerId == jm.daemonID {
//         jm.executeLocalJob(jobState, decision)
//         return
//     }
//     
//     // Remote execution (subprocess to handle asynchronously)
//     jm.executeRemoteJob(jobState, jm.scheduler)
// }
// ```
//
// ### 3. New Remote Job Handler
// ```go
// func (jm *JobManager) executeRemoteJob(jobState *JobState, decision *scheduler.SchedulingDecision) {
//     // 1. Remote execution is asynchronous - request job submission to remote peer
//     // 2. Remote peer runs the job and streams progress back via WatchJobStatus RPC
//     // 3. Local daemon receives status updates via remote daemon's gRPC notifications
//     // 4. Status updates are forwarded to local watchers via jobState.UpdateProgress()
// }
// ```
//
// ## Status Streaming (Unchanged)
//
// The driver-daemon interface remains unchanged:
// - Driver still calls daemon.SubmitJob() and daemon.WatchJobStatus()
// - Job status is still tracked in JobState.Progress
// - Status updates are still sent to watchers regardless of where job executes
// - For remote jobs: status updates come from remote peer notifications -> UpdateProgress()
//
// ## Why This Works
//
// 1. **No Protocol Changes**: Job proto stays the same, RuntimeRequirement still supports both
//    explicit Runtime and RuntimeMatchQuery
// 2. **Status Abstraction**: JobState abstracts execution location - watchers don't know if 
//    job executed locally or remotely
// 3. **Placement Transparency**: Drivers get same result regardless of execution location
// 4. **Error Handling**: Remote job failures look like local job failures to driver
//
// ## Future Enhancement: Delegated Execution
//
// When remote daemon receives a remote job:
// 1. Remote daemon calls its own job manager's SubmitJob()
// 2. Remote daemon treats it as a local job (runs on remote's runtime)
// 3. Remote daemon sends progress updates back to original daemon
// 4. Original daemon forwards updates to watchers
//
// This creates a transparent relay: Driver -> Local Daemon -> Remote Daemon -> Execution
//
// ## Testing Strategy
//
// 1. Unit tests in scheduler package (✅ Done)
// 2. Integration tests in job_manager_test.go:
//    - Mock scheduler with fixed decisions
//    - Verify local vs remote decisions route correctly
//    - Verify status streaming works for both paths
// 3. Network tests (future):
//    - Real inter-daemon communication
//    - Job status streaming across network
//    - Failure scenarios
//
// ## Configuration
//
// Scheduler heuristic can be configured via:
// - Daemon startup flags
// - Configuration file
// - Runtime environment (for now, SimpleLocalFirstHeuristic is hardcoded)
//
// Future: SchedulingHeuristic loaded from config name mapping
//
