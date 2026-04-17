package scheduler

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	"github.com/Manu343726/buildozer/pkg/peers"
)

func TestRemoteExecutor_ErrorWithNilPeerManager(t *testing.T) {
	executor := NewRemoteExecutor(nil)

	job := &v1.Job{Id: "job-1"}

	remoteJobID, progressChan, err := executor.ExecuteRemote(context.Background(), "peer-1", job)

	assert.Error(t, err, "should error with nil peer manager")
	assert.Empty(t, remoteJobID, "remote job ID should be empty")
	assert.Nil(t, progressChan, "progress channel should be nil")
}

func TestRemoteExecutor_ErrorWithNonexistentPeer(t *testing.T) {
	peerManager := peers.NewManager()
	executor := NewRemoteExecutor(peerManager)

	job := &v1.Job{Id: "job-1"}

	remoteJobID, progressChan, err := executor.ExecuteRemote(context.Background(), "nonexistent-peer", job)

	assert.Error(t, err, "should error with nonexistent peer")
	assert.Empty(t, remoteJobID, "remote job ID should be empty")
	assert.Nil(t, progressChan, "progress channel should be nil")
	assert.Contains(t, err.Error(), "peer not found", "error should indicate peer not found")
}

func TestRemoteExecutor_ErrorWithEmptyPeerID(t *testing.T) {
	peerManager := peers.NewManager()
	executor := NewRemoteExecutor(peerManager)

	job := &v1.Job{Id: "job-1"}

	remoteJobID, progressChan, err := executor.ExecuteRemote(context.Background(), "", job)

	assert.Error(t, err, "should error with empty peer ID")
	assert.Empty(t, remoteJobID, "remote job ID should be empty")
	assert.Nil(t, progressChan, "progress channel should be nil")
}

func TestRemoteExecutor_ErrorWithNilJob(t *testing.T) {
	peerManager := peers.NewManager()
	executor := NewRemoteExecutor(peerManager)

	remoteJobID, progressChan, err := executor.ExecuteRemote(context.Background(), "peer-1", nil)

	assert.Error(t, err, "should error with nil job")
	assert.Empty(t, remoteJobID, "remote job ID should be empty")
	assert.Nil(t, progressChan, "progress channel should be nil")
}

func TestRemoteExecutor_CancelRemoteJobErrorWithNilPeerManager(t *testing.T) {
	executor := NewRemoteExecutor(nil)

	err := executor.CancelRemoteJob(context.Background(), "peer-1", "remote-job-1", "user cancelled")

	assert.Error(t, err, "should error with nil peer manager")
}

func TestRemoteExecutor_CancelRemoteJobErrorWithNonexistentPeer(t *testing.T) {
	peerManager := peers.NewManager()
	executor := NewRemoteExecutor(peerManager)

	err := executor.CancelRemoteJob(context.Background(), "nonexistent-peer", "remote-job-1", "reason")

	assert.Error(t, err, "should error with nonexistent peer")
	assert.Contains(t, err.Error(), "peer not found", "error should indicate peer not found")
}

func TestRemoteExecutor_ErrorCancelWithEmptyPeerID(t *testing.T) {
	peerManager := peers.NewManager()
	executor := NewRemoteExecutor(peerManager)

	err := executor.CancelRemoteJob(context.Background(), "", "remote-job-1", "reason")

	assert.Error(t, err, "should error with empty peer ID")
}

func TestRemoteExecutor_ErrorCancelWithEmptyRemoteJobID(t *testing.T) {
	peerManager := peers.NewManager()
	executor := NewRemoteExecutor(peerManager)

	err := executor.CancelRemoteJob(context.Background(), "peer-1", "", "reason")

	assert.Error(t, err, "should error with empty remote job ID")
}
