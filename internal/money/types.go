// Package money provides an immutable value type for monetary amounts.
//
// Amounts are stored as an integer number of minor units (for example cents)
// in a fixed currency and never use floating point, so arithmetic is exact.
package money

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"
)

// Currency is an ISO-4217 currency code such as "USD", "EUR".
type Currency string

// Money is an immutable monetary amount stored as an integer number of
// minor units (cents) in a fixed currency. It never uses floating point.
type Money struct {
	amount   int64
	currency Currency
}

// New returns a Money value of amount minor units in the given currency.
func New(amount int64, currency Currency) Money {
	return Money{amount: amount, currency: currency}
}

// Amount returns the value in minor units (cents).
func (m Money) Amount() int64 {
	return m.amount
}

// Currency returns the currency of the amount.
func (m Money) Currency() Currency {
	return m.currency
}

// Add returns the sum of m and o. The bool is false when the currencies
// differ, in which case the returned Money is not meaningful.
func (m Money) Add(o Money) (Money, bool) {
	if m.currency != o.currency {
		return Money{}, false
	}

	if m.amount > math.MaxInt64-o.amount {
		return Money{}, false
	}

	return Money{amount: m.amount + o.amount, currency: m.currency}, true
}

// Sub returns m minus o. The bool is false when the currencies differ,
// in which case the returned Money is not meaningful.
func (m Money) Sub(o Money) (Money, bool) {
	if m.currency != o.currency {
		return Money{}, false
	}
	return Money{amount: m.amount - o.amount, currency: m.currency}, true
}

// Format returns the amount as a decimal string with its currency,
// for example "12.34 USD".
func (m Money) Format() string {
	absVal, prefix := m.Amount(), ""
	if m.Amount() < 0 {
		prefix = "-"
		absVal = -absVal
	}
	return fmt.Sprintf("%s%d.%02d %s", prefix, absVal/100, absVal%100, m.Currency())
}

// ParseMoney parses a decimal string with up to two fractional digits
// (for example "12.34") into Money of the given currency. The bool is
// false when s is not a valid non-negative amount.
func ParseMoney(s string, currency Currency) (Money, bool) {
	if strings.HasPrefix(s, "-") {
		return Money{}, false
	}

	split := strings.Split(s, ".")
	if len(split) > 2 {
		return Money{}, false
	}

	intPart, err := strconv.ParseInt(split[0], 10, 64)
	if err != nil {
		return Money{}, false
	}

	if intPart > math.MaxInt64/100 {
		return Money{}, false
	}

	intPart *= 100

	if len(split) == 1 {
		return Money{amount: intPart, currency: currency}, true
	}

	var frac string

	switch {
	case len(split[1]) == 1:
		frac = split[1] + "0"
	case len(split[1]) == 2:
		frac = split[1][0:2]
	case len(split[1]) > 2:
		return Money{}, false
	default:
		frac = "0"
	}

	for _, r := range frac {
		if !unicode.IsDigit(r) {
			return Money{}, false
		}
	}

	fracPart, err := strconv.ParseInt(frac, 10, 64)
	if err != nil {
		return Money{}, false
	}

	if intPart > math.MaxInt64-fracPart {
		return Money{}, false
	}

	return Money{amount: intPart + fracPart, currency: currency}, true
}
