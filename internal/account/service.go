package account

import "github.com/pog7x/wallet-service/internal/money"

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

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
