package main

import (
	"context"
	"fmt"
	"io"
)

func roundRobinDispatcher(ctx context.Context, w io.Writer, k int, n int) {
	if k < 1 || n < 1 {
		return
	}

	chArr := make([]chan int, k)
	output := make(chan string)

	for ki := range k {
		chArr[ki] = make(chan int)

		go func() {
			for {
				select {
				case v := <-chArr[ki]:
					select {
					case output <- fmt.Sprintf("goroutine %d: %d", ki, v):
					case <-ctx.Done():
						return
					}
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	for ni := 1; ni <= n; ni++ {
		select {
		case chArr[(ni-1)%k] <- ni:
		case <-ctx.Done():
			return
		}
		select {
		case v := <-output:
			fmt.Fprintln(w, v)
		case <-ctx.Done():
			return
		}
	}
}
