package account

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/pog7x/wallet-service/internal/money"
)

func TestNewAccount(t *testing.T) {
	expectedID, expectedAmount, expectedCurrency := "67534", int64(0), money.Currency("USD")

	acc := NewAccount(expectedID, expectedCurrency)

	if acc.id != expectedID {
		t.Errorf("account id not equal to expected id, %s != %s", expectedID, acc.id)
	}

	if acc.currency != expectedCurrency {
		t.Errorf("account currency not equal to expected currency, %s != %s", expectedCurrency, acc.currency)
	}

	if acc.Balance().Amount() != expectedAmount {
		t.Errorf("account amount not equal to expected amount, %d != %d", expectedAmount, acc.Balance().Amount())
	}
}

func TestDeposit(t *testing.T) {
	testCurrency := money.Currency("USD")
	tests := []struct {
		name        string
		amount      money.Money
		expectedErr error
	}{
		{"ok", money.New(5000, testCurrency), nil},
		{"error currency mismatch", money.New(5000, "EUR"), ErrCurrencyMismatch},
		{"error non positive amount", money.New(-5000, testCurrency), ErrNonPositiveAmount},
		{"error zero amount", money.New(0, testCurrency), ErrNonPositiveAmount},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			acc := NewAccount("342314321", testCurrency)

			err := acc.Deposit(tt.amount)
			if !errors.Is(err, tt.expectedErr) {
				t.Errorf("Deposit(%q) unexpected error, want %q got %q", tt.amount, tt.expectedErr, err)
			}
		})
	}
}

func TestWithdraw(t *testing.T) {
	testCurrency := money.Currency("USD")
	tests := []struct {
		name          string
		depositAmount money.Money
		amount        money.Money
		expectedErr   error
	}{
		{"ok", money.New(5001, testCurrency), money.New(5000, testCurrency), nil},
		{"ok zero balance", money.New(5000, testCurrency), money.New(5000, testCurrency), nil},
		{"error currency mismatch", money.New(5000, testCurrency), money.New(4000, "EUR"), ErrCurrencyMismatch},
		{"error non positive amount", money.New(5000, testCurrency), money.New(-5000, testCurrency), ErrNonPositiveAmount},
		{"error zero amount", money.New(5000, testCurrency), money.New(0, testCurrency), ErrNonPositiveAmount},
		{"error insufficient funds", money.New(5000, testCurrency), money.New(5001, testCurrency), ErrInsufficientFunds},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			acc := NewAccount("342314321", testCurrency)
			_ = acc.Deposit(tt.depositAmount)

			err := acc.Withdraw(tt.amount)
			if !errors.Is(err, tt.expectedErr) {
				t.Errorf("Withdraw(%q) unexpected error, want %q got %q", tt.amount, tt.expectedErr, err)
			}
		})
	}
}

func TestBalance(t *testing.T) {
	expectedAmount, expectedCurrency := int64(0), money.Currency("USD")

	acc := NewAccount("1111", expectedCurrency)

	bal := acc.Balance()

	if bal.Amount() != expectedAmount {
		t.Errorf("account amount not equal to expected amount, %d != %d", expectedAmount, acc.Balance().Amount())
	}
	if bal.Currency() != expectedCurrency {
		t.Errorf("account currency not equal to expected currency, %s != %s", expectedCurrency, bal.Currency())
	}
}

func TestDepositWithdrawOperations(t *testing.T) {
	testCurrency := money.Currency("USD")

	for i, maxAmount := range []int{1, 5, 15, 100} {
		seed1, seed2 := uint64(i), uint64(maxAmount)
		rng := rand.New(rand.NewPCG(seed1, seed2))

		t.Run(fmt.Sprintf("max amount=%d", maxAmount), func(t *testing.T) {
			acc := NewAccount("342314321", testCurrency)

			for i := range 100 {
				r := rng.IntN(2*maxAmount+1) - maxAmount

				if i%2 == 0 {
					_ = acc.Deposit(money.New(int64(r), testCurrency))
				} else {
					_ = acc.Withdraw(money.New(int64(r), testCurrency))
				}
			}

			if acc.Balance().Amount() < 0 {
				t.Errorf("invalid behavior: account balance amount less than 0, seed1: %d, seed2 %d", seed1, seed2)
			}
		})
	}
}
