package golcms

import (
	"math"
	"unsafe"
)

// This is the default factory
var cmsInterpPluginChunk = cmsInterpPluginChunkType{Interpolators: nil}

// cmsAllocInterpPluginChunk allocates and duplicates the interpolation plug-in memory chunk.
func cmsAllocInterpPluginChunk(ctx, src *CmsContextStruct) {
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
func cmsRegisterInterpPlugin(ContextID CmsContext, Data *cmsPluginBase) bool {
	plugin := (*cmsPluginInterpolation)(unsafe.Pointer(Data))
	ptr := (*cmsInterpPluginChunkType)(CmsContextGetClientChunk(ContextID, InterpPlugin))

	if Data == nil {
		ptr.Interpolators = nil
		return true
	}

	// Set replacement functions
	ptr.Interpolators = plugin.InterpolatorsFactory
	return true
}

// cmsSetInterpolationRoutine sets the interpolation method.
func cmsSetInterpolationRoutine(ContextID CmsContext, p *cmsInterpParams) bool {
	ptr := (*cmsInterpPluginChunkType)(CmsContextGetClientChunk(ContextID, InterpPlugin))

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
	ContextID CmsContext,
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
	ContextID CmsContext,
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

// Linear interpolation (Fixed-point optimized)
func LinLerp1D(Value, Output *uint16, p *cmsInterpParams) {
	var y1, y0 uint16
	var val3, cell0, rest int
	LutTable := (*uint16)(p.Table)

	// if last value or just one point
	if *Value == 0xffff || p.Domain[0] == 0 {
		LutTablePtr := (*uint16)(unsafe.Add(unsafe.Pointer(LutTable), p.Domain[0]*uint32(unsafe.Sizeof(uint16(0)))))
		*Output = *LutTablePtr
	} else {
		val3 = int(p.Domain[0] * uint32(*Value))
		val3 = int(cmsToFixedDomain(val3)) // To fixed 15.16

		cell0 = int(FIXED_TO_INT(cmsS15Fixed16Number(val3)))     // Cell is 16 MSB bits
		rest = int(FIXED_REST_TO_INT(cmsS15Fixed16Number(val3))) // Rest is 16 LSB bits

		LutTablePtr0 := (*uint16)(unsafe.Add(unsafe.Pointer(LutTable), uint32(cell0)*uint32(unsafe.Sizeof(uint16(0)))))
		LutTablePtr1 := (*uint16)(unsafe.Add(unsafe.Pointer(LutTable), uint32(cell0+1)*uint32(unsafe.Sizeof(uint16(0)))))
		y0 = *LutTablePtr0
		y1 = *LutTablePtr1

		*Output = LinearInterp(int32(rest), int32(y0), int32(y1))
	}
}

// To prevent out-of-bounds indexing
func fclamp(v float32) float32 {
	if v < 1.0e-9 || math.IsNaN(float64(v)) {
		return 0.0
	} else if v > 1.0 {
		return 1.0
	}
	return v
}

// Floating-point version of 1D interpolation
func LinLerp1Dfloat(Value *float32, Output *float32, p *cmsInterpParams) {
	var y1, y0, val2, rest float32
	var cell0, cell1 int
	LutTable := (*float32)(p.Table)

	val2 = fclamp(*Value)

	// if last value...
	if val2 == 1.0 || p.Domain[0] == 0 {
		LutTablePtr := (*float32)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(p.Domain[0])*unsafe.Sizeof(float32(0))))
		*Output = *LutTablePtr
	} else {
		val2 *= float32(p.Domain[0])

		cell0 = int(math.Floor(float64(val2)))
		cell1 = int(math.Ceil(float64(val2)))

		// Rest is the fractional part
		rest = val2 - float32(cell0)

		LutTablePtr0 := (*float32)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(cell0)*unsafe.Sizeof(float32(0))))
		LutTablePtr1 := (*float32)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(cell1)*unsafe.Sizeof(float32(0))))
		y0 = *LutTablePtr0
		y1 = *LutTablePtr1

		*Output = y0 + (y1-y0)*rest
	}
}

// Eval gray LUT having only one input channel
func Eval1Input(Input *uint16, Output *uint16, p16 *cmsInterpParams) {
	var fk, k0, k1, rk, K0, K1 cmsS15Fixed16Number
	var v int
	var OutChan uint32
	LutTable := (*uint16)(p16.Table)

	// if last value...
	if *Input == 0xffff || p16.Domain[0] == 0 {
		y0 := uint32(p16.Domain[0]) * uint32(p16.opta[0])

		for OutChan = 0; OutChan < p16.nOutputs; OutChan++ {
			LutTablePtr := (*uint16)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(y0+OutChan)*unsafe.Sizeof(uint16(0))))
			OutputPtr := (*uint16)(unsafe.Add(unsafe.Pointer(Output), uintptr(OutChan)*unsafe.Sizeof(uint16(0))))
			*OutputPtr = *LutTablePtr
		}
	} else {
		v = int(*Input) * int(p16.Domain[0])
		fk = cmsToFixedDomain(v)

		k0 = cmsS15Fixed16Number(FIXED_TO_INT(fk))
		rk = cmsS15Fixed16Number(FIXED_REST_TO_INT(fk))

		if *Input != 0xffff {
			k1 = k0 + 1
		} else {
			k1 = k0
		}

		K0 = cmsS15Fixed16Number(p16.opta[0]) * k0
		K1 = cmsS15Fixed16Number(p16.opta[0]) * k1

		for OutChan = 0; OutChan < p16.nOutputs; OutChan++ {
			LutTablePtr0 := (*uint16)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(K0+cmsS15Fixed16Number(OutChan))*unsafe.Sizeof(uint16(0))))
			LutTablePtr1 := (*uint16)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(K1+cmsS15Fixed16Number(OutChan))*unsafe.Sizeof(uint16(0))))
			OutputPtr := (*uint16)(unsafe.Add(unsafe.Pointer(Output), uintptr(OutChan)*unsafe.Sizeof(uint16(0))))
			*OutputPtr = LinearInterp(int32(rk), int32(*LutTablePtr0), int32(*LutTablePtr1))
		}
	}
}

// Eval1InputFloat evaluates a gray LUT having only one input channel (float version)
func Eval1InputFloat(Value *float32, Output *float32, p *cmsInterpParams) {
	var y1, y0, val2, rest float32
	var cell0, cell1 int
	LutTable := (*float32)(p.Table)

	val2 = fclamp(*Value)

	// If last value or domain is zero
	if val2 == 1.0 || p.Domain[0] == 0 {
		start := uint32(p.Domain[0]) * uint32(p.opta[0])

		for OutChan := uint32(0); OutChan < p.nOutputs; OutChan++ {
			LutTablePtr := (*float32)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(start+OutChan)*unsafe.Sizeof(float32(0))))
			OutputPtr := (*float32)(unsafe.Add(unsafe.Pointer(Output), uintptr(OutChan)*unsafe.Sizeof(float32(0))))
			*OutputPtr = *LutTablePtr
		}
	} else {
		val2 *= float32(p.Domain[0])

		cell0 = int(math.Floor(float64(val2)))
		cell1 = int(math.Ceil(float64(val2)))

		// Rest is the fractional part
		rest = val2 - float32(cell0)

		cell0 *= int(p.opta[0])
		cell1 *= int(p.opta[0])

		for OutChan := uint32(0); OutChan < p.nOutputs; OutChan++ {
			LutTablePtr0 := (*float32)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(cell0+int(OutChan))*unsafe.Sizeof(float32(0))))
			LutTablePtr1 := (*float32)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(cell1+int(OutChan))*unsafe.Sizeof(float32(0))))
			OutputPtr := (*float32)(unsafe.Add(unsafe.Pointer(Output), uintptr(OutChan)*unsafe.Sizeof(float32(0))))

			y0 = *LutTablePtr0
			y1 = *LutTablePtr1

			*OutputPtr = y0 + (y1-y0)*rest
		}
	}
}

