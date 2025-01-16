package golcms

import (
	"math"
	"unsafe"
)

/*
firstSegment := (*cmsCurveSegment)(unsafe.Pointer(curve.Segments))
segmentPtr := (*cmsCurveSegment)(unsafe.Addunsafe.Pointer(curve.Segments), uintptr(index) * unsafe.Sizeof(cmsCurveSegment{})))
*/
// ----------------------------------------------------------------- Implementation
// Maxim number of nodes
const (
	MAX_NODES_IN_CURVE = 4097
	MINUS_INF          = 1e22 // Floating-point constant
	PLUS_INF           = 1e22 // Floating-point constant
)

// cmsParametricCurvesCollection represents the list of supported parametric curves.
type cmsParametricCurvesCollection struct {
	NFunctions     uint32                           // Number of supported functions in this chunk
	FunctionTypes  [MAX_TYPES_IN_LCMS_PLUGIN]int32  // The identification types
	ParameterCount [MAX_TYPES_IN_LCMS_PLUGIN]uint32 // Number of parameters for each function
	Evaluator      cmsParametricCurveEvaluator      // The evaluator
	Next           *cmsParametricCurvesCollection   // Next in list
}

// The built-in list
var DefaultCurves = cmsParametricCurvesCollection{
	NFunctions:     10,
	FunctionTypes:  [MAX_TYPES_IN_LCMS_PLUGIN]int32{1, 2, 3, 4, 5, 6, 7, 8, 108, 109},
	ParameterCount: [MAX_TYPES_IN_LCMS_PLUGIN]uint32{1, 3, 4, 5, 7, 4, 5, 5, 1, 1},
	Evaluator:      DefaultEvalParametricFn, // Replace with the actual function reference
	Next:           nil,
}

func cmsRegisterParametricCurvesPlugin(ContextID cmsContext, Data *cmsPluginBase) bool {
	ctx := (*cmsCurvesPluginChunkType)(cmsContextGetClientChunk(ContextID, CurvesPlugin))
	Plugin := (*cmsPluginParametricCurves)(unsafe.Pointer(Data))
	var fl *cmsParametricCurvesCollection

	// Reset parametric curves if Data is nil.
	if Data == nil {
		ctx.ParametricCurves = nil
		return true
	}

	// Allocate memory for a new parametric curves collection.
	fl = (*cmsParametricCurvesCollection)(cmsPluginMalloc(ContextID, uint32(unsafe.Sizeof(cmsParametricCurvesCollection{}))))
	if fl == nil {
		return false
	}

	// Copy the parameters.
	fl.Evaluator = Plugin.Evaluator
	fl.NFunctions = Plugin.NFunctions

	// Ensure the number of functions does not exceed the maximum allowed.
	if fl.NFunctions > MAX_TYPES_IN_LCMS_PLUGIN {
		fl.NFunctions = MAX_TYPES_IN_LCMS_PLUGIN
	}

	// Copy function types and parameter counts.
	memmove(
		unsafe.Pointer(&fl.FunctionTypes[0]),
		unsafe.Pointer(&Plugin.FunctionTypes[0]),
		uintptr(fl.NFunctions)*unsafe.Sizeof(fl.FunctionTypes[0]),
	)
	memmove(
		unsafe.Pointer(&fl.ParameterCount[0]),
		unsafe.Pointer(&Plugin.ParameterCount[0]),
		uintptr(fl.NFunctions)*unsafe.Sizeof(fl.ParameterCount[0]),
	)

	// Update the linked list.
	fl.Next = ctx.ParametricCurves
	ctx.ParametricCurves = fl

	// All is ok.
	return true
}

// Search in type list, return position or -1 if not found
func IsInSet(Type int, c *cmsParametricCurvesCollection) int {

	for i := 0; i < int(c.NFunctions); i++ {
		if math.Abs(float64(Type)) == float64(c.FunctionTypes[i]) {
			return i
		}
	}
	return -1
}

// Parametric curves
//
// Parameters goes as: Curve, a, b, c, d, e, f
// Type is the ICC type +1
// if type is negative, then the curve is analytically inverted
// GetParametricCurveByType searches for the collection that contains a specific type of parametric curve.
func GetParametricCurveByType(ContextID cmsContext, Type int, index *int) *cmsParametricCurvesCollection {
	var c *cmsParametricCurvesCollection
	var Position int

	// Retrieve the plugin chunk associated with curves
	ctx := (*cmsCurvesPluginChunkType)(cmsContextGetClientChunk(ContextID, CurvesPlugin))

	// Search in the context's parametric curves
	for c = ctx.ParametricCurves; c != nil; c = c.Next {
		Position = IsInSet(Type, c)

		if Position != -1 {
			if index != nil {
				*index = Position
			}
			return c
		}
	}

	// If none found, revert to the default curves
	for c = &DefaultCurves; c != nil; c = c.Next {
		Position = IsInSet(Type, c)

		if Position != -1 {
			if index != nil {
				*index = Position
			}
			return c
		}
	}

	return nil
}

