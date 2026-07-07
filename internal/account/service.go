package account

import (
	"sync"

	"github.com/pog7x/wallet-service/internal/money"
)

// Service coordinates operations across accounts using a Repository. It holds
// the Repository as an interface, not a concrete type, so that the storage
// implementation can change without affecting Service.
type Service struct {
	repo Repository
	mu   sync.Mutex
}

// NewService returns a Service that uses repo for account storage.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Transfer moves amount from the source account to the destination account.
// It is safe for concurrent use: a package-level mutex serializes every
// Transfer on this Service, so no two transfers interleave and the read-modify-
// write sequence on each account is isolated from other transfers.
//
// This isolation only covers changes made through this Service. Modifications
// applied directly to the repository, bypassing Transfer, are not protected.
//
// Transfer is not all-or-nothing on partial failure: if the first Save succeeds
// and the second fails, the source is debited without crediting the
// destination. Restoring that guarantee requires transactional storage and is
// deferred to the database layer.
func (s *Service) Transfer(fromID, toID string, amount money.Money) error {
	if fromID == toID {
		return ErrSameAccount
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	fromAcc, err := s.repo.Load(fromID)
	if err != nil {
		return err
	}

	toAcc, err := s.repo.Load(toID)
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
	if err = s.repo.Save(fromAcc); err != nil {
		return err
	}

	if err = s.repo.Save(toAcc); err != nil {
		return err
	}

	return nil
}
