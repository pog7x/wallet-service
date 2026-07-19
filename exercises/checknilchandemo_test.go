package main

import (
	"bytes"
	"context"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"go.uber.org/goleak"
)

// intSource returns a channel that yields values and is then closed. It is the
// producer side of every merge test: merge only learns that an input is
// exhausted from the close, which is what triggers the nil-channel branch.
func intSource(ctx context.Context, values ...int) <-chan int {
	ch := make(chan int)
	go func() {
		defer close(ch)
		for _, v := range values {
			select {
			case ch <- v:
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch
}

// sortedInts returns a sorted copy. merge interleaves its two inputs in an
// order the scheduler decides, so its output can only be compared as a
// multiset, never as a sequence.
func sortedInts(vs []int) []int {
	out := append([]int(nil), vs...)
	sort.Ints(out)
	return out
}

func assertSameMultiset(t *testing.T, got, want []int) {
	t.Helper()

	g, w := sortedInts(got), sortedInts(want)
	if len(g) != len(w) {
		t.Fatalf("got %d values %v, want %d values %v", len(g), g, len(w), w)
	}
	for i := range w {
		if g[i] != w[i] {
			t.Fatalf("sorted output mismatch:\n got: %v\nwant: %v", g, w)
		}
	}
}

func TestMerge_DrainsBothSources(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	a := intSource(ctx, 1, 2, 3)
	b := intSource(ctx, 10, 20)

	got := collectChan(t, merge(ctx, a, b), 2*time.Second)
	assertSameMultiset(t, got, []int{1, 2, 3, 10, 20})
}

// TestMerge_OneSourceEmpty covers the case where one input closes without ever
// producing a value. merge must disable that branch immediately and keep
// serving the other input instead of spinning on the closed channel.
func TestMerge_OneSourceEmpty(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("a is empty", func(t *testing.T) {
		got := collectChan(t, merge(ctx, intSource(ctx), intSource(ctx, 7, 8, 9)), 2*time.Second)
		assertSameMultiset(t, got, []int{7, 8, 9})
	})

	t.Run("b is empty", func(t *testing.T) {
		got := collectChan(t, merge(ctx, intSource(ctx, 7, 8, 9), intSource(ctx)), 2*time.Second)
		assertSameMultiset(t, got, []int{7, 8, 9})
	})

	t.Run("both empty", func(t *testing.T) {
		got := collectChan(t, merge(ctx, intSource(ctx), intSource(ctx)), 2*time.Second)
		if len(got) != 0 {
			t.Fatalf("got %v, want no values", got)
		}
	})
}

// TestMerge_UnevenSources is the direct test of the nil-channel technique: the
// short input closes long before the long one, so merge spends most of the run
// with one branch disabled. If the exhausted branch were left enabled it would
// be permanently ready with ok == false and could starve the other input.
func TestMerge_UnevenSources(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const long = 500
	want := make([]int, 0, long+1)
	values := make([]int, 0, long)
	for i := 1; i <= long; i++ {
		values = append(values, i)
		want = append(want, i)
	}
	want = append(want, -1)

	got := collectChan(t, merge(ctx, intSource(ctx, -1), intSource(ctx, values...)), 5*time.Second)
	assertSameMultiset(t, got, want)
}

// TestMerge_CancelMidRun_NoLeak cancels while both inputs still have values.
// merge blocks on three operations, the two receives and the send into out,
// and all three must observe cancellation for the goroutine to return.
func TestMerge_CancelMidRun_NoLeak(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctx, cancel := context.WithCancel(context.Background())

	values := make([]int, 100_000)
	for i := range values {
		values[i] = i
	}

	out := merge(ctx, intSource(ctx, values...), intSource(ctx, values...))

	<-out
	<-out
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
		t.Fatal("merge did not shut down after cancellation")
	}
}

// TestConveyor_EmitsEachValueTwice pins down the current behaviour: every
// worker sends its value into a and then into b, and merge forwards both
// copies, so each value from 0 to n-1 appears exactly twice.
func TestConveyor_EmitsEachValueTwice(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const n = 50
	want := make([]int, 0, 2*n)
	for v := range n {
		want = append(want, v, v)
	}

	got := collectChan(t, conveyor(ctx, n), 5*time.Second)
	assertSameMultiset(t, got, want)
}

func TestConveyor_EmptyForNonPositiveN(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if got := collectChan(t, conveyor(ctx, 0), 2*time.Second); len(got) != 0 {
		t.Fatalf("got %v, want no values", got)
	}
}

func TestConveyor_CancelMidRun_NoLeak(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctx, cancel := context.WithCancel(context.Background())

	out := conveyor(ctx, 100_000)

	<-out
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
	case <-time.After(5 * time.Second):
		t.Fatal("conveyor did not shut down after cancellation")
	}
}

// TestChecknilchandemo_Output verifies the printed form: one value per line,
// two lines per value. The lines arrive in scheduler-dependent order, so they
// are compared as a multiset after parsing.
func TestChecknilchandemo_Output(t *testing.T) {
	defer goleak.VerifyNone(t)

	const n = 20

	var buf bytes.Buffer
	checknilchandemo(&buf, n)

	lines := parseRoundRobinOutput(buf.String())
	if len(lines) != 2*n {
		t.Fatalf("got %d lines, want %d", len(lines), 2*n)
	}

	got := make([]int, 0, len(lines))
	for i, line := range lines {
		v, err := strconv.Atoi(strings.TrimSpace(line))
		if err != nil {
			t.Fatalf("line %d = %q, not an integer: %v", i, line, err)
		}
		got = append(got, v)
	}

	want := make([]int, 0, 2*n)
	for v := range n {
		want = append(want, v, v)
	}
	assertSameMultiset(t, got, want)
}

func TestChecknilchandemo_EmptyForZero(t *testing.T) {
	defer goleak.VerifyNone(t)

	var buf bytes.Buffer
	checknilchandemo(&buf, 0)

	if got := buf.String(); got != "" {
		t.Fatalf("got %q, want empty output", got)
	}
}
