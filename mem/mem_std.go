//go:build !goexperiment.arenas

// SPDX-License-Identifier: MIT
package mem

import "sync"

const MaxScratchChannels = 128     // == MAX_STAGE_CHANNELS in lcms
const MaxScratchChannelsShort = 16 // cmsMAXCHANNELS

// Scratch holds reusable working buffers for hot paths.
// Preallocated once when the Manager is created.
type Scratch struct {
	LUT     [2][]float32 // len == MaxScratchChannels
	In16    []uint16     // len == MaxScratchChannels
	Out16   []uint16     // len == MaxScratchChannels
	WInU16  []uint16     // len == MaxScratchChannelsShort
	WOutU16 []uint16     // len == MaxScratchChannelsShort
	WInF32  []float32    // len == MaxScratchChannelsShort
	WOutF32 []float32    // len == MaxScratchChannelsShort
	Tmp1U16 []uint16     // len == MaxScratchChannels
	Tmp2U16 []uint16     // len == MaxScratchChannels
	Tmp1F32 []float32    // len == MaxScratchChannels
	Tmp2F32 []float32    // len == MaxScratchChannels

	// tiny, tone-curve-only buffers, never used elsewhere
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

// ---------- internal helpers ----------

var heapScratchPool = sync.Pool{
	New: func() any { return newHeapScratch() },
}

func newHeapScratch() *Scratch {
	return &Scratch{
		LUT: [2][]float32{
			make([]float32, MaxScratchChannels),
			make([]float32, MaxScratchChannels),
		},
		In16:    make([]uint16, MaxScratchChannels),
		Out16:   make([]uint16, MaxScratchChannels),
		WInU16:  make([]uint16, MaxScratchChannelsShort),
		WOutU16: make([]uint16, MaxScratchChannelsShort),
		WInF32:  make([]float32, MaxScratchChannelsShort),
		WOutF32: make([]float32, MaxScratchChannelsShort),
		Tmp1U16: make([]uint16, MaxScratchChannels),
		Tmp2U16: make([]uint16, MaxScratchChannels),
		Tmp1F32: make([]float32, MaxScratchChannels),
		Tmp2F32: make([]float32, MaxScratchChannels),
	}
}

// ---------- public API (existing) ----------

// NewManager returns a heap-backed Manager with preallocated scratch.
func NewManager() Manager {
	return Manager{Sc: newHeapScratch()}
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

// ---------- new: frames ----------

// NewFrame returns a child Manager with its own Scratch bundle.
// Heap-backed: Scratch objects are taken from a pool (cheap, no per-pixel allocs).
func (m Manager) NewFrame() Manager {
	return Manager{Sc: heapScratchPool.Get().(*Scratch)}
}

// Close returns the Scratch to the pool in this build.
func (m Manager) Close() {
	if m.Sc != nil {
		// (optional) zero small pieces if you want, but not required
		heapScratchPool.Put(m.Sc)
	}
}

// WithFrame runs fn with a child Manager and closes it on return.
func (m Manager) WithFrame(fn func(Manager)) {
	child := m.NewFrame()
	defer child.Close()
	fn(child)
}
