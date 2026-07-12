package main

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/pog7x/wallet-service/internal/account"
	"github.com/pog7x/wallet-service/internal/money"
)

// stubService satisfies api.Service without touching storage: newServer only
// wires dependencies, so the behaviour of the service is irrelevant here.
type stubService struct{}

func (stubService) Transfer(context.Context, string, string, money.Money) error { return nil }
func (stubService) TransferBatch(context.Context, []account.BatchRequest, int) []error {
	return nil
}

func TestNewServer(t *testing.T) {
	srv := newServer("127.0.0.1:0", stubService{}, 5)

	if srv.Addr != "127.0.0.1:0" {
		t.Errorf("addr = %q, want 127.0.0.1:0", srv.Addr)
	}
	if srv.Handler == nil {
		t.Fatal("handler is nil: the routes were not wired")
	}
	// A zero ReadHeaderTimeout leaves the server open to a client that sends its
	// headers indefinitely slowly, so its absence is a defect, not a style issue.
	if srv.ReadHeaderTimeout == 0 {
		t.Error("ReadHeaderTimeout is not set")
	}
	if srv.WriteTimeout == 0 {
		t.Error("WriteTimeout is not set")
	}
	if shutdownTimeout < srv.WriteTimeout {
		t.Errorf("shutdownTimeout = %v, must be at least WriteTimeout = %v: "+
			"otherwise shutdown cuts off a request that is still allowed to write",
			shutdownTimeout, srv.WriteTimeout)
	}
}

func TestNewServer_RoutesAreRegistered(t *testing.T) {
	srv := newServer("127.0.0.1:0", stubService{}, 5)

	mux, ok := srv.Handler.(*http.ServeMux)
	if !ok {
		t.Fatalf("handler type = %T, want *http.ServeMux", srv.Handler)
	}

	for _, path := range []string{"/transfers", "/transfers/batch"} {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://example.com"+path, nil)
		if err != nil {
			t.Fatalf("build request: %v", err)
		}
		if _, pattern := mux.Handler(req); pattern == "" {
			t.Errorf("no handler registered for POST %s", path)
		}
	}

	_ = time.Second // keep the import if unused after edits
}
