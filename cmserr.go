package golcms

//import "C"
import (
	//"bytes"
	//"encoding/binary"
	"fmt"
	"math"
	"os"
	"unicode"
	"unsafe"

	"github.com/yzigangirova/lcms-go/mem"
)

// ---------------------------------------------------------------------------------------------------------

// This is our default log error

// Context0 storage, which is global
var cmsLogErrorChunk = cmsLogErrorChunkType{DefaultLogErrorHandlerFunction}

// Allocates and inits error logger container for a given context. If src is NULL, only initializes the value
// to the default. Otherwise, it duplicates the value. The interface is standard across all context clients
/*func cmsAllocLogErrorChunk(struct _cmsContext_struct* ctx,
                            const struct _cmsContext_struct* src){
    static _cmsLogErrorChunkType LogErrorChunk = { DefaultLogErrorHandlerFunction };
    void* from;

     if (src != NULL) {
        from = src ->chunks[Logger];
    }
    else {
       from = &LogErrorChunk;
    }

    ctx ->chunks[Logger] = cmsSubAllocDup(ctx ->MemPool, from, sizeof(_cmsLogErrorChunkType));
}*/

// The default error logger does nothing.
func DefaultLogErrorHandlerFunction(ContextID CmsContext, ErrorCode uint32, text string) {
	// fprintf(stderr, "[lcms]: %s\n", Text);
	// fflush(stderr);

}

// cmsSignalError simulates error signaling
func cmsSignalError(id any, code int, message string, args ...any) {
	msg := fmt.Sprintf(message, args...)
	fmt.Fprintf(os.Stderr, "Error GOLCMS (%d) %s\n", code, msg)
	/* not translated in Go yet
		 // Check for the context, if specified go there. If not, go for the global
	    lhg = (cmsLogErrorChunkType*) CmsContextGetClientChunk(ContextID, Logger);
	    if (lhg.LogErrorHandler) {
	        lhg.LogErrorHandler(ContextID, ErrorCode, Buffer);
	    }   */
}

// Maximum allowed memory allocation (equivalent to MAX_MEMORY_FOR_ALLOC in C)
const MAX_MEMORY_FOR_ALLOC = 512 * 1024 * 1024 // 512MB

// allocateMemory allocates a block of memory for a given size in bytes.
func allocateMemory(size uintptr) []byte {
	if size == 0 {
		return nil
	}

	// Allocate memory manually using a Go slice and return its pointer.
	mem := make([]byte, size)
	return mem
}

// freeMemory frees manually allocated memory. (No-op in Go)
func freeMemory(ptr any, size uintptr) {
	// Memory will be garbage collected, but this function can be used for compatibility.
}

// accessMemory allows accessing memory at an offset from a base pointer.

// Default memory allocation function
func cmsMallocDefaultFn(ContextID CmsContext, size uint32) []byte {
	return allocateMemory(uintptr(size))
}

// Generic allocate & zero
func cmsMallocZeroDefaultFn(ContextID CmsContext, size uint32) []byte {
	ptr := allocateMemory(uintptr(size))
	if ptr == nil {
		return nil
	}
	return ptr
}

// Default free function
func cmsFreeDefaultFn(ContextID CmsContext, ptr any, size uint32) {
	freeMemory(ptr, uintptr(size))
}

// Default realloc function
func cmsReallocDefaultFn(ContextID CmsContext, ptr any, newSize uint32, oldSize uint32) []byte {
	if newSize > MAX_MEMORY_FOR_ALLOC {
		return nil
	}
	newPtr := allocateMemory(uintptr(newSize))
	if newPtr == nil {
		return nil
	}
	if ptr != nil {
		switch src := ptr.(type) {
		case []byte:
			copySize := int(oldSize)
			if int(newSize) < copySize {
				copySize = int(newSize)
			}
			copy(newPtr, src[:copySize])
		case *[]byte:
			copySize := int(oldSize)
			if int(newSize) < copySize {
				copySize = int(newSize)
			}
			copy(newPtr, (*src)[:copySize])
		default:
			cmsSignalError(nil, cmsERROR_RANGE, "Unsupported buffer type in cmsReallocDefaultFn")
			return nil
		}
	}
	return newPtr
}

