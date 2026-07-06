package money_test

import (
	"fmt"

	"github.com/pog7x/wallet-service/internal/money"
)

func ExampleParseMoney() {
	m, ok := money.ParseMoney("12.34", "USD")
	fmt.Println(ok)
	fmt.Println(m.Format())
	// Output:
	// true
	// 12.34 USD
}

func ExampleMoney_Format() {
	m := money.New(2000, "USD")
	fmt.Println(m.Format())
	// Output: 20.00 USD
}
