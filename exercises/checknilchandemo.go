package main

import (
	"context"
	"fmt"
	"io"
	"sync"
)

func merge(ctx context.Context, a, b <-chan int) <-chan int {
	out := make(chan int)

	go func() {
		defer close(out)
		for a != nil || b != nil {
			select {
			case v, ok := <-a:
				if !ok {
					a = nil
					continue
				}
				select {
				case out <- v:
				case <-ctx.Done():
					return
				}
			case v, ok := <-b:
				if !ok {
					b = nil
					continue
				}
				select {
				case out <- v:
				case <-ctx.Done():
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return out
}

func conveyor(ctx context.Context, n int) <-chan int {
	a, b := make(chan int), make(chan int)
	wg := sync.WaitGroup{}

	for v := range n {
		wg.Go(func() {
			select {
			case a <- v:
			case <-ctx.Done():
				return
			}
			select {
			case b <- v:
			case <-ctx.Done():
				return
			}
		})
	}

	go func() {
		defer close(a)
		defer close(b)
		wg.Wait()
	}()

	return merge(ctx, a, b)
}

func checknilchandemo(w io.Writer, n int) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for v := range conveyor(ctx, n) {
		fmt.Fprintln(w, fmt.Sprint(v))
	}
}
