package reference

import (
	"errors"
	"strings"
	"unicode"
)

var ErrInvalidReference = errors.New("reference: invalid reference string")

type Reference string

func (r Reference) RuneLen() (n int) {
	for range r {
		n++
	}
	return n
}

func Parse(s string) (Reference, error) {
	var b strings.Builder
	var runeCount int

	for _, r := range s {
		if unicode.IsControl(r) {
			return Reference(s), ErrInvalidReference
		}
		if !unicode.IsSpace(r) {
			b.WriteRune(r)
			runeCount++
		}
	}

	if runeCount > 35 {
		return Reference(s), ErrInvalidReference
	}

	s = b.String()

	if s == "" {
		return Reference(s), ErrInvalidReference
	}

	return Reference(s), nil
}
