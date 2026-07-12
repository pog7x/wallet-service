package account

import (
	"context"
	"sync"

	"github.com/pog7x/wallet-service/internal/money"
)

// chanMutex is a mutual-exclusion lock whose acquisition can be cancelled
// through a context. It is a channel with a one-slot buffer: the single slot
// holds the lock, so at most one goroutine can be inside the critical section
// at a time. Unlike sync.Mutex, a goroutine waiting to acquire it can be
// released by context cancellation instead of blocking unconditionally.
//
// The zero value is not usable; construct a chanMutex with newChanMutex.
// A chanMutex must not be copied after first use in a way that creates an
// independent lock, but assigning the channel value shares the same lock,
// which is why the registry can store it by value.
type chanMutex chan struct{}

// newChanMutex returns a chanMutex ready to be acquired. The returned lock is
// initially free, because its one-slot buffer starts empty.
func newChanMutex() chanMutex {
	return make(chanMutex, 1)
}

// Lock acquires the lock, blocking until the slot is free or ctx is cancelled.
// It returns nil once the lock is held. If ctx is cancelled before the lock is
// acquired, Lock returns ctx.Err() and does not acquire the lock, so the caller
// must not release it in that case. A successful Lock must be paired with
// exactly one Unlock.
func (cm chanMutex) Lock(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case cm <- struct{}{}:
		return nil
	}
}

// Unlock releases the lock so that a waiting Lock can acquire it. Unlock must
// be called only after a Lock that returned nil, and exactly once per such
// Lock. Calling Unlock without holding the lock breaks mutual exclusion or
// blocks forever, because it removes a slot that this goroutine did not fill.
func (cm chanMutex) Unlock() {
	<-cm
}

type keyedMutex struct {
	mu    sync.Mutex
	byKey map[string]chanMutex
}

// lockFor returns the chanMutex associated with key, creating it on first use
// and returning the same lock for the same key afterwards. Returning the same
// lock is what makes two goroutines using the same key exclude each other; a
// fresh lock each time would leave them unsynchronised. The registry's own
// mutex is held only for the duration of this method and is released before any
// account lock is taken, so it is a leaf lock and cannot take part in a
// deadlock cycle.
func (k *keyedMutex) lockFor(key string) chanMutex {
	k.mu.Lock()
	defer k.mu.Unlock()

	m, ok := k.byKey[key]
	if !ok {
		m = newChanMutex()
		k.byKey[key] = m
	}
	return m
}

// Svc is the subset of the wallet service required by the HTTP handlers.
//
// The interface is declared on the consumer side so that handlers depend only
// on the operations they actually call, and so that tests can substitute an
// implementation returning arbitrary domain errors.
//
// Every method receives the request context and must abort when it is cancelled.
type Svc interface {
	Transfer(ctx context.Context, fromID, toID string, amount money.Money) error
	TransferBatch(ctx context.Context, reqs []BatchRequest, concurrency int) []error
}

// Service coordinates operations across accounts using a Repository. It holds
// the Repository as an interface, not a concrete type, so that the storage
// implementation can change without affecting Service.
type Service struct {
	repo Repository
	kMu  keyedMutex
}

var _ Svc = (*Service)(nil)

// NewService returns a Service that uses repo for account storage.
func NewService(repo Repository) *Service {
	return &Service{repo: repo, kMu: keyedMutex{byKey: make(map[string]chanMutex)}}
}

// Transfer moves amount from the source account to the destination account.
// It is safe for concurrent use. Transfer locks both the source and the
// destination account for the whole read-modify-write sequence, so no other
// transfer touching either account can interleave with it. Transfers that
// share no account run in parallel, because each account has its own lock.
//
// Transfer respects ctx during the whole acquisition phase. If ctx is already
// cancelled when the call begins, or if it is cancelled while Transfer is
// waiting to acquire either account lock, Transfer returns ctx.Err() without
// modifying either balance. A lock already acquired at that point is released
// before returning, so a cancelled Transfer leaves no lock held.
//
// Deadlock is prevented by always acquiring the two account locks in a fixed
// global order determined by comparing the account IDs, so two transfers in
// opposite directions cannot form a cycle.
//
// This isolation only covers changes made through this Service. Modifications
// applied directly to the repository, bypassing Transfer, are not protected.
//
// Transfer is not all-or-nothing on partial failure: if the first Save succeeds
// and the second fails, the source is debited without crediting the
// destination. Restoring that guarantee requires transactional storage and is
// deferred to the database layer.
func (s *Service) Transfer(ctx context.Context, fromID, toID string, amount money.Money) error {
	if fromID == toID {
		return ErrSameAccount
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	first, second := fromID, toID
	if first > second {
		first, second = second, first
	}

	m1 := s.kMu.lockFor(first)
	if err := m1.Lock(ctx); err != nil {
		return err
	}
	defer m1.Unlock()

	m2 := s.kMu.lockFor(second)
	if err := m2.Lock(ctx); err != nil {
		return err
	}
	defer m2.Unlock()

	fromAcc, err := s.repo.Load(ctx, fromID)
	if err != nil {
		return err
	}

	toAcc, err := s.repo.Load(ctx, toID)
	if err != nil {
		return err
	}

	if err = fromAcc.Withdraw(amount); err != nil {
		return err
	}

	if err = toAcc.Deposit(amount); err != nil {
		return err
	}

	// TODO: atomic problem
	if err = s.repo.Save(ctx, fromAcc); err != nil {
		return err
	}

	if err = s.repo.Save(ctx, toAcc); err != nil {
		return err
	}

	return nil
}

// BatchRequest describes a single transfer within a batch: move Amount from
// the account identified by From to the account identified by To.
type BatchRequest struct {
	From   string
	To     string
	Amount money.Money
}

// TransferBatch executes reqs concurrently and returns a slice of errors
// aligned with reqs by index: results[i] reports the outcome of reqs[i] and is
// nil when that transfer succeeded. Each request runs as an independent
// Transfer, so the failure of one request does not affect the others, and the
// batch is not atomic across requests.
//
// At most concurrency transfers run at the same time; a value below 1 is
// treated as 1. Bounding parallelism prevents an arbitrarily large input from
// starting an unbounded number of goroutines and exhausting resources such as
// storage connections.
//
// TransferBatch respects ctx. If ctx is cancelled, requests not yet started
// receive ctx.Err() without running, and requests already started observe the
// cancellation through the Transfer they invoke. A cancelled batch leaves the
// accounts of unstarted requests unchanged.
func (s *Service) TransferBatch(ctx context.Context, reqs []BatchRequest, concurrency int) []error {
	if concurrency < 1 {
		concurrency = 1
	}

	results := make([]error, len(reqs))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for i, req := range reqs {
		// Занять слот семафора или прекратить запуск, если контекст отменён.
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			results[i] = ctx.Err()
			continue
		}

		wg.Go(func() {
			defer func() { <-sem }() // освободить слот в любом случае
			results[i] = s.Transfer(ctx, req.From, req.To, req.Amount)
		})
	}

	wg.Wait()
	return results
}
