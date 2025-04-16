package golcms

import (
	"math"
	"reflect"
	"sync"
	"time"
	"unsafe"
)

// Determinant lower than that are assumed zero (used on matrix invert)
const MATRIX_DET_TOLERANCE = 0.0001

// Maximum encodeable values in floating point
const MAX_ENCODEABLE_XYZ = (1.0 + 32767.0/32768.0)
const MIN_ENCODEABLE_ab2 = (-128.0)
const MAX_ENCODEABLE_ab2 = ((65535.0 / 256.0) - 128.0)
const MIN_ENCODEABLE_ab4 = (-128.0)
const MAX_ENCODEABLE_ab4 = (127.0)

const M_LOG10E = 0.434294481903251827651

// Maximum of channels for internal pipeline evaluation
const MAX_STAGE_CHANNELS = 128

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

// cmsALIGNLONG aligns `x` to a multiple of 4 bytes
func cmsALIGNLONG(x uint32) uint32 {
	const alignment = uint32(unsafe.Sizeof(uint32(0)))
	return (x + (alignment - 1)) & ^(alignment - 1)
}

// CMS_PTR_ALIGNMENT determines the pointer alignment for the platform
// For most architectures, this is equal to the size of a pointer.
const CMS_PTR_ALIGNMENT = unsafe.Alignof(unsafe.Pointer(nil))

// cmsALIGNMEM aligns `x` to the pointer alignment boundary
func cmsALIGNMEM(x uintptr) uintptr {
	return (x + (CMS_PTR_ALIGNMENT - 1)) & ^(CMS_PTR_ALIGNMENT - 1)
}
func cmsAssert(condition bool, message string) {
	if !condition {
		panic(message)
	}
}

// Fast floor conversion
func cmsQuickFloor(val float64) int {
	//	fmt.Println("cmsQuickFloor got ", val)
	const _lcms_double2fixmagic = 68719476736.0 * 1.5 // 2^36 * 1.5

	temp := val + _lcms_double2fixmagic
	bits := math.Float64bits(temp) // Get IEEE-754 bit representation as uint64

	if isBigEndian() {
		return int(int32(bits >> 32)) // Get upper 32 bits for big-endian
	}
	//	fmt.Println("cmsQuickFloor result ", int(int32(bits)>>16))
	return int(int32(bits) >> 16)

}

func isBigEndian() bool {
	var i uint32 = 0x1
	b := (*[4]byte)(unsafe.Pointer(&i))
	return b[0] == 0
}

// Fast floor restricted to 0..65535.0
func cmsQuickFloorWord(d float64) uint16 {
	return uint16(cmsQuickFloor(d-32767.0)) + 32767
}

// Floor to word with saturation
func cmsQuickSaturateWord(d float64) uint16 {
	d += 0.5
	if d <= 0 {
		return 0
	}
	if d >= 65535.0 {
		return 0xFFFF
	}
	return cmsQuickFloorWord(d)
}

/* The locking scheme in LCMS described above relies heavily on Windows-specific behaviors, particularly the use of CRITICAL_SECTION for lightweight synchronization. The implementation is tied closely to platform-specific details, such as how CRITICAL_SECTION is initialized and its internal structure.

In Go, sync.Mutex and sync.RWMutex are portable abstractions over platform-specific locking mechanisms. They don't provide direct control over the internals of the mutex implementation, nor do they expose the ability to initialize locks in the same way as CRITICAL_SECTION in LCMS. Go's philosophy is to abstract away such platform-specific details to ensure portability and simplicity.

Here’s how the scenarios differ:
Key Differences Between LCMS Locking and Go sync.Mutex

    Initialization:
        LCMS relies on manual initialization of CRITICAL_SECTION with specific memory values, potentially bypassing InitializeCriticalSection.
        Go's sync.Mutex is zero-initialized and ready for use without explicit initialization.

    Lightweight Locking:
        LCMS prefers CRITICAL_SECTION because it is lighter weight than a kernel mutex on Windows.
        sync.Mutex abstracts this, and the underlying implementation on Windows may use CRITICAL_SECTION or another mechanism, but you cannot control or optimize it for specific use cases.

    Static Initialization:
        LCMS works around Windows' inability to statically initialize CRITICAL_SECTION.
        In Go, sync.Mutex is statically initializable.

    InterlockedCompareExchangePointer:
        LCMS uses InterlockedCompareExchangePointer for atomic operations during mutex initialization.
        In Go, atomic operations are encapsulated in the sync/atomic package but are not needed for sync.Mutex initialization.

    Pre-Windows XP Compatibility:
        LCMS accounts for compatibility with pre-Windows XP systems.
        Go's runtime does not officially support platforms older than Windows 7, so this is not a concern.

Translating LCMS's Mutex Behavior to Go

While Go's sync.Mutex is not a one-to-one match for CRITICAL_SECTION, it provides equivalent functionality for the vast majority of use cases without exposing the low-level control that LCMS uses. */

