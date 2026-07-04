package account

import "github.com/pog7x/wallet-service/internal/money"

// Service coordinates operations across accounts using a Repository. It holds
// the Repository as an interface, not a concrete type, so that the storage
// implementation can change without affecting Service.
type Service struct {
	repo Repository
}

// NewService returns a Service that uses repo for account storage.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Transfer moves amount from the account fromID to the account toID.
//
// It returns ErrSameAccount if fromID equals toID,
// ErrAccountNotFound if either account does not exist, ErrNonPositiveAmount if
// amount is not strictly positive, ErrCurrencyMismatch if amount's currency
// differs from an account's currency, and ErrInsufficientFunds if the source
// balance is smaller than amount.
//
// Transfer is not atomic in this in-memory implementation: if saving the
// second account fails after the first has been saved, the two accounts can be
// left in an inconsistent state. Atomicity will be provided by database
// transactions later.
func (s *Service) Transfer(fromID, toID string, amount money.Money) error {
	if fromID == toID {
		return ErrSameAccount
	}

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
