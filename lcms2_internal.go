package golcms

import (
	"arena"
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"sync"
	"time"
	"unsafe"

	"github.com/yzigangirova/lcms-go/mem"
)

// Determinant lower than that are assumed zero (used on matrix invert)
const MATRIX_DET_TOLERANCE = 0.0001

// Maximum encodeable values in floating point
const MAX_ENCODEABLE_XYZ = float64(1.0 + 32767.0/32768.0)
const MIN_ENCODEABLE_ab2 = float64(-128.0)
const MAX_ENCODEABLE_ab2 = float64((65535.0 / 256.0) - 128.0)
const MIN_ENCODEABLE_ab4 = float64(-128.0)
const MAX_ENCODEABLE_ab4 = float64(127.0)

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

// only littleendian version
func cmsQuickFloor(val float64) int {
	const _lcms_double2fixmagic = 68719476736.0 * 1.5 // 2^36 * 1.5

	temp := val + _lcms_double2fixmagic
	bits := math.Float64bits(temp)

	// Little-endian only
	return int(int32(bits) >> 16)
}

// Fast floor conversion
/*func cmsQuickFloor(val float64) int {
	//	fmt.Println("cmsQuickFloor got ", val)
	const _lcms_double2fixmagic = 68719476736.0 * 1.5 // 2^36 * 1.5

	temp := val + _lcms_double2fixmagic
	bits := math.Float64bits(temp) // Get IEEE-754 bit representation as uint64

	if isBigEndian() {
		return int(int32(bits >> 32)) // Get upper 32 bits for big-endian
	}
	//	fmt.Println("cmsQuickFloor result ", int(int32(bits)>>16))
	return int(int32(bits) >> 16)

}*/

func isBigEndian() bool {
	var i uint32 = 0x01000000 // MSB first
	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.BigEndian, i)
	return buf.Bytes()[0] != 0x01
}

// Fast floor restricted to 0..65535.0
func cmsQuickFloorWord(d float64) uint16 {
	return uint16(cmsQuickFloor(d-32767.0)) + 32767
}

// Floor to word with saturation

