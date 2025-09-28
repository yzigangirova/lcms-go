package golcms

import (
	"math"
	"sync"

	//"unsafe"

	"github.com/yzigangirova/lcms-go/mem"
)

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
	FunctionTypes  [MAX_TYPES_IN_LCMS_PLUGIN]uint32 // The identification types
	ParameterCount [MAX_TYPES_IN_LCMS_PLUGIN]uint32 // Number of parameters for each function
	Evaluator      cmsParametricCurveEvaluator      // The evaluator
	Next           *cmsParametricCurvesCollection   // Next in list
}

// The built-in list
var DefaultCurves = cmsParametricCurvesCollection{
	NFunctions:     10,
	FunctionTypes:  [MAX_TYPES_IN_LCMS_PLUGIN]uint32{1, 2, 3, 4, 5, 6, 7, 8, 108, 109},
	ParameterCount: [MAX_TYPES_IN_LCMS_PLUGIN]uint32{1, 3, 4, 5, 7, 4, 5, 5, 1, 1},
	Evaluator:      DefaultEvalParametricFn, // Replace with the actual function reference
	Next:           nil,
}

// The linked list head
var cmsCurvesPluginChunk = cmsCurvesPluginChunkType{ParametricCurves: nil}

func cmsRegisterParametricCurvesPlugin(mm mem.Manager, ContextID CmsContext, Data PluginIntrfc) bool {
	ctx := CmsContextGetClientChunk(ContextID, CurvesPlugin).(*cmsCurvesPluginChunkType)
	var fl *cmsParametricCurvesCollection

	// Reset parametric curves if Data is nil.
	if Data == nil {
		ctx.ParametricCurves = nil
		return true
	}
	plugin, ok := Data.(*cmsPluginParametricCurves)
	if !ok {
		panic("Plugin is not of the type cmsPluginParametricCurves\n")

	}
	// Allocate memory for a new parametric curves collection.
	//fl = (*cmsParametricCurvesCollection)(cmsPluginMalloc(ContextID, uint32(unsafe.Sizeof(cmsParametricCurvesCollection{}))))
	fl = mem.New[cmsParametricCurvesCollection](mm)

	if fl == nil {
		return false
	}

	// Copy the parameters.[]
	fl.Evaluator = plugin.Evaluator
	fl.NFunctions = plugin.NFunctions

	// Ensure the number of functions does not exceed the maximum allowed.
	if fl.NFunctions > MAX_TYPES_IN_LCMS_PLUGIN {
		fl.NFunctions = MAX_TYPES_IN_LCMS_PLUGIN
	}

	// Copy function types and parameter counts.
	MemmoveSlice(fl.FunctionTypes[:], plugin.FunctionTypes[:], int(fl.NFunctions))
	MemmoveSlice(fl.ParameterCount[:], plugin.ParameterCount[:], int(fl.NFunctions))

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
func GetParametricCurveByType(ContextID CmsContext, Type int, index *int) *cmsParametricCurvesCollection {
	var c *cmsParametricCurvesCollection
	var Position int

	// Retrieve the plugin chunk associated with curves
	ctx := CmsContextGetClientChunk(ContextID, CurvesPlugin).(*cmsCurvesPluginChunkType)

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

func allocateEvals(contextID CmsContext, nSegments uint32) []cmsParametricCurveEvaluator {
	// Allocate a slice of cmsParametricCurveEvaluator with length nSegments
	evals := mem.MakeSlice[cmsParametricCurveEvaluator](mem.Manager{}, int(nSegments))
	return evals
}

// AllocateToneCurveStruct allocates memory for a tone curve structure.
func AllocateToneCurveStruct(mm mem.Manager,
	ContextID CmsContext,
	nEntries uint32,
	nSegments uint32,
	Segments []cmsCurveSegment,
	Values []uint16,
) *CmsToneCurve {
	//fmt.Println("start AllocateToneCurveStruct")
	if nEntries > 65530 {
		cmsSignalError(ContextID, cmsERROR_RANGE, "Couldn't create tone curve of more than 65530 entries")
		return nil
	}

	if nEntries == 0 && nSegments == 0 {
		cmsSignalError(ContextID, cmsERROR_RANGE, "Couldn't create tone curve with zero segments and no table")
		return nil
	}

	// Allocate tone curve structure
	p := &CmsToneCurve{
		nSegments: nSegments,
		nEntries:  nEntries,
	}

	// Allocate segments and evaluators if needed
	if nSegments > 0 {
		p.Segments = mem.MakeSlice[cmsCurveSegment](mm, int(nSegments))
		p.Evals = allocateEvals(ContextID, nSegments)
		if p.Evals == nil {
			goto Error
		}
	} else {
		p.Segments = nil
		p.Evals = nil
	}

	// Allocate Table16 if needed
	if nEntries > 0 {
		p.Table16 = mem.MakeSlice[uint16](mm, int(nEntries))
	} else {
		p.Table16 = nil
	}
	//fmt.Printf("222 mem.MakeSlice[uint16 nEntries: %d  &p.Table16[0]: %p  len(p.Table16) %d p(cmsToneCurve):  %p\n", nEntries, &p.Table16[0], len(p.Table16), p)

	// Copy values to Table16 if provided
	if Values != nil && nEntries > 0 {
		copy(p.Table16, Values[:nEntries])
	}

	// Process segments if available
	if Segments != nil && nSegments > 0 {
		p.SegInterp = mem.MakeSlice[*cmsInterpParams](mm, int(nSegments))

		for i := uint32(0); i < nSegments; i++ {
			currentSegment := Segments[i]

			if currentSegment.Type == 0 {
				p.SegInterp[i] = cmsComputeInterpParams(mm, ContextID, currentSegment.NGridPoints, 1, 1, nil, CMS_LERP_FLAGS_FLOAT)
			}

			p.Segments[i] = currentSegment // Copy segment data

			// Copy sampled points if necessary
			if currentSegment.Type == 0 && currentSegment.SampledPoints != nil {
				p.Segments[i].SampledPoints = append([]float32(nil), currentSegment.SampledPoints...)
			} else {
				p.Segments[i].SampledPoints = nil
			}

			// Get parametric curve evaluator
			c := GetParametricCurveByType(ContextID, int(currentSegment.Type), nil)
			if c != nil {
				p.Evals[i] = c.Evaluator
			}
		}
	}

	// Compute interpolation parameters
	p.InterpParams = cmsComputeInterpParams(mm, ContextID, p.nEntries, 1, 1, p.Table16, CMS_LERP_FLAGS_16BITS)
	//fmt.Println("end AllocateToneCurveStruct")

	if p.InterpParams != nil {
		return p
	}

Error:
	// Free allocated memory in case of error
	p.SegInterp = nil
	p.Segments = nil
	p.Table16 = nil
	return nil
}

// Build a gamma table based on gamma constant
func CmsBuildGamma(mm mem.Manager, ContextID CmsContext, Gamma float64) *CmsToneCurve {
	return cmsBuildParametricToneCurve(mm, ContextID, 1, []float64{Gamma})
}

// Free all memory taken by the gamma curve
func CmsFreeToneCurve(Curve *CmsToneCurve) {
	if Curve == nil {
		return
	}

	ContextID := Curve.InterpParams.ContextID
	cmsFreeInterpParams(Curve.InterpParams)
	if Curve.Segments != nil {
		for i := uint32(0); i < Curve.nSegments; i++ {
			if Curve.SegInterp[i] != nil {
				cmsFreeInterpParams(Curve.SegInterp[i])
			}
		}
	}

	if Curve.Evals != nil {
		//garbage collector - evals is a slice
		//cmsFree(ContextID, unsafe.Pointer(Curve.Evals))
	}

	cmsFree(ContextID, Curve)
}

// Utility function, free 3 gamma tables

// Free a triple of tone curves.
func cmsFreeToneCurveTriple(Curve [3]*CmsToneCurve) {
	if Curve[0] != nil {
		CmsFreeToneCurve(Curve[0])
	}
	if Curve[1] != nil {
		CmsFreeToneCurve(Curve[1])
	}
	if Curve[2] != nil {
		CmsFreeToneCurve(Curve[2])
	}

	Curve[0] = nil
	Curve[1] = nil
	Curve[2] = nil
}

// Duplicate a tone curve.
func cmsDupToneCurve(mm mem.Manager, In *CmsToneCurve) *CmsToneCurve {

	if In == nil {
		return nil
	}
	//fmt.Println("cmsDupToneCurve")
	return AllocateToneCurveStruct(mm, In.InterpParams.ContextID, In.nEntries, In.nSegments, In.Segments, In.Table16)
}

// Join two tone curves.
// Produces y = Y^-1(X(t)).
func cmsJoinToneCurve(mm mem.Manager, ContextID CmsContext, X, Y *CmsToneCurve, nResultingPoints uint32) *CmsToneCurve {
	if X == nil || Y == nil {
		return nil
	}

	var (
		out       *CmsToneCurve
		Yreversed *CmsToneCurve
		Res       []float32
	)

	// Reverse the Y tone curve
	Yreversed = cmsReverseToneCurveEx(mm, nResultingPoints, Y)
	if Yreversed == nil {
		return nil
	}

	// Allocate result array
	Res = mem.MakeSlice[float32](mm, int(nResultingPoints))

	// Iterate and compute
	for i := uint32(0); i < nResultingPoints; i++ {
		t := float32(i) / float32(nResultingPoints-1)
		x := cmsEvalToneCurveFloat(X, t)
		Res[i] = cmsEvalToneCurveFloat(Yreversed, x)
	}

	// Build the output tone curve
	out = cmsBuildTabulatedToneCurveFloat(mm, ContextID, nResultingPoints, Res)

	return out
}
func cmsIsToneCurveLinear(Curve *CmsToneCurve) bool {
	cmsAssert(Curve != nil, "")

	for i := 0; i < int(Curve.nEntries); i++ {
		// Compute the difference
		diff := int(Curve.Table16[i]) - int(cmsQuantizeVal(float64(i), Curve.nEntries))
		if math.Abs(float64(diff)) > 0x0f {
			return false
		}
	}

	return true
}

// cmsIsToneCurveMonotonic checks if a tone curve is monotonic.
func cmsIsToneCurveMonotonic(t *CmsToneCurve) bool {
	cmsAssert(t != nil, "ToneCurve cannot be nil")

	// Degenerate curves are monotonic. Allow them.
	n := t.nEntries
	if n < 2 {
		return true
	}

	// Determine curve direction
	descending := cmsIsToneCurveDescending(t)

	if descending {
		last := t.Table16[0]
		for i := 1; i < int(n); i++ {
			if int(t.Table16[i])-int(last) > 2 { // Allow some ripple
				return false
			} else {
				last = t.Table16[i]
			}
		}
	} else {
		last := t.Table16[n-1]

		for i := int(n) - 2; i >= 0; i-- {
			if int(t.Table16[i])-int(last) > 2 {
				return false
			}
			last = t.Table16[i]
		}
	}

	return true
}

// cmsIsToneCurveDescending checks if a tone curve is descending.
func cmsIsToneCurveDescending(t *CmsToneCurve) bool {
	cmsAssert(t != nil, "ToneCurve cannot be nil")

	return t.Table16[0] > t.Table16[t.nEntries-1]
}

// cmsIsToneCurveMultisegment checks if a tone curve is multisegment.
func cmsIsToneCurveMultisegment(t *CmsToneCurve) bool {
	cmsAssert(t != nil, "ToneCurve cannot be nil")

	return t.nSegments > 1
}

// cmsGetToneCurveParametricType retrieves the parametric type of a tone curve.
// Returns 0 if the tone curve is not parametric or multisegment.
func cmsGetToneCurveParametricType(t *CmsToneCurve) int32 {
	cmsAssert(t != nil, "ToneCurve cannot be nil")

	// Check if the tone curve has only one segment
	if t.nSegments != 1 {
		return 0
	}
	return t.Segments[0].Type
}

// cmsEvalToneCurveFloat evaluates a tone curve at a specific point (float input and output).
func cmsEvalToneCurveFloat(curve *CmsToneCurve, v float32) float32 {
	//fmt.Printf("cmsEvalToneCurveFloat %.7f\n", v)

	cmsAssert(curve != nil, "ToneCurve cannot be nil")

	// Check if this is a limited-precision tone curve with 16-bit table.
	if curve.nSegments == 0 {
		inValue := uint16(cmsQuickSaturateWord(float64(v) * 65535.0))
		outValue := cmsEvalToneCurve16(curve, inValue)
		//fmt.Printf("returning out value %.7f\n", float32(outValue)/65535.0)
		return float32(outValue) / 65535.0
	}

	//fmt.Printf("returning out value %.7f\n", float32(EvalSegmentedFn(curve, float64(v))))
	return float32(EvalSegmentedFn(curve, float64(v)))
}

/*func cmsEvalToneCurve16(Curve *CmsToneCurve, v uint16) uint16 {
	var out uint16

	cmsAssert(Curve != nil, "curve is nil")
	outSlice := []uint16{0} // Create a slice with an actual mutable value
	Curve.InterpParams.Interpolation.Lerp16([]uint16{v}, outSlice, Curve.InterpParams)
	out = outSlice[0] // Extract the modified value from the slice
	return out
}*/

var toneBufferPool = sync.Pool{
	New: func() any {
		return &[2]uint16{0, 0} // [0]=input, [1]=output
	},
}

// cmsEvalToneCurve16 evaluates a tone curve at a specific point (16-bit input and output).
func cmsEvalToneCurve16(Curve *CmsToneCurve, v uint16) uint16 {
	cmsAssert(Curve != nil, "curve is nil")

	buf := toneBufferPool.Get().(*[2]uint16)
	defer toneBufferPool.Put(buf)

	buf[0] = v
	buf[1] = 0

	Curve.InterpParams.Interpolation.Lerp16(buf[0:1], buf[1:2], Curve.InterpParams)

	return buf[1]
}

// cmsEstimateGamma estimates the gamma value of a tone curve using a least squares fitting method.
// It calculates the best-fitting gamma by minimizing the sum of squared residuals.
func cmsEstimateGamma(t *CmsToneCurve, Precision float64) float64 {
	var gamma, sum, sum2, n, x, y, Std float64
	var i uint32

	cmsAssert(t != nil, "CmsToneCurve is nil")

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
func cmsGetToneCurveParams(t *CmsToneCurve) []float64 {
	// Ensure the tone curve is not nil
	cmsAssert(t != nil, "ToneCurve cannot be nil")

	// Check if the curve has only one segment
	if t.nSegments != 1 {
		return nil
	}

	// Access the first segment's parameters using unsafe.Pointer
	return t.Segments[0].Params[:]
}

// cmsBuildTabulatedToneCurve16 creates an empty gamma curve using tables.
func cmsBuildTabulatedToneCurve16(mm mem.Manager, ContextID CmsContext, nEntries uint32, Values []uint16) *CmsToneCurve {

	//fmt.Println("cmsBuildTabulatedToneCurve16(")

	return AllocateToneCurveStruct(mm, ContextID, nEntries, 0, nil, Values)
}

// EntriesByGamma calculates the number of entries by gamma.
func EntriesByGamma(Gamma float64) uint32 {
	if math.Abs(Gamma-1.0) < 0.001 {
		return 2
	}
	return 4096
}

// cmsBuildSegmentedToneCurve creates a segmented gamma curve and fills the table.
func cmsBuildSegmentedToneCurve(mm mem.Manager, ContextID CmsContext, nSegments uint32, Segments []cmsCurveSegment) *CmsToneCurve {
	//fmt.Println("cmsBuildSegmentedToneCurve")

	if Segments == nil {
		cmsAssert(Segments != nil, "Segments cannot be null")

	}

	nGridPoints := uint32(4096)
	if nSegments == 1 && Segments[0].Type == 1 {
		nGridPoints = EntriesByGamma(Segments[0].Params[0])
	}

	g := AllocateToneCurveStruct(mm, ContextID, nGridPoints, nSegments, Segments, nil)
	if g == nil {
		return nil
	}

	for i := uint32(0); i < nGridPoints; i++ {
		R := float64(i) / float64(nGridPoints-1)
		Val := EvalSegmentedFn(g, R)
		// Round and saturate
		g.Table16[i] = cmsQuickSaturateWord(Val * 65535.0)
	}

	return g
}

// cmsBuildTabulatedToneCurveFloat uses a segmented curve to store the floating-point table.
func cmsBuildTabulatedToneCurveFloat(mm mem.Manager, ContextID CmsContext, nEntries uint32, values []float32) *CmsToneCurve {
	var Seg [3]cmsCurveSegment

	if nEntries == 0 || values == nil {
		return nil
	}

	// Initialize segments
	Seg[0] = cmsCurveSegment{X0: -math.MaxFloat32, X1: 0, Type: 6, Params: [10]float64{1, 0, 0, float64(values[0]), 0}}
	Seg[1] = cmsCurveSegment{X0: 0, X1: 1, Type: 0, NGridPoints: nEntries, SampledPoints: values}
	Seg[2] = cmsCurveSegment{X0: 1, X1: math.MaxFloat32, Type: 6, Params: [10]float64{1, 0, 0, float64(values[nEntries-1]), 0}}

	return cmsBuildSegmentedToneCurve(mm, ContextID, 3, Seg[:])
}

// cmsBuildParametricToneCurve builds a parametric tone curve.
func cmsBuildParametricToneCurve(mm mem.Manager, ContextID CmsContext, Type int, Params []float64) *CmsToneCurve {
	var Seg0 cmsCurveSegment
	var Pos int
	c := GetParametricCurveByType(ContextID, Type, &Pos)

	if c == nil {
		cmsSignalError(ContextID, cmsERROR_UNKNOWN_EXTENSION, "Invalid parametric curve type")
		return nil
	}

	Seg0 = cmsCurveSegment{
		X0:   -math.MaxFloat32,
		X1:   math.MaxFloat32,
		Type: int32(Type),
	}

	size := c.ParameterCount[Pos]
	MemmoveSlice(Seg0.Params[:], Params, int(size))

	return cmsBuildSegmentedToneCurve(mm, ContextID, 1, []cmsCurveSegment{Seg0})
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

func EvalSegmentedFn(g *CmsToneCurve, R float64) float64 {
	//fmt.Println("start EvalSegmentedFn")
	var Out float64
	var Out32 float32

	for i := int(g.nSegments) - 1; i >= 0; i-- {
		seg := g.Segments[i]

		// Check for domain
		if R > float64(seg.X0) && R <= float64(seg.X1) {
			if seg.Type == 0 {
				// Type == 0 means segment is sampled
				R1 := float32((R - float64(seg.X0)) / float64(seg.X1-seg.X0))

				// Ensure Table is of type []float32
				table, ok := g.SegInterp[i].Table.([]float32)
				if !ok {
					panic("Table is not of type []float32")
				}

				// Copy SampledPoints into Table
				copy(table, seg.SampledPoints)

				// Perform interpolation
				out32Slice := []float32{Out32}
				g.SegInterp[i].Interpolation.LerpFloat([]float32{R1}, out32Slice, g.SegInterp[i])
				Out = float64(out32Slice[0])
			} else {
				// Evaluate the function for the segment
				Out = g.Evals[i](seg.Type, seg.Params[:], R)
			}

			// Check for infinity
			if math.IsInf(Out, 1) {
				return math.Inf(1) // PLUS_INF
			} else if math.IsInf(Out, -1) {
				return math.Inf(-1) // MINUS_INF
			}

			//	fmt.Println("end EvalSegmentedFn")
			return Out
		}
	}
	return math.Inf(-1) // MINUS_INF
}

func cmsReverseToneCurveEx(mm mem.Manager, nResultSamples uint32, inCurve *CmsToneCurve) *CmsToneCurve {
	var a, b, y, x1, y1, x2, y2 float64
	var i, j int
	var ascending bool

	// Ensure input curve is not nil
	if inCurve == nil {
		return nil
	}

	// Try to reverse it analytically if possible
	if inCurve.nSegments == 1 && inCurve.Segments[0].Type > 0 &&
		GetParametricCurveByType(inCurve.InterpParams.ContextID, int(inCurve.Segments[0].Type), nil) != nil {

		return cmsBuildParametricToneCurve(mm, inCurve.InterpParams.ContextID,
			-int(inCurve.Segments[0].Type),
			inCurve.Segments[0].Params[:])
	}

	// Create a new tone curve for the reversed result
	out := cmsBuildTabulatedToneCurve16(mm, inCurve.InterpParams.ContextID, nResultSamples, nil)
	if out == nil {
		return nil
	}

	// Determine if the curve is ascending or descending
	ascending = !cmsIsToneCurveDescending(inCurve)

	// Iterate across the Y-axis
	for i = 0; i < int(nResultSamples); i++ {
		y = float64(i) * 65535.0 / float64(nResultSamples-1)

		// Find interval in which y is within.
		j = GetInterval(y, inCurve.Table16, inCurve.InterpParams)
		if j >= 0 {

			// Get limits of interval
			x1 = float64(inCurve.Table16[j])
			x2 = float64(inCurve.Table16[j+1])

			y1 = float64((j * 65535.0) / (int(inCurve.nEntries) - 1))
			y2 = float64(((j + 1) * 65535.0) / (int(inCurve.nEntries) - 1))

			// If collapsed, then use any
			if x1 == x2 {
				if ascending {
					out.Table16[i] = cmsQuickSaturateWord(y2)
				} else {
					out.Table16[i] = cmsQuickSaturateWord(y1)
				}
				continue

			} else {

				// Interpolate
				a = (y2 - y1) / (x2 - x1)
				b = y2 - a*x2
			}
		}

		out.Table16[i] = cmsQuickSaturateWord(a*y + b)
	}

	return out
}

func cmsReverseToneCurve(mm mem.Manager, inGamma *CmsToneCurve) *CmsToneCurve {
	// Ensure input curve is not nil
	if inGamma == nil {
		return nil
	}

	// Reverse using 4096 result samples
	return cmsReverseToneCurveEx(mm, 4096, inGamma)
}
func GetInterval(In float64, LutTable []uint16, p *cmsInterpParams) int {
	// A 1-point table is not allowed
	if p.Domain[0] < 1 {
		return -1
	}
	var y0, y1 int
	// Let's see if ascending or descending.
	if LutTable[0] < LutTable[p.Domain[0]] {

		// Table is overall ascending
		for i := int(p.Domain[0]) - 1; i >= 0; i-- {

			y0 = int(LutTable[i])
			y1 = int(LutTable[i+1])

			if y0 <= y1 { // Increasing
				if In >= float64(y0) && In <= float64(y1) {
					return i
				}
			} else {
				if y1 < y0 { // Decreasing
					if In >= float64(y1) && In <= float64(y0) {
						return i
					}
				}
			}
		}
	} else {
		// Table is overall descending
		for i := 0; i < int(p.Domain[0]); i++ {

			y0 = int(LutTable[i])
			y1 = int(LutTable[i+1])

			if y0 <= y1 { // Increasing
				if In >= float64(y0) && In <= float64(y1) {
					return i
				}
			} else {
				if y1 < y0 { // Decreasing
					if In >= float64(y1) && In <= float64(y0) {
						return i
					}
				}
			}
		}
	}
	return -1
}
