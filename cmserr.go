package golcms

import "C"
import (
	"fmt"
	"math"
	"sync"
	"unicode"

	//"reflect"
	"unsafe"
	//"syscall"
)

// cmsSignalError simulates error signaling
func cmsSignalError(id unsafe.Pointer, code int, message string) {
	fmt.Printf("Error: %s (code %d)\n", message, code)
	/* not translated in Go yet
		 // Check for the context, if specified go there. If not, go for the global
	    lhg = (cmsLogErrorChunkType*) cmsContextGetClientChunk(ContextID, Logger);
	    if (lhg ->LogErrorHandler) {
	        lhg ->LogErrorHandler(ContextID, ErrorCode, Buffer);
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
func cmsMallocDefaultFn(ContextID cmsContext, size uint32) unsafe.Pointer {
	return allocateMemory(uintptr(size))
}

// Generic allocate & zero
func cmsMallocZeroDefaultFn(ContextID cmsContext, size uint32) unsafe.Pointer {
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
func cmsFreeDefaultFn(ContextID cmsContext, Ptr unsafe.Pointer, size uint32) {
	freeMemory(Ptr, uintptr(size))
}

// Default realloc function
func cmsReallocDefaultFn(ContextID cmsContext, Ptr unsafe.Pointer, oldSize, newSize uint32) unsafe.Pointer {
	if newSize > MAX_MEMORY_FOR_ALLOC {
		return nil
	}
	newPtr := allocateMemory(uintptr(newSize))
	if newPtr == nil {
		return nil
	}
	if Ptr != nil && oldSize > 0 {
		src := (*[1 << 30]byte)(Ptr)[:oldSize:oldSize]
		dst := (*[1 << 30]byte)(newPtr)[:newSize:newSize]
		copy(dst, src)
		freeMemory(Ptr, uintptr(oldSize))
	}
	return newPtr
}

// Default calloc function
func cmsCallocDefaultFn(ContextID cmsContext, num, size uint32) unsafe.Pointer {
	total := uint64(num) * uint64(size)
	if total == 0 || total > MAX_MEMORY_FOR_ALLOC || num > math.MaxUint32/size {
		return nil
	}
	return cmsMallocZeroDefaultFn(ContextID, uint32(total))
}

// Generic block duplication
func cmsDupDefaultFn(ContextID cmsContext, Org unsafe.Pointer, size uint32) unsafe.Pointer {
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

// Plug-in replacement entry
func cmsRegisterMemHandlerPlugin(context cmsContext, data *cmsPluginBase) bool {
	plugin := (*cmsPluginMemHandler)(unsafe.Pointer(data))
	var ptr *cmsMemPluginChunkType

	// NULL forces reset to defaults
	// NULL forces to reset to defaults. In this special case, the defaults are stored in the context structure.
	// Remaining plug-ins does NOT have any copy in the context structure, but this is somehow special as the
	// context internal data should be malloce'd by using those functions.
	if data == nil {
		ctx := (*cmsContextStruct)(unsafe.Pointer(context))

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
	ptr = (*cmsMemPluginChunkType)(cmsContextGetClientChunk(context, MemPlugin))
	if ptr == nil {
		return false
	}

	cmsInstallAllocFunctions(plugin, ptr)
	return true
}

// Generic allocate
func cmsMalloc(contextID cmsContext, size uint32) unsafe.Pointer {
	ptr := (*cmsMemPluginChunkType)(cmsContextGetClientChunk(contextID, MemPlugin)) // Assume 0 is the MemPlugin index
	if ptr == nil || ptr.MallocPtr == nil {
		return nil
	}
	return ptr.MallocPtr(contextID, size)
}

// Generic allocate & zero
func cmsMallocZero(contextID cmsContext, size uint32) unsafe.Pointer {
	ptr := (*cmsMemPluginChunkType)(cmsContextGetClientChunk(contextID, MemPlugin))
	if ptr == nil || ptr.MallocZeroPtr == nil {
		return nil
	}
	return ptr.MallocZeroPtr(contextID, size)
}

// Generic calloc
func cmsCalloc(contextID cmsContext, num, size uint32) unsafe.Pointer {
	ptr := (*cmsMemPluginChunkType)(cmsContextGetClientChunk(contextID, MemPlugin))
	if ptr == nil || ptr.CallocPtr == nil {
		return nil
	}
	return ptr.CallocPtr(contextID, num, size)
}

// Generic reallocate
func cmsRealloc(contextID cmsContext, oldPtr unsafe.Pointer, size uint32) unsafe.Pointer {
	ptr := (*cmsMemPluginChunkType)(cmsContextGetClientChunk(contextID, MemPlugin))
	if ptr == nil || ptr.ReallocPtr == nil {
		return nil
	}
	return ptr.ReallocPtr(contextID, oldPtr, size)
}

// Generic free memory
func cmsFree(contextID cmsContext, oldPtr unsafe.Pointer) {
	if oldPtr != nil {
		ptr := (*cmsMemPluginChunkType)(cmsContextGetClientChunk(contextID, MemPlugin))
		if ptr != nil && ptr.FreePtr != nil {
			ptr.FreePtr(contextID, oldPtr)
		}
	}
}

// Generic block duplication
func cmsDupMem(contextID cmsContext, org unsafe.Pointer, size uint32) unsafe.Pointer {
	ptr := (*cmsMemPluginChunkType)(cmsContextGetClientChunk(contextID, MemPlugin))
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
func cmsCreateSubAllocChunk(contextID cmsContext, initial uint32) *cmsSubAllocatorChunk {
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
func cmsCreateSubAlloc(contextID cmsContext, initial uint32) *cmsSubAllocator {
	sub := (*cmsSubAllocator)(cmsMallocZero(contextID, uint32(unsafe.Sizeof(cmsSubAllocator{}))))
	if sub == nil {
		return nil
	}

	sub.ContextID = (cmsContext)(contextID)
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

func cmsRegisterMutexPlugin(ContextID cmsContext, Data *cmsPluginBase) bool {
	ctx := (*cmsMutexPluginChunkType)(cmsContextGetClientChunk(ContextID, MutexPlugin))
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

// Register parallel processing plugin.
func cmsRegisterParallelizationPlugin(ContextID cmsContext, Data unsafe.Pointer) bool {
	Plugin := (*cmsPluginParalellization)(Data)
	ctx := (*cmsParallelizationPluginChunkType)(cmsContextGetClientChunk(ContextID, ParallelizationPlugin))

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

// Mutex for thread safety.
var globalMutex sync.Mutex

// Allocate and initialize mutex container.
func cmsAllocMutexPluginChunk(ctx *cmsContextStruct, src *cmsContextStruct) {
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
func cmsAllocParallelizationPluginChunk(ctx *cmsContextStruct, src *cmsContextStruct) {
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