// Default calloc function
func cmsCallocDefaultFn(ContextID CmsContext, num, size uint32) []byte {
	total := uint64(num) * uint64(size)
	if total == 0 || total > MAX_MEMORY_FOR_ALLOC || num > math.MaxUint32/size {
		return nil
	}
	return cmsMallocZeroDefaultFn(ContextID, uint32(total))
}
func cmsDupDefaultFn(ContextID CmsContext, Org any, size uint32) []byte {
	if size > MAX_MEMORY_FOR_ALLOC {
		return nil
	}

	dst := allocateMemory(uintptr(size)) // returns []byte or interface holding []byte
	if dst == nil || Org == nil {
		return dst
	}

	switch src := Org.(type) {
	case []byte:
		copySize := int(size)
		if len(src) < copySize {
			copySize = len(src)
		}
		copy(dst, src[:copySize])
	case *[]byte:
		copySize := int(size)
		if len(*src) < copySize {
			copySize = len(*src)
		}
		copy(dst, (*src)[:copySize])
	default:
		cmsSignalError(nil, cmsERROR_RANGE, "Unsupported source type in cmsDupDefaultFn")
		return nil
	}

	return dst
}

// DupMem duplicates a single struct or value CAN NOT USE THIS!  generic function
//can not be assigned
/*func cmsDupDefaultFn[T any](src *T) *T {
	if src == nil {
		return nil
	}

	// Allocate new memory (Go manages this)
	dst := new(T)

	// Copy memory
	*dst = *src

	return dst
}*/

// Pointers to memory manager functions in Context0
var cmsMemPluginChunk = cmsMemPluginChunkType{cmsMallocDefaultFn, cmsMallocZeroDefaultFn, cmsFreeDefaultFn,
	cmsReallocDefaultFn, cmsCallocDefaultFn, cmsDupDefaultFn}

// Plug-in replacement entry
//
//lint:ignore U1000 kept for parity with lcms; used in future ports
func cmsRegisterMemHandlerPlugin(context CmsContext, Data PluginIntrfc) bool {
	var ptr *cmsMemPluginChunkType
	if Data == nil {
		// NULL forces to reset to defaults. In this special case, the defaults are stored in the context structure.
		// Remaining plug-ins does NOT have any copy in the context structure, but this is somehow special as the
		// context internal data should be malloce'd by using those functions.
		ctx := (*CmsContextStruct)(context)

		// Return to the default allocators
		if context != nil {
			ctx.chunks[MemPlugin] = &ctx.DefaultMemoryManager
		}
		return true
	}

	plugin, ok := Data.(*cmsPluginMemHandler)
	if !ok {
		panic("Plugin is not of the type cmsPluginMemHandler")
	}
	// Check for required callbacks
	if plugin.MallocPtr == nil || plugin.FreePtr == nil || plugin.ReallocPtr == nil {
		return false
	}

	// Set replacement functions
	ptr, ok = CmsContextGetClientChunk(context, MemPlugin).(*cmsMemPluginChunkType)
	if !ok || ptr == nil {
		return false
	}

	cmsInstallAllocFunctions(plugin, ptr)
	return true
}

// Generic allocate
func cmsMalloc(contextID CmsContext, size uint32) []byte {
	ptr, ok := CmsContextGetClientChunk(contextID, MemPlugin).(*cmsMemPluginChunkType) // Assume 0 is the MemPlugin index
	if !ok || ptr == nil || ptr.MallocPtr == nil {
		return nil
	}
	return ptr.MallocPtr(contextID, size)
}

// Generic allocate & zero replaced with allocStruct
/*func cmsMallocZero(contextID CmsContext, size uint32) unsafe.Pointer {
	ptr := (*cmsMemPluginChunkType)(CmsContextGetClientChunk(contextID, MemPlugin))
	return ptr.MallocZeroPtr(contextID, size)
}*/

