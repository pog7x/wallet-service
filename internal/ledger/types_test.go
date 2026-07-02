package ledger

import (
	"errors"
	"testing"

	"github.com/pog7x/wallet-service/internal/money"
)

func TestNewLedger(t *testing.T) {
	ld := NewLedger()

	if ld.entries == nil {
		t.Error("ledger entries must be not nil map")
	}
}

func TestRecordSuccess(t *testing.T) {
	ld := NewLedger()
	accID := "63546123"
	amountOne, amountTwo := int64(463), int64(726)
	expectedCurr := money.Currency("EUR")
	expectedSum := amountOne + amountTwo

	err := ld.Record(accID, money.New(amountOne, expectedCurr))
	if err != nil {
		t.Errorf("Got error %v", err)
	}

	err = ld.Record(accID, money.New(amountTwo, expectedCurr))
	if err != nil {
		t.Errorf("Got error %v", err)
	}

	if ld.entries[accID][0].Seq >= ld.entries[accID][1].Seq {
		t.Errorf("Got error %v", err)
	}

	err = ld.Record(accID, money.New(0, expectedCurr))
	if err != nil {
		t.Errorf("Got error %v", err)
	}

	bal, err := ld.Balance(accID)
	if err != nil {
		t.Errorf("Got error %v", err)
	}

	if expectedSum != bal.Amount() {
		t.Errorf("account amount is not equal to expected amount, %d != %d", expectedSum, bal.Amount())
	}

	if expectedCurr != bal.Currency() {
		t.Errorf("account currency is not equal to expected currency, %s != %s", expectedCurr, bal.Currency())
	}
}

func TestRecordFailed(t *testing.T) {
	ld := NewLedger()
	accID := "63546123"
	amountOne, amountTwo := int64(463), int64(726)
	expectedCurr := money.Currency("EUR")
	expectedSum := amountOne + amountTwo

	err := ld.Record(accID, money.New(amountOne, expectedCurr))
	if err != nil {
		t.Errorf("Got error %v", err)
	}

	err = ld.Record(accID, money.New(amountTwo, expectedCurr))
	if err != nil {
		t.Errorf("Got error %v", err)
	}

	err = ld.Record(accID, money.New(0, "RSD"))
	if !errors.Is(err, ErrMixedCurrency) {
		t.Errorf("Expected error %v but got %v", ErrMixedCurrency, err)
	}

	bal, err := ld.Balance(accID)
	if err != nil {
		t.Errorf("Got error %v", err)
	}

	if expectedSum != bal.Amount() {
		t.Errorf("account amount is not equal to expected amount, %d != %d", expectedSum, bal.Amount())
	}

	if expectedCurr != bal.Currency() {
		t.Errorf("account currency is not equal to expected currency, %s != %s", expectedCurr, bal.Currency())
	}
}

func TestBalanceEmptyLedger(t *testing.T) {
	ld := NewLedger()
	accID := "63546123"

	bal, err := ld.Balance(accID)
	if err != nil {
		t.Errorf("Got error %v", err)
	}

	if bal.Amount() != 0 {
		t.Errorf("account amount is not equal to expected amount, %d != %d", 0, bal.Amount())
	}

	if bal.Currency() != "" {
		t.Errorf("account currency is not equal to expected currency, %s != %s", "", bal.Currency())
	}
}