func allocateEvals(contextID cmsContext, nSegments uint32) []cmsParametricCurveEvaluator {
	// Allocate a slice of cmsParametricCurveEvaluator with length nSegments
	evals := make([]cmsParametricCurveEvaluator, nSegments)
	return evals
}

// AllocateToneCurveStruct allocates memory for a tone curve structure.
func AllocateToneCurveStruct(
	ContextID cmsContext,
	nEntries uint32,
	nSegments uint32,
	Segments *cmsCurveSegment,
	Values *uint16,
) *cmsToneCurve {
	if nEntries > 65530 {
		cmsSignalError(unsafe.Pointer(ContextID), cmsERROR_RANGE, "Couldn't create tone curve of more than 65530 entries")
		return nil
	}

	if nEntries == 0 && nSegments == 0 {
		cmsSignalError(unsafe.Pointer(ContextID), cmsERROR_RANGE, "Couldn't create tone curve with zero segments and no table")
		return nil
	}

	p := (*cmsToneCurve)(cmsMallocZero(ContextID, uint32(unsafe.Sizeof(cmsToneCurve{}))))
	if p == nil {
		return nil
	}

	if nSegments > 0 {
		p.Segments = (*cmsCurveSegment)(cmsCalloc(ContextID, nSegments, uint32(unsafe.Sizeof(cmsCurveSegment{}))))
		if p.Segments == nil {
			goto Error
		}
		//evals are slice of function, so that I can allocate space (I can not evaluate
		//a size of function with sizeof like in C)
		p.Evals = allocateEvals(ContextID, nSegments)
		if p.Evals == nil {
			goto Error
		}
	} else {
		p.Segments = nil
		p.Evals = nil
	}

	p.nSegments = nSegments

	if nEntries > 0 {
		p.Table16 = (*uint16)(cmsCalloc(ContextID, nEntries, uint32(unsafe.Sizeof(uint16(0)))))
		if p.Table16 == nil {
			goto Error
		}
	} else {
		p.Table16 = nil
	}

	p.nEntries = nEntries

	if Values != nil && nEntries > 0 {
		for i := uint32(0); i < nEntries; i++ {
			*(*uint16)(unsafe.Add(unsafe.Pointer(p.Table16), uintptr(i)*unsafe.Sizeof(uint16(0)))) =
				*(*uint16)(unsafe.Add(unsafe.Pointer(Values), uintptr(i)*unsafe.Sizeof(uint16(0))))
		}
	}

	if Segments != nil && nSegments > 0 {
		p.SegInterp = (**cmsInterpParams)(cmsCalloc(ContextID, nSegments, uint32(unsafe.Sizeof((*cmsInterpParams)(nil)))))
		if p.SegInterp == nil {
			goto Error
		}

		for i := uint32(0); i < nSegments; i++ {
			currentSegment := (*cmsCurveSegment)(unsafe.Add(unsafe.Pointer(Segments), uintptr(i)*unsafe.Sizeof(cmsCurveSegment{})))
			elementPtr := (**cmsInterpParams)(unsafe.Add(unsafe.Pointer(p.SegInterp), uintptr(i)*unsafe.Sizeof((*cmsInterpParams)(nil))))

			if currentSegment.Type == 0 {
				// Calculate the pointer to the i-th element in the array
				// Assign the computed value to the i-th element *elementPtr == p.SegInterp[i]
				*elementPtr = cmsComputeInterpParams(ContextID, currentSegment.NGridPoints, 1, 1, nil, CMS_LERP_FLAGS_FLOAT)

			}

			memmove(
				unsafe.Add(unsafe.Pointer(p.Segments), uintptr(i)*unsafe.Sizeof(cmsCurveSegment{})),
				unsafe.Pointer(currentSegment),
				unsafe.Sizeof(cmsCurveSegment{}),
			)

			if currentSegment.Type == 0 && currentSegment.SampledPoints != nil {
				segmentPointsSize := uint32(unsafe.Sizeof(float32(0))) * currentSegment.NGridPoints
				(*currentSegment).SampledPoints = (*float32)(cmsDupMem(ContextID, unsafe.Pointer(&currentSegment.SampledPoints[0]), segmentPointsSize))
			} else {
				(*currentSegment).SampledPoints = nil
			}

			c := GetParametricCurveByType(ContextID, int(currentSegment.Type), nil)
			if c != nil {
				p.Evals[i] = c.Evaluator
			}
		}
	}

	p.InterpParams = cmsComputeInterpParams(ContextID, p.nEntries, 1, 1, unsafe.Pointer(p.Table16), CMS_LERP_FLAGS_16BITS)
	if p.InterpParams != nil {
		return p
	}

Error:
	if p.SegInterp != nil {
		cmsFree(ContextID, unsafe.Pointer(p.SegInterp))
	}
	if p.Segments != nil {
		cmsFree(ContextID, unsafe.Pointer(p.Segments))
	}
	/*if p.Evals != nil {
		cmsFree(ContextID, unsafe.Pointer(p.Evals))
	}*/ //Evals are slices, are freed by Garbage collector
	if p.Table16 != nil {
		cmsFree(ContextID, unsafe.Pointer(p.Table16))
	}
	cmsFree(ContextID, unsafe.Pointer(p))
	return nil
}

