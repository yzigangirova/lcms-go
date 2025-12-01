package golcms

import (
	"math"
	"unsafe"

	"github.com/yzigangirova/lcms-go/mem"
)

// This is the default factory
var cmsInterpPluginChunk = cmsInterpPluginChunkType{Interpolators: nil}

// cmsAllocInterpPluginChunk allocates and duplicates the interpolation plug-in memory chunk.
func cmsAllocInterpPluginChunk(mm mem.Manager, ctx, src *CmsContextStruct) {
	var from cmsContextChunk

	if src != nil {
		from = src.chunks[InterpPlugin]
	} else {
		// Default interpolation chunk
		staticInterpPluginChunk := cmsInterpPluginChunkType{Interpolators: nil}
		from = &staticInterpPluginChunk
	}

	ctx.chunks[InterpPlugin] = cmsSubAllocDup(mm, ctx.MemPool, from, uint32(unsafe.Sizeof(cmsInterpPluginChunkType{})))
}

// cmsRegisterInterpPlugin is the main entry for interpolation plug-in registration.
func cmsRegisterInterpPlugin(ContextID CmsContext, Data PluginIntrfc) bool {
	ptr := CmsContextGetClientChunk(ContextID, InterpPlugin).(*cmsInterpPluginChunkType)

	if Data == nil {
		ptr.Interpolators = nil
		return true
	}
	plugin, ok := Data.(*cmsPluginInterpolation)
	if !ok {
		panic("Plugin is not of the type cmsPluginInterpolation\n")
	}
	// Set replacement functions
	ptr.Interpolators = plugin.InterpolatorsFactory
	return true
}

// cmsSetInterpolationRoutine sets the interpolation method.
func cmsSetInterpolationRoutine(ContextID CmsContext, p *cmsInterpParams) bool {
	ptr := CmsContextGetClientChunk(ContextID, InterpPlugin).(*cmsInterpPluginChunkType)

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
func cmsComputeInterpParamsEx(mm mem.Manager,
	ContextID CmsContext,
	nSamples []uint32,
	InputChan uint32,
	OutputChan uint32,
	Table any,
	dwFlags uint32,
) *cmsInterpParams {
	//fmt.Printf("cmsComputeInterpParamsEx")

	var i uint32

	// Check for maximum inputs
	if InputChan > MAX_INPUT_DIMENSIONS {
		cmsSignalError(ContextID, cmsERROR_RANGE, "Too many input channels ")
		return nil
	}

	// Create an empty object
	p := mem.New[cmsInterpParams](mm)
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
		cmsSignalError(ContextID, cmsERROR_UNKNOWN_EXTENSION, "Unsupported interpolation")
		cmsFree(ContextID, p)
		return nil
	}

	return p
}

// cmsComputeInterpParams is a wrapper assuming all directions have the same number of nodes.
func cmsComputeInterpParams(mm mem.Manager,
	ContextID CmsContext,
	nSamples uint32,
	InputChan uint32,
	OutputChan uint32,
	Table any,
	dwFlags uint32,
) *cmsInterpParams {
	//fmt.Printf("cmsComputeInterpParams")
	var Samples [MAX_INPUT_DIMENSIONS]uint32

	// Fill the auxiliary array
	for i := 0; i < MAX_INPUT_DIMENSIONS; i++ {
		Samples[i] = nSamples
	}

	// Call the extended function
	return cmsComputeInterpParamsEx(mm, ContextID, Samples[:], InputChan, OutputChan, Table, dwFlags)
}

// cmsFreeInterpParams frees all associated memory.
func cmsFreeInterpParams(p *cmsInterpParams) {
	if p != nil {
		cmsFree(p.ContextID, p)
	}
}

// LinearInterp performs inline fixed-point interpolation.
func LinearInterp(a, l, h int32) uint16 {
	dif := uint32(uint32(h-l)*uint32(a) + 0x8000)
	dif = uint32(int32(dif>>16) + l)
	return uint16(dif)
}


