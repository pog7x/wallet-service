package main

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"go.uber.org/goleak"
)

// runRoundRobinDispatcherWithTimeout runs roundRobinDispatcher in a separate
// goroutine and fails the test if it does not return within timeout. Without
// the wrapper a deadlock would hang the whole package until the go test
// timeout fires instead of producing a failing test.
func runRoundRobinDispatcherWithTimeout(t *testing.T, k, n int, timeout time.Duration) string {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	var buf bytes.Buffer
	done := make(chan struct{})
	go func() {
		roundRobinDispatcher(ctx, &buf, k, n)
		close(done)
	}()

	select {
	case <-done:
		cancel()
		return buf.String()
	case <-time.After(timeout):
		cancel()
		t.Fatalf("roundRobinDispatcher(k=%d, n=%d) did not finish within %s", k, n, timeout)
		return ""
	}
}

// TestRoundRobinDispatcher_MatchesReference checks that the dispatcher prints
// exactly the same sequence as the reference formula. The order must be
// identical on every run, so the test is meaningful only when executed
// repeatedly: run it with -race -count=100.
func TestRoundRobinDispatcher_MatchesReference(t *testing.T) {
	defer goleak.VerifyNone(t)

	cases := []struct {
		k, n int
	}{
		{7, 1},
		{3, 5},
		{5, 12},
		{7, 30},
		{10, 100},
		{100, 100},
		{1, 1},
		{1, 3},
	}

	for _, tc := range cases {
		t.Run(fmt.Sprintf("k=%d_n=%d", tc.k, tc.n), func(t *testing.T) {
			got := parseRoundRobinOutput(runRoundRobinDispatcherWithTimeout(t, tc.k, tc.n, 5*time.Second))
			assertRoundRobinOutput(t, got, tc.k, tc.n)
		})
	}
}

func TestRoundRobinDispatcher_EdgeCases(t *testing.T) {
	defer goleak.VerifyNone(t)

	t.Run("invalid k returns immediately", func(t *testing.T) {
		if got := runRoundRobinDispatcherWithTimeout(t, 0, 10, time.Second); got != "" {
			t.Fatalf("got %q, want empty output", got)
		}
	})

	t.Run("invalid n returns immediately", func(t *testing.T) {
		if got := runRoundRobinDispatcherWithTimeout(t, 3, 0, time.Second); got != "" {
			t.Fatalf("got %q, want empty output", got)
		}
	})

	t.Run("k=1 single worker serves every number", func(t *testing.T) {
		got := parseRoundRobinOutput(runRoundRobinDispatcherWithTimeout(t, 1, 5, 2*time.Second))
		assertRoundRobinOutput(t, got, 1, 5)
	})

	t.Run("k>n leaves surplus workers unused", func(t *testing.T) {
		got := parseRoundRobinOutput(runRoundRobinDispatcherWithTimeout(t, 9, 4, 2*time.Second))
		assertRoundRobinOutput(t, got, 9, 4)
	})
}

// TestRoundRobinDispatcher_CancelMidRun checks that cancelling the context
// unblocks the dispatcher itself, not only its workers. The dispatcher loop
// performs two blocking operations, the send into chArr and the receive from
// output, and both must observe cancellation: once the workers have returned
// there is nobody left to meet either operation.
func TestRoundRobinDispatcher_CancelMidRun(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	k, n := 7, 1_000_000
	done := make(chan struct{})
	go func() {
		var buf bytes.Buffer
		roundRobinDispatcher(ctx, &buf, k, n)
		close(done)
	}()

	// Let the pipeline reach a steady state before cancelling, so the
	// cancellation lands in the middle of the run rather than before it.
	time.Sleep(10 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("roundRobinDispatcher did not stop after context cancellation")
	}
}

// TestRoundRobinDispatcher_CancelBeforeStart checks the degenerate case where
// the context is already cancelled when the dispatcher is called.
func TestRoundRobinDispatcher_CancelBeforeStart(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan struct{})
	go func() {
		var buf bytes.Buffer
		roundRobinDispatcher(ctx, &buf, 3, 100)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("roundRobinDispatcher did not return for an already cancelled context")
	}
}
