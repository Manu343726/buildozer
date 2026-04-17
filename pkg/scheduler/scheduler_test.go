package scheduler

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	"github.com/Manu343726/buildozer/pkg/peers"
	"github.com/Manu343726/buildozer/pkg/runtime"
	"github.com/Manu343726/buildozer/pkg/runtimes"
)

// MockRuntimeManager is a test mock for runtimes.Manager
type MockRuntimeManager struct {
	localRuntimes []runtime.Runtime
	allRuntimes   []*v1.Runtime
	shouldError   bool
	errorMessage  string
}

func NewMockRuntimeManager() *MockRuntimeManager {
	return &MockRuntimeManager{
		localRuntimes: make([]runtime.Runtime, 0),
		allRuntimes:   make([]*v1.Runtime, 0),
	}
}

func (m *MockRuntimeManager) ListRuntimes(ctx context.Context) ([]*v1.Runtime, string, error) {
	if m.shouldError {
		return nil, "", errors.New(m.errorMessage)
	}
	return m.allRuntimes, "mock runtimes", nil
}

func (m *MockRuntimeManager) GetRuntimes(ctx context.Context) ([]runtime.Runtime, string, error) {
	if m.shouldError {
		return nil, "", errors.New(m.errorMessage)
	}
	return m.localRuntimes, "mock local runtimes", nil
}

func (m *MockRuntimeManager) GetRuntimeByID(ctx context.Context, runtimeID string) (runtime.Runtime, error) {
	for _, rt := range m.localRuntimes {
		rtProto, err := rt.Proto(ctx)
		if err != nil {
			continue
		}
		if rtProto.Id == runtimeID {
			return rt, nil
		}
	}
	return nil, fmt.Errorf("runtime not found: %s", runtimeID)
}

func (m *MockRuntimeManager) Match(ctx context.Context, query *v1.RuntimeMatchQuery) ([]*v1.Runtime, error) {
	if m.shouldError {
		return nil, errors.New(m.errorMessage)
	}
	// Return all runtimes as a simple mock (no actual matching logic)
	return m.allRuntimes, nil
}

// MockLocalRuntime is a test mock for runtime.Runtime
type MockLocalRuntime struct {
	id string
}

func (r *MockLocalRuntime) RuntimeID() string {
	return r.id
}

func (r *MockLocalRuntime) Proto(ctx context.Context) (*v1.Runtime, error) {
	return &v1.Runtime{
		Id:        r.id,
		Platform:  v1.RuntimePlatform_RUNTIME_PLATFORM_NATIVE_LINUX,
		Toolchain: v1.RuntimeToolchain_RUNTIME_TOOLCHAIN_CPP,
	}, nil
}

func (r *MockLocalRuntime) Execute(ctx context.Context, req *runtime.ExecutionRequest) (*runtime.ExecutionResult, error) {
	return nil, fmt.Errorf("not implemented")
}

func (r *MockLocalRuntime) Available(ctx context.Context) (bool, error) {
	return true, nil
}

func (r *MockLocalRuntime) Metadata(ctx context.Context) (*runtime.Metadata, error) {
	return nil, fmt.Errorf("not implemented")
}

func (r *MockLocalRuntime) MatchesQuery(ctx context.Context, query *v1.RuntimeMatchQuery) (bool, error) {
	return false, nil
}

func (r *MockLocalRuntime) Close() error {
	return nil
}

// Ensure MockRuntimeManager implements runtimes.Manager
var _ runtimes.Manager = (*MockRuntimeManager)(nil)

// NewMockPeerManager creates a new mock peer manager for testing
func NewMockPeerManager() *peers.Manager {
	return peers.NewManager()
}

func TestScheduler_NewSchedulerWithValidConfig(t *testing.T) {
	config := &SchedulerConfig{
		Heuristic:      NewSimpleLocalFirstHeuristic(),
		RuntimeManager: NewMockRuntimeManager(),
		PeerManager:    NewMockPeerManager(),
		LocalDaemonID:  "daemon-1",
	}

	scheduler, err := NewScheduler(config)

	require.NoError(t, err, "should create scheduler successfully")
	assert.NotNil(t, scheduler, "scheduler should not be nil")
	assert.Equal(t, "daemon-1", scheduler.config.LocalDaemonID, "daemon ID should be set")
}

func TestScheduler_ErrorWithNilConfig(t *testing.T) {
	scheduler, err := NewScheduler(nil)

	assert.Error(t, err, "should error with nil config")
	assert.Nil(t, scheduler, "scheduler should be nil")
}

func TestScheduler_ErrorWithNilHeuristic(t *testing.T) {
	config := &SchedulerConfig{
		Heuristic:      nil,
		RuntimeManager: NewMockRuntimeManager(),
		PeerManager:    NewMockPeerManager(),
		LocalDaemonID:  "daemon-1",
	}

	scheduler, err := NewScheduler(config)

	assert.Error(t, err, "should error with nil heuristic")
	assert.Nil(t, scheduler, "scheduler should be nil")
}

