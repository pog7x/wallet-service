package account

import (
	"errors"
	"testing"

	"github.com/pog7x/wallet-service/internal/money"
)

func TestNewMemRepository(t *testing.T) {
	mr := NewMemRepository()

	if mr.accMap == nil {
		t.Error("mem repository acc map must be not nil map")
	}
}

func TestMemRepositorySaveSuccess(t *testing.T) {
	expectedAccountID, expectedCurrency := "654635634", money.Currency("USD")
	expectedAmount := money.New(int64(734643), expectedCurrency)

	mr := NewMemRepository()
	mr.accMap[expectedAccountID] = Account{
		balance:  money.New(0, expectedCurrency),
		currency: expectedCurrency,
		id:       expectedAccountID,
	}

	err := mr.Save(&Account{balance: expectedAmount, currency: expectedCurrency, id: expectedAccountID})
	if err != nil {
		t.Errorf("Save(*Account) unexpected error %v", err)
	}

	acc := mr.accMap[expectedAccountID]
	if acc.Balance().Amount() != expectedAmount.Amount() {
		t.Errorf(
			"account amount not equal to expected amount, want: %d got: %d",
			expectedAmount.Amount(), acc.Balance().Amount(),
		)
	}
}

func TestMemRepositorySaveFail(t *testing.T) {
	expectedAccountID, expectedCurrency := "654635634", money.Currency("USD")
	expectedAmount := money.New(int64(0), expectedCurrency)

	mr := NewMemRepository()
	mr.accMap[expectedAccountID] = Account{
		balance:  expectedAmount,
		currency: expectedCurrency,
		id:       expectedAccountID,
	}

	err := mr.Save(&Account{balance: money.New(int64(734643), "EUR"), currency: "EUR", id: expectedAccountID})
	if !errors.Is(err, ErrCurrencyMismatch) {
		t.Errorf("Save(*Account) unexpected error, want %v got %v", ErrCurrencyMismatch, err)
	}

	err = mr.Save(&Account{balance: money.New(int64(734643), "EUR"), currency: expectedCurrency, id: expectedAccountID})
	if !errors.Is(err, ErrCurrencyMismatch) {
		t.Errorf("Save(*Account) unexpected error, want %v got %v", ErrCurrencyMismatch, err)
	}

	acc := mr.accMap[expectedAccountID]
	if acc.Balance().Amount() != expectedAmount.Amount() {
		t.Errorf(
			"account amount not equal to expected amount, want: %d got: %d",
			expectedAmount.Amount(), acc.Balance().Amount(),
		)
	}
}

func TestMemRepositoryLoadSuccess(t *testing.T) {
	expectedAccountID, expectedCurrency := "654635634", money.Currency("USD")
	expectedAmount := money.New(int64(734643), expectedCurrency)
	mr := NewMemRepository()

	mr.accMap[expectedAccountID] = Account{balance: expectedAmount, currency: expectedCurrency, id: expectedAccountID}

	acc, err := mr.Load(expectedAccountID)
	if err != nil {
		t.Errorf("Got unexpected error %v", err)
	}

	if acc.Balance().Amount() != expectedAmount.Amount() {
		t.Errorf(
			"account amount not equal to expected amount, want: %d got: %d",
			expectedAmount.Amount(), acc.Balance().Amount(),
		)
	}
	if acc.Balance().Currency() != expectedCurrency {
		t.Errorf(
			"account balance currency not equal to expected currency, want: %s got: %s",
			expectedCurrency, acc.Balance().Currency(),
		)
	}
	if acc.currency != expectedCurrency {
		t.Errorf(
			"account currency not equal to expected currency, want: %s got: %s",
			expectedCurrency, acc.currency,
		)
	}
}

func TestMemRepositoryLoadFail(t *testing.T) {
	expectedAccountID := "654635634"
	mr := NewMemRepository()

	_, err := mr.Load(expectedAccountID)
	if !errors.Is(err, ErrAccountNotFound) {
		t.Errorf("Load(%s) unexpected error, want %v got %v", expectedAccountID, ErrAccountNotFound, err)
	}
}

func TestMemRepositoryLoadDataNotChange(t *testing.T) {
	expectedAccountID, expectedCurrency := "654635634", money.Currency("USD")
	expectedAmount := money.New(int64(734643), expectedCurrency)
	mr := NewMemRepository()

	mr.accMap[expectedAccountID] = Account{balance: expectedAmount, currency: expectedCurrency, id: expectedAccountID}

	acc, err := mr.Load(expectedAccountID)
	if err != nil {
		t.Errorf("Got unexpected error %v", err)
	}

	acc.balance = money.New(0, "EUR")
	acc.currency = money.Currency("EUR")
	acc.id = "changed id"

	acc, err = mr.Load(expectedAccountID)
	if err != nil {
		t.Errorf("Got unexpected error %v", err)
	}
	if acc.Balance().Amount() != expectedAmount.Amount() {
		t.Errorf(
			"account amount not equal to expected amount, want: %d got: %d",
			expectedAmount.Amount(), acc.Balance().Amount(),
		)
	}
	if acc.Balance().Currency() != expectedCurrency {
		t.Errorf(
			"account balance currency not equal to expected currency, want: %s got: %s",
			expectedCurrency, acc.Balance().Currency(),
		)
	}
	if acc.currency != expectedCurrency {
		t.Errorf(
			"account currency not equal to expected currency, want: %s got: %s",
			expectedCurrency, acc.currency,
		)
	}
}