// Generic calloc
func cmsCalloc(contextID CmsContext, num, size uint32) []byte {
	ptr, ok := CmsContextGetClientChunk(contextID, MemPlugin).(*cmsMemPluginChunkType)
	if !ok || ptr == nil || ptr.CallocPtr == nil {
		return nil
	}
	return ptr.CallocPtr(contextID, num, size)
}

// Generic reallocate
func cmsRealloc(contextID CmsContext, oldPtr any, size uint32) []byte {
	ptr, ok := CmsContextGetClientChunk(contextID, MemPlugin).(*cmsMemPluginChunkType)
	if !ok || ptr == nil || ptr.ReallocPtr == nil {
		return nil
	}
	return ptr.ReallocPtr(contextID, oldPtr, size, size)
}

// Generic free memory
func cmsFree(contextID CmsContext, oldPtr any) {
	if oldPtr != nil {
		ptr, ok := CmsContextGetClientChunk(contextID, MemPlugin).(*cmsMemPluginChunkType)
		if !ok || ptr != nil && ptr.FreePtr != nil {
			ptr.FreePtr(contextID, oldPtr, 0) //have to think about freeing memory and size variable
		}
	}
}

// Generic block duplication for structures
func cmsDupMem(contextID CmsContext, org any, size uint32) []byte {
	ptr, ok := CmsContextGetClientChunk(contextID, MemPlugin).(*cmsMemPluginChunkType)
	if !ok || ptr == nil || ptr.DupPtr == nil || org == nil {
		return nil
	}
	return ptr.DupPtr(contextID, org, size)
}

// for slices
// DupMemSlice duplicates a slice of any type
func cmsDupMemSlice[T any](src []T) []T {
	if len(src) == 0 {
		return nil
	}

	// Allocate new slice
	dst := make([]T, len(src))

	// Copy contents
	copy(dst, src)

	return dst
}

// ********************************************************************************************

// Sub allocation takes care of many pointers of small size. The memory allocated in
// this way have be freed at once. Next function allocates a single chunk for linked list
// I prefer this method over realloc due to the big impact on xput realloc may have if
// memory is being swapped to disk. This approach is safer (although that may not be true on all platforms)
// Create a new suballocation chunk
func cmsCreateSubAllocChunk(mm mem.Manager, contextID CmsContext, initial uint32) *cmsSubAllocatorChunk {
	if initial == 0 {
		initial = 20 * 1024 // Default to 20KB
	}

	chunk := mem.New[cmsSubAllocatorChunk](mm)
	if chunk == nil {
		return nil
	}

	chunk.Block = make([]byte, initial)
	if chunk.Block == nil {
		cmsFree(contextID, (chunk))
		return nil
	}

	chunk.BlockSize = initial
	chunk.Used = 0
	chunk.Next = nil

	return chunk
}

// Create a new suballocator
func cmsCreateSubAlloc(mm mem.Manager, contextID CmsContext, initial uint32) *cmsSubAllocator {
	sub := mem.New[cmsSubAllocator](mm)
	if sub == nil {
		return nil
	}

	sub.ContextID = (CmsContext)(contextID)
	sub.Head = cmsCreateSubAllocChunk(mm, contextID, initial)
	if sub.Head == nil {
		cmsFree(contextID, sub)
		return nil
	}

	return sub
}

// Destroy the suballocator and free all associated memory
func cmsSubAllocDestroy(sub *cmsSubAllocator) {
	for chunk := sub.Head; chunk != nil; {
		next := chunk.Next
		if chunk.Block != nil {
			cmsFree(sub.ContextID, chunk.Block)
		}
		cmsFree(sub.ContextID, chunk)
		chunk = next
	}

	cmsFree(sub.ContextID, sub)
}