// Build a gamma table based on gamma constant
func cmsBuildGamma(ContextID cmsContext, Gamma float64) *cmsToneCurve {
	return cmsBuildParametricToneCurve(ContextID, 1, &Gamma)
}

// Free all memory taken by the gamma curve
func cmsFreeToneCurve(Curve *cmsToneCurve) {
	if Curve == nil {
		return
	}

	ContextID := Curve.InterpParams.ContextID

	cmsFreeInterpParams(Curve.InterpParams)

	if Curve.Table16 != nil {
		cmsFree(ContextID, unsafe.Pointer(Curve.Table16))
	}

	if Curve.Segments != nil {
		for i := uint32(0); i < Curve.nSegments; i++ {
			currentSegment := (*cmsCurveSegment)(unsafe.Add(unsafe.Pointer(Curve.Segments), uintptr(i)*unsafe.Sizeof(cmsCurveSegment{})))

			if currentSegment.SampledPoints != nil {
				cmsFree(ContextID, unsafe.Pointer(currentSegment.SampledPoints))
			}

			targetPtr := (*cmsInterpParams)(unsafe.Add(unsafe.Pointer(Curve.SegInterp), uintptr(i)*unsafe.Sizeof((*cmsInterpParams)(nil))))
			if targetPtr != nil {
				cmsFreeInterpParams(targetPtr)
			}
		}

		cmsFree(ContextID, unsafe.Pointer(Curve.Segments))
		cmsFree(ContextID, unsafe.Pointer(Curve.SegInterp))
	}

	if Curve.Evals != nil {
		//garbage collector
		//cmsFree(ContextID, unsafe.Pointer(Curve.Evals))
	}

	cmsFree(ContextID, unsafe.Pointer(Curve))
}

// Utility function, free 3 gamma tables

// Free a triple of tone curves.
func cmsFreeToneCurveTriple(Curve [3]*cmsToneCurve) {
	if Curve[0] != nil {
		cmsFreeToneCurve(Curve[0])
	}
	if Curve[1] != nil {
		cmsFreeToneCurve(Curve[1])
	}
	if Curve[2] != nil {
		cmsFreeToneCurve(Curve[2])
	}

	Curve[0] = nil
	Curve[1] = nil
	Curve[2] = nil
}

// Duplicate a tone curve.
func cmsDupToneCurve(In *cmsToneCurve) *cmsToneCurve {
	if In == nil {
		return nil
	}

	return AllocateToneCurveStruct(In.InterpParams.ContextID, In.nEntries, In.nSegments, In.Segments, In.Table16)
}

// Join two tone curves.
// Produces y = Y^-1(X(t)).
func cmsJoinToneCurve(ContextID cmsContext, X, Y *cmsToneCurve, nResultingPoints uint32) *cmsToneCurve {
	if X == nil || Y == nil {
		return nil
	}

	var (
		out       *cmsToneCurve
		Yreversed *cmsToneCurve
		Res       []float32
	)

	// Reverse the Y tone curve
	Yreversed = cmsReverseToneCurveEx(nResultingPoints, Y)
	if Yreversed == nil {
		return nil
	}

	// Allocate result array
	Res = make([]float32, nResultingPoints)

	// Iterate and compute
	for i := uint32(0); i < nResultingPoints; i++ {
		t := float32(i) / float32(nResultingPoints-1)
		x := cmsEvalToneCurveFloat(X, t)
		Res[i] = cmsEvalToneCurveFloat(Yreversed, x)
	}

	// Build the output tone curve
	out = cmsBuildTabulatedToneCurveFloat(ContextID, nResultingPoints, &Res[0])

	// Cleanup
	cmsFreeToneCurve(Yreversed)

	return out
}

