package golcms

import (
	"unsafe"
	//"sync"
)

// Transformations stuff
// -----------------------------------------------------------------------

// Constants
const DEFAULT_OBSERVER_ADAPTATION_STATE = 1.0

// Global variable: The Context0 observer adaptation state.
var cmsAdaptationStateChunk = cmsAdaptationStateChunkType{
	AdaptationState: DEFAULT_OBSERVER_ADAPTATION_STATE,
}

// cmsAllocAdaptationStateChunk initializes and duplicates the observer adaptation state.
func cmsAllocAdaptationStateChunk(ctx cmsContext, src cmsContext) {
	// Default adaptation state chunk used when no source is provided.
	defaultAdaptationStateChunk := cmsAdaptationStateChunkType{
		AdaptationState: DEFAULT_OBSERVER_ADAPTATION_STATE,
	}

	var from unsafe.Pointer
	if src != nil {
		from = src.Chunks[AdaptationStateContext]
	} else {
		from = unsafe.Pointer(&defaultAdaptationStateChunk)
	}

	ctx.Chunks[AdaptationStateContext] = cmsSubAllocDup(ctx.MemPool, from, uint32(unsafe.Sizeof(cmsAdaptationStateChunkType{})))
}

// Transform plug-ins ----------------------------------------------------------------------------------------------------

// cmsTransformCollection represents a linked list of transform factories.
type cmsTransformCollection struct {
	Factory  cmsTransform2Factory
	OldXform bool // Indicates if the factory returns transform functions in the old style
	Next     *cmsTransformCollection
}

// cmsTransformPluginChunkType represents the plugin chunk for transform plugins.
type cmsTransformPluginChunkType struct {
	TransformCollection *cmsTransformCollection
}

// Global transform plugin chunk.
var cmsTransformPluginChunk = cmsTransformPluginChunkType{TransformCollection: nil}

// DupPluginTransformList duplicates the transform plugin list for a new context.
func DupPluginTransformList(ctx cmsContext, src cmsContext) {
	var newHead cmsTransformPluginChunkType
	var entry, prev *cmsTransformCollection
	head := (*cmsTransformPluginChunkType)(src.chunks[TransformPlugin])

	if head == nil {
		return
	}

	// Walk the list and copy each node.
	for entry = head.TransformCollection; entry != nil; entry = entry.Next {
		newEntry := (*cmsTransformCollection)(cmsSubAllocDup(ctx.MemPool, unsafe.Pointer(entry), uint32(unsafe.Sizeof(*entry))))
		if newEntry == nil {
			return
		}

		// Maintain order in the linked list.
		newEntry.Next = nil
		if prev != nil {
			prev.Next = newEntry
		}

		prev = newEntry

		if newHead.TransformCollection == nil {
			newHead.TransformCollection = newEntry
		}
	}

	ctx.chunks[TransformPlugin] = cmsSubAllocDup(ctx.MemPool, unsafe.Pointer(&newHead), uint32(unsafe.Sizeof(newHead)))
}

// cmsAllocTransformPluginChunk allocates the transform plugin chunk.
func cmsAllocTransformPluginChunk(ctx cmsContext, src cmsContext) {
	if src != nil {
		DupPluginTransformList(ctx, src)
	} else {
		var defaultChunk cmsTransformPluginChunkType
		ctx.chunks[TransformPlugin] = cmsSubAllocDup(ctx.MemPool, unsafe.Pointer(&defaultChunk), uint32(unsafe.Sizeof(defaultChunk)))
	}
}

// cmsTransform2toTransformAdaptor adapts new-style transforms to the old-style interface.
func cmsTransform2toTransformAdaptor(cmmcargo *cmsTRANSFORM, inputBuffer unsafe.Pointer, outputBuffer unsafe.Pointer, pixelsPerLine, lineCount uint32, stride *cmsStride) {
	var strideIn, strideOut uint32

	cmsHandleExtraChannels(cmmcargo, inputBuffer, outputBuffer, pixelsPerLine, lineCount, stride)

	for i := uint32(0); i < lineCount; i++ {
		accum := unsafe.Pointer(uintptr(inputBuffer) + uintptr(strideIn))
		output := unsafe.Pointer(uintptr(outputBuffer) + uintptr(strideOut))

		cmmcargo.OldXform(cmmcargo, accum, output, pixelsPerLine, stride.BytesPerPlaneIn)

		strideIn += stride.BytesPerLineIn
		strideOut += stride.BytesPerLineOut
	}
}