// Define a type for the mutex
type cmsMutex struct {
	mutex *sync.Mutex
}

func NewCmsMutex() *cmsMutex {
	return &cmsMutex{
		mutex: new(sync.Mutex), // Allocates a Mutex and assigns its pointer
	}
}

// Lock the mutex
func cmsLockPrimitive(m *cmsMutex) int {
	//m.mutex.Lock() //TEMPORARY
	return 0
}

// Unlock the mutex
func cmsUnlockPrimitive(m *cmsMutex) int {
	//m.mutex.Unlock() //TEMPORARY
	return 0
}

// Initialize the mutex (no-op since `sync.Mutex` does not require explicit initialization)
func cmsInitMutexPrimitive(m *cmsMutex) int {
	// In Go, `sync.Mutex` is ready to use once declared.
	return 0
}

// Destroy the mutex (no-op since Go does not require explicit destruction)
func cmsDestroyMutexPrimitive(m *cmsMutex) int {
	// Go's garbage collector handles cleanup automatically.
	return 0
}

// Enter a critical section (equivalent to Lock)
func cmsEnterCriticalSectionPrimitive(m *cmsMutex) int {
	m.mutex.Lock()
	return 0
}

// Leave a critical section (equivalent to Unlock)
func cmsLeaveCriticalSectionPrimitive(m *cmsMutex) int {
	m.mutex.Unlock()
	return 0
}

/* */

// cmsTRANSFORM represents the translation of the C struct cmsTRANSFORM in Go.
type cmsTRANSFORM struct {
	InputFormat     uint32                 // uint32
	OutputFormat    uint32                 // uint32
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
	DwOriginalFlags uint32                 // uint32
	AdaptationState float64                // float64
	RenderingIntent uint32                 // uint32
	ContextID       CmsContext             // CmsContext
	UserData        unsafe.Pointer         // void*
	FreeUserData    cmsFreeUserDataFn      // cmsFreeUserDataFn (function pointer, requires C interop)
	OldXform        cmsTransformFn         // cmsTransformFn (function pointer, requires C interop)
	Worker          cmsTransform2Fn        // cmsTransform2Fn (function pointer, requires C interop)
	MaxWorkers      int32                  // int32
	WorkerFlags     uint32                 // uint32
}

// Named color list internal representation
type cmsNAMEDCOLOR struct {
	Name           [cmsMAX_PATH]byte
	PCS            [3]uint16
	DeviceColorant [cmsMAXCHANNELS]uint16
}

type cmsNAMEDCOLORLIST struct {
	nColors       uint32
	Allocated     uint32
	ColorantCount uint32

	Prefix [33]byte // Prefix and suffix are defined to be 32 characters at most
	Suffix [33]byte

	List []cmsNAMEDCOLOR

	ContextID CmsContext
}

// Internal structure for context
type CmsContextStruct struct {
	Next    CmsContext                      // Points to next context in the new style
	MemPool *cmsSubAllocator                // The memory pool that stores context data
	chunks  [MemoryClientMax]unsafe.Pointer // array of pointers to client chunks. Memory itself is hold in the suballocator.
	// If NULL, then it reverts to global Context0
	DefaultMemoryManager cmsMemPluginChunkType // The allocators used for creating the context itself. Cannot be overridden
}

