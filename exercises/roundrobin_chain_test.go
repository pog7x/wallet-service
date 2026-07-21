package main

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"go.uber.org/goleak"
)

func expectedRoundRobinChain(k, n int) []string {
	if k < 1 || n < 1 {
		return nil
	}
	lines := make([]string, 0, n)
	for v := 1; v <= n; v++ {
		goroutine := (v - 1) % k
		lines = append(lines, fmt.Sprintf("goroutine %d: %d", goroutine, v))
	}
	return lines
}

func runRoundRobinChainWithTimeout(t *testing.T, k, n int, timeout time.Duration) string {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	var buf bytes.Buffer
	done := make(chan struct{})
	go func() {
		roundRobinChain(ctx, &buf, k, n)
		close(done)
	}()

	select {
	case <-done:
		cancel()
		return buf.String()
	case <-time.After(timeout):
		cancel()
		t.Fatalf("roundRobinChain(k=%d, n=%d) did not finish within %s", k, n, timeout)
		return ""
	}
}

func parseRoundRobinOutput(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func assertRoundRobinOutput(t *testing.T, got []string, k, n int) {
	t.Helper()
	want := expectedRoundRobinChain(k, n)
	if len(got) != len(want) {
		t.Fatalf("got %d lines, want %d\n got: %q\nwant: %q", len(got), len(want), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestRoundRobinChain_MatchesReference(t *testing.T) {
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
			got := parseRoundRobinOutput(runRoundRobinChainWithTimeout(t, tc.k, tc.n, 5*time.Second))
			assertRoundRobinOutput(t, got, tc.k, tc.n)
		})
	}
}

func TestRoundRobinChain_EdgeCases(t *testing.T) {
	defer goleak.VerifyNone(t)

	t.Run("invalid k returns immediately", func(t *testing.T) {
		got := runRoundRobinChainWithTimeout(t, 0, 10, time.Second)
		if got != "" {
			t.Fatalf("got %q, want empty output", got)
		}
	})

	t.Run("invalid n returns immediately", func(t *testing.T) {
		got := runRoundRobinChainWithTimeout(t, 3, 0, time.Second)
		if got != "" {
			t.Fatalf("got %q, want empty output", got)
		}
	})

	t.Run("k=1 n=1 completes", func(t *testing.T) {
		got := parseRoundRobinOutput(runRoundRobinChainWithTimeout(t, 1, 1, 2*time.Second))
		assertRoundRobinOutput(t, got, 1, 1)
	})

	t.Run("k=1 n>1 completes without deadlock", func(t *testing.T) {
		// Single goroutine forwards to itself; without a timeout wrapper this
		// would hang the whole test suite if the ring deadlocks.
		got := parseRoundRobinOutput(runRoundRobinChainWithTimeout(t, 1, 5, 2*time.Second))
		assertRoundRobinOutput(t, got, 1, 5)
	})
}

func TestRoundRobinChain_CancelMidRun(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	k, n := 7, 1_000_000
	done := make(chan struct{})
	go func() {
		var buf bytes.Buffer
		roundRobinChain(ctx, &buf, k, n)
		close(done)
	}()
	time.Sleep(10 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("roundRobinChain did not stop after context cancellation")
	}
}

func TestRoundRobinChain_CancelBeforeStart(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan struct{})
	go func() {
		var buf bytes.Buffer
		roundRobinChain(ctx, &buf, 3, 100)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("roundRobinChain did not return for an already cancelled context")
	}
}
