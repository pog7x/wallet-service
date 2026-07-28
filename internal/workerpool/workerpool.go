package workerpool

import (
	"context"
	"sync"
	"sync/atomic"
)

// Job represents a function to be executed by a worker.
// It is called synchronously in the worker's goroutine.
type Job func()

// WorkerPool manages a fixed number of worker goroutines that process jobs
// from a queue. It is bound to a context that signals cancellation.
//
// Contract:
//   - Create a pool with New(). Workers are not started until Run() is called.
//   - Run() starts the workers. It is idempotent; extra calls have no effect.
//   - Jobs are submitted via Do(). Calling Do() after Wait() will panic (send on closed channel).
//   - Wait() closes the job queue and blocks until all workers have finished.
//     It must be called only after all jobs have been submitted.
//   - After Wait() returns, the pool is considered terminated; reuse is not allowed.
//
// On context cancellation:
//   - Workers exit as soon as they observe ctx.Done().
//   - Jobs that have already started run to completion.
//   - Jobs still waiting in the queue may remain unexecuted (if no worker picks them up before termination).
//   - Do() will not block and will drop the job if the context is already cancelled.
//   - Cancellation does not guarantee that all submitted jobs will be executed.
type WorkerPool struct {
	ctx       context.Context
	wg        sync.WaitGroup
	nWorkers  int
	jobQueue  chan Job
	isRunning atomic.Bool
	started   atomic.Int64
}

// New creates a new worker pool with the given number of workers.
// nWorkers must be >= 1 (if less, it is set to 1).
// The provided context controls the workers' lifecycle.
func New(ctx context.Context, nWorkers int) *WorkerPool {
	if nWorkers < 1 {
		nWorkers = 1
	}
	return &WorkerPool{
		ctx:      ctx,
		nWorkers: nWorkers,
		jobQueue: make(chan Job),
	}
}

// Run starts nWorkers goroutines that wait for jobs from the queue.
// Run is idempotent: the first call starts the workers, any later call is a no-op.
// Workers exit either when the queue is closed (by Wait()) or when the context is cancelled.
func (wp *WorkerPool) Run() {
	if !wp.isRunning.CompareAndSwap(false, true) {
		return
	}

	for range wp.nWorkers {
		wp.wg.Go(func() {
			wp.started.Add(1)
			for {
				select {
				case job, ok := <-wp.jobQueue:
					if !ok {
						return // queue closed
					}
					job()
				case <-wp.ctx.Done():
					return // context cancelled
				}
			}
		})
	}
}

// Do submits a job to the queue for execution.
// If the context is already cancelled, the job is ignored (not executed, not enqueued).
// Calling Do() after Wait() (i.e., after the queue is closed) will panic.
// Because the queue is unbuffered, Do() blocks until a worker picks up the job,
// unless the context is cancelled in the meantime.
// Returns nil on successful enqueue, or ctx.Err() if the context is cancelled
// before the job is sent.
func (wp *WorkerPool) Do(job Job) error {
	select {
	case wp.jobQueue <- job:
		return nil
	case <-wp.ctx.Done():
		// job is dropped on context cancellation
		return wp.ctx.Err()
	}
}

// Wait closes the job queue and waits for all workers to finish.
// This function must be called after all jobs have been submitted.
// After Wait() returns, the pool is finished; further calls to Do() are not allowed.
// If the context was cancelled before Wait() is called, workers will terminate,
// and any jobs left in the queue will remain unexecuted – Wait() still blocks until
// all worker goroutines have exited.
func (wp *WorkerPool) Wait() {
	close(wp.jobQueue)
	wp.wg.Wait()
}