// Allocate memory from the suballocator
func cmsSubAlloc(mm mem.Manager, sub *cmsSubAllocator, size uint32) []byte {
	size = uint32(cmsALIGNMEM((uintptr(size))))

	freeSpace := sub.Head.BlockSize - sub.Head.Used
	if size > freeSpace {
		newSize := sub.Head.BlockSize * 2
		if newSize < size {
			newSize = size
		}

		newChunk := cmsCreateSubAllocChunk(mm, sub.ContextID, newSize)
		if newChunk == nil {
			return nil
		}

		newChunk.Next = sub.Head
		sub.Head = newChunk
	}

	ptr := sub.Head.Block[sub.Head.Used:]
	sub.Head.Used += size

	return ptr
}

func cmsSubAllocDup(mm mem.Manager, sub *cmsSubAllocator, ptr any, size uint32) []byte {
	if ptr == nil {
		return nil
	}

	newPtr := cmsSubAlloc(mm, sub, size)
	if newPtr == nil {
		return nil
	}

	switch src := ptr.(type) {
	case []byte:
		copy(newPtr, src)
	case *[]byte:
		copy(newPtr, *src)
	default:
		cmsSignalError(nil, cmsERROR_RANGE, "Unsupported type for duplication")
		return nil
	}

	return newPtr
}

// cmsInstallAllocFunctions copies memory management function pointers from a plug-in to the chunk, taking care of missing routines.

func cmsInstallAllocFunctions(plugin *cmsPluginMemHandler, ptr *cmsMemPluginChunkType) {
	if plugin == nil {
		// Copy the default memory plugin chunk
		*ptr = cmsMemPluginChunk
	} else {
		// Assign custom functions from the plugin
		ptr.MallocPtr = plugin.MallocPtr
		ptr.FreePtr = plugin.FreePtr
		ptr.ReallocPtr = plugin.ReallocPtr

		// Assign default functions for optional fields
		ptr.MallocZeroPtr = cmsMallocZeroDefaultFn
		ptr.CallocPtr = cmsCallocDefaultFn
		ptr.DupPtr = cmsDupDefaultFn

		// Override defaults if provided by the plugin
		if plugin.MallocZeroPtr != nil {
			ptr.MallocZeroPtr = plugin.MallocZeroPtr
		}
		if plugin.CallocPtr != nil {
			ptr.CallocPtr = plugin.CallocPtr
		}
		if plugin.DupPtr != nil {
			ptr.DupPtr = plugin.DupPtr
		}
	}
}

// Pointers to memory manager functions in Context0
var cmsMutexPluginChunk = cmsMutexPluginChunkType{CreateMutexPtr: defMtxCreate, DestroyMutexPtr: defMtxDestroy, LockMutexPtr: defMtxLock, UnlockMutexPtr: defMtxUnlock}

// Equivalent of defMtxCreate
func defMtxCreate() *cmsMutex {
	ptr_mutex := NewCmsMutex()
	cmsInitMutexPrimitive(ptr_mutex)
	return ptr_mutex
}

// Equivalent of defMtxDestroy
func defMtxDestroy(mtx *cmsMutex) {
	// In Go, there's no need for explicit destruction of Mutex.
	// We simply stop using it, and garbage collection will clean it up.
	cmsDestroyMutexPrimitive((mtx))
}

// Equivalent of defMtxLock
func defMtxLock(mtx *cmsMutex) bool {
	cmsLockPrimitive((mtx))
	return true // Always returns true in Go since mutex locking doesn't fail.
}

// Equivalent of defMtxUnlock
func defMtxUnlock(mtx *cmsMutex) {
	cmsUnlockPrimitive((mtx))
}

