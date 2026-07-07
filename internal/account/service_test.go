package account

import (
	"errors"
	"sync"
	"testing"

	"github.com/pog7x/wallet-service/internal/money"
)

// transferCurrency is the single currency used to fund accounts in these
// tests. A distinct name avoids colliding with helpers in other test files.
const transferCurrency = money.Currency("USD")

// newFundedRepo builds a MemRepository seeded with accounts.
// balances maps an account id to its initial balance in minor units,
// all in transferCurrency. It fails the test if any seeding step errors,
// because a broken setup must not be reported as a Transfer failure.
func newFundedRepo(t *testing.T, balances map[string]int64) *MemRepository {
	t.Helper()
	repo := NewMemRepository()
	for id, amount := range balances {
		acc := NewAccount(id, transferCurrency)
		if amount > 0 {
			if err := acc.Deposit(money.New(amount, transferCurrency)); err != nil {
				t.Fatalf("seed deposit for %q: %v", id, err)
			}
		}
		if err := repo.Save(acc); err != nil {
			t.Fatalf("seed save for %q: %v", id, err)
		}
	}
	return repo
}

// mustBalance loads an account and returns its balance in minor units,
// failing the test if the account cannot be loaded.
func mustBalance(t *testing.T, repo *MemRepository, id string) int64 {
	t.Helper()
	acc, err := repo.Load(id)
	if err != nil {
		t.Fatalf("load %q: %v", id, err)
	}
	return acc.Balance().Amount()
}

// TestTransfer_Success checks the core invariant: money moves between the two
// accounts but the total across the system is unchanged. This is what no
// component-level test can verify, because it concerns two accounts at once.
func TestTransfer_Success(t *testing.T) {
	repo := newFundedRepo(t, map[string]int64{"A": 10000, "B": 5000})
	svc := NewService(repo)

	before := mustBalance(t, repo, "A") + mustBalance(t, repo, "B")

	if err := svc.Transfer("A", "B", money.New(3000, transferCurrency)); err != nil {
		t.Fatalf("Transfer: unexpected error: %v", err)
	}

	gotA := mustBalance(t, repo, "A")
	gotB := mustBalance(t, repo, "B")

	if gotA != 7000 {
		t.Errorf("source balance = %d, want %d", gotA, 7000)
	}
	if gotB != 8000 {
		t.Errorf("destination balance = %d, want %d", gotB, 8000)
	}
	if after := gotA + gotB; before != after {
		t.Errorf("money not conserved: before %d, after %d", before, after)
	}
}

// TestTransfer_InsufficientFunds checks that a rejected transfer leaves BOTH
// balances untouched. This proves there was no partial application, where the
// debit or credit happened but the other half did not.
func TestTransfer_InsufficientFunds(t *testing.T) {
	repo := newFundedRepo(t, map[string]int64{"A": 1000, "B": 5000})
	svc := NewService(repo)

	err := svc.Transfer("A", "B", money.New(2000, transferCurrency))
	if !errors.Is(err, ErrInsufficientFunds) {
		t.Fatalf("Transfer error = %v, want ErrInsufficientFunds", err)
	}

	if got := mustBalance(t, repo, "A"); got != 1000 {
		t.Errorf("source balance changed on failed transfer: got %d, want %d", got, 1000)
	}
	if got := mustBalance(t, repo, "B"); got != 5000 {
		t.Errorf("destination balance changed on failed transfer: got %d, want %d", got, 5000)
	}
}

// TestTransfer_SourceNotFound checks that a missing source account is reported
// and does not alter the existing destination account.
func TestTransfer_SourceNotFound(t *testing.T) {
	repo := newFundedRepo(t, map[string]int64{"B": 5000})
	svc := NewService(repo)

	err := svc.Transfer("ghost", "B", money.New(1000, transferCurrency))
	if !errors.Is(err, ErrAccountNotFound) {
		t.Fatalf("Transfer error = %v, want ErrAccountNotFound", err)
	}
	if got := mustBalance(t, repo, "B"); got != 5000 {
		t.Errorf("destination balance changed when source missing: got %d, want %d", got, 5000)
	}
}

