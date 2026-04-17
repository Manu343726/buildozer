package scheduler

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	"github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1/protov1connect"
	"github.com/Manu343726/buildozer/pkg/logging"
	"github.com/Manu343726/buildozer/pkg/peers"
)

// RemoteExecutor handles execution of jobs on remote peer daemons
// Uses the ExecutorService (P2P service) for daemon-to-daemon job execution
// Maintains streaming connections to remote peers for receiving job progress updates
type RemoteExecutor struct {
	logger      *logging.Logger
	peerManager *peers.Manager

	mu                  sync.RWMutex
	activeStreams       map[string]*remoteJobStream       // remote job ID -> stream handler
	progressSubscribers map[string][]chan *v1.JobProgress // remote job ID -> list of subscribers
}

// remoteJobStream manages a single remote job execution stream
type remoteJobStream struct {
	remotePeerID string
	remoteJobID  string
	stream       *connect.ServerStreamForClient[v1.ExecuteJobResponse]
	cancel       context.CancelFunc
	wg           sync.WaitGroup
}

// NewRemoteExecutor creates a new remote job executor
func NewRemoteExecutor(peerManager *peers.Manager) *RemoteExecutor {
	return &RemoteExecutor{
		logger:              LogSubsystem("RemoteExecutor"),
		peerManager:         peerManager,
		activeStreams:       make(map[string]*remoteJobStream),
		progressSubscribers: make(map[string][]chan *v1.JobProgress),
	}
}

// ExecuteRemote submits a prepared job to a remote peer daemon for execution
// Uses the ExecutorService (daemon-to-daemon P2P service) which returns a stream
// Keeps the stream alive to forward job progress updates to subscribers
// Returns the remote job ID and a subscription channel to receive progress updates
func (re *RemoteExecutor) ExecuteRemote(ctx context.Context, remotePeerID string, job *v1.Job) (string, <-chan *v1.JobProgress, error) {
	if remotePeerID == "" {
		return "", nil, fmt.Errorf("remote peer ID cannot be empty")
	}

	if job == nil {
		return "", nil, fmt.Errorf("job cannot be nil")
	}

	if re.peerManager == nil {
		return "", nil, fmt.Errorf("peer manager is not initialized")
	}

	re.logger.Debug("Submitting job to remote peer for execution via ExecutorService",
		"jobID", job.Id,
		"remotePeerID", remotePeerID,
	)

	// Get peer info from manager
	peer := re.peerManager.GetPeer(remotePeerID)
	if peer == nil {
		return "", nil, fmt.Errorf("peer not found: %s", remotePeerID)
	}

	if !peer.IsAlive {
		return "", nil, fmt.Errorf("peer is not alive: %s", remotePeerID)
	}

	// Create ExecutorService client to remote peer
	client := newExecutorServiceClient(peer.Endpoint)

	// Create ExecuteJob request
	req := &v1.ExecuteJobRequest{
		Job: job,
		Timeout: &v1.TimeDuration{
			Count: 3600, // 1 hour default timeout
			Unit:  v1.TimeUnit_TIME_UNIT_SECOND,
		},
		RequesterInfo: &v1.RequesterInfo{
			RequesterId:      "scheduler",
			RequesterType:    "scheduler",
			RequestTimestamp: &v1.TimeStamp{UnixMillis: time.Now().UnixMilli()},
		},
	}

	// Submit job to remote peer via ExecutorService stream
	stream, err := client.ExecuteJob(ctx, &connect.Request[v1.ExecuteJobRequest]{
		Msg: req,
	})
	if err != nil {
		re.logger.Error("Failed to start job execution on remote peer",
			"jobID", job.Id,
			"remotePeerID", remotePeerID,
			"error", err)
		return "", nil, fmt.Errorf("failed to execute job on remote peer %s: %w", remotePeerID, err)
	}

	// Read first response from the stream to confirm job was accepted
	if !stream.Receive() {
		err := stream.Err()
		if err != nil {
			re.logger.Error("Remote peer rejected job execution",
				"jobID", job.Id,
				"remotePeerID", remotePeerID,
				"error", err)
		}
		return "", nil, fmt.Errorf("remote peer rejected job execution or closed stream")
	}

	resp := stream.Msg()

	// Verify job started successfully (progress should indicate queued or running status)
	if resp.Progress == nil || resp.Progress.JobId == "" {
		re.logger.Error("Invalid response from remote peer",
			"jobID", job.Id,
			"remotePeerID", remotePeerID)
		stream.Close()
		return "", nil, fmt.Errorf("remote peer sent invalid response")
	}

	remoteJobID := resp.Progress.JobId

	re.logger.Debug("Job execution started on remote peer",
		"localJobID", job.Id,
		"remotePeerID", remotePeerID,
		"remoteJobID", remoteJobID,
		"remoteStatus", resp.Progress.Status.String(),
	)

	// Create a context for this stream (can be cancelled independently)
	streamCtx, cancel := context.WithCancel(context.Background())

	// Register the stream and start consuming it in background
	jobStream := &remoteJobStream{
		remotePeerID: remotePeerID,
		remoteJobID:  remoteJobID,
		stream:       stream,
		cancel:       cancel,
	}

	re.mu.Lock()
	re.activeStreams[remoteJobID] = jobStream
	re.mu.Unlock()

	// Subscribe to progress updates
	progressChan := re.SubscribeToProgress(remoteJobID)

	// Start background goroutine to consume stream and distribute progress updates
	jobStream.wg.Add(1)
	go re.consumeRemoteStream(streamCtx, jobStream, resp.Progress) // Pass initial progress

	re.logger.Debug("Started consuming remote job stream",
		"remoteJobID", remoteJobID,
		"remotePeerID", remotePeerID,
	)

	return remoteJobID, progressChan, nil
}