func cmsRegisterMutexPlugin(ContextID CmsContext, Data PluginIntrfc) bool {
	ctx, ok := CmsContextGetClientChunk(ContextID, MutexPlugin).(*cmsMutexPluginChunkType)
	if !ok {
		cmsSignalError(nil, cmsERROR_UNDEFINED, " Interface data assertion error, not cmsMutexPluginChunkType\n")
		return false
	}
	// If Data is nil, reset the mutex pointers to nil and return true.
	if Data == nil {
		ctx.CreateMutexPtr = nil
		ctx.DestroyMutexPtr = nil
		ctx.LockMutexPtr = nil
		ctx.UnlockMutexPtr = nil
		return true
	}

	plugin, ok := Data.(*cmsPluginMutex)
	if !ok {
		panic(" Plugin is not of the type cmsPluginMutex\n")

	}
	// Ensure all required callback functions are provided.
	if plugin.CreateMutexPtr == nil || plugin.DestroyMutexPtr == nil ||
		plugin.LockMutexPtr == nil || plugin.UnlockMutexPtr == nil {
		return false
	}

	// Set the mutex function pointers.
	ctx.CreateMutexPtr = plugin.CreateMutexPtr
	ctx.DestroyMutexPtr = plugin.DestroyMutexPtr
	ctx.LockMutexPtr = plugin.LockMutexPtr
	ctx.UnlockMutexPtr = plugin.UnlockMutexPtr

	// All is ok.
	return true
}

var cmsParallelizationPluginChunk = cmsParallelizationPluginChunkType{}

// Register parallel processing plugin.
func cmsRegisterParallelizationPlugin(ContextID CmsContext, Data any) bool {
	// If Data is nil, reset to default.

	Plugin, ok := Data.(*cmsPluginParalellization)
	if !ok {
		cmsSignalError(nil, cmsERROR_UNDEFINED, "Interface data assertion error, not cmsPluginParalellization\n")
		return false
	}
	ctx, ok := CmsContextGetClientChunk(ContextID, ParallelizationPlugin).(*cmsParallelizationPluginChunkType)
	if !ok {
		cmsSignalError(nil, cmsERROR_UNDEFINED, "Interface data assertion error not cmsParallelizationPluginChunkType\n")
		return false
	}
	if Data == nil {
		ctx.MaxWorkers = 0
		ctx.WorkerFlags = 0
		ctx.SchedulerFn = nil
		return true
	}

	// Check if the Scheduler function is provided.
	if Plugin.SchedulerFn == nil {
		return false
	}

	// Update the context with the plugin details.
	ctx.MaxWorkers = Plugin.MaxWorkers
	ctx.WorkerFlags = int32(Plugin.WorkerFlags)
	ctx.SchedulerFn = Plugin.SchedulerFn
	return true
}

// Generic Mutex fns
// Create a new mutex.
func cmsCreateMutex(ContextID CmsContext) *cmsMutex {

	ptr, ok := CmsContextGetClientChunk(ContextID, MutexPlugin).(*cmsMutexPluginChunkType)
	if !ok {
		cmsSignalError(nil, cmsERROR_UNDEFINED, "Interface data assertion error, not cmsMutexPluginChunkType\n")
		return nil
	}
	if ptr.CreateMutexPtr == nil {
		return nil
	}

	return ptr.CreateMutexPtr()
}

// Destroy a mutex.
func cmsDestroyMutex(ContextID CmsContext, mtx *cmsMutex) {

	ptr, ok := CmsContextGetClientChunk(ContextID, MutexPlugin).(*cmsMutexPluginChunkType)
	if !ok {
		cmsSignalError(nil, cmsERROR_UNDEFINED, "Interface data assertion error, not cmsMutexPluginChunkType\n")
	}
	if ptr.DestroyMutexPtr != nil {

		ptr.DestroyMutexPtr(mtx)
	}
}

// Lock the mutex.
func cmsLockMutex(ContextID CmsContext, mtx *cmsMutex) bool {

	ptr, ok := CmsContextGetClientChunk(ContextID, MutexPlugin).(*cmsMutexPluginChunkType)
	if !ok {
		cmsSignalError(nil, cmsERROR_UNDEFINED, "Interface data assertion error, not cmsMutexPluginChunkType\n")
		return false
	}
	if ptr.LockMutexPtr == nil {
		return true
	}

	return ptr.LockMutexPtr(mtx)
}

