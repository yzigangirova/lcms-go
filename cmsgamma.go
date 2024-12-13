package golcms

import (
	"unsafe"
)

// cmsParametricCurvesCollection represents the list of supported parametric curves.
type cmsParametricCurvesCollection struct {
	NFunctions     uint32                           // Number of supported functions in this chunk
	FunctionTypes  [MAX_TYPES_IN_LCMS_PLUGIN]int32  // The identification types
	ParameterCount [MAX_TYPES_IN_LCMS_PLUGIN]uint32 // Number of parameters for each function
	Evaluator      cmsParametricCurveEvaluator      // The evaluator
	Next           *cmsParametricCurvesCollection   // Next in list
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
	fl.nFunctions = Plugin.nFunctions

	// Ensure the number of functions does not exceed the maximum allowed.
	if fl.nFunctions > MAX_TYPES_IN_LCMS_PLUGIN {
		fl.nFunctions = MAX_TYPES_IN_LCMS_PLUGIN
	}

	// Copy function types and parameter counts.
	memmove(
		unsafe.Pointer(&fl.FunctionTypes[0]),
		unsafe.Pointer(&Plugin.FunctionTypes[0]),
		uintptr(fl.nFunctions)*unsafe.Sizeof(fl.FunctionTypes[0]),
	)
	memmove(
		unsafe.Pointer(&fl.ParameterCount[0]),
		unsafe.Pointer(&Plugin.ParameterCount[0]),
		uintptr(fl.nFunctions)*unsafe.Sizeof(fl.ParameterCount[0]),
	)

	// Update the linked list.
	fl.Next = ctx.ParametricCurves
	ctx.ParametricCurves = fl

	// All is ok.
	return true
}

// Parametric curves
//
// Parameters goes as: Curve, a, b, c, d, e, f
// Type is the ICC type +1
// if type is negative, then the curve is analytically inverted

// cmsBuildParametricToneCurve builds a parametric tone curve.
func cmsBuildParametricToneCurve(ContextID interface{}, Type int32, Params []float64) *cmsToneCurve {
	var Seg0 cmsCurveSegment
	var Pos int

	// Retrieve the parametric curve collection.
	c := GetParametricCurveByType(ContextID, Type, &Pos)
	if c == nil {
		return nil
	}

	// Initialize the curve segment.
	Seg0.X0 = float32(-1e22) // Equivalent to MINUS_INF
	Seg0.X1 = float32(1e22)  // Equivalent to PLUS_INF
	Seg0.Type = Type

	// Copy the parameters.
	size := c.ParameterCount[Pos]
	if size > uint32(len(Seg0.Params)) {
		return nil
	}
	copy(Seg0.Params[:size], Params[:size])                                                                                                                              

	// Build and return the segmented tone curve.
	return cmsBuildSegmentedToneCurve(ContextID, 1, &Seg0)
}

// Free all memory taken by the gamma curve
func cmsFreeToneCurve(Curve *cmsToneCurve) {
	if Curve == nil {
		return
	}

	ContextID := Curve.InterpParams.ContextID

	cmsFreeInterpParams(Curve.InterpParams)

	if Curve.Table16 != nil {
		cmsFree(ContextID, Curve.Table16)
	}

	if Curve.Segments != nil {
		for i := uint32(0); i < Curve.NSegments; i++ {
			if Curve.Segments[i].SampledPoints != nil {
				cmsFree(ContextID, Curve.Segments[i].SampledPoints)
			}

			if Curve.SegInterp[i] != nil {
				cmsFreeInterpParams(Curve.SegInterp[i])
			}
		}

		cmsFree(ContextID, Curve.Segments)
		cmsFree(ContextID, Curve.SegInterp)
	}

	if Curve.Evals != nil {
		cmsFree(ContextID, Curve.Evals)
	}

	cmsFree(ContextID, Curve)
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

	return AllocateToneCurveStruct(In.InterpParams.ContextID, In.NEntries, In.NSegments, In.Segments, In.Table16)
}

// Join two tone curves.
// Produces y = Y^-1(X(t)).
func cmsJoinToneCurve(ContextID interface{}, X, Y *cmsToneCurve, nResultingPoints uint32) (*cmsToneCurve, error) {
	if X == nil || Y == nil {
		return nil, errors.New("input curves cannot be nil")
	}

	var (
		out        *cmsToneCurve
		Yreversed  *cmsToneCurve
		Res        []float32
		err        error
	)

	// Reverse the Y tone curve
	Yreversed, err = cmsReverseToneCurveEx(nResultingPoints, Y)
	if err != nil {
		return nil, err
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
	out = cmsBuildTabulatedToneCurveFloat(ContextID, nResultingPoints, Res)

	// Cleanup
	cmsFreeToneCurve(Yreversed)

	return out, nil
}