func BilinearInterpFloat(Input *float32, Output *float32, p *cmsInterpParams) {
	LutTable := (*float32)(p.Table)
	TotalOut := int(p.nOutputs)

	// Inline functions for LERP and DENS
	LERP := func(a, l, h float32) float32 {
		return l + (h-l)*a
	}

	DENS := func(i, j, outChan int) float32 {
		return *(*float32)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(i+j+outChan)*unsafe.Sizeof(float32(0))))
	}

	px := fclamp(*(*float32)(unsafe.Add(unsafe.Pointer(Input), 0))) * float32(p.Domain[0])
	py := fclamp(*(*float32)(unsafe.Add(unsafe.Pointer(Input), unsafe.Sizeof(float32(0))))) * float32(p.Domain[1])

	x0 := int(math.Floor(float64(px)))
	fx := px - float32(x0)
	y0 := int(math.Floor(float64(py)))
	fy := py - float32(y0)

	X0 := int(p.opta[1]) * x0
	X1 := X0 + func() int {
		if fclamp(*(*float32)(unsafe.Add(unsafe.Pointer(Input), 0))) >= 1.0 {
			return 0
		}
		return int(p.opta[1])
	}()

	Y0 := int(p.opta[0]) * y0
	Y1 := Y0 + func() int {
		if fclamp(*(*float32)(unsafe.Add(unsafe.Pointer(Input), unsafe.Sizeof(float32(0))))) >= 1.0 {
			return 0
		}
		return int(p.opta[0])
	}()

	for outChan := 0; outChan < TotalOut; outChan++ {
		d00 := DENS(X0, Y0, outChan)
		d01 := DENS(X0, Y1, outChan)
		d10 := DENS(X1, Y0, outChan)
		d11 := DENS(X1, Y1, outChan)

		dx0 := LERP(fx, d00, d10)
		dx1 := LERP(fx, d01, d11)

		dxy := LERP(fy, dx0, dx1)

		outputPtr := (*float32)(unsafe.Add(unsafe.Pointer(Output), uintptr(outChan)*unsafe.Sizeof(float32(0))))
		*outputPtr = dxy
	}
}
func BilinearInterp16(Input *uint16, Output *uint16, p *cmsInterpParams) {
	LutTable := (*uint16)(p.Table)
	TotalOut := int(p.nOutputs)

	// Inline functions for LERP and DENS
	LERP := func(a int, l, h int) uint16 {
		return uint16(int32(l) + ROUND_FIXED_TO_INT(cmsS15Fixed16Number((h-l)*a)))
	}

	DENS := func(i, j, outChan int) int {
		return int(*(*uint16)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(i+j+outChan)*unsafe.Sizeof(uint16(0)))))
	}

	fx := cmsToFixedDomain(int(*Input) * int(p.Domain[0]))
	x0 := FIXED_TO_INT(fx)
	rx := FIXED_REST_TO_INT(fx)

	fy := cmsToFixedDomain(int(*(*uint16)(unsafe.Add(unsafe.Pointer(Input), unsafe.Sizeof(uint16(0))))) * int(p.Domain[1]))
	y0 := FIXED_TO_INT(fy)
	ry := FIXED_REST_TO_INT(fy)

	X0 := int(p.opta[1] * uint32(x0))
	X1 := X0 + func() int {
		if *Input == 0xFFFF {
			return 0
		}
		return int(p.opta[1])
	}()

	Y0 := int(p.opta[0] * uint32(y0))
	Y1 := Y0 + func() int {
		if *(*uint16)(unsafe.Add(unsafe.Pointer(Input), unsafe.Sizeof(uint16(0)))) == 0xFFFF {
			return 0
		}
		return int(p.opta[0])
	}()

	for outChan := 0; outChan < TotalOut; outChan++ {
		d00 := DENS(X0, Y0, outChan)
		d01 := DENS(X0, Y1, outChan)
		d10 := DENS(X1, Y0, outChan)
		d11 := DENS(X1, Y1, outChan)

		dx0 := LERP(int(rx), d00, d10)
		dx1 := LERP(int(rx), d01, d11)

		dxy := LERP(int(ry), int(dx0), int(dx1))

		outputPtr := (*uint16)(unsafe.Add(unsafe.Pointer(Output), uintptr(outChan)*unsafe.Sizeof(uint16(0))))
		*outputPtr = uint16(dxy)
	}
}
func TrilinearInterpFloat(Input *float32, Output *float32, p *cmsInterpParams) {
	LutTable := (*float32)(p.Table)
	TotalOut := int(p.nOutputs)

	// Inline functions for LERP and DENS
	LERP := func(a, l, h float32) float32 {
		return l + (h-l)*a
	}

	DENS := func(i, j, k, outChan int) float32 {
		return *(*float32)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(i+j+k+outChan)*unsafe.Sizeof(float32(0))))
	}

	px := fclamp(*(*float32)(unsafe.Add(unsafe.Pointer(Input), 0))) * float32(p.Domain[0])
	py := fclamp(*(*float32)(unsafe.Add(unsafe.Pointer(Input), unsafe.Sizeof(float32(0))))) * float32(p.Domain[1])
	pz := fclamp(*(*float32)(unsafe.Add(unsafe.Pointer(Input), 2*unsafe.Sizeof(float32(0))))) * float32(p.Domain[2])

	x0 := int(math.Floor(float64(px)))
	fx := px - float32(x0)
	y0 := int(math.Floor(float64(py)))
	fy := py - float32(y0)
	z0 := int(math.Floor(float64(pz)))
	fz := pz - float32(z0)

	X0 := int(p.opta[2]) * x0
	X1 := X0 + func() int {
		if fclamp(*(*float32)(unsafe.Add(unsafe.Pointer(Input), 0))) >= 1.0 {
			return 0
		}
		return int(p.opta[2])
	}()

	Y0 := int(p.opta[1]) * y0
	Y1 := Y0 + func() int {
		if fclamp(*(*float32)(unsafe.Add(unsafe.Pointer(Input), unsafe.Sizeof(float32(0))))) >= 1.0 {
			return 0
		}
		return int(p.opta[1])
	}()

	Z0 := int(p.opta[0]) * z0
	Z1 := Z0 + func() int {
		if fclamp(*(*float32)(unsafe.Add(unsafe.Pointer(Input), 2*unsafe.Sizeof(float32(0))))) >= 1.0 {
			return 0
		}
		return int(p.opta[0])
	}()

	for outChan := 0; outChan < TotalOut; outChan++ {
		d000 := DENS(X0, Y0, Z0, outChan)
		d001 := DENS(X0, Y0, Z1, outChan)
		d010 := DENS(X0, Y1, Z0, outChan)
		d011 := DENS(X0, Y1, Z1, outChan)
		d100 := DENS(X1, Y0, Z0, outChan)
		d101 := DENS(X1, Y0, Z1, outChan)
		d110 := DENS(X1, Y1, Z0, outChan)
		d111 := DENS(X1, Y1, Z1, outChan)

		dx00 := LERP(fx, d000, d100)
		dx01 := LERP(fx, d001, d101)
		dx10 := LERP(fx, d010, d110)
		dx11 := LERP(fx, d011, d111)

		dxy0 := LERP(fy, dx00, dx10)
		dxy1 := LERP(fy, dx01, dx11)

		dxyz := LERP(fz, dxy0, dxy1)

		outputPtr := (*float32)(unsafe.Add(unsafe.Pointer(Output), uintptr(outChan)*unsafe.Sizeof(float32(0))))
		*outputPtr = dxyz
	}
}
func TrilinearInterp16(Input *uint16, Output *uint16, p *cmsInterpParams) {
	LutTable := (*uint16)(p.Table)
	TotalOut := int(p.nOutputs)

	// Inline functions for LERP and DENS
	LERP := func(a, l, h int) uint16 {
		return uint16(int32(l) + ROUND_FIXED_TO_INT(cmsS15Fixed16Number((h-l)*a)))
	}

	DENS := func(i, j, k, outChan int) int {
		return int(*(*uint16)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(i+j+k+outChan)*unsafe.Sizeof(uint16(0)))))
	}

	fx := cmsToFixedDomain(int(*(*uint16)(unsafe.Add(unsafe.Pointer(Input), 0))) * int(p.Domain[0]))
	x0 := FIXED_TO_INT(fx)
	rx := FIXED_REST_TO_INT(fx)

	fy := cmsToFixedDomain(int(*(*uint16)(unsafe.Add(unsafe.Pointer(Input), unsafe.Sizeof(uint16(0))))) * int(p.Domain[1]))
	y0 := FIXED_TO_INT(fy)
	ry := FIXED_REST_TO_INT(fy)

	fz := cmsToFixedDomain(int(*(*uint16)(unsafe.Add(unsafe.Pointer(Input), 2*unsafe.Sizeof(uint16(0))))) * int(p.Domain[2]))
	z0 := FIXED_TO_INT(fz)
	rz := FIXED_REST_TO_INT(fz)

	X0 := int(p.opta[2]) * int(x0)
	X1 := int(X0) + func() int {
		if *(*uint16)(unsafe.Add(unsafe.Pointer(Input), 0)) == 0xFFFF {
			return 0
		}
		return int(p.opta[2])
	}()

	Y0 := int(p.opta[1]) * int(y0)
	Y1 := int(Y0) + func() int {
		if *(*uint16)(unsafe.Add(unsafe.Pointer(Input), unsafe.Sizeof(uint16(0)))) == 0xFFFF {
			return 0
		}
		return int(p.opta[1])
	}()

	Z0 := int(p.opta[0]) * int(z0)
	Z1 := int(Z0) + func() int {
		if *(*uint16)(unsafe.Add(unsafe.Pointer(Input), 2*unsafe.Sizeof(uint16(0)))) == 0xFFFF {
			return 0
		}
		return int(p.opta[0])
	}()

	for outChan := 0; outChan < TotalOut; outChan++ {
		d000 := DENS(X0, Y0, Z0, outChan)
		d001 := DENS(X0, Y0, Z1, outChan)
		d010 := DENS(X0, Y1, Z0, outChan)
		d011 := DENS(X0, Y1, Z1, outChan)
		d100 := DENS(X1, Y0, Z0, outChan)
		d101 := DENS(X1, Y0, Z1, outChan)
		d110 := DENS(X1, Y1, Z0, outChan)
		d111 := DENS(X1, Y1, Z1, outChan)

		dx00 := int(LERP(int(rx), d000, d100))
		dx01 := int(LERP(int(rx), d001, d101))
		dx10 := int(LERP(int(rx), d010, d110))
		dx11 := int(LERP(int(rx), d011, d111))

		dxy0 := int(LERP(int(ry), dx00, dx10))
		dxy1 := int(LERP(int(ry), dx01, dx11))

		dxyz := LERP(int(rz), dxy0, dxy1)

		outputPtr := (*uint16)(unsafe.Add(unsafe.Pointer(Output), uintptr(outChan)*unsafe.Sizeof(uint16(0))))
		*outputPtr = uint16(dxyz)
	}
}
func TetrahedralInterpFloat(Input *float32, Output *float32, p *cmsInterpParams) {
	LutTable := (*float32)(p.Table)
	TotalOut := int(p.nOutputs)

	// Inline DENS macro
	DENS := func(i, j, k, outChan int) float32 {
		return *(*float32)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(i+j+k+outChan)*unsafe.Sizeof(float32(0))))
	}

	px := fclamp(*(*float32)(unsafe.Add(unsafe.Pointer(Input), 0))) * float32(p.Domain[0])
	py := fclamp(*(*float32)(unsafe.Add(unsafe.Pointer(Input), unsafe.Sizeof(float32(0))))) * float32(p.Domain[1])
	pz := fclamp(*(*float32)(unsafe.Add(unsafe.Pointer(Input), 2*unsafe.Sizeof(float32(0))))) * float32(p.Domain[2])

	x0 := int(math.Floor(float64(px)))
	rx := px - float32(x0)
	y0 := int(math.Floor(float64(py)))
	ry := py - float32(y0)
	z0 := int(math.Floor(float64(pz)))
	rz := pz - float32(z0)

	X0 := int(p.opta[2]) * x0
	X1 := X0 + func() int {
		if fclamp(*(*float32)(unsafe.Add(unsafe.Pointer(Input), 0))) >= 1.0 {
			return 0
		}
		return int(p.opta[2])
	}()

	Y0 := int(p.opta[1]) * y0
	Y1 := Y0 + func() int {
		if fclamp(*(*float32)(unsafe.Add(unsafe.Pointer(Input), unsafe.Sizeof(float32(0))))) >= 1.0 {
			return 0
		}
		return int(p.opta[1])
	}()

	Z0 := int(p.opta[0]) * z0
	Z1 := Z0 + func() int {
		if fclamp(*(*float32)(unsafe.Add(unsafe.Pointer(Input), 2*unsafe.Sizeof(float32(0))))) >= 1.0 {
			return 0
		}
		return int(p.opta[0])
	}()

	for outChan := 0; outChan < TotalOut; outChan++ {
		c0 := DENS(X0, Y0, Z0, outChan)
		var c1, c2, c3 float32

		if rx >= ry && ry >= rz {
			c1 = DENS(X1, Y0, Z0, outChan) - c0
			c2 = DENS(X1, Y1, Z0, outChan) - DENS(X1, Y0, Z0, outChan)
			c3 = DENS(X1, Y1, Z1, outChan) - DENS(X1, Y1, Z0, outChan)
		} else if rx >= rz && rz >= ry {
			c1 = DENS(X1, Y0, Z0, outChan) - c0
			c2 = DENS(X1, Y1, Z1, outChan) - DENS(X1, Y0, Z1, outChan)
			c3 = DENS(X1, Y0, Z1, outChan) - DENS(X1, Y0, Z0, outChan)
		} else if rz >= rx && rx >= ry {
			c1 = DENS(X1, Y0, Z1, outChan) - DENS(X0, Y0, Z1, outChan)
			c2 = DENS(X1, Y1, Z1, outChan) - DENS(X1, Y0, Z1, outChan)
			c3 = DENS(X0, Y0, Z1, outChan) - c0
		} else if ry >= rx && rx >= rz {
			c1 = DENS(X1, Y1, Z0, outChan) - DENS(X0, Y1, Z0, outChan)
			c2 = DENS(X0, Y1, Z0, outChan) - c0
			c3 = DENS(X1, Y1, Z1, outChan) - DENS(X1, Y1, Z0, outChan)
		} else if ry >= rz && rz >= rx {
			c1 = DENS(X1, Y1, Z1, outChan) - DENS(X0, Y1, Z1, outChan)
			c2 = DENS(X0, Y1, Z0, outChan) - c0
			c3 = DENS(X0, Y1, Z1, outChan) - DENS(X0, Y1, Z0, outChan)
		} else if rz >= ry && ry >= rx {
			c1 = DENS(X1, Y1, Z1, outChan) - DENS(X0, Y1, Z1, outChan)
			c2 = DENS(X0, Y1, Z1, outChan) - DENS(X0, Y0, Z1, outChan)
			c3 = DENS(X0, Y0, Z1, outChan) - c0
		} else {
			c1, c2, c3 = 0, 0, 0
		}

		outputPtr := (*float32)(unsafe.Add(unsafe.Pointer(Output), uintptr(outChan)*unsafe.Sizeof(float32(0))))
		*outputPtr = c0 + c1*rx + c2*ry + c3*rz
	}
}
func TetrahedralInterp16(Input *uint16, Output *uint16, p *cmsInterpParams) {
	LutTable := (*uint16)(p.Table)
	TotalOut := uint32(p.nOutputs)

	fx := cmsToFixedDomain(int(*(*uint16)(unsafe.Add(unsafe.Pointer(Input), 0))) * int(p.Domain[0]))
	fy := cmsToFixedDomain(int(*(*uint16)(unsafe.Add(unsafe.Pointer(Input), unsafe.Sizeof(uint16(0))))) * int(p.Domain[1]))
	fz := cmsToFixedDomain(int(*(*uint16)(unsafe.Add(unsafe.Pointer(Input), 2*unsafe.Sizeof(uint16(0))))) * int(p.Domain[2]))

	x0 := FIXED_TO_INT(fx)
	y0 := FIXED_TO_INT(fy)
	z0 := FIXED_TO_INT(fz)

	rx := FIXED_REST_TO_INT(fx)
	ry := FIXED_REST_TO_INT(fy)
	rz := FIXED_REST_TO_INT(fz)

	X0 := uint32(p.opta[2]) * uint32(x0)
	X1 := func() uint32 {
		if *(*uint16)(unsafe.Add(unsafe.Pointer(Input), 0)) == 0xFFFF {
			return 0
		}
		return uint32(p.opta[2])
	}()

	Y0 := uint32(p.opta[1]) * uint32(y0)
	Y1 := func() uint32 {
		if *(*uint16)(unsafe.Add(unsafe.Pointer(Input), unsafe.Sizeof(uint16(0)))) == 0xFFFF {
			return 0
		}
		return uint32(p.opta[1])
	}()

	Z0 := uint32(p.opta[0]) * uint32(z0)
	Z1 := func() uint32 {
		if *(*uint16)(unsafe.Add(unsafe.Pointer(Input), 2*unsafe.Sizeof(uint16(0)))) == 0xFFFF {
			return 0
		}
		return uint32(p.opta[0])
	}()

	for outChan := uint32(0); outChan < TotalOut; outChan++ {
		c0 := *(*int)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(X0+Y0+Z0+outChan)*unsafe.Sizeof(uint16(0))))
		var c1, c2, c3 int

		if rx >= ry {
			if ry >= rz {
				c1 = *(*int)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(X1+Y0+Z0+outChan)*unsafe.Sizeof(uint16(0)))) - c0
				c2 = *(*int)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(X1+Y1+Z0+outChan)*unsafe.Sizeof(uint16(0)))) -
					*(*int)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(X1+Y0+Z0+outChan)*unsafe.Sizeof(uint16(0))))
				c3 = *(*int)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(X1+Y1+Z1+outChan)*unsafe.Sizeof(uint16(0)))) -
					*(*int)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(X1+Y1+Z0+outChan)*unsafe.Sizeof(uint16(0))))
			} else if rz >= rx {
				c1 = *(*int)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(X1+Y0+Z0+outChan)*unsafe.Sizeof(uint16(0)))) - c0
				c2 = *(*int)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(X1+Y1+Z1+outChan)*unsafe.Sizeof(uint16(0)))) -
					*(*int)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(X1+Y0+Z1+outChan)*unsafe.Sizeof(uint16(0))))
				c3 = *(*int)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(X1+Y0+Z1+outChan)*unsafe.Sizeof(uint16(0)))) -
					*(*int)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(X1+Y0+Z0+outChan)*unsafe.Sizeof(uint16(0))))
			} else {
				c1 = *(*int)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(X1+Y0+Z1+outChan)*unsafe.Sizeof(uint16(0)))) -
					*(*int)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(X0+Y0+Z1+outChan)*unsafe.Sizeof(uint16(0))))
				c2 = *(*int)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(X1+Y1+Z1+outChan)*unsafe.Sizeof(uint16(0)))) -
					*(*int)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(X1+Y0+Z1+outChan)*unsafe.Sizeof(uint16(0))))
				c3 = *(*int)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(X0+Y0+Z1+outChan)*unsafe.Sizeof(uint16(0)))) - c0
			}
		} else {
			if rx >= rz {
				c1 = *(*int)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(X1+Y1+Z0+outChan)*unsafe.Sizeof(uint16(0)))) -
					*(*int)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(X0+Y1+Z0+outChan)*unsafe.Sizeof(uint16(0))))
				c2 = *(*int)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(X0+Y1+Z0+outChan)*unsafe.Sizeof(uint16(0)))) - c0
				c3 = *(*int)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(X1+Y1+Z1+outChan)*unsafe.Sizeof(uint16(0)))) -
					*(*int)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(X1+Y1+Z0+outChan)*unsafe.Sizeof(uint16(0))))
			} else if ry >= rz {
				c1 = *(*int)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(X1+Y1+Z1+outChan)*unsafe.Sizeof(uint16(0)))) -
					*(*int)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(X0+Y1+Z1+outChan)*unsafe.Sizeof(uint16(0))))
				c2 = *(*int)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(X0+Y1+Z0+outChan)*unsafe.Sizeof(uint16(0)))) - c0
				c3 = *(*int)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(X0+Y1+Z1+outChan)*unsafe.Sizeof(uint16(0)))) -
					*(*int)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(X0+Y1+Z0+outChan)*unsafe.Sizeof(uint16(0))))
			} else {
				c1 = *(*int)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(X1+Y1+Z1+outChan)*unsafe.Sizeof(uint16(0)))) -
					*(*int)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(X0+Y1+Z1+outChan)*unsafe.Sizeof(uint16(0))))
				c2 = *(*int)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(X0+Y1+Z1+outChan)*unsafe.Sizeof(uint16(0)))) -
					*(*int)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(X0+Y0+Z1+outChan)*unsafe.Sizeof(uint16(0))))
				c3 = *(*int)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(X0+Y0+Z1+outChan)*unsafe.Sizeof(uint16(0)))) - c0
			}
		}

		Rest := c1*int(rx) + c2*int(ry) + c3*int(rz) + 0x8001
		outputPtr := (*uint16)(unsafe.Add(unsafe.Pointer(Output), uintptr(outChan)*unsafe.Sizeof(uint16(0))))
		*outputPtr = uint16(c0 + ((Rest + (Rest >> 16)) >> 16))
	}
}
func Eval4Inputs(Input *uint16, Output *uint16, p16 *cmsInterpParams) {
	var fk, k0, rk cmsS15Fixed16Number
	var fx, fy, fz, rx, ry, rz cmsS15Fixed16Number
	var x0, y0, z0 int
	var K0, K1, X0, X1, Y0, Y1, Z0, Z1 int
	var c0, c1, c2, c3, Rest cmsS15Fixed16Number
	var Tmp1, Tmp2 [MAX_STAGE_CHANNELS]uint16

	LutTable := (*uint16)(p16.Table)

	// Define the DENS function for accessing the LUT
	DENS := func(LutTable *uint16, i, j, k, outChan int) cmsS15Fixed16Number {
		offset := uintptr(i+j+k+outChan) * unsafe.Sizeof(uint16(0))
		return cmsS15Fixed16Number(*(*uint16)(unsafe.Add(unsafe.Pointer(LutTable), offset)))
	}

	// Perform fixed-point calculations
	fk = cmsToFixedDomain(int(*Input) * int(p16.Domain[0]))
	fx = cmsToFixedDomain(int(*(*uint16)(unsafe.Add(unsafe.Pointer(Input), unsafe.Sizeof(uint16(0))))) * int(p16.Domain[1]))
	fy = cmsToFixedDomain(int(*(*uint16)(unsafe.Add(unsafe.Pointer(Input), 2*unsafe.Sizeof(uint16(0))))) * int(p16.Domain[2]))
	fz = cmsToFixedDomain(int(*(*uint16)(unsafe.Add(unsafe.Pointer(Input), 3*unsafe.Sizeof(uint16(0))))) * int(p16.Domain[3]))

	k0 = cmsS15Fixed16Number(FIXED_TO_INT(fk))
	x0 = int(FIXED_TO_INT(fx))
	y0 = int(FIXED_TO_INT(fy))
	z0 = int(FIXED_TO_INT(fz))

	rk = cmsS15Fixed16Number(FIXED_REST_TO_INT(fk))
	rx = cmsS15Fixed16Number(FIXED_REST_TO_INT(fx))
	ry = cmsS15Fixed16Number(FIXED_REST_TO_INT(fy))
	rz = cmsS15Fixed16Number(FIXED_REST_TO_INT(fz))

	K0 = int(p16.opta[3]) * int(k0)
	K1 = K0
	if *Input != 0xFFFF {
		K1 += int(p16.opta[3])
	}

	X0 = int(p16.opta[2]) * x0
	X1 = X0
	if *(*uint16)(unsafe.Add(unsafe.Pointer(Input), unsafe.Sizeof(uint16(0)))) != 0xFFFF {
		X1 += int(p16.opta[2])
	}

	Y0 = int(p16.opta[1]) * y0
	Y1 = Y0
	if *(*uint16)(unsafe.Add(unsafe.Pointer(Input), 2*unsafe.Sizeof(uint16(0)))) != 0xFFFF {
		Y1 += int(p16.opta[1])
	}

	Z0 = int(p16.opta[0]) * z0
	Z1 = Z0
	if *(*uint16)(unsafe.Add(unsafe.Pointer(Input), 3*unsafe.Sizeof(uint16(0)))) != 0xFFFF {
		Z1 += int(p16.opta[0])
	}

	// Process K0
	LutTable = (*uint16)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(K0)*unsafe.Sizeof(uint16(0))))
	for outChan := uint32(0); outChan < p16.nOutputs; outChan++ {
		c0 = DENS(LutTable, X0, Y0, Z0, int(outChan))

		if rx >= ry && ry >= rz {
			c1 = DENS(LutTable, X1, Y0, Z0, int(outChan)) - c0
			c2 = DENS(LutTable, X1, Y1, Z0, int(outChan)) - DENS(LutTable, X1, Y0, Z0, int(outChan))
			c3 = DENS(LutTable, X1, Y1, Z1, int(outChan)) - DENS(LutTable, X1, Y1, Z0, int(outChan))
		} else if rx >= rz && rz >= ry {
			c1 = DENS(LutTable, X1, Y0, Z0, int(outChan)) - c0
			c2 = DENS(LutTable, X1, Y1, Z1, int(outChan)) - DENS(LutTable, X1, Y0, Z1, int(outChan))
			c3 = DENS(LutTable, X1, Y0, Z1, int(outChan)) - DENS(LutTable, X1, Y0, Z0, int(outChan))
		} else if rz >= rx && rx >= ry {
			c1 = DENS(LutTable, X1, Y0, Z1, int(outChan)) - DENS(LutTable, X0, Y0, Z1, int(outChan))
			c2 = DENS(LutTable, X1, Y1, Z1, int(outChan)) - DENS(LutTable, X1, Y0, Z1, int(outChan))
			c3 = DENS(LutTable, X0, Y0, Z1, int(outChan)) - c0
		} else if ry >= rx && rx >= rz {
			c1 = DENS(LutTable, X1, Y1, Z0, int(outChan)) - DENS(LutTable, X0, Y1, Z0, int(outChan))
			c2 = DENS(LutTable, X0, Y1, Z0, int(outChan)) - c0
			c3 = DENS(LutTable, X1, Y1, Z1, int(outChan)) - DENS(LutTable, X1, Y1, Z0, int(outChan))
		} else if ry >= rz && rz >= rx {
			c1 = DENS(LutTable, X1, Y1, Z1, int(outChan)) - DENS(LutTable, X0, Y1, Z1, int(outChan))
			c2 = DENS(LutTable, X0, Y1, Z0, int(outChan)) - c0
			c3 = DENS(LutTable, X0, Y1, Z1, int(outChan)) - DENS(LutTable, X0, Y1, Z0, int(outChan))
		} else if rz >= ry && ry >= rx {
			c1 = DENS(LutTable, X1, Y1, Z1, int(outChan)) - DENS(LutTable, X0, Y1, Z1, int(outChan))
			c2 = DENS(LutTable, X0, Y1, Z1, int(outChan)) - DENS(LutTable, X0, Y0, Z1, int(outChan))
			c3 = DENS(LutTable, X0, Y0, Z1, int(outChan)) - c0
		} else {
			c1, c2, c3 = 0, 0, 0
		}

		Rest = c1*rx + c2*ry + c3*rz
		Tmp1[outChan] = uint16(int(c0) + int(ROUND_FIXED_TO_INT(cmsToFixedDomain(int(Rest)))))
	}

	// Process K1
	LutTable = (*uint16)(unsafe.Add(unsafe.Pointer(p16.Table), uintptr(K1)*unsafe.Sizeof(uint16(0))))
	for outChan := uint32(0); outChan < p16.nOutputs; outChan++ {
		c0 = DENS(LutTable, X0, Y0, Z0, int(outChan))

		if rx >= ry && ry >= rz {
			c1 = DENS(LutTable, X1, Y0, Z0, int(outChan)) - c0
			c2 = DENS(LutTable, X1, Y1, Z0, int(outChan)) - DENS(LutTable, X1, Y0, Z0, int(outChan))
			c3 = DENS(LutTable, X1, Y1, Z1, int(outChan)) - DENS(LutTable, X1, Y1, Z0, int(outChan))
		} else if rx >= rz && rz >= ry {
			c1 = DENS(LutTable, X1, Y0, Z0, int(outChan)) - c0
			c2 = DENS(LutTable, X1, Y1, Z1, int(outChan)) - DENS(LutTable, X1, Y0, Z1, int(outChan))
			c3 = DENS(LutTable, X1, Y0, Z1, int(outChan)) - DENS(LutTable, X1, Y0, Z0, int(outChan))
		} else if rz >= rx && rx >= ry {
			c1 = DENS(LutTable, X1, Y0, Z1, int(outChan)) - DENS(LutTable, X0, Y0, Z1, int(outChan))
			c2 = DENS(LutTable, X1, Y1, Z1, int(outChan)) - DENS(LutTable, X1, Y0, Z1, int(outChan))
			c3 = DENS(LutTable, X0, Y0, Z1, int(outChan)) - c0
		} else if ry >= rx && rx >= rz {
			c1 = DENS(LutTable, X1, Y1, Z0, int(outChan)) - DENS(LutTable, X0, Y1, Z0, int(outChan))
			c2 = DENS(LutTable, X0, Y1, Z0, int(outChan)) - c0
			c3 = DENS(LutTable, X1, Y1, Z1, int(outChan)) - DENS(LutTable, X1, Y1, Z0, int(outChan))
		} else if ry >= rz && rz >= rx {
			c1 = DENS(LutTable, X1, Y1, Z1, int(outChan)) - DENS(LutTable, X0, Y1, Z1, int(outChan))
			c2 = DENS(LutTable, X0, Y1, Z0, int(outChan)) - c0
			c3 = DENS(LutTable, X0, Y1, Z1, int(outChan)) - DENS(LutTable, X0, Y1, Z0, int(outChan))
		} else if rz >= ry && ry >= rx {
			c1 = DENS(LutTable, X1, Y1, Z1, int(outChan)) - DENS(LutTable, X0, Y1, Z1, int(outChan))
			c2 = DENS(LutTable, X0, Y1, Z1, int(outChan)) - DENS(LutTable, X0, Y0, Z1, int(outChan))
			c3 = DENS(LutTable, X0, Y0, Z1, int(outChan)) - c0
		} else {
			c1, c2, c3 = 0, 0, 0
		}

		Rest = c1*rx + c2*ry + c3*rz
		Tmp2[outChan] = uint16(int(c0) + int(ROUND_FIXED_TO_INT(cmsToFixedDomain(int(Rest)))))
	}

	// Final interpolation
	for i := uint32(0); i < p16.nOutputs; i++ {
		*(*uint16)(unsafe.Add(unsafe.Pointer(Output), uintptr(i)*unsafe.Sizeof(uint16(0)))) = LinearInterp(int32(rk), int32(Tmp1[i]), int32(Tmp2[i]))
	}
}
func Eval4InputsFloat(Input *float32, Output *float32, p *cmsInterpParams) {
	LutTable := (*float32)(p.Table)
	var Tmp1, Tmp2 [MAX_STAGE_CHANNELS]float32

	pk := fclamp(*Input) * float32(p.Domain[0])
	k0 := int(math.Floor(float64(pk)))
	rest := pk - float32(k0)

	K0 := int(p.opta[3]) * k0
	K1 := K0 + func() int {
		if fclamp(*Input) >= 1.0 {
			return 0
		}
		return int(p.opta[3])
	}()

	p1 := *p
	copy(p1.Domain[:3], p.Domain[1:4])

	T := (*float32)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(K0)*unsafe.Sizeof(float32(0))))
	p1.Table = unsafe.Pointer(T)
	TetrahedralInterpFloat((*float32)(unsafe.Add(unsafe.Pointer(Input), unsafe.Sizeof(float32(0)))), &Tmp1[0], &p1)

	T = (*float32)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(K1)*unsafe.Sizeof(float32(0))))
	p1.Table = unsafe.Pointer(T)
	TetrahedralInterpFloat((*float32)(unsafe.Add(unsafe.Pointer(Input), unsafe.Sizeof(float32(0)))), &Tmp2[0], &p1)

	// Linear interpolation between Tmp1 and Tmp2
	for i := uint32(0); i < p.nOutputs; i++ {
		y0 := Tmp1[i]
		y1 := Tmp2[i]
		*(*float32)(unsafe.Add(unsafe.Pointer(Output), uintptr(i)*unsafe.Sizeof(float32(0)))) = y0 + (y1-y0)*rest
	}
}
func Eval5Inputs(Input *uint16, Output *uint16, p16 *cmsInterpParams) {
	LutTable := (*uint16)(p16.Table)
	var fk, k0, rk cmsS15Fixed16Number
	var K0, K1 int
	var Tmp1, Tmp2 [MAX_STAGE_CHANNELS]uint16
	var p1 cmsInterpParams

	fk = cmsToFixedDomain(int(cmsS15Fixed16Number(*Input) * cmsS15Fixed16Number(p16.Domain[0])))
	k0 = cmsS15Fixed16Number(FIXED_TO_INT(fk))
	rk = cmsS15Fixed16Number(FIXED_REST_TO_INT(fk))

	K0 = int(cmsS15Fixed16Number(p16.opta[4]) * k0)
	K1 = K0 + int(p16.opta[4])
	if *Input == 0xFFFF {
		K1 = K0
	}

	p1 = *p16
	copy(p1.Domain[:4], p16.Domain[1:])

	T := (*uint16)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(K0)*unsafe.Sizeof(uint16(0))))
	p1.Table = unsafe.Pointer(T)
	InputPtr0 := (*uint16)(unsafe.Add(unsafe.Pointer(Input), uintptr(1)*unsafe.Sizeof(uint16(0))))
	Eval4Inputs(InputPtr0, &Tmp1[0], &p1)

	T = (*uint16)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(K1)*unsafe.Sizeof(uint16(0))))
	p1.Table = unsafe.Pointer(T)
	InputPtr1 := (*uint16)(unsafe.Add(unsafe.Pointer(Input), uintptr(1)*unsafe.Sizeof(uint16(0))))
	Eval4Inputs(InputPtr1, &Tmp2[0], &p1)

	for i := uint32(0); i < p16.nOutputs; i++ {
		OutputPtr := (*uint16)(unsafe.Add(unsafe.Pointer(Output), uintptr(i)*unsafe.Sizeof(uint16(0))))
		*OutputPtr = LinearInterp(int32(rk), int32(Tmp1[i]), int32(Tmp2[i]))
	}
}