// cmsRegisterTransformPlugin registers a new transform plugin.
func cmsRegisterTransformPlugin(ContextID cmsContext, Data *cmsPluginBase) bool {
	plugin := (*cmsPluginTransform)(unsafe.Pointer(Data))
	ctx := (*cmsTransformPluginChunkType)(ContextID.chunks[TransformPlugin])

	if Data == nil {
		// Free the chain. Memory is safely freed at exit.
		ctx.TransformCollection = nil
		return true
	}

	// Ensure the factory callback is present.
	if plugin.Factories.Xform == nil {
		return false
	}

	// Allocate memory for the transform collection.
	fl := (*cmsTransformCollection)(cmsPluginMalloc(ContextID, uint32(unsafe.Sizeof(cmsTransformCollection{}))))
	if fl == nil {
		return false
	}

	// Check for old-style transform plugins (pre-version 2.8).
	if plugin.Base.ExpectedVersion < 2080 {
		fl.OldXform = true
	} else {
		fl.OldXform = false
	}

	// Copy the parameters.
	fl.Factory = plugin.Factories.Xform

	// Maintain the linked list.
	fl.Next = ctx.TransformCollection
	ctx.TransformCollection = fl

	return true
}

// SetTransformUserData sets the user-defined data and its cleanup function.
func SetTransformUserData(cmmCargo *cmsTRANSFORM, ptr unsafe.Pointer, freePrivateDataFn cmsFreeUserDataFn) {
	if cmmCargo == nil {
		panic("CMMcargo cannot be nil")
	}

	cmmCargo.UserData = ptr
	cmmCargo.FreeUserData = freePrivateDataFn
}

// GetTransformUserData retrieves the user-defined data.
func GetTransformUserData(cmmCargo *cmsTRANSFORM) unsafe.Pointer {
	if cmmCargo == nil {
		panic("CMMcargo cannot be nil")
	}

	return cmmCargo.UserData
}

// GetTransformFormatters16 retrieves the current 16-bit formatters.
func GetTransformFormatters16(cmmCargo *cmsTRANSFORM) (fromInput, toOutput cmsFormatter16) {
	if cmmCargo == nil {
		panic("CMMcargo cannot be nil")
	}

	return cmmCargo.FromInput, cmmCargo.ToOutput
}

// GetTransformFormattersFloat retrieves the current float formatters.
func GetTransformFormattersFloat(cmmCargo *cmsTRANSFORM) (fromInputFloat, toOutputFloat cmsFormatterFloat) {
	if cmmCargo == nil {
		panic("CMMcargo cannot be nil")
	}

	return cmmCargo.FromInputFloat, cmmCargo.ToOutputFloat
}

// GetTransformFlags retrieves the original flags.
func GetTransformFlags(cmmCargo *cmsTRANSFORM) uint32 {
	if cmmCargo == nil {
		panic("CMMcargo cannot be nil")
	}

	return cmmCargo.DwOriginalFlags
}

// GetTransformWorker retrieves the worker callback for parallelization plugins.
func GetTransformWorker(cmmCargo *cmsTRANSFORM) cmsTransform2Fn {
	if cmmCargo == nil {
		panic("CMMcargo cannot be nil")
	}

	return cmmCargo.Worker
}

// GetTransformMaxWorkers retrieves the maximum number of workers or -1 for auto.
func GetTransformMaxWorkers(cmmCargo *cmsTRANSFORM) int32 {
	if cmmCargo == nil {
		panic("CMMcargo cannot be nil")
	}

	return cmmCargo.MaxWorkers
}

// GetTransformWorkerFlags retrieves the worker flags.
func GetTransformWorkerFlags(cmmCargo *cmsTRANSFORM) uint32 {
	if cmmCargo == nil {
		panic("CMMcargo cannot be nil")
	}

	return cmmCargo.WorkerFlags
}
