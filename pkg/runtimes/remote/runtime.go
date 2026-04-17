package remote

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"connectrpc.com/connect"
	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	protov1connect "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1/protov1connect"
	"github.com/Manu343726/buildozer/pkg/logging"
	"github.com/Manu343726/buildozer/pkg/peers"
	"github.com/Manu343726/buildozer/pkg/runtime"
	"github.com/Manu343726/buildozer/pkg/sandbox"
)

// RemoteRuntime implements the runtime.Runtime interface for remote daemon execution.
// It wraps RPC calls to a remote peer's RuntimeService and ExecutorService to provide
// transparent execution on remote resources.
type RemoteRuntime struct {
	*logging.Logger

	// proto is the remote runtime's proto representation
	proto *v1.Runtime

	// peerId is the ID of the peer daemon hosting this runtime
	peerId string

	// peerManager provides access to peer information for RPC endpoint construction
	peerManager *peers.Manager
}

// NewRemoteRuntime creates a new RemoteRuntime that delegates operations to a remote peer via RPC.
// The runtime is wrapped with EmbedInputs sandbox to ensure all job inputs are embedded
// for self-contained remote execution without file references.
func NewRemoteRuntime(
	proto *v1.Runtime,
	peerId string,
	peerManager *peers.Manager,
) runtime.Runtime {
	logger := Log().Child(proto.Id)
	remoteRT := &RemoteRuntime{
		Logger:      logger,
		proto:       proto,
		peerId:      peerId,
		peerManager: peerManager,
	}

	// Wrap with EmbedInputs sandbox to automatically embed all inputs
	params := sandbox.SandboxParams{Logger: logger}
	return sandbox.MustApply(remoteRT, params, sandbox.EmbedInputs())
}

// Execute submits a job to the remote daemon for execution via ExecutorService P2P RPC.
// The job streams progress and results back from the remote peer.
// Job progress is forwarded through the request's ProgressCallback.
// All job inputs are expected to be embedded (content-based, not reference-based) via EmbedInputsSandbox wrapper.
func (r *RemoteRuntime) Execute(ctx context.Context, req *runtime.ExecutionRequest) (*runtime.ExecutionResult, error) {
	r.Debug("Executing job remotely",
		"runtimeID", r.proto.Id,
		"jobID", req.FullJob.Id,
		"peerId", r.peerId,
	)

	job := req.FullJob

	// Get the peer and verify it's alive
	peer := r.peerManager.GetPeer(r.peerId)
	if peer == nil {
		return nil, fmt.Errorf("peer not found: %s", r.peerId)
	}

	if !peer.IsAlive {
		return nil, fmt.Errorf("peer not alive: %s", r.peerId)
	}

	// Create ExecutorService client for the remote peer (P2P execution service)
	executorClient := protov1connect.NewExecutorServiceClient(http.DefaultClient, "http://"+peer.Endpoint)

	// Stream job execution via ExecuteJob RPC
	executeReq := &v1.ExecuteJobRequest{
		Job: job,
		// TODO: Set metadata, timeout, and requester_info as needed
	}

	stream, err := executorClient.ExecuteJob(ctx, connect.NewRequest(executeReq))
	if err != nil {
		r.Error("Remote job execution failed", "jobID", req.FullJob.Id, "error", err)
		return nil, fmt.Errorf("failed to execute job on remote peer %s: %w", r.peerId, err)
	}

	// Consume the response stream to completion, forwarding progress
	var finalResult *v1.JobResult
	for stream.Receive() {
		resp := stream.Msg()

		// Forward progress updates if callback is provided
		if resp.Progress != nil && req.ProgressCallback != nil {
			// Report status changes
			progress := &runtime.Progress{
				Type:        runtime.ProgressTypeStatus,
				Source:      "remote-executor",
				Data:        []byte(resp.Progress.Status.String()),
				Timestamp:   time.Now(),
				ExecutionID: req.FullJob.Id,
			}

			if err := req.ProgressCallback(ctx, progress); err != nil {
				r.Warn("Progress callback error", "jobID", req.FullJob.Id, "error", err)
			}

			// Report log output if available
			if resp.Progress.LogOutput != "" {
				logProgress := &runtime.Progress{
					Type:        runtime.ProgressTypeLog,
					Source:      "remote-executor",
					Data:        []byte(resp.Progress.LogOutput),
					Timestamp:   time.Now(),
					ExecutionID: req.FullJob.Id,
				}

				if err := req.ProgressCallback(ctx, logProgress); err != nil {
					r.Warn("Progress callback error", "jobID", req.FullJob.Id, "error", err)
				}
			}
		}

		// Forward output data if provided
		if resp.Output != nil && req.ProgressCallback != nil {
			// Extract data content based on type
			var dataBytes []byte

			if fileData := resp.Output.GetFile(); fileData != nil {
				dataBytes = fileData.Content
			} else if streamChunk := resp.Output.GetStreamChunk(); streamChunk != nil {
				dataBytes = streamChunk.Data
			}

			if len(dataBytes) > 0 {
				progress := &runtime.Progress{
					Type:        runtime.ProgressTypeOutput,
					Source:      "remote-executor",
					Data:        dataBytes,
					Timestamp:   time.Now(),
					ExecutionID: req.FullJob.Id,
				}

				if err := req.ProgressCallback(ctx, progress); err != nil {
					r.Warn("Progress callback error", "jobID", req.FullJob.Id, "error", err)
				}
			}
		}

		// Capture final result
		if resp.Result != nil {
			finalResult = resp.Result
		}
	}

	if err := stream.Err(); err != nil {
		r.Error("Error receiving job results", "jobID", req.FullJob.Id, "error", err)
		return nil, fmt.Errorf("error during remote job execution on peer %s: %w", r.peerId, err)
	}

	// Convert final result to ExecutionResult
	exitCode := 1
	if finalResult != nil {
		exitCode = int(finalResult.ExitCode)
	}

	r.Debug("Remote job execution completed",
		"jobID", req.FullJob.Id,
		"peerId", r.peerId,
		"exitCode", exitCode,
	)

	return &runtime.ExecutionResult{
		ExitCode: exitCode,
	}, nil
}

