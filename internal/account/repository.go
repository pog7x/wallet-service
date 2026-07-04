package account

type Repository interface {
	Load(id string) (*Account, error)
	Save(a *Account) error
}

type MemRepository struct {
	accMap map[string]Account
}

func NewMemRepository() *MemRepository {
	return &MemRepository{accMap: make(map[string]Account)}
}

func (mr *MemRepository) Load(id string) (*Account, error) {
	if acc, ok := mr.accMap[id]; ok {
		return &acc, nil
	}

	return nil, ErrAccountNotFound
}

func (mr *MemRepository) Save(a *Account) error {
	if a.currency != a.Balance().Currency() {
		return ErrCurrencyMismatch
	}

	if existAcc, ok := mr.accMap[a.id]; ok {
		if existAcc.currency != a.currency {
			return ErrCurrencyMismatch
		}
	}

	mr.accMap[a.id] = *a
	return nil
}