// TestTransfer_DestinationNotFound probes operation order. If Transfer debits
// the source before confirming the destination exists, the source will be
// changed here and this test will fail, revealing the atomicity gap. If both
// accounts are loaded up front before any mutation, the source stays intact.
func TestTransfer_DestinationNotFound(t *testing.T) {
	repo := newFundedRepo(t, map[string]int64{"A": 5000})
	svc := NewService(repo)

	err := svc.Transfer("A", "ghost", money.New(1000, transferCurrency))
	if !errors.Is(err, ErrAccountNotFound) {
		t.Fatalf("Transfer error = %v, want ErrAccountNotFound", err)
	}
	if got := mustBalance(t, repo, "A"); got != 5000 {
		t.Errorf("source balance changed when destination missing: got %d, want %d", got, 5000)
	}
}

// TestTransfer_SameAccount checks that a transfer to the same account is
// rejected before any mutation, so the balance is left unchanged.
func TestTransfer_SameAccount(t *testing.T) {
	repo := newFundedRepo(t, map[string]int64{"A": 5000})
	svc := NewService(repo)

	err := svc.Transfer("A", "A", money.New(1000, transferCurrency))
	if !errors.Is(err, ErrSameAccount) {
		t.Fatalf("Transfer error = %v, want ErrSameAccount", err)
	}
	if got := mustBalance(t, repo, "A"); got != 5000 {
		t.Errorf("balance changed on same-account transfer: got %d, want %d", got, 5000)
	}
}

// TestTransfer_CurrencyMismatch checks that an amount in a currency different
// from the accounts is rejected and leaves both balances untouched.
func TestTransfer_CurrencyMismatch(t *testing.T) {
	repo := newFundedRepo(t, map[string]int64{"A": 5000, "B": 5000})
	svc := NewService(repo)

	err := svc.Transfer("A", "B", money.New(1000, money.Currency("EUR")))
	if !errors.Is(err, ErrCurrencyMismatch) {
		t.Fatalf("Transfer error = %v, want ErrCurrencyMismatch", err)
	}
	if got := mustBalance(t, repo, "A"); got != 5000 {
		t.Errorf("source balance changed on currency mismatch: got %d, want %d", got, 5000)
	}
	if got := mustBalance(t, repo, "B"); got != 5000 {
		t.Errorf("destination balance changed on currency mismatch: got %d, want %d", got, 5000)
	}
}

func TestTransferConcurrent(t *testing.T) {
	const (
		fromAmount = 1_000_000
		maxWorkers = 200
		perWorker  = 100
		amount     = 1
	)

	fromAcc := Account{balance: money.New(int64(fromAmount), "USD"), currency: "USD", id: "1"}
	toAcc := Account{balance: money.New(0, "USD"), currency: "USD", id: "2"}

	repo := MemRepository{accMap: map[string]Account{"1": fromAcc, "2": toAcc}}
	svc := NewService(&repo)

	wg := sync.WaitGroup{}
	wg.Add(maxWorkers)

	for range maxWorkers {
		go func() {
			defer wg.Done()
			for range perWorker {
				if err := svc.Transfer("1", "2", money.New(amount, "USD")); err != nil {
					t.Errorf("unexpected transfer error: %v", err)
					return
				}
			}
		}()
	}

	wg.Wait()

	moved := int64(maxWorkers * perWorker * amount)
	if got := repo.accMap["1"].balance.Amount(); got != fromAmount-moved {
		t.Errorf("from balance: want %d, got %d", fromAmount-moved, got)
	}
	if got := repo.accMap["2"].balance.Amount(); got != moved {
		t.Errorf("to balance: want %d, got %d", moved, got)
	}
}
