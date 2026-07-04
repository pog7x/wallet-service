package account

import (
	"errors"
	"testing"

	"github.com/pog7x/wallet-service/internal/money"
)

func TestTransferSuccess(t *testing.T) {
	fromAccountID, toAccountID := "654635634", "134635461"
	testCurrency := money.Currency("USD")

	fromAmount, toAmount := money.New(111111, testCurrency), money.New(222222, testCurrency)
	toTransfer := money.New(555, testCurrency)

	expectedFromAmount, _ := fromAmount.Sub(toTransfer)
	expectedToAmount, _ := toAmount.Add(toTransfer)

	mr := NewMemRepository()

	mr.accMap[fromAccountID] = Account{balance: fromAmount, currency: testCurrency, id: fromAccountID}
	mr.accMap[toAccountID] = Account{balance: toAmount, currency: testCurrency, id: toAccountID}

	s := NewService(mr)

	err := s.Transfer(fromAccountID, toAccountID, toTransfer)
	if err != nil {
		t.Errorf("Got unexpected error %v", err)
	}

	acc1, acc2 := mr.accMap[fromAccountID], mr.accMap[toAccountID]

	if acc1.Balance().Amount() != expectedFromAmount.Amount() {
		t.Errorf(
			"'from' account amount is not equal to expected amount after transfer, want: %d got: %d",
			expectedFromAmount.Amount(), acc1.Balance().Amount(),
		)
	}

	if acc2.Balance().Amount() != expectedToAmount.Amount() {
		t.Errorf(
			"'to' account amount is not equal to expected amount after transfer, want: %d got: %d",
			expectedToAmount.Amount(), acc2.Balance().Amount(),
		)
	}
}

func TestSameAccountError(t *testing.T) {
	testAcoountID := "654635634"
	testCurrency := money.Currency("USD")

	expectedAmount := money.New(111111, testCurrency)

	mr := NewMemRepository()

	mr.accMap[testAcoountID] = Account{balance: expectedAmount, currency: testCurrency, id: testAcoountID}

	s := NewService(mr)

	err := s.Transfer(testAcoountID, testAcoountID, money.New(555, testCurrency))
	if !errors.Is(err, ErrSameAccount) {
		t.Errorf("Transfer unexpected error, want %v got %v", ErrSameAccount, err)
	}

	acc := mr.accMap[testAcoountID]

	if acc.Balance().Amount() != expectedAmount.Amount() {
		t.Errorf(
			"account amount not equal to expected amount, want: %d got: %d",
			expectedAmount.Amount(), acc.Balance().Amount(),
		)
	}
}

func TestTransferNonPositiveError(t *testing.T) {
	fromAccountID, toAccountID := "654635634", "134635461"
	testCurrency := money.Currency("USD")

	fromAmount, toAmount := money.New(111111, testCurrency), money.New(222222, testCurrency)

	mr := NewMemRepository()

	mr.accMap[fromAccountID] = Account{balance: fromAmount, currency: testCurrency, id: fromAccountID}
	mr.accMap[toAccountID] = Account{balance: toAmount, currency: testCurrency, id: toAccountID}

	s := NewService(mr)

	err := s.Transfer(fromAccountID, toAccountID, money.New(-5555, testCurrency))
	if !errors.Is(err, ErrNonPositiveAmount) {
		t.Errorf("Transfer unexpected error, want %v got %v", ErrSameAccount, err)
	}

	acc1, acc2 := mr.accMap[fromAccountID], mr.accMap[toAccountID]

	if acc1.Balance().Amount() != fromAmount.Amount() {
		t.Errorf(
			"account amount not equal to expected amount, want: %d got: %d",
			fromAmount.Amount(), acc1.Balance().Amount(),
		)
	}
	if acc2.Balance().Amount() != toAmount.Amount() {
		t.Errorf(
			"account amount not equal to expected amount, want: %d got: %d",
			toAmount.Amount(), acc2.Balance().Amount(),
		)
	}
}

func TestTransferInsufficientFundsError(t *testing.T) {
	fromAccountID, toAccountID := "654635634", "134635461"
	testCurrency := money.Currency("USD")

	fromAmount, toAmount := money.New(0, testCurrency), money.New(222222, testCurrency)

	expectedAmount := money.New(111111, testCurrency)

	mr := NewMemRepository()

	mr.accMap[fromAccountID] = Account{balance: fromAmount, currency: testCurrency, id: fromAccountID}
	mr.accMap[toAccountID] = Account{balance: toAmount, currency: testCurrency, id: toAccountID}

	s := NewService(mr)

	err := s.Transfer(fromAccountID, toAccountID, money.New(5555, testCurrency))
	if !errors.Is(err, ErrNonPositiveAmount) {
		t.Errorf("Transfer unexpected error, want %v got %v", ErrSameAccount, err)
	}

	acc := mr.accMap[fromAccountID]

	if acc.Balance().Amount() != expectedAmount.Amount() {
		t.Errorf(
			"account amount not equal to expected amount, want: %d got: %d",
			expectedAmount.Amount(), acc.Balance().Amount(),
		)
	}
}
