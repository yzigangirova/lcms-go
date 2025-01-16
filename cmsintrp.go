package golcms

import (
	"unsafe"
)

// DefaultInterpolatorsFactory defines the default interpolation routine.
func DefaultInterpolatorsFactory(nInputChannels, nOutputChannels, dwFlags uint32) cmsInterpFunction {
	// Implementation of the default interpolation routine goes here.
	// This would return an appropriate cmsInterpFunction based on the arguments.
	return cmsInterpFunction{}
}

// cmsAllocInterpPluginChunk allocates and duplicates the interpolation plug-in memory chunk.
func cmsAllocInterpPluginChunk(ctx, src *cmsContextStruct) {
	var from unsafe.Pointer

	if src != nil {
		from = src.chunks[InterpPlugin]
	} else {
		// Default interpolation chunk
		staticInterpPluginChunk := cmsInterpPluginChunkType{Interpolators: nil}
		from = unsafe.Pointer(&staticInterpPluginChunk)
	}

	ctx.chunks[InterpPlugin] = cmsSubAllocDup(ctx.MemPool, from, uint32(unsafe.Sizeof(cmsInterpPluginChunkType{})))
}

// cmsRegisterInterpPlugin is the main entry for interpolation plug-in registration.
func cmsRegisterInterpPlugin(ContextID cmsContext, Data *cmsPluginBase) bool {
	plugin := (*cmsPluginInterpolation)(unsafe.Pointer(Data))
	ptr := (*cmsInterpPluginChunkType)(cmsContextGetClientChunk(ContextID, InterpPlugin))

	if Data == nil {
		ptr.Interpolators = nil
		return true
	}

	// Set replacement functions
	ptr.Interpolators = plugin.InterpolatorsFactory
	return true
}

// cmsSetInterpolationRoutine sets the interpolation method.
func cmsSetInterpolationRoutine(ContextID cmsContext, p *cmsInterpParams) bool {
	ptr := (*cmsInterpPluginChunkType)(cmsContextGetClientChunk(ContextID, InterpPlugin))

	// Reset the interpolation function
	p.Interpolation.Lerp16 = nil

	// Invoke factory, possibly from the plug-in
	if ptr.Interpolators != nil {
		p.Interpolation = ptr.Interpolators(p.nInputs, p.nOutputs, p.dwFlags)
	}

	// If unsupported by the plug-in, fall back to the default LittleCMS implementation
	if p.Interpolation.Lerp16 == nil {
		p.Interpolation = DefaultInterpolatorsFactory(p.nInputs, p.nOutputs, p.dwFlags)
	}

	// Validate the interpolator (check at least one member of the union)
	if p.Interpolation.Lerp16 == nil {
		return false
	}

	return true
}

// cmsComputeInterpParamsEx precalculates parameters to speed up interpolation.
func cmsComputeInterpParamsEx(
	ContextID cmsContext,
	nSamples *uint32,
	InputChan uint32,
	OutputChan uint32,
	Table unsafe.Pointer,
	dwFlags uint32,
) *cmsInterpParams {
	var i uint32

	// Check for maximum inputs
	if InputChan > MAX_INPUT_DIMENSIONS {
		cmsSignalError(unsafe.Pointer(ContextID), cmsERROR_RANGE, "Too many input channels ")
		return nil
	}

	// Create an empty object
	p := (*cmsInterpParams)(cmsMallocZero(ContextID, uint32(unsafe.Sizeof(cmsInterpParams{}))))
	if p == nil {
		return nil
	}

	// Keep original parameters
	p.dwFlags = dwFlags
	p.nInputs = InputChan
	p.nOutputs = OutputChan
	p.Table = Table
	p.ContextID = ContextID

	// Fill samples per input direction and domain (which is number of nodes minus one)
	for i = 0; i < InputChan; i++ {
		p.nSamples[i] = *(*uint32)(unsafe.Add(unsafe.Pointer(nSamples), uintptr(i)*unsafe.Sizeof(uint32(0))))
		p.Domain[i] = p.nSamples[i] - 1
	}

	// Compute factors to apply to each component to index the grid array
	p.opta[0] = p.nOutputs
	for i = 1; i < InputChan; i++ {
		p.opta[i] = p.opta[i-1] * *(*uint32)(unsafe.Add(unsafe.Pointer(nSamples), uintptr(InputChan-i)*unsafe.Sizeof(uint32(0))))
	}

	// Set the interpolation routine
	if !cmsSetInterpolationRoutine(ContextID, p) {
		cmsSignalError(unsafe.Pointer(ContextID), cmsERROR_UNKNOWN_EXTENSION, "Unsupported interpolation")
		cmsFree(ContextID, unsafe.Pointer(p))
		return nil
	}

	return p
}

// cmsComputeInterpParams is a wrapper assuming all directions have the same number of nodes.
func cmsComputeInterpParams(
	ContextID cmsContext,
	nSamples uint32,
	InputChan uint32,
	OutputChan uint32,
	Table unsafe.Pointer,
	dwFlags uint32,
) *cmsInterpParams {
	var Samples [MAX_INPUT_DIMENSIONS]uint32

	// Fill the auxiliary array
	for i := 0; i < MAX_INPUT_DIMENSIONS; i++ {
		Samples[i] = nSamples
	}

	// Call the extended function
	return cmsComputeInterpParamsEx(ContextID, &Samples[0], InputChan, OutputChan, Table, dwFlags)
}

// cmsFreeInterpParams frees all associated memory.
func cmsFreeInterpParams(p *cmsInterpParams) {
	if p != nil {
		cmsFree(p.ContextID, unsafe.Pointer(p))
	}
}

// LinearInterp performs inline fixed-point interpolation.
func LinearInterp(a, l, h int32) uint16 {
	dif := uint32(h-l)*uint32(a) + 0x8000
	dif = (dif >> 16) + uint32(l)
	return uint16(dif)
}