func TestScheduler_ErrorWithNilRuntimeManager(t *testing.T) {
	config := &SchedulerConfig{
		Heuristic:      NewSimpleLocalFirstHeuristic(),
		RuntimeManager: nil,
		PeerManager:    NewMockPeerManager(),
		LocalDaemonID:  "daemon-1",
	}

	scheduler, err := NewScheduler(config)

	assert.Error(t, err, "should error with nil runtime manager")
	assert.Nil(t, scheduler, "scheduler should be nil")
}

func TestScheduler_ErrorWithEmptyDaemonID(t *testing.T) {
	config := &SchedulerConfig{
		Heuristic:      NewSimpleLocalFirstHeuristic(),
		RuntimeManager: NewMockRuntimeManager(),
		PeerManager:    NewMockPeerManager(),
		LocalDaemonID:  "",
	}

	scheduler, err := NewScheduler(config)

	assert.Error(t, err, "should error with empty daemon ID")
	assert.Nil(t, scheduler, "scheduler should be nil")
}

func TestScheduler_ScheduleLocalJob(t *testing.T) {
	mockMgr := NewMockRuntimeManager()

	localRuntime := &MockLocalRuntime{id: "runtime-1"}
	mockMgr.localRuntimes = []runtime.Runtime{localRuntime}

	protoRuntime := &v1.Runtime{
		Id:        "runtime-1",
		Platform:  v1.RuntimePlatform_RUNTIME_PLATFORM_NATIVE_LINUX,
		Toolchain: v1.RuntimeToolchain_RUNTIME_TOOLCHAIN_CPP,
	}
	mockMgr.allRuntimes = []*v1.Runtime{protoRuntime}

	config := &SchedulerConfig{
		Heuristic:      NewSimpleLocalFirstHeuristic(),
		RuntimeManager: mockMgr,
		PeerManager:    NewMockPeerManager(),
		LocalDaemonID:  "daemon-1",
	}

	scheduler, err := NewScheduler(config)
	require.NoError(t, err, "should create scheduler")

	job := &v1.Job{
		Id:                 "job-1",
		RuntimeRequirement: &v1.Job_Runtime{Runtime: protoRuntime},
	}

	// Use EnqueueJob (the public entry point)
	execReq := &runtime.ExecutionRequest{FullJob: job}
	decision, err := scheduler.EnqueueJob(context.Background(), execReq)

	require.NoError(t, err, "should schedule job successfully")
	assert.NotNil(t, decision, "decision should not be nil")
	assert.Equal(t, "daemon-1", decision.PeerId, "should schedule locally")
	assert.Equal(t, "runtime-1", decision.SelectedRuntime.Id, "should use available runtime")
}

func TestScheduler_EnqueueJobQueuesWhenNothingMatches(t *testing.T) {
	runtimeManager := NewMockRuntimeManager()

	heuristic := NewSimpleLocalFirstHeuristic()

	config := &SchedulerConfig{
		Heuristic:      heuristic,
		RuntimeManager: runtimeManager,
		PeerManager:    &peers.Manager{},
		LocalDaemonID:  "daemon-1",
	}

	scheduler, err := NewScheduler(config)
	require.NoError(t, err, "should create scheduler")

	// Create a job with incompatible runtime requirement
	job := &v1.Job{
		Id: "job-1",
		RuntimeRequirement: &v1.Job_RuntimeMatchQuery{
			RuntimeMatchQuery: &v1.RuntimeMatchQuery{
				Platforms: []v1.RuntimePlatform{v1.RuntimePlatform_RUNTIME_PLATFORM_DOCKER_LINUX},
			},
		},
	}

	// EnqueueJob should queue the job since no compatible runtimes exist
	execReq := &runtime.ExecutionRequest{FullJob: job}
	decision, err := scheduler.EnqueueJob(context.Background(), execReq)

	assert.Nil(t, decision, "decision should be nil when queued")
	assert.Equal(t, ErrAllRuntimesBusy, err, "should return ErrAllRuntimesBusy when no runtimes match")
	assert.Equal(t, 1, scheduler.GetQueueLength(), "job should be in queue")

	// Verify the job is in the queue
	queued := scheduler.GetQueuedJobs()
	assert.Equal(t, 1, len(queued), "should have 1 queued job")
	assert.Equal(t, "job-1", queued[0].ExecReq.FullJob.Id, "queued job should be correct")
}

