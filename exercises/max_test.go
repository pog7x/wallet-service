package main

import "testing"

type Amount int64

func TestMaxWithAmount(t *testing.T) {
	a := Amount(10)
	b := Amount(20)
	got := Max(a, b)
	want := Amount(20)
	if got != want {
		t.Errorf("Max(%v, %v) = %v, want %v", a, b, got, want)
	}
}