func Eval5InputsFloat(Input *float32, Output *float32, p *cmsInterpParams) {
	LutTable := (*float32)(p.Table)
	var pk, rest float32
	var k0, K0, K1 int
	var Tmp1, Tmp2 [MAX_STAGE_CHANNELS]float32
	var p1 cmsInterpParams

	pk = fclamp(*Input) * float32(p.Domain[0])
	k0 = cmsQuickFloor(float64(pk))
	rest = pk - float32(k0)

	K0 = int(p.opta[4]) * k0
	K1 = K0
	if fclamp(*Input) < 1.0 {
		K1 += int(p.opta[4])
	}

	p1 = *p
	copy(p1.Domain[:4], p.Domain[1:])

	T := (*float32)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(K0)*unsafe.Sizeof(float32(0))))
	p1.Table = unsafe.Pointer(T)
	InputPtr0 := (*float32)(unsafe.Add(unsafe.Pointer(Input), uintptr(1)*unsafe.Sizeof(float32(0))))
	Eval4InputsFloat(InputPtr0, &Tmp1[0], &p1)

	T = (*float32)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(K1)*unsafe.Sizeof(float32(0))))
	p1.Table = unsafe.Pointer(T)
	InputPtr1 := (*float32)(unsafe.Add(unsafe.Pointer(Input), uintptr(1)*unsafe.Sizeof(float32(0))))
	Eval4InputsFloat(InputPtr1, &Tmp2[0], &p1)

	for i := uint32(0); i < p.nOutputs; i++ {
		OutputPtr := (*float32)(unsafe.Add(unsafe.Pointer(Output), uintptr(i)*unsafe.Sizeof(float32(0))))
		*OutputPtr = Tmp1[i] + (Tmp2[i]-Tmp1[i])*rest
	}
}