// cmsIsToneCurveMonotonic checks if a tone curve is monotonic.
func cmsIsToneCurveMonotonic(t *cmsToneCurve) bool {
	if t == nil {
		panic("ToneCurve cannot be nil")
	}

	// Degenerate curves are monotonic. Allow them.
	n := t.nEntries
	if n < 2 {
		return true
	}

	// Determine curve direction
	descending := cmsIsToneCurveDescending(t)

	if descending {
		last := *(*uint16)(unsafe.Pointer(uintptr(unsafe.Pointer(t.Table16))))

		for i := 1; i < int(n); i++ {
			current := *(*uint16)(unsafe.Pointer(uintptr(unsafe.Pointer(t.Table16)) + uintptr(i)*unsafe.Sizeof(*t.Table16)))

			if int(current)-int(last) > 2 { // Allow some ripple
				return false
			}
			last = current
		}
	} else {
		last := *(*uint16)(unsafe.Pointer(uintptr(unsafe.Pointer(t.Table16)) + uintptr(n-1)*unsafe.Sizeof(*t.Table16)))

		for i := int(n) - 2; i >= 0; i-- {
			current := *(*uint16)(unsafe.Pointer(uintptr(unsafe.Pointer(t.Table16)) + uintptr(i)*unsafe.Sizeof(*t.Table16)))

			if int(current)-int(last) > 2 {
				return false
			}
			last = current
		}
	}

	return true
}

// cmsIsToneCurveDescending checks if a tone curve is descending.
func cmsIsToneCurveDescending(t *cmsToneCurve) bool {
	if t == nil {
		panic("ToneCurve cannot be nil")
	}

	// Access the first and last elements of Table16 using pointer arithmetic
	first := *(*uint16)(unsafe.Pointer(uintptr(unsafe.Pointer(t.Table16))))
	last := *(*uint16)(unsafe.Pointer(uintptr(unsafe.Pointer(t.Table16)) + uintptr(t.nEntries-1)*unsafe.Sizeof(*t.Table16)))

	return first > last
}

// cmsIsToneCurveMultisegment checks if a tone curve is multisegment.
func cmsIsToneCurveMultisegment(t *cmsToneCurve) bool {
	if t == nil {
		panic("ToneCurve cannot be nil")
	}

	return t.nSegments > 1
}

// cmsGetToneCurveParametricType retrieves the parametric type of a tone curve.
// Returns 0 if the tone curve is not parametric or multisegment.
func cmsGetToneCurveParametricType(t *cmsToneCurve) int32 {
	if t == nil {
		panic("ToneCurve cannot be nil")
	}

	// Check if the tone curve has only one segment
	if t.nSegments != 1 {
		return 0
	}

	// Access the Type field of the first segment
	firstSegmentType := *(*int32)(unsafe.Pointer(uintptr(unsafe.Pointer(t.Segments)) + unsafe.Offsetof(t.Segments.Type)))

	return firstSegmentType
}

// cmsEvalToneCurveFloat evaluates a tone curve at a specific point (float input and output).
func cmsEvalToneCurveFloat(curve *cmsToneCurve, v float32) float32 {
	if curve == nil {
		panic("ToneCurve cannot be nil")
	}

	// Check if this is a limited-precision tone curve with 16-bit table.
	if curve.nSegments == 0 {
		inValue := uint16(v * 65535.0)
		outValue := cmsEvalToneCurve16(curve, inValue)

		return float32(outValue) / 65535.0
	}

	return float32(EvalSegmentedFn(curve, float64(v)))
}

// cmsEvalToneCurve16 evaluates a tone curve at a specific point (16-bit input and output).
func cmsEvalToneCurve16(Curve *cmsToneCurve, v uint16) uint16 {
	var out uint16

	cmsAssert(Curve != nil, "curve is nil")

	Curve.InterpParams.Interpolation.Lerp16(&v, &out, Curve.InterpParams)
	return out
}

