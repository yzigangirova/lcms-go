package golcms

import("unsafe")
// Constants
const (
	VX = 0
	VY = 1
	VZ = 2
)


// Vectors and Matrices
type cmsVEC3 struct {
	N [3]cmsFloat64Number
}

type cmsMAT3 struct {
	V [3]cmsVEC3
}

// Tag Base
type cmsTagTypeSignature uint32
type cmsTagSignature uint32
type _cmsTagBase struct {
	Sig      cmsTagTypeSignature
	Reserved [4]cmsInt8Number
}


// Vectors and Matrices Operations
func _cmsVEC3Init(r *cmsVEC3, x, y, z cmsFloat64Number) {
	r.N[0] = x
	r.N[1] = y
	r.N[2] = z
}

// Constants for interpolation flags
const (
	CMS_LERP_FLAGS_16BITS    uint32 = 0x0000 // Default
	CMS_LERP_FLAGS_FLOAT     uint32 = 0x0001 // Floating-point implementation required
	CMS_LERP_FLAGS_TRILINEAR uint32 = 0x0100 // Hint for trilinear interpolation
)

// Maximum input dimensions for interpolation
const MAX_INPUT_DIMENSIONS = 15

// _cmsInterpFn16 is a function type for 16-bit interpolation functions.
// Performs precision-limited linear interpolation (e.g., tetrahedral or trilinear).
type cmsInterpFn16 func(input []uint16, output []uint16, params *cmsInterpParams)

// _cmsInterpFnFloat is a function type for floating-point interpolation functions.
// Performs full-precision interpolation (e.g., tetrahedral or trilinear).
type cmsInterpFnFloat func(input []float32, output []float32, params *cmsInterpParams)

// cmsInterpFunction holds either a 16-bit or floating-point interpolation function.
type cmsInterpFunction struct {
	Lerp16   cmsInterpFn16
	LerpFloat cmsInterpFnFloat
}

// cmsInterpParams represents the parameters for interpolation.
type cmsInterpParams struct {
	ContextID  cmsContext       // The calling thread context
	dwFlags    uint32           // Flags for interpolation
	nInputs    uint32           // Number of input channels (3D interpolation if > 1)
	nOutputs   uint32           // Number of output channels (3D interpolation if > 1)
	nSamples   [MAX_INPUT_DIMENSIONS]uint32 // Valid samples for each dimension
	Domain     [MAX_INPUT_DIMENSIONS]uint32 // Domain = nSamples - 1
	opta       [MAX_INPUT_DIMENSIONS]uint32 // Optimization values for 3D CLUT
	Table      unsafe.Pointer   // Pointer to the actual interpolation table
	Interpolation cmsInterpFunction // Interpolation functions
}

// cmsInterpFnFactory is a function type for creating an interpolator.
// It returns an interpolator function (either 16-bit or float).
type cmsInterpFnFactory func(nInputChannels, nOutputChannels, dwFlags uint32) cmsInterpFunction

// cmsPluginBase represents the base structure for plugins.
type cmsPluginBase struct {
	Magic          uint32 // Magic number for validation
	ExpectedVersion uint32 // Expected version of the library
	Type            uint32 // Plugin type
	Next            *cmsPluginBase // Pointer to the next plugin in the chain
}

// cmsPluginInterpolation represents the plugin structure for interpolators.
type cmsPluginInterpolation struct {
	base                 cmsPluginBase
	InterpolatorsFactory cmsInterpFnFactory // Factory function for interpolators
}


// The rest of the vector/matrix operations
// Each follows a similar idiomatic Go implementation as above

// Parametric Curve Evaluator
type cmsParametricCurveEvaluator func(cmsInt32Number, *[10]cmsFloat64Number, cmsFloat64Number) cmsFloat64Number

type cmsPluginParametricCurves struct {
	Base            cmsPluginBase
	NFunctions      cmsUInt32Number
	FunctionTypes   [20]cmsUInt32Number
	ParameterCount  [20]cmsUInt32Number
	Evaluator       cmsParametricCurveEvaluator
}

