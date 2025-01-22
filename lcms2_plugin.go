package golcms

import (
	"unsafe"
)

// Constants
const (
	VX = 0
	VY = 1
	VZ = 2
)

// Vectors and Matrices
type cmsVEC3 struct {
	N [3]float64
}

type cmsMAT3 struct {
	V [3]cmsVEC3
}

// Plug-in foundation
const (
	cmsPluginMagicNumber            uint32 = 0x61637070 // 'acpp'
	cmsPluginMemHandlerSig          uint32 = 0x6D656D48 // 'memH'
	cmsPluginInterpolationSig       uint32 = 0x696E7048 // 'inpH'
	cmsPluginParametricCurveSig     uint32 = 0x70617248 // 'parH'
	cmsPluginFormattersSig          uint32 = 0x66726D48 // 'frmH'
	cmsPluginTagTypeSig             uint32 = 0x74797048 // 'typH'
	cmsPluginTagSig                 uint32 = 0x74616748 // 'tagH'
	cmsPluginRenderingIntentSig     uint32 = 0x696E7448 // 'intH'
	cmsPluginMultiProcessElementSig uint32 = 0x6D706548 // 'mpeH'
	cmsPluginOptimizationSig        uint32 = 0x6F707448 // 'optH'
	cmsPluginTransformSig           uint32 = 0x7A666D48 // 'xfmH'
	cmsPluginMutexSig               uint32 = 0x6D747A48 // 'mtxH'
	cmsPluginParallelizationSig     uint32 = 0x70726C48 // 'prlH'

)

// Tag Base
type cmsTagTypeSignature uint32

type cmsTagSignature uint32