// Repeat for Eval6Inputs, Eval6InputsFloat, Eval7Inputs, Eval7InputsFloat, and so on...
func Eval6Inputs(Input *uint16, Output *uint16, p16 *cmsInterpParams) {
	LutTable := (*uint16)(p16.Table)
	var fk, k0, rk cmsS15Fixed16Number
	var K0, K1 int
	var Tmp1, Tmp2 [MAX_STAGE_CHANNELS]uint16
	var p1 cmsInterpParams

	fk = cmsToFixedDomain(int(cmsS15Fixed16Number(*Input) * cmsS15Fixed16Number(p16.Domain[0])))
	k0 = cmsS15Fixed16Number(FIXED_TO_INT(fk))
	rk = cmsS15Fixed16Number(FIXED_REST_TO_INT(fk))

	K0 = int(cmsS15Fixed16Number(p16.opta[5]) * k0)
	K1 = K0 + int(p16.opta[5])
	if *Input == 0xFFFF {
		K1 = K0
	}

	p1 = *p16
	copy(p1.Domain[:5], p16.Domain[1:])

	T := (*uint16)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(K0)*unsafe.Sizeof(uint16(0))))
	p1.Table = unsafe.Pointer(T)
	InputPtr0 := (*uint16)(unsafe.Add(unsafe.Pointer(Input), uintptr(1)*unsafe.Sizeof(uint16(0))))
	Eval5Inputs(InputPtr0, &Tmp1[0], &p1)

	T = (*uint16)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(K1)*unsafe.Sizeof(uint16(0))))
	p1.Table = unsafe.Pointer(T)
	InputPtr1 := (*uint16)(unsafe.Add(unsafe.Pointer(Input), uintptr(1)*unsafe.Sizeof(uint16(0))))
	Eval5Inputs(InputPtr1, &Tmp2[0], &p1)

	for i := uint32(0); i < p16.nOutputs; i++ {
		OutputPtr := (*uint16)(unsafe.Add(unsafe.Pointer(Output), uintptr(i)*unsafe.Sizeof(uint16(0))))
		*OutputPtr = LinearInterp(int32(rk), int32(Tmp1[i]), int32(Tmp2[i]))
	}
}