// Available checks if the remote peer is alive and the runtime is available.
func (r *RemoteRuntime) Available(ctx context.Context) (bool, error) {
	if r.peerManager == nil {
		return false, fmt.Errorf("peer manager not available")
	}

	peer := r.peerManager.GetPeer(r.peerId)
	if peer == nil {
		r.Warn("Peer not found", "peerId", r.peerId)
		return false, nil
	}

	if !peer.IsAlive {
		r.Warn("Peer is not alive", "peerId", r.peerId)
		return false, nil
	}

	r.Debug("Remote runtime is available",
		"runtimeID", r.proto.Id,
		"peerId", r.peerId,
	)

	return true, nil
}

// Metadata returns metadata about the remote runtime.
func (r *RemoteRuntime) Metadata(ctx context.Context) (*runtime.Metadata, error) {
	description := ""
	if r.proto.Description != nil {
		description = *r.proto.Description
	}

	meta := &runtime.Metadata{
		RuntimeID:   r.proto.Id,
		Language:    r.proto.Toolchain.String(),
		Description: description,
		IsNative:    false, // Remote runtimes are accessed through RPC
	}

	// Extract toolchain-specific metadata if available
	if cpp := r.proto.GetCpp(); cpp != nil {
		meta.Language = "cpp"
		meta.TargetArch = cpp.Architecture.String()
		if cpp.CompilerVersion != nil {
			meta.Version = cpp.CompilerVersion.String()
		}
	}

	return meta, nil
}

// Proto returns the runtime's proto representation.
func (r *RemoteRuntime) Proto(ctx context.Context) (*v1.Runtime, error) {
	return r.proto, nil
}

// RuntimeID returns the unique identifier for this runtime.
func (r *RemoteRuntime) RuntimeID() string {
	return r.proto.Id
}

// MatchesQuery delegates query matching to the remote peer via RPC.
// The peer's runtime will evaluate whether it matches the query constraints.
func (r *RemoteRuntime) MatchesQuery(ctx context.Context, query *v1.RuntimeMatchQuery) (bool, error) {
	// Get the peer
	peer := r.peerManager.GetPeer(r.peerId)
	if peer == nil {
		return false, fmt.Errorf("peer not found: %s", r.peerId)
	}

	// Create RuntimeService client for the remote peer
	runtimeClient := protov1connect.NewRuntimeServiceClient(http.DefaultClient, "http://"+peer.Endpoint)

	// Call Match RPC to check if this runtime matches
	resp, err := runtimeClient.Match(ctx, connect.NewRequest(&v1.MatchRuntimesRequest{Query: query}))
	if err != nil {
		return false, fmt.Errorf("failed to query remote peer %s for runtime match: %w", r.peerId, err)
	}

	// Check if our runtime is in the results
	for _, rt := range resp.Msg.Runtimes {
		if rt.Id == r.proto.Id {
			return true, nil
		}
	}

	return false, nil
}

// Close releases any resources held by this runtime.
// For remote runtimes, this is a no-op as there are no persistent resources.
func (r *RemoteRuntime) Close() error {
	return nil
}

// PeerId returns the ID of the peer hosting this runtime.
func (r *RemoteRuntime) PeerId() string {
	return r.peerId
}
