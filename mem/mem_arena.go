//go:build goexperiment.arenas

// SPDX-License-Identifier: MIT
package mem

import "arena"

// Manager optionally holds an arena. If nil, it falls back to heap.
type Manager struct{ a *arena.Arena }

// NewManager returns a Manager using standard heap allocation.
func NewManager() Manager { return Manager{} }

// NewArena returns a Manager backed by a fresh arena.
func NewArena() Manager { return Manager{a: arena.NewArena()} }

// Generic helpers (package-level) allocate from the arena when present.
func New[T any](m Manager) *T {
	if m.a != nil {
		return arena.New[T](m.a)
	}
	return new(T)
}

func MakeSlice[T any](m Manager, n int) []T {
	if m.a != nil {
		return arena.MakeSlice[T](m.a, n, n)
	}
	return make([]T, n)
}

// FreeAll releases the arena, if any.
func (m Manager) FreeAll() {
	if m.a != nil {
		m.a.Free()
	}
}
func (m Manager) GerArenaPtr() *arena.Arena {
	return m.a
}
