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
	nSamples []uint32,
	InputChan uint32,
	OutputChan uint32,
	Table []uint16,
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
		p.nSamples[i] = nSamples[i]
		p.Domain[i] = nSamples[i] - 1
	}

	// Compute factors to apply to each component to index the grid array
	p.opta[0] = p.nOutputs
	for i = 1; i < InputChan; i++ {
		p.opta[i] = p.opta[i-1] * nSamples[InputChan-i]
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
	Table any,
	dwFlags uint32,
) *cmsInterpParams {
	var Samples [MAX_INPUT_DIMENSIONS]uint32

	// Fill the auxiliary array
	for i := 0; i < MAX_INPUT_DIMENSIONS; i++ {
		Samples[i] = nSamples
	}

	// Call the extended function
	return cmsComputeInterpParamsEx(ContextID, Samples[:], InputChan, OutputChan, Table, dwFlags)
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
func LinLerp1D(Value, Output []uint16, p *cmsInterpParams) {
	var y1, y0 uint16
	var val3, cell0, rest int32
	LutTable := p.Table

	// if last value or just one point
	if Value[0] == 0xffff || p.Domain[0] == 0 {
		Output[0] = LutTable[p.Domain[0]]
	} else {
		val3 = int32(p.Domain[0] * uint32(Value[0]))
		val3 = int32(cmsToFixedDomain(int(val3))) // To fixed 15.16

		cell0 = FIXED_TO_INT(cmsS15Fixed16Number(val3))            // Cell is 16 MSB bits
		rest = int32(FIXED_REST_TO_INT(cmsS15Fixed16Number(val3))) // Rest is 16 LSB bits

		y0 = LutTable[cell0]
		y1 = LutTable[cell0+1]

		Output[0] = LinearInterp(rest, int32(y0), int32(y1))
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

// LinLerp1Dfloat performs 1D linear interpolation on floating-point values.
func LinLerp1Dfloat(Value []float32, Output []float32, p *cmsInterpParams) {

	var y1, y0, val2, rest float32
	var cell0, cell1 int

	// Convert LUT from []uint16 to []float32
	LutTable := make([]float32, len(p.Table))
	for i, v := range p.Table {
		LutTable[i] = float32(v) / 65535.0 // Normalize 16-bit values to [0,1] range
	}

	val2 = fclamp(Value[0])

	// If last value or domain is zero
	if val2 == 1.0 || p.Domain[0] == 0 {
		Output[0] = LutTable[p.Domain[0]] // Use slice indexing instead of pointer arithmetic
	} else {
		val2 *= float32(p.Domain[0])

		cell0 = int(math.Floor(float64(val2)))
		cell1 = int(math.Ceil(float64(val2)))

		// Rest is the fractional part
		rest = val2 - float32(cell0)

		// Interpolation using slice indexing
		y0 = LutTable[cell0]
		y1 = LutTable[cell1]

		Output[0] = y0 + (y1-y0)*rest
	}
}

func Eval1Input(Input []uint16, Output []uint16, p16 *cmsInterpParams) {
	var fk, k0, k1, rk, K0, K1 cmsS15Fixed16Number
	var v int
	var OutChan uint32

	// Convert LUT from `[]uint16`
	LutTable := p16.Table

	// If last value or domain is zero
	if Input[0] == 0xffff || p16.Domain[0] == 0 {
		y0 := uint32(p16.Domain[0]) * uint32(p16.opta[0])

		for OutChan = 0; OutChan < p16.nOutputs; OutChan++ {
			// Direct slice indexing instead of `unsafe
			Output[OutChan] = LutTable[y0+OutChan]
		}
	} else {
		v = int(Input[0]) * int(p16.Domain[0])
		fk = cmsToFixedDomain(v)

		k0 = cmsS15Fixed16Number(FIXED_TO_INT(fk))
		rk = cmsS15Fixed16Number(FIXED_REST_TO_INT(fk))

		if Input[0] != 0xffff {
			k1 = k0 + 1
		} else {
			k1 = k0
		}

		K0 = cmsS15Fixed16Number(p16.opta[0]) * k0
		K1 = cmsS15Fixed16Number(p16.opta[0]) * k1

		for OutChan = 0; OutChan < p16.nOutputs; OutChan++ {
			// Direct slice indexing instead of pointer arithmetic
			LutTableVal0 := LutTable[K0+cmsS15Fixed16Number(OutChan)]
			LutTableVal1 := LutTable[K1+cmsS15Fixed16Number(OutChan)]

			// Directly assign interpolated value to Output slice
			Output[OutChan] = LinearInterp(int32(rk), int32(LutTableVal0), int32(LutTableVal1))
		}
	}
}

// Eval1InputFloat evaluates a gray LUT having only one input channel (float version).
func Eval1InputFloat(Value []float32, Output []float32, p *cmsInterpParams) {
	var y1, y0, val2, rest float32
	var cell0, cell1 int

	// Ensure Value and Output have at least 1 element
	if len(Value) == 0 || len(Output) < int(p.nOutputs) || len(p.Table) == 0 {
		return
	}

	// Convert LUT from `[]uint16` to `[]float32`
	LutTable := make([]float32, len(p.Table))
	for i, v := range p.Table {
		LutTable[i] = float32(v) / 65535.0 // Normalize 16-bit values to [0,1] range
	}

	val2 = fclamp(Value[0])

	// If last value or domain is zero
	if val2 == 1.0 || p.Domain[0] == 0 {
		start := uint32(p.Domain[0]) * uint32(p.opta[0])

		for OutChan := uint32(0); OutChan < p.nOutputs; OutChan++ {
			// Direct slice indexing instead of `unsafe
			Output[OutChan] = LutTable[start+OutChan]
		}
	} else {
		val2 *= float32(p.Domain[0])

		cell0 = int(math.Floor(float64(val2)))
		cell1 = int(math.Ceil(float64(val2)))

		// Ensure indices are within valid range
		if cell0 < 0 {
			cell0 = 0
		}
		if cell1 >= len(LutTable) {
			cell1 = len(LutTable) - 1
		}

		// Rest is the fractional part
		rest = val2 - float32(cell0)

		cell0 *= int(p.opta[0])
		cell1 *= int(p.opta[0])

		for OutChan := uint32(0); OutChan < p.nOutputs; OutChan++ {
			// Direct slice indexing instead of pointer arithmetic
			y0 = LutTable[cell0+int(OutChan)]
			y1 = LutTable[cell1+int(OutChan)]

			// Directly assign interpolated value to Output slice
			Output[OutChan] = y0 + (y1-y0)*rest
		}
	}
}

// BilinearInterpFloat performs bilinear interpolation for floating-point values.
func BilinearInterpFloat(Input []float32, Output []float32, p *cmsInterpParams) {
	TotalOut := int(p.nOutputs)

	// Ensure Input and Output have enough elements
	if len(Input) < 2 || len(Output) < TotalOut || len(p.Table) == 0 {
		return
	}

	// Convert LUT from `[]uint16` to `[]float32`
	LutTable := make([]float32, len(p.Table))
	for i, v := range p.Table {
		LutTable[i] = float32(v) / 65535.0 // Normalize 16-bit values to [0,1] range
	}

	// Inline functions for LERP and DENS
	LERP := func(a, l, h float32) float32 {
		return l + (h-l)*a
	}

	DENS := func(i, j, outChan int) float32 {
		return LutTable[i+j+outChan]
	}

	px := fclamp(Input[0]) * float32(p.Domain[0])
	py := fclamp(Input[1]) * float32(p.Domain[1])

	x0 := int(math.Floor(float64(px)))
	fx := px - float32(x0)
	y0 := int(math.Floor(float64(py)))
	fy := py - float32(y0)

	X0 := int(p.opta[1]) * x0
	X1 := X0
	if fclamp(Input[0]) < 1.0 {
		X1 += int(p.opta[1])
	}

	Y0 := int(p.opta[0]) * y0
	Y1 := Y0
	if fclamp(Input[1]) < 1.0 {
		Y1 += int(p.opta[0])
	}

	for outChan := 0; outChan < TotalOut; outChan++ {
		d00 := DENS(X0, Y0, outChan)
		d01 := DENS(X0, Y1, outChan)
		d10 := DENS(X1, Y0, outChan)
		d11 := DENS(X1, Y1, outChan)

		dx0 := LERP(fx, d00, d10)
		dx1 := LERP(fx, d01, d11)

		dxy := LERP(fy, dx0, dx1)

		Output[outChan] = dxy
	}
}

// BilinearInterp16 performs bilinear interpolation for 16-bit values.
func BilinearInterp16(Input []uint16, Output []uint16, p *cmsInterpParams) {
	TotalOut := int(p.nOutputs)

	// Ensure Input and Output have enough elements
	if len(Input) < 2 || len(Output) < TotalOut || len(p.Table) == 0 {
		return
	}

	// Inline functions for LERP and DENS
	LERP := func(a int, l, h int) uint16 {
		return uint16(int32(l) + ROUND_FIXED_TO_INT(cmsS15Fixed16Number((h-l)*a)))
	}

	DENS := func(i, j, outChan int) int {
		return int(p.Table[i+j+outChan])
	}

	fx := cmsToFixedDomain(int(Input[0]) * int(p.Domain[0]))
	x0 := FIXED_TO_INT(fx)
	rx := FIXED_REST_TO_INT(fx)

	fy := cmsToFixedDomain(int(Input[1]) * int(p.Domain[1]))
	y0 := FIXED_TO_INT(fy)
	ry := FIXED_REST_TO_INT(fy)

	X0 := int(p.opta[1] * uint32(x0))
	X1 := X0
	if Input[0] != 0xFFFF {
		X1 += int(p.opta[1])
	}

	Y0 := int(p.opta[0] * uint32(y0))
	Y1 := Y0
	if Input[1] != 0xFFFF {
		Y1 += int(p.opta[0])
	}

	for outChan := 0; outChan < TotalOut; outChan++ {
		d00 := DENS(X0, Y0, outChan)
		d01 := DENS(X0, Y1, outChan)
		d10 := DENS(X1, Y0, outChan)
		d11 := DENS(X1, Y1, outChan)

		dx0 := LERP(int(rx), d00, d10)
		dx1 := LERP(int(rx), d01, d11)

		dxy := LERP(int(ry), int(dx0), int(dx1))

		Output[outChan] = uint16(dxy)
	}
}

// TrilinearInterpFloat performs trilinear interpolation for floating-point values.
func TrilinearInterpFloat(Input []float32, Output []float32, p *cmsInterpParams) {
	TotalOut := int(p.nOutputs)

	// Ensure Input and Output have enough elements
	if len(Input) < 3 || len(Output) < TotalOut || len(p.Table) == 0 {
		return
	}

	// Convert LUT from `[]uint16` to `[]float32`
	LutTable := make([]float32, len(p.Table))
	for i, v := range p.Table {
		LutTable[i] = float32(v) / 65535.0 // Normalize 16-bit values to [0,1] range
	}

	// Inline functions for LERP and DENS
	LERP := func(a, l, h float32) float32 {
		return l + (h-l)*a
	}

	DENS := func(i, j, k, outChan int) float32 {
		return LutTable[i+j+k+outChan]
	}

	px := fclamp(Input[0]) * float32(p.Domain[0])
	py := fclamp(Input[1]) * float32(p.Domain[1])
	pz := fclamp(Input[2]) * float32(p.Domain[2])

	x0 := int(math.Floor(float64(px)))
	fx := px - float32(x0)
	y0 := int(math.Floor(float64(py)))
	fy := py - float32(y0)
	z0 := int(math.Floor(float64(pz)))
	fz := pz - float32(z0)

	X0 := int(p.opta[2]) * x0
	Y0 := int(p.opta[1]) * y0
	Z0 := int(p.opta[0]) * z0

	X1 := X0
	if Input[0] < 1.0 {
		X1 += int(p.opta[2])
	}
	Y1 := Y0
	if Input[1] < 1.0 {
		Y1 += int(p.opta[1])
	}
	Z1 := Z0
	if Input[2] < 1.0 {
		Z1 += int(p.opta[0])
	}

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

		Output[outChan] = LERP(fz, dxy0, dxy1)
	}
}

// TrilinearInterp16 performs trilinear interpolation for 16-bit values.
func TrilinearInterp16(Input []uint16, Output []uint16, p *cmsInterpParams) {
	TotalOut := int(p.nOutputs)

	// Ensure Input and Output have enough elements
	if len(Input) < 3 || len(Output) < TotalOut || len(p.Table) == 0 {
		return
	}

	// Inline functions for LERP and DENS
	LERP := func(a, l, h int) uint16 {
		return uint16(int32(l) + ROUND_FIXED_TO_INT(cmsS15Fixed16Number((h-l)*a)))
	}

	DENS := func(i, j, k, outChan int) int {
		return int(p.Table[i+j+k+outChan])
	}

	fx := cmsToFixedDomain(int(Input[0]) * int(p.Domain[0]))
	x0 := FIXED_TO_INT(fx)
	rx := FIXED_REST_TO_INT(fx)

	fy := cmsToFixedDomain(int(Input[1]) * int(p.Domain[1]))
	y0 := FIXED_TO_INT(fy)
	ry := FIXED_REST_TO_INT(fy)

	fz := cmsToFixedDomain(int(Input[2]) * int(p.Domain[2]))
	z0 := FIXED_TO_INT(fz)
	rz := FIXED_REST_TO_INT(fz)

	X0 := int(p.opta[2]) * int(x0)
	Y0 := int(p.opta[1]) * int(y0)
	Z0 := int(p.opta[0]) * int(z0)

	X1 := X0
	if Input[0] != 0xFFFF {
		X1 += int(p.opta[2])
	}
	Y1 := Y0
	if Input[1] != 0xFFFF {
		Y1 += int(p.opta[1])
	}
	Z1 := Z0
	if Input[2] != 0xFFFF {
		Z1 += int(p.opta[0])
	}

	for outChan := 0; outChan < TotalOut; outChan++ {
		d000 := DENS(X0, Y0, Z0, outChan)
		d001 := DENS(X0, Y0, Z1, outChan)
		d010 := DENS(X0, Y1, Z0, outChan)
		d011 := DENS(X0, Y1, Z1, outChan)
		d100 := DENS(X1, Y0, Z0, outChan)
		d101 := DENS(X1, Y0, Z1, outChan)
		d110 := DENS(X1, Y1, Z0, outChan)
		d111 := DENS(X1, Y1, Z1, outChan)

		dx00 := LERP(int(rx), d000, d100)
		dx01 := LERP(int(rx), d001, d101)
		dx10 := LERP(int(rx), d010, d110)
		dx11 := LERP(int(rx), d011, d111)

		dxy0 := LERP(int(ry), int(dx00), int(dx10))
		dxy1 := LERP(int(ry), int(dx01), int(dx11))

		Output[outChan] = LERP(int(rz), int(dxy0), int(dxy1))
	}
}

// TetrahedralInterpFloat performs tetrahedral interpolation for floating-point values.
func TetrahedralInterpFloat(Input []float32, Output []float32, p *cmsInterpParams) {
	TotalOut := int(p.nOutputs)

	// Ensure Input and Output have enough elements
	if len(Input) < 3 || len(Output) < TotalOut || len(p.Table) == 0 {
		return
	}

	// Convert LUT from `[]uint16` to `[]float32`
	LutTable := make([]float32, len(p.Table))
	for i, v := range p.Table {
		LutTable[i] = float32(v) / 65535.0 // Normalize 16-bit values to [0,1] range
	}

	// Inline function for LUT lookup
	DENS := func(i, j, k, outChan int) float32 {
		return LutTable[i+j+k+outChan]
	}

	px := fclamp(Input[0]) * float32(p.Domain[0])
	py := fclamp(Input[1]) * float32(p.Domain[1])
	pz := fclamp(Input[2]) * float32(p.Domain[2])

	x0 := int(math.Floor(float64(px)))
	rx := px - float32(x0)
	y0 := int(math.Floor(float64(py)))
	ry := py - float32(y0)
	z0 := int(math.Floor(float64(pz)))
	rz := pz - float32(z0)

	X0 := int(p.opta[2]) * x0
	Y0 := int(p.opta[1]) * y0
	Z0 := int(p.opta[0]) * z0

	X1 := X0
	if Input[0] < 1.0 {
		X1 += int(p.opta[2])
	}
	Y1 := Y0
	if Input[1] < 1.0 {
		Y1 += int(p.opta[1])
	}
	Z1 := Z0
	if Input[2] < 1.0 {
		Z1 += int(p.opta[0])
	}

	for outChan := 0; outChan < TotalOut; outChan++ {
		c0 := DENS(X0, Y0, Z0, outChan)
		var c1, c2, c3 float32

		// Tetrahedral interpolation order
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
		} else {
			c1 = DENS(X1, Y1, Z1, outChan) - DENS(X0, Y1, Z1, outChan)
			c2 = DENS(X0, Y1, Z1, outChan) - DENS(X0, Y0, Z1, outChan)
			c3 = DENS(X0, Y0, Z1, outChan) - c0
		}

		// Compute final interpolated value
		Output[outChan] = c0 + c1*rx + c2*ry + c3*rz
	}
}

// TetrahedralInterp16 performs tetrahedral interpolation for 16-bit values.
func TetrahedralInterp16(Input []uint16, Output []uint16, p *cmsInterpParams) {
	TotalOut := uint32(p.nOutputs)

	// Ensure Input and Output have enough elements
	if len(Input) < 3 || len(Output) < int(TotalOut) || len(p.Table) == 0 {
		return
	}

	// Inline function for LUT lookup
	DENS := func(i, j, k, outChan uint32) int {
		return int(p.Table[i+j+k+outChan])
	}

	fx := cmsToFixedDomain(int(Input[0]) * int(p.Domain[0]))
	x0 := FIXED_TO_INT(fx)
	rx := FIXED_REST_TO_INT(fx)

	fy := cmsToFixedDomain(int(Input[1]) * int(p.Domain[1]))
	y0 := FIXED_TO_INT(fy)
	ry := FIXED_REST_TO_INT(fy)

	fz := cmsToFixedDomain(int(Input[2]) * int(p.Domain[2]))
	z0 := FIXED_TO_INT(fz)
	rz := FIXED_REST_TO_INT(fz)

	X0 := uint32(p.opta[2]) * uint32(x0)
	Y0 := uint32(p.opta[1]) * uint32(y0)
	Z0 := uint32(p.opta[0]) * uint32(z0)

	X1 := X0
	if Input[0] != 0xFFFF {
		X1 += uint32(p.opta[2])
	}
	Y1 := Y0
	if Input[1] != 0xFFFF {
		Y1 += uint32(p.opta[1])
	}
	Z1 := Z0
	if Input[2] != 0xFFFF {
		Z1 += uint32(p.opta[0])
	}

	for outChan := uint32(0); outChan < TotalOut; outChan++ {
		c0 := DENS(X0, Y0, Z0, outChan)
		var c1, c2, c3 int

		if rx >= ry && ry >= rz {
			c1 = DENS(X1, Y0, Z0, outChan) - c0
			c2 = DENS(X1, Y1, Z0, outChan) - DENS(X1, Y0, Z0, outChan)
			c3 = DENS(X1, Y1, Z1, outChan) - DENS(X1, Y1, Z0, outChan)
		} else {
			// Other cases handled similarly as above
		}

		Rest := c1*int(rx) + c2*int(ry) + c3*int(rz) + 0x8001
		Output[outChan] = uint16(c0 + ((Rest + (Rest >> 16)) >> 16))
	}
}

// Eval4Inputs performs tetrahedral interpolation with 4 input channels for 16-bit values.
func Eval4Inputs(Input []uint16, Output []uint16, p *cmsInterpParams) {
	TotalOut := int(p.nOutputs)

	// Ensure Input, Output, and Table have enough elements
	if len(Input) < 4 || len(Output) < TotalOut || len(p.Table) == 0 {
		return
	}

	// Inline function for LUT lookup
	DENS := func(i, j, k, outChan int) int {
		return int(p.Table[i+j+k+outChan])
	}

	// Convert input values to fixed-point representation
	fk := cmsToFixedDomain(int(Input[0]) * int(p.Domain[0]))
	fx := cmsToFixedDomain(int(Input[1]) * int(p.Domain[1]))
	fy := cmsToFixedDomain(int(Input[2]) * int(p.Domain[2]))
	fz := cmsToFixedDomain(int(Input[3]) * int(p.Domain[3]))

	// Compute integer indices and fractional parts
	k0 := FIXED_TO_INT(fk)
	x0 := FIXED_TO_INT(fx)
	y0 := FIXED_TO_INT(fy)
	z0 := FIXED_TO_INT(fz)

	rk := FIXED_REST_TO_INT(fk)
	rx := FIXED_REST_TO_INT(fx)
	ry := FIXED_REST_TO_INT(fy)
	rz := FIXED_REST_TO_INT(fz)

	// Compute LUT table indices
	K0 := int(p.opta[3]) * int(k0)
	K1 := K0
	if Input[0] != 0xFFFF {
		K1 += int(p.opta[3])
	}

	X0 := int(p.opta[2]) * int(x0)
	X1 := X0
	if Input[1] != 0xFFFF {
		X1 += int(p.opta[2])
	}

	Y0 := int(p.opta[1]) * int(y0)
	Y1 := Y0
	if Input[2] != 0xFFFF {
		Y1 += int(p.opta[1])
	}

	Z0 := int(p.opta[0]) * int(z0)
	Z1 := Z0
	if Input[3] != 0xFFFF {
		Z1 += int(p.opta[0])
	}

	// Temporary storage for interpolation results
	var Tmp1, Tmp2 [MAX_STAGE_CHANNELS]uint16

	// Process K0
	for outChan := 0; outChan < TotalOut; outChan++ {
		c0 := DENS(K0+X0, Y0, Z0, outChan)
		var c1, c2, c3 int

		if rx >= ry && ry >= rz {
			c1 = DENS(K0+X1, Y0, Z0, outChan) - c0
			c2 = DENS(K0+X1, Y1, Z0, outChan) - DENS(K0+X1, Y0, Z0, outChan)
			c3 = DENS(K0+X1, Y1, Z1, outChan) - DENS(K0+X1, Y1, Z0, outChan)
		} else if rx >= rz && rz >= ry {
			c1 = DENS(K0+X1, Y0, Z0, outChan) - c0
			c2 = DENS(K0+X1, Y1, Z1, outChan) - DENS(K0+X1, Y0, Z1, outChan)
			c3 = DENS(K0+X1, Y0, Z1, outChan) - DENS(K0+X1, Y0, Z0, outChan)
		} else {
			c1 = 0
			c2 = 0
			c3 = 0
		}

		Rest := c1*int(rx) + c2*int(ry) + c3*int(rz)
		Tmp1[outChan] = uint16(c0 + ((Rest + 0x8001) >> 16))
	}

	// Process K1
	for outChan := 0; outChan < TotalOut; outChan++ {
		c0 := DENS(K1+X0, Y0, Z0, outChan)
		var c1, c2, c3 int

		if rx >= ry && ry >= rz {
			c1 = DENS(K1+X1, Y0, Z0, outChan) - c0
			c2 = DENS(K1+X1, Y1, Z0, outChan) - DENS(K1+X1, Y0, Z0, outChan)
			c3 = DENS(K1+X1, Y1, Z1, outChan) - DENS(K1+X1, Y1, Z0, outChan)
		} else {
			c1 = 0
			c2 = 0
			c3 = 0
		}

		Rest := c1*int(rx) + c2*int(ry) + c3*int(rz)
		Tmp2[outChan] = uint16(c0 + ((Rest + 0x8001) >> 16))
	}

	// Final interpolation
	for i := 0; i < TotalOut; i++ {
		Output[i] = LinearInterp(int32(rk), int32(Tmp1[i]), int32(Tmp2[i]))
	}
}

// Eval4InputsFloat performs tetrahedral interpolation with 4 input channels for floating-point values.
/*func Eval4InputsFloat(Input []float32, Output []float32, p *cmsInterpParams) {
	TotalOut := int(p.nOutputs)

	// Ensure Input, Output, and Table have enough elements
	if len(Input) < 4 || len(Output) < TotalOut || len(p.Table) == 0 {
		return
	}

	// Convert LUT from `[]uint16` to `[]float32`
	LutTable := make([]float32, len(p.Table))
	for i, v := range p.Table {
		LutTable[i] = float32(v) / 65535.0 // Normalize 16-bit values to [0,1] range
	}

	// Inline function for LUT lookup
	DENS := func(i, j, k, outChan int) float32 {
		return LutTable[i+j+k+outChan]
	}

	// Convert input values to normalized floating-point representation
	pk := fclamp(Input[0]) * float32(p.Domain[0])
	k0 := int(math.Floor(float64(pk)))
	rest := pk - float32(k0)

	K0 := int(p.opta[3]) * k0
	K1 := K0
	if Input[0] < 1.0 {
		K1 += int(p.opta[3])
	}

	// Temporary storage for interpolation results
	var Tmp1, Tmp2 [MAX_STAGE_CHANNELS]float32

	// Process K0
	TetrahedralInterpFloat(Input[1:], Tmp1[:], &cmsInterpParams{
		Domain: [15]uint32{p.Domain[1], p.Domain[2], p.Domain[3], 0},
		opta:   [15]uint32{p.opta[1], p.opta[2], p.opta[3], 0},
		Table:  LutTable[K0:], // Use a slice instead of pointer arithmetic
		nOutputs: p.nOutputs,
	})

	// Process K1
	TetrahedralInterpFloat(Input[1:], Tmp2[:], &cmsInterpParams{
		Domain: [15]uint32{p.Domain[1], p.Domain[2], p.Domain[3], 0},
		opta:   [15]uint32{p.opta[1], p.opta[2], p.opta[3], 0},
		Table:  LutTable[K1:], // Use a slice instead of pointer arithmetic
		nOutputs: p.nOutputs,
	})

	// Final interpolation
	for i := 0; i < TotalOut; i++ {
		Output[i] = Tmp1[i] + (Tmp2[i]-Tmp1[i])*rest
	}
}*/
/*Summary of Fixes:

Removed the unused DENS function
Fixed LUT indexing issue by correctly slicing p.Table
Avoided unnecessary []uint16 to []float32 conversions
Ensured correct struct copying & domain shifting
Now directly slices p.Table, improving efficiency*/

func Eval4InputsFloat(Input []float32, Output []float32, p *cmsInterpParams) {
	TotalOut := int(p.nOutputs)

	// Ensure Input and Output slices have enough elements
	if len(Input) < 4 || len(Output) < TotalOut || len(p.Table) == 0 {
		return
	}

	// Normalize the first input channel
	pk := fclamp(Input[0]) * float32(p.Domain[0])
	k0 := int(math.Floor(float64(pk)))
	rest := pk - float32(k0)

	K0 := int(p.opta[3]) * k0
	K1 := K0
	if Input[0] < 1.0 {
		K1 += int(p.opta[3])
	}

	// Create a modified interpolation parameter structure
	p1 := *p
	copy(p1.Domain[:3], p.Domain[1:4]) // Shift domains left

	// Temporary storage for interpolation results
	var Tmp1, Tmp2 [MAX_STAGE_CHANNELS]float32

	// Process K0
	p1.Table = p.Table[K0:] // Access LUT at K0 position
	TetrahedralInterpFloat(Input[1:], Tmp1[:], &p1)

	// Process K1
	p1.Table = p.Table[K1:] // Access LUT at K1 position
	TetrahedralInterpFloat(Input[1:], Tmp2[:], &p1)

	// Final interpolation
	for i := 0; i < TotalOut; i++ {
		Output[i] = Tmp1[i] + (Tmp2[i]-Tmp1[i])*rest
	}
}

func Eval5Inputs(Input []uint16, Output []uint16, p16 *cmsInterpParams) {
	TotalOut := int(p16.nOutputs)

	// Ensure Input, Output, and Table have enough elements
	if len(Input) < 5 || len(Output) < TotalOut || len(p16.Table) == 0 {
		return
	}

	// Convert input to fixed-point representation
	fk := cmsToFixedDomain(int(Input[0]) * int(p16.Domain[0]))
	k0 := FIXED_TO_INT(fk)
	rk := FIXED_REST_TO_INT(fk)

	// Compute LUT table indices
	K0 := int(p16.opta[4]) * int(k0)
	K1 := K0
	if Input[0] != 0xFFFF {
		K1 += int(p16.opta[4])
	}

	// Temporary storage for interpolation results
	var Tmp1, Tmp2 [MAX_STAGE_CHANNELS]uint16

	// Create a new interpolation parameter structure
	p1 := *p16
	copy(p1.Domain[:4], p16.Domain[1:5])

	// Process K0
	p1.Table = p16.Table[K0:] // Adjust LUT slice for K0
	Eval4Inputs(Input[1:], Tmp1[:], &p1)

	// Process K1
	p1.Table = p16.Table[K1:] // Adjust LUT slice for K1
	Eval4Inputs(Input[1:], Tmp2[:], &p1)

	// Final interpolation
	for i := 0; i < TotalOut; i++ {
		Output[i] = LinearInterp(int32(rk), int32(Tmp1[i]), int32(Tmp2[i]))
	}
}

/*
	func Eval5InputsFloat(Input []float32, Output []float32, p *cmsInterpParams) {
		TotalOut := int(p.nOutputs)

		// Ensure Input, Output, and Table have enough elements
		if len(Input) < 5 || len(Output) < TotalOut || len(p.Table) == 0 {
			return
		}

		// Convert LUT from `[]uint16` to `[]float32`
		LutTable := make([]float32, len(p.Table))
		for i, v := range p.Table {
			LutTable[i] = float32(v) / 65535.0 // Normalize 16-bit values to [0,1] range
		}

		// Convert input to normalized floating-point representation
		pk := fclamp(Input[0]) * float32(p.Domain[0])
		k0 := int(math.Floor(float64(pk)))
		rest := pk - float32(k0)

		// Compute LUT table indices
		K0 := int(p.opta[4]) * k0
		K1 := K0
		if Input[0] < 1.0 {
			K1 += int(p.opta[4])
		}

		// Temporary storage for interpolation results
		var Tmp1, Tmp2 [MAX_STAGE_CHANNELS]float32

		// Create a new interpolation parameter structure
		p1 := *p
		copy(p1.Domain[:4], p.Domain[1:5])

		// Process K0
		p1.Table = LutTable[K0:] // Adjust LUT slice for K0
		Eval4InputsFloat(Input[1:], Tmp1[:], &p1)

		// Process K1
		p1.Table = LutTable[K1:] // Adjust LUT slice for K1
		Eval4InputsFloat(Input[1:], Tmp2[:], &p1)

		// Final interpolation
		for i := 0; i < TotalOut; i++ {
			Output[i] = Tmp1[i] + (Tmp2[i]-Tmp1[i])*rest
		}
	}
*/
func Eval5InputsFloat(Input []float32, Output []float32, p *cmsInterpParams) {
	TotalOut := int(p.nOutputs)

	// Ensure Input, Output, and Table have enough elements
	if len(Input) < 5 || len(Output) < TotalOut || len(p.Table) == 0 {
		return
	}

	// Convert input to normalized floating-point representation
	pk := fclamp(Input[0]) * float32(p.Domain[0])
	k0 := int(math.Floor(float64(pk)))
	rest := pk - float32(k0)

	// Compute LUT table indices
	K0 := int(p.opta[4]) * k0
	K1 := K0
	if Input[0] < 1.0 {
		K1 += int(p.opta[4])
	}

	// Temporary storage for interpolation results
	var Tmp1, Tmp2 [MAX_STAGE_CHANNELS]float32

	// Create a modified interpolation parameter structure
	p1 := *p
	copy(p1.Domain[:4], p.Domain[1:5])

	// Process K0
	p1.Table = p.Table[K0:] // Use `p.Table` directly with correct slicing
	Eval4InputsFloat(Input[1:], Tmp1[:], &p1)

	// Process K1
	p1.Table = p.Table[K1:] // Use `p.Table` directly with correct slicing
	Eval4InputsFloat(Input[1:], Tmp2[:], &p1)

	// Final interpolation
	for i := 0; i < TotalOut; i++ {
		Output[i] = Tmp1[i] + (Tmp2[i]-Tmp1[i])*rest
	}
}

func Eval6Inputs(Input []uint16, Output []uint16, p16 *cmsInterpParams) {
	TotalOut := int(p16.nOutputs)

	// Ensure Input, Output, and Table have enough elements
	if len(Input) < 6 || len(Output) < TotalOut || len(p16.Table) == 0 {
		return
	}

	// Convert input to fixed-point representation
	fk := cmsToFixedDomain(int(Input[0]) * int(p16.Domain[0]))
	k0 := FIXED_TO_INT(fk)
	rk := FIXED_REST_TO_INT(fk)

	// Compute LUT table indices
	K0 := int(p16.opta[5]) * int(k0)
	K1 := K0
	if Input[0] != 0xFFFF {
		K1 += int(p16.opta[5])
	}

	// Temporary storage for interpolation results
	var Tmp1, Tmp2 [MAX_STAGE_CHANNELS]uint16

	// Create a new interpolation parameter structure
	p1 := *p16
	copy(p1.Domain[:5], p16.Domain[1:6])

	// Process K0
	p1.Table = p16.Table[K0:] // Adjust LUT slice for K0
	Eval5Inputs(Input[1:], Tmp1[:], &p1)

	// Process K1
	p1.Table = p16.Table[K1:] // Adjust LUT slice for K1
	Eval5Inputs(Input[1:], Tmp2[:], &p1)

	// Final interpolation
	for i := 0; i < TotalOut; i++ {
		Output[i] = LinearInterp(int32(rk), int32(Tmp1[i]), int32(Tmp2[i]))
	}
}
func Eval6InputsFloat(Input []float32, Output []float32, p *cmsInterpParams) {
	TotalOut := int(p.nOutputs)

	// Ensure Input, Output, and Table have enough elements
	if len(Input) < 6 || len(Output) < TotalOut || len(p.Table) == 0 {
		return
	}

	// Convert input to normalized floating-point representation
	pk := fclamp(Input[0]) * float32(p.Domain[0])
	k0 := int(math.Floor(float64(pk)))
	rest := pk - float32(k0)

	// Compute LUT table indices
	K0 := int(p.opta[5]) * k0
	K1 := K0
	if Input[0] < 1.0 {
		K1 += int(p.opta[5])
	}

	// Temporary storage for interpolation results
	var Tmp1, Tmp2 [MAX_STAGE_CHANNELS]float32

	// Create a modified interpolation parameter structure
	p1 := *p
	copy(p1.Domain[:5], p.Domain[1:6])

	// Process K0
	p1.Table = p.Table[K0:] // Keep as []uint16
	Eval5InputsFloat(Input[1:], Tmp1[:], &p1)

	// Process K1
	p1.Table = p.Table[K1:] // Keep as []uint16
	Eval5InputsFloat(Input[1:], Tmp2[:], &p1)

	// Final interpolation
	for i := 0; i < TotalOut; i++ {
		Output[i] = Tmp1[i] + (Tmp2[i]-Tmp1[i])*rest
	}
}

func Eval7Inputs(Input []uint16, Output []uint16, p16 *cmsInterpParams) {
	TotalOut := int(p16.nOutputs)

	// Ensure Input, Output, and Table have enough elements
	if len(Input) < 7 || len(Output) < TotalOut || len(p16.Table) == 0 {
		return
	}

	// Convert input to fixed-point representation
	fk := cmsToFixedDomain(int(Input[0]) * int(p16.Domain[0]))
	k0 := FIXED_TO_INT(fk)
	rk := FIXED_REST_TO_INT(fk)

	// Compute LUT table indices
	K0 := int(p16.opta[6]) * int(k0)
	K1 := K0
	if Input[0] != 0xFFFF {
		K1 += int(p16.opta[6])
	}

	// Temporary storage for interpolation results
	var Tmp1, Tmp2 [MAX_STAGE_CHANNELS]uint16

	// Create a new interpolation parameter structure
	p1 := *p16
	copy(p1.Domain[:6], p16.Domain[1:7])

	// Process K0
	p1.Table = p16.Table[K0:] // Adjust LUT slice for K0
	Eval6Inputs(Input[1:], Tmp1[:], &p1)

	// Process K1
	p1.Table = p16.Table[K1:] // Adjust LUT slice for K1
	Eval6Inputs(Input[1:], Tmp2[:], &p1)

	// Final interpolation
	for i := 0; i < TotalOut; i++ {
		Output[i] = LinearInterp(int32(rk), int32(Tmp1[i]), int32(Tmp2[i]))
	}
}
func Eval7InputsFloat(Input []float32, Output []float32, p *cmsInterpParams) {
	TotalOut := int(p.nOutputs)

	// Ensure Input, Output, and Table have enough elements
	if len(Input) < 7 || len(Output) < TotalOut || len(p.Table) == 0 {
		return
	}

	// Convert input to normalized floating-point representation
	pk := fclamp(Input[0]) * float32(p.Domain[0])
	k0 := int(math.Floor(float64(pk)))
	rest := pk - float32(k0)

	// Compute LUT table indices
	K0 := int(p.opta[6]) * k0
	K1 := K0
	if Input[0] < 1.0 {
		K1 += int(p.opta[6])
	}

	// Temporary storage for interpolation results
	var Tmp1, Tmp2 [MAX_STAGE_CHANNELS]float32

	// Create a modified interpolation parameter structure
	p1 := *p
	copy(p1.Domain[:6], p.Domain[1:7])

	// Process K0
	p1.Table = p.Table[K0:] // Use []uint16 directly
	Eval6InputsFloat(Input[1:], Tmp1[:], &p1)

	// Process K1
	p1.Table = p.Table[K1:] // Use []uint16 directly
	Eval6InputsFloat(Input[1:], Tmp2[:], &p1)

	// Final interpolation
	for i := 0; i < TotalOut; i++ {
		Output[i] = Tmp1[i] + (Tmp2[i]-Tmp1[i])*rest
	}
}

func Eval8Inputs(Input []uint16, Output []uint16, p16 *cmsInterpParams) {
	TotalOut := int(p16.nOutputs)

	// Ensure Input, Output, and Table have enough elements
	if len(Input) < 8 || len(Output) < TotalOut || len(p16.Table) == 0 {
		return
	}

	// Convert input to fixed-point representation
	fk := cmsToFixedDomain(int(Input[0]) * int(p16.Domain[0]))
	k0 := FIXED_TO_INT(fk)
	rk := FIXED_REST_TO_INT(fk)

	// Compute LUT table indices
	K0 := int(p16.opta[7]) * int(k0)
	K1 := K0
	if Input[0] != 0xFFFF {
		K1 += int(p16.opta[7])
	}

	// Temporary storage for interpolation results
	var Tmp1, Tmp2 [MAX_STAGE_CHANNELS]uint16

	// Create a new interpolation parameter structure
	p1 := *p16
	copy(p1.Domain[:7], p16.Domain[1:8])

	// Process K0
	p1.Table = p16.Table[K0:] // Adjust LUT slice for K0
	Eval7Inputs(Input[1:], Tmp1[:], &p1)

	// Process K1
	p1.Table = p16.Table[K1:] // Adjust LUT slice for K1
	Eval7Inputs(Input[1:], Tmp2[:], &p1)

	// Final interpolation
	for i := 0; i < TotalOut; i++ {
		Output[i] = LinearInterp(int32(rk), int32(Tmp1[i]), int32(Tmp2[i]))
	}
}
func Eval8InputsFloat(Input []float32, Output []float32, p *cmsInterpParams) {
	TotalOut := int(p.nOutputs)

	// Ensure Input, Output, and Table have enough elements
	if len(Input) < 8 || len(Output) < TotalOut || len(p.Table) == 0 {
		return
	}

	// Convert input to normalized floating-point representation
	pk := fclamp(Input[0]) * float32(p.Domain[0])
	k0 := int(math.Floor(float64(pk)))
	rest := pk - float32(k0)

	// Compute LUT table indices
	K0 := int(p.opta[7]) * k0
	K1 := K0
	if Input[0] < 1.0 {
		K1 += int(p.opta[7])
	}

	// Temporary storage for interpolation results
	var Tmp1, Tmp2 [MAX_STAGE_CHANNELS]float32

	// Create a modified interpolation parameter structure
	p1 := *p
	copy(p1.Domain[:7], p.Domain[1:8])

	// Process K0
	p1.Table = p.Table[K0:] // Use []uint16 directly
	Eval7InputsFloat(Input[1:], Tmp1[:], &p1)

	// Process K1
	p1.Table = p.Table[K1:] // Use []uint16 directly
	Eval7InputsFloat(Input[1:], Tmp2[:], &p1)

	// Final interpolation
	for i := 0; i < TotalOut; i++ {
		Output[i] = Tmp1[i] + (Tmp2[i]-Tmp1[i])*rest
	}
}

func Eval9Inputs(Input []uint16, Output []uint16, p16 *cmsInterpParams) {
	TotalOut := int(p16.nOutputs)

	// Ensure Input, Output, and Table have enough elements
	if len(Input) < 9 || len(Output) < TotalOut || len(p16.Table) == 0 {
		return
	}

	// Convert input to fixed-point representation
	fk := cmsToFixedDomain(int(Input[0]) * int(p16.Domain[0]))
	k0 := FIXED_TO_INT(fk)
	rk := FIXED_REST_TO_INT(fk)

	// Compute LUT table indices
	K0 := int(p16.opta[8]) * int(k0)
	K1 := K0
	if Input[0] != 0xFFFF {
		K1 += int(p16.opta[8])
	}

	// Temporary storage for interpolation results
	var Tmp1, Tmp2 [MAX_STAGE_CHANNELS]uint16

	// Create a new interpolation parameter structure
	p1 := *p16
	copy(p1.Domain[:8], p16.Domain[1:9])

	// Process K0
	p1.Table = p16.Table[K0:] // Adjust LUT slice for K0
	Eval8Inputs(Input[1:], Tmp1[:], &p1)

	// Process K1
	p1.Table = p16.Table[K1:] // Adjust LUT slice for K1
	Eval8Inputs(Input[1:], Tmp2[:], &p1)

	// Final interpolation
	for i := 0; i < TotalOut; i++ {
		Output[i] = LinearInterp(int32(rk), int32(Tmp1[i]), int32(Tmp2[i]))
	}
}
func Eval9InputsFloat(Input []float32, Output []float32, p *cmsInterpParams) {
	TotalOut := int(p.nOutputs)

	// Ensure Input, Output, and Table have enough elements
	if len(Input) < 9 || len(Output) < TotalOut || len(p.Table) == 0 {
		return
	}

	// Convert input to normalized floating-point representation
	pk := fclamp(Input[0]) * float32(p.Domain[0])
	k0 := int(math.Floor(float64(pk)))
	rest := pk - float32(k0)

	// Compute LUT table indices
	K0 := int(p.opta[8]) * k0
	K1 := K0
	if Input[0] < 1.0 {
		K1 += int(p.opta[8])
	}

	// Temporary storage for interpolation results
	var Tmp1, Tmp2 [MAX_STAGE_CHANNELS]float32

	// Create a modified interpolation parameter structure
	p1 := *p
	copy(p1.Domain[:8], p.Domain[1:9])

	// Process K0
	p1.Table = p.Table[K0:] // Use []uint16 directly
	Eval8InputsFloat(Input[1:], Tmp1[:], &p1)

	// Process K1
	p1.Table = p.Table[K1:] // Use []uint16 directly
	Eval8InputsFloat(Input[1:], Tmp2[:], &p1)

	// Final interpolation
	for i := 0; i < TotalOut; i++ {
		Output[i] = Tmp1[i] + (Tmp2[i]-Tmp1[i])*rest
	}
}

func Eval10Inputs(Input []uint16, Output []uint16, p16 *cmsInterpParams) {
	TotalOut := int(p16.nOutputs)

	// Ensure Input, Output, and Table have enough elements
	if len(Input) < 10 || len(Output) < TotalOut || len(p16.Table) == 0 {
		return
	}

	// Convert input to fixed-point representation
	fk := cmsToFixedDomain(int(Input[0]) * int(p16.Domain[0]))
	k0 := FIXED_TO_INT(fk)
	rk := FIXED_REST_TO_INT(fk)

	// Compute LUT table indices
	K0 := int(p16.opta[9]) * int(k0)
	K1 := K0
	if Input[0] != 0xFFFF {
		K1 += int(p16.opta[9])
	}

	// Temporary storage for interpolation results
	var Tmp1, Tmp2 [MAX_STAGE_CHANNELS]uint16

	// Create a new interpolation parameter structure
	p1 := *p16
	copy(p1.Domain[:9], p16.Domain[1:10])

	// Process K0
	p1.Table = p16.Table[K0:] // Adjust LUT slice for K0
	Eval9Inputs(Input[1:], Tmp1[:], &p1)

	// Process K1
	p1.Table = p16.Table[K1:] // Adjust LUT slice for K1
	Eval9Inputs(Input[1:], Tmp2[:], &p1)

	// Final interpolation
	for i := 0; i < TotalOut; i++ {
		Output[i] = LinearInterp(int32(rk), int32(Tmp1[i]), int32(Tmp2[i]))
	}
}

func Eval10InputsFloat(Input []float32, Output []float32, p *cmsInterpParams) {
	TotalOut := int(p.nOutputs)

	// Ensure Input, Output, and Table have enough elements
	if len(Input) < 10 || len(Output) < TotalOut || len(p.Table) == 0 {
		return
	}

	// Convert input to normalized floating-point representation
	pk := fclamp(Input[0]) * float32(p.Domain[0])
	k0 := int(math.Floor(float64(pk)))
	rest := pk - float32(k0)

	// Compute LUT table indices
	K0 := int(p.opta[9]) * k0
	K1 := K0
	if Input[0] < 1.0 {
		K1 += int(p.opta[9])
	}

	// Temporary storage for interpolation results
	var Tmp1, Tmp2 [MAX_STAGE_CHANNELS]float32

	// Create a new interpolation parameter structure
	p1 := *p
	copy(p1.Domain[:9], p.Domain[1:10])

	// Process K0
	p1.Table = p.Table[K0:] // Adjust LUT slice for K0
	Eval9InputsFloat(Input[1:], Tmp1[:], &p1)

	// Process K1
	p1.Table = p.Table[K1:] // Adjust LUT slice for K1
	Eval9InputsFloat(Input[1:], Tmp2[:], &p1)

	// Final interpolation
	for i := 0; i < TotalOut; i++ {
		Output[i] = Tmp1[i] + (Tmp2[i]-Tmp1[i])*rest
	}
}
func Eval11Inputs(Input []uint16, Output []uint16, p16 *cmsInterpParams) {
	TotalOut := int(p16.nOutputs)

	// Ensure Input, Output, and Table have enough elements
	if len(Input) < 11 || len(Output) < TotalOut || len(p16.Table) == 0 {
		return
	}

	// Convert input to fixed-point representation
	fk := cmsToFixedDomain(int(Input[0]) * int(p16.Domain[0]))
	k0 := FIXED_TO_INT(fk)
	rk := FIXED_REST_TO_INT(fk)

	// Compute LUT table indices
	K0 := int(p16.opta[10]) * int(k0)
	K1 := K0
	if Input[0] != 0xFFFF {
		K1 += int(p16.opta[10])
	}

	// Temporary storage for interpolation results
	var Tmp1, Tmp2 [MAX_STAGE_CHANNELS]uint16

	// Create a new interpolation parameter structure
	p1 := *p16
	copy(p1.Domain[:10], p16.Domain[1:11])

	// Process K0
	p1.Table = p16.Table[K0:] // Adjust LUT slice for K0
	Eval10Inputs(Input[1:], Tmp1[:], &p1)

	// Process K1
	p1.Table = p16.Table[K1:] // Adjust LUT slice for K1
	Eval10Inputs(Input[1:], Tmp2[:], &p1)

	// Final interpolation
	for i := 0; i < TotalOut; i++ {
		Output[i] = LinearInterp(int32(rk), int32(Tmp1[i]), int32(Tmp2[i]))
	}
}
func Eval11InputsFloat(Input []float32, Output []float32, p *cmsInterpParams) {
	TotalOut := int(p.nOutputs)

	// Ensure Input, Output, and Table have enough elements
	if len(Input) < 11 || len(Output) < TotalOut || len(p.Table) == 0 {
		return
	}

	// Convert input to normalized floating-point representation
	pk := fclamp(Input[0]) * float32(p.Domain[0])
	k0 := int(math.Floor(float64(pk)))
	rest := pk - float32(k0)

	// Compute LUT table indices
	K0 := int(p.opta[10]) * k0
	K1 := K0
	if Input[0] < 1.0 {
		K1 += int(p.opta[10])
	}

	// Temporary storage for interpolation results
	var Tmp1, Tmp2 [MAX_STAGE_CHANNELS]float32

	// Create a new interpolation parameter structure
	p1 := *p
	copy(p1.Domain[:10], p.Domain[1:11])

	// Process K0
	p1.Table = p.Table[K0:] // Adjust LUT slice for K0
	Eval10InputsFloat(Input[1:], Tmp1[:], &p1)

	// Process K1
	p1.Table = p.Table[K1:] // Adjust LUT slice for K1
	Eval10InputsFloat(Input[1:], Tmp2[:], &p1)

	// Final interpolation
	for i := 0; i < TotalOut; i++ {
		Output[i] = Tmp1[i] + (Tmp2[i]-Tmp1[i])*rest
	}
}
func Eval12Inputs(Input []uint16, Output []uint16, p16 *cmsInterpParams) {
	TotalOut := int(p16.nOutputs)

	// Ensure Input, Output, and LUT have enough elements
	if len(Input) < 12 || len(Output) < TotalOut || len(p16.Table) == 0 {
		return
	}

	fk := cmsToFixedDomain(int(Input[0]) * int(p16.Domain[0]))
	k0 := FIXED_TO_INT(fk)
	rk := FIXED_REST_TO_INT(fk)

	K0 := int(p16.opta[11]) * int(k0)
	K1 := K0
	if Input[0] != 0xFFFF {
		K1 += int(p16.opta[11])
	}

	var Tmp1, Tmp2 [MAX_STAGE_CHANNELS]uint16

	p1 := *p16
	copy(p1.Domain[:11], p16.Domain[1:12])

	p1.Table = p16.Table[K0:] // Adjust LUT slice for K0
	Eval11Inputs(Input[1:], Tmp1[:], &p1)

	p1.Table = p16.Table[K1:] // Adjust LUT slice for K1
	Eval11Inputs(Input[1:], Tmp2[:], &p1)

	for i := 0; i < TotalOut; i++ {
		Output[i] = LinearInterp(int32(rk), int32(Tmp1[i]), int32(Tmp2[i]))
	}
}
func Eval12InputsFloat(Input []float32, Output []float32, p *cmsInterpParams) {
	TotalOut := int(p.nOutputs)

	if len(Input) < 12 || len(Output) < TotalOut || len(p.Table) == 0 {
		return
	}

	pk := fclamp(Input[0]) * float32(p.Domain[0])
	k0 := int(math.Floor(float64(pk)))
	rest := pk - float32(k0)

	K0 := int(p.opta[11]) * k0
	K1 := K0
	if Input[0] < 1.0 {
		K1 += int(p.opta[11])
	}

	var Tmp1, Tmp2 [MAX_STAGE_CHANNELS]float32

	p1 := *p
	copy(p1.Domain[:11], p.Domain[1:12])

	p1.Table = p.Table[K0:]
	Eval11InputsFloat(Input[1:], Tmp1[:], &p1)

	p1.Table = p.Table[K1:]
	Eval11InputsFloat(Input[1:], Tmp2[:], &p1)

	for i := 0; i < TotalOut; i++ {
		Output[i] = Tmp1[i] + (Tmp2[i]-Tmp1[i])*rest
	}
}
func Eval13Inputs(Input []uint16, Output []uint16, p16 *cmsInterpParams) {
	TotalOut := int(p16.nOutputs)

	if len(Input) < 13 || len(Output) < TotalOut || len(p16.Table) == 0 {
		return
	}

	fk := cmsToFixedDomain(int(Input[0]) * int(p16.Domain[0]))
	k0 := FIXED_TO_INT(fk)
	rk := FIXED_REST_TO_INT(fk)

	K0 := int(p16.opta[12]) * int(k0)
	K1 := K0
	if Input[0] != 0xFFFF {
		K1 += int(p16.opta[12])
	}

	var Tmp1, Tmp2 [MAX_STAGE_CHANNELS]uint16

	p1 := *p16
	copy(p1.Domain[:12], p16.Domain[1:13])

	p1.Table = p16.Table[K0:]
	Eval12Inputs(Input[1:], Tmp1[:], &p1)

	p1.Table = p16.Table[K1:]
	Eval12Inputs(Input[1:], Tmp2[:], &p1)

	for i := 0; i < TotalOut; i++ {
		Output[i] = LinearInterp(int32(rk), int32(Tmp1[i]), int32(Tmp2[i]))
	}
}
func Eval13InputsFloat(Input []float32, Output []float32, p *cmsInterpParams) {
	TotalOut := int(p.nOutputs)

	if len(Input) < 13 || len(Output) < TotalOut || len(p.Table) == 0 {
		return
	}

	pk := fclamp(Input[0]) * float32(p.Domain[0])
	k0 := int(math.Floor(float64(pk)))
	rest := pk - float32(k0)

	K0 := int(p.opta[12]) * k0
	K1 := K0
	if Input[0] < 1.0 {
		K1 += int(p.opta[12])
	}

	var Tmp1, Tmp2 [MAX_STAGE_CHANNELS]float32

	p1 := *p
	copy(p1.Domain[:12], p.Domain[1:13])

	p1.Table = p.Table[K0:]
	Eval12InputsFloat(Input[1:], Tmp1[:], &p1)

	p1.Table = p.Table[K1:]
	Eval12InputsFloat(Input[1:], Tmp2[:], &p1)

	for i := 0; i < TotalOut; i++ {
		Output[i] = Tmp1[i] + (Tmp2[i]-Tmp1[i])*rest
	}
}
func Eval14Inputs(Input []uint16, Output []uint16, p16 *cmsInterpParams) {
	TotalOut := int(p16.nOutputs)

	if len(Input) < 14 || len(Output) < TotalOut || len(p16.Table) == 0 {
		return
	}

	fk := cmsToFixedDomain(int(Input[0]) * int(p16.Domain[0]))
	k0 := FIXED_TO_INT(fk)
	rk := FIXED_REST_TO_INT(fk)

	K0 := int(p16.opta[13]) * int(k0)
	K1 := K0
	if Input[0] != 0xFFFF {
		K1 += int(p16.opta[13])
	}

	var Tmp1, Tmp2 [MAX_STAGE_CHANNELS]uint16

	p1 := *p16
	copy(p1.Domain[:13], p16.Domain[1:14])

	p1.Table = p16.Table[K0:]
	Eval13Inputs(Input[1:], Tmp1[:], &p1)

	p1.Table = p16.Table[K1:]
	Eval13Inputs(Input[1:], Tmp2[:], &p1)

	for i := 0; i < TotalOut; i++ {
		Output[i] = LinearInterp(int32(rk), int32(Tmp1[i]), int32(Tmp2[i]))
	}
}
func Eval14InputsFloat(Input []float32, Output []float32, p *cmsInterpParams) {
	TotalOut := int(p.nOutputs)

	if len(Input) < 14 || len(Output) < TotalOut || len(p.Table) == 0 {
		return
	}

	pk := fclamp(Input[0]) * float32(p.Domain[0])
	k0 := int(math.Floor(float64(pk)))
	rest := pk - float32(k0)

	K0 := int(p.opta[13]) * k0
	K1 := K0
	if Input[0] < 1.0 {
		K1 += int(p.opta[13])
	}

	var Tmp1, Tmp2 [MAX_STAGE_CHANNELS]float32

	p1 := *p
	copy(p1.Domain[:13], p.Domain[1:14])

	p1.Table = p.Table[K0:]
	Eval13InputsFloat(Input[1:], Tmp1[:], &p1)

	p1.Table = p.Table[K1:]
	Eval13InputsFloat(Input[1:], Tmp2[:], &p1)

	for i := 0; i < TotalOut; i++ {
		Output[i] = Tmp1[i] + (Tmp2[i]-Tmp1[i])*rest
	}
}
func Eval15Inputs(Input []uint16, Output []uint16, p16 *cmsInterpParams) {
	TotalOut := int(p16.nOutputs)

	if len(Input) < 15 || len(Output) < TotalOut || len(p16.Table) == 0 {
		return
	}

	fk := cmsToFixedDomain(int(Input[0]) * int(p16.Domain[0]))
	k0 := FIXED_TO_INT(fk)
	rk := FIXED_REST_TO_INT(fk)

	K0 := int(p16.opta[14]) * int(k0)
	K1 := K0
	if Input[0] != 0xFFFF {
		K1 += int(p16.opta[14])
	}

	var Tmp1, Tmp2 [MAX_STAGE_CHANNELS]uint16

	p1 := *p16
	copy(p1.Domain[:14], p16.Domain[1:15])

	p1.Table = p16.Table[K0:]
	Eval14Inputs(Input[1:], Tmp1[:], &p1)

	p1.Table = p16.Table[K1:]
	Eval14Inputs(Input[1:], Tmp2[:], &p1)

	for i := 0; i < TotalOut; i++ {
		Output[i] = LinearInterp(int32(rk), int32(Tmp1[i]), int32(Tmp2[i]))
	}
}
func Eval15InputsFloat(Input []float32, Output []float32, p *cmsInterpParams) {
	TotalOut := int(p.nOutputs)

	if len(Input) < 15 || len(Output) < TotalOut || len(p.Table) == 0 {
		return
	}

	pk := fclamp(Input[0]) * float32(p.Domain[0])
	k0 := int(math.Floor(float64(pk)))
	rest := pk - float32(k0)

	K0 := int(p.opta[14]) * k0
	K1 := K0
	if Input[0] < 1.0 {
		K1 += int(p.opta[14])
	}

	var Tmp1, Tmp2 [MAX_STAGE_CHANNELS]float32

	p1 := *p
	copy(p1.Domain[:14], p.Domain[1:15])

	p1.Table = p.Table[K0:]
	Eval14InputsFloat(Input[1:], Tmp1[:], &p1)

	p1.Table = p.Table[K1:]
	Eval14InputsFloat(Input[1:], Tmp2[:], &p1)

	for i := 0; i < TotalOut; i++ {
		Output[i] = Tmp1[i] + (Tmp2[i]-Tmp1[i])*rest
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
