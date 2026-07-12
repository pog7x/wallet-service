// Package main is the entry point for the application.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/pog7x/wallet-service/handlers/api"
	"github.com/pog7x/wallet-service/internal/account"
)

const (
	defaultAddr       = "0.0.0.0:8000"
	defaultBatchLimit = 5

	// shutdownTimeout bounds how long the server waits for in-flight requests to
	// finish after a termination signal. It must be at least as large as the
	// write timeout, because a request that is still allowed to write must be
	// allowed to finish; otherwise Shutdown would cut off a transfer between the
	// two Save calls, which is exactly the non-atomic window the Service has.
	shutdownTimeout = 15 * time.Second
)

// newServer builds the HTTP server for the wallet service.
//
// It has no side effects: it neither listens on a port nor starts goroutines, so
// the wiring it performs (routes, timeouts) can be verified in a test.
func newServer(addr string, svc api.Service, batchLimit int) *http.Server {
	mux := http.NewServeMux()
	api.NewHandler(svc, batchLimit).Routes(mux)

	return &http.Server{
		Addr:    addr,
		Handler: mux,
		// ReadHeaderTimeout bounds how long a client may take to send its
		// headers. Without it a client can hold a connection open indefinitely
		// by sending headers one byte at a time.
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}

// run starts the server and blocks until a termination signal arrives, then
// shuts the server down, letting in-flight requests finish within
// shutdownTimeout.
func run(w io.Writer) error {
	addr := envOr("WALLETD_ADDR", defaultAddr)
	batchLimit := envIntOr("WALLETD_BATCH_LIMIT", defaultBatchLimit)

	svc := account.NewService(account.NewMemRepository())
	srv := newServer(addr, svc, batchLimit)

	// signal.NotifyContext returns a context that is cancelled when one of the
	// listed signals arrives. It is used instead of a raw channel so that the
	// cancellation composes with everything else that already takes a context.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		// ListenAndServe returns ErrServerClosed after Shutdown is called, which
		// is the normal path and not a failure.
		errCh <- srv.ListenAndServe()
	}()

	fmt.Fprintf(w, "walletd: listening on %s\n", addr)

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		// Stop translating signals before shutting down, so that a second signal
		// terminates the process immediately instead of being swallowed.
		stop()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown: %w", err)
		}

		return nil
	}
}

func main() {
	if err := run(os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "walletd: %v\n", err)
		os.Exit(1)
	}
}

// envOr returns the value of the environment variable named key, or def if the
// variable is unset or empty. Configuration comes from the environment because
// the address depends on where the binary runs, and rebuilding the binary to
// change a port is not an option in a container.
func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// envIntOr returns the integer value of the environment variable named key, or
// def if the variable is unset, empty or not a valid integer. An invalid value
// falls back to the default rather than failing, because a batch limit is a
// tuning parameter and a typo in it must not prevent the service from starting.
func envIntOr(key string, def int) int {
	v, err := strconv.Atoi(os.Getenv(key))
	if err != nil || v < 1 {
		return def
	}
	return v
}
