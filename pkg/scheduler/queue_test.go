package scheduler

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1 "github.com/Manu343726/buildozer/internal/gen/buildozer/proto/v1"
	"github.com/Manu343726/buildozer/pkg/runtime"
)

func TestJobQueue_EnqueueAndDequeue(t *testing.T) {
	queue := NewJobQueue()

	job1 := &v1.Job{Id: "job-1"}
	job2 := &v1.Job{Id: "job-2"}

	qj1 := &QueuedJob{ExecReq: &runtime.ExecutionRequest{FullJob: job1}, EnqueuedAt: time.Now().UnixMilli()}
	qj2 := &QueuedJob{ExecReq: &runtime.ExecutionRequest{FullJob: job2}, EnqueuedAt: time.Now().UnixMilli()}

	// Enqueue two jobs
	queue.Enqueue(qj1)
	queue.Enqueue(qj2)

	assert.Equal(t, 2, queue.Len(), "queue should have 2 jobs")

	// Dequeue first job
	dequeued := queue.Dequeue()
	require.NotNil(t, dequeued, "first dequeue should return a job")
	assert.Equal(t, job1.Id, dequeued.ExecReq.FullJob.Id, "should dequeue jobs in FIFO order")

	// Dequeue second job
	dequeued = queue.Dequeue()
	require.NotNil(t, dequeued, "second dequeue should return a job")
	assert.Equal(t, job2.Id, dequeued.ExecReq.FullJob.Id, "should dequeue jobs in FIFO order")

	// Dequeue from empty queue
	dequeued = queue.Dequeue()
	assert.Nil(t, dequeued, "dequeue from empty queue should return nil")
}

func TestJobQueue_Peek(t *testing.T) {
	queue := NewJobQueue()

	job := &v1.Job{Id: "job-1"}
	qj := &QueuedJob{ExecReq: &runtime.ExecutionRequest{FullJob: job}, EnqueuedAt: time.Now().UnixMilli()}

	// Peek at empty queue
	peeked := queue.Peek()
	assert.Nil(t, peeked, "peek at empty queue should return nil")

	// Enqueue and peek
	queue.Enqueue(qj)
	peeked = queue.Peek()
	require.NotNil(t, peeked, "peek should return the job")
	assert.Equal(t, job.Id, peeked.ExecReq.FullJob.Id, "peeked job should be correct")

	// Peek again without dequeue
	peeked2 := queue.Peek()
	require.NotNil(t, peeked2, "peek again should return the same job")
	assert.Equal(t, job.Id, peeked2.ExecReq.FullJob.Id, "peeked job should still be there")

	assert.Equal(t, 1, queue.Len(), "queue length should still be 1 after peek")
}

func TestJobQueue_Remove(t *testing.T) {
	queue := NewJobQueue()

	job1 := &v1.Job{Id: "job-1"}
	job2 := &v1.Job{Id: "job-2"}
	job3 := &v1.Job{Id: "job-3"}

	qj1 := &QueuedJob{ExecReq: &runtime.ExecutionRequest{FullJob: job1}, EnqueuedAt: time.Now().UnixMilli()}
	qj2 := &QueuedJob{ExecReq: &runtime.ExecutionRequest{FullJob: job2}, EnqueuedAt: time.Now().UnixMilli()}
	qj3 := &QueuedJob{ExecReq: &runtime.ExecutionRequest{FullJob: job3}, EnqueuedAt: time.Now().UnixMilli()}

	queue.Enqueue(qj1)
	queue.Enqueue(qj2)
	queue.Enqueue(qj3)

	assert.Equal(t, 3, queue.Len(), "queue should have 3 jobs")

	// Remove middle job
	removed := queue.Remove("job-2")
	assert.True(t, removed, "should successfully remove job-2")
	assert.Equal(t, 2, queue.Len(), "queue should have 2 jobs after removal")

	// Verify remaining jobs are in correct order
	first := queue.Dequeue()
	assert.Equal(t, job1.Id, first.ExecReq.FullJob.Id, "first job should still be job-1")

	second := queue.Dequeue()
	assert.Equal(t, job3.Id, second.ExecReq.FullJob.Id, "second job should be job-3")

	// Try to remove non-existent job
	removed = queue.Remove("job-2")
	assert.False(t, removed, "should return false for non-existent job")
}

func TestJobQueue_Clear(t *testing.T) {
	queue := NewJobQueue()

	for i := 1; i <= 5; i++ {
		job := &v1.Job{Id: "job-" + string(rune('0'+i))}
		qj := &QueuedJob{ExecReq: &runtime.ExecutionRequest{FullJob: job}, EnqueuedAt: time.Now().UnixMilli()}
		queue.Enqueue(qj)
	}

	assert.Equal(t, 5, queue.Len(), "queue should have 5 jobs")

	queue.Clear()

	assert.Equal(t, 0, queue.Len(), "queue should be empty after clear")
	assert.Nil(t, queue.Peek(), "peek should return nil after clear")
}

func TestJobQueue_GetAll(t *testing.T) {
	queue := NewJobQueue()

	job1 := &v1.Job{Id: "job-1"}
	job2 := &v1.Job{Id: "job-2"}

	qj1 := &QueuedJob{ExecReq: &runtime.ExecutionRequest{FullJob: job1}, EnqueuedAt: time.Now().UnixMilli()}
	qj2 := &QueuedJob{ExecReq: &runtime.ExecutionRequest{FullJob: job2}, EnqueuedAt: time.Now().UnixMilli()}

	queue.Enqueue(qj1)
	queue.Enqueue(qj2)

	all := queue.GetAll()
	assert.Equal(t, 2, len(all), "GetAll should return all jobs")
	assert.Equal(t, job1.Id, all[0].ExecReq.FullJob.Id, "first job should be job-1")
	assert.Equal(t, job2.Id, all[1].ExecReq.FullJob.Id, "second job should be job-2")

	// Verify it's a copy - modifying the returned slice shouldn't affect queue
	all = append(all, &QueuedJob{ExecReq: &runtime.ExecutionRequest{FullJob: &v1.Job{Id: "job-3"}}})
	assert.Equal(t, 2, queue.Len(), "queue should still have 2 jobs")
}

func TestJobQueue_NilHandling(t *testing.T) {
	queue := NewJobQueue()

	// Enqueueing nil should not crash
	queue.Enqueue(nil)
	assert.Equal(t, 0, queue.Len(), "queue should remain empty when nil is enqueued")

	// Dequeue from empty queue should return nil
	dequeued := queue.Dequeue()
	assert.Nil(t, dequeued, "dequeue from empty queue should return nil")
}

func TestJobQueue_ConcurrentOperations(t *testing.T) {
	queue := NewJobQueue()

	// Enqueue jobs concurrently
	done := make(chan bool)
	for i := 1; i <= 10; i++ {
		go func(id int) {
			job := &v1.Job{Id: "job-" + string(rune('0'+id))}
			qj := &QueuedJob{ExecReq: &runtime.ExecutionRequest{FullJob: job}, EnqueuedAt: time.Now().UnixMilli()}
			queue.Enqueue(qj)
			done <- true
		}(i)
	}

	// Wait for all enqueues
	for i := 0; i < 10; i++ {
		<-done
	}

	assert.Equal(t, 10, queue.Len(), "queue should have 10 jobs after concurrent enqueues")

	// Dequeue all
	count := 0
	for queue.Len() > 0 {
		queue.Dequeue()
		count++
	}
	assert.Equal(t, 10, count, "should have dequeued all 10 jobs")
}
