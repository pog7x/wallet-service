package main

import (
	"context"
	"fmt"
	"io"
)

func generate(ctx context.Context, n int) <-chan int {
	out := make(chan int)

	go func() {
		defer close(out)
		for i := 1; i <= n; i++ {
			select {
			case out <- i:
			case <-ctx.Done():
				return
			}
		}
	}()

	return out
}

func sqr(ctx context.Context, in <-chan int) <-chan int {
	out := make(chan int)

	go func() {
		defer close(out)
		for i := range in {
			select {
			case out <- i * i:
			case <-ctx.Done():
				return
			}
		}
	}()

	return out
}

func sum(in <-chan int) int {
	res := 0
	for i := range in {
		res += i
	}

	return res
}

func pipelinedemo(w io.Writer, n int) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	fmt.Fprintln(w, sum(sqr(ctx, generate(ctx, n))))
}
