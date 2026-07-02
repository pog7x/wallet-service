package reference

import (
	"errors"
	"testing"
)

func TestRuneLen(t *testing.T) {
	s, runeLen, bytesLen := "Привет", 6, 12
	if Reference(s).RuneLen() != runeLen && len(s) != bytesLen {
		t.Errorf("RuneLen(%s) unexpected length", s)
	}
}

func TestParse(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectedRes string
		expectedErr error
	}{
		{"ASCII x 35", "HelloHelloHelloHelloHelloHelloHello", "HelloHelloHelloHelloHelloHelloHello", nil},
		{"ASCII x 34 + emoji", "HelloHelloHelloHelloHelloHelloHell😊", "HelloHelloHelloHelloHelloHelloHell😊", nil},
		{"ASCII x 36", "HelloHelloHelloHelloHelloHelloHello1", "", ErrInvalidReference},
		{"cyrilic x 35", "ПриветПриветПриветПриветПриветПриве", "ПриветПриветПриветПриветПриветПриве", nil},
		{"cyrilic x 34 + emoji", "ПриветПриветПриветПриветПриветПрив😊", "ПриветПриветПриветПриветПриветПрив😊", nil},
		{"cyrilic x 36", "ПриветПриветПриветПриветПриветПривет", "", ErrInvalidReference},
		{"large emoji x 17", "❤️❤️❤️❤️❤️❤️❤️❤️❤️❤️❤️❤️❤️❤️❤️❤️❤️", "❤️❤️❤️❤️❤️❤️❤️❤️❤️❤️❤️❤️❤️❤️❤️❤️❤️", nil},
		{"large emoji x 18", "❤️❤️❤️❤️❤️❤️❤️❤️❤️❤️❤️❤️❤️❤️❤️❤️❤️❤️", "", ErrInvalidReference},
		{"leading control symbol", "\nПривет", "Привет", nil},
		{"trailing control symbol", "Привет\t", "Привет", nil},
		{"middle control symbol", "При\nвет", "", ErrInvalidReference},
		{"spaces in cyrilic", " При вет ", "При вет", nil},
		{"spaces in ASCII", " He llo ", "He llo", nil},
		{"only spaces", "    ", "", ErrInvalidReference},
		{"empty string", "", "", ErrInvalidReference},
		{"emoji", "  ❤️  ", "❤️", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.input)
			if !errors.Is(err, tt.expectedErr) {
				t.Errorf("Parse(%q) unexpected error, want %q got %q", tt.input, tt.expectedErr, err)
			}
			if got != Reference(tt.expectedRes) {
				t.Errorf("Parse(%q) unexpected result, want %s got %s", tt.input, tt.expectedRes, got)
			}
		})
	}
}
