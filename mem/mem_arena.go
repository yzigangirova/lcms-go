//go:build goexperiment.arenas

package mem

import "arena"

const MaxScratchChannels = 128 //== MAX_STAGE_CHANNELS in lcms
const MaxScratchChannelsShort = 16  //cmsMAXCHANNELS

// Scratch holds reusable working buffers for hot paths.
// These are preallocated to MaxScratchChannels once when the Manager is created.
type Scratch struct {
	LUT         [2][]float32 // len == MaxScratchChannels
	In16        []uint16     // len == MaxScratchChannels
	Out16       []uint16     // len == MaxScratchChannels
	WInU16        []uint16     // len == MaxScratchChannelsShort
	WOutU16       []uint16     // len == MaxScratchChannelsShort
	WInF32        []float32     // len == MaxScratchChannelsShort
	WOutF32       []float32     // len == MaxScratchChannelsShort
	Tmp1U16  []uint16     // len == MaxScratchChannels
	Tmp2U16  []uint16     // len == MaxScratchChannels
	Tmp1F32  []float32    // len == MaxScratchChannels
	Tmp2F32  []float32    // len == MaxScratchChannels
	  // new: tiny, tone-curve-only buffers, never used elsewhere
    ToneInU16  [1]uint16
    ToneOutU16 [1]uint16
    ToneInF32  [1]float32
    ToneOutF32 [1]float32

}

// Manager carries an optional arena and one reusable Scratch bundle.
// It is safe to pass by value; the fields are pointers.
type Manager struct {
	A  *arena.Arena
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
		Tmp1F32:  make([]float32, MaxScratchChannels), // len == MaxScratchChannels
		Tmp2F32:  make([]float32, MaxScratchChannels), // len == MaxScratchChannels
		WInU16:        make([]uint16, MaxScratchChannelsShort),
		WOutU16:       make([]uint16, MaxScratchChannelsShort),
		WInF32:        make([]float32, MaxScratchChannelsShort),
		WOutF32:       make([]float32, MaxScratchChannelsShort),

	}
	return Manager{A: nil, Sc: s}
}

// NewArena returns an arena-backed Manager with preallocated scratch in the arena.
func NewArena() Manager {
	a := arena.NewArena()
	s := &Scratch{
		LUT: [2][]float32{
			arena.MakeSlice[float32](a, MaxScratchChannels, MaxScratchChannels),
			arena.MakeSlice[float32](a, MaxScratchChannels, MaxScratchChannels),
		},
		In16:        arena.MakeSlice[uint16](a, MaxScratchChannels, MaxScratchChannels),
		Out16:       arena.MakeSlice[uint16](a, MaxScratchChannels, MaxScratchChannels),
		WInU16:        arena.MakeSlice[uint16](a, MaxScratchChannelsShort, MaxScratchChannelsShort),
		WOutU16:       arena.MakeSlice[uint16](a, MaxScratchChannelsShort, MaxScratchChannelsShort),
		WInF32:        arena.MakeSlice[float32](a, MaxScratchChannelsShort, MaxScratchChannelsShort),
		WOutF32:       arena.MakeSlice[float32](a, MaxScratchChannelsShort, MaxScratchChannelsShort),
		Tmp1U16:  arena.MakeSlice[uint16](a, MaxScratchChannels, MaxScratchChannels),
		Tmp2U16:  arena.MakeSlice[uint16](a, MaxScratchChannels, MaxScratchChannels),
		Tmp1F32: arena.MakeSlice[float32](a, MaxScratchChannels, MaxScratchChannels),
		Tmp2F32: arena.MakeSlice[float32](a, MaxScratchChannels, MaxScratchChannels),
	}
	return Manager{A: a, Sc: s}
}

// Scratch returns the reusable scratch buffers (never nil for Managers created via NewManager/NewArena).
func (m Manager) Scratch() *Scratch { return m.Sc }

// Generic helpers — allocate from arena when present, else heap.
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

// FreeAll releases the arena (if any). After this, m.A is cleared.
func (m Manager) FreeAll() {
	if m.A != nil {
		m.A.Free()
		// NOTE: scratch slices were allocated inside the arena; after Free they are invalid.
		// Callers should discard the Manager after FreeAll.
	}
}

// Kept for compatibility with your previous API.
func (m Manager) GerArenaPtr() *arena.Arena { return m.A }

// IsZero reports whether the Manager has no backing state.
// Passing a zero Manager means "please use the transform's manager".
func (m Manager) IsZero() bool { return m.Scratch() == nil || m.Sc == nil }
