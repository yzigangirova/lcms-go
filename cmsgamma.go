package golcms

// _cmsParametricCurvesCollection represents the list of supported parametric curves.
type cmsParametricCurvesCollection struct {
	NFunctions     cmsUInt32Number                           // Number of supported functions in this chunk
	FunctionTypes  [MAX_TYPES_IN_LCMS_PLUGIN]cmsInt32Number  // The identification types
	ParameterCount [MAX_TYPES_IN_LCMS_PLUGIN]cmsUInt32Number // Number of parameters for each function
	Evaluator      cmsParametricCurveEvaluator               // The evaluator
	Next           *cmsParametricCurvesCollection           // Next in list
}
