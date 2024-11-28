package golcms

import (
	//"math"
	"unsafe"
)
// Maximum encodeable values in floating point
const MAX_ENCODEABLE_XYZ = (1.0 + 32767.0/32768.0)
const MIN_ENCODEABLE_ab2 = (-128.0)
const MAX_ENCODEABLE_ab2 = ((65535.0/256.0) - 128.0)
const MIN_ENCODEABLE_ab4 = (-128.0)
const MAX_ENCODEABLE_ab4 = (127.0)

// Fixed point macros translated to Go functions
func FIXED_TO_INT(x cmsS15Fixed16Number) int32 {
	return int32(x >> 16)
}

func FIXED_REST_TO_INT(x cmsS15Fixed16Number) uint16 {
	return uint16(x & 0xFFFF)
}

func ROUND_FIXED_TO_INT(x cmsS15Fixed16Number) int32 {
	return int32((x + 0x8000) >> 16)
}

// Conversion functions
func cmsToFixedDomain(a int) cmsS15Fixed16Number {
	return cmsS15Fixed16Number(a + ((a + 0x7FFF) / 0xFFFF))
}

func cmsFromFixedDomain(a cmsS15Fixed16Number) int {
	return int(a - ((a + 0x7FFF) >> 16))
}

// Fast floor conversion
func cmsQuickFloor(val cmsFloat64Number) int {
	// Adjust for specific configurations
	const _lcms_double2fixmagic = 68719476736.0 * 1.5 // 2^36 * 1.5

	var temp struct {
		val    cmsFloat64Number
		halves [2]int32
	}

	temp.val = val + _lcms_double2fixmagic

	if isBigEndian() {
		return int(temp.halves[1] >> 16)
	}
	return int(temp.halves[0] >> 16)
}

func isBigEndian() bool {
	var i uint32 = 0x1
	b := (*[4]byte)(unsafe.Pointer(&i))
	return b[0] == 0
}

// Fast floor restricted to 0..65535.0
func cmsQuickFloorWord(d cmsFloat64Number) cmsUInt16Number {
	return cmsUInt16Number(cmsQuickFloor(d-32767.0)) + 32767
}

// Floor to word with saturation
func cmsQuickSaturateWord(d cmsFloat64Number) cmsUInt16Number {
	d += 0.5
	if d <= 0 {
		return 0
	}
	if d >= 65535.0 {
		return 0xFFFF
	}
	return cmsQuickFloorWord(d)
}

// cmsTRANSFORM represents the translation of the C struct cmsTRANSFORM in Go.
type cmsTRANSFORM struct {
	InputFormat     cmsUInt32Number        // cmsUInt32Number
	OutputFormat    cmsUInt32Number        // cmsUInt32Number
	Xform           cmsTransform2Fn        // cmsTransform2Fn (function pointer, requires C interop)
	FromInput       cmsFormatter16         // cmsFormatter16
	ToOutput        cmsFormatter16         // cmsFormatter16
	FromInputFloat  cmsFormatterFloat      // cmsFormatterFloat
	ToOutputFloat   cmsFormatterFloat      // cmsFormatterFloat
	Cache           cmsCACHE               // cmsCACHE
	Lut             *cmsPipeline           //sPipeline*
	GamutCheck      *cmsPipeline           // cmsPipeline*
	InputColorant   *cmsNAMEDCOLORLIST     // cmsNAMEDCOLORLIST*
	OutputColorant  *cmsNAMEDCOLORLIST     // cmsNAMEDCOLORLIST*
	EntryColorSpace cmsColorSpaceSignature // cmsColorSpaceSignature
	ExitColorSpace  cmsColorSpaceSignature // cmsColorSpaceSignature
	EntryWhitePoint cmsCIEXYZ              // cmsCIEXYZ
	ExitWhitePoint  cmsCIEXYZ              // cmsCIEXYZ
	Sequence        *cmsSEQ                // cmsSEQ*
	DwOriginalFlags cmsUInt32Number        // cmsUInt32Number
	AdaptationState cmsFloat64Number       // cmsFloat64Number
	RenderingIntent cmsUInt32Number        // cmsUInt32Number
	ContextID       cmsContext             // cmsContext
	UserData        unsafe.Pointer         // void*
	FreeUserData    cmsFreeUserDataFn      // cmsFreeUserDataFn (function pointer, requires C interop)
	OldXform        cmsTransform2Fn        // cmsTransformFn (function pointer, requires C interop)
	Worker          cmsTransform2Fn        // cmsTransform2Fn (function pointer, requires C interop)
	MaxWorkers      cmsInt32Number         // cmsInt32Number
	WorkerFlags     cmsUInt32Number        // cmsUInt32Number
}