func Eval6InputsFloat(Input *float32, Output *float32, p *cmsInterpParams) {
	LutTable := (*float32)(p.Table)
	var pk, rest float32
	var k0, K0, K1 int
	var Tmp1, Tmp2 [MAX_STAGE_CHANNELS]float32
	var p1 cmsInterpParams

	pk = fclamp(*Input) * float32(p.Domain[0])
	k0 = cmsQuickFloor(float64(pk))
	rest = pk - float32(k0)

	K0 = int(p.opta[5]) * k0
	K1 = K0
	if fclamp(*Input) < 1.0 {
		K1 += int(p.opta[5])
	}

	p1 = *p
	copy(p1.Domain[:5], p.Domain[1:])

	T := (*float32)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(K0)*unsafe.Sizeof(float32(0))))
	p1.Table = unsafe.Pointer(T)
	InputPtr0 := (*float32)(unsafe.Add(unsafe.Pointer(Input), uintptr(1)*unsafe.Sizeof(float32(0))))
	Eval5InputsFloat(InputPtr0, &Tmp1[0], &p1)

	T = (*float32)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(K1)*unsafe.Sizeof(float32(0))))
	p1.Table = unsafe.Pointer(T)
	InputPtr1 := (*float32)(unsafe.Add(unsafe.Pointer(Input), uintptr(1)*unsafe.Sizeof(float32(0))))
	Eval5InputsFloat(InputPtr1, &Tmp2[0], &p1)

	for i := uint32(0); i < p.nOutputs; i++ {
		OutputPtr := (*float32)(unsafe.Add(unsafe.Pointer(Output), uintptr(i)*unsafe.Sizeof(float32(0))))
		*OutputPtr = Tmp1[i] + (Tmp2[i]-Tmp1[i])*rest
	}
}
func Eval7Inputs(Input *uint16, Output *uint16, p16 *cmsInterpParams) {
	LutTable := (*uint16)(p16.Table)
	var fk, k0, rk cmsS15Fixed16Number
	var K0, K1 int
	var Tmp1, Tmp2 [MAX_STAGE_CHANNELS]uint16
	var p1 cmsInterpParams

	fk = cmsToFixedDomain(int(cmsS15Fixed16Number(*Input) * cmsS15Fixed16Number(p16.Domain[0])))
	k0 = cmsS15Fixed16Number(FIXED_TO_INT(fk))
	rk = cmsS15Fixed16Number(FIXED_REST_TO_INT(fk))

	K0 = int(cmsS15Fixed16Number(p16.opta[6]) * k0)
	K1 = K0 + int(p16.opta[6])
	if *Input == 0xFFFF {
		K1 = K0
	}

	p1 = *p16
	copy(p1.Domain[:6], p16.Domain[1:])

	T := (*uint16)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(K0)*unsafe.Sizeof(uint16(0))))
	p1.Table = unsafe.Pointer(T)
	InputPtr0 := (*uint16)(unsafe.Add(unsafe.Pointer(Input), uintptr(1)*unsafe.Sizeof(uint16(0))))
	Eval6Inputs(InputPtr0, &Tmp1[0], &p1)

	T = (*uint16)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(K1)*unsafe.Sizeof(uint16(0))))
	p1.Table = unsafe.Pointer(T)
	InputPtr1 := (*uint16)(unsafe.Add(unsafe.Pointer(Input), uintptr(1)*unsafe.Sizeof(uint16(0))))
	Eval6Inputs(InputPtr1, &Tmp2[0], &p1)

	for i := uint32(0); i < p16.nOutputs; i++ {
		OutputPtr := (*uint16)(unsafe.Add(unsafe.Pointer(Output), uintptr(i)*unsafe.Sizeof(uint16(0))))
		*OutputPtr = LinearInterp(int32(rk), int32(Tmp1[i]), int32(Tmp2[i]))
	}
}

func Eval7InputsFloat(Input *float32, Output *float32, p *cmsInterpParams) {
	LutTable := (*float32)(p.Table)
	var pk, rest float32
	var k0, K0, K1 int
	var Tmp1, Tmp2 [MAX_STAGE_CHANNELS]float32
	var p1 cmsInterpParams

	pk = fclamp(*Input) * float32(p.Domain[0])
	k0 = cmsQuickFloor(float64(pk))
	rest = pk - float32(k0)

	K0 = int(p.opta[6]) * k0
	K1 = K0
	if fclamp(*Input) < 1.0 {
		K1 += int(p.opta[6])
	}

	p1 = *p
	copy(p1.Domain[:6], p.Domain[1:])

	T := (*float32)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(K0)*unsafe.Sizeof(float32(0))))
	p1.Table = unsafe.Pointer(T)
	InputPtr0 := (*float32)(unsafe.Add(unsafe.Pointer(Input), uintptr(1)*unsafe.Sizeof(float32(0))))
	Eval6InputsFloat(InputPtr0, &Tmp1[0], &p1)

	T = (*float32)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(K1)*unsafe.Sizeof(float32(0))))
	p1.Table = unsafe.Pointer(T)
	InputPtr1 := (*float32)(unsafe.Add(unsafe.Pointer(Input), uintptr(1)*unsafe.Sizeof(float32(0))))
	Eval6InputsFloat(InputPtr1, &Tmp2[0], &p1)

	for i := uint32(0); i < p.nOutputs; i++ {
		OutputPtr := (*float32)(unsafe.Add(unsafe.Pointer(Output), uintptr(i)*unsafe.Sizeof(float32(0))))
		*OutputPtr = Tmp1[i] + (Tmp2[i]-Tmp1[i])*rest
	}
}
func Eval8Inputs(Input *uint16, Output *uint16, p16 *cmsInterpParams) {
	LutTable := (*uint16)(p16.Table)
	var fk, k0, rk cmsS15Fixed16Number
	var K0, K1 int
	var Tmp1, Tmp2 [MAX_STAGE_CHANNELS]uint16
	var p1 cmsInterpParams

	fk = cmsToFixedDomain(int(cmsS15Fixed16Number(*Input) * cmsS15Fixed16Number(p16.Domain[0])))
	k0 = cmsS15Fixed16Number(FIXED_TO_INT(fk))
	rk = cmsS15Fixed16Number(FIXED_REST_TO_INT(fk))

	K0 = int(cmsS15Fixed16Number(p16.opta[7]) * k0)
	K1 = K0 + int(p16.opta[7])
	if *Input == 0xFFFF {
		K1 = K0
	}

	p1 = *p16
	copy(p1.Domain[:7], p16.Domain[1:])

	T := (*uint16)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(K0)*unsafe.Sizeof(uint16(0))))
	p1.Table = unsafe.Pointer(T)
	InputPtr0 := (*uint16)(unsafe.Add(unsafe.Pointer(Input), uintptr(1)*unsafe.Sizeof(uint16(0))))
	Eval7Inputs(InputPtr0, &Tmp1[0], &p1)

	T = (*uint16)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(K1)*unsafe.Sizeof(uint16(0))))
	p1.Table = unsafe.Pointer(T)
	InputPtr1 := (*uint16)(unsafe.Add(unsafe.Pointer(Input), uintptr(1)*unsafe.Sizeof(uint16(0))))
	Eval7Inputs(InputPtr1, &Tmp2[0], &p1)

	for i := uint32(0); i < p16.nOutputs; i++ {
		OutputPtr := (*uint16)(unsafe.Add(unsafe.Pointer(Output), uintptr(i)*unsafe.Sizeof(uint16(0))))
		*OutputPtr = LinearInterp(int32(rk), int32(Tmp1[i]), int32(Tmp2[i]))
	}
}

