package workerpool

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"go.uber.org/goleak"
)

func TestWorkerPool_ConcurrentIncomingJobs(t *testing.T) {
	defer goleak.VerifyNone(t)

	wp := New(t.Context(), 5)
	wp.Run()

	jobNum, addSum := int64(20), int64(200)
	x := atomic.Int64{}
	wg := sync.WaitGroup{}

	for range jobNum {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = wp.Do(func() {
				x.Add(addSum)
			})
		}()
	}

	wg.Wait()
	wp.Wait()

	if x.Load() != jobNum*addSum {
		t.Errorf("want = %d, got = %d", jobNum*addSum, int(x.Load()))
	}
}

func TestWorkerPool_CancelContextBeforeStart(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	wp := New(ctx, 5)
	wp.Run()

	x := atomic.Int64{}

	_ = wp.Do(func() { x.Add(99999) })

	wp.Wait()

	if x.Load() != 0 {
		t.Errorf("want = %d, got = %d", 0, x.Load())
	}
}

func TestWorkerPool_MultipleRunHasNoEffect(t *testing.T) {
	defer goleak.VerifyNone(t)

	wp := New(t.Context(), 5)

	wg := sync.WaitGroup{}
	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			wp.Run()
		}()
	}

	wg.Wait()
	wp.Wait()

	if int64(wp.nWorkers) != wp.started.Load() {
		t.Errorf("want = %d, got = %d", wp.nWorkers, wp.started.Load())
	}
}

func TestWorkerPool_CancelWhileRunning_NoJobs(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctx, cancel := context.WithCancel(context.Background())

	wp := New(ctx, 5)
	wp.Run()

	cancel()
}

func TestWorkerPool_CancelWhileRunning_WithJobs(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctx, cancel := context.WithCancel(context.Background())

	wp := New(ctx, 5)
	wp.Run()

	done := make(chan struct{})
	_ = wp.Do(func() { close(done) })
	<-done

	cancel()
}
