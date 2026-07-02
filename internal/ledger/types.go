// Package ledger provides an in-memory journal of monetary entries and
// computes per-account balances.
//
// All entries for a single account must share one currency; mixing
// currencies on the same account violates the ledger invariant and is
// reported as an error.
package ledger

import (
	"errors"

	"github.com/pog7x/wallet-service/internal/money"
)

// ErrMixedCurrency indicates that an operation would combine amounts of
// different currencies on the same account, which the ledger forbids.
var ErrMixedCurrency = errors.New("ledger: mixed currencies on one account")

// Entry is a single immutable posting to an account: a signed amount
// recorded under a monotonically increasing sequence number.
type Entry struct {
	AccountID string
	Amount    money.Money
	Seq       int64
}

// Ledger is an in-memory, append-only journal of entries. Its zero value
// is not usable; create one with NewLedger.
type Ledger struct {
	entries map[string][]Entry
	nextSeq int64
}

// NewLedger returns an empty ledger ready to record entries.
func NewLedger() *Ledger {
	return &Ledger{entries: make(map[string][]Entry)}
}

// Record appends an entry for accountID with the given amount. It returns
// ErrMixedCurrency if the account already holds entries in a different
// currency.
func (l *Ledger) Record(accountID string, amount money.Money) error {
	if existing := l.entries[accountID]; len(existing) > 0 {
		existEntryCurrency := existing[0].Amount.Currency()
		if existEntryCurrency != amount.Currency() {
			return ErrMixedCurrency
		}

	}

	l.entries[accountID] = append(l.entries[accountID], Entry{AccountID: accountID, Amount: amount, Seq: l.nextSeq})
	l.nextSeq++

	return nil
}

// Balance returns the sum of all entries for accountID. An account with no
// entries has a zero balance and is not an error. It returns ErrMixedCurrency
// if the account's entries somehow span multiple currencies.
func (l *Ledger) Balance(accountID string) (money.Money, error) {
	entries := l.entries[accountID]
	if len(entries) == 0 {
		return money.Money{}, nil
	}

	var total = money.New(0, entries[0].Amount.Currency())
	var ok bool

	for _, entry := range entries {
		if total, ok = total.Add(entry.Amount); !ok {
			return money.Money{}, ErrMixedCurrency
		}
	}

	return total, nil
}