// Named color list internal representation
type cmsNAMEDCOLOR struct {
	Name           [cmsMAX_PATH]byte
	PCS            [3]cmsUInt16Number
	DeviceColorant [cmsMAXCHANNELS]cmsUInt16Number
}

type cmsNAMEDCOLORLIST struct {
	nColors       cmsUInt32Number
	Allocated     cmsUInt32Number
	ColorantCount cmsUInt32Number

	Prefix [33]byte // Prefix and suffix are defined to be 32 characters at most
	Suffix [33]byte

	List *cmsNAMEDCOLOR

	ContextID cmsContext
}

// Internal structure for context
type cmsContext struct {
	Next    *cmsContext                     // Points to next context in the new style
	MemPool *cmsSubAllocator                // The memory pool that stores context data
	chunks  [MemoryClientMax]unsafe.Pointer // array of pointers to client chunks. Memory itself is hold in the suballocator.
	// If NULL, then it reverts to global Context0
	DefaultMemoryManager cmsMemPluginChunkType // The allocators used for creating the context itself. Cannot be overridden
}

type cmsCACHE struct {
	// 1-pixel cache (16 bits only)
	CacheIn  [cmsMAXCHANNELS]cmsUInt16Number
	CacheOut [cmsMAXCHANNELS]cmsUInt16Number
}

// Pipelines & Stages ---------------------------------------------------------------------------------------------
type cmsStage struct {
	ContextID      cmsContext         // Context identifier
	Type           cmsStageSignature  // Identifies the stage
	Implements     cmsStageSignature  // Identifies the *function* of the stage (for optimizations)
	InputChannels  cmsUInt32Number    // Input channels -- for optimization purposes
	OutputChannels cmsUInt32Number    // Output channels -- for optimization purposes
	EvalPtr        cmsStageEvalFn     // Points to fn that evaluates the stage (always in floating point)
	DupElemPtr     cmsStageDupElemFn  // Points to a fn that duplicates the *data* of the stage
	FreePtr        cmsStageFreeElemFn // Points to a fn that sets the *data* of the stage free
	Data           unsafe.Pointer     // A generic pointer to whatever memory needed by the stage
	Next           *cmsStage          // Pointer to the next stage in the linked list
}

//----------------------------------------------------------------------------------------------------------

// Pipelines, Multi Process Elements.
// Define function pointer types
type cmsStageEvalFn func(In []cmsFloat32Number, Out []cmsFloat32Number, mpe *cmsStage)
type cmsStageDupElemFn func(mpe *cmsStage) interface{}
type cmsStageFreeElemFn func(mpe *cmsStage)

// Placeholder function allocation
/*func cmsStageAllocPlaceholder(
	ContextID cmsContext,
	Type cmsStageSignature,
	InputChannels cmsUInt32Number,
	OutputChannels cmsUInt32Number,
	EvalPtr cmsStageEvalFn,
	DupElemPtr cmsStageDupElemFn,
	FreePtr cmsStageFreeElemFn,
	Data interface{},
) *cmsStage {
	return &cmsStage{
		ContextID:      ContextID,
		Type:           Type,
		InputChannels:  InputChannels,
		OutputChannels: OutputChannels,
		EvalPtr:        EvalPtr,
		DupElemPtr:     DupElemPtr,
		FreePtr:        FreePtr,
		Data:           Data,
		Next:           nil, // Default to nil for linked list
	}
}*/

// cmsPluginMultiProcessElement struct definition
type cmsPluginMultiProcessElement struct {
	Base    cmsPluginBase
	Handler cmsTagTypeHandler
}

type cmsPipeline struct {
	Elements       *cmsStage // Points to elements chain
	InputChannels  cmsUInt32Number
	OutputChannels cmsUInt32Number

	// Data & evaluators
	Data unsafe.Pointer

	Eval16Fn    cmsPipelineEval16Fn
	EvalFloatFn cmsPipelineEvalFloatFn
	FreeDataFn  cmsFreeUserDataFn
	DupDataFn   cmsDupUserDataFn

	ContextID cmsContext // Environment

	SaveAs8Bits cmsBool // Implementation-specific: save as 8 bits if possible
}