// Vectors and Matrices Operations
func _cmsVEC3Init(r *cmsVEC3, x, y, z float64) {
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
type cmsInterpFn16 func(input *uint16, output *uint16, params *cmsInterpParams)

// _cmsInterpFnFloat is a function type for floating-point interpolation functions.
// Performs full-precision interpolation (e.g., tetrahedral or trilinear).
type cmsInterpFnFloat func(input *float32, output *float32, params *cmsInterpParams)

// cmsInterpFunction holds either a 16-bit or floating-point interpolation function.
type cmsInterpFunction struct {
	Lerp16    cmsInterpFn16
	LerpFloat cmsInterpFnFloat
}

// cmsInterpParams represents the parameters for interpolation.
type cmsInterpParams struct {
	ContextID     cmsContext                   // The calling thread context
	dwFlags       uint32                       // Flags for interpolation
	nInputs       uint32                       // Number of input channels (3D interpolation if > 1)
	nOutputs      uint32                       // Number of output channels (3D interpolation if > 1)
	nSamples      [MAX_INPUT_DIMENSIONS]uint32 // Valid samples for each dimension
	Domain        [MAX_INPUT_DIMENSIONS]uint32 // Domain = nSamples - 1
	opta          [MAX_INPUT_DIMENSIONS]uint32 // Optimization values for 3D CLUT
	Table         unsafe.Pointer               // Pointer to the actual interpolation table
	Interpolation cmsInterpFunction            // Interpolation functions
}

// cmsInterpFnFactory is a function type for creating an interpolator.
// It returns an interpolator function (either 16-bit or float).
type cmsInterpFnFactory func(nInputChannels, nOutputChannels, dwFlags uint32) cmsInterpFunction

// cmsPluginBase represents the base structure for plugins.
type cmsPluginBase struct {
	Magic           uint32         // Magic number for validation
	ExpectedVersion uint32         // Expected version of the library
	Type            uint32         // Plugin type
	Next            *cmsPluginBase // Pointer to the next plugin in the chain
}

// cmsIntentFn defines the function type for custom intents.
type cmsIntentFn func(
	ContextID cmsContext, // Context ID
	nProfiles uint32, // Number of profiles
	Intents []uint32, // Array of intents
	hProfiles []cmsHPROFILE, // Array of profile handles
	BPC []bool, // Array of Black Point Compensation flags
	AdaptationStates []float64, // Array of adaptation states
	dwFlags uint32, // Flags
) *cmsPipeline

// cmsPluginRenderingIntent represents a plug-in that defines a single rendering intent.
type cmsPluginRenderingIntent struct {
	Base        cmsPluginBase // Base structure for plugins
	Intent      uint32        // Intent number
	Link        cmsIntentFn   // Function link to handle the intent
	Description string        // Description of the intent
}

// cmsPluginInterpolation represents the plugin structure for interpolators.
type cmsPluginInterpolation struct {
	base                 cmsPluginBase
	InterpolatorsFactory cmsInterpFnFactory // Factory function for interpolators
}

// The rest of the vector/matrix operations
// Each follows a similar idiomatic Go implementation as above

// Parametric Curve Evaluator
type cmsParametricCurveEvaluator func(int32, []float64, float64) float64

type cmsPluginParametricCurves struct {
	Base           cmsPluginBase
	NFunctions     uint32
	FunctionTypes  [20]uint32
	ParameterCount [20]uint32
	Evaluator      cmsParametricCurveEvaluator
}

// Plugin Tag Type

// _cmsIOHandler represents the internal structure.
type cms_io_handler struct {
	Stream       unsafe.Pointer // Associated stream, implemented differently based on media
	ContextID    cmsContext     // Context ID
	UsedSpace    uint32         // Used space in the stream
	ReportedSize uint32         // Reported size of the stream
	PhysicalFile string         // Physical file path
	Read         func(iohandler *cms_io_handler, buffer unsafe.Pointer, size, count uint32) uint32
	Seek         func(iohandler *cms_io_handler, offset uint32) bool
	Close        func(iohandler *cms_io_handler) bool
	Tell         func(iohandler *cms_io_handler) uint32
	Write        func(iohandler *cms_io_handler, size uint32, buffer unsafe.Pointer) bool
}

//----------------------------------------------------------------------------------------------------------

// This is the tag plugin, which identifies tags. For writing, a pointer to function is provided.
// This function should return the desired type for this tag, given the version of profile
// and the data being serialized.

// cmsTagDescriptor represents a tag plugin that identifies tags and manages reading and writing.
type cmsTagDescriptor struct {
	ElemCount       uint32                                        // Number of elements if this tag needs an array
	NSupportedTypes uint32                                        // Number of supported types for this tag (MAX_TYPES_IN_LCMS_PLUGIN maximum)
	SupportedTypes  [MAX_TYPES_IN_LCMS_PLUGIN]cmsTagTypeSignature // Array of supported types

	// Function for determining the type for writing, based on profile version and data.
	DecideType func(iccVersion float64, data unsafe.Pointer) cmsTagTypeSignature
}

// cmsPluginTag represents a plugin that implements a single tag.
type cmsPluginTag struct {
	Base       cmsPluginBase    // Base plugin structure
	Signature  cmsTagSignature  // Tag signature
	Descriptor cmsTagDescriptor // Descriptor defining the tag's behavior
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

type cmsFormatterFactory func(uint32, cmsFormatterDirection, uint32) cmsFormatter

type cmsPluginFormatters struct {
	Base              cmsPluginBase
	FormattersFactory cmsFormatterFactory
}

// Transform
type cmsStride struct {
	BytesPerLineIn   uint32
	BytesPerLineOut  uint32
	BytesPerPlaneIn  uint32
	BytesPerPlaneOut uint32
}

// cmsPluginTransform represents the plugin transform structure.
type cmsPluginTransform struct {
	Base cmsPluginBase // Base plugin information

	// Transform entry points
	Factories struct {
		LegacyXform cmsTransformFactory  // Legacy transform factory
		Xform       cmsTransform2Factory // Modern transform factory
	}
}

// Shared callbacks for user data //YULIANA: i can not find implemenation for this functions, only declarations!  investigate further
type cmsFreeUserDataFn func(ContextID cmsContext, Data unsafe.Pointer)
type cmsDupUserDataFn func(ContextID cmsContext, Data unsafe.Pointer) unsafe.Pointer
type cmsFormatter16 func(CMMcargo *cmsTRANSFORM, Values []uint16, Buffer []uint8, Stride uint32) []uint8
type cmsFormatterFloat func(CMMcargo *cmsTRANSFORM, Values []float32, Buffer []uint8, Stride uint32) []uint8
type cmsTransformFn func(CMMcargo *cmsTRANSFORM, InputBuffer unsafe.Pointer,
	OutputBuffer unsafe.Pointer, Size uint32, Stride uint32)

type cmsTransform2Fn func(CMMcargo *cmsTRANSFORM, InputBuffer unsafe.Pointer,
	OutputBuffer unsafe.Pointer, PixelsPerLine uint32, LineCount uint32, Stride *cmsStride)

type cmsTransformFactory func(xform *cmsTransformFn, UserData *unsafe.Pointer,
	FreePrivateDataFn *cmsFreeUserDataFn, Lut **cmsPipeline, InputFormat *uint32, OutputFormat *uint32, dwFlags *uint32) bool

type cmsTransform2Factory func(xform *cmsTransform2Fn, UserData *unsafe.Pointer,
	FreePrivateDataFn *cmsFreeUserDataFn, Lut **cmsPipeline, InputFormat *uint32, OutputFormat *uint32, dwFlags *uint32) bool

type cmsFormatter struct {
	Fmt16    cmsFormatter16
	FmtFloat cmsFormatterFloat
}

const CMS_PACK_FLAGS_16BITS = 0x0000
const CMS_PACK_FLAGS_FLOAT = 0x0001

// StageToneCurvesData represents data for tone curves.
type cmsStageToneCurvesData struct {
	NCurves   uint32          // Number of curves
	TheCurves **cmsToneCurve // Slice of pointers to ToneCurve
}

// StageMatrixData represents data for a matrix transformation.
type cmsStageMatrixData struct {
	Double *float64 // Floating-point matrix data
	Offset *float64 // Optional offset data
}

// StageCLutData represents data for a color lookup table (CLUT).
type cmsStageCLutData struct {
	Tab struct {
		T      *uint16  // 16-bit table
		TFloat *float32 // Float table
	} // Union-like structure for CLUT representation
	Params         *cmsInterpParams // Interpolation parameters
	NEntries       uint32           // Number of entries in the table
	HasFloatValues bool             // Indicates if the table uses float values
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
	Intent uint32,
	InputFormat *uint32,
	OutputFormat *uint32,
	dwFlags *uint32,
) bool

// _cmsPipelineEval16Fn is a function type for evaluating the pipeline in 16-bit precision.
type cmsPipelineEval16Fn func(
	In []uint16, // Input array
	Out []uint16, // Output array
	Data unsafe.Pointer, // Arbitrary data
)

// _cmsPipelineEvalFloatFn is a function type for evaluating the pipeline in floating-point precision.
type cmsPipelineEvalFloatFn func(
	In []float32, // Input array
	Out []float32, // Output array
	Data unsafe.Pointer, // Arbitrary data
)

// Optimize entry point
// cmsPluginOptimization represents a plugin that implements optimization strategies.
type cmsPluginOptimization struct {
	Base        cmsPluginBase    // Base plugin structure
	OptimizePtr cmsOPToptimizeFn // Optimization entry point
}

// ----------------------------------------------------------------------------------------------------------
// Maximum number of types in a plugin array
const MAX_TYPES_IN_LCMS_PLUGIN = 20

// Memory handler. Each new plug-in type replaces current behaviour

// Function type definitions for memory handler plug-ins.

// _cmsMallocFnPtrType defines a function that allocates memory.
type cmsMallocFnPtrType func(contextID cmsContext, size uint32) unsafe.Pointer

// _cmsFreeFnPtrType defines a function that frees allocated memory.
type cmsFreeFnPtrType func(contextID cmsContext, ptr unsafe.Pointer)

// _cmsReallocFnPtrType defines a function that reallocates memory.
type cmsReallocFnPtrType func(contextID cmsContext, ptr unsafe.Pointer, newSize uint32) unsafe.Pointer

// _cmsMalloZerocFnPtrType defines a function that allocates zero-initialized memory.
type cmsMalloZerocFnPtrType func(contextID cmsContext, size uint32) unsafe.Pointer

// _cmsCallocFnPtrType defines a function that allocates zero-initialized memory for an array.
type cmsCallocFnPtrType func(contextID cmsContext, num uint32, size uint32) unsafe.Pointer

// _cmsDupFnPtrType defines a function that duplicates a memory block.
type cmsDupFnPtrType func(contextID cmsContext, org unsafe.Pointer, size uint32) unsafe.Pointer

// cmsPluginMemHandler represents the memory handler plug-in structure.
type cmsPluginMemHandler struct {
	Base          cmsPluginBase          // Base structure for plug-in
	MallocPtr     cmsMallocFnPtrType     // Required: Function to allocate memory
	FreePtr       cmsFreeFnPtrType       // Required: Function to free memory
	ReallocPtr    cmsReallocFnPtrType    // Required: Function to reallocate memory
	MallocZeroPtr cmsMalloZerocFnPtrType // Optional: Function to allocate zero-initialized memory
	CallocPtr     cmsCallocFnPtrType     // Optional: Function to allocate zero-initialized memory for an array
	DupPtr        cmsDupFnPtrType        // Optional: Function to duplicate memory
}

// Type aliases for function pointer types.
type cmsCreateMutexFnPtrType func(ContextID cmsContext) unsafe.Pointer
type cmsDestroyMutexFnPtrType func(ContextID cmsContext, mtx unsafe.Pointer)
type cmsLockMutexFnPtrType func(ContextID cmsContext, mtx unsafe.Pointer) bool
type cmsUnlockMutexFnPtrType func(ContextID cmsContext, mtx unsafe.Pointer)

// Mutex plugin structure.
type cmsPluginMutex struct {
	base            cmsPluginBase
	CreateMutexPtr  cmsCreateMutexFnPtrType
	DestroyMutexPtr cmsDestroyMutexFnPtrType
	LockMutexPtr    cmsLockMutexFnPtrType
	UnlockMutexPtr  cmsUnlockMutexFnPtrType
}

// CMSAPI equivalent functions.

// Create a new mutex.
func cmsCreateMutex(ContextID cmsContext) unsafe.Pointer

// Destroy a mutex.
func cmsDestroyMutex(ContextID cmsContext, mtx unsafe.Pointer)

// Lock the mutex.
func cmsLockMutex(ContextID cmsContext, mtx unsafe.Pointer) bool

// Unlock the mutex.
func cmsUnlockMutex(ContextID cmsContext, mtx unsafe.Pointer)

type cmsPluginParalellization struct {
	base        cmsPluginBase
	MaxWorkers  int32           // Number of starts to do as maximum
	WorkerFlags uint32          // Reserved
	SchedulerFn cmsTransform2Fn // callback to setup functions

}

// ICC base tag
type cmsTagBas struct {
	sig      cmsTagTypeSignature
	reserved [4]uint8
}