func Eval8InputsFloat(Input *float32, Output *float32, p *cmsInterpParams) {
	LutTable := (*float32)(p.Table)
	var pk, rest float32
	var k0, K0, K1 int
	var Tmp1, Tmp2 [MAX_STAGE_CHANNELS]float32
	var p1 cmsInterpParams

	pk = fclamp(*Input) * float32(p.Domain[0])
	k0 = cmsQuickFloor(float64(pk))
	rest = pk - float32(k0)

	K0 = int(p.opta[7]) * k0
	K1 = K0
	if fclamp(*Input) < 1.0 {
		K1 += int(p.opta[7])
	}

	p1 = *p
	copy(p1.Domain[:7], p.Domain[1:])

	T := (*float32)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(K0)*unsafe.Sizeof(float32(0))))
	p1.Table = unsafe.Pointer(T)
	InputPtr0 := (*float32)(unsafe.Add(unsafe.Pointer(Input), uintptr(1)*unsafe.Sizeof(float32(0))))
	Eval7InputsFloat(InputPtr0, &Tmp1[0], &p1)

	T = (*float32)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(K1)*unsafe.Sizeof(float32(0))))
	p1.Table = unsafe.Pointer(T)
	InputPtr1 := (*float32)(unsafe.Add(unsafe.Pointer(Input), uintptr(1)*unsafe.Sizeof(float32(0))))
	Eval7InputsFloat(InputPtr1, &Tmp2[0], &p1)

	for i := uint32(0); i < p.nOutputs; i++ {
		OutputPtr := (*float32)(unsafe.Add(unsafe.Pointer(Output), uintptr(i)*unsafe.Sizeof(float32(0))))
		*OutputPtr = Tmp1[i] + (Tmp2[i]-Tmp1[i])*rest
	}
}

func Eval9Inputs(Input *uint16, Output *uint16, p16 *cmsInterpParams) {
	LutTable := (*uint16)(p16.Table)
	var fk, k0, rk cmsS15Fixed16Number
	var K0, K1 int
	var Tmp1, Tmp2 [MAX_STAGE_CHANNELS]uint16
	var p1 cmsInterpParams

	fk = cmsToFixedDomain(int(cmsS15Fixed16Number(*Input) * cmsS15Fixed16Number(p16.Domain[0])))
	k0 = cmsS15Fixed16Number(FIXED_TO_INT(fk))
	rk = cmsS15Fixed16Number(FIXED_REST_TO_INT(fk))

	K0 = int(cmsS15Fixed16Number(p16.opta[8]) * k0)
	K1 = K0 + int(p16.opta[8])
	if *Input == 0xFFFF {
		K1 = K0
	}

	p1 = *p16
	copy(p1.Domain[:8], p16.Domain[1:])

	T := (*uint16)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(K0)*unsafe.Sizeof(uint16(0))))
	p1.Table = unsafe.Pointer(T)
	InputPtr0 := (*uint16)(unsafe.Add(unsafe.Pointer(Input), uintptr(1)*unsafe.Sizeof(uint16(0))))
	Eval8Inputs(InputPtr0, &Tmp1[0], &p1)

	T = (*uint16)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(K1)*unsafe.Sizeof(uint16(0))))
	p1.Table = unsafe.Pointer(T)
	InputPtr1 := (*uint16)(unsafe.Add(unsafe.Pointer(Input), uintptr(1)*unsafe.Sizeof(uint16(0))))
	Eval8Inputs(InputPtr1, &Tmp2[0], &p1)

	for i := uint32(0); i < p16.nOutputs; i++ {
		OutputPtr := (*uint16)(unsafe.Add(unsafe.Pointer(Output), uintptr(i)*unsafe.Sizeof(uint16(0))))
		*OutputPtr = LinearInterp(int32(rk), int32(Tmp1[i]), int32(Tmp2[i]))
	}
}

func Eval9InputsFloat(Input *float32, Output *float32, p *cmsInterpParams) {
	LutTable := (*float32)(p.Table)
	var pk, rest float32
	var k0, K0, K1 int
	var Tmp1, Tmp2 [MAX_STAGE_CHANNELS]float32
	var p1 cmsInterpParams

	pk = fclamp(*Input) * float32(p.Domain[0])
	k0 = cmsQuickFloor(float64(pk))
	rest = pk - float32(k0)

	K0 = int(p.opta[8]) * k0
	K1 = K0
	if fclamp(*Input) < 1.0 {
		K1 += int(p.opta[8])
	}

	p1 = *p
	copy(p1.Domain[:8], p.Domain[1:])

	T := (*float32)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(K0)*unsafe.Sizeof(float32(0))))
	p1.Table = unsafe.Pointer(T)
	InputPtr0 := (*float32)(unsafe.Add(unsafe.Pointer(Input), uintptr(1)*unsafe.Sizeof(float32(0))))
	Eval8InputsFloat(InputPtr0, &Tmp1[0], &p1)

	T = (*float32)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(K1)*unsafe.Sizeof(float32(0))))
	p1.Table = unsafe.Pointer(T)
	InputPtr1 := (*float32)(unsafe.Add(unsafe.Pointer(Input), uintptr(1)*unsafe.Sizeof(float32(0))))
	Eval8InputsFloat(InputPtr1, &Tmp2[0], &p1)

	for i := uint32(0); i < p.nOutputs; i++ {
		OutputPtr := (*float32)(unsafe.Add(unsafe.Pointer(Output), uintptr(i)*unsafe.Sizeof(float32(0))))
		*OutputPtr = Tmp1[i] + (Tmp2[i]-Tmp1[i])*rest
	}
}

func Eval10Inputs(Input *uint16, Output *uint16, p16 *cmsInterpParams) {
	LutTable := (*uint16)(p16.Table)
	var fk, k0, rk cmsS15Fixed16Number
	var K0, K1 int
	var Tmp1, Tmp2 [MAX_STAGE_CHANNELS]uint16
	var p1 cmsInterpParams

	fk = cmsToFixedDomain(int(cmsS15Fixed16Number(*Input) * cmsS15Fixed16Number(p16.Domain[0])))
	k0 = cmsS15Fixed16Number(FIXED_TO_INT(fk))
	rk = cmsS15Fixed16Number(FIXED_REST_TO_INT(fk))

	K0 = int(cmsS15Fixed16Number(p16.opta[9]) * k0)
	K1 = K0 + int(p16.opta[9])
	if *Input == 0xFFFF {
		K1 = K0
	}

	p1 = *p16
	copy(p1.Domain[:9], p16.Domain[1:])

	T := (*uint16)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(K0)*unsafe.Sizeof(uint16(0))))
	p1.Table = unsafe.Pointer(T)
	InputPtr0 := (*uint16)(unsafe.Add(unsafe.Pointer(Input), uintptr(1)*unsafe.Sizeof(uint16(0))))
	Eval9Inputs(InputPtr0, &Tmp1[0], &p1)

	T = (*uint16)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(K1)*unsafe.Sizeof(uint16(0))))
	p1.Table = unsafe.Pointer(T)
	InputPtr1 := (*uint16)(unsafe.Add(unsafe.Pointer(Input), uintptr(1)*unsafe.Sizeof(uint16(0))))
	Eval9Inputs(InputPtr1, &Tmp2[0], &p1)

	for i := uint32(0); i < p16.nOutputs; i++ {
		OutputPtr := (*uint16)(unsafe.Add(unsafe.Pointer(Output), uintptr(i)*unsafe.Sizeof(uint16(0))))
		*OutputPtr = LinearInterp(int32(rk), int32(Tmp1[i]), int32(Tmp2[i]))
	}
}

func Eval10InputsFloat(Input *float32, Output *float32, p *cmsInterpParams) {
	LutTable := (*float32)(p.Table)
	var pk, rest float32
	var k0, K0, K1 int
	var Tmp1, Tmp2 [MAX_STAGE_CHANNELS]float32
	var p1 cmsInterpParams

	pk = fclamp(*Input) * float32(p.Domain[0])
	k0 = cmsQuickFloor(float64(pk))
	rest = pk - float32(k0)

	K0 = int(p.opta[9]) * k0
	K1 = K0
	if fclamp(*Input) < 1.0 {
		K1 += int(p.opta[9])
	}

	p1 = *p
	copy(p1.Domain[:9], p.Domain[1:])

	T := (*float32)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(K0)*unsafe.Sizeof(float32(0))))
	p1.Table = unsafe.Pointer(T)
	InputPtr0 := (*float32)(unsafe.Add(unsafe.Pointer(Input), uintptr(1)*unsafe.Sizeof(float32(0))))
	Eval9InputsFloat(InputPtr0, &Tmp1[0], &p1)

	T = (*float32)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(K1)*unsafe.Sizeof(float32(0))))
	p1.Table = unsafe.Pointer(T)
	InputPtr1 := (*float32)(unsafe.Add(unsafe.Pointer(Input), uintptr(1)*unsafe.Sizeof(float32(0))))
	Eval9InputsFloat(InputPtr1, &Tmp2[0], &p1)

	for i := uint32(0); i < p.nOutputs; i++ {
		OutputPtr := (*float32)(unsafe.Add(unsafe.Pointer(Output), uintptr(i)*unsafe.Sizeof(float32(0))))
		*OutputPtr = Tmp1[i] + (Tmp2[i]-Tmp1[i])*rest
	}
}
func Eval11Inputs(Input *uint16, Output *uint16, p16 *cmsInterpParams) {
	LutTable := (*uint16)(p16.Table)
	var fk, k0, rk cmsS15Fixed16Number
	var K0, K1 int
	var Tmp1, Tmp2 [MAX_STAGE_CHANNELS]uint16
	var p1 cmsInterpParams

	fk = cmsToFixedDomain(int(cmsS15Fixed16Number(*Input) * cmsS15Fixed16Number(p16.Domain[0])))
	k0 = cmsS15Fixed16Number(FIXED_TO_INT(fk))
	rk = cmsS15Fixed16Number(FIXED_REST_TO_INT(fk))

	K0 = int(cmsS15Fixed16Number(p16.opta[10]) * k0)
	K1 = K0 + int(p16.opta[10])
	if *Input == 0xFFFF {
		K1 = K0
	}

	p1 = *p16
	copy(p1.Domain[:10], p16.Domain[1:])

	T := (*uint16)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(K0)*unsafe.Sizeof(uint16(0))))
	p1.Table = unsafe.Pointer(T)
	InputPtr0 := (*uint16)(unsafe.Add(unsafe.Pointer(Input), uintptr(1)*unsafe.Sizeof(uint16(0))))
	Eval10Inputs(InputPtr0, &Tmp1[0], &p1)

	T = (*uint16)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(K1)*unsafe.Sizeof(uint16(0))))
	p1.Table = unsafe.Pointer(T)
	InputPtr1 := (*uint16)(unsafe.Add(unsafe.Pointer(Input), uintptr(1)*unsafe.Sizeof(uint16(0))))
	Eval10Inputs(InputPtr1, &Tmp2[0], &p1)

	for i := uint32(0); i < p16.nOutputs; i++ {
		OutputPtr := (*uint16)(unsafe.Add(unsafe.Pointer(Output), uintptr(i)*unsafe.Sizeof(uint16(0))))
		*OutputPtr = LinearInterp(int32(rk), int32(Tmp1[i]), int32(Tmp2[i]))
	}
}

