package main

type Account struct {
	id      int64
	balance int64
}

func (a *Account) Size() int {
	return int(a.balance & 0x3F)
}

type Sized interface {
	Size() int
}

func TotalSizeGeneric[T Sized](xs []T) int {
	total := 0
	for _, x := range xs {
		total += x.Size()
	}
	return total
}

func TotalSizeConcrete(xs []*Account) int {
	total := 0
	for _, x := range xs {
		total += x.Size()
	}
	return total
}
