package golcms

import "C"
import (
	"fmt"
	"math"
	"sync"
	"unicode"
	"unsafe"
	//"syscall"
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
func cmsSignalError(id unsafe.Pointer, code int, message string) {
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
func allocateMemory(size uintptr) unsafe.Pointer {
	if size == 0 {
		return nil
	}

	// Allocate memory manually using a Go slice and return its pointer.
	mem := make([]byte, size)
	return unsafe.Pointer(&mem[0])
}

// freeMemory frees manually allocated memory. (No-op in Go)
func freeMemory(ptr unsafe.Pointer, size uintptr) {
	// Memory will be garbage collected, but this function can be used for compatibility.
}

// accessMemory allows accessing memory at an offset from a base pointer.
func accessMemory(base unsafe.Pointer, offset uintptr) unsafe.Pointer {
	return unsafe.Pointer(uintptr(base) + offset)
}

// Default memory allocation function
func cmsMallocDefaultFn(ContextID CmsContext, size uint32) unsafe.Pointer {
	return allocateMemory(uintptr(size))
}

// Generic allocate & zero
func cmsMallocZeroDefaultFn(ContextID CmsContext, size uint32) unsafe.Pointer {
	ptr := allocateMemory(uintptr(size))
	if ptr == nil {
		return nil
	}
	mem := (*[1 << 30]byte)(ptr)[:size:size]
	for i := range mem {
		mem[i] = 0
	}
	return ptr
}

// Default free function
func cmsFreeDefaultFn(ContextID CmsContext, Ptr unsafe.Pointer, size uint32) {
	freeMemory(Ptr, uintptr(size))
}

// Default realloc function
func cmsReallocDefaultFn(ContextID CmsContext, Ptr unsafe.Pointer, newSize uint32, oldSize uint32) unsafe.Pointer {
	if newSize > MAX_MEMORY_FOR_ALLOC {
		return nil
	}
	newPtr := allocateMemory(uintptr(newSize))
	if newPtr == nil {
		return nil
	}
	if Ptr != nil {
		src := (*[1 << 30]byte)(Ptr)[:oldSize:oldSize]
		dst := (*[1 << 30]byte)(newPtr)[:newSize:newSize]
		copy(dst, src)
		freeMemory(Ptr, uintptr(oldSize)) //need to know old size, but that would spoil function  prototype!
	}
	return newPtr
}

// Default calloc function
func cmsCallocDefaultFn(ContextID CmsContext, num, size uint32) unsafe.Pointer {
	total := uint64(num) * uint64(size)
	if total == 0 || total > MAX_MEMORY_FOR_ALLOC || num > math.MaxUint32/size {
		return nil
	}
	return cmsMallocZeroDefaultFn(ContextID, uint32(total))
}

// Generic block duplication
func cmsDupDefaultFn(ContextID CmsContext, Org unsafe.Pointer, size uint32) unsafe.Pointer {
	if size > MAX_MEMORY_FOR_ALLOC {
		return nil
	}
	mem := allocateMemory(uintptr(size))
	if mem != nil && Org != nil {
		src := (*[1 << 30]byte)(Org)[:size:size]
		dst := (*[1 << 30]byte)(mem)[:size:size]
		copy(dst, src)
	}
	return mem
}

// Pointers to memory manager functions in Context0
var cmsMemPluginChunk = cmsMemPluginChunkType{cmsMallocDefaultFn, cmsMallocZeroDefaultFn, cmsFreeDefaultFn,
	cmsReallocDefaultFn, cmsCallocDefaultFn, cmsDupDefaultFn}

// Plug-in replacement entry
func cmsRegisterMemHandlerPlugin(context CmsContext, data *cmsPluginBase) bool {
	plugin := (*cmsPluginMemHandler)(unsafe.Pointer(data))
	var ptr *cmsMemPluginChunkType

	// NULL forces reset to defaults
	// NULL forces to reset to defaults. In this special case, the defaults are stored in the context structure.
	// Remaining plug-ins does NOT have any copy in the context structure, but this is somehow special as the
	// context internal data should be malloce'd by using those functions.
	if data == nil {
		ctx := (*CmsContextStruct)(unsafe.Pointer(context))

		// Return to the default allocators
		if context != nil {
			ctx.chunks[MemPlugin] = unsafe.Pointer(&ctx.DefaultMemoryManager)
		}
		return true
	}

	// Check for required callbacks
	if plugin.MallocPtr == nil || plugin.FreePtr == nil || plugin.ReallocPtr == nil {
		return false
	}

	// Set replacement functions
	ptr = (*cmsMemPluginChunkType)(CmsContextGetClientChunk(context, MemPlugin))
	if ptr == nil {
		return false
	}

	cmsInstallAllocFunctions(plugin, ptr)
	return true
}

// Generic allocate
func cmsMalloc(contextID CmsContext, size uint32) unsafe.Pointer {
	ptr := (*cmsMemPluginChunkType)(CmsContextGetClientChunk(contextID, MemPlugin)) // Assume 0 is the MemPlugin index
	if ptr == nil || ptr.MallocPtr == nil {
		return nil
	}
	return ptr.MallocPtr(contextID, size)
}

// Generic allocate & zero
func cmsMallocZero(contextID CmsContext, size uint32) unsafe.Pointer {
	ptr := (*cmsMemPluginChunkType)(CmsContextGetClientChunk(contextID, MemPlugin))
	/*	if ptr == nil || ptr.MallocZeroPtr == nil {
		return nil
	}*/
	return ptr.MallocZeroPtr(contextID, size)
}

// Generic calloc
func cmsCalloc(contextID CmsContext, num, size uint32) unsafe.Pointer {
	ptr := (*cmsMemPluginChunkType)(CmsContextGetClientChunk(contextID, MemPlugin))
	if ptr == nil || ptr.CallocPtr == nil {
		return nil
	}
	return ptr.CallocPtr(contextID, num, size)
}

// Generic reallocate
func cmsRealloc(contextID CmsContext, oldPtr unsafe.Pointer, size uint32) unsafe.Pointer {
	ptr := (*cmsMemPluginChunkType)(CmsContextGetClientChunk(contextID, MemPlugin))
	if ptr == nil || ptr.ReallocPtr == nil {
		return nil
	}
	return ptr.ReallocPtr(contextID, oldPtr, size, size)
}

// Generic free memory
func cmsFree(contextID CmsContext, oldPtr unsafe.Pointer) {
	if oldPtr != nil {
		ptr := (*cmsMemPluginChunkType)(CmsContextGetClientChunk(contextID, MemPlugin))
		if ptr != nil && ptr.FreePtr != nil {
			ptr.FreePtr(contextID, oldPtr, 0) //have to thing about freeing memory and size variable
		}
	}
}

// Generic block duplication
func cmsDupMem(contextID CmsContext, org unsafe.Pointer, size uint32) unsafe.Pointer {
	ptr := (*cmsMemPluginChunkType)(CmsContextGetClientChunk(contextID, MemPlugin))
	if ptr == nil || ptr.DupPtr == nil || org == nil {
		return nil
	}
	return ptr.DupPtr(contextID, org, size)
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

	chunk := (*cmsSubAllocatorChunk)(cmsMallocZero(contextID, uint32(unsafe.Sizeof(cmsSubAllocatorChunk{}))))
	if chunk == nil {
		return nil
	}

	chunk.Block = (*uint8)(cmsMalloc(contextID, initial))
	if chunk.Block == nil {
		cmsFree(contextID, unsafe.Pointer(chunk))
		return nil
	}

	chunk.BlockSize = initial
	chunk.Used = 0
	chunk.Next = nil

	return chunk
}

// Create a new suballocator
func cmsCreateSubAlloc(contextID CmsContext, initial uint32) *cmsSubAllocator {
	sub := (*cmsSubAllocator)(cmsMallocZero(contextID, uint32(unsafe.Sizeof(cmsSubAllocator{}))))
	if sub == nil {
		return nil
	}

	sub.ContextID = (CmsContext)(contextID)
	sub.Head = cmsCreateSubAllocChunk(contextID, initial)
	if sub.Head == nil {
		cmsFree(contextID, unsafe.Pointer(sub))
		return nil
	}

	return sub
}

// Destroy the suballocator and free all associated memory
func cmsSubAllocDestroy(sub *cmsSubAllocator) {
	for chunk := sub.Head; chunk != nil; {
		next := chunk.Next
		if chunk.Block != nil {
			cmsFree(sub.ContextID, unsafe.Pointer(chunk.Block))
		}
		cmsFree(sub.ContextID, unsafe.Pointer(chunk))
		chunk = next
	}

	cmsFree(sub.ContextID, unsafe.Pointer(sub))
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

// Duplicate memory into the suballocator
func cmsSubAllocDup(sub *cmsSubAllocator, ptr unsafe.Pointer, size uint32) unsafe.Pointer {
	if ptr == nil {
		return nil
	}

	newPtr := cmsSubAlloc(sub, size)
	if newPtr != nil && ptr != nil {
		memcpy(newPtr, ptr, uintptr(size))
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

// Define a Mutex wrapper to emulate the behavior of _cmsMutex
type MutexWrapper struct {
	mutex sync.Mutex
}

// Equivalent of defMtxCreate
func defMtxCreate() unsafe.Pointer {
	return unsafe.Pointer(&MutexWrapper{})
}

// Equivalent of defMtxDestroy
func defMtxDestroy(mtx unsafe.Pointer) {
	// In Go, there's no need for explicit destruction of Mutex.
	// We simply stop using it, and garbage collection will clean it up.
}

// Equivalent of defMtxLock
func defMtxLock(mtx unsafe.Pointer) bool {
	((*MutexWrapper)(mtx)).mutex.Lock()
	return true // Always returns true in Go since mutex locking doesn't fail.
}

// Equivalent of defMtxUnlock
func defMtxUnlock(mtx unsafe.Pointer) {
	((*MutexWrapper)(mtx)).mutex.Unlock()
}

func cmsRegisterMutexPlugin(ContextID CmsContext, Data *cmsPluginBase) bool {
	ctx := (*cmsMutexPluginChunkType)(CmsContextGetClientChunk(ContextID, MutexPlugin))
	Plugin := (*cmsPluginMutex)(unsafe.Pointer(Data))

	// If Data is nil, reset the mutex pointers to nil and return true.
	if Data == nil {
		ctx.CreateMutexPtr = nil
		ctx.DestroyMutexPtr = nil
		ctx.LockMutexPtr = nil
		ctx.UnlockMutexPtr = nil
		return true
	}

	// Ensure all required callback functions are provided.
	if Plugin.CreateMutexPtr == nil || Plugin.DestroyMutexPtr == nil ||
		Plugin.LockMutexPtr == nil || Plugin.UnlockMutexPtr == nil {
		return false
	}

	// Set the mutex function pointers.
	ctx.CreateMutexPtr = Plugin.CreateMutexPtr
	ctx.DestroyMutexPtr = Plugin.DestroyMutexPtr
	ctx.LockMutexPtr = Plugin.LockMutexPtr
	ctx.UnlockMutexPtr = Plugin.UnlockMutexPtr

	// All is ok.
	return true
}

var cmsParallelizationPluginChunk = cmsParallelizationPluginChunkType{}

// Register parallel processing plugin.
func cmsRegisterParallelizationPlugin(ContextID CmsContext, Data unsafe.Pointer) bool {
	Plugin := (*cmsPluginParalellization)(Data)
	ctx := (*cmsParallelizationPluginChunkType)(CmsContextGetClientChunk(ContextID, ParallelizationPlugin))

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
func cmsCreateMutex(ContextID CmsContext) unsafe.Pointer {

	ptr := (*cmsMutexPluginChunkType)(CmsContextGetClientChunk(ContextID, MutexPlugin))

	if ptr.CreateMutexPtr == nil {
		return nil
	}

	return ptr.CreateMutexPtr()
}

// Destroy a mutex.
func cmsDestroyMutex(ContextID CmsContext, mtx unsafe.Pointer) {

	ptr := (*cmsMutexPluginChunkType)(CmsContextGetClientChunk(ContextID, MutexPlugin))

	if ptr.DestroyMutexPtr != nil {

		ptr.DestroyMutexPtr(mtx)
	}
}

// Lock the mutex.
func cmsLockMutex(ContextID CmsContext, mtx unsafe.Pointer) bool {

	ptr := (*cmsMutexPluginChunkType)(CmsContextGetClientChunk(ContextID, MutexPlugin))

	if ptr.LockMutexPtr == nil {
		return true
	}

	return ptr.LockMutexPtr(mtx)
}

// Unlock the mutex.
func cmsUnlockMutex(ContextID CmsContext, mtx unsafe.Pointer) {
	ptr := (*cmsMutexPluginChunkType)(CmsContextGetClientChunk(ContextID, MutexPlugin))

	if ptr.UnlockMutexPtr != nil {

		ptr.UnlockMutexPtr(mtx)
	}
}

// Mutex for thread safety.
var globalMutex sync.Mutex

// Allocate and initialize mutex container.
func cmsAllocMutexPluginChunk(ctx *CmsContextStruct, src *CmsContextStruct) {
	globalMutex.Lock()
	defer globalMutex.Unlock()

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
	globalMutex.Lock()
	defer globalMutex.Unlock()

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
}

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
