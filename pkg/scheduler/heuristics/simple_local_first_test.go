package heuristics

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	"github.com/Manu343726/buildozer/pkg/runtime"
)

// mockRuntimeManager simulates a runtime manager for testing
type mockRuntimeManager struct {
	localRuntimeIDs []*v1.Runtime // Local runtimes (visible by ID lookup)
	allRuntimes     []*v1.Runtime // All runtimes (visible by Match)
}

func (m *mockRuntimeManager) ListRuntimes(ctx context.Context) ([]*v1.Runtime, string, error) {
	return m.allRuntimes, "test", nil
}

func (m *mockRuntimeManager) GetRuntimes(ctx context.Context) ([]runtime.Runtime, string, error) {
	// Return local runtimes as mock implementations
	var result []runtime.Runtime
	localIDMap := make(map[string]bool)
	for _, rt := range m.localRuntimeIDs {
		localIDMap[rt.Id] = true
	}
	for _, rt := range m.allRuntimes {
		if localIDMap[rt.Id] {
			result = append(result, &mockRuntime{proto: rt})
		}
	}
	return result, "test", nil
}

func (m *mockRuntimeManager) GetRuntimeByID(ctx context.Context, runtimeID string) (runtime.Runtime, error) {
	// Check if runtime is in local runtimes
	for _, rt := range m.localRuntimeIDs {
		if rt.Id == runtimeID {
			return &mockRuntime{proto: rt}, nil
		}
	}
	return nil, fmt.Errorf("runtime not found: %s", runtimeID)
}

func (m *mockRuntimeManager) Match(ctx context.Context, query *v1.RuntimeMatchQuery) ([]*v1.Runtime, error) {
	var matches []*v1.Runtime
	for _, rt := range m.allRuntimes {
		if runtimeMatchesQuery(rt, query) {
			matches = append(matches, rt)
		}
	}
	return matches, nil
}

// mockRuntime implements runtime.Runtime interface for testing
type mockRuntime struct {
	proto *v1.Runtime
}

func (m *mockRuntime) Execute(ctx context.Context, req *runtime.ExecutionRequest) (*runtime.ExecutionResult, error) {
	return nil, fmt.Errorf("not implemented for mock")
}

func (m *mockRuntime) Available(ctx context.Context) (bool, error) {
	return true, nil
}

func (m *mockRuntime) Metadata(ctx context.Context) (*runtime.Metadata, error) {
	return nil, fmt.Errorf("not implemented for mock")
}

func (m *mockRuntime) Proto(ctx context.Context) (*v1.Runtime, error) {
	return m.proto, nil
}

func (m *mockRuntime) RuntimeID() string {
	return m.proto.Id
}

func (m *mockRuntime) MatchesQuery(ctx context.Context, query *v1.RuntimeMatchQuery) (bool, error) {
	return runtimeMatchesQuery(m.proto, query), nil
}

func (m *mockRuntime) Close() error {
	return nil
}

// runtimeMatchesQuery checks if a runtime matches all constraints in the query
func runtimeMatchesQuery(rt *v1.Runtime, query *v1.RuntimeMatchQuery) bool {
	// Check platform constraint
	if len(query.Platforms) > 0 {
		platformMatches := false
		for _, platform := range query.Platforms {
			if rt.Platform == platform {
				platformMatches = true
				break
			}
		}
		if !platformMatches {
			return false
		}
	}

	// Check toolchain constraint
	if len(query.Toolchains) > 0 {
		toolchainMatches := false
		for _, tc := range query.Toolchains {
			if rt.Toolchain == tc {
				toolchainMatches = true
				break
			}
		}
		if !toolchainMatches {
			return false
		}
	}

	// Check parameter constraints - this is a simplified version
	// In production, use the runtime manager's Match() method which has full logic
	if len(query.Params) > 0 {
		// For now, if params are specified but this is a simple matching,
		// we return true (params are complex and vary by toolchain)
		// The actual runtime managers handle full parameter matching
		return true
	}

	return true
}

func TestSimpleLocalFirstHeuristic_ScheduleWithExplicitLocalRuntime(t *testing.T) {
	heuristic := NewSimpleLocalFirstHeuristic()

	// Create a job with explicit local runtime
	runtime := &v1.Runtime{
		Id:        "runtime-1",
		Platform:  v1.RuntimePlatform_RUNTIME_PLATFORM_NATIVE_LINUX,
		Toolchain: v1.RuntimeToolchain_RUNTIME_TOOLCHAIN_CPP,
	}

	job := &v1.Job{
		Id:                 "job-1",
		RuntimeRequirement: &v1.Job_Runtime{Runtime: runtime},
	}

	mockManager := &mockRuntimeManager{
		localRuntimeIDs: []*v1.Runtime{runtime},
		allRuntimes:     []*v1.Runtime{runtime},
	}

	decision := &SchedulingContext{
		Job:            job,
		RuntimeManager: mockManager,
		LocalDaemonID:  "daemon-1",
		JobCount:       1,
	}

	result, err := heuristic.Schedule(context.Background(), decision)

	require.NoError(t, err, "heuristic should not error")
	assert.Equal(t, "daemon-1", result.PeerId, "should execute job locally")
	assert.Equal(t, runtime, result.SelectedRuntime, "should use the same runtime")
	assert.Contains(t, result.Reason, "local", "reason should mention local execution")
}

