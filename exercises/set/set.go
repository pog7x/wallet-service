// Package set provides a generic Set implementation for comparable types.
//
// The type parameter T must be comparable. If T is an interface type,
// the dynamic value stored in the Set must itself be comparable.
// Storing a non‑comparable dynamic value (e.g., []int, map, or func)
// will panic at runtime when the value is added (Add) or checked for
// membership (Contains). It is the caller's responsibility to ensure
// that only values with comparable dynamic types are ever inserted.
package set

// Set is a collection of unique elements of type T.
//
// T must be comparable. If T is an interface, the concrete type of any
// stored value must also be comparable; otherwise, methods that perform
// equality checks or use the value as a map key will panic.
type Set[T comparable] struct {
	items map[T]struct{}
}

// Add inserts v into the Set.
// If v is already present, the Set remains unchanged.
//
// Panics if T is an interface and the dynamic type of v is not comparable,
// because v is used as a map key internally.
func (s *Set[T]) Add(v T) {
	s.items[v] = struct{}{}
}

// Contains reports whether v is present in the Set.
//
// Panics if T is an interface and the dynamic type of v is not comparable,
// because v is compared with existing elements using ==.
func (s *Set[T]) Contains(v T) bool {
	for key := range s.items {
		if key == v {
			return true
		}
	}
	return false
}

// Remove deletes v from the Set.
// If v is not present, the Set remains unchanged.
//
// This method does not panic even if v's dynamic type is non‑comparable,
// because delete does not compare values. However, if v was previously
// added and caused a panic, the Set may be in an inconsistent state.
func (s *Set[T]) Remove(v T) {
	delete(s.items, v)
}

// Len returns the number of elements in the Set.
func (s *Set[T]) Len() int { // исправлен лишний параметр v
	return len(s.items)
}

// Items returns a slice containing all elements of the Set.
// The order is unspecified.
func (s *Set[T]) Items() []T { // исправлен лишний параметр v
	items := make([]T, 0, len(s.items))
	for key := range s.items {
		items = append(items, key)
	}
	return items
}
