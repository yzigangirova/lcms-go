//go:build goexperiment.arenas

package mem

import (
	"arena"
	"sync"
)

const MaxScratchChannels = 128 // == MAX_STAGE_CHANNELS in lcms
const MaxScratchChannelsShort = 16 // cmsMAXCHANNELS

// Scratch holds reusable working buffers for hot paths.
type Scratch struct {
	LUT       [2][]float32 // len == MaxScratchChannels
	In16      []uint16     // len == MaxScratchChannels
	Out16     []uint16     // len == MaxScratchChannels
	WInU16    []uint16     // len == MaxScratchChannelsShort
	WOutU16   []uint16     // len == MaxScratchChannelsShort
	WInF32    []float32    // len == MaxScratchChannelsShort
	WOutF32   []float32    // len == MaxScratchChannelsShort
	Tmp1U16   []uint16     // len == MaxScratchChannels
	Tmp2U16   []uint16     // len == MaxScratchChannels
	Tmp1F32   []float32    // len == MaxScratchChannels
	Tmp2F32   []float32    // len == MaxScratchChannels

	// tiny tone-curve-only buffers
	ToneInU16  [1]uint16
	ToneOutU16 [1]uint16
	ToneInF32  [1]float32
	ToneOutF32 [1]float32
}

// Manager carries an optional arena and one reusable Scratch bundle.
type Manager struct {
	A  *arena.Arena
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
		Tmp1U16: make([]uint16, MaxScratchChannels),
		Tmp2U16: make([]uint16, MaxScratchChannels),
		Tmp1F32: make([]float32, MaxScratchChannels),
		Tmp2F32: make([]float32, MaxScratchChannels),
		WInU16:  make([]uint16, MaxScratchChannelsShort),
		WOutU16: make([]uint16, MaxScratchChannelsShort),
		WInF32:  make([]float32, MaxScratchChannelsShort),
		WOutF32: make([]float32, MaxScratchChannelsShort),
	}
}

func newArenaScratch(a *arena.Arena) *Scratch {
	return &Scratch{
		LUT: [2][]float32{
			arena.MakeSlice[float32](a, MaxScratchChannels, MaxScratchChannels),
			arena.MakeSlice[float32](a, MaxScratchChannels, MaxScratchChannels),
		},
		In16:    arena.MakeSlice[uint16](a, MaxScratchChannels, MaxScratchChannels),
		Out16:   arena.MakeSlice[uint16](a, MaxScratchChannels, MaxScratchChannels),
		WInU16:  arena.MakeSlice[uint16](a, MaxScratchChannelsShort, MaxScratchChannelsShort),
		WOutU16: arena.MakeSlice[uint16](a, MaxScratchChannelsShort, MaxScratchChannelsShort),
		WInF32:  arena.MakeSlice[float32](a, MaxScratchChannelsShort, MaxScratchChannelsShort),
		WOutF32: arena.MakeSlice[float32](a, MaxScratchChannelsShort, MaxScratchChannelsShort),
		Tmp1U16: arena.MakeSlice[uint16](a, MaxScratchChannels, MaxScratchChannels),
		Tmp2U16: arena.MakeSlice[uint16](a, MaxScratchChannels, MaxScratchChannels),
		Tmp1F32: arena.MakeSlice[float32](a, MaxScratchChannels, MaxScratchChannels),
		Tmp2F32: arena.MakeSlice[float32](a, MaxScratchChannels, MaxScratchChannels),
	}
}

// ---------- public API (existing) ----------

func NewManager() Manager {
	return Manager{A: nil, Sc: newHeapScratch()}
}

func NewArena() Manager {
	a := arena.NewArena()
	return Manager{A: a, Sc: newArenaScratch(a)}
}

// Scratch returns the reusable scratch bundle.
func (m Manager) Scratch() *Scratch { return m.Sc }

func New[T any](m Manager) *T {
	if m.A != nil {
		return arena.New[T](m.A)
	}
	return new(T)
}

func MakeSlice[T any](m Manager, n int) []T {
	if m.A != nil {
		return arena.MakeSlice[T](m.A, n, n)
	}
	return make([]T, n)
}

func (m Manager) FreeAll() {
	if m.A != nil {
		m.A.Free()
	}
}

func (m Manager) GerArenaPtr() *arena.Arena { return m.A }
func (m Manager) IsZero() bool { return m.Sc == nil }

// ---------- new: frames ----------

// NewFrame returns a child Manager with its own Scratch bundle.
// Arena-backed: slices are carved from the same arena (cheap).
// Heap-backed: Scratch objects are taken from a pool (cheap, no per-pixel allocs).
func (m Manager) NewFrame() Manager {
	if m.A != nil {
		return Manager{A: m.A, Sc: newArenaScratch(m.A)}
	}
	return Manager{A: nil, Sc: heapScratchPool.Get().(*Scratch)}
}

// Close returns a heap-backed Scratch to the pool. No-op for arena-backed.
func (m Manager) Close() {
	if m.A == nil && m.Sc != nil {
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
