package main

import (
	"bytes"
	"testing"
)

func TestRun(t *testing.T) {
	var buf bytes.Buffer
	expectedOutPut := "account id1 balance: 20.00 USD\naccount id2 balance: 5.55 USD\n"

	err := run(&buf)
	if err != nil {
		t.Errorf("run() got unexpected error %q", err)
	}

	if buf.String() != expectedOutPut {
		t.Errorf("run() got buffer output, want %s got %s", expectedOutPut, buf.String())
	}
}