type cmsMLUentry struct {
	Language cmsUInt16Number
	Country  cmsUInt16Number
	StrW     cmsUInt32Number // Offset to current unicode string
	Len      cmsUInt32Number // Length in bytes
}

type cmsMLU struct {
	ContextID        cmsContext
	AllocatedEntries cmsUInt32Number // Number of allocated entries
	UsedEntries      cmsUInt32Number // Number of used entries
	Entries          []*cmsMLUentry  // Array of pointers to cmsMLUentry

	PoolSize cmsUInt32Number // Maximum allocated size of the pool
	PoolUsed cmsUInt32Number // Currently used size of the pool
	MemPool  unsafe.Pointer  // Pointer to the beginning of the memory pool
}

// ---------------------------------------------------------------------------------------------------------

// cmsSubAllocatorChunk represents a chunk of memory in the suballocator.
type cmsSubAllocatorChunk struct {
	Block     *cmsUInt8Number       // Pointer to the memory block
	BlockSize cmsUInt32Number       // Size of the memory block
	Used      cmsUInt32Number       // Amount of memory used in the block
	Next      *cmsSubAllocatorChunk // Pointer to the next chunk
}

// cmsSubAllocator represents the suballocator.
type cmsSubAllocator struct {
	ContextID cmsContext            // Context ID for memory management
	Head      *cmsSubAllocatorChunk // Pointer to the first chunk
}

// cmsCreateSubAlloc creates a suballocator with an initial size.
type cmsCreateSubAlloc func(ContextID cmsContext, Initial cmsUInt32Number) *cmsSubAllocator

// cmsSubAllocDestroy destroys the suballocator and frees all associated memory.
type cmsSubAllocDestroy func(s *cmsSubAllocator)

// cmsSubAlloc allocates a block of memory from the suballocator.
type cmsSubAlloc func(s *cmsSubAllocator, size cmsUInt32Number) unsafe.Pointer

// cmsSubAllocDup duplicates a block of memory within the suballocator.
type cmsSubAllocDup func(s *cmsSubAllocator, ptr unsafe.Pointer, size cmsUInt32Number) unsafe.Pointer

// cmsMemoryClient represents the different memory client types.
type cmsMemoryClient int

const (
	UserPtr cmsMemoryClient = iota
	Logger
	AlarmCodesContext
	AdaptationStateContext
	MemPlugin
	InterpPlugin
	CurvesPlugin
	FormattersPlugin
	TagTypePlugin
	TagPlugin
	IntentPlugin
	MPEPlugin
	OptimizationPlugin
	TransformPlugin
	MutexPlugin
	ParallelizationPlugin

	MemoryClientMax // Last in the list
)

// cmsMemPluginChunkType represents a container for memory management plug-ins.
type cmsMemPluginChunkType struct {
	MallocPtr     cmsMallocFnPtrType     // Pointer to malloc function
	MallocZeroPtr cmsMalloZerocFnPtrType // Pointer to malloc zero-initialized function
	FreePtr       cmsFreeFnPtrType       // Pointer to free function
	ReallocPtr    cmsReallocFnPtrType    // Pointer to realloc function
	CallocPtr     cmsCallocFnPtrType     // Pointer to calloc function
	DupPtr        cmsDupFnPtrType        // Pointer to duplicate memory function
}

// cmsContextStruct represents the internal structure for context management.
type cmsContextStruct struct {
	Next                 *cmsContextStruct               // Pointer to the next context in the chain
	MemPool              *cmsSubAllocator                // Memory pool for context data
	Chunks               [MemoryClientMax]unsafe.Pointer // Array of pointers to client chunks
	DefaultMemoryManager cmsMemPluginChunkType           // Default memory manager for creating the context
}

// cmsInstallAllocFunctions copies memory management function pointers from a plug-in to the chunk, taking care of missing routines.
type cmsInstallAllocFunctions func(plugin *cmsPluginMemHandler, ptr *cmsMemPluginChunkType)

// cmsGetContext returns a pointer to a valid context structure, including the global one if the ID is zero.
// Verifies the magic number.
type cmsGetContext func(contextID cmsContext) *cmsContextStruct

// cmsContextGetClientChunk returns the block assigned to the specific memory client zone.
type cmsContextGetClientChunk func(id cmsContext, mc cmsMemoryClient) unsafe.Pointer


// Chunks of context memory by plug-in client -------------------------------------------------------
// Container for error logger -- not a plug-in
type cmsLogErrorChunkType struct {
	LogErrorHandler cmsLogErrorHandlerFunction // Set to NULL for Context0 fallback
}