// Linear interpolation (fixed-point optimized)
func LinLerp1D(mm mem.Manager, Value, Output []uint16, p *cmsInterpParams) {
	// Ensure `p.Table` is a `[]uint16`
	LutTable, ok := p.Table.([]uint16)
	if !ok {
		panic("p.Table is not of type []uint16 in LinLerp1D")
	}
	// Fast path for last value or degenerate table.
	if Value[0] == 0xFFFF || p.Domain[0] == 0 {
		Output[0] = LutTable[p.Domain[0]]
	} else {
		// val3 = Domain * Value in integer space, then convert to 15.16 fixed.
		val3 := int(p.Domain[0]) * int(Value[0])
		fx := cmsToFixedDomain(val3) // cmsS15Fixed16Number

		cell0 := FIXED_TO_INT(fx)     // integer part
		rest := FIXED_REST_TO_INT(fx) // fractional part (0..0xFFFF)

		i := int(cell0)
		// Prove to the compiler that i+1 is in range for both accesses below.
		_ = LutTable[i+1]

		y0 := LutTable[i]
		y1 := LutTable[i+1]

		Output[0] = LinearInterp(int32(rest), int32(y0), int32(y1))
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

// LinLerp1Dfloat performs 1D linear interpolation on floating-point values.
func LinLerp1Dfloat(mm mem.Manager, Value []float32, Output []float32, p *cmsInterpParams) {
	var y1, y0, val2, rest float32
	var cell0, cell1 int

	// Ensure p.Table is a []float32
	LutTable, ok := p.Table.([]float32)
	if !ok {
		panic("p.Table is not of type []float32 in LinLerp1Dfloat")
	}

	val2 = fclamp(Value[0])

	// If last value or domain is zero
	if val2 == 1.0 || p.Domain[0] == 0 {
		Output[0] = LutTable[p.Domain[0]]
	} else {
		val2 *= float32(p.Domain[0])

		cell0 = int(math.Floor(float64(val2)))
		cell1 = int(math.Ceil(float64(val2)))

		// Rest is the fractional part
		rest = val2 - float32(cell0)

		y0 = LutTable[cell0]
		y1 = LutTable[cell1]

		Output[0] = y0 + (y1-y0)*rest
	}
}

// Scalar, fixed-point path (uint16 -> uint16)
func LinLerp1DScalar16(v uint16, p *cmsInterpParams) uint16 {
	// Table is []uint16, domain is number of intervals (so last node is Domain[0])
	LutTable, ok := p.Table.([]uint16)
	if !ok {
		panic("p.Table is not of type []uint16 in LinLerp1DScalar16")
	}

	// If last value or degenerate domain => return last node
	if v == 0xFFFF || p.Domain[0] == 0 {
		idx := int(p.Domain[0])
		if idx < 0 || idx >= len(LutTable) {
			panic("LinLerp1DScalar16: last index out of range")
		}
		return LutTable[idx]
	}

	// Scale to table domain and convert to 15.16 fixed
	val := uint32(p.Domain[0]) * uint32(v)     // up to ~65535*65535, fits in uint32
	fixed := int32(cmsToFixedDomain(int(val))) // 15.16 fixed

	cell0 := int32(FIXED_TO_INT(cmsS15Fixed16Number(fixed)))     // integer part
	rest := int32(FIXED_REST_TO_INT(cmsS15Fixed16Number(fixed))) // fractional (0..65535)

	// Bounds: we need cell0 and cell0+1 valid
	if cell0 < 0 || int(cell0)+1 >= len(LutTable) {
		panic("LinLerp1DScalar16: interpolation index out of range")
	}

	y0 := int32(LutTable[cell0])
	y1 := int32(LutTable[cell0+1])

	return LinearInterp(rest, y0, y1)
}

// Eval1Input performs 1D interpolation for a single input.
func Eval1Input(mm mem.Manager, Input []uint16, Output []uint16, p16 *cmsInterpParams) {
	var fk, k0, k1, rk, K0, K1 cmsS15Fixed16Number
	var v int
	var OutChan uint32

	// Ensure p16.Table is a []uint16
	LutTable, ok := p16.Table.([]uint16)
	if !ok {
		panic("p16.Table is not of type []uint16 in Eval1Input")
	}

	// If last value or domain is zero
	if Input[0] == 0xffff || p16.Domain[0] == 0 {
		y0 := uint32(p16.Domain[0]) * uint32(p16.opta[0])

		for OutChan = 0; OutChan < p16.nOutputs; OutChan++ {
			// Direct slice indexing
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
			// Direct slice indexing
			LutTableVal0 := LutTable[K0+cmsS15Fixed16Number(OutChan)]
			LutTableVal1 := LutTable[K1+cmsS15Fixed16Number(OutChan)]

			// Assign interpolated value to Output slice
			Output[OutChan] = LinearInterp(int32(rk), int32(LutTableVal0), int32(LutTableVal1))
		}
	}
}

// Eval1InputFloat evaluates a gray LUT having only one input channel (float version).
func Eval1InputFloat(mm mem.Manager, Value []float32, Output []float32, p *cmsInterpParams) {
	var y1, y0, val2, rest float32
	var cell0, cell1 int

	// Ensure Value and Output have at least 1 element
	if len(Value) == 0 || len(Output) < int(p.nOutputs) || p.Table == nil {
		cmsSignalError(nil, cmsERROR_UNDEFINED, "Invalid input parameters in Eval1InputFloat")
		return
	}

	// Ensure p.Table is of type []float32
	LutTable, ok := p.Table.([]float32)
	if !ok {
		panic("p.Table is not of type []float32 in Eval1InputFloat")

	}

	val2 = fclamp(Value[0])

	// If last value or domain is zero
	if val2 == 1.0 || p.Domain[0] == 0 {
		start := uint32(p.Domain[0]) * uint32(p.opta[0])

		for OutChan := uint32(0); OutChan < p.nOutputs; OutChan++ {
			// Direct slice indexing
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
func BilinearInterpFloat(mm mem.Manager, Input []float32, Output []float32, p *cmsInterpParams) {
	TotalOut := int(p.nOutputs)

	// Ensure Input and Output have enough elements
	if len(Input) < 2 || len(Output) < TotalOut {
		return
	}

	// Ensure p.Table is of type []float32
	LutTable, ok := p.Table.([]float32)
	if !ok {
		panic("p.Table is not of type []float32 in Eval1InputFloat")

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
func BilinearInterp16(mm mem.Manager, Input []uint16, Output []uint16, p *cmsInterpParams) {
	TotalOut := int(p.nOutputs)

	// Ensure Input and Output have enough elements
	if len(Input) < 2 || len(Output) < TotalOut {
		return
	}
	// Ensure p16.Table is a []uint16
	LutTable, ok := p.Table.([]uint16)
	if !ok {
		panic("p16.Table is not of type []uint16 in Eval1Input")

	}

	// Inline functions for LERP and DENS
	LERP := func(a int, l, h int) uint16 {
		return uint16(int32(l) + ROUND_FIXED_TO_INT(cmsS15Fixed16Number((h-l)*a)))
	}

	DENS := func(i, j, outChan int) int {
		return int(LutTable[i+j+outChan])
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
func TrilinearInterpFloat(mm mem.Manager, Input []float32, Output []float32, p *cmsInterpParams) {
	TotalOut := int(p.nOutputs)
	// Ensure p.Table is of type []float32
	LutTable, ok := p.Table.([]float32)
	if !ok {
		panic("p.Table is not of type []float32 in Eval1InputFloat")

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
func TrilinearInterp16(mm mem.Manager, Input []uint16, Output []uint16, p *cmsInterpParams) {
	//fmt.Println("start TrilinearInterp16")
	TotalOut := int(p.nOutputs)

	// Ensure p16.Table is a []uint16
	LutTable, ok := p.Table.([]uint16)
	if !ok {
		panic("p16.Table is not of type []uint16 in Eval1Input")

	}

	// Inline functions for LERP and DENS
	LERP := func(a, l, h int) uint16 {
		return uint16(int32(l) + ROUND_FIXED_TO_INT(cmsS15Fixed16Number((h-l)*a)))
	}

	DENS := func(i, j, k, outChan int) int {
		return int(LutTable[i+j+k+outChan])
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
	//fmt.Println("end TrilinearInterp16")

}

// TetrahedralInterpFloat — 3D tetrahedral for float32 (no K split).
func TetrahedralInterpFloat(mm mem.Manager, Input, Output []float32, p *cmsInterpParams) {
	totalOut := int(p.nOutputs)

	LutTable, ok := p.Table.([]float32)
	if !ok {
		panic("p.Table is not of type []float32 in TetrahedralInterpFloat")
	}

	px := fclamp(Input[0]) * float32(p.Domain[0])
	py := fclamp(Input[1]) * float32(p.Domain[1])
	pz := fclamp(Input[2]) * float32(p.Domain[2])

	x0 := int(px)
	rx := px - float32(x0)
	y0 := int(py)
	ry := py - float32(y0)
	z0 := int(pz)
	rz := pz - float32(z0)

	opx := int(p.opta[2])
	X0 := x0 * opx
	X1 := X0
	if Input[0] < 1.0 {
		X1 += opx
	}
	opy := int(p.opta[1])
	Y0 := y0 * opy
	Y1 := Y0
	if Input[1] < 1.0 {
		Y1 += opy
	}
	opz := int(p.opta[0])
	Z0 := z0 * opz
	Z1 := Z0
	if Input[2] < 1.0 {
		Z1 += opz
	}

	// one-time guard
	maxOff := X1 + Y1 + Z1 + (totalOut - 1)
	_ = LutTable[maxOff]

	// indices
	idx000 := X0 + Y0 + Z0
	idx100 := X1 + Y0 + Z0
	idx110 := X1 + Y1 + Z0
	idx111 := X1 + Y1 + Z1
	idx101 := X1 + Y0 + Z1
	idx001 := X0 + Y0 + Z1
	idx011 := X0 + Y1 + Z1
	idx010 := X0 + Y1 + Z0

	// case once
	var caseID int
	switch {
	case rx >= ry && ry >= rz:
		caseID = 0
	case rx >= rz && rz >= ry:
		caseID = 1
	case rz >= rx && rx >= ry:
		caseID = 2
	case ry >= rx && rx >= rz:
		caseID = 3
	case ry >= rz && rz >= rx:
		caseID = 4
	default:
		caseID = 5
	}

	switch caseID {
	case 0:
		for out := 0; out < totalOut; out++ {
			c0 := LutTable[idx000+out]
			c1 := LutTable[idx100+out] - c0
			c2 := LutTable[idx110+out] - LutTable[idx100+out]
			c3 := LutTable[idx111+out] - LutTable[idx110+out]
			Output[out] = c0 + c1*rx + c2*ry + c3*rz
		}
	case 1:
		for out := 0; out < totalOut; out++ {
			c0 := LutTable[idx000+out]
			c1 := LutTable[idx100+out] - c0
			c2 := LutTable[idx111+out] - LutTable[idx101+out]
			c3 := LutTable[idx101+out] - LutTable[idx100+out]
			Output[out] = c0 + c1*rx + c2*ry + c3*rz
		}
	case 2:
		for out := 0; out < totalOut; out++ {
			c0 := LutTable[idx000+out]
			c1 := LutTable[idx101+out] - LutTable[idx001+out]
			c2 := LutTable[idx111+out] - LutTable[idx101+out]
			c3 := LutTable[idx001+out] - c0
			Output[out] = c0 + c1*rx + c2*ry + c3*rz
		}
	case 3:
		for out := 0; out < totalOut; out++ {
			c0 := LutTable[idx000+out]
			c1 := LutTable[idx110+out] - LutTable[idx010+out]
			c2 := LutTable[idx010+out] - c0
			c3 := LutTable[idx111+out] - LutTable[idx110+out]
			Output[out] = c0 + c1*rx + c2*ry + c3*rz
		}
	case 4:
		for out := 0; out < totalOut; out++ {
			c0 := LutTable[idx000+out]
			c1 := LutTable[idx111+out] - LutTable[idx011+out]
			c2 := LutTable[idx010+out] - c0
			c3 := LutTable[idx011+out] - LutTable[idx010+out]
			Output[out] = c0 + c1*rx + c2*ry + c3*rz
		}
	default:
		for out := 0; out < totalOut; out++ {
			c0 := LutTable[idx000+out]
			c1 := LutTable[idx111+out] - LutTable[idx011+out]
			c2 := LutTable[idx011+out] - LutTable[idx001+out]
			c3 := LutTable[idx001+out] - c0
			Output[out] = c0 + c1*rx + c2*ry + c3*rz
		}
	}
}

func TetrahedralInterp16(mm mem.Manager, Input []uint16, Output []uint16, p *cmsInterpParams) {
	//fmt.Println("TetrahedralInterp16")

	// Variables
	LutTable, ok := p.Table.([]uint16)
	if !ok {
		panic("p.Table is not []uint16\n")
	}

	var fx, fy, fz cmsS15Fixed16Number
	var rx, ry, rz cmsS15Fixed16Number
	//var x0, y0, z0 int
	var c0, c1, c2, c3, Rest int32
	var X0, X1, Y0, Y1, Z0, Z1 uint32
	TotalOut := p.nOutputs

	fx = cmsToFixedDomain(int(Input[0]) * int(p.Domain[0]))
	fy = cmsToFixedDomain(int(Input[1]) * int(p.Domain[1]))
	fz = cmsToFixedDomain(int(Input[2]) * int(p.Domain[2]))

	x0 := FIXED_TO_INT(fx)
	y0 := FIXED_TO_INT(fy)
	z0 := FIXED_TO_INT(fz)

	rx = cmsS15Fixed16Number(FIXED_REST_TO_INT(fx))
	ry = cmsS15Fixed16Number(FIXED_REST_TO_INT(fy))
	rz = cmsS15Fixed16Number(FIXED_REST_TO_INT(fz))

	X0 = uint32(p.opta[2]) * uint32(x0)
	X1 = 0
	if Input[0] != 0xFFFF {
		X1 = uint32(p.opta[2])
	}

	Y0 = uint32(p.opta[1]) * uint32(y0)
	Y1 = 0
	if Input[1] != 0xFFFF {
		Y1 = uint32(p.opta[1])
	}

	Z0 = uint32(p.opta[0]) * uint32(z0)
	Z1 = 0
	if Input[2] != 0xFFFF {
		Z1 = uint32(p.opta[0])
	}

	LutOffset := X0 + Y0 + Z0

	if rx >= ry {
		if ry >= rz {
			Y1 += X1
			Z1 += Y1
			for n := uint32(0); n < TotalOut; n++ {
				base := LutOffset + n
				c1 = int32(LutTable[base+X1])
				c2 = int32(LutTable[base+Y1])
				c3 = int32(LutTable[base+Z1])
				c0 = int32(LutTable[base])
				c3 -= c2
				c2 -= c1
				c1 -= c0
				Rest = c1*int32(rx) + c2*int32(ry) + c3*int32(rz) + 0x8001
				Output[n] = uint16(c0 + ((Rest + (Rest >> 16)) >> 16))
			}

		} else if rz >= rx {
			X1 += Z1
			Y1 += X1
			for n := uint32(0); n < TotalOut; n++ {
				base := LutOffset + n
				c1 = int32(LutTable[base+X1])
				c2 = int32(LutTable[base+Y1])
				c3 = int32(LutTable[base+Z1])
				c0 = int32(LutTable[base])
				c2 -= c1
				c1 -= c3
				c3 -= c0
				Rest = c1*int32(rx) + c2*int32(ry) + c3*int32(rz) + 0x8001
				Output[n] = uint16(c0 + ((Rest + (Rest >> 16)) >> 16))
			}

		} else {
			Z1 += X1
			Y1 += Z1
			for n := uint32(0); n < TotalOut; n++ {
				base := LutOffset + n
				c1 = int32(LutTable[base+X1])
				c2 = int32(LutTable[base+Y1])
				c3 = int32(LutTable[base+Z1])
				c0 = int32(LutTable[base])
				c2 -= c3
				c3 -= c1
				c1 -= c0
				Rest = c1*int32(rx) + c2*int32(ry) + c3*int32(rz) + 0x8001
				Output[n] = uint16(c0 + ((Rest + (Rest >> 16)) >> 16))
			}
		}

	} else {
		if rx >= rz {
			X1 += Y1
			Z1 += X1
			for n := uint32(0); n < TotalOut; n++ {
				base := LutOffset + n
				c1 = int32(LutTable[base+X1])
				c2 = int32(LutTable[base+Y1])
				c3 = int32(LutTable[base+Z1])
				c0 = int32(LutTable[base])
				c3 -= c1
				c1 -= c2
				c2 -= c0
				Rest = c1*int32(rx) + c2*int32(ry) + c3*int32(rz) + 0x8001
				Output[n] = uint16(c0 + ((Rest + (Rest >> 16)) >> 16))
			}

		} else if ry >= rz {
			Z1 += Y1
			X1 += Z1
			for n := uint32(0); n < TotalOut; n++ {
				base := LutOffset + n
				c1 = int32(LutTable[base+X1])
				c2 = int32(LutTable[base+Y1])
				c3 = int32(LutTable[base+Z1])
				c0 = int32(LutTable[base])
				c1 -= c3
				c3 -= c2
				c2 -= c0
				Rest = c1*int32(rx) + c2*int32(ry) + c3*int32(rz) + 0x8001
				Output[n] = uint16(c0 + ((Rest + (Rest >> 16)) >> 16))
			}

		} else {
			Y1 += Z1
			X1 += Y1
			for n := uint32(0); n < TotalOut; n++ {
				base := LutOffset + n
				c1 = int32(LutTable[base+X1])
				c2 = int32(LutTable[base+Y1])
				c3 = int32(LutTable[base+Z1])
				c0 = int32(LutTable[base])
				c1 -= c2
				c2 -= c3
				c3 -= c0
				Rest = c1*int32(rx) + c2*int32(ry) + c3*int32(rz) + 0x8001
				Output[n] = uint16(c0 + ((Rest + (Rest >> 16)) >> 16))
			}
		}
	}
}

// Eval4Inputs — optimized, WASM-safe, no unsafe pointers.
// Key changes explained after the code.
func Eval4Inputs(mm mem.Manager, Input, Output []uint16, p *cmsInterpParams) {
	totalOut := int(p.nOutputs)

	LutTable, ok := p.Table.([]uint16)
	if !ok {
		panic("p.Table is not of type []uint16 in Eval4Inputs")
	}

	// --- fixed-point mapping of inputs to grid (keep your original helpers here) ---
	fk := cmsToFixedDomain(int(Input[0]) * int(p.Domain[0]))
	fx := cmsToFixedDomain(int(Input[1]) * int(p.Domain[1]))
	fy := cmsToFixedDomain(int(Input[2]) * int(p.Domain[2]))
	fz := cmsToFixedDomain(int(Input[3]) * int(p.Domain[3]))

	// integer parts (grid indices)
	k0 := FIXED_TO_INT(fk)
	x0 := FIXED_TO_INT(fx)
	y0 := FIXED_TO_INT(fy)
	z0 := FIXED_TO_INT(fz)

	// fractional parts [0..65535]
	rk := int32(FIXED_REST_TO_INT(fk))
	rx := int32(FIXED_REST_TO_INT(fx))
	ry := int32(FIXED_REST_TO_INT(fy))
	rz := int32(FIXED_REST_TO_INT(fz))

	// Convert stride multipliers once
	opk := int(p.opta[3])
	opx := int(p.opta[2])
	opy := int(p.opta[1])
	opz := int(p.opta[0])

	// Base offsets for K planes
	K0 := int(k0) * opk
	K1 := K0
	if Input[0] != 0xFFFF {
		K1 += opk
	}

	// Base offsets for X/Y/Z planes
	X0 := int(x0) * opx
	X1 := X0
	if Input[1] != 0xFFFF {
		X1 += opx
	}
	Y0 := int(y0) * opy
	Y1 := Y0
	if Input[2] != 0xFFFF {
		Y1 += opy
	}
	Z0 := int(z0) * opz
	Z1 := Z0
	if Input[3] != 0xFFFF {
		Z1 += opz
	}

	// ---- scratch buffers (preallocated) ----
	sc := mm.Scratch()
	Tmp1 := sc.Tmp1U16[:totalOut]
	Tmp2 := sc.Tmp2U16[:totalOut]

	// ---- slice bases and a single bounds-guard to elide inner checks ----
	baseK0 := LutTable[K0:]
	baseK1 := LutTable[K1:]
	maxOff := X1 + Y1 + Z1 + (totalOut - 1)
	_ = baseK0[maxOff]
	_ = baseK1[maxOff]

	// Corner indices once (independent of channel)
	idx000 := X0 + Y0 + Z0
	idx100 := X1 + Y0 + Z0
	idx110 := X1 + Y1 + Z0
	idx111 := X1 + Y1 + Z1
	idx101 := X1 + Y0 + Z1
	idx001 := X0 + Y0 + Z1
	idx011 := X0 + Y1 + Z1
	idx010 := X0 + Y1 + Z0

	// Decide permutation order ONCE (don’t redo per channel)
	var caseID int
	switch {
	case rx >= ry && ry >= rz:
		caseID = 0
	case rx >= rz && rz >= ry:
		caseID = 1
	case rz >= rx && rx >= ry:
		caseID = 2
	case ry >= rx && rx >= rz:
		caseID = 3
	case ry >= rz && rz >= rx:
		caseID = 4
	default: // rz >= ry && ry >= rx
		caseID = 5
	}

	// Pre-widen fractions once
	rx64, ry64, rz64 := int64(rx), int64(ry), int64(rz)

	// Plane runner: identical math as before, but we (1) avoid cmsToFixedDomain+ROUND inside,
	// and (2) avoid re-evaluating the branchy order selection per channel.
	doPlane := func(base []uint16, tmp []uint16) {
		switch caseID {
		case 0: // rx >= ry >= rz
			for out := 0; out < totalOut; out++ {
				c0 := int32(base[idx000+out])
				p100 := int32(base[idx100+out])
				p110 := int32(base[idx110+out])
				p111 := int32(base[idx111+out])

				c1 := p100 - c0
				c2 := p110 - p100
				c3 := p111 - p110

				rest := int64(c1)*rx64 + int64(c2)*ry64 + int64(c3)*rz64 // 16.16
				tmp[out] = uint16(c0 + int32((rest+0x8000)>>16))         // round >> 16
			}
		case 1: // rx >= rz >= ry
			for out := 0; out < totalOut; out++ {
				c0 := int32(base[idx000+out])
				p100 := int32(base[idx100+out])
				p101 := int32(base[idx101+out])
				p111 := int32(base[idx111+out])

				c1 := p100 - c0
				c2 := p111 - p101
				c3 := p101 - p100

				rest := int64(c1)*rx64 + int64(c2)*ry64 + int64(c3)*rz64
				tmp[out] = uint16(c0 + int32((rest+0x8000)>>16))
			}
		case 2: // rz >= rx >= ry
			for out := 0; out < totalOut; out++ {
				c0 := int32(base[idx000+out])
				p001 := int32(base[idx001+out])
				p101 := int32(base[idx101+out])
				p111 := int32(base[idx111+out])

				c1 := p101 - p001
				c2 := p111 - p101
				c3 := p001 - c0

				rest := int64(c1)*rx64 + int64(c2)*ry64 + int64(c3)*rz64
				tmp[out] = uint16(c0 + int32((rest+0x8000)>>16))
			}
		case 3: // ry >= rx >= rz
			for out := 0; out < totalOut; out++ {
				c0 := int32(base[idx000+out])
				p010 := int32(base[idx010+out])
				p110 := int32(base[idx110+out])
				p111 := int32(base[idx111+out])

				c1 := p110 - p010
				c2 := p010 - c0
				c3 := p111 - p110

				rest := int64(c1)*rx64 + int64(c2)*ry64 + int64(c3)*rz64
				tmp[out] = uint16(c0 + int32((rest+0x8000)>>16))
			}
		case 4: // ry >= rz >= rx
			for out := 0; out < totalOut; out++ {
				c0 := int32(base[idx000+out])
				p010 := int32(base[idx010+out])
				p011 := int32(base[idx011+out])
				p111 := int32(base[idx111+out])

				c1 := p111 - p011
				c2 := p010 - c0
				c3 := p011 - p010

				rest := int64(c1)*rx64 + int64(c2)*ry64 + int64(c3)*rz64
				tmp[out] = uint16(c0 + int32((rest+0x8000)>>16))
			}
		default: // 5: rz >= ry >= rx
			for out := 0; out < totalOut; out++ {
				c0 := int32(base[idx000+out])
				p001 := int32(base[idx001+out])
				p011 := int32(base[idx011+out])
				p111 := int32(base[idx111+out])

				c1 := p111 - p011
				c2 := p011 - p001
				c3 := p001 - c0

				rest := int64(c1)*rx64 + int64(c2)*ry64 + int64(c3)*rz64
				tmp[out] = uint16(c0 + int32((rest+0x8000)>>16))
			}
		}
	}

	// Process both planes
	doPlane(baseK0, Tmp1)
	doPlane(baseK1, Tmp2)

	// ---- final blend (inline LinearInterp) ----
	rk64 := int64(rk)
	for i := 0; i < totalOut; i++ {
		lo := int32(Tmp1[i])
		hi := int32(Tmp2[i])
		// lo + ((hi-lo)*rk + 0x8000)>>16
		Output[i] = uint16(lo + int32(((int64(hi-lo)*rk64 + 0x8000) >> 16)))
	}
}

// Eval4InputsFloat performs tetrahedral interpolation with 4 input channels for floating-point values.
// Eval4InputsFloat — tetrahedral interp with 4 inputs (float32), K split + 3D tetra.
// WASM-safe, no unsafe. Mirrors the structure you now use in Eval4Inputs (U16).
func Eval4InputsFloat(mm mem.Manager, Input, Output []float32, p *cmsInterpParams) {
	totalOut := int(p.nOutputs)

	LutTable, ok := p.Table.([]float32)
	if !ok {
		panic("p.Table is not of type []float32 in Eval4InputsFloat")
	}

	// ----- K axis (1st input) -----
	pk := fclamp(Input[0]) * float32(p.Domain[0])
	k0 := int(pk)             // floor
	restK := pk - float32(k0) // [0..1)
	opk := int(p.opta[3])
	K0 := k0 * opk
	K1 := K0
	if Input[0] < 1.0 { // top-cell clamp
		K1 += opk
	}

	// ----- X/Y/Z axes (inputs 1..3) -----
	px := fclamp(Input[1]) * float32(p.Domain[1])
	py := fclamp(Input[2]) * float32(p.Domain[2])
	pz := fclamp(Input[3]) * float32(p.Domain[3])

	x0 := int(px)
	rx := px - float32(x0)
	y0 := int(py)
	ry := py - float32(y0)
	z0 := int(pz)
	rz := pz - float32(z0)

	opx := int(p.opta[2])
	opy := int(p.opta[1])
	opz := int(p.opta[0])

	X0 := x0 * opx
	Y0 := y0 * opy
	Z0 := z0 * opz

	X1 := X0
	if Input[1] < 1.0 {
		X1 += opx
	}
	Y1 := Y0
	if Input[2] < 1.0 {
		Y1 += opy
	}
	Z1 := Z0
	if Input[3] < 1.0 {
		Z1 += opz
	}

	// ---- scratch buffers ----
	sc := mm.Scratch()
	Tmp1 := sc.Tmp1F32[:totalOut]
	Tmp2 := sc.Tmp2F32[:totalOut]

	// ---- base slices + single bounds guard ----
	baseK0 := LutTable[K0:]
	baseK1 := LutTable[K1:]
	maxOff := X1 + Y1 + Z1 + (totalOut - 1)
	_ = baseK0[maxOff]
	_ = baseK1[maxOff]

	// Corner linear indices (independent of channel)
	idx000 := X0 + Y0 + Z0
	idx100 := X1 + Y0 + Z0
	idx110 := X1 + Y1 + Z0
	idx111 := X1 + Y1 + Z1
	idx101 := X1 + Y0 + Z1
	idx001 := X0 + Y0 + Z1
	idx011 := X0 + Y1 + Z1
	idx010 := X0 + Y1 + Z0

	// Order selection once (same cases as U16 path)
	var caseID int
	switch {
	case rx >= ry && ry >= rz:
		caseID = 0
	case rx >= rz && rz >= ry:
		caseID = 1
	case rz >= rx && rx >= ry:
		caseID = 2
	case ry >= rx && rx >= rz:
		caseID = 3
	case ry >= rz && rz >= rx:
		caseID = 4
	default: // rz >= ry && ry >= rx
		caseID = 5
	}

	// ----- plane worker: inlined tetrahedral on floats -----
	doPlane := func(base []float32, tmp []float32) {
		switch caseID {
		case 0: // rx >= ry >= rz
			for out := 0; out < totalOut; out++ {
				c0 := base[idx000+out]
				p100 := base[idx100+out]
				p110 := base[idx110+out]
				p111 := base[idx111+out]

				c1 := p100 - c0
				c2 := p110 - p100
				c3 := p111 - p110

				tmp[out] = c0 + c1*rx + c2*ry + c3*rz
			}
		case 1: // rx >= rz >= ry
			for out := 0; out < totalOut; out++ {
				c0 := base[idx000+out]
				p100 := base[idx100+out]
				p101 := base[idx101+out]
				p111 := base[idx111+out]

				c1 := p100 - c0
				c2 := p111 - p101
				c3 := p101 - p100

				tmp[out] = c0 + c1*rx + c2*ry + c3*rz
			}
		case 2: // rz >= rx >= ry
			for out := 0; out < totalOut; out++ {
				c0 := base[idx000+out]
				p001 := base[idx001+out]
				p101 := base[idx101+out]
				p111 := base[idx111+out]

				c1 := p101 - p001
				c2 := p111 - p101
				c3 := p001 - c0

				tmp[out] = c0 + c1*rx + c2*ry + c3*rz
			}
		case 3: // ry >= rx >= rz
			for out := 0; out < totalOut; out++ {
				c0 := base[idx000+out]
				p010 := base[idx010+out]
				p110 := base[idx110+out]
				p111 := base[idx111+out]

				c1 := p110 - p010
				c2 := p010 - c0
				c3 := p111 - p110

				tmp[out] = c0 + c1*rx + c2*ry + c3*rz
			}
		case 4: // ry >= rz >= rx
			for out := 0; out < totalOut; out++ {
				c0 := base[idx000+out]
				p010 := base[idx010+out]
				p011 := base[idx011+out]
				p111 := base[idx111+out]

				c1 := p111 - p011
				c2 := p010 - c0
				c3 := p011 - p010

				tmp[out] = c0 + c1*rx + c2*ry + c3*rz
			}
		default: // 5: rz >= ry >= rx
			for out := 0; out < totalOut; out++ {
				c0 := base[idx000+out]
				p001 := base[idx001+out]
				p011 := base[idx011+out]
				p111 := base[idx111+out]

				c1 := p111 - p011
				c2 := p011 - p001
				c3 := p001 - c0

				tmp[out] = c0 + c1*rx + c2*ry + c3*rz
			}
		}
	}

	// Evaluate both K-planes
	doPlane(baseK0, Tmp1)
	doPlane(baseK1, Tmp2)

	// Final blend along K
	for i := 0; i < totalOut; i++ {
		Output[i] = Tmp1[i] + (Tmp2[i]-Tmp1[i])*restK
	}
}

// Eval5Inputs evaluates a 5-input LUT.
func Eval5Inputs(mm mem.Manager, Input []uint16, Output []uint16, p16 *cmsInterpParams) {
	TotalOut := int(p16.nOutputs)

	// Ensure p16.Table is of type []uint16
	LutTable, ok := p16.Table.([]uint16)
	if !ok {
		panic("p16.Table is not of type []uint16 in Eval5Inputs")

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

	// Ensure K0 and K1 do not exceed LUT bounds
	if K0 >= len(LutTable) || K1 >= len(LutTable) {
		panic("LUT index out of range in Eval5Inputs")

	}

	// Temporary storage for interpolation results
	// ---- scratch buffers (replace local Tmp1/Tmp2) ----
	sc := mm.Scratch()
	Tmp1 := sc.Tmp1U16[:TotalOut]
	Tmp2 := sc.Tmp2U16[:TotalOut]
	// Create a new interpolation parameter structure
	p1 := *p16
	copy(p1.Domain[:4], p16.Domain[1:5])

	// Use a CHILD FRAME for the nested  calls
	mmInner := mm.NewFrame()
	defer mmInner.Close()

	// Process K0
	p1.Table = LutTable[K0:] // Adjust LUT slice for K0
	Eval4Inputs(mmInner, Input[1:], Tmp1[:], &p1)

	// Process K1
	p1.Table = LutTable[K1:] // Adjust LUT slice for K1
	Eval4Inputs(mmInner, Input[1:], Tmp2[:], &p1)

	// Final interpolation
	for i := 0; i < TotalOut; i++ {
		Output[i] = LinearInterp(int32(rk), int32(Tmp1[i]), int32(Tmp2[i]))
	}
}

// Eval5InputsFloat evaluates a 5-input LUT using floating-point interpolation.
func Eval5InputsFloat(mm mem.Manager, Input []float32, Output []float32, p *cmsInterpParams) {
	TotalOut := int(p.nOutputs)

	// Ensure p.Table is of type []float32
	LutTable, ok := p.Table.([]float32)
	if !ok {
		panic("p.Table is not of type []float32 in Eval5InputsFloat")

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

	// Ensure K0 and K1 do not exceed LUT bounds
	if K0 >= len(LutTable) || K1 >= len(LutTable) {
		panic("LUT index out of range in Eval5InputsFloat")

	}

	// Temporary storage for interpolation results
	// ---- scratch buffers (replace local Tmp1/Tmp2) ----
	sc := mm.Scratch()
	Tmp1 := sc.Tmp1F32[:TotalOut]
	Tmp2 := sc.Tmp2F32[:TotalOut]
	// Create a modified interpolation parameter structure
	p1 := *p
	copy(p1.Domain[:4], p.Domain[1:5])

	// Process K0
	p1.Table = LutTable[K0:] // Use `LutTable` directly with correct slicing
	// Use a CHILD FRAME for the nested  calls
	mmInner := mm.NewFrame()
	defer mmInner.Close()
	Eval4InputsFloat(mmInner, Input[1:], Tmp1[:], &p1)

	// Process K1
	p1.Table = LutTable[K1:] // Use `LutTable` directly with correct slicing
	Eval4InputsFloat(mmInner, Input[1:], Tmp2[:], &p1)

	// Final interpolation
	for i := 0; i < TotalOut; i++ {
		Output[i] = Tmp1[i] + (Tmp2[i]-Tmp1[i])*rest
	}
}

// Eval6Inputs evaluates a 6-input LUT with `[]uint16` table.
func Eval6Inputs(mm mem.Manager, Input []uint16, Output []uint16, p16 *cmsInterpParams) {
	TotalOut := int(p16.nOutputs)

	// Ensure p.Table is of type []uint16
	LutTable, ok := p16.Table.([]uint16)
	if !ok {
		panic("p.Table is not of type []uint16 in Eval6Inputs")

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

	// Ensure K0 and K1 do not exceed LUT bounds
	if K0 >= len(LutTable) || K1 >= len(LutTable) {
		panic("LUT index out of range in Eval6Inputs")

	}

	// Temporary storage for interpolation results
	// ---- scratch buffers (replace local Tmp1/Tmp2) ----
	sc := mm.Scratch()
	Tmp1 := sc.Tmp1U16[:TotalOut]
	Tmp2 := sc.Tmp2U16[:TotalOut]

	// Create a new interpolation parameter structure
	p1 := *p16
	copy(p1.Domain[:5], p16.Domain[1:6])

	// Use a CHILD FRAME for the nested  calls
	mmInner := mm.NewFrame()
	defer mmInner.Close()
	// Process K0
	p1.Table = LutTable[K0:] // Adjust LUT slice for K0
	Eval5Inputs(mmInner, Input[1:], Tmp1[:], &p1)

	// Process K1
	p1.Table = LutTable[K1:] // Adjust LUT slice for K1
	Eval5Inputs(mmInner, Input[1:], Tmp2[:], &p1)

	// Final interpolation
	for i := 0; i < TotalOut; i++ {
		Output[i] = LinearInterp(int32(rk), int32(Tmp1[i]), int32(Tmp2[i]))
	}
}

// Eval6InputsFloat evaluates a 6-input LUT with `[]float32` table.
func Eval6InputsFloat(mm mem.Manager, Input []float32, Output []float32, p *cmsInterpParams) {
	TotalOut := int(p.nOutputs)

	// Ensure p.Table is of type []float32
	LutTable, ok := p.Table.([]float32)
	if !ok {
		panic("p.Table is not of type []float32 in Eval6InputsFloat")

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

	// Ensure K0 and K1 do not exceed LUT bounds
	if K0 >= len(LutTable) || K1 >= len(LutTable) {
		panic("LUT index out of range in Eval6InputsFloat")

	}

	// Temporary storage for interpolation results
	// ---- scratch buffers (replace local Tmp1/Tmp2) ----
	sc := mm.Scratch()
	Tmp1 := sc.Tmp1F32[:TotalOut]
	Tmp2 := sc.Tmp2F32[:TotalOut]
	// Create a modified interpolation parameter structure
	p1 := *p
	copy(p1.Domain[:5], p.Domain[1:6])
	// Use a CHILD FRAME for the nested  calls
	mmInner := mm.NewFrame()
	defer mmInner.Close()

	// Process K0
	p1.Table = LutTable[K0:] // Use correct slicing
	Eval5InputsFloat(mmInner, Input[1:], Tmp1[:], &p1)

	// Process K1
	p1.Table = LutTable[K1:] // Use correct slicing
	Eval5InputsFloat(mmInner, Input[1:], Tmp2[:], &p1)

	// Final interpolation
	for i := 0; i < TotalOut; i++ {
		Output[i] = Tmp1[i] + (Tmp2[i]-Tmp1[i])*rest
	}
}

// Eval7Inputs evaluates a 7-input LUT with `[]uint16` table.
func Eval7Inputs(mm mem.Manager, Input []uint16, Output []uint16, p16 *cmsInterpParams) {
	TotalOut := int(p16.nOutputs)

	// Ensure p.Table is of type []uint16
	LutTable, ok := p16.Table.([]uint16)
	if !ok {
		panic("p.Table is not of type []uint16 in Eval7Inputs")

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

	// Ensure K0 and K1 do not exceed LUT bounds
	if K0 >= len(LutTable) || K1 >= len(LutTable) {
		panic("LUT index out of range in Eval7Inputs")

	}

	// Temporary storage for interpolation results
	// ---- scratch buffers (replace local Tmp1/Tmp2) ----
	sc := mm.Scratch()
	Tmp1 := sc.Tmp1U16[:TotalOut]
	Tmp2 := sc.Tmp2U16[:TotalOut]

	// Create a new interpolation parameter structure
	p1 := *p16
	copy(p1.Domain[:6], p16.Domain[1:7])
	// Use a CHILD FRAME for the nested  calls
	mmInner := mm.NewFrame()
	defer mmInner.Close()

	// Process K0
	p1.Table = LutTable[K0:] // Adjust LUT slice for K0
	Eval6Inputs(mmInner, Input[1:], Tmp1[:], &p1)

	// Process K1
	p1.Table = LutTable[K1:] // Adjust LUT slice for K1
	Eval6Inputs(mmInner, Input[1:], Tmp2[:], &p1)

	// Final interpolation
	for i := 0; i < TotalOut; i++ {
		Output[i] = LinearInterp(int32(rk), int32(Tmp1[i]), int32(Tmp2[i]))
	}
}

// Eval7InputsFloat evaluates a 7-input LUT with `[]float32` table.
func Eval7InputsFloat(mm mem.Manager, Input []float32, Output []float32, p *cmsInterpParams) {
	TotalOut := int(p.nOutputs)

	// Ensure p.Table is of type []float32
	LutTable, ok := p.Table.([]float32)
	if !ok {
		panic("p.Table is not of type []float32 in Eval7InputsFloat")

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

	// Ensure K0 and K1 do not exceed LUT bounds
	if K0 >= len(LutTable) || K1 >= len(LutTable) {
		panic("LUT index out of range in Eval7InputsFloat")
	}

	// Temporary storage for interpolation results
	// ---- scratch buffers (replace local Tmp1/Tmp2) ----
	sc := mm.Scratch()
	Tmp1 := sc.Tmp1F32[:TotalOut]
	Tmp2 := sc.Tmp2F32[:TotalOut]
	// Create a modified interpolation parameter structure
	p1 := *p
	copy(p1.Domain[:6], p.Domain[1:7])
	// Use a CHILD FRAME for the nested  calls
	mmInner := mm.NewFrame()
	defer mmInner.Close()

	// Process K0
	p1.Table = LutTable[K0:] // Adjust LUT slice for K0
	Eval6InputsFloat(mmInner, Input[1:], Tmp1[:], &p1)

	// Process K1
	p1.Table = LutTable[K1:] // Adjust LUT slice for K1
	Eval6InputsFloat(mmInner, Input[1:], Tmp2[:], &p1)

	// Final interpolation
	for i := 0; i < TotalOut; i++ {
		Output[i] = Tmp1[i] + (Tmp2[i]-Tmp1[i])*rest
	}
}

// Eval8Inputs evaluates an 8-input LUT with `[]uint16` table.
func Eval8Inputs(mm mem.Manager, Input []uint16, Output []uint16, p16 *cmsInterpParams) {
	TotalOut := int(p16.nOutputs)

	// Ensure p.Table is of type []uint16
	LutTable, ok := p16.Table.([]uint16)
	if !ok {
		panic("p.Table is not of type []uint16 in Eval8Inputs")

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

	// Ensure K0 and K1 do not exceed LUT bounds
	if K0 >= len(LutTable) || K1 >= len(LutTable) {
		panic("LUT index out of range in Eval8Inputs")
	}

	// Temporary storage for interpolation results
	// ---- scratch buffers (replace local Tmp1/Tmp2) ----
	sc := mm.Scratch()
	Tmp1 := sc.Tmp1U16[:TotalOut]
	Tmp2 := sc.Tmp2U16[:TotalOut]

	// Create a new interpolation parameter structure
	p1 := *p16
	copy(p1.Domain[:7], p16.Domain[1:8])
	// Use a CHILD FRAME for the nested  calls
	mmInner := mm.NewFrame()
	defer mmInner.Close()

	// Process K0
	p1.Table = LutTable[K0:] // Adjust LUT slice for K0
	Eval7Inputs(mmInner, Input[1:], Tmp1[:], &p1)

	// Process K1
	p1.Table = LutTable[K1:] // Adjust LUT slice for K1
	Eval7Inputs(mmInner, Input[1:], Tmp2[:], &p1)

	// Final interpolation
	for i := 0; i < TotalOut; i++ {
		Output[i] = LinearInterp(int32(rk), int32(Tmp1[i]), int32(Tmp2[i]))
	}
}

// Eval8InputsFloat evaluates an 8-input LUT with `[]float32` table.
func Eval8InputsFloat(mm mem.Manager, Input []float32, Output []float32, p *cmsInterpParams) {
	TotalOut := int(p.nOutputs)

	// Ensure p.Table is of type []float32
	LutTable, ok := p.Table.([]float32)
	if !ok {
		panic("p.Table is not of type []float32 in Eval8InputsFloat")

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

	// Ensure K0 and K1 do not exceed LUT bounds
	if K0 >= len(LutTable) || K1 >= len(LutTable) {
		panic("LUT index out of range in Eval8InputsFloat")

	}

	// Temporary storage for interpolation results
	// ---- scratch buffers (replace local Tmp1/Tmp2) ----
	sc := mm.Scratch()
	Tmp1 := sc.Tmp1F32[:TotalOut]
	Tmp2 := sc.Tmp2F32[:TotalOut]

	// Create a modified interpolation parameter structure
	p1 := *p
	copy(p1.Domain[:7], p.Domain[1:8])
	// Use a CHILD FRAME for the nested  calls
	mmInner := mm.NewFrame()
	defer mmInner.Close()

	// Process K0
	p1.Table = LutTable[K0:] // Adjust LUT slice for K0
	Eval7InputsFloat(mmInner, Input[1:], Tmp1[:], &p1)

	// Process K1
	p1.Table = LutTable[K1:] // Adjust LUT slice for K1
	Eval7InputsFloat(mmInner, Input[1:], Tmp2[:], &p1)

	// Final interpolation
	for i := 0; i < TotalOut; i++ {
		Output[i] = Tmp1[i] + (Tmp2[i]-Tmp1[i])*rest
	}
}

// Eval9Inputs evaluates a 9-input LUT with `[]uint16` table.
func Eval9Inputs(mm mem.Manager, Input []uint16, Output []uint16, p16 *cmsInterpParams) {
	TotalOut := int(p16.nOutputs)

	// Ensure p.Table is of type []uint16
	LutTable, ok := p16.Table.([]uint16)
	if !ok {
		panic("p.Table is not of type []uint16 in Eval9Inputs")

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

	// Ensure K0 and K1 do not exceed LUT bounds
	if K0 >= len(LutTable) || K1 >= len(LutTable) {
		panic("LUT index out of range in Eval9Inputs")

	}

	// Temporary storage for interpolation results
	// ---- scratch buffers (replace local Tmp1/Tmp2) ----
	sc := mm.Scratch()
	Tmp1 := sc.Tmp1U16[:TotalOut]
	Tmp2 := sc.Tmp2U16[:TotalOut]

	// Create a new interpolation parameter structure
	p1 := *p16
	copy(p1.Domain[:8], p16.Domain[1:9])
	// Use a CHILD FRAME for the nested  calls
	mmInner := mm.NewFrame()
	defer mmInner.Close()

	// Process K0
	p1.Table = LutTable[K0:] // Adjust LUT slice for K0
	Eval8Inputs(mmInner, Input[1:], Tmp1[:], &p1)

	// Process K1
	p1.Table = LutTable[K1:] // Adjust LUT slice for K1
	Eval8Inputs(mmInner, Input[1:], Tmp2[:], &p1)

	// Final interpolation
	for i := 0; i < TotalOut; i++ {
		Output[i] = LinearInterp(int32(rk), int32(Tmp1[i]), int32(Tmp2[i]))
	}
}

// Eval9InputsFloat evaluates a 9-input LUT with `[]float32` table.
func Eval9InputsFloat(mm mem.Manager, Input []float32, Output []float32, p *cmsInterpParams) {
	TotalOut := int(p.nOutputs)

	// Ensure p.Table is of type []float32
	LutTable, ok := p.Table.([]float32)
	if !ok {
		panic("p.Table is not of type []float32 in Eval9InputsFloat")

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

	// Ensure K0 and K1 do not exceed LUT bounds
	if K0 >= len(LutTable) || K1 >= len(LutTable) {
		panic("LUT index out of range in Eval9InputsFloat")

	}

	// Temporary storage for interpolation results
	// ---- scratch buffers (replace local Tmp1/Tmp2) ----
	sc := mm.Scratch()
	Tmp1 := sc.Tmp1F32[:TotalOut]
	Tmp2 := sc.Tmp2F32[:TotalOut]

	// Create a modified interpolation parameter structure
	p1 := *p
	copy(p1.Domain[:8], p.Domain[1:9])
	// Use a CHILD FRAME for the nested  calls
	mmInner := mm.NewFrame()
	defer mmInner.Close()

	// Process K0
	p1.Table = LutTable[K0:] // Adjust LUT slice for K0
	Eval8InputsFloat(mmInner, Input[1:], Tmp1[:], &p1)

	// Process K1
	p1.Table = LutTable[K1:] // Adjust LUT slice for K1
	Eval8InputsFloat(mmInner, Input[1:], Tmp2[:], &p1)

	// Final interpolation
	for i := 0; i < TotalOut; i++ {
		Output[i] = Tmp1[i] + (Tmp2[i]-Tmp1[i])*rest
	}
}

// Eval10Inputs evaluates a 10-input LUT with `[]uint16` table.
func Eval10Inputs(mm mem.Manager, Input []uint16, Output []uint16, p16 *cmsInterpParams) {
	TotalOut := int(p16.nOutputs)

	// Ensure p.Table is of type []uint16
	LutTable, ok := p16.Table.([]uint16)
	if !ok {
		panic("p.Table is not of type []uint16 in Eval10Inputs")
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

	// Ensure K0 and K1 do not exceed LUT bounds
	if K0 >= len(LutTable) || K1 >= len(LutTable) {
		panic("LUT index out of range in Eval10Inputs")

	}

	// Temporary storage for interpolation results
	// ---- scratch buffers (replace local Tmp1/Tmp2) ----
	sc := mm.Scratch()
	Tmp1 := sc.Tmp1U16[:TotalOut]
	Tmp2 := sc.Tmp2U16[:TotalOut]

	// Create a new interpolation parameter structure
	p1 := *p16
	copy(p1.Domain[:9], p16.Domain[1:10])
	// Use a CHILD FRAME for the nested  calls
	mmInner := mm.NewFrame()
	defer mmInner.Close()

	// Process K0
	p1.Table = LutTable[K0:] // Adjust LUT slice for K0
	Eval9Inputs(mmInner, Input[1:], Tmp1[:], &p1)

	// Process K1
	p1.Table = LutTable[K1:] // Adjust LUT slice for K1
	Eval9Inputs(mmInner, Input[1:], Tmp2[:], &p1)

	// Final interpolation
	for i := 0; i < TotalOut; i++ {
		Output[i] = LinearInterp(int32(rk), int32(Tmp1[i]), int32(Tmp2[i]))
	}
}

// Eval10InputsFloat evaluates a 10-input LUT with `[]float32` table.
func Eval10InputsFloat(mm mem.Manager, Input []float32, Output []float32, p *cmsInterpParams) {
	TotalOut := int(p.nOutputs)

	// Ensure p.Table is of type []float32
	LutTable, ok := p.Table.([]float32)
	if !ok {
		panic("p.Table is not of type []float32 in Eval10InputsFloat")

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

	// Ensure K0 and K1 do not exceed LUT bounds
	if K0 >= len(LutTable) || K1 >= len(LutTable) {
		panic("LUT index out of range in Eval10InputsFloat")

	}

	// Temporary storage for interpolation results
	// ---- scratch buffers (replace local Tmp1/Tmp2) ----
	sc := mm.Scratch()
	Tmp1 := sc.Tmp1F32[:TotalOut]
	Tmp2 := sc.Tmp2F32[:TotalOut]

	// Create a modified interpolation parameter structure
	p1 := *p
	copy(p1.Domain[:9], p.Domain[1:10])
	// Use a CHILD FRAME for the nested  calls
	mmInner := mm.NewFrame()
	defer mmInner.Close()

	// Process K0
	p1.Table = LutTable[K0:] // Adjust LUT slice for K0
	Eval9InputsFloat(mmInner, Input[1:], Tmp1[:], &p1)

	// Process K1
	p1.Table = LutTable[K1:] // Adjust LUT slice for K1
	Eval9InputsFloat(mmInner, Input[1:], Tmp2[:], &p1)

	// Final interpolation
	for i := 0; i < TotalOut; i++ {
		Output[i] = Tmp1[i] + (Tmp2[i]-Tmp1[i])*rest
	}
}

// Eval11Inputs evaluates an 11-input LUT with `[]uint16` table.
func Eval11Inputs(mm mem.Manager, Input []uint16, Output []uint16, p16 *cmsInterpParams) {
	TotalOut := int(p16.nOutputs)

	// Ensure p.Table is of type []uint16
	LutTable, ok := p16.Table.([]uint16)
	if !ok {
		panic("p.Table is not of type []uint16 in Eval11Inputs")

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

	// Ensure K0 and K1 do not exceed LUT bounds
	if K0 >= len(LutTable) || K1 >= len(LutTable) {
		panic("LUT index out of range in Eval11Inputs")

	}

	// Temporary storage for interpolation results
	// ---- scratch buffers (replace local Tmp1/Tmp2) ----
	sc := mm.Scratch()
	Tmp1 := sc.Tmp1U16[:TotalOut]
	Tmp2 := sc.Tmp2U16[:TotalOut]

	// Create a new interpolation parameter structure
	p1 := *p16
	copy(p1.Domain[:10], p16.Domain[1:11])
	// Use a CHILD FRAME for the nested  calls
	mmInner := mm.NewFrame()
	defer mmInner.Close()

	// Process K0
	p1.Table = LutTable[K0:] // Adjust LUT slice for K0
	Eval10Inputs(mmInner, Input[1:], Tmp1[:], &p1)

	// Process K1
	p1.Table = LutTable[K1:] // Adjust LUT slice for K1
	Eval10Inputs(mmInner, Input[1:], Tmp2[:], &p1)

	// Final interpolation
	for i := 0; i < TotalOut; i++ {
		Output[i] = LinearInterp(int32(rk), int32(Tmp1[i]), int32(Tmp2[i]))
	}
}

// Eval11InputsFloat evaluates an 11-input LUT with `[]float32` table.
func Eval11InputsFloat(mm mem.Manager, Input []float32, Output []float32, p *cmsInterpParams) {
	TotalOut := int(p.nOutputs)

	// Ensure p.Table is of type []float32
	LutTable, ok := p.Table.([]float32)
	if !ok {
		panic("p.Table is not of type []float32 in Eval11InputsFloat")

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

	// Ensure K0 and K1 do not exceed LUT bounds
	if K0 >= len(LutTable) || K1 >= len(LutTable) {
		panic("LUT index out of range in Eval11InputsFloat")

	}

	// Temporary storage for interpolation results
	// ---- scratch buffers (replace local Tmp1/Tmp2) ----
	sc := mm.Scratch()
	Tmp1 := sc.Tmp1F32[:TotalOut]
	Tmp2 := sc.Tmp2F32[:TotalOut]

	// Create a new interpolation parameter structure
	p1 := *p
	copy(p1.Domain[:10], p.Domain[1:11])
	// Use a CHILD FRAME for the nested  calls
	mmInner := mm.NewFrame()
	defer mmInner.Close()

	// Process K0
	p1.Table = LutTable[K0:] // Adjust LUT slice for K0
	Eval10InputsFloat(mmInner, Input[1:], Tmp1[:], &p1)

	// Process K1
	p1.Table = LutTable[K1:] // Adjust LUT slice for K1
	Eval10InputsFloat(mmInner, Input[1:], Tmp2[:], &p1)

	// Final interpolation
	for i := 0; i < TotalOut; i++ {
		Output[i] = Tmp1[i] + (Tmp2[i]-Tmp1[i])*rest
	}
}

// Eval12Inputs evaluates a 12-input LUT with `[]uint16` table.
func Eval12Inputs(mm mem.Manager, Input []uint16, Output []uint16, p16 *cmsInterpParams) {
	TotalOut := int(p16.nOutputs)

	// Ensure p.Table is of type []uint16
	LutTable, ok := p16.Table.([]uint16)
	if !ok {
		panic("p.Table is not of type []uint16 in Eval12Inputs")

	}

	// Convert input to fixed-point representation
	fk := cmsToFixedDomain(int(Input[0]) * int(p16.Domain[0]))
	k0 := FIXED_TO_INT(fk)
	rk := FIXED_REST_TO_INT(fk)

	// Compute LUT table indices
	K0 := int(p16.opta[11]) * int(k0)
	K1 := K0
	if Input[0] != 0xFFFF {
		K1 += int(p16.opta[11])
	}

	// Ensure K0 and K1 do not exceed LUT bounds
	if K0 >= len(LutTable) || K1 >= len(LutTable) {
		panic("LUT index out of range in Eval12Inputs")

	}

	// Temporary storage for interpolation results
	// ---- scratch buffers (replace local Tmp1/Tmp2) ----
	sc := mm.Scratch()
	Tmp1 := sc.Tmp1U16[:TotalOut]
	Tmp2 := sc.Tmp2U16[:TotalOut]

	// Create a new interpolation parameter structure
	p1 := *p16
	copy(p1.Domain[:11], p16.Domain[1:12])
	// Use a CHILD FRAME for the nested  calls
	mmInner := mm.NewFrame()
	defer mmInner.Close()

	// Process K0
	p1.Table = LutTable[K0:] // Adjust LUT slice for K0
	Eval11Inputs(mmInner, Input[1:], Tmp1[:], &p1)

	// Process K1
	p1.Table = LutTable[K1:] // Adjust LUT slice for K1
	Eval11Inputs(mmInner, Input[1:], Tmp2[:], &p1)

	// Final interpolation
	for i := 0; i < TotalOut; i++ {
		Output[i] = LinearInterp(int32(rk), int32(Tmp1[i]), int32(Tmp2[i]))
	}
}

// Eval12InputsFloat evaluates a 12-input LUT with `[]float32` table.
func Eval12InputsFloat(mm mem.Manager, Input []float32, Output []float32, p *cmsInterpParams) {
	TotalOut := int(p.nOutputs)

	// Ensure p.Table is of type []float32
	LutTable, ok := p.Table.([]float32)
	if !ok {
		panic("p.Table is not of type []float32 in Eval12InputsFloat")
	}

	// Convert input to normalized floating-point representation
	pk := fclamp(Input[0]) * float32(p.Domain[0])
	k0 := int(math.Floor(float64(pk)))
	rest := pk - float32(k0)

	// Compute LUT table indices
	K0 := int(p.opta[11]) * k0
	K1 := K0
	if Input[0] < 1.0 {
		K1 += int(p.opta[11])
	}

	// Ensure K0 and K1 do not exceed LUT bounds
	if K0 >= len(LutTable) || K1 >= len(LutTable) {
		panic("LUT index out of range in Eval12InputsFloat")

	}

	// Temporary storage for interpolation results
	// ---- scratch buffers (replace local Tmp1/Tmp2) ----
	sc := mm.Scratch()
	Tmp1 := sc.Tmp1F32[:TotalOut]
	Tmp2 := sc.Tmp2F32[:TotalOut]

	// Create a new interpolation parameter structure
	p1 := *p
	copy(p1.Domain[:11], p.Domain[1:12])
	// Use a CHILD FRAME for the nested  calls
	mmInner := mm.NewFrame()
	defer mmInner.Close()

	// Process K0
	p1.Table = LutTable[K0:] // Adjust LUT slice for K0
	Eval11InputsFloat(mmInner, Input[1:], Tmp1[:], &p1)

	// Process K1
	p1.Table = LutTable[K1:] // Adjust LUT slice for K1
	Eval11InputsFloat(mmInner, Input[1:], Tmp2[:], &p1)

	// Final interpolation
	for i := 0; i < TotalOut; i++ {
		Output[i] = Tmp1[i] + (Tmp2[i]-Tmp1[i])*rest
	}
}

// Eval13Inputs evaluates a 13-input LUT with `[]uint16` table.
func Eval13Inputs(mm mem.Manager, Input []uint16, Output []uint16, p16 *cmsInterpParams) {
	TotalOut := int(p16.nOutputs)

	// Ensure p.Table is of type []uint16
	LutTable, ok := p16.Table.([]uint16)
	if !ok {
		panic("p.Table is not of type []uint16 in Eval13Inputs")
	}

	// Convert input to fixed-point representation
	fk := cmsToFixedDomain(int(Input[0]) * int(p16.Domain[0]))
	k0 := FIXED_TO_INT(fk)
	rk := FIXED_REST_TO_INT(fk)

	// Compute LUT table indices
	K0 := int(p16.opta[12]) * int(k0)
	K1 := K0
	if Input[0] != 0xFFFF {
		K1 += int(p16.opta[12])
	}

	// Ensure K0 and K1 do not exceed LUT bounds
	if K0 >= len(LutTable) || K1 >= len(LutTable) {
		panic("LUT index out of range in Eval13Inputs")

	}

	// Temporary storage for interpolation results
	// ---- scratch buffers (replace local Tmp1/Tmp2) ----
	sc := mm.Scratch()
	Tmp1 := sc.Tmp1U16[:TotalOut]
	Tmp2 := sc.Tmp2U16[:TotalOut]

	// Create a new interpolation parameter structure
	p1 := *p16
	copy(p1.Domain[:12], p16.Domain[1:13])
	// Use a CHILD FRAME for the nested  calls
	mmInner := mm.NewFrame()
	defer mmInner.Close()

	// Process K0
	p1.Table = LutTable[K0:] // Adjust LUT slice for K0
	Eval12Inputs(mmInner, Input[1:], Tmp1[:], &p1)

	// Process K1
	p1.Table = LutTable[K1:] // Adjust LUT slice for K1
	Eval12Inputs(mmInner, Input[1:], Tmp2[:], &p1)

	// Final interpolation
	for i := 0; i < TotalOut; i++ {
		Output[i] = LinearInterp(int32(rk), int32(Tmp1[i]), int32(Tmp2[i]))
	}
}

// Eval13InputsFloat evaluates a 13-input LUT with `[]float32` table.
func Eval13InputsFloat(mm mem.Manager, Input []float32, Output []float32, p *cmsInterpParams) {
	TotalOut := int(p.nOutputs)

	// Ensure p.Table is of type []float32
	LutTable, ok := p.Table.([]float32)
	if !ok {
		panic("p.Table is not of type []float32 in Eval13InputsFloat")

	}

	// Convert input to normalized floating-point representation
	pk := fclamp(Input[0]) * float32(p.Domain[0])
	k0 := int(math.Floor(float64(pk)))
	rest := pk - float32(k0)

	// Compute LUT table indices
	K0 := int(p.opta[12]) * k0
	K1 := K0
	if Input[0] < 1.0 {
		K1 += int(p.opta[12])
	}

	// Ensure K0 and K1 do not exceed LUT bounds
	if K0 >= len(LutTable) || K1 >= len(LutTable) {
		panic("LUT index out of range in Eval13InputsFloat")

	}

	// Temporary storage for interpolation results
	// ---- scratch buffers (replace local Tmp1/Tmp2) ----
	sc := mm.Scratch()
	Tmp1 := sc.Tmp1F32[:TotalOut]
	Tmp2 := sc.Tmp2F32[:TotalOut]

	// Create a new interpolation parameter structure
	p1 := *p
	copy(p1.Domain[:12], p.Domain[1:13])
	// Use a CHILD FRAME for the nested  calls
	mmInner := mm.NewFrame()
	defer mmInner.Close()

	// Process K0
	p1.Table = LutTable[K0:] // Adjust LUT slice for K0
	Eval12InputsFloat(mmInner, Input[1:], Tmp1[:], &p1)

	// Process K1
	p1.Table = LutTable[K1:] // Adjust LUT slice for K1
	Eval12InputsFloat(mmInner, Input[1:], Tmp2[:], &p1)

	// Final interpolation
	for i := 0; i < TotalOut; i++ {
		Output[i] = Tmp1[i] + (Tmp2[i]-Tmp1[i])*rest
	}
}

// Eval14Inputs evaluates a 14-input LUT with `[]uint16` table.
func Eval14Inputs(mm mem.Manager, Input []uint16, Output []uint16, p16 *cmsInterpParams) {
	TotalOut := int(p16.nOutputs)

	// Ensure p.Table is of type []uint16
	LutTable, ok := p16.Table.([]uint16)
	if !ok {
		panic("p.Table is not of type []uint16 in Eval14Inputs")
	}

	// Convert input to fixed-point representation
	fk := cmsToFixedDomain(int(Input[0]) * int(p16.Domain[0]))
	k0 := FIXED_TO_INT(fk)
	rk := FIXED_REST_TO_INT(fk)

	// Compute LUT table indices
	K0 := int(p16.opta[13]) * int(k0)
	K1 := K0
	if Input[0] != 0xFFFF {
		K1 += int(p16.opta[13])
	}

	// Ensure K0 and K1 do not exceed LUT bounds
	if K0 >= len(LutTable) || K1 >= len(LutTable) {
		panic("LUT index out of range in Eval14Inputs")

	}

	// Temporary storage for interpolation results
	// ---- scratch buffers (replace local Tmp1/Tmp2) ----
	sc := mm.Scratch()
	Tmp1 := sc.Tmp1U16[:TotalOut]
	Tmp2 := sc.Tmp2U16[:TotalOut]

	// Create a new interpolation parameter structure
	p1 := *p16
	copy(p1.Domain[:13], p16.Domain[1:14])
	// Use a CHILD FRAME for the nested  calls
	mmInner := mm.NewFrame()
	defer mmInner.Close()

	// Process K0
	p1.Table = LutTable[K0:] // Adjust LUT slice for K0
	Eval13Inputs(mmInner, Input[1:], Tmp1[:], &p1)

	// Process K1
	p1.Table = LutTable[K1:] // Adjust LUT slice for K1
	Eval13Inputs(mmInner, Input[1:], Tmp2[:], &p1)

	// Final interpolation
	for i := 0; i < TotalOut; i++ {
		Output[i] = LinearInterp(int32(rk), int32(Tmp1[i]), int32(Tmp2[i]))
	}
}

// Eval14InputsFloat evaluates a 14-input LUT with `[]float32` table.
func Eval14InputsFloat(mm mem.Manager, Input []float32, Output []float32, p *cmsInterpParams) {
	TotalOut := int(p.nOutputs)

	// Ensure p.Table is of type []float32
	LutTable, ok := p.Table.([]float32)
	if !ok {
		panic("p.Table is not of type []float32 in Eval14InputsFloat")
	}

	// Convert input to normalized floating-point representation
	pk := fclamp(Input[0]) * float32(p.Domain[0])
	k0 := int(math.Floor(float64(pk)))
	rest := pk - float32(k0)

	// Compute LUT table indices
	K0 := int(p.opta[13]) * k0
	K1 := K0
	if Input[0] < 1.0 {
		K1 += int(p.opta[13])
	}

	// Ensure K0 and K1 do not exceed LUT bounds
	if K0 >= len(LutTable) || K1 >= len(LutTable) {
		panic("LUT index out of range in Eval14InputsFloat")

	}

	// Temporary storage for interpolation results
	// ---- scratch buffers (replace local Tmp1/Tmp2) ----
	sc := mm.Scratch()
	Tmp1 := sc.Tmp1F32[:TotalOut]
	Tmp2 := sc.Tmp2F32[:TotalOut]

	// Create a new interpolation parameter structure
	p1 := *p
	copy(p1.Domain[:13], p.Domain[1:14])
	// Use a CHILD FRAME for the nested  calls
	mmInner := mm.NewFrame()
	defer mmInner.Close()

	// Process K0
	p1.Table = LutTable[K0:] // Adjust LUT slice for K0
	Eval13InputsFloat(mmInner, Input[1:], Tmp1[:], &p1)

	// Process K1
	p1.Table = LutTable[K1:] // Adjust LUT slice for K1
	Eval13InputsFloat(mmInner, Input[1:], Tmp2[:], &p1)

	// Final interpolation
	for i := 0; i < TotalOut; i++ {
		Output[i] = Tmp1[i] + (Tmp2[i]-Tmp1[i])*rest
	}
}

// Eval15Inputs evaluates a 15-input LUT with `[]uint16` table.
func Eval15Inputs(mm mem.Manager, Input []uint16, Output []uint16, p16 *cmsInterpParams) {
	TotalOut := int(p16.nOutputs)

	// Ensure p.Table is of type []uint16
	LutTable, ok := p16.Table.([]uint16)
	if !ok {
		panic("p.Table is not of type []uint16 in Eval15Inputs")
	}

	// Convert input to fixed-point representation
	fk := cmsToFixedDomain(int(Input[0]) * int(p16.Domain[0]))
	k0 := FIXED_TO_INT(fk)
	rk := FIXED_REST_TO_INT(fk)

	// Compute LUT table indices
	K0 := int(p16.opta[14]) * int(k0)
	K1 := K0
	if Input[0] != 0xFFFF {
		K1 += int(p16.opta[14])
	}

	// Ensure K0 and K1 do not exceed LUT bounds
	if K0 >= len(LutTable) || K1 >= len(LutTable) {
		panic("LUT index out of range in Eval15Inputs")

	}

	// Temporary storage for interpolation results
	// ---- scratch buffers (replace local Tmp1/Tmp2) ----
	sc := mm.Scratch()
	Tmp1 := sc.Tmp1U16[:TotalOut]
	Tmp2 := sc.Tmp2U16[:TotalOut]

	// Create a new interpolation parameter structure
	p1 := *p16
	copy(p1.Domain[:14], p16.Domain[1:15])
	// Use a CHILD FRAME for the nested  calls
	mmInner := mm.NewFrame()
	defer mmInner.Close()

	// Process K0
	p1.Table = LutTable[K0:] // Adjust LUT slice for K0
	Eval14Inputs(mmInner, Input[1:], Tmp1[:], &p1)

	// Process K1
	p1.Table = LutTable[K1:] // Adjust LUT slice for K1
	Eval14Inputs(mmInner, Input[1:], Tmp2[:], &p1)

	// Final interpolation
	for i := 0; i < TotalOut; i++ {
		Output[i] = LinearInterp(int32(rk), int32(Tmp1[i]), int32(Tmp2[i]))
	}
}

// Eval15InputsFloat evaluates a 15-input LUT with `[]float32` table.
func Eval15InputsFloat(mm mem.Manager, Input []float32, Output []float32, p *cmsInterpParams) {
	TotalOut := int(p.nOutputs)

	// Ensure p.Table is of type []float32
	LutTable, ok := p.Table.([]float32)
	if !ok {
		panic("p.Table is not of type []float32 in Eval15InputsFloat")
	}

	// Convert input to normalized floating-point representation
	pk := fclamp(Input[0]) * float32(p.Domain[0])
	k0 := int(math.Floor(float64(pk)))
	rest := pk - float32(k0)

	// Compute LUT table indices
	K0 := int(p.opta[14]) * k0
	K1 := K0
	if Input[0] < 1.0 {
		K1 += int(p.opta[14])
	}

	// Ensure K0 and K1 do not exceed LUT bounds
	if K0 >= len(LutTable) || K1 >= len(LutTable) {
		panic("LUT index out of range in Eval15InputsFloat")

	}

	// Temporary storage for interpolation results
	// ---- scratch buffers (replace local Tmp1/Tmp2) ----
	sc := mm.Scratch()
	Tmp1 := sc.Tmp1F32[:TotalOut]
	Tmp2 := sc.Tmp2F32[:TotalOut]

	// Create a new interpolation parameter structure
	p1 := *p
	copy(p1.Domain[:14], p.Domain[1:15])
	// Use a CHILD FRAME for the nested  calls
	mmInner := mm.NewFrame()
	defer mmInner.Close()

	// Process K0
	p1.Table = LutTable[K0:] // Adjust LUT slice for K0
	Eval14InputsFloat(mmInner, Input[1:], Tmp1[:], &p1)

	// Process K1
	p1.Table = LutTable[K1:] // Adjust LUT slice for K1
	Eval14InputsFloat(mmInner, Input[1:], Tmp2[:], &p1)

	// Final interpolation
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
				Interpolation.Lerp16Scalar = LinLerp1DScalar16 // NEW
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