func TestSimpleLocalFirstHeuristic_ScheduleWithExplicitRemoteRuntime(t *testing.T) {
	heuristic := NewSimpleLocalFirstHeuristic()

	// Create a job with explicit remote runtime
	remoteRuntime := &v1.Runtime{
		Id:        "remote-runtime-1",
		Platform:  v1.RuntimePlatform_RUNTIME_PLATFORM_NATIVE_LINUX,
		Toolchain: v1.RuntimeToolchain_RUNTIME_TOOLCHAIN_CPP,
		PeerIds:   []string{"peer-1"}, // Add peer ID for remote runtime
	}

	localRuntime := &v1.Runtime{
		Id:        "local-runtime-1",
		Platform:  v1.RuntimePlatform_RUNTIME_PLATFORM_NATIVE_LINUX,
		Toolchain: v1.RuntimeToolchain_RUNTIME_TOOLCHAIN_CPP,
	}

	job := &v1.Job{
		Id:                 "job-1",
		RuntimeRequirement: &v1.Job_Runtime{Runtime: remoteRuntime},
	}

	mockManager := &mockRuntimeManager{
		localRuntimeIDs: []*v1.Runtime{localRuntime}, // Only local-runtime-1 is local
		allRuntimes:     []*v1.Runtime{remoteRuntime, localRuntime},
	}

	decision := &SchedulingContext{
		Job:            job,
		RuntimeManager: mockManager,
		LocalDaemonID:  "daemon-1",
		JobCount:       1,
	}

	result, err := heuristic.Schedule(context.Background(), decision)

	require.NoError(t, err, "heuristic should not error")
	assert.NotEqual(t, "daemon-1", result.PeerId, "should execute job remotely")
	assert.Equal(t, remoteRuntime, result.SelectedRuntime, "should use the specified remote runtime")
}

func TestSimpleLocalFirstHeuristic_ScheduleWithQueryPreferLocal(t *testing.T) {
	heuristic := NewSimpleLocalFirstHeuristic()

	localRuntime := &v1.Runtime{
		Id:        "local-runtime-1",
		Platform:  v1.RuntimePlatform_RUNTIME_PLATFORM_NATIVE_LINUX,
		Toolchain: v1.RuntimeToolchain_RUNTIME_TOOLCHAIN_CPP,
		ToolchainSpec: &v1.Runtime_Cpp{
			Cpp: &v1.CppToolchain{
				Compiler: v1.CppCompiler_CPP_COMPILER_GCC,
			},
		},
	}

	remoteRuntime := &v1.Runtime{
		Id:        "remote-runtime-1",
		Platform:  v1.RuntimePlatform_RUNTIME_PLATFORM_NATIVE_LINUX,
		Toolchain: v1.RuntimeToolchain_RUNTIME_TOOLCHAIN_CPP,
		ToolchainSpec: &v1.Runtime_Cpp{
			Cpp: &v1.CppToolchain{
				Compiler: v1.CppCompiler_CPP_COMPILER_CLANG,
			},
		},
	}

	query := &v1.RuntimeMatchQuery{
		Platforms: []v1.RuntimePlatform{v1.RuntimePlatform_RUNTIME_PLATFORM_NATIVE_LINUX},
	}

	job := &v1.Job{
		Id:                 "job-1",
		RuntimeRequirement: &v1.Job_RuntimeMatchQuery{RuntimeMatchQuery: query},
	}

	mockManager := &mockRuntimeManager{
		localRuntimeIDs: []*v1.Runtime{localRuntime},
		allRuntimes:     []*v1.Runtime{localRuntime, remoteRuntime},
	}

	decision := &SchedulingContext{
		Job:            job,
		RuntimeManager: mockManager,
		LocalDaemonID:  "daemon-1",
		JobCount:       1,
	}

	result, err := heuristic.Schedule(context.Background(), decision)

	require.NoError(t, err, "heuristic should not error")
	assert.Equal(t, "daemon-1", result.PeerId, "should execute job locally when match available")
	assert.Equal(t, localRuntime.Id, result.SelectedRuntime.Id, "should select local runtime")
}