// consumeRemoteStream continuously reads job progress from a remote peer's ExecutorService stream
// and distributes updates to all subscribers
func (re *RemoteExecutor) consumeRemoteStream(ctx context.Context, jobStream *remoteJobStream, initialProgress *v1.JobProgress) {
	defer jobStream.wg.Done()
	defer jobStream.stream.Close()

	remoteJobID := jobStream.remoteJobID

	// Distribute initial progress to all subscribers
	re.distributeProgress(remoteJobID, initialProgress)

	// Keep reading from stream until completion or context cancellation
	for jobStream.stream.Receive() {
		select {
		case <-ctx.Done():
			re.logger.Debug("Stream context cancelled", "remoteJobID", remoteJobID)
			return
		default:
		}

		resp := jobStream.stream.Msg()

		// Forward progress updates to all subscribers
		if resp.Progress != nil {
			re.distributeProgress(remoteJobID, resp.Progress)

			progressValue := uint32(0)
			if resp.Progress.ProgressPercent != nil {
				progressValue = resp.Progress.ProgressPercent.Value
			}

			re.logger.Debug("Received remote job progress",
				"remoteJobID", remoteJobID,
				"status", resp.Progress.Status.String(),
				"percentComplete", progressValue,
			)
		}

		// If we received a final result, stream is ending
		if resp.Result != nil {
			resultStatus := "unknown"
			if resp.Result != nil {
				resultStatus = resp.Result.Status.String()
			}
			re.logger.Info("Remote job completed",
				"remoteJobID", remoteJobID,
				"status", resultStatus,
			)
			break
		}
	}

	// Stream ended, clean up
	if err := jobStream.stream.Err(); err != nil {
		re.logger.Error("Remote stream error",
			"remoteJobID", remoteJobID,
			"remotePeerID", jobStream.remotePeerID,
			"error", err)
	}

	re.mu.Lock()
	delete(re.activeStreams, remoteJobID)
	re.mu.Unlock()

	re.logger.Debug("Remote job stream closed",
		"remoteJobID", remoteJobID,
		"remotePeerID", jobStream.remotePeerID,
	)
}

// SubscribeToProgress subscribes to progress updates for a remote job
// Returns a channel that will receive JobProgress updates until the job completes
// Multiple subscribers can listen to the same remote job
func (re *RemoteExecutor) SubscribeToProgress(remoteJobID string) <-chan *v1.JobProgress {
	progressChan := make(chan *v1.JobProgress, 10) // Buffered to avoid blocking

	re.mu.Lock()
	defer re.mu.Unlock()

	re.progressSubscribers[remoteJobID] = append(re.progressSubscribers[remoteJobID], progressChan)

	re.logger.Debug("New progress subscriber",
		"remoteJobID", remoteJobID,
		"subscriberCount", len(re.progressSubscribers[remoteJobID]),
	)

	return progressChan
}