type cmsCACHE struct {
	// 1-pixel cache (16 bits only)
	CacheIn  [cmsMAXCHANNELS]uint16
	CacheOut [cmsMAXCHANNELS]uint16
}

// Pipelines & Stages ---------------------------------------------------------------------------------------------
type cmsStage struct {
	ContextID      CmsContext         // Context identifier
	Type           cmsStageSignature  // Identifies the stage
	Implements     cmsStageSignature  // Identifies the *function* of the stage (for optimizations)
	InputChannels  uint32             // Input channels -- for optimization purposes
	OutputChannels uint32             // Output channels -- for optimization purposes
	EvalPtr        cmsStageEvalFn     // Points to fn that evaluates the stage (always in floating point)
	DupElemPtr     cmsStageDupElemFn  // Points to a fn that duplicates the *data* of the stage
	FreePtr        cmsStageFreeElemFn // Points to a fn that sets the *data* of the stage free
	Data           unsafe.Pointer     // A generic pointer to whatever memory needed by the stage
	Next           *cmsStage          // Pointer to the next stage in the linked list
}

//----------------------------------------------------------------------------------------------------------

// Pipelines, Multi Process Elements.
// Define function pointer types
type cmsStageEvalFn func(In []float32, Out []float32, mpe *cmsStage)
type cmsStageDupElemFn func(mpe *cmsStage) unsafe.Pointer
type cmsStageFreeElemFn func(mpe *cmsStage)

// cmsPluginMultiProcessElement struct definition
type cmsPluginMultiProcessElement struct {
	Base    cmsPluginBase
	Handler cmsTagTypeHandler
}

type cmsPipeline struct {
	Elements       *cmsStage // Points to elements chain
	InputChannels  uint32
	OutputChannels uint32

	// Data & evaluators
	Data unsafe.Pointer

	Eval16Fn    cmsPipelineEval16Fn
	EvalFloatFn cmsPipelineEvalFloatFn
	FreeDataFn  cmsFreeUserDataFn
	DupDataFn   cmsDupUserDataFn

	ContextID CmsContext // Environment

	SaveAs8Bits bool // Implementation-specific: save as 8 bits if possible
}

// Multilocalized Unicode management ---------------------------------------------------------------------------------------

const cmsNoLanguage = "\x00\x00" // Equivalent to "\0\0"
const cmsNoCountry = "\x00\x00"  // Equivalent to "\0\0"

type cmsMLUentry struct {
	Language uint16
	Country  uint16
	StrW     uint32 // Offset to current unicode string
	Len      uint32 // Length in bytes
}

type cmsMLU struct {
	ContextID        CmsContext
	AllocatedEntries uint32 // Number of allocated entries
	UsedEntries      uint32 // Number of used entries
	Entries          []cmsMLUentry

	PoolSize uint32         // Maximum allocated size of the pool
	PoolUsed uint32         // Currently used size of the pool
	MemPool  unsafe.Pointer // Pointer to the beginning of the memory pool
}

// ---------------------------------------------------------------------------------------------------------

// cmsSubAllocatorChunk represents a chunk of memory in the suballocator.
type cmsSubAllocatorChunk struct {
	Block     *uint8                // Pointer to the memory block
	BlockSize uint32                // Size of the memory block
	Used      uint32                // Amount of memory used in the block
	Next      *cmsSubAllocatorChunk // Pointer to the next chunk
}

// cmsSubAllocator represents the suballocator.
type cmsSubAllocator struct {
	ContextID CmsContext            // Context ID for memory management
	Head      *cmsSubAllocatorChunk // Pointer to the first chunk
}

