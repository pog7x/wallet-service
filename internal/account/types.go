// Package account provides a wallet account bound to a single currency.
// It supports deposits and withdrawals and guarantees that the balance
// never becomes negative.
package account

import (
	"errors"
	"fmt"

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
	// ErrAccountNotFound indicates that no account exists for the requested
	// identifier.
	ErrAccountNotFound = errors.New("account: account not found")
	// ErrSameAccount indicates that a transfer was requested with the
	// same source and destination account and was therefore rejected.
	ErrSameAccount = errors.New("account: same source and destination account")
)

// operation identifies the account operation during which an error occurred.
// It is carried by RepositoryError and ServiceError so that a failure can be
// attributed to a specific operation without parsing the error text.
//
// The zero value is opUnknown and denotes an unset operation; it is never a
// valid operation and its presence in an error indicates that the operation
// was not recorded.
type operation int

const (
	opUnknown operation = iota
	opLoad
	opSave
	opTransfer
)

// String returns the lowercase name of the operation for use in error text and
// logs. Unknown values, including opUnknown, render as "unknown", so an
// unrecorded operation is visible rather than blank.
func (o operation) String() string {
	switch o {
	case opLoad:
		return "load"
	case opSave:
		return "save"
	case opTransfer:
		return "transfer"
	default:
		return "unknown"
	}
}

// RepositoryError reports a failure of a repository operation and identifies
// the operation and the account it concerned. It wraps the underlying cause,
// which may be a domain sentinel such as ErrAccountNotFound or an error from
// the storage layer such as a cancelled context.
//
// The wrapped error is reachable with errors.Is and errors.As through Unwrap,
// so callers must match causes with those functions rather than comparing the
// error directly. The error text carries the internal account identifier for
// diagnosis; it never carries balances or amounts.
type RepositoryError struct {
	Op        operation
	AccountID string
	Err       error
}

// Error reports the operation, the account identifier and the wrapped cause.
// The identifier is an internal one and is safe to log; no monetary values
// appear in the text.
func (e *RepositoryError) Error() string {
	return fmt.Sprintf("%s id='%s': %v", e.Op, e.AccountID, e.Err)
}

// Unwrap returns the wrapped cause so that errors.Is and errors.As can inspect
// it. It is the sole mechanism by which the wrapped error is exposed; without
// it the cause would be unreachable and the error would be a dead end in the
// chain.
func (e *RepositoryError) Unwrap() error {
	return e.Err
}

// InsufficientFundsError reports that a withdrawal was rejected because the
// balance was smaller than the requested amount. It carries the requested
// amount and the available balance so that a caller can act on the exact
// values, retrieving them with errors.As.
//
// The amounts live in fields only and never appear in the error text, because
// the text is expected to reach logs, where balances must not be exposed. A
// caller that surfaces these values elsewhere, such as an API response, is
// responsible for deciding whether the recipient is allowed to see them.
//
// InsufficientFundsError satisfies errors.Is(err, ErrInsufficientFunds): every
// instance reports itself as equal to that sentinel regardless of its field
// values, so existing checks against the sentinel keep working after the
// change from a plain sentinel to a data-carrying error.
type InsufficientFundsError struct {
	Requested, Available money.Money
}

// Error returns the same text as ErrInsufficientFunds, deliberately without the
// amounts, so that the message is safe to log. The amounts are available
// through the struct fields for callers that need them.
func (e *InsufficientFundsError) Error() string {
	return ErrInsufficientFunds.Error()
}

// Is reports whether target is ErrInsufficientFunds, which makes every
// InsufficientFundsError match that sentinel under errors.Is. target is the
// error being searched for, not a wrapped cause, so it is compared directly
// rather than unwrapped.
func (e *InsufficientFundsError) Is(target error) bool {
	return target == ErrInsufficientFunds
}

// ServiceError reports a failure of a service operation and identifies the
// operation and the source and destination accounts it concerned. It wraps the
// underlying cause, which may be a domain sentinel, an InsufficientFundsError,
// or a RepositoryError raised by the storage the service called.
//
// The wrapped error is reachable with errors.Is and errors.As through Unwrap,
// so a service error raised around a repository error still matches the
// original domain cause. The error text carries the internal account
// identifiers for diagnosis and never carries amounts.
type ServiceError struct {
	Op           operation
	FromID, ToID string
	Err          error
}

// Error reports the operation, both account identifiers and the wrapped cause.
// The identifiers are internal ones and are safe to log; no monetary values
// appear in the text.
func (e *ServiceError) Error() string {
	return fmt.Sprintf("%s from %s to %s: %v", e.Op, e.FromID, e.ToID, e.Err)
}

// Unwrap returns the wrapped cause so that errors.Is and errors.As can traverse
// past this error to the cause the service wrapped.
func (e *ServiceError) Unwrap() error {
	return e.Err
}

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
		return &InsufficientFundsError{Requested: amount, Available: a.balance}
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
