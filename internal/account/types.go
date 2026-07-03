// Package account provides a wallet account bound to a single currency.
// It supports deposits and withdrawals and guarantees that the balance
// never becomes negative.
package account

import (
	"errors"

	"github.com/pog7x/wallet-service/internal/money"
)

var (
	// ErrCurrencyMismatch indicates that an operation was attempted with an
	// amount whose currency differs from the account's currency.
	ErrCurrencyMismatch = errors.New("account: currency mismatch")
	// ErrNonPositiveAmount indicates that a deposit or withdrawal was attempted
	// with an amount that is zero or negative.
	ErrNonPositiveAmount = errors.New("account: non positive amount")
	// ErrInsufficientFunds indicates that a withdrawal was rejected because it
	// would reduce the balance below zero.
	ErrInsufficientFunds = errors.New("account: insufficient funds")
)

// Account is a wallet account holding a balance in a single fixed currency.
// Its zero value is not usable; create an account with NewAccount. All
// methods use a pointer receiver, because Deposit and Withdraw mutate the
// balance and mixing value and pointer receivers would give Account and
// *Account different method sets.
type Account struct {
	balance  money.Money
	currency money.Currency
	id       string
}

// NewAccount returns a new account identified by id, with a zero balance
// in the given currency.
func NewAccount(id string, currency money.Currency) *Account {
	return &Account{balance: money.New(0, currency), currency: currency, id: id}
}

// Deposit adds amount to the balance. It returns ErrNonPositiveAmount if
// amount is not strictly positive, and ErrCurrencyMismatch if amount is in
// a different currency than the account.
func (a *Account) Deposit(amount money.Money) error {
	if a.currency != amount.Currency() {
		return ErrCurrencyMismatch
	}

	if amount.Amount() <= 0 {
		return ErrNonPositiveAmount
	}

	res, ok := a.balance.Add(amount)
	if !ok {
		return ErrCurrencyMismatch
	}

	a.balance = res
	return nil
}

// Withdraw subtracts amount from the balance. It returns ErrNonPositiveAmount
// if amount is not strictly positive, ErrCurrencyMismatch if the currencies
// differ, and ErrInsufficientFunds if the balance is smaller than amount.
func (a *Account) Withdraw(amount money.Money) error {
	if a.currency != amount.Currency() {
		return ErrCurrencyMismatch
	}

	if amount.Amount() <= 0 {
		return ErrNonPositiveAmount
	}

	if amount.Amount() > a.balance.Amount() {
		return ErrInsufficientFunds
	}

	res, ok := a.balance.Sub(amount)
	if !ok {
		return ErrCurrencyMismatch
	}

	a.balance = res
	return nil
}

// Balance returns a copy of the account's current balance.
func (a *Account) Balance() money.Money {
	return a.balance
}