// cmsCreateSubAlloc creates a suballocator with an initial size.
/*type cmsCreateSubAlloc func(ContextID CmsContext, Initial uint32) *cmsSubAllocator

// cmsSubAllocDestroy destroys the suballocator and frees all associated memory.
type cmsSubAllocDestroy func(s *cmsSubAllocator)

// cmsSubAlloc allocates a block of memory from the suballocator.
type cmsSubAlloc func(s *cmsSubAllocator, size uint32) unsafe.Pointer

// cmsSubAllocDup duplicates a block of memory within the suballocator.
type cmsSubAllocDup func(s *cmsSubAllocator, ptr unsafe.Pointer, size uint32) unsafe.Pointer*/

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

// Chunks of context memory by plug-in client -------------------------------------------------------
// Container for error logger -- not a plug-in
type cmsLogErrorChunkType struct {
	LogErrorHandler cmsLogErrorHandlerFunction // Set to NULL for Context0 fallback
}

// The global Context0 storage for error logger
//var cmsLogErrorChunk cmsLogErrorChunkType

// Container for alarm codes -- not a plug-in
type cmsAlarmCodesChunkType struct {
	AlarmCodes [cmsMAXCHANNELS]uint16
}

// The global Context0 storage for alarm codes
//var cmsAlarmCodesChunk cmsAlarmCodesChunkType

// Container for adaptation state -- not a plug-in
type cmsAdaptationStateChunkType struct {
	AdaptationState float64
}

// The global Context0 storage for memory management
//var cmsMemPluginChunk cmsMemPluginChunkType

// Container for interpolation plug-in
type cmsInterpPluginChunkType struct {
	Interpolators cmsInterpFnFactory
}

// The global Context0 storage for interpolation plug-in
//var cmsInterpPluginChunk cmsInterpPluginChunkType

// Container for parametric curves plug-in
type cmsCurvesPluginChunkType struct {
	ParametricCurves *cmsParametricCurvesCollection
}

// The global Context0 storage for tone curves plug-in
//var cmsCurvesPluginChunk cmsCurvesPluginChunkType

// Container for formatters plug-in
type cmsFormattersPluginChunkType struct {
	FactoryList *cmsFormattersFactoryList
}

// Formatters ------------------------------------------------------------------------------------------------------------

const cmsFLAGS_CAN_CHANGE_FORMATTER = 0x02000000 // Allow change buffer format

// cmsCurveStruct represents the gamma function main structure.
type cms_curve_struct struct {
	InterpParams *cmsInterpParams              // Private optimizations for interpolation
	nSegments    uint32                        // Number of segments in the curve. Zero for a 16-bit based tables
	Segments     []cmsCurveSegment             // The segments
	SegInterp    []*cmsInterpParams            // Array of private optimizations for interpolation in table-based segments
	Evals        []cmsParametricCurveEvaluator // Evaluators (one per segment)

	// 16-bit Table-based representation follows
	nEntries uint32   // Number of table elements
	Table16  []uint16 // The table itself
}

// The global Context0 storage for formatters plug-in//
//var cmsFormattersPluginChunk cmsFormattersPluginChunkType

// Allocate and init formatters container.
//type cmsAllocFormattersPluginChunkFunc func(ctx, src CmsContext)

// This chunk type is shared by TagType plug-in and MPE Plug-in
type cmsTagTypePluginChunkType struct {
	TagTypes *cmsTagTypeLinkedList
}

// The global Context0 storage for tag types plug-in
//var cmsTagTypePluginChunk cmsTagTypePluginChunkType

// The global Context0 storage for multi-process elements plug-in
//var cmsMPETypePluginChunk cmsTagTypePluginChunkType

// Container for tag plug-in
type cmsTagPluginChunkType struct {
	Tag *cmsTagLinkedList
}

// The global Context0 storage for tag plug-in
//var cmsTagPluginChunk cmsTagPluginChunkType

// Container for intents plug-in
type cmsIntentsPluginChunkType struct {
	Intents *cmsIntentsList
}

// The global Context0 storage for intents plug-in
//var cmsIntentsPluginChunk cmsIntentsPluginChunkType

// Container for optimization plug-in  see cmsxform

// cmsICCPROFILE represents the Go version of the C structure.
// Maximum supported tags in a profile
const MAX_TABLE_TAG = 100

