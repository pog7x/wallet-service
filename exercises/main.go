// Package main is the entry point for the application.
package main

import (
	"context"
	"os"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pipelinedemo(os.Stdout, 1000)
	checknilchandemo(os.Stdout, 50)
	roundRobinDispatcher(ctx, os.Stdout, 30, 1589)
	roundRobinChain(ctx, os.Stdout, 7, 1)
}