// Unlock the mutex.
func cmsUnlockMutex(ContextID CmsContext, mtx *cmsMutex) {
	ptr, ok := CmsContextGetClientChunk(ContextID, MutexPlugin).(*cmsMutexPluginChunkType)
	if !ok {
		cmsSignalError(nil, cmsERROR_UNDEFINED, "Interface data assertion error, not cmsMutexPluginChunkType\n")
	}
	if ptr.UnlockMutexPtr != nil {

		ptr.UnlockMutexPtr(mtx)
	}
}

func cmsTagSignature2String(sig cmsTagSignature) string {
	return string([]byte{
		byte(sig >> 24),
		byte(sig >> 16),
		byte(sig >> 8),
		byte(sig),
	})
}

//lint:ignore U1000 kept for parity with lcms; used in future ports
func cmsstrcasecmp(s1, s2 *byte) int {
	// Convert *byte pointers into slices to traverse
	us1 := unsafe.Slice(s1, cmsMAX_PATH)
	us2 := unsafe.Slice(s2, cmsMAX_PATH)

	for i := 0; i < len(us1) && i < len(us2); i++ {
		// Convert to uppercase for case-insensitive comparison
		c1 := byte(unicode.ToUpper(rune(us1[i])))
		c2 := byte(unicode.ToUpper(rune(us2[i])))

		if c1 != c2 {
			return int(c1) - int(c2)
		}

		// Break on null terminator
		if us1[i] == 0 {
			return 0
		}
	}

	return 0
}

// Convert []float32 to []byte

/*func float32SliceToBytes(floats []float32) []byte {
	buf := new(bytes.Buffer)
	for _, f := range floats {
		binary.Write(buf, binary.LittleEndian, f)
	}
	return buf.Bytes()
}

func float64SliceToBytes(floats []float64) []byte {
	buf := new(bytes.Buffer)
	for _, f := range floats {
		binary.Write(buf, binary.LittleEndian, f)
	}
	return buf.Bytes()
}

func uint16SliceToBytes(ints []uint16) []byte {
	buf := new(bytes.Buffer)
	for _, i := range ints {
		binary.Write(buf, binary.LittleEndian, i)
	}
	return buf.Bytes()
}

func bytesToUint16Slice(b []uint8) []uint16 {
	if len(b)%2 != 0 {
		return nil
	}
	u16 := make([]uint16, len(b)/2)
	for i := 0; i < len(u16); i++ {
		u16[i] = binary.LittleEndian.Uint16(b[i*2 : i*2+2])
	}
	return u16
}

func bytesToLab(b []byte) cmsCIELab {
	var lab cmsCIELab
	buf := bytes.NewReader(b)
	binary.Read(buf, binary.LittleEndian, &lab.L)
	binary.Read(buf, binary.LittleEndian, &lab.a)
	binary.Read(buf, binary.LittleEndian, &lab.b)
	return lab
}*/
// Fill dst from b (little-endian). Returns number of elements written.
func writeIntoUint16Slice(dst []uint16, b []byte) int {
	n := len(b) / 2
	if n > len(dst) {
		n = len(dst)
	}
	off := 0
	for i := 0; i < n; i++ {
		dst[i] = uint16(b[off]) | uint16(b[off+1])<<8
		off += 2
	}
	return n
}

func writeIntoFloat32Slice(dst []float32, b []byte) int {
	n := len(b) / 4
	if n > len(dst) {
		n = len(dst)
	}
	off := 0
	for i := 0; i < n; i++ {
		u := uint32(b[off]) |
			uint32(b[off+1])<<8 |
			uint32(b[off+2])<<16 |
			uint32(b[off+3])<<24
		dst[i] = math.Float32frombits(u)
		off += 4
	}
	return n
}

func writeIntoFloat64Slice(dst []float64, b []byte) int {
	n := len(b) / 8
	if n > len(dst) {
		n = len(dst)
	}
	off := 0
	for i := 0; i < n; i++ {
		u := uint64(b[off]) |
			uint64(b[off+1])<<8 |
			uint64(b[off+2])<<16 |
			uint64(b[off+3])<<24 |
			uint64(b[off+4])<<32 |
			uint64(b[off+5])<<40 |
			uint64(b[off+6])<<48 |
			uint64(b[off+7])<<56
		dst[i] = math.Float64frombits(u)
		off += 8
	}
	return n
}

