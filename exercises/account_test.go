package main

import (
	"testing"
)

func makeAccounts(n int) []*Account {
	res := make([]*Account, 0, n)

	for i := range n {
		res = append(res, &Account{id: int64(i), balance: int64(i * 1000)})
	}

	return res
}

func BenchmarkTotalSize(b *testing.B) {
	cases := []struct {
		name string
		f    func(xs []*Account)
	}{
		{"generic", func(xs []*Account) { _ = TotalSizeGeneric(xs) }},
		{"concrete", func(xs []*Account) { _ = TotalSizeConcrete(xs) }},
	}

	for _, cc := range cases {
		b.Run(cc.name, func(b *testing.B) {
			xs := makeAccounts(1000)
			b.ResetTimer()
			b.ReportAllocs()
			for b.Loop() {
				cc.f(xs)
			}
		})
	}
}