func TestSimpleLocalFirstHeuristic_ScheduleWithQueryNoLocalMatch(t *testing.T) {
	heuristic := NewSimpleLocalFirstHeuristic()

	remoteRuntime := &v1.Runtime{
		Id:        "remote-runtime-1",
		Platform:  v1.RuntimePlatform_RUNTIME_PLATFORM_NATIVE_LINUX,
		Toolchain: v1.RuntimeToolchain_RUNTIME_TOOLCHAIN_CPP,
		PeerIds:   []string{"peer-1"}, // Add peer ID for remote runtime
	}

	query := &v1.RuntimeMatchQuery{
		Platforms: []v1.RuntimePlatform{v1.RuntimePlatform_RUNTIME_PLATFORM_NATIVE_LINUX},
	}

	job := &v1.Job{
		Id:                 "job-1",
		RuntimeRequirement: &v1.Job_RuntimeMatchQuery{RuntimeMatchQuery: query},
	}

	mockManager := &mockRuntimeManager{
		localRuntimeIDs: []*v1.Runtime{}, // No local runtimes
		allRuntimes:     []*v1.Runtime{remoteRuntime},
	}

	decision := &SchedulingContext{
		Job:            job,
		RuntimeManager: mockManager,
		LocalDaemonID:  "daemon-1",
		JobCount:       1,
	}

	result, err := heuristic.Schedule(context.Background(), decision)

	require.NoError(t, err, "heuristic should not error")
	assert.NotEqual(t, "daemon-1", result.PeerId, "should execute job remotely when no local match")
	assert.Equal(t, remoteRuntime.Id, result.SelectedRuntime.Id, "should select remote runtime")
	assert.Contains(t, result.Reason, "remote", "reason should mention remote execution")
}

func TestSimpleLocalFirstHeuristic_ErrorWithNilContext(t *testing.T) {
	heuristic := NewSimpleLocalFirstHeuristic()

	result, err := heuristic.Schedule(context.Background(), nil)

	assert.Error(t, err, "should error with nil scheduling context")
	assert.Nil(t, result, "result should be nil")
	assert.Equal(t, ErrNilSchedulingContext, err, "should return specific error")
}

func TestSimpleLocalFirstHeuristic_ErrorWithNilJob(t *testing.T) {
	heuristic := NewSimpleLocalFirstHeuristic()

	decision := &SchedulingContext{
		Job: nil,
	}

	result, err := heuristic.Schedule(context.Background(), decision)

	assert.Error(t, err, "should error with nil job")
	assert.Nil(t, result, "result should be nil")
	assert.Equal(t, ErrNilJob, err, "should return specific error")
}

func TestSimpleLocalFirstHeuristic_ErrorWithNoRuntimeRequirement(t *testing.T) {
	heuristic := NewSimpleLocalFirstHeuristic()

	job := &v1.Job{
		Id: "job-1",
		// No RuntimeRequirement set
	}

	mockManager := &mockRuntimeManager{
		localRuntimeIDs: []*v1.Runtime{},
		allRuntimes:     []*v1.Runtime{},
	}

	decision := &SchedulingContext{
		Job:            job,
		RuntimeManager: mockManager,
		LocalDaemonID:  "daemon-1",
	}

	result, err := heuristic.Schedule(context.Background(), decision)

	assert.Error(t, err, "should error with no runtime requirement")
	assert.Nil(t, result, "result should be nil")
	assert.Equal(t, ErrNoRuntimeRequirement, err, "should return specific error")
}

func TestSimpleLocalFirstHeuristic_ErrorWithNoMatchingRuntimes(t *testing.T) {
	heuristic := NewSimpleLocalFirstHeuristic()

	// Query that won't match any runtimes
	query := &v1.RuntimeMatchQuery{
		Platforms: []v1.RuntimePlatform{v1.RuntimePlatform_RUNTIME_PLATFORM_DOCKER_LINUX},
	}

	job := &v1.Job{
		Id:                 "job-1",
		RuntimeRequirement: &v1.Job_RuntimeMatchQuery{RuntimeMatchQuery: query},
	}

	// Available runtime doesn't match the query
	runtime := &v1.Runtime{
		Id:        "runtime-1",
		Platform:  v1.RuntimePlatform_RUNTIME_PLATFORM_NATIVE_LINUX,
		Toolchain: v1.RuntimeToolchain_RUNTIME_TOOLCHAIN_CPP,
	}

	mockManager := &mockRuntimeManager{
		localRuntimeIDs: []*v1.Runtime{runtime},
		allRuntimes:     []*v1.Runtime{runtime},
	}

	decision := &SchedulingContext{
		Job:            job,
		RuntimeManager: mockManager,
		LocalDaemonID:  "daemon-1",
	}

	result, err := heuristic.Schedule(context.Background(), decision)

	assert.Error(t, err, "should error with no matching runtimes")
	assert.Nil(t, result, "result should be nil")
	assert.Equal(t, ErrNoMatchingRuntimes, err, "should return specific error")
}

func TestSimpleLocalFirstHeuristic_Name(t *testing.T) {
	heuristic := NewSimpleLocalFirstHeuristic()

	name := heuristic.Name()

	assert.Equal(t, "SimpleLocalFirst", name, "name should match")
}
