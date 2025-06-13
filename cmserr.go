package golcms

import "C"
import (
	"fmt"
	"math"
	"unicode"
	"unsafe"
	"encoding/binary"

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
func cmsSignalError(id interface{}, code int, message string) {
	fmt.Printf("Error: %s (code %d)\n", message, code)
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

func allocateStruct[T any]() *T {
	return new(T) // Allocates and returns a pointer to type T
}

// freeMemory frees manually allocated memory. (No-op in Go)
func freeMemory(ptr interface{}, size uintptr) {
	// Memory will be garbage collected, but this function can be used for compatibility.
}

// accessMemory allows accessing memory at an offset from a base pointer.
/*func accessMemory(base interface{}, offset uintptr) interface{} {
	return unsafe.Pointer(unsafe.Pointer(base) + offset)
}*/

// Default memory allocation function
func cmsMallocDefaultFn(ContextID CmsContext, size uint32) interface{} {
	return allocateMemory(uintptr(size))
}

// Generic allocate & zero
func cmsMallocZeroDefaultFn(ContextID CmsContext, size uint32) interface{} {
	ptr := allocateMemory(uintptr(size))
	if ptr == nil {
		return nil
	}
	return ptr
}

// Default free function
func cmsFreeDefaultFn(ContextID CmsContext, ptr interface{}, size uint32) {
	freeMemory(ptr, uintptr(size))
}

// Default realloc function
func cmsReallocDefaultFn(ContextID CmsContext, ptr interface{}, newSize uint32, oldSize uint32) interface{} {
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
func cmsCallocDefaultFn(ContextID CmsContext, num, size uint32) interface{} {
	total := uint64(num) * uint64(size)
	if total == 0 || total > MAX_MEMORY_FOR_ALLOC || num > math.MaxUint32/size {
		return nil
	}
	return cmsMallocZeroDefaultFn(ContextID, uint32(total))
}
func cmsDupDefaultFn(ContextID CmsContext, Org interface{}, size uint32) interface{} {
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
func cmsRegisterMemHandlerPlugin(context CmsContext, Data PluginIntrfc) bool {
	//plugin := (*cmsPluginMemHandler)(unsafe.Pointer(data))
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
		fmt.Printf("Error: Plugin is not of the type cmsPluginMemHandler\n")
		return false
	}
	// Check for required callbacks
	if plugin.MallocPtr == nil || plugin.FreePtr == nil || plugin.ReallocPtr == nil {
		return false
	}

	// Set replacement functions
	ptr = CmsContextGetClientChunk(context, MemPlugin).(*cmsMemPluginChunkType)
	if ptr == nil {
		return false
	}

	cmsInstallAllocFunctions(plugin, ptr)
	return true
}

// Generic allocate
func cmsMalloc(contextID CmsContext, size uint32) interface{} {
	ptr := CmsContextGetClientChunk(contextID, MemPlugin).(*cmsMemPluginChunkType) // Assume 0 is the MemPlugin index
	if ptr == nil || ptr.MallocPtr == nil {
		return nil
	}
	return ptr.MallocPtr(contextID, size)
}

// Generic allocate & zero
/*func cmsMallocZero(contextID CmsContext, size uint32) unsafe.Pointer {
	ptr := (*cmsMemPluginChunkType)(CmsContextGetClientChunk(contextID, MemPlugin))
	return ptr.MallocZeroPtr(contextID, size)
}*/

// Generic calloc
func cmsCalloc(contextID CmsContext, num, size uint32) interface{} {
	ptr := CmsContextGetClientChunk(contextID, MemPlugin).(*cmsMemPluginChunkType)
	if ptr == nil || ptr.CallocPtr == nil {
		return nil
	}
	return ptr.CallocPtr(contextID, num, size)
}

// Generic reallocate
func cmsRealloc(contextID CmsContext, oldPtr interface{}, size uint32) interface{} {
	ptr := CmsContextGetClientChunk(contextID, MemPlugin).(*cmsMemPluginChunkType)
	if ptr == nil || ptr.ReallocPtr == nil {
		return nil
	}
	return ptr.ReallocPtr(contextID, oldPtr, size, size)
}

// Generic free memory
func cmsFree(contextID CmsContext, oldPtr interface{}) {
	if oldPtr != nil {
		ptr := CmsContextGetClientChunk(contextID, MemPlugin).(*cmsMemPluginChunkType)
		if ptr != nil && ptr.FreePtr != nil {
			ptr.FreePtr(contextID, oldPtr, 0) //have to thing about freeing memory and size variable
		}
	}
}

// Generic block duplication for structures
func cmsDupMem(contextID CmsContext, org interface{}, size uint32) interface{} {
	ptr := CmsContextGetClientChunk(contextID, MemPlugin).(*cmsMemPluginChunkType)
	if ptr == nil || ptr.DupPtr == nil || org == nil {
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
func cmsCreateSubAllocChunk(contextID CmsContext, initial uint32) *cmsSubAllocatorChunk {
	if initial == 0 {
		initial = 20 * 1024 // Default to 20KB
	}

	chunk := allocateStruct[cmsSubAllocatorChunk]()
	if chunk == nil {
		return nil
	}

	chunk.Block = cmsMalloc(contextID, initial).(*uint8)
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
func cmsCreateSubAlloc(contextID CmsContext, initial uint32) *cmsSubAllocator {
	sub := allocateStruct[cmsSubAllocator]()
	if sub == nil {
		return nil
	}

	sub.ContextID = (CmsContext)(contextID)
	sub.Head = cmsCreateSubAllocChunk(contextID, initial)
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
func cmsSubAlloc(sub *cmsSubAllocator, size uint32) unsafe.Pointer {
	size = uint32(cmsALIGNMEM((uintptr(size))))

	freeSpace := sub.Head.BlockSize - sub.Head.Used
	if size > freeSpace {
		newSize := sub.Head.BlockSize * 2
		if newSize < size {
			newSize = size
		}

		newChunk := cmsCreateSubAllocChunk(sub.ContextID, newSize)
		if newChunk == nil {
			return nil
		}

		newChunk.Next = sub.Head
		sub.Head = newChunk
	}

	ptr := unsafe.Pointer(uintptr(unsafe.Pointer(sub.Head.Block)) + uintptr(sub.Head.Used))
	sub.Head.Used += size

	return ptr
}

func cmsSubAllocDup(sub *cmsSubAllocator, ptr interface{}, size uint32) interface{} {
	if ptr == nil {
		return nil
	}

	newPtr := cmsSubAlloc(sub, size)
	if newPtr == nil {
		return nil
	}

	dest := unsafe.Slice((*byte)(newPtr), size)

	switch src := ptr.(type) {
	case []byte:
		copy(dest, src)
	case *[]byte:
		copy(dest, *src)
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
	ctx := CmsContextGetClientChunk(ContextID, MutexPlugin).(*cmsMutexPluginChunkType)
	//Plugin := (*cmsPluginMutex)(unsafe.Pointer(Data))

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
		fmt.Printf("Error: Plugin is not of the type cmsPluginMutex\n")
		return false
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
func cmsRegisterParallelizationPlugin(ContextID CmsContext, Data interface{}) bool {
	Plugin := Data.(*cmsPluginParalellization)
	ctx := CmsContextGetClientChunk(ContextID, ParallelizationPlugin).(*cmsParallelizationPluginChunkType)

	// If Data is nil, reset to default.
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

	ptr := CmsContextGetClientChunk(ContextID, MutexPlugin).(*cmsMutexPluginChunkType)

	if ptr.CreateMutexPtr == nil {
		return nil
	}

	return ptr.CreateMutexPtr()
}

// Destroy a mutex.
func cmsDestroyMutex(ContextID CmsContext, mtx *cmsMutex) {

	ptr := CmsContextGetClientChunk(ContextID, MutexPlugin).(*cmsMutexPluginChunkType)

	if ptr.DestroyMutexPtr != nil {

		ptr.DestroyMutexPtr(mtx)
	}
}

// Lock the mutex.
func cmsLockMutex(ContextID CmsContext, mtx *cmsMutex) bool {

	ptr := CmsContextGetClientChunk(ContextID, MutexPlugin).(*cmsMutexPluginChunkType)

	if ptr.LockMutexPtr == nil {
		return true
	}

	return ptr.LockMutexPtr(mtx)
}

// Unlock the mutex.
func cmsUnlockMutex(ContextID CmsContext, mtx *cmsMutex) {
	ptr := CmsContextGetClientChunk(ContextID, MutexPlugin).(*cmsMutexPluginChunkType)

	if ptr.UnlockMutexPtr != nil {

		ptr.UnlockMutexPtr(mtx)
	}
}

// Allocate and initialize mutex container.  ARE UNUSED
/*func cmsAllocMutexPluginChunk(ctx *CmsContextStruct, src *CmsContextStruct) {

	if src != nil {
		// Copy the source mutex plugin chunk.
		srcChunk := (*cmsMutexPluginChunkType)(unsafe.Pointer(src.chunks[MutexPlugin]))
		dstChunk := (*cmsMutexPluginChunkType)(unsafe.Pointer(ctx.chunks[MutexPlugin]))
		*dstChunk = *srcChunk
	} else {
		// Use the global mutex plugin chunk as default.
		dstChunk := (*cmsMutexPluginChunkType)(unsafe.Pointer(ctx.chunks[MutexPlugin]))
		*dstChunk = cmsMutexPluginChunk
	}
}

// Allocate and initialize parallelization container.
func cmsAllocParallelizationPluginChunk(ctx *CmsContextStruct, src *CmsContextStruct) {

	if src != nil {
		// Copy the source parallelization plugin chunk.
		srcChunk := (*cmsParallelizationPluginChunkType)(unsafe.Pointer(src.chunks[ParallelizationPlugin]))
		dstChunk := (*cmsParallelizationPluginChunkType)(unsafe.Pointer(ctx.chunks[ParallelizationPlugin]))
		*dstChunk = *srcChunk
	} else {
		// Use the global parallelization plugin chunk as default.
		dstChunk := (*cmsParallelizationPluginChunkType)(unsafe.Pointer(ctx.chunks[ParallelizationPlugin]))
		*dstChunk = cmsParallelizationPluginChunk
	}
}*/

// Utility function to print signatures
func cmsTagSignature2String(String [5]byte, sig cmsTagSignature) {
	// Convert to big endian
	be := cmsAdjustEndianess32(uint32(sig))

	// Move chars
	memmove(unsafe.Pointer(&String[0]), unsafe.Pointer(&be), 4)

	// Make sure of terminator
	String[4] = 0
}
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
func float32SliceToBytes(floats []float32) []byte {
	size := len(floats) * 4
	return unsafe.Slice((*byte)(unsafe.Pointer(&floats[0])), size)
}

// Convert []float64 to []byte
func float64SliceToBytes(floats []float64) []byte {
	size := len(floats) * 8
	return unsafe.Slice((*byte)(unsafe.Pointer(&floats[0])), size)
}

// Convert []uint16 to []byte
func uint16SliceToBytes(ints []uint16) []byte {
	size := len(ints) * 2
	return unsafe.Slice((*byte)(unsafe.Pointer(&ints[0])), size)
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

/*import (
	"bytes"
	"encoding/binary"
	"fmt"
)
// Convert []float32 to []byte
func float32SliceToBytes(floats []float32) []byte {
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.LittleEndian, floats) // Change to binary.BigEndian if needed
	if err != nil {
		panic("Error converting float32 slice to bytes: " + err.Error())
	}
	return buf.Bytes()
}

// Convert []float64 to []byte
func float64SliceToBytes(floats []float64) []byte {
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.LittleEndian, floats)
	if err != nil {
		panic("Error converting float64 slice to bytes: " + err.Error())
	}
	return buf.Bytes()
}

// Convert []uint16 to []byte
func uint16SliceToBytes(ints []uint16) []byte {
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.LittleEndian, ints)
	if err != nil {
		panic("Error converting uint16 slice to bytes: " + err.Error())
	}
	return buf.Bytes()
}
*/
