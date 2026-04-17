package scheduler

import (
	"sync"

	"github.com/Manu343726/buildozer/pkg/logging"
	"github.com/Manu343726/buildozer/pkg/runtime"
)

// QueuedJob represents a job waiting in the scheduling queue
type QueuedJob struct {
	// ExecReq is the execution request containing the job and progress callback
	ExecReq *runtime.ExecutionRequest

	// EnqueuedAt is the Unix millisecond timestamp when the job was enqueued
	EnqueuedAt int64
}

// JobQueue stores pending jobs waiting to be scheduled
type JobQueue struct {
	mu     sync.Mutex
	jobs   []*QueuedJob
	logger *logging.Logger
}

// NewJobQueue creates a new job queue
func NewJobQueue() *JobQueue {
	return &JobQueue{
		jobs:   make([]*QueuedJob, 0),
		logger: LogSubsystem("JobQueue"),
	}
}

// Enqueue adds a job to the queue
func (q *JobQueue) Enqueue(qj *QueuedJob) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if qj == nil {
		return
	}

	q.jobs = append(q.jobs, qj)
	q.logger.Debug("Job enqueued",
		"jobID", qj.ExecReq.FullJob.Id,
		"queueLen", len(q.jobs),
	)
}

// Dequeue removes and returns the first job from the queue
// Returns nil if queue is empty
func (q *JobQueue) Dequeue() *QueuedJob {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.jobs) == 0 {
		return nil
	}

	qj := q.jobs[0]
	q.jobs = q.jobs[1:]

	q.logger.Debug("Job dequeued",
		"jobID", qj.ExecReq.FullJob.Id,
		"remainingInQueue", len(q.jobs),
	)

	return qj
}

// Peek returns the first job without removing it
// Returns nil if queue is empty
func (q *JobQueue) Peek() *QueuedJob {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.jobs) == 0 {
		return nil
	}

	return q.jobs[0]
}

// Len returns the number of jobs in the queue
func (q *JobQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.jobs)
}

// Remove removes a specific job from the queue by ID
// Returns true if job was found and removed
func (q *JobQueue) Remove(jobID string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	for i, qj := range q.jobs {
		if qj.ExecReq.FullJob.Id == jobID {
			q.jobs = append(q.jobs[:i], q.jobs[i+1:]...)
			q.logger.Debug("Job removed from queue",
				"jobID", jobID,
				"remainingInQueue", len(q.jobs),
			)
			return true
		}
	}

	return false
}

// Clear empties the queue
func (q *JobQueue) Clear() {
	q.mu.Lock()
	defer q.mu.Unlock()

	count := len(q.jobs)
	q.jobs = q.jobs[:0]

	q.logger.Debug("Queue cleared", "removedCount", count)
}

// GetAll returns a copy of all jobs in the queue (in order)
func (q *JobQueue) GetAll() []*QueuedJob {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Return a copy to prevent external modifications
	result := make([]*QueuedJob, len(q.jobs))
	copy(result, q.jobs)
	return result
}