// -------- bytes -> slices (allocate-return) --------

func BytesToUint16sLE(b []byte) []uint16 {
	n := len(b) >> 1
	out := make([]uint16, n)
	off := 0
	for i := 0; i < n; i++ {
		out[i] = uint16(b[off]) | uint16(b[off+1])<<8
		off += 2
	}
	return out
}

func BytesToFloat32sLE(b []byte) []float32 {
	n := len(b) >> 2
	out := make([]float32, n)
	off := 0
	for i := 0; i < n; i++ {
		u := uint32(b[off]) |
			uint32(b[off+1])<<8 |
			uint32(b[off+2])<<16 |
			uint32(b[off+3])<<24
		out[i] = math.Float32frombits(u)
		off += 4
	}
	return out
}

func BytesToFloat64sLE(b []byte) []float64 {
	n := len(b) >> 3
	out := make([]float64, n)
	off := 0
	for i := 0; i < n; i++ {
		u := uint64(b[off]) |
			uint64(b[off+1])<<8 | uint64(b[off+2])<<16 | uint64(b[off+3])<<24 |
			uint64(b[off+4])<<32 | uint64(b[off+5])<<40 | uint64(b[off+6])<<48 | uint64(b[off+7])<<56
		out[i] = math.Float64frombits(u)
		off += 8
	}
	return out
}

// -------- slices -> bytes (allocate-return) --------

func Uint16sToBytesLE(src []uint16) []byte {
	out := make([]byte, len(src)*2)
	off := 0
	for _, v := range src {
		out[off] = byte(v)
		out[off+1] = byte(v >> 8)
		off += 2
	}
	return out
}

func Float32sToBytesLE(src []float32) []byte {
	out := make([]byte, len(src)*4)
	off := 0
	for _, f := range src {
		u := math.Float32bits(f)
		out[off] = byte(u)
		out[off+1] = byte(u >> 8)
		out[off+2] = byte(u >> 16)
		out[off+3] = byte(u >> 24)
		off += 4
	}
	return out
}

func Float64sToBytesLE(src []float64) []byte {
	out := make([]byte, len(src)*8)
	off := 0
	for _, f := range src {
		u := math.Float64bits(f)
		out[off] = byte(u)
		out[off+1] = byte(u >> 8)
		out[off+2] = byte(u >> 16)
		out[off+3] = byte(u >> 24)
		out[off+4] = byte(u >> 32)
		out[off+5] = byte(u >> 40)
		out[off+6] = byte(u >> 48)
		out[off+7] = byte(u >> 56)
		off += 8
	}
	return out
}

func bytesToLab(b []byte) cmsCIELab {
	// assumes len(b) >= 24 (3 * float64)
	var lab cmsCIELab
	// L
	u0 := uint64(b[0]) | uint64(b[1])<<8 | uint64(b[2])<<16 | uint64(b[3])<<24 |
		uint64(b[4])<<32 | uint64(b[5])<<40 | uint64(b[6])<<48 | uint64(b[7])<<56
	// a
	u1 := uint64(b[8]) | uint64(b[9])<<8 | uint64(b[10])<<16 | uint64(b[11])<<24 |
		uint64(b[12])<<32 | uint64(b[13])<<40 | uint64(b[14])<<48 | uint64(b[15])<<56
	// b
	u2 := uint64(b[16]) | uint64(b[17])<<8 | uint64(b[18])<<16 | uint64(b[19])<<24 |
		uint64(b[20])<<32 | uint64(b[21])<<40 | uint64(b[22])<<48 | uint64(b[23])<<56

	lab.L = math.Float64frombits(u0)
	lab.a = math.Float64frombits(u1)
	lab.b = math.Float64frombits(u2)
	return lab
}