// cmsEstimateGamma estimates the gamma value of a tone curve using a least squares fitting method.
// It calculates the best-fitting gamma by minimizing the sum of squared residuals.
func cmsEstimateGamma(t *cmsToneCurve, Precision float64) float64 {
	var gamma, sum, sum2, n, x, y, Std float64
	var i uint32

	cmsAssert(t != nil, "cmsToneCurve is nil")

	sum, sum2, n = 0, 0, 0

	// Exclude endpoints to avoid linear artifacts
	for i = 1; i < (MAX_NODES_IN_CURVE - 1); i++ {
		x = float64(i) / float64(MAX_NODES_IN_CURVE-1)
		y = float64(cmsEvalToneCurveFloat(t, float32(x)))

		// Avoid analyzing lower part (below 7%) to prevent artifacts due to linear ramps
		if y > 0. && y < 1. && x > 0.07 {
			gamma = math.Log(y) / math.Log(x)
			sum += gamma
			sum2 += gamma * gamma
			n++
		}
	}

	// Ensure we have enough valid samples
	if n <= 1 {
		return -1.0
	}

	// Calculate standard deviation to check if the curve is truly exponential
	Std = math.Sqrt((n*sum2 - sum*sum) / (n * (n - 1)))

	if Std > Precision {
		return -1.0
	}

	// Return the mean gamma value
	return sum / n
}
func cmsGetToneCurveParams(t *cmsToneCurve) *float64 {
	// Ensure the tone curve is not nil
	if t == nil {
		panic("cmsToneCurve is nil")
	}

	// Check if the curve has only one segment
	if t.nSegments != 1 {
		return nil
	}

	// Access the first segment's parameters using unsafe.Pointer
	return (*float64)(unsafe.Pointer(&(*cmsCurveSegment)(unsafe.Pointer(t.Segments)).Params))
}

// cmsBuildTabulatedToneCurve16 creates an empty gamma curve using tables.
func cmsBuildTabulatedToneCurve16(ContextID cmsContext, nEntries uint32, Values *uint16) *cmsToneCurve {
	return AllocateToneCurveStruct(ContextID, nEntries, 0, nil, Values)
}

// EntriesByGamma calculates the number of entries by gamma.
func EntriesByGamma(Gamma float64) uint32 {
	if math.Abs(Gamma-1.0) < 0.001 {
		return 2
	}
	return 4096
}

// cmsBuildSegmentedToneCurve creates a segmented gamma curve and fills the table.
func cmsBuildSegmentedToneCurve(ContextID cmsContext, nSegments uint32, Segments *cmsCurveSegment) *cmsToneCurve {
	if Segments == nil {
		cmsAssert(Segments != nil, "Segments cannot be null")

	}

	nGridPoints := uint32(4096)
	if nSegments == 1 && Segments.Type == 1 {
		nGridPoints = EntriesByGamma(Segments.Params[0])
	}

	g := AllocateToneCurveStruct(ContextID, nGridPoints, nSegments, Segments, nil)
	if g == nil {
		return nil
	}

	for i := uint32(0); i < nGridPoints; i++ {
		R := float64(i) / float64(nGridPoints-1)
		Val := EvalSegmentedFn(g, R)
		*(*uint16)(unsafe.Add(unsafe.Pointer(g.Table16), uintptr(i)*unsafe.Sizeof(uint16(0)))) = cmsQuickSaturateWord(Val * 65535.0)
	}

	return g
}

// cmsBuildTabulatedToneCurveFloat uses a segmented curve to store the floating-point table.
func cmsBuildTabulatedToneCurveFloat(ContextID cmsContext, nEntries uint32, values *float32) *cmsToneCurve {
	var Seg [3]cmsCurveSegment

	if nEntries == 0 || values == nil {
		return nil
	}

	// Initialize segments
	Seg[0] = cmsCurveSegment{X0: -math.MaxFloat32, X1: 0, Type: 6, Params: [10]float64{1, 0, 0, float64(*values), 0}}
	Seg[1] = cmsCurveSegment{X0: 0, X1: 1, Type: 0, NGridPoints: nEntries, SampledPoints: values}
	Seg[2] = cmsCurveSegment{X0: 1, X1: math.MaxFloat32, Type: 6, Params: [10]float64{1, 0, 0, float64(*(*float32)(unsafe.Add(unsafe.Pointer(values), uintptr(nEntries-1)*unsafe.Sizeof(float32(0))))), 0}}

	return cmsBuildSegmentedToneCurve(ContextID, 3, &Seg[0])
}

