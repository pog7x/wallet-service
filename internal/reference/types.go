// Package reference validates and normalizes human-entered payment
// references (the "purpose of payment" field).
//
// Length is bounded in Unicode code points (runes), not bytes, so a
// multibyte reference is measured by how many characters it contains
// rather than how many bytes it occupies.
package reference

import (
	"errors"
	"strings"
	"unicode"
)

// ErrInvalidReference indicates that a string is not a valid payment
// reference: it is empty after normalizing, exceeds MaxRunes runes, or
// contains a control character.
var ErrInvalidReference = errors.New("reference: invalid string")

// Reference is a validated, normalized payment reference of at most
// MaxRunes runes, free of control characters.
type Reference string

// MaxRunes is the maximum length of a Reference in runes (SEPA limit).
const MaxRunes = 35

// RuneLen returns the length of the reference in runes (characters),
// which differs from len(r) for any non-ASCII content.
func (r Reference) RuneLen() (n int) {
	for range r {
		n++
	}
	return n
}

// Parse normalizes surrounding whitespace and validates s as a payment
// reference. It returns error == ErrInvalidReference when the normalized value is empty,
// exceeds MaxRunes runes, or contains any control character. The length
// limit is counted in runes, so multibyte input is not unfairly rejected.
func Parse(s string) (Reference, error) {
	var b strings.Builder
	var runeCount int

	for _, r := range s {
		if unicode.IsControl(r) {
			return Reference(s), ErrInvalidReference
		}
		if !unicode.IsSpace(r) {
			b.WriteRune(unicode.ToLower(r))
			runeCount++
		}
	}

	if runeCount > MaxRunes {
		return Reference(s), ErrInvalidReference
	}

	s = b.String()

	if s == "" {
		return Reference(s), ErrInvalidReference
	}

	return Reference(s), nil
}
