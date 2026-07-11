package account

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pog7x/wallet-service/internal/money"
)

func TestTransferBatch_AllSucceed(t *testing.T) {
	repo := newFundedRepo(t, map[string]int64{"A": 10000, "B": 0, "C": 5000, "D": 0})
	svc := NewService(repo)

	reqs := []BatchRequest{
		{From: "A", To: "B", Amount: money.New(3000, transferCurrency)},
		{From: "C", To: "D", Amount: money.New(1500, transferCurrency)},
	}

	results := svc.TransferBatch(t.Context(), reqs, 2)

	for i, err := range results {
		if err != nil {
			t.Errorf("results[%d] = %v, want nil", i, err)
		}
	}

	want := map[string]int64{"A": 7000, "B": 3000, "C": 3500, "D": 1500}
	for id, w := range want {
		if got := mustBalance(t, repo, id); got != w {
			t.Errorf("%s = %d, want %d", id, got, w)
		}
	}
}

func TestTransferBatch_PartialFailure(t *testing.T) {
	// C has only 100, so the 5000 transfer must fail with ErrInsufficientFunds
	// while the A->B transfer succeeds independently.
	repo := newFundedRepo(t, map[string]int64{"A": 10000, "B": 0, "C": 100, "D": 0})
	svc := NewService(repo)

	reqs := []BatchRequest{
		{From: "A", To: "B", Amount: money.New(3000, transferCurrency)},
		{From: "C", To: "D", Amount: money.New(5000, transferCurrency)},
	}

	results := svc.TransferBatch(t.Context(), reqs, 2)

	if results[0] != nil {
		t.Errorf("results[0] = %v, want nil", results[0])
	}
	if !errors.Is(results[1], ErrInsufficientFunds) {
		t.Errorf("results[1] = %v, want ErrInsufficientFunds", results[1])
	}

	// Successful transfer applied.
	if got := mustBalance(t, repo, "A"); got != 7000 {
		t.Errorf("A = %d, want 7000", got)
	}
	if got := mustBalance(t, repo, "B"); got != 3000 {
		t.Errorf("B = %d, want 3000", got)
	}
	// Failed transfer left both its accounts untouched.
	if got := mustBalance(t, repo, "C"); got != 100 {
		t.Errorf("C = %d, want unchanged 100", got)
	}
	if got := mustBalance(t, repo, "D"); got != 0 {
		t.Errorf("D = %d, want unchanged 0", got)
	}
}

func TestTransferBatch_ContextCancelled(t *testing.T) {
	initial := map[string]int64{"A": 10000, "B": 0, "C": 5000, "D": 0}
	repo := newFundedRepo(t, initial)
	svc := NewService(repo)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // отменяем до вызова: ни один перевод не должен изменить состояние

	reqs := []BatchRequest{
		{From: "A", To: "B", Amount: money.New(3000, transferCurrency)},
		{From: "C", To: "D", Amount: money.New(1500, transferCurrency)},
	}

	results := svc.TransferBatch(ctx, reqs, 2)

	for i, err := range results {
		if !errors.Is(err, context.Canceled) {
			t.Errorf("results[%d] = %v, want context.Canceled", i, err)
		}
	}
	for id, w := range initial {
		if got := mustBalance(t, repo, id); got != w {
			t.Errorf("%s = %d, want unchanged %d", id, got, w)
		}
	}
}

// countingRepo wraps a MemRepository and records the maximum number of Load
// calls that were ever in flight at the same time. The hold widens the overlap
// window so that, without a working semaphore, concurrent transfers actually
// overlap and the recorded maximum exceeds the limit.
type countingRepo struct {
	inner    *MemRepository
	inFlight atomic.Int32
	maxSeen  atomic.Int32
	hold     time.Duration
}

func (c *countingRepo) Load(ctx context.Context, id string) (*Account, error) {
	n := c.inFlight.Add(1)
	for {
		m := c.maxSeen.Load()
		if n <= m || c.maxSeen.CompareAndSwap(m, n) {
			break
		}
	}
	time.Sleep(c.hold)
	acc, err := c.inner.Load(ctx, id)
	c.inFlight.Add(-1)
	return acc, err
}

func (c *countingRepo) Save(ctx context.Context, a *Account) error {
	return c.inner.Save(ctx, a)
}

func TestTransferBatch_ConcurrencyLimit(t *testing.T) {
	const (
		pairs       = 20
		concurrency = 4
	)

	// Disjoint account pairs so nothing serializes on per-account locks; only
	// the semaphore can bound how many transfers run at once.
	balances := make(map[string]int64, pairs*2)
	reqs := make([]BatchRequest, 0, pairs)
	for i := range pairs {
		from := fmt.Sprintf("from-%02d", i)
		to := fmt.Sprintf("to-%02d", i)
		balances[from] = 1000
		balances[to] = 0
		reqs = append(reqs, BatchRequest{From: from, To: to, Amount: money.New(100, transferCurrency)})
	}

	cr := &countingRepo{inner: newFundedRepo(t, balances), hold: 2 * time.Millisecond}
	svc := NewService(cr)

	results := svc.TransferBatch(t.Context(), reqs, concurrency)

	for i, err := range results {
		if err != nil {
			t.Errorf("results[%d] = %v, want nil", i, err)
		}
	}
	if got := cr.maxSeen.Load(); got > int32(concurrency) {
		t.Fatalf("max concurrent Load = %d, want <= %d", got, concurrency)
	}
}