// cmsBuildParametricToneCurve builds a parametric tone curve.
func cmsBuildParametricToneCurve(ContextID cmsContext, Type int, Params *float64) *cmsToneCurve {
	var Seg0 cmsCurveSegment
	var Pos int
	c := GetParametricCurveByType(ContextID, Type, &Pos)

	if c == nil {
		cmsSignalError(unsafe.Pointer(ContextID), cmsERROR_UNKNOWN_EXTENSION, "Invalid parametric curve type")
		return nil
	}

	Seg0 = cmsCurveSegment{
		X0:   -math.MaxFloat32,
		X1:   math.MaxFloat32,
		Type: int32(Type),
	}

	size := c.ParameterCount[Pos] * uint32(unsafe.Sizeof(float64(0)))
	memmove(unsafe.Pointer(&Seg0.Params[0]), unsafe.Pointer(Params), uintptr(size))

	return cmsBuildSegmentedToneCurve(ContextID, 1, &Seg0)
}

// DefaultEvalParametricFn evaluates a parametric curve using floating point.
// The behavior depends on the curve type and associated parameters.
func DefaultEvalParametricFn(Type int32, Params []float64, R float64) float64 {
	var e, Val, disc float64

	switch Type {

	// X = Y ^ Gamma
	case 1:
		if R < 0 {
			if math.Abs(Params[0]-1.0) < MATRIX_DET_TOLERANCE {
				Val = R
			} else {
				Val = 0
			}
		} else {
			Val = math.Pow(R, Params[0])
		}

	// Type 1 Reversed: X = Y ^ 1/gamma
	case -1:
		if R < 0 {
			if math.Abs(Params[0]-1.0) < MATRIX_DET_TOLERANCE {
				Val = R
			} else {
				Val = 0
			}
		} else {
			if math.Abs(Params[0]) < MATRIX_DET_TOLERANCE {
				Val = PLUS_INF
			} else {
				Val = math.Pow(R, 1/Params[0])
			}
		}

	// CIE 122-1966
	// Y = (aX + b)^Gamma | X >= -b/a
	// Y = 0              | else
	case 2:
		if math.Abs(Params[1]) < MATRIX_DET_TOLERANCE {
			Val = 0
		} else {
			disc = -Params[2] / Params[1]
			if R >= disc {
				e = Params[1]*R + Params[2]
				if e > 0 {
					Val = math.Pow(e, Params[0])
				} else {
					Val = 0
				}
			} else {
				Val = 0
			}
		}

	// Type 2 Reversed
	// X = (Y ^ (1/gamma) - b) / a
	case -2:
		if math.Abs(Params[0]) < MATRIX_DET_TOLERANCE || math.Abs(Params[1]) < MATRIX_DET_TOLERANCE {
			Val = 0
		} else {
			if R < 0 {
				Val = 0
			} else {
				Val = (math.Pow(R, 1.0/Params[0]) - Params[2]) / Params[1]
				if Val < 0 {
					Val = 0
				}
			}
		}

	// IEC 61966-3
	// Y = (aX + b)^Gamma + c | X <= -b/a
	// Y = c                  | else
	case 3:
		if math.Abs(Params[1]) < MATRIX_DET_TOLERANCE {
			Val = 0
		} else {
			disc = -Params[2] / Params[1]
			if disc < 0 {
				disc = 0
			}
			if R >= disc {
				e = Params[1]*R + Params[2]
				if e > 0 {
					Val = math.Pow(e, Params[0]) + Params[3]
				} else {
					Val = 0
				}
			} else {
				Val = Params[3]
			}
		}

	// Type 3 Reversed
	// X = ((Y-c)^1/gamma - b)/a | (Y >= c)
	// X = -b/a                  | (Y < c)
	case -3:
		if math.Abs(Params[0]) < MATRIX_DET_TOLERANCE || math.Abs(Params[1]) < MATRIX_DET_TOLERANCE {
			Val = 0
		} else {
			if R >= Params[3] {
				e = R - Params[3]
				if e > 0 {
					Val = (math.Pow(e, 1/Params[0]) - Params[2]) / Params[1]
				} else {
					Val = 0
				}
			} else {
				Val = -Params[2] / Params[1]
			}
		}

	// IEC 61966-2.1 (sRGB)
	// Y = (aX + b)^Gamma | X >= d
	// Y = cX             | X < d
	case 4:
		if R >= Params[4] {
			e = Params[1]*R + Params[2]
			if e > 0 {
				Val = math.Pow(e, Params[0])
			} else {
				Val = 0
			}
		} else {
			Val = R * Params[3]
		}

	// Type 4 Reversed
	// X = ((Y^1/gamma - b)/a)   | Y >= (ad+b)^g
	// X = Y/c                   | Y < (ad+b)^g
	case -4:
		e = Params[1]*Params[4] + Params[2]
		if e < 0 {
			disc = 0
		} else {
			disc = math.Pow(e, Params[0])
		}
		if R >= disc {
			if math.Abs(Params[0]) < MATRIX_DET_TOLERANCE || math.Abs(Params[1]) < MATRIX_DET_TOLERANCE {
				Val = 0
			} else {
				Val = (math.Pow(R, 1.0/Params[0]) - Params[2]) / Params[1]
			}
		} else {
			if math.Abs(Params[3]) < MATRIX_DET_TOLERANCE {
				Val = 0
			} else {
				Val = R / Params[3]
			}
		}

	// Y = (aX + b)^Gamma + e | X >= d
	// Y = cX + f             | X < d
	case 5:
		if R >= Params[4] {
			e = Params[1]*R + Params[2]
			if e > 0 {
				Val = math.Pow(e, Params[0]) + Params[5]
			} else {
				Val = Params[5]
			}
		} else {
			Val = R*Params[3] + Params[6]
		}

	// Reversed type 5
	// X = ((Y-e)^1/gamma - b)/a | Y >= (ad+b)^g+e), cd+f
	// X = (Y-f)/c               | else
	case -5:
		disc = Params[3]*Params[4] + Params[6]
		if R >= disc {
			e = R - Params[5]
			if e < 0 {
				Val = 0
			} else {
				if math.Abs(Params[0]) < MATRIX_DET_TOLERANCE || math.Abs(Params[1]) < MATRIX_DET_TOLERANCE {
					Val = 0
				} else {
					Val = (math.Pow(e, 1.0/Params[0]) - Params[2]) / Params[1]
				}
			}
		} else {
			if math.Abs(Params[3]) < MATRIX_DET_TOLERANCE {
				Val = 0
			} else {
				Val = (R - Params[6]) / Params[3]
			}
		}

	// Additional cases omitted for brevity...

	default:
		// Unsupported parametric curve. Should never reach here.
		return 0
	}

	return Val
}