func Eval11InputsFloat(Input *float32, Output *float32, p *cmsInterpParams) {
	LutTable := (*float32)(p.Table)
	var pk, rest float32
	var k0, K0, K1 int
	var Tmp1, Tmp2 [MAX_STAGE_CHANNELS]float32
	var p1 cmsInterpParams

	pk = fclamp(*Input) * float32(p.Domain[0])
	k0 = cmsQuickFloor(float64(pk))
	rest = pk - float32(k0)

	K0 = int(p.opta[10]) * k0
	K1 = K0
	if fclamp(*Input) < 1.0 {
		K1 += int(p.opta[10])
	}

	p1 = *p
	copy(p1.Domain[:10], p.Domain[1:])

	T := (*float32)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(K0)*unsafe.Sizeof(float32(0))))
	p1.Table = unsafe.Pointer(T)
	InputPtr0 := (*float32)(unsafe.Add(unsafe.Pointer(Input), uintptr(1)*unsafe.Sizeof(float32(0))))
	Eval10InputsFloat(InputPtr0, &Tmp1[0], &p1)

	T = (*float32)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(K1)*unsafe.Sizeof(float32(0))))
	p1.Table = unsafe.Pointer(T)
	InputPtr1 := (*float32)(unsafe.Add(unsafe.Pointer(Input), uintptr(1)*unsafe.Sizeof(float32(0))))
	Eval10InputsFloat(InputPtr1, &Tmp2[0], &p1)

	for i := uint32(0); i < p.nOutputs; i++ {
		OutputPtr := (*float32)(unsafe.Add(unsafe.Pointer(Output), uintptr(i)*unsafe.Sizeof(float32(0))))
		*OutputPtr = Tmp1[i] + (Tmp2[i]-Tmp1[i])*rest
	}
}

func Eval12Inputs(Input *uint16, Output *uint16, p16 *cmsInterpParams) {
	LutTable := (*uint16)(p16.Table)
	var fk, k0, rk cmsS15Fixed16Number
	var K0, K1 int
	var Tmp1, Tmp2 [MAX_STAGE_CHANNELS]uint16
	var p1 cmsInterpParams

	fk = cmsToFixedDomain(int(cmsS15Fixed16Number(*Input) * cmsS15Fixed16Number(p16.Domain[0])))
	k0 = cmsS15Fixed16Number(FIXED_TO_INT(fk))
	rk = cmsS15Fixed16Number(FIXED_REST_TO_INT(fk))

	K0 = int(cmsS15Fixed16Number(p16.opta[11]) * k0)
	K1 = K0 + int(p16.opta[11])
	if *Input == 0xFFFF {
		K1 = K0
	}

	p1 = *p16
	copy(p1.Domain[:11], p16.Domain[1:])

	T := (*uint16)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(K0)*unsafe.Sizeof(uint16(0))))
	p1.Table = unsafe.Pointer(T)
	InputPtr0 := (*uint16)(unsafe.Add(unsafe.Pointer(Input), uintptr(1)*unsafe.Sizeof(uint16(0))))
	Eval11Inputs(InputPtr0, &Tmp1[0], &p1)

	T = (*uint16)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(K1)*unsafe.Sizeof(uint16(0))))
	p1.Table = unsafe.Pointer(T)
	InputPtr1 := (*uint16)(unsafe.Add(unsafe.Pointer(Input), uintptr(1)*unsafe.Sizeof(uint16(0))))
	Eval11Inputs(InputPtr1, &Tmp2[0], &p1)

	for i := uint32(0); i < p16.nOutputs; i++ {
		OutputPtr := (*uint16)(unsafe.Add(unsafe.Pointer(Output), uintptr(i)*unsafe.Sizeof(uint16(0))))
		*OutputPtr = LinearInterp(int32(rk), int32(Tmp1[i]), int32(Tmp2[i]))
	}
}

func Eval12InputsFloat(Input *float32, Output *float32, p *cmsInterpParams) {
	LutTable := (*float32)(p.Table)
	var pk, rest float32
	var k0, K0, K1 int
	var Tmp1, Tmp2 [MAX_STAGE_CHANNELS]float32
	var p1 cmsInterpParams

	pk = fclamp(*Input) * float32(p.Domain[0])
	k0 = cmsQuickFloor(float64(pk))
	rest = pk - float32(k0)

	K0 = int(p.opta[11]) * k0
	K1 = K0
	if fclamp(*Input) < 1.0 {
		K1 += int(p.opta[11])
	}

	p1 = *p
	copy(p1.Domain[:11], p.Domain[1:])

	T := (*float32)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(K0)*unsafe.Sizeof(float32(0))))
	p1.Table = unsafe.Pointer(T)
	InputPtr0 := (*float32)(unsafe.Add(unsafe.Pointer(Input), uintptr(1)*unsafe.Sizeof(float32(0))))
	Eval11InputsFloat(InputPtr0, &Tmp1[0], &p1)

	T = (*float32)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(K1)*unsafe.Sizeof(float32(0))))
	p1.Table = unsafe.Pointer(T)
	InputPtr1 := (*float32)(unsafe.Add(unsafe.Pointer(Input), uintptr(1)*unsafe.Sizeof(float32(0))))
	Eval11InputsFloat(InputPtr1, &Tmp2[0], &p1)

	for i := uint32(0); i < p.nOutputs; i++ {
		OutputPtr := (*float32)(unsafe.Add(unsafe.Pointer(Output), uintptr(i)*unsafe.Sizeof(float32(0))))
		*OutputPtr = Tmp1[i] + (Tmp2[i]-Tmp1[i])*rest
	}
}
func Eval13Inputs(Input *uint16, Output *uint16, p16 *cmsInterpParams) {
	LutTable := (*uint16)(p16.Table)
	var fk, k0, rk cmsS15Fixed16Number
	var K0, K1 int
	var Tmp1, Tmp2 [MAX_STAGE_CHANNELS]uint16
	var p1 cmsInterpParams

	fk = cmsToFixedDomain(int(cmsS15Fixed16Number(*Input) * cmsS15Fixed16Number(p16.Domain[0])))
	k0 = cmsS15Fixed16Number(FIXED_TO_INT(fk))
	rk = cmsS15Fixed16Number(FIXED_REST_TO_INT(fk))

	K0 = int(cmsS15Fixed16Number(p16.opta[12]) * k0)
	K1 = K0 + int(p16.opta[12])
	if *Input == 0xFFFF {
		K1 = K0
	}

	p1 = *p16
	copy(p1.Domain[:12], p16.Domain[1:])

	T := (*uint16)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(K0)*unsafe.Sizeof(uint16(0))))
	p1.Table = unsafe.Pointer(T)
	InputPtr0 := (*uint16)(unsafe.Add(unsafe.Pointer(Input), uintptr(1)*unsafe.Sizeof(uint16(0))))
	Eval12Inputs(InputPtr0, &Tmp1[0], &p1)

	T = (*uint16)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(K1)*unsafe.Sizeof(uint16(0))))
	p1.Table = unsafe.Pointer(T)
	InputPtr1 := (*uint16)(unsafe.Add(unsafe.Pointer(Input), uintptr(1)*unsafe.Sizeof(uint16(0))))
	Eval12Inputs(InputPtr1, &Tmp2[0], &p1)

	for i := uint32(0); i < p16.nOutputs; i++ {
		OutputPtr := (*uint16)(unsafe.Add(unsafe.Pointer(Output), uintptr(i)*unsafe.Sizeof(uint16(0))))
		*OutputPtr = LinearInterp(int32(rk), int32(Tmp1[i]), int32(Tmp2[i]))
	}
}

func Eval13InputsFloat(Input *float32, Output *float32, p *cmsInterpParams) {
	LutTable := (*float32)(p.Table)
	var pk, rest float32
	var k0, K0, K1 int
	var Tmp1, Tmp2 [MAX_STAGE_CHANNELS]float32
	var p1 cmsInterpParams

	pk = fclamp(*Input) * float32(p.Domain[0])
	k0 = cmsQuickFloor(float64(pk))
	rest = pk - float32(k0)

	K0 = int(p.opta[12]) * k0
	K1 = K0
	if fclamp(*Input) < 1.0 {
		K1 += int(p.opta[12])
	}

	p1 = *p
	copy(p1.Domain[:12], p.Domain[1:])

	T := (*float32)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(K0)*unsafe.Sizeof(float32(0))))
	p1.Table = unsafe.Pointer(T)
	InputPtr0 := (*float32)(unsafe.Add(unsafe.Pointer(Input), uintptr(1)*unsafe.Sizeof(float32(0))))
	Eval12InputsFloat(InputPtr0, &Tmp1[0], &p1)

	T = (*float32)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(K1)*unsafe.Sizeof(float32(0))))
	p1.Table = unsafe.Pointer(T)
	InputPtr1 := (*float32)(unsafe.Add(unsafe.Pointer(Input), uintptr(1)*unsafe.Sizeof(float32(0))))
	Eval12InputsFloat(InputPtr1, &Tmp2[0], &p1)

	for i := uint32(0); i < p.nOutputs; i++ {
		OutputPtr := (*float32)(unsafe.Add(unsafe.Pointer(Output), uintptr(i)*unsafe.Sizeof(float32(0))))
		*OutputPtr = Tmp1[i] + (Tmp2[i]-Tmp1[i])*rest
	}
}
func Eval14Inputs(Input *uint16, Output *uint16, p16 *cmsInterpParams) {
	LutTable := (*uint16)(p16.Table)
	var fk, k0, rk cmsS15Fixed16Number
	var K0, K1 int
	var Tmp1, Tmp2 [MAX_STAGE_CHANNELS]uint16
	var p1 cmsInterpParams

	fk = cmsToFixedDomain(int(cmsS15Fixed16Number(*Input) * cmsS15Fixed16Number(p16.Domain[0])))
	k0 = cmsS15Fixed16Number(FIXED_TO_INT(fk))
	rk = cmsS15Fixed16Number(FIXED_REST_TO_INT(fk))

	K0 = int(cmsS15Fixed16Number(p16.opta[13]) * k0)
	K1 = K0 + int(p16.opta[13])
	if *Input == 0xFFFF {
		K1 = K0
	}

	p1 = *p16
	copy(p1.Domain[:13], p16.Domain[1:])

	T := (*uint16)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(K0)*unsafe.Sizeof(uint16(0))))
	p1.Table = unsafe.Pointer(T)
	InputPtr0 := (*uint16)(unsafe.Add(unsafe.Pointer(Input), uintptr(1)*unsafe.Sizeof(uint16(0))))
	Eval13Inputs(InputPtr0, &Tmp1[0], &p1)

	T = (*uint16)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(K1)*unsafe.Sizeof(uint16(0))))
	p1.Table = unsafe.Pointer(T)
	InputPtr1 := (*uint16)(unsafe.Add(unsafe.Pointer(Input), uintptr(1)*unsafe.Sizeof(uint16(0))))
	Eval13Inputs(InputPtr1, &Tmp2[0], &p1)

	for i := uint32(0); i < p16.nOutputs; i++ {
		OutputPtr := (*uint16)(unsafe.Add(unsafe.Pointer(Output), uintptr(i)*unsafe.Sizeof(uint16(0))))
		*OutputPtr = LinearInterp(int32(rk), int32(Tmp1[i]), int32(Tmp2[i]))
	}
}