// UnsubscribeFromProgress unsubscribes a progress channel
func (re *RemoteExecutor) UnsubscribeFromProgress(remoteJobID string, progressChan chan *v1.JobProgress) {
	re.mu.Lock()
	defer re.mu.Unlock()

	subscribers, exists := re.progressSubscribers[remoteJobID]
	if !exists {
		return
	}

	// Find and remove the channel
	for i, ch := range subscribers {
		if ch == progressChan {
			// Remove from slice
			re.progressSubscribers[remoteJobID] = append(
				subscribers[:i],
				subscribers[i+1:]...,
			)
			close(progressChan)
			break
		}
	}

	// Clean up if no more subscribers
	if len(re.progressSubscribers[remoteJobID]) == 0 {
		delete(re.progressSubscribers, remoteJobID)
	}
}

// distributeProgress sends a progress update to all subscribers of a remote job
func (re *RemoteExecutor) distributeProgress(remoteJobID string, progress *v1.JobProgress) {
	re.mu.RLock()
	subscribers, exists := re.progressSubscribers[remoteJobID]
	re.mu.RUnlock()

	if !exists || len(subscribers) == 0 {
		return
	}

	for _, ch := range subscribers {
		select {
		case ch <- progress:
		default:
			// Channel full, skip this update to avoid blocking
			// Subscriber should keep up with the stream
		}
	}
}

// CancelRemoteJob cancels a job running on a remote peer
// Note: ExecutorService doesn't have a CancelJob RPC yet, so this is a placeholder
// In a complete implementation, we'd need to add CancelJob to ExecutorService
func (re *RemoteExecutor) CancelRemoteJob(ctx context.Context, remotePeerID string, remoteJobID string, reason string) error {
	if remotePeerID == "" {
		return fmt.Errorf("remote peer ID cannot be empty")
	}

	if remoteJobID == "" {
		return fmt.Errorf("remote job ID cannot be empty")
	}

	if re.peerManager == nil {
		return fmt.Errorf("peer manager is not initialized")
	}

	// Verify peer exists and is alive
	peer := re.peerManager.GetPeer(remotePeerID)
	if peer == nil {
		return fmt.Errorf("peer not found: %s", remotePeerID)
	}

	if !peer.IsAlive {
		return fmt.Errorf("peer is not alive: %s", remotePeerID)
	}

	re.logger.Debug("Cancelling remote job",
		"remoteJobID", remoteJobID,
		"remotePeerID", remotePeerID,
		"reason", reason,
	)

	// Cancel the stream if it's active
	re.mu.Lock()
	jobStream, exists := re.activeStreams[remoteJobID]
	re.mu.Unlock()

	if exists && jobStream != nil {
		jobStream.cancel()
	}

	// TODO: Implement CancelJob RPC in ExecutorService proto
	// For now, we just cancel the local stream subscription
	re.logger.Warn("CancelJob not implemented in ExecutorService yet, cancelled local stream",
		"remoteJobID", remoteJobID,
		"remotePeerID", remotePeerID,
	)

	return nil
}

// executorServiceClient wraps the gRPC client for ExecutorService
type executorServiceClient struct {
	client protov1connect.ExecutorServiceClient
}

// newExecutorServiceClient creates a new executor service client for a remote peer endpoint
func newExecutorServiceClient(endpoint string) *executorServiceClient {
	// Ensure endpoint has a protocol scheme
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		endpoint = "http://" + endpoint
	}

	httpClient := &http.Client{}
	client := protov1connect.NewExecutorServiceClient(httpClient, endpoint)

	return &executorServiceClient{
		client: client,
	}
}

// ExecuteJob calls the ExecuteJob RPC method on the remote peer
func (c *executorServiceClient) ExecuteJob(ctx context.Context, req *connect.Request[v1.ExecuteJobRequest]) (*connect.ServerStreamForClient[v1.ExecuteJobResponse], error) {
	return c.client.ExecuteJob(ctx, req)
}
