// Package main is the entry point for the application.
package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/pog7x/wallet-service/internal/account"
	"github.com/pog7x/wallet-service/internal/money"
)

func run(w io.Writer) error {
	ctx := context.Background()
	mr := account.NewMemRepository()
	s := account.NewService(mr)

	fromAccID, toAccID := "id1", "id2"
	fromAcc, toAcc := account.NewAccount(fromAccID, money.Currency("USD")), account.NewAccount(toAccID, money.Currency("USD"))

	err := fromAcc.Deposit(money.New(2555, money.Currency("USD")))
	if err != nil {
		return err
	}

	err = mr.Save(ctx, fromAcc)
	if err != nil {
		return err
	}

	err = mr.Save(ctx, toAcc)
	if err != nil {
		return err
	}

	err = s.Transfer(ctx, fromAccID, toAccID, money.New(555, money.Currency("USD")))
	if err != nil {
		return err
	}

	fromAcc, err = mr.Load(ctx, fromAccID)
	if err != nil {
		return err
	}

	toAcc, err = mr.Load(ctx, toAccID)
	if err != nil {
		return err
	}

	fmt.Fprintf(w, "account %s balance: %s\n", fromAccID, fromAcc.Balance().Format())
	fmt.Fprintf(w, "account %s balance: %s\n", toAccID, toAcc.Balance().Format())

	return nil
}

func main() {
	if err := run(os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
