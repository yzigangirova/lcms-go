//go:build !goexperiment.arenas

// SPDX-License-Identifier: MIT
package mem

// Manager allocates using the regular Go heap in this build.
type Manager struct{}

// NewManager returns a Manager using standard allocation.
func NewManager() Manager { return Manager{} }

// Generic helpers (package-level functions).
func New[T any](m Manager) *T               { return new(T) }
func MakeSlice[T any](m Manager, n int) []T { return make([]T, n) }

// FreeAll is a no-op on the heap build.
func (Manager) FreeAll() {}
