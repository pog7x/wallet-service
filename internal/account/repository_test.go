package account

import (
	"errors"
	"fmt"
	"sync"
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

	err := mr.Save(t.Context(), &Account{balance: expectedAmount, currency: expectedCurrency, id: expectedAccountID})
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
	ctx := t.Context()
	expectedAccountID, expectedCurrency := "654635634", money.Currency("USD")
	expectedAmount := money.New(int64(0), expectedCurrency)

	mr := NewMemRepository()
	mr.accMap[expectedAccountID] = Account{
		balance:  expectedAmount,
		currency: expectedCurrency,
		id:       expectedAccountID,
	}

	err := mr.Save(ctx, &Account{balance: money.New(int64(734643), "EUR"), currency: "EUR", id: expectedAccountID})
	if !errors.Is(err, ErrCurrencyMismatch) {
		t.Errorf("Save(*Account) unexpected error, want %v got %v", ErrCurrencyMismatch, err)
	}

	err = mr.Save(ctx, &Account{balance: money.New(int64(734643), "EUR"), currency: expectedCurrency, id: expectedAccountID})
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

	acc, err := mr.Load(t.Context(), expectedAccountID)
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

	_, err := mr.Load(t.Context(), expectedAccountID)
	if !errors.Is(err, ErrAccountNotFound) {
		t.Errorf("Load(%s) unexpected error, want %v got %v", expectedAccountID, ErrAccountNotFound, err)
	}
}

func TestMemRepositoryLoadDataNotChange(t *testing.T) {
	ctx := t.Context()
	expectedAccountID, expectedCurrency := "654635634", money.Currency("USD")
	expectedAmount := money.New(int64(734643), expectedCurrency)
	mr := NewMemRepository()

	mr.accMap[expectedAccountID] = Account{balance: expectedAmount, currency: expectedCurrency, id: expectedAccountID}

	acc, err := mr.Load(ctx, expectedAccountID)
	if err != nil {
		t.Errorf("Got unexpected error %v", err)
	}

	acc.balance = money.New(0, "EUR")
	acc.currency = money.Currency("EUR")
	acc.id = "changed id"

	acc, err = mr.Load(ctx, expectedAccountID)
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

// TestMemRepositoryLoadAndSaveConcurrent exercises concurrent Load and Save
// calls on a single MemRepository. It makes no assertions by design: the
// verdict comes from the race detector, so the test must be run with -race to
// be meaningful. Without -race the only remaining failure signal is the
// runtime's "concurrent map read and map write" panic, which is best-effort
// and not guaranteed on every run.
func TestMemRepositoryLoadAndSaveConcurrent(t *testing.T) {
	ctx := t.Context()
	mr := NewMemRepository()
	maxC := 200

	wg := sync.WaitGroup{}
	wg.Add(maxC)

	for i := range maxC {
		go func(c int) {
			defer wg.Done()
			testID := fmt.Sprintf("%d", c)
			_, _ = mr.Load(ctx, testID)
			testAcc := NewAccount(testID, "USD")
			_ = mr.Save(ctx, testAcc)
			_, _ = mr.Load(ctx, testID)
			testAcc.balance = money.New(int64(c), "USD")
			_ = mr.Save(ctx, testAcc)
			_, _ = mr.Load(ctx, testID)
			testAcc.balance = money.New(int64(c+30), "USD")
			_ = mr.Save(ctx, testAcc)
			_, _ = mr.Load(ctx, testID)
		}(i)
	}

	wg.Wait()
}
