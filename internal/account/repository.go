package account

import "sync"

// Repository stores and retrieves accounts. It is an interface so that
// callers depend on the abstraction rather than on a concrete storage
// implementation, which allows the in-memory store to be replaced by a
// database-backed one without changing the code that uses it.
type Repository interface {
	Load(id string) (*Account, error)
	Save(a *Account) error
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
// if it does not exist. The returned account is independent of stored state:
// changes made through it are not persisted until it is passed to Save.
func (mr *MemRepository) Load(id string) (*Account, error) {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	if acc, ok := mr.accMap[id]; ok {
		return &acc, nil
	}

	return nil, ErrAccountNotFound
}

// Save stores a copy of a under its identifier, replacing any existing account
// with the same identifier. Because a copy is stored, later changes to a made
// by the caller do not affect the stored account until Save is called again.
func (mr *MemRepository) Save(a *Account) error {
	if a.currency != a.balance.Currency() {
		return ErrCurrencyMismatch
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
