package set

import (
	"slices"
	"testing"
)

func TestSet_Add(t *testing.T) {
	s := New[int]()

	toSet := 1

	s.Add(toSet)
	s.Add(toSet)
	s.Add(toSet)

	if _, ok := s.items[toSet]; !ok {
		t.Errorf("want = %t, got = %t", true, ok)
	}
	if len(s.items) != 1 {
		t.Errorf("want len = %d, got = %d", 1, s.Len())
	}
}

func TestSet_Contains(t *testing.T) {
	s := New[int]()

	toSet := 1

	s.Add(toSet)
	if ok := s.Contains(toSet); !ok {
		t.Errorf("want = %t, got = %t", true, ok)
	}
}

func TestSet_Remove(t *testing.T) {
	s := New[int]()

	toSet := 1

	s.Add(toSet)
	s.Remove(toSet)
	if ok := s.Contains(toSet); ok {
		t.Errorf("want = %t, got = %t", false, ok)
	}
}

func TestSet_Len(t *testing.T) {
	s := New[int]()

	toSet := 1

	s.Add(toSet)

	if s.Len() != 1 {
		t.Errorf("want = %d, got = %d", 1, s.Len())
	}
}

func TestSet_Items(t *testing.T) {
	s := New[int]()
	itemsLen := 10
	want := make([]int, 0, itemsLen)
	for i := range itemsLen {
		s.Add(i)
		want = append(want, i)
	}
	got := s.Items()
	slices.Sort(got) // порядок Items() не специфицирован
	if !slices.Equal(got, want) {
		t.Errorf("want = %v, got = %v", want, got)
	}
}