func TestScheduler_EnqueueJobSchedulesImmediatelyWhenAvailable(t *testing.T) {
	// Create a mock runtime
	mockRuntime := &MockLocalRuntime{id: "runtime-1"}

	runtimeManager := NewMockRuntimeManager()
	runtimeManager.localRuntimes = []runtime.Runtime{mockRuntime}
	runtimeManager.allRuntimes = []*v1.Runtime{&v1.Runtime{
		Id:        "runtime-1",
		Platform:  v1.RuntimePlatform_RUNTIME_PLATFORM_NATIVE_LINUX,
		Toolchain: v1.RuntimeToolchain_RUNTIME_TOOLCHAIN_CPP,
	}}

	heuristic := NewSimpleLocalFirstHeuristic()

	config := &SchedulerConfig{
		Heuristic:      heuristic,
		RuntimeManager: runtimeManager,
		PeerManager:    &peers.Manager{},
		LocalDaemonID:  "daemon-1",
	}

	scheduler, err := NewScheduler(config)
	require.NoError(t, err, "should create scheduler")

	// Create a job with compatible runtime requirement
	job := &v1.Job{
		Id: "job-1",
		RuntimeRequirement: &v1.Job_Runtime{
			Runtime: &v1.Runtime{
				Id:        "runtime-1",
				Platform:  v1.RuntimePlatform_RUNTIME_PLATFORM_NATIVE_LINUX,
				Toolchain: v1.RuntimeToolchain_RUNTIME_TOOLCHAIN_CPP,
			},
		},
	}

	// EnqueueJob should schedule the job immediately
	execReq := &runtime.ExecutionRequest{FullJob: job}
	decision, err := scheduler.EnqueueJob(context.Background(), execReq)

	require.NoError(t, err, "should not error when runtimes are available")
	require.NotNil(t, decision, "decision should be returned")
	assert.Equal(t, "daemon-1", decision.PeerId, "should choose local execution")
	assert.Equal(t, 0, scheduler.GetQueueLength(), "job should not be queued")
}

func TestScheduler_TryScheduleNextRetries(t *testing.T) {
	// Create a mock runtime
	mockRuntime := &MockLocalRuntime{id: "runtime-1"}

	runtimeManager := NewMockRuntimeManager()
	runtimeManager.localRuntimes = []runtime.Runtime{mockRuntime}
	runtimeManager.allRuntimes = []*v1.Runtime{&v1.Runtime{
		Id:        "runtime-1",
		Platform:  v1.RuntimePlatform_RUNTIME_PLATFORM_NATIVE_LINUX,
		Toolchain: v1.RuntimeToolchain_RUNTIME_TOOLCHAIN_CPP,
	}}

	heuristic := NewSimpleLocalFirstHeuristic()

	config := &SchedulerConfig{
		Heuristic:      heuristic,
		RuntimeManager: runtimeManager,
		PeerManager:    &peers.Manager{},
		LocalDaemonID:  "daemon-1",
	}

	scheduler, err := NewScheduler(config)
	require.NoError(t, err, "should create scheduler")

	// Manually add a job to the queue
	job := &v1.Job{
		Id: "job-1",
		RuntimeRequirement: &v1.Job_Runtime{
			Runtime: &v1.Runtime{
				Id:        "runtime-1",
				Platform:  v1.RuntimePlatform_RUNTIME_PLATFORM_NATIVE_LINUX,
				Toolchain: v1.RuntimeToolchain_RUNTIME_TOOLCHAIN_CPP,
			},
		},
	}

	queuedJob := &QueuedJob{
		ExecReq:    &runtime.ExecutionRequest{FullJob: job},
		EnqueuedAt: getUnixMillis(),
	}

	scheduler.queue.Enqueue(queuedJob)
	assert.Equal(t, 1, scheduler.GetQueueLength(), "job should be in queue")

	// TryScheduleNext should successfully schedule the job
	success := scheduler.TryScheduleNext(context.Background())

	assert.True(t, success, "should successfully schedule queued job")
	assert.Equal(t, 0, scheduler.GetQueueLength(), "queue should be empty after scheduling")
}

func TestScheduler_RemoveQueuedJob(t *testing.T) {
	scheduler, err := NewScheduler(&SchedulerConfig{
		Heuristic:      NewSimpleLocalFirstHeuristic(),
		RuntimeManager: NewMockRuntimeManager(),
		PeerManager:    &peers.Manager{},
		LocalDaemonID:  "daemon-1",
	})
	require.NoError(t, err, "should create scheduler")

	// Add jobs to queue
	for i := 1; i <= 3; i++ {
		job := &v1.Job{Id: fmt.Sprintf("job-%d", i)}
		queuedJob := &QueuedJob{
			ExecReq:    &runtime.ExecutionRequest{FullJob: job},
			EnqueuedAt: getUnixMillis(),
		}
		scheduler.queue.Enqueue(queuedJob)
	}

	assert.Equal(t, 3, scheduler.GetQueueLength(), "should have 3 jobs queued")

	// Remove a job
	removed := scheduler.RemoveQueuedJob("job-2")

	assert.True(t, removed, "should successfully remove job-2")
	assert.Equal(t, 2, scheduler.GetQueueLength(), "should have 2 jobs queued")

	queued := scheduler.GetQueuedJobs()
	assert.Equal(t, "job-1", queued[0].ExecReq.FullJob.Id, "job-1 should remain")
	assert.Equal(t, "job-3", queued[1].ExecReq.FullJob.Id, "job-3 should remain")
}
