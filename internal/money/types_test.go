package money

import "testing"

func TestNew(t *testing.T) {
	expectedAmount := int64(3476)
	expectedCurrency := Currency("EUR")
	money := New(expectedAmount, expectedCurrency)

	if money.Amount() != expectedAmount {
		t.Errorf("expected amount '%d' but got '%d'", expectedAmount, money.Amount())
	}
	if money.Currency() != expectedCurrency {
		t.Errorf("expected currency '%s' but got '%s'", expectedCurrency, money.Currency())
	}
}

func TestAdd(t *testing.T) {
	firstAmount, secondAmount, thirdAmount := int64(4554), int64(81346), int64(7346)
	expectedAmount := firstAmount + secondAmount
	expectedCurrency, badCurrency := Currency("EUR"), Currency("RUB")

	firstMoney := New(firstAmount, expectedCurrency)
	secondMoney := New(secondAmount, expectedCurrency)
	badMoney := New(thirdAmount, badCurrency)

	result, ok := firstMoney.Add(secondMoney)
	if !ok {
		t.Errorf("got not ok result")
	}
	if result.Amount() != expectedAmount {
		t.Errorf("expected amount '%d' but got '%d'", expectedAmount, result.Amount())
	}
	if result.Currency() != expectedCurrency {
		t.Errorf("expected currency '%s' but got '%s'", expectedCurrency, result.Currency())
	}

	if _, ok = firstMoney.Add(badMoney); ok {
		t.Error("expected error, but got ok")
	}
}

func TestSub(t *testing.T) {
	firstAmount, secondAmount, thirdAmount := int64(4554), int64(81346), int64(7346)
	expectedAmount := firstAmount - secondAmount
	expectedCurrency, badCurrency := Currency("EUR"), Currency("RUB")

	firstMoney := New(firstAmount, expectedCurrency)
	secondMoney := New(secondAmount, expectedCurrency)
	badMoney := New(thirdAmount, badCurrency)

	result, ok := firstMoney.Sub(secondMoney)
	if !ok {
		t.Errorf("got not ok result")
	}
	if result.Amount() != expectedAmount {
		t.Errorf("expected amount '%d' but got '%d'", expectedAmount, result.Amount())
	}
	if result.Currency() != expectedCurrency {
		t.Errorf("expected currency '%s' but got '%s'", expectedCurrency, result.Currency())
	}

	if _, ok = firstMoney.Sub(badMoney); ok {
		t.Error("expected error, but got ok")
	}
}

func TestParseMoney(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantAmount int64
		wantOK     bool
	}{
		{"integer only", "12", 1200, true},
		{"integer with dot", "12.", 1200, true},
		{"two decimals", "12.34", 1234, true},
		{"one decimal is tens", "0.5", 50, true},
		{"no integer part is tens it's OK", ".5", 50, true},
		{"leading zero decimal five cents", "0.05", 5, true},
		{"empty string", "", 0, false},
		{"negative", "-5000", 0, false},
		{"multiple dots", "12.3.4", 0, false},
		{"only dot", ".", 0, false},
		{"letters", "abc", 0, false},
		{"one character", "12.3x", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseMoney(tt.input, "USD")

			if ok != tt.wantOK {
				t.Fatalf("ParseMoney(%q) ok = %v, want %v", tt.input, ok, tt.wantOK)
			}
			if ok && got.Amount() != tt.wantAmount {
				t.Errorf("ParseMoney(%q) amount = %d, want %d",
					tt.input, got.Amount(), tt.wantAmount)
			}
		})
	}
}

func TestFormat(t *testing.T) {
	tests := []struct {
		money          Money
		expectedResult string
	}{
		{Money{amount: int64(1), currency: Currency("USD")}, "0.01 USD"},
		{Money{amount: int64(9), currency: Currency("USD")}, "0.09 USD"},
		{Money{amount: int64(19), currency: Currency("USD")}, "0.19 USD"},
		{Money{amount: int64(59), currency: Currency("USD")}, "0.59 USD"},
		{Money{amount: int64(90), currency: Currency("USD")}, "0.90 USD"},
		{Money{amount: int64(99), currency: Currency("USD")}, "0.99 USD"},
		{Money{amount: int64(100), currency: Currency("USD")}, "1.00 USD"},
		{Money{amount: int64(599), currency: Currency("USD")}, "5.99 USD"},
		{Money{amount: int64(999), currency: Currency("USD")}, "9.99 USD"},
		{Money{amount: int64(1235), currency: Currency("USD")}, "12.35 USD"},
		{Money{amount: int64(9999), currency: Currency("USD")}, "99.99 USD"},
		{Money{amount: int64(99999), currency: Currency("USD")}, "999.99 USD"},
		{Money{amount: int64(99999), currency: Currency("TRY")}, "999.99 TRY"},
	}

	for _, tt := range tests {
		t.Run(tt.expectedResult, func(t *testing.T) {
			result := tt.money.Format()

			if result != tt.expectedResult {
				t.Errorf("%v.Format() got result = %s, want %s", tt.money, result, tt.expectedResult)
			}
		})
	}
}