func cmsQuickSaturateWord(d float64) uint16 {
	if d <= 0 {
		return 0
	}
	if d >= 65535.0 {
		return 0xFFFF
	}
	return cmsQuickFloorWord(d + 0.5)
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
/*type cmsMutex struct {
	mutex *sync.Mutex
}*/

type cmsMutex *sync.Mutex

func NewCmsMutex() *cmsMutex {
	m := new(sync.Mutex)
	return (*cmsMutex)(&m) // Allocates a Mutex and assigns its pointer
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
	mm := *m
	((*sync.Mutex)(mm)).Lock()
	return 0
}

// Leave a critical section (equivalent to Unlock)
func cmsLeaveCriticalSectionPrimitive(m *cmsMutex) int {
	mm := *m
	((*sync.Mutex)(mm)).Unlock()
	return 0
}

/* */

// cmsTRANSFORM represents the translation of the C struct cmsTRANSFORM in Go.
type cmsTRANSFORM struct {
	Ar              *arena.Arena
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
	UserData        any                    // void*
	FreeUserData    cmsFreeUserDataFn      // cmsFreeUserDataFn (function pointer, requires C interop)
	OldXform        cmsTransformFn         // cmsTransformFn (function pointer, requires C interop)
	Worker          cmsTransform2Fn        // cmsTransform2Fn (function pointer, requires C interop)
	MaxWorkers      int32                  // int32
	WorkerFlags     uint32                 // uint32
}

func (tr *cmsTRANSFORM) DestroyArena() {
	fmt.Println("DestroyArena")
	if tr.Ar != nil {
		fmt.Println("freeing arena")
		tr.Ar.Free()
	}
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
	Next    CmsContext                       // Points to next context in the new style
	MemPool *cmsSubAllocator                 // The memory pool that stores context data
	chunks  [MemoryClientMax]cmsContextChunk // array of pointers to client chunks. Memory itself is hold in the suballocator.
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
	Data           any                // A generic pointer to whatever memory needed by the stage
	Next           *cmsStage          // Pointer to the next stage in the linked list
}

//----------------------------------------------------------------------------------------------------------

// Pipelines, Multi Process Elements.
// Define function pointer types
type cmsStageEvalFn func(mm mem.Manager, In []float32, Out []float32, mpe *cmsStage)
type cmsStageDupElemFn func(mm mem.Manager, mpe *cmsStage) any
type cmsStageFreeElemFn func(mm mem.Manager, mpe *cmsStage)

type cmsPipeline struct {
	Elements       *cmsStage // Points to elements chain
	InputChannels  uint32
	OutputChannels uint32

	// Data & evaluators
	Data any

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

	PoolSize uint32 // Maximum allocated size of the pool
	PoolUsed uint32 // Currently used size of the pool
	MemPool  any    // Pointer to the beginning of the memory pool
}

// ---------------------------------------------------------------------------------------------------------

// cmsSubAllocatorChunk represents a chunk of memory in the suballocator.
type cmsSubAllocatorChunk struct {
	Block     []uint8               // Pointer to the memory block
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
	TagPtrs         [MAX_TABLE_TAG]interface{}        // Pointers to tag data
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
	if length > len(src) || length > len(dst) {
		panic("Memcpy: length exceeds slice bounds") // Mimic segmentation fault in C
	}
	copy(dst, src) // Copy raw memory bytes
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
	for i := 0; i < length; i++ {
		slice[i] = value // Repeat first byte of value
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
	//fmt.Println("SliceToVec")
	if len(s) != 3 {
		panic("SliceToVec: slice must have 3 elements")
	}
	return cmsVEC3{N: [3]float64{s[0], s[1], s[2]}}
}

// memmove copies `n` bytes from `src` to `dst`.
// It works like C's memmove, supporting overlapping memory regions.

// memset sets a block of memory to a specified value.
// Equivalent to C's memset function.
func memset(ptr any, value int, num uintptr) {
	// Convert value to byte (0-255).
	byteValue := byte(value)

	// Get a slice pointing to the memory location.
	mem := ptr.([]byte)

	// Fill the slice with the given value.
	for i := uintptr(0); i < num; i++ {
		mem[i] = byteValue
	}
}

// memmove copies n bytes from src to dst, handling overlapping regions correctly.
// This is a direct implementation without slice conversion overhead.
/*func memmove(dst, src unsafe.Pointer, n uintptr) {
	if dst == src || n == 0 {
		return
	}

	// Determine copy direction for overlapping regions
	if uintptr(dst) < uintptr(src) {
		// Forward copy
		for i := uintptr(0); i < n; i++ {
			*(*byte)(unsafe.Pointer(uintptr(dst) + i)) = *(*byte)(unsafe.Pointer(uintptr(src) + i))
		}
	} else {
		// Backward copy for overlapping regions
		for i := n; i > 0; i-- {
			*(*byte)(unsafe.Pointer(uintptr(dst) + i - 1)) = *(*byte)(unsafe.Pointer(uintptr(src) + i - 1))
		}
	}
}

// memcpy copies n bytes from src to dst.
// Unlike memmove, memcpy doesn't handle overlapping regions (undefined behavior if they overlap)
func memcpy(dst, src unsafe.Pointer, n uintptr) {
	if dst == src || n == 0 {
		return
	}

	// Copy byte by byte
	for i := uintptr(0); i < n; i++ {
		*(*byte)(unsafe.Pointer(uintptr(dst) + i)) = *(*byte)(unsafe.Pointer(uintptr(src) + i))
	}
}*/

// strncpy copies up to `n` characters from `src` to a new `dst`.
// It returns the resulting string, null-padded to `n` if `src` is shorter.
// strncpy copies up to n bytes from src to a new string.
// If src is shorter than n, the remaining bytes are null ('\x00') padded.
func strncpy(src string, n int) string {
	if n <= 0 {
		return ""
	}

	// Fast path: if src is exactly n bytes, we can use it directly
	if len(src) == n {
		return src
	}

	// Create destination buffer (automatically zero-initialized)
	dst := make([]byte, n)

	// Copy only the available bytes (min of src length or n)
	copy(dst, src)

	// Convert to string (this makes an immutable copy)
	return string(dst)
}