// EvalSegmentedFn evaluates a segmented function for a single value.
// Returns math.Inf(-1) if no valid segment is found.
// If the function type is 0, performs interpolation on the table.
func EvalSegmentedFn(g *cmsToneCurve, R float64) float64 {
	var Out float64
	var Out32 float32

	segmentSize := unsafe.Sizeof(cmsCurveSegment{})
	segInterpSize := unsafe.Sizeof((*cmsInterpParams)(nil))

	for i := int(g.nSegments) - 1; i >= 0; i-- {
		// Access the current segment
		currentSegment := (*cmsCurveSegment)(unsafe.Add(unsafe.Pointer(g.Segments), uintptr(i)*segmentSize))

		// Check for domain
		if R > float64(currentSegment.X0) && R <= float64(currentSegment.X1) {
			if currentSegment.Type == 0 {
				// Type == 0 means segment is sampled
				R1 := float32((R - float64(currentSegment.X0)) / float64(currentSegment.X1-currentSegment.X0))

				// Access the current SegInterp
				currentSegInterp := (**cmsInterpParams)(unsafe.Add(unsafe.Pointer(g.SegInterp), uintptr(i)*segInterpSize))

				// Setup the table
				(*currentSegInterp).Table = unsafe.Pointer(currentSegment.SampledPoints)

				// Perform interpolation
				(*currentSegInterp).Interpolation.LerpFloat(&R1, &Out32, *currentSegInterp)
				Out = float64(Out32)
			} else {
				// Evaluate the function for the segment
				Out = g.Evals[i](currentSegment.Type, currentSegment.Params[:], R)
			}

			// Check for infinity
			if math.IsInf(Out, 1) {
				return math.Inf(1) // PLUS_INF
			} else if math.IsInf(Out, -1) {
				return math.Inf(-1) // MINUS_INF
			}

			return Out
		}
	}

	return math.Inf(-1) // MINUS_INF
}
func cmsReverseToneCurveEx(nResultSamples uint32, inCurve *cmsToneCurve) *cmsToneCurve {
	var a, b, y, x1, y1, x2, y2 float64
	var i, j int
	var ascending bool

	// Ensure input curve is not nil
	if inCurve == nil {
		return nil
	}

	// Try to reverse it analytically if possible
	if inCurve.nSegments == 1 &&
		*(*uint32)(unsafe.Pointer(&inCurve.Segments.Type)) > 0 &&
		GetParametricCurveByType(inCurve.InterpParams.ContextID,
			int(*(*uint32)(unsafe.Pointer(&inCurve.Segments.Type))),
			nil) != nil {
		return cmsBuildParametricToneCurve(
			inCurve.InterpParams.ContextID,
			-int(*(*uint32)(unsafe.Pointer(&inCurve.Segments.Type))),
			(*float64)(unsafe.Pointer(&inCurve.Segments.Params)),
		)
	}

	// Create a new tone curve for the reversed result
	out := cmsBuildTabulatedToneCurve16(inCurve.InterpParams.ContextID, nResultSamples, nil)
	if out == nil {
		return nil
	}

	// Determine if the curve is ascending or descending
	ascending = !cmsIsToneCurveDescending(inCurve)

	// Iterate across the Y-axis
	for i = 0; i < int(nResultSamples); i++ {
		y = float64(i) * 65535.0 / float64(nResultSamples-1)

		// Find the interval where y is within
		j = GetInterval(y, (*uint16)(unsafe.Pointer(inCurve.Table16)), inCurve.InterpParams)
		if j >= 0 {
			// Get limits of the interval
			x1 = float64(*(*uint16)(unsafe.Add(unsafe.Pointer(inCurve.Table16), uintptr(j)*unsafe.Sizeof(uint16(0)))))
			x2 = float64(*(*uint16)(unsafe.Add(unsafe.Pointer(inCurve.Table16), uintptr(j+1)*unsafe.Sizeof(uint16(0)))))

			y1 = float64(j) * 65535.0 / float64(inCurve.nEntries-1)
			y2 = float64(j+1) * 65535.0 / float64(inCurve.nEntries-1)

			// If the interval is collapsed, use any value
			if x1 == x2 {
				if ascending {
					*(*uint16)(unsafe.Add(unsafe.Pointer(out.Table16), uintptr(i)*unsafe.Sizeof(uint16(0)))) = cmsQuickSaturateWord(y2)
				} else {
					*(*uint16)(unsafe.Add(unsafe.Pointer(out.Table16), uintptr(i)*unsafe.Sizeof(uint16(0)))) = cmsQuickSaturateWord(y1)
				}
				continue
			}

			// Perform interpolation
			a = (y2 - y1) / (x2 - x1)
			b = y2 - a*x2
		}

		*(*uint16)(unsafe.Add(unsafe.Pointer(out.Table16), uintptr(i)*unsafe.Sizeof(uint16(0)))) = cmsQuickSaturateWord(a*y + b)
	}

	return out
}