type cmsICCPROFILE struct {
	IOhandler       *cmsIOHANDLER                     // I/O handler
	ContextID       CmsContext                        // Thread ID or context
	Created         time.Time                         // Creation time
	Version         uint32                            // ICC profile version
	DeviceClass     cmsProfileClassSignature          // Device class signature
	ColorSpace      cmsColorSpaceSignature            // Color space signature
	PCS             cmsColorSpaceSignature            // PCS signature
	RenderingIntent uint32                            // Rendering intent
	Flags           uint32                            // Flags
	Manufacturer    uint32                            // Manufacturer ID
	Model           uint32                            // Model ID
	Attributes      uint64                            // Profile attributes
	Creator         uint32                            // Creator ID
	ProfileID       cmsProfileID                      // Profile ID
	TagCount        uint32                            // Number of tags
	TagNames        [MAX_TABLE_TAG]cmsTagSignature    // Names of tags
	TagLinked       [MAX_TABLE_TAG]cmsTagSignature    // Tags to which these are linked
	TagSizes        [MAX_TABLE_TAG]uint32             // Sizes of tags on disk
	TagOffsets      [MAX_TABLE_TAG]uint32             // Offsets of tags on disk
	TagSaveAsRaw    [MAX_TABLE_TAG]bool               // Whether to write the tag as raw data
	TagPtrs         [MAX_TABLE_TAG]unsafe.Pointer     // Pointers to tag data
	TagTypeHandlers [MAX_TABLE_TAG]*cmsTagTypeHandler // Handlers for each tag type
	IsWrite         bool                              // Whether the profile is being written
	UsrMutex        *sync.Mutex                       // Mutex for thread-safe access
}

// Mutex plugin container structure.
type cmsMutexPluginChunkType struct {
	CreateMutexPtr  cmsCreateMutexFnPtrType
	DestroyMutexPtr cmsDestroyMutexFnPtrType
	LockMutexPtr    cmsLockMutexFnPtrType
	UnlockMutexPtr  cmsUnlockMutexFnPtrType
}

// Global context storage for mutex plugin.
//var cmsMutexPluginChunk cmsMutexPluginChunkType

// Parallelization plugin container structure.
type cmsParallelizationPluginChunkType struct {
	MaxWorkers  int32
	WorkerFlags int32
	SchedulerFn cmsTransform2Fn
}

// Global context storage for parallelization plugin.
//var cmsParallelizationPluginChunk cmsParallelizationPluginChunkType

func MemcpySlice[T any](dst, src []T, length int) {
	if length > len(src)*int(unsafe.Sizeof(src[0])) || length > len(dst)*int(unsafe.Sizeof(dst[0])) {
		panic("Memcpy: length exceeds slice bounds") // Mimic segmentation fault in C
	}

	// Convert slices to raw byte slices for true memory copying
	dstBytes := unsafe.Slice((*byte)(unsafe.Pointer(&dst[0])), length)
	srcBytes := unsafe.Slice((*byte)(unsafe.Pointer(&src[0])), length)

	copy(dstBytes, srcBytes) // Copy raw memory bytes
}

func MemmoveSlice[T any](dest, src []T, count int) {
	if count > len(src) || count > len(dest) {
		panic("MemmoveSlice: count exceeds slice bounds")
	}
	copy(dest[:count], src[:count])
}

func MemsetSlice[T any](slice []T, value T, length int) {
	if length > len(slice) {
		panic("Memset: length exceeds slice bounds")
	}

	// Convert slice to byte slice for raw memory manipulation
	byteSize := int(unsafe.Sizeof(slice[0])) * length
	byteSlice := unsafe.Slice((*byte)(unsafe.Pointer(&slice[0])), byteSize)

	// Fill memory byte by byte
	valBytes := unsafe.Slice((*byte)(unsafe.Pointer(&value)), byteSize)
	for i := 0; i < byteSize; i++ {
		byteSlice[i] = valBytes[0] // Repeat first byte of value
	}
}
func LabToSlice(lab cmsCIELab) []float64 {
	return []float64{lab.L, lab.a, lab.b}
}

