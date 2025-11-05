//go:build !goexperiment.arenas

// SPDX-License-Identifier: MIT
package mem

const MaxScratchChannels = 128 //== MAX_STAGE_CHANNELS in lcms

// Scratch holds reusable working buffers for hot paths.
// Preallocated once when the Manager is created.
type Scratch struct {
	LUT   [2][]float32 // len == MaxScratchChannels
	In16  []uint16     // len == MaxScratchChannels
	Out16 []uint16     // len == MaxScratchChannels
	  // new: tiny, tone-curve-only buffers, never used elsewhere
    ToneInU16  [1]uint16
    ToneOutU16 [1]uint16
    ToneInF32  [1]float32
    ToneOutF32 [1]float32
}

// Manager carries one reusable Scratch bundle (heap-backed).
// Safe to pass by value; field is a pointer.
type Manager struct {
	Sc *Scratch
}

// NewManager returns a heap-backed Manager with preallocated scratch.
func NewManager() Manager {
	s := &Scratch{
		LUT: [2][]float32{
			make([]float32, MaxScratchChannels),
			make([]float32, MaxScratchChannels),
		},
		In16:        make([]uint16, MaxScratchChannels),
		Out16:       make([]uint16, MaxScratchChannels),
		Tmp1U16:  make([]uint16, MaxScratchChannels),  // len == MaxScratchChannels
		Tmp2U16:  make([]uint16, MaxScratchChannels),  // len == MaxScratchChannels
		Tmp1F32: make([]float32, MaxScratchChannels), // len == MaxScratchChannels
		Tmp2F32: make([]float32, MaxScratchChannels), // len == MaxScratchChannels

	}
	return Manager{Sc: s}
}

// NewArena keeps API parity in non-arena builds (same as NewManager).
func NewArena() Manager { return NewManager() }

// Scratch returns the reusable scratch buffers.
func (m Manager) Scratch() *Scratch { return m.Sc }

// Generic helpers — heap allocations in this build.
func New[T any](m Manager) *T               { return new(T) }
func MakeSlice[T any](m Manager, n int) []T { return make([]T, n) }

// FreeAll is a no-op on the heap build.
func (Manager) FreeAll() {}

// Compatibility stub — there is no arena here; return nil.
func (Manager) GerArenaPtr() any { return nil }
// IsZero reports whether the Manager has no backing state.
// Passing a zero Manager means "please use the transform's manager".
func (m Manager) IsZero() bool { return m.Scratch() == nil || m.Sc == nil }
