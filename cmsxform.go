package golcms

import(
	"unsafe"
)
// Transformations stuff
// -----------------------------------------------------------------------


// Constants
const DEFAULT_OBSERVER_ADAPTATION_STATE = 1.0

// Global variable: The Context0 observer adaptation state.
var _cmsAdaptationStateChunk = cmsAdaptationStateChunkType{
	AdaptationState: DEFAULT_OBSERVER_ADAPTATION_STATE,
}

// _cmsAllocAdaptationStateChunk initializes and duplicates the observer adaptation state.
func _cmsAllocAdaptationStateChunk(ctx *cmsContextStruct, src *cmsContextStruct) {
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

	ctx.Chunks[AdaptationStateContext] = cmsSubAllocDup(ctx.MemPool, from, cmsUInt32Number(unsafe.Sizeof(cmsAdaptationStateChunkType{})))
}
