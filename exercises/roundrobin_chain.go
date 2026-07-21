package main

import (
	"context"
	"fmt"
	"io"
)

// roundRobinChain prints the numbers 1..n as lines of the form
// "goroutine i: v". The workers form a ring of k goroutines and each value is
// forwarded to the next worker, so value v is always printed by worker
// (v-1)%k. It returns immediately when k < 1 or n < 1.
//
// roundRobinChain respects ctx: cancelling it stops the pipeline and makes the
// function return without printing the remaining values.
//
// The caller must cancel ctx to release the worker goroutines. They do not
// exit on their own: after the last value is printed the ring has no more work,
// yet every worker stays blocked on its receive until ctx is cancelled. A
// caller that lets ctx outlive the call therefore leaks k goroutines, each
// parked in a waiting state and keeping its stack alive. Pairing the call with
// defer cancel() is the intended usage.
func roundRobinChain(ctx context.Context, w io.Writer, k int, n int) {
	if k < 1 || n < 1 {
		return
	}

	chArr := make([]chan int, k)
	output := make(chan string)
	isDone := make(chan struct{})

	for ki := range k {
		chArr[ki] = make(chan int, n/k+1)

		go func() {
			next := ki + 1
			if next >= k {
				next = 0
			}
			for {
				select {
				case v := <-chArr[ki]:
					select {
					case output <- fmt.Sprintf("goroutine %d: %d", ki, v):
					case <-ctx.Done():
						return
					}

					if v < n {
						select {
						case chArr[next] <- v + 1:
						case <-ctx.Done():
							return
						}
						continue
					}

					select {
					case isDone <- struct{}{}:
					case <-ctx.Done():
						return
					}
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	go func() {
		select {
		case <-isDone:
			close(output)
		case <-ctx.Done():
		}
	}()

	select {
	case chArr[0] <- 1:
	case <-ctx.Done():
		return
	}

	for {
		select {
		case line, ok := <-output:
			if !ok {
				return
			}
			fmt.Fprintln(w, line)
		case <-ctx.Done():
			return
		}
	}
}
