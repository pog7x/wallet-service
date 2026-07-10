package account

import (
	"context"
	"sync"
)

// Repository stores and retrieves accounts. It is an interface so that
// callers depend on the abstraction rather than on a concrete storage
// implementation, which allows the in-memory store to be replaced by a
// database-backed one without changing the code that uses it.
type Repository interface {
	// Load returns the account stored under id, or ErrAccountNotFound if none
	// exists. It respects ctx and returns ctx.Err() without accessing storage
	// when ctx is already cancelled.
	Load(ctx context.Context, id string) (*Account, error)

	// Save stores a under its identifier, replacing any existing account with the
	// same identifier. It respects ctx and returns ctx.Err() without writing to
	// storage when ctx is already cancelled.
	Save(ctx context.Context, a *Account) error
}

// MemRepository is an in-memory implementation of Repository backed by a map
// keyed by account identifier. It is intended for tests and local development,
// not for persistence, because its contents are lost when the process exits.
// It is safe for concurrent use by multiple goroutines.
type MemRepository struct {
	accMap map[string]Account
	mu     sync.Mutex
}

var _ Repository = (*MemRepository)(nil)

// NewMemRepository returns an empty in-memory repository. It initializes the
// underlying map, because writing to a nil map panics, so the zero value of
// MemRepository must not be used directly.
func NewMemRepository() *MemRepository {
	return &MemRepository{accMap: make(map[string]Account)}
}

// Load returns a pointer to a copy of the stored account, or ErrAccountNotFound
// if it does not exist. It returns ctx.Err() without reading the map when ctx
// is already cancelled. The returned account is independent of stored state:
// changes made through it are not persisted until it is passed to Save.
func (mr *MemRepository) Load(ctx context.Context, id string) (*Account, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	mr.mu.Lock()
	defer mr.mu.Unlock()

	if acc, ok := mr.accMap[id]; ok {
		return &acc, nil
	}

	return nil, ErrAccountNotFound
}

// Save stores a copy of a under its identifier, replacing any existing account
// with the same identifier. It returns ctx.Err() without writing the map when
// ctx is already cancelled. Because a copy is stored, later changes to a made
// by the caller do not affect the stored account until Save is called again.
func (mr *MemRepository) Save(ctx context.Context, a *Account) error {
	if a.currency != a.balance.Currency() {
		return ErrCurrencyMismatch
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	mr.mu.Lock()
	defer mr.mu.Unlock()

	if existAcc, ok := mr.accMap[a.id]; ok {
		if existAcc.currency != a.currency {
			return ErrCurrencyMismatch
		}
	}

	mr.accMap[a.id] = *a
	return nil
}
