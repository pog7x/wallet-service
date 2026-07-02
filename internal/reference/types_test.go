package reference

import (
	"errors"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectedRes string
		expectedErr error
	}{
		{"ASCII x 35", "HelloHelloHelloHelloHelloHelloHello", "hellohellohellohellohellohellohello", nil},
		{"ASCII x 34 + emoji", "HelloHelloHelloHelloHelloHelloHell😊", "hellohellohellohellohellohellohell😊", nil},
		{"ASCII x 36", "HelloHelloHelloHelloHelloHelloHello1", "HelloHelloHelloHelloHelloHelloHello1", ErrInvalidReference},
		{"cirilic x 35", "ПриветПриветПриветПриветПриветПриве", "приветприветприветприветприветприве", nil},
		{"cirilic x 34 + emoji", "ПриветПриветПриветПриветПриветПрив😊", "приветприветприветприветприветприв😊", nil},
		{"cirilic x 36", "ПриветПриветПриветПриветПриветПривет", "ПриветПриветПриветПриветПриветПривет", ErrInvalidReference},
		{"large emoji x 17", "❤️❤️❤️❤️❤️❤️❤️❤️❤️❤️❤️❤️❤️❤️❤️❤️❤️", "❤️❤️❤️❤️❤️❤️❤️❤️❤️❤️❤️❤️❤️❤️❤️❤️❤️", nil},
		{"large emoji x 18", "❤️❤️❤️❤️❤️❤️❤️❤️❤️❤️❤️❤️❤️❤️❤️❤️❤️❤️", "❤️❤️❤️❤️❤️❤️❤️❤️❤️❤️❤️❤️❤️❤️❤️❤️❤️❤️", ErrInvalidReference},
		{"leading control symbol", "\nПривет", "\nПривет", ErrInvalidReference},
		{"trailing control symbol", "Привет\t", "Привет\t", ErrInvalidReference},
		{"middle control symbol", "При\nвет", "При\nвет", ErrInvalidReference},
		{"spaces in cirilic", " При вет ", "привет", nil},
		{"spaces in ASCII", " He llo ", "hello", nil},
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