// The global Context0 storage for error logger
var cmsLogErrorChunk cmsLogErrorChunkType

// Allocate and init error logger container.
func cmsAllocLogErrorChunk(ctx, src *cmsContextStruct) 

// Container for alarm codes -- not a plug-in
type cmsAlarmCodesChunkType struct {
	AlarmCodes [cmsMAXCHANNELS]cmsUInt16Number
}

// The global Context0 storage for alarm codes
var cmsAlarmCodesChunk cmsAlarmCodesChunkType

// Allocate and init alarm codes container.
func cmsAllocAlarmCodesChunk(ctx, src *cmsContextStruct) 

// Container for adaptation state -- not a plug-in
type cmsAdaptationStateChunkType struct {
	AdaptationState cmsFloat64Number
}

// The global Context0 storage for adaptation state
var cmsAdaptationStateChunk cmsAdaptationStateChunkType

// Allocate and init adaptation state container.
func cmsAllocAdaptationStateChunk(ctx, src *cmsContextStruct) 

// The global Context0 storage for memory management
var cmsMemPluginChunk cmsMemPluginChunkType

// Allocate and init memory management container.
func cmsAllocMemPluginChunk(ctx, src *cmsContextStruct) 

// Container for interpolation plug-in
type cmsInterpPluginChunkType struct {
	Interpolators cmsInterpFnFactory
}

// The global Context0 storage for interpolation plug-in
var cmsInterpPluginChunk cmsInterpPluginChunkType

// Allocate and init interpolation container.
func cmsAllocInterpPluginChunk(ctx, src *cmsContextStruct) 

// Container for parametric curves plug-in
type cmsCurvesPluginChunkType struct {
	ParametricCurves *cmsParametricCurvesCollection
}

// The global Context0 storage for tone curves plug-in
var cmsCurvesPluginChunk cmsCurvesPluginChunkType

// Allocate and init parametric curves container.
func cmsAllocCurvesPluginChunk(ctx, src *cmsContextStruct) 

// Container for formatters plug-in
type cmsFormattersPluginChunkType struct {
	FactoryList *cmsFormattersFactoryList
}

// The global Context0 storage for formatters plug-in
var cmsFormattersPluginChunk cmsFormattersPluginChunkType

// Allocate and init formatters container.
type cmsAllocFormattersPluginChunkFunc func(ctx, src *cmsContextStruct) 

// This chunk type is shared by TagType plug-in and MPE Plug-in
type cmsTagTypePluginChunkType struct {
	TagTypes *cmsTagTypeLinkedList
}

// The global Context0 storage for tag types plug-in
var cmsTagTypePluginChunk cmsTagTypePluginChunkType

// The global Context0 storage for multi-process elements plug-in
var cmsMPETypePluginChunk cmsTagTypePluginChunkType

// Allocate and init Tag types container.
func cmsAllocTagTypePluginChunk(ctx, src *cmsContextStruct) 

// Allocate and init MPE container.
func cmsAllocMPETypePluginChunk(ctx, src *cmsContextStruct) 

// Container for tag plug-in
type cmsTagPluginChunkType struct {
	Tag *cmsTagLinkedList
}

// The global Context0 storage for tag plug-in
var cmsTagPluginChunk cmsTagPluginChunkType

// Allocate and init Tag container.
func cmsAllocTagPluginChunk(ctx, src *cmsContextStruct) 

// Container for intents plug-in
type cmsIntentsPluginChunkType struct {
	Intents *cmsIntentsList
}

// The global Context0 storage for intents plug-in
var cmsIntentsPluginChunk cmsIntentsPluginChunkType

// Allocate and init intents container.
func cmsAllocIntentsPluginChunk(ctx, src *cmsContextStruct) 

// Container for optimization plug-in
type cmsOptimizationPluginChunkType struct {
	OptimizationCollection *cmsOptimizationCollection
}

// The global Context0 storage for optimizers plug-in
var cmsOptimizationPluginChunk cmsOptimizationPluginChunkType

// Allocate and init optimizers container.
func cmsAllocOptimizationPluginChunk(ctx, src *cmsContextStruct) 

// Container for transform plug-in
type cmsTransformPluginChunkType struct {
	TransformCollection *cmsTransformCollection
}

// The global Context0 storage for full-transform replacement plug-in
var cmsTransformPluginChunk cmsTransformPluginChunkType

// Allocate and init transform container.
func cmsAllocTransformPluginChunk(ctx, src *cmsContextStruct) 
