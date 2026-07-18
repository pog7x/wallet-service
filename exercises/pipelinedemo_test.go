package main

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"go.uber.org/goleak"
)

// sumOfSquares returns the closed-form sum of squares from 1 to n. Computing
// the expected value from a formula rather than from a second loop keeps the
// test independent of the implementation it verifies.
func sumOfSquares(n int) int {
	if n < 1 {
		return 0
	}
	return n * (n + 1) * (2*n + 1) / 6
}

// collectChan drains ch and returns everything it produced. It fails the test
// if the channel is not closed within timeout, which turns a stuck stage into
// a failing test rather than a hang.
func collectChan(t *testing.T, ch <-chan int, timeout time.Duration) []int {
	t.Helper()

	var (
		got  []int
		done = make(chan struct{})
	)
	go func() {
		defer close(done)
		for v := range ch {
			got = append(got, v)
		}
	}()

	select {
	case <-done:
		return got
	case <-time.After(timeout):
		t.Fatalf("channel was not closed within %s", timeout)
		return nil
	}
}

func TestGenerate_YieldsOneToN(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const n = 5
	got := collectChan(t, generate(ctx, n), 2*time.Second)

	if len(got) != n {
		t.Fatalf("got %d values %v, want %d", len(got), got, n)
	}
	for i, v := range got {
		if want := i + 1; v != want {
			t.Errorf("got[%d] = %d, want %d", i, v, want)
		}
	}
}

func TestGenerate_EmptyForNonPositiveN(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for _, n := range []int{0, -1} {
		t.Run(fmt.Sprintf("n=%d", n), func(t *testing.T) {
			if got := collectChan(t, generate(ctx, n), 2*time.Second); len(got) != 0 {
				t.Fatalf("got %v, want no values", got)
			}
		})
	}
}

func TestSqr_SquaresEveryValueInOrder(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const n = 6
	got := collectChan(t, sqr(ctx, generate(ctx, n)), 2*time.Second)

	if len(got) != n {
		t.Fatalf("got %d values %v, want %d", len(got), got, n)
	}
	for i, v := range got {
		if want := (i + 1) * (i + 1); v != want {
			t.Errorf("got[%d] = %d, want %d", i, v, want)
		}
	}
}

func TestPipelinedemo_PrintsSumOfSquares(t *testing.T) {
	defer goleak.VerifyNone(t)

	cases := []int{0, 1, 2, 10, 1000}

	for _, n := range cases {
		t.Run(fmt.Sprintf("n=%d", n), func(t *testing.T) {
			var buf bytes.Buffer
			pipelinedemo(&buf, n)

			want := fmt.Sprintf("%d\n", sumOfSquares(n))
			if got := buf.String(); got != want {
				t.Errorf("got %q, want %q", got, want)
			}
		})
	}
}

// TestPipeline_CancelMidRun_NoLeak cancels the context while values are still
// in flight and verifies that every stage returns. Each stage blocks on a send
// into its own output channel, so a stage without a cancellation branch would
// stay parked forever once the consumer stops reading; goleak is what turns
// that into a failure.
func TestPipeline_CancelMidRun_NoLeak(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctx, cancel := context.WithCancel(context.Background())

	out := sqr(ctx, generate(ctx, 1_000_000))

	// Consume a few values so the cancellation lands mid-stream.
	<-out
	<-out
	cancel()

	// Drain what is still buffered in transit; the stages close their outputs
	// on the way out, so this loop terminates once they have all returned.
	drained := make(chan struct{})
	var drainedCount int
	go func() {
		defer close(drained)
		for range out {
			drainedCount++
		}
	}()

	select {
	case <-drained:
		t.Logf("drained %d values after cancellation", drainedCount)
	case <-time.After(2 * time.Second):
		t.Fatal("pipeline did not shut down after cancellation")
	}
}

// TestPipeline_CancelBeforeConsuming_NoLeak cancels before a single value is
// read. The generator is then parked on its first send with no reader at all,
// which is the earliest point at which a missing cancellation branch leaks.
func TestPipeline_CancelBeforeConsuming_NoLeak(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctx, cancel := context.WithCancel(context.Background())

	out := sqr(ctx, generate(ctx, 1_000_000))
	cancel()

	drained := make(chan struct{})
	var drainedCount int
	go func() {
		defer close(drained)
		for range out {
			drainedCount++
		}
	}()

	select {
	case <-drained:
		t.Logf("drained %d values after cancellation", drainedCount)
	case <-time.After(2 * time.Second):
		t.Fatal("pipeline did not shut down after cancellation")
	}
}

// TestPipeline_ConsumerAbandons_NoLeak stops reading and cancels without
// draining. A stage parked on a send then has no reader at all, so only its
// cancellation branch can release it.
func TestPipeline_ConsumerAbandons_NoLeak(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctx, cancel := context.WithCancel(context.Background())

	out := sqr(ctx, generate(ctx, 1_000_000))

	<-out
	<-out

	cancel()
}
