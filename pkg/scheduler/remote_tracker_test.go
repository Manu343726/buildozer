package scheduler

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
)

func TestRemoteTracker_TrackRemoteExecution(t *testing.T) {
	tracker := NewRemoteTracker()

	job := &v1.Job{
		Id: "job-1",
	}

	err := tracker.TrackRemoteExecution(context.Background(), "job-1", "peer-1", job)

	require.NoError(t, err, "should not error")

	// Verify tracking
	exec, err := tracker.GetRemoteExecution("job-1")
	require.NoError(t, err, "should retrieve remote execution")
	assert.Equal(t, "job-1", exec.JobID, "job ID should match")
	assert.Equal(t, "peer-1", exec.RemotePeerID, "peer ID should match")
}

func TestRemoteTracker_ErrorTrackingDuplicateJob(t *testing.T) {
	tracker := NewRemoteTracker()

	job := &v1.Job{Id: "job-1"}

	err1 := tracker.TrackRemoteExecution(context.Background(), "job-1", "peer-1", job)
	require.NoError(t, err1, "first tracking should succeed")

	err2 := tracker.TrackRemoteExecution(context.Background(), "job-1", "peer-2", job)
	assert.Error(t, err2, "should error when tracking duplicate job")
}

func TestRemoteTracker_UpdateRemoteStatus(t *testing.T) {
	tracker := NewRemoteTracker()

	job := &v1.Job{Id: "job-1"}

	err := tracker.TrackRemoteExecution(context.Background(), "job-1", "peer-1", job)
	require.NoError(t, err, "should track remote execution")

	// Update status
	newStatus := &v1.JobProgress{
		JobId:  "job-1",
		Status: v1.JobProgress_JOB_STATUS_RUNNING,
	}

	err = tracker.UpdateRemoteStatus(context.Background(), "job-1", newStatus)
	require.NoError(t, err, "should update status")

	// Verify
	status, err := tracker.GetRemoteStatus("job-1")
	require.NoError(t, err, "should retrieve status")
	assert.Equal(t, v1.JobProgress_JOB_STATUS_RUNNING, status.Status, "status should be updated")
}

func TestRemoteTracker_ErrorUpdateStatusNonexistent(t *testing.T) {
	tracker := NewRemoteTracker()

	newStatus := &v1.JobProgress{
		JobId:  "job-1",
		Status: v1.JobProgress_JOB_STATUS_RUNNING,
	}

	err := tracker.UpdateRemoteStatus(context.Background(), "job-1", newStatus)
	assert.Error(t, err, "should error when updating nonexistent job")
}

func TestRemoteTracker_GetRemoteExecution(t *testing.T) {
	tracker := NewRemoteTracker()

	job := &v1.Job{Id: "job-1"}
	err := tracker.TrackRemoteExecution(context.Background(), "job-1", "peer-1", job)
	require.NoError(t, err, "should track remote execution")

	exec, err := tracker.GetRemoteExecution("job-1")

	require.NoError(t, err, "should retrieve remote execution")
	assert.Equal(t, "job-1", exec.JobID, "job ID should match")
	assert.Equal(t, "peer-1", exec.RemotePeerID, "peer ID should match")
}

func TestRemoteTracker_ErrorGetNonexistentExecution(t *testing.T) {
	tracker := NewRemoteTracker()

	exec, err := tracker.GetRemoteExecution("job-1")

	assert.Error(t, err, "should error when getting nonexistent execution")
	assert.Nil(t, exec, "execution should be nil")
}

func TestRemoteTracker_IsRemoteJob(t *testing.T) {
	tracker := NewRemoteTracker()

	job := &v1.Job{Id: "job-1"}
	err := tracker.TrackRemoteExecution(context.Background(), "job-1", "peer-1", job)
	require.NoError(t, err, "should track remote execution")

	assert.True(t, tracker.IsRemoteJob("job-1"), "should identify remote job")
	assert.False(t, tracker.IsRemoteJob("job-2"), "should not identify nonexistent job as remote")
}

func TestRemoteTracker_StopTracking(t *testing.T) {
	tracker := NewRemoteTracker()

	job := &v1.Job{Id: "job-1"}
	err := tracker.TrackRemoteExecution(context.Background(), "job-1", "peer-1", job)
	require.NoError(t, err, "should track remote execution")

	// Verify it's tracked
	assert.True(t, tracker.IsRemoteJob("job-1"), "should be remote job")

	// Stop tracking
	err = tracker.StopTracking("job-1")
	require.NoError(t, err, "should stop tracking")

	// Verify it's no longer tracked
	assert.False(t, tracker.IsRemoteJob("job-1"), "should no longer be remote job")

	exec, err := tracker.GetRemoteExecution("job-1")
	assert.Error(t, err, "should error when getting stopped job")
	assert.Nil(t, exec, "execution should be nil")
}

func TestRemoteTracker_GetAllRemoteExecutions(t *testing.T) {
	tracker := NewRemoteTracker()

	job1 := &v1.Job{Id: "job-1"}
	job2 := &v1.Job{Id: "job-2"}
	job3 := &v1.Job{Id: "job-3"}

	err1 := tracker.TrackRemoteExecution(context.Background(), "job-1", "peer-1", job1)
	err2 := tracker.TrackRemoteExecution(context.Background(), "job-2", "peer-1", job2)
	err3 := tracker.TrackRemoteExecution(context.Background(), "job-3", "peer-2", job3)

	require.NoError(t, err1, "should track job 1")
	require.NoError(t, err2, "should track job 2")
	require.NoError(t, err3, "should track job 3")

	all := tracker.GetAllRemoteExecutions()

	assert.Len(t, all, 3, "should return all tracked executions")

	jobIDs := make(map[string]bool)
	for _, exec := range all {
		jobIDs[exec.JobID] = true
	}

	assert.True(t, jobIDs["job-1"], "should include job-1")
	assert.True(t, jobIDs["job-2"], "should include job-2")
	assert.True(t, jobIDs["job-3"], "should include job-3")
}