func Eval14InputsFloat(Input *float32, Output *float32, p *cmsInterpParams) {
	LutTable := (*float32)(p.Table)
	var pk, rest float32
	var k0, K0, K1 int
	var Tmp1, Tmp2 [MAX_STAGE_CHANNELS]float32
	var p1 cmsInterpParams

	pk = fclamp(*Input) * float32(p.Domain[0])
	k0 = cmsQuickFloor(float64(pk))
	rest = pk - float32(k0)

	K0 = int(p.opta[13]) * k0
	K1 = K0
	if fclamp(*Input) < 1.0 {
		K1 += int(p.opta[13])
	}

	p1 = *p
	copy(p1.Domain[:13], p.Domain[1:])

	T := (*float32)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(K0)*unsafe.Sizeof(float32(0))))
	p1.Table = unsafe.Pointer(T)
	InputPtr0 := (*float32)(unsafe.Add(unsafe.Pointer(Input), uintptr(1)*unsafe.Sizeof(float32(0))))
	Eval13InputsFloat(InputPtr0, &Tmp1[0], &p1)

	T = (*float32)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(K1)*unsafe.Sizeof(float32(0))))
	p1.Table = unsafe.Pointer(T)
	InputPtr1 := (*float32)(unsafe.Add(unsafe.Pointer(Input), uintptr(1)*unsafe.Sizeof(float32(0))))
	Eval13InputsFloat(InputPtr1, &Tmp2[0], &p1)

	for i := uint32(0); i < p.nOutputs; i++ {
		OutputPtr := (*float32)(unsafe.Add(unsafe.Pointer(Output), uintptr(i)*unsafe.Sizeof(float32(0))))
		*OutputPtr = Tmp1[i] + (Tmp2[i]-Tmp1[i])*rest
	}
}
func Eval15Inputs(Input *uint16, Output *uint16, p16 *cmsInterpParams) {
	LutTable := (*uint16)(p16.Table)
	var fk, k0, rk cmsS15Fixed16Number
	var K0, K1 int
	var Tmp1, Tmp2 [MAX_STAGE_CHANNELS]uint16
	var p1 cmsInterpParams

	fk = cmsToFixedDomain(int(cmsS15Fixed16Number(*Input) * cmsS15Fixed16Number(p16.Domain[0])))
	k0 = cmsS15Fixed16Number(FIXED_TO_INT(fk))
	rk = cmsS15Fixed16Number(FIXED_REST_TO_INT(fk))

	K0 = int(cmsS15Fixed16Number(p16.opta[14]) * k0)
	K1 = K0 + int(p16.opta[14])
	if *Input == 0xFFFF {
		K1 = K0
	}

	p1 = *p16
	copy(p1.Domain[:14], p16.Domain[1:])

	T := (*uint16)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(K0)*unsafe.Sizeof(uint16(0))))
	p1.Table = unsafe.Pointer(T)
	InputPtr0 := (*uint16)(unsafe.Add(unsafe.Pointer(Input), uintptr(1)*unsafe.Sizeof(uint16(0))))
	Eval14Inputs(InputPtr0, &Tmp1[0], &p1)

	T = (*uint16)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(K1)*unsafe.Sizeof(uint16(0))))
	p1.Table = unsafe.Pointer(T)
	InputPtr1 := (*uint16)(unsafe.Add(unsafe.Pointer(Input), uintptr(1)*unsafe.Sizeof(uint16(0))))
	Eval14Inputs(InputPtr1, &Tmp2[0], &p1)

	for i := uint32(0); i < p16.nOutputs; i++ {
		OutputPtr := (*uint16)(unsafe.Add(unsafe.Pointer(Output), uintptr(i)*unsafe.Sizeof(uint16(0))))
		*OutputPtr = LinearInterp(int32(rk), int32(Tmp1[i]), int32(Tmp2[i]))
	}
}

func Eval15InputsFloat(Input *float32, Output *float32, p *cmsInterpParams) {
	LutTable := (*float32)(p.Table)
	var pk, rest float32
	var k0, K0, K1 int
	var Tmp1, Tmp2 [MAX_STAGE_CHANNELS]float32
	var p1 cmsInterpParams

	pk = fclamp(*Input) * float32(p.Domain[0])
	k0 = cmsQuickFloor(float64(pk))
	rest = pk - float32(k0)

	K0 = int(p.opta[14]) * k0
	K1 = K0
	if fclamp(*Input) < 1.0 {
		K1 += int(p.opta[14])
	}

	p1 = *p
	copy(p1.Domain[:14], p.Domain[1:])

	T := (*float32)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(K0)*unsafe.Sizeof(float32(0))))
	p1.Table = unsafe.Pointer(T)
	InputPtr0 := (*float32)(unsafe.Add(unsafe.Pointer(Input), uintptr(1)*unsafe.Sizeof(float32(0))))
	Eval14InputsFloat(InputPtr0, &Tmp1[0], &p1)

	T = (*float32)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(K1)*unsafe.Sizeof(float32(0))))
	p1.Table = unsafe.Pointer(T)
	InputPtr1 := (*float32)(unsafe.Add(unsafe.Pointer(Input), uintptr(1)*unsafe.Sizeof(float32(0))))
	Eval14InputsFloat(InputPtr1, &Tmp2[0], &p1)

	for i := uint32(0); i < p.nOutputs; i++ {
		OutputPtr := (*float32)(unsafe.Add(unsafe.Pointer(Output), uintptr(i)*unsafe.Sizeof(float32(0))))
		*OutputPtr = Tmp1[i] + (Tmp2[i]-Tmp1[i])*rest
	}
}



// The default factory
// DefaultInterpolatorsFactory defines the default interpolation routine.
func DefaultInterpolatorsFactory(nInputChannels, nOutputChannels, dwFlags uint32) cmsInterpFunction {
	var Interpolation cmsInterpFunction
	var IsFloat bool
	var IsTrilinear bool
	if dwFlags&CMS_LERP_FLAGS_FLOAT != 0 {
		IsFloat = true
	}
	if dwFlags&CMS_LERP_FLAGS_TRILINEAR != 0 {
		IsTrilinear = true
	}

	// Safety check
	if nInputChannels >= 4 && nOutputChannels >= MAX_STAGE_CHANNELS {
		return Interpolation
	}
	switch nInputChannels {

	case 1: // Gray LUT / linear

		if nOutputChannels == 1 {

			if IsFloat {
				Interpolation.LerpFloat = LinLerp1Dfloat
			} else {
				Interpolation.Lerp16 = LinLerp1D
			}
		} else {
			if IsFloat {
				Interpolation.LerpFloat = Eval1InputFloat
			} else {
				Interpolation.Lerp16 = Eval1Input
			}
		}
	case 2: // Duotone
		if IsFloat {
			Interpolation.LerpFloat = BilinearInterpFloat
		} else {
			Interpolation.Lerp16 = BilinearInterp16
		}

	case 3: // RGB et al

		if IsTrilinear {

			if IsFloat {
				Interpolation.LerpFloat = TrilinearInterpFloat
			} else {
				Interpolation.Lerp16 = TrilinearInterp16
			}
		} else {

			if IsFloat {
				Interpolation.LerpFloat = TetrahedralInterpFloat
			} else {

				Interpolation.Lerp16 = TetrahedralInterp16
			}
		}

	case 4: // CMYK lut

		if IsFloat {
			Interpolation.LerpFloat = Eval4InputsFloat
		} else {
			Interpolation.Lerp16 = Eval4Inputs
		}

	case 5: // 5 Inks
		if IsFloat {
			Interpolation.LerpFloat = Eval5InputsFloat
		} else {
			Interpolation.Lerp16 = Eval5Inputs
		}

	case 6: // 6 Inks
		if IsFloat {
			Interpolation.LerpFloat = Eval6InputsFloat
		} else {
			Interpolation.Lerp16 = Eval6Inputs
		}

	case 7: // 7 inks
		if IsFloat {
			Interpolation.LerpFloat = Eval7InputsFloat
		} else {
			Interpolation.Lerp16 = Eval7Inputs
		}

	case 8: // 8 inks
		if IsFloat {
			Interpolation.LerpFloat = Eval8InputsFloat
		} else {
			Interpolation.Lerp16 = Eval8Inputs
		}

	case 9:
		if IsFloat {
			Interpolation.LerpFloat = Eval9InputsFloat
		} else {
			Interpolation.Lerp16 = Eval9Inputs
		}

	case 10:
		if IsFloat {
			Interpolation.LerpFloat = Eval10InputsFloat
		} else {
			Interpolation.Lerp16 = Eval10Inputs
		}

	case 11:
		if IsFloat {
			Interpolation.LerpFloat = Eval11InputsFloat
		} else {
			Interpolation.Lerp16 = Eval11Inputs
		}
	case 12:
		if IsFloat {
			Interpolation.LerpFloat = Eval12InputsFloat
		} else {
			Interpolation.Lerp16 = Eval12Inputs
		}

	case 13:
		if IsFloat {
			Interpolation.LerpFloat = Eval13InputsFloat
		} else {
			Interpolation.Lerp16 = Eval13Inputs
		}

	case 14:
		if IsFloat {
			Interpolation.LerpFloat = Eval14InputsFloat
		} else {
			Interpolation.Lerp16 = Eval14Inputs
		}

	case 15:
		if IsFloat {
			Interpolation.LerpFloat = Eval15InputsFloat
		} else {
			Interpolation.Lerp16 = Eval15Inputs
		}

	default:
		Interpolation.Lerp16 = nil
	}

	return Interpolation
}