// Plugin Tag Type
type cmsTagTypeHandler struct {
	Signature   cmsTagTypeSignature
	ReadPtr     func(*cmsTagTypeHandler, cmsHANDLE, *cmsUInt32Number, cmsUInt32Number) any
	WritePtr    func(*cmsTagTypeHandler, cmsHANDLE, any, cmsUInt32Number) cmsBool
	DupPtr      func(*cmsTagTypeHandler, any, cmsUInt32Number) any
	FreePtr     func(*cmsTagTypeHandler, any)
	ContextID   cmsContext
	ICCVersion  cmsUInt32Number
}

type cmsPluginTagType struct {
	Base    cmsPluginBase
	Handler cmsTagTypeHandler
}

// Plugin Formatters
type cmsFormatterDirection int

const (
	cmsFormatterInput cmsFormatterDirection = iota
	cmsFormatterOutput
)

type cmsFormatterFactory func(cmsUInt32Number, cmsFormatterDirection, cmsUInt32Number) cmsFormatter

type cmsPluginFormatters struct {
	Base              cmsPluginBase
	FormattersFactory cmsFormatterFactory
}

// Transform
type cmsStride struct {
	BytesPerLineIn  cmsUInt32Number
	BytesPerLineOut cmsUInt32Number
	BytesPerPlaneIn cmsUInt32Number
	BytesPerPlaneOut cmsUInt32Number
}

// Mutex Plugin
type cmsPluginMutex struct {
	Base          cmsPluginBase
	CreateMutexPtr func(cmsContext) any
	DestroyMutexPtr func(cmsContext, any)
	LockMutexPtr func(cmsContext, any) cmsBool
	UnlockMutexPtr func(cmsContext, any)
}



// Shared callbacks for user data
type cmsFreeUserDataFn func(ContextID cmsContext, Data unsafe.Pointer)
type cmsDupUserDataFn func(ContextID cmsContext, Data unsafe.Pointer) unsafe.Pointer

type cmsTransformFn func(CMMcargo *cmsTRANSFORM, InputBuffer interface{},
	OutputBuffer interface{}, Size cmsUInt32Number, Stride cmsUInt32Number)

type cmsTransform2Fn func(CMMcargo *cmsTRANSFORM, InputBuffer interface{},
	OutputBuffer interface{}, PixelsPerLine cmsUInt32Number, LineCount cmsUInt32Number, Stride *cmsStride)

type cmsTransformFactory func(xform *cmsTransformFn, UserData **interface{},
	FreePrivateDataFn *cmsFreeUserDataFn, Lut **cmsPipeline, InputFormat *cmsUInt32Number, OutputFormat *cmsUInt32Number, dwFlags *cmsUInt32Number) cmsBool

type cmsTransform2Factory func(xform *cmsTransform2Fn, UserData **interface{},
	FreePrivateDataFn *cmsFreeUserDataFn, Lut **cmsPipeline, InputFormat *cmsUInt32Number, OutputFormat *cmsUInt32Number, dwFlags *cmsUInt32Number) cmsBool

type cmsFormatter16 func(CMMcargo *cmsTRANSFORM, Values []cmsUInt16Number, Buffer []cmsUInt8Number, Stride cmsUInt32Number) []cmsUInt8Number

type cmsFormatterFloat func(CMMcargo *cmsTRANSFORM, Values []cmsFloat32Number, Buffer []cmsUInt8Number, Stride cmsUInt32Number) []cmsUInt8Number

type cmsFormatter struct {
	Fmt16    cmsFormatter16
	FmtFloat cmsFormatterFloat
}


//----------------------------------------------------------------------------------------------------------
// Optimization. Using this plug-in, additional optimization strategies may be implemented.
// The function should return TRUE if any optimization is done on the LUT, this terminates
// the optimization  search. Or FALSE if it is unable to optimize and want to give a chance
// to the rest of optimizers.