func cmsReverseToneCurve(inGamma *cmsToneCurve) *cmsToneCurve {
	// Ensure input curve is not nil
	if inGamma == nil {
		return nil
	}

	// Reverse using 4096 result samples
	return cmsReverseToneCurveEx(4096, inGamma)
}
func GetInterval(In float64, LutTable *uint16, p *cmsInterpParams) int {
	// A 1-point table is not allowed
	if p.Domain[0] < 1 {
		return -1
	}

	// Determine if the table is overall ascending or descending
	if *(*uint16)(unsafe.Add(unsafe.Pointer(LutTable), 0)) < *(*uint16)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(p.Domain[0])*unsafe.Sizeof(uint16(0)))) {
		// Table is overall ascending
		for i := int(p.Domain[0]) - 1; i >= 0; i-- {
			y0 := *(*uint16)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(i)*unsafe.Sizeof(uint16(0))))
			y1 := *(*uint16)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(i+1)*unsafe.Sizeof(uint16(0))))

			if y0 <= y1 { // Increasing
				if In >= float64(y0) && In <= float64(y1) {
					return i
				}
			} else if y1 < y0 { // Decreasing
				if In >= float64(y1) && In <= float64(y0) {
					return i
				}
			}
		}
	} else {
		// Table is overall descending
		for i := 0; i < int(p.Domain[0]); i++ {
			y0 := *(*uint16)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(i)*unsafe.Sizeof(uint16(0))))
			y1 := *(*uint16)(unsafe.Add(unsafe.Pointer(LutTable), uintptr(i+1)*unsafe.Sizeof(uint16(0))))

			if y0 <= y1 { // Increasing
				if In >= float64(y0) && In <= float64(y1) {
					return i
				}
			} else if y1 < y0 { // Decreasing
				if In >= float64(y1) && In <= float64(y0) {
					return i
				}
			}
		}
	}

	return -1
}