func SliceToLab(f []float64) cmsCIELab {
	return cmsCIELab{L: f[0], a: f[1], b: f[2]}
}
func MatToSlice(mat cmsMAT3) []float64 {
	return []float64{
		mat.V[0].N[0], mat.V[0].N[1], mat.V[0].N[2],
		mat.V[1].N[0], mat.V[1].N[1], mat.V[1].N[2],
		mat.V[2].N[0], mat.V[2].N[1], mat.V[2].N[2],
	}
}

func SliceToMat(s []float64) cmsMAT3 {
	if len(s) != 9 {
		panic("SliceToMat: slice must have 9 elements")
	}
	return cmsMAT3{
		V: [3]cmsVEC3{
			{N: [3]float64{s[0], s[1], s[2]}},
			{N: [3]float64{s[3], s[4], s[5]}},
			{N: [3]float64{s[6], s[7], s[8]}},
		},
	}
}

func VecToSlice(vec cmsVEC3) []float64 {
	return []float64{vec.N[0], vec.N[1], vec.N[2]}
}

func SliceToVec(s []float64) cmsVEC3 {
	if len(s) != 3 {
		panic("SliceToVec: slice must have 3 elements")
	}
	return cmsVEC3{N: [3]float64{s[0], s[1], s[2]}}
}

// memmove copies `n` bytes from `src` to `dst`.
// It works like C's memmove, supporting overlapping memory regions.

// memset sets a block of memory to a specified value.
// Equivalent to C's memset function.
func memset(ptr unsafe.Pointer, value int, num uintptr) {
	// Convert value to byte (0-255).
	byteValue := byte(value)

	// Get a slice pointing to the memory location.
	mem := (*[1 << 30]byte)(ptr)[:num:num]

	// Fill the slice with the given value.
	for i := uintptr(0); i < num; i++ {
		mem[i] = byteValue
	}
}

func memmove(dst, src unsafe.Pointer, n uintptr) {
	// Create byte slices from the pointers
	dstSlice := *(*[]byte)(unsafe.Pointer(&reflect.SliceHeader{
		Data: uintptr(dst),
		Len:  int(n),
		Cap:  int(n),
	}))

	srcSlice := *(*[]byte)(unsafe.Pointer(&reflect.SliceHeader{
		Data: uintptr(src),
		Len:  int(n),
		Cap:  int(n),
	}))

	// Use Go's copy function which handles overlapping memory safely
	copy(dstSlice, srcSlice)
}

func memcpy(dst, src unsafe.Pointer, size uintptr) {
	dstSlice := *(*[]byte)(unsafe.Pointer(&reflect.SliceHeader{
		Data: uintptr(dst),
		Len:  int(size),
		Cap:  int(size),
	}))

	srcSlice := *(*[]byte)(unsafe.Pointer(&reflect.SliceHeader{
		Data: uintptr(src),
		Len:  int(size),
		Cap:  int(size),
	}))

	copy(dstSlice, srcSlice)
}

// strncpy copies up to `n` characters from `src` to a new `dst`.
// It returns the resulting string, null-padded to `n` if `src` is shorter.
func strncpy(src string, n int) string {
	if n <= 0 {
		return ""
	}

	// Convert src to a slice of bytes
	srcBytes := []byte(src)

	// Create a destination buffer of size `n`
	dst := make([]byte, n)

	// Determine how many characters to copy
	copyLen := n
	if len(srcBytes) < n {
		copyLen = len(srcBytes)
	}

	// Copy bytes from src to dst
	copy(dst, srcBytes[:copyLen])

	// Remaining bytes in `dst` are already initialized to '\x00' by `make`

	return string(dst)
}

// strlen calculates the length of a null-terminated byte string.
func strlen(str *byte) int {
	if str == nil {
		return 0
	}

	length := 0
	ptr := uintptr(unsafe.Pointer(str))

	for {
		// Dereference the pointer to get the current byte
		currentByte := *(*byte)(unsafe.Pointer(ptr))
		if currentByte == 0 {
			break
		}

		// Move to the next byte
		ptr++
		length++
	}

	return length
}