// _cmsOPToptimizeFn is a function type for optimization strategies.
// Returns true if any optimization is done on the LUT, false otherwise.
type cmsOPToptimizeFn func(
	Lut **cmsPipeline,
	Intent cmsUInt32Number,
	InputFormat *cmsUInt32Number,
	OutputFormat *cmsUInt32Number,
	dwFlags *cmsUInt32Number,
) cmsBool

// _cmsPipelineEval16Fn is a function type for evaluating the pipeline in 16-bit precision.
type cmsPipelineEval16Fn func(
	In []cmsUInt16Number, // Input array
	Out []cmsUInt16Number, // Output array
	Data unsafe.Pointer,   // Arbitrary data
)

// _cmsPipelineEvalFloatFn is a function type for evaluating the pipeline in floating-point precision.
type cmsPipelineEvalFloatFn func(
	In []cmsFloat32Number, // Input array
	Out []cmsFloat32Number, // Output array
	Data unsafe.Pointer,    // Arbitrary data
)



// This function may be used to set the optional evaluator and a block of private data. If private data is being used, an optional
// duplicator and free functions should also be specified in order to duplicate the LUT construct. Use NULL to inhibit such functionality.


// _cmsPipelineSetOptimizationParameters sets optional evaluator and private data for optimization.
type  cmsPipelineSetOptimizationParameters func(
	Lut *cmsPipeline,
	Eval16 cmsPipelineEval16Fn,
	PrivateData unsafe.Pointer,
	FreePrivateDataFn cmsFreeUserDataFn,
	DupPrivateDataFn cmsDupUserDataFn,
) 
// Optimize entry point
// cmsPluginOptimization represents a plugin that implements optimization strategies.
type cmsPluginOptimization struct {
	Base         cmsPluginBase      // Base plugin structure
	OptimizePtr  cmsOPToptimizeFn  // Optimization entry point
}

//----------------------------------------------------------------------------------------------------------
// Maximum number of types in a plugin array
const MAX_TYPES_IN_LCMS_PLUGIN = 20


// Memory handler. Each new plug-in type replaces current behaviour

// Function type definitions for memory handler plug-ins.

// _cmsMallocFnPtrType defines a function that allocates memory.
type cmsMallocFnPtrType func(contextID cmsContext, size cmsUInt32Number) unsafe.Pointer

// _cmsFreeFnPtrType defines a function that frees allocated memory.
type cmsFreeFnPtrType func(contextID cmsContext, ptr unsafe.Pointer)

// _cmsReallocFnPtrType defines a function that reallocates memory.
type cmsReallocFnPtrType func(contextID cmsContext, ptr unsafe.Pointer, newSize cmsUInt32Number) unsafe.Pointer

// _cmsMalloZerocFnPtrType defines a function that allocates zero-initialized memory.
type cmsMalloZerocFnPtrType func(contextID cmsContext, size cmsUInt32Number) unsafe.Pointer

// _cmsCallocFnPtrType defines a function that allocates zero-initialized memory for an array.
type cmsCallocFnPtrType func(contextID cmsContext, num cmsUInt32Number, size cmsUInt32Number) unsafe.Pointer

// _cmsDupFnPtrType defines a function that duplicates a memory block.
type cmsDupFnPtrType func(contextID cmsContext, org unsafe.Pointer, size cmsUInt32Number) unsafe.Pointer

// cmsPluginMemHandler represents the memory handler plug-in structure.
type cmsPluginMemHandler struct {
	Base          cmsPluginBase         // Base structure for plug-in
	MallocPtr     cmsMallocFnPtrType   // Required: Function to allocate memory
	FreePtr       cmsFreeFnPtrType     // Required: Function to free memory
	ReallocPtr    cmsReallocFnPtrType  // Required: Function to reallocate memory
	MallocZeroPtr cmsMalloZerocFnPtrType // Optional: Function to allocate zero-initialized memory
	CallocPtr     cmsCallocFnPtrType   // Optional: Function to allocate zero-initialized memory for an array
	DupPtr        cmsDupFnPtrType      // Optional: Function to duplicate memory
}



