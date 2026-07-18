package main

import (
	"context"
	"fmt"
	"io"
)

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

	chArr[0] <- 1

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
