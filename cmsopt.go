package golcms

import "unsafe"

// cmsOPToptimizeFn defines the function type for optimizations.

// cmsOptimizationCollection represents a linked list of optimization methods.
type cmsOptimizationCollection struct {
	OptimizePtr cmsOPToptimizeFn
	Next        *cmsOptimizationCollection
}

// cmsOptimizationPluginChunkType represents the plugin chunk for optimizations.
type cmsOptimizationPluginChunkType struct {
	OptimizationCollection *cmsOptimizationCollection
}

// Static array for DefaultOptimization.
var DefaultOptimization = []cmsOptimizationCollection{
	{OptimizePtr: OptimizeByJoiningCurves, Next: &DefaultOptimization[1]},
	{OptimizePtr: OptimizeMatrixShaper, Next: &DefaultOptimization[2]},
	{OptimizePtr: OptimizeByComputingLinearization, Next: &DefaultOptimization[3]},
	{OptimizePtr: OptimizeByResampling, Next: nil},
}

// Global optimization plugin chunk.
var cmsOptimizationPluginChunk = cmsOptimizationPluginChunkType{OptimizationCollection: nil}

// DupPluginOptimizationList duplicates the optimization list for a new context.
func DupPluginOptimizationList(ctx cmsContext, src cmsContext) {
	var newHead cmsOptimizationPluginChunkType
	var entry, prev *cmsOptimizationCollection
	head := (*cmsOptimizationPluginChunkType)(src.chunks[OptimizationPlugin])

	if head == nil {
		return
	}

	// Walk the list and copy each node.
	for entry = head.OptimizationCollection; entry != nil; entry = entry.Next {
		newEntry := (*cmsOptimizationCollection)(cmsSubAllocDup(ctx.MemPool, unsafe.Pointer(entry), uint32(unsafe.Sizeof(*entry))))
		if newEntry == nil {
			return
		}

		// Maintain order in the linked list.
		newEntry.Next = nil
		if prev != nil {
			prev.Next = newEntry
		}

		prev = newEntry

		if newHead.OptimizationCollection == nil {
			newHead.OptimizationCollection = newEntry
		}
	}

	ctx.chunks[OptimizationPlugin] = cmsSubAllocDup(ctx.MemPool, unsafe.Pointer(&newHead), uint32(unsafe.Sizeof(newHead)))
}

// cmsAllocOptimizationPluginChunk allocates the optimization plugin chunk.
func cmsAllocOptimizationPluginChunk(ctx cmsContext, src cmsContext) {
	if src != nil {
		DupPluginOptimizationList(ctx, src)
	} else {
		var defaultChunk cmsOptimizationPluginChunkType
		ctx.chunks[OptimizationPlugin] = cmsSubAllocDup(ctx.MemPool, unsafe.Pointer(&defaultChunk), uint32(unsafe.Sizeof(defaultChunk)))
	}
}

// cmsRegisterOptimizationPlugin registers a new optimization plugin.
func cmsRegisterOptimizationPlugin(ContextID cmsContext, Data *cmsPluginBase) bool {
	plugin := (*cmsPluginOptimization)(unsafe.Pointer(Data))
	ctx := (*cmsOptimizationPluginChunkType)(ContextID.chunks[OptimizationPlugin])
	var newNode *cmsOptimizationCollection

	if Data == nil {
		ctx.OptimizationCollection = nil
		return true
	}

	// Ensure the optimizer callback is present.
	if plugin.OptimizePtr == nil {
		return false
	}

	newNode = (*cmsOptimizationCollection)(cmsPluginMalloc(ContextID, uint32(unsafe.Sizeof(cmsOptimizationCollection{}))))
	if newNode == nil {
		return false
	}

	// Copy parameters and maintain the linked list.
	newNode.OptimizePtr = plugin.OptimizePtr
	newNode.Next = ctx.OptimizationCollection
	ctx.OptimizationCollection = newNode

	return true
}

// cmsOptimizePipeline performs optimizations on a pipeline.
func cmsOptimizePipeline(ContextID cmsContext, PtrLut **cmsPipeline, Intent uint32, InputFormat, OutputFormat, dwFlags *uint32) bool {
	ctx := (*cmsOptimizationPluginChunkType)(ContextID.chunks[OptimizationPlugin])
	var AnySuccess bool
	var mpe *cmsStage

	// A CLUT is being asked, so force this specific optimization.
	if *dwFlags&cmsFLAGS_FORCE_CLUT != 0 {
		PreOptimize(*PtrLut)
		return OptimizeByResampling(PtrLut, Intent, InputFormat, OutputFormat, dwFlags)
	}

	// Check if there's anything to optimize.
	if (*PtrLut).Elements == nil {
		cmsPipelineSetOptimizationParameters(*PtrLut, FastIdentity16, unsafe.Pointer(*PtrLut), nil, nil)
		return true
	}

	// Avoid optimization for named color pipelines.
	for mpe = cmsPipelineGetPtrToFirstStage(*PtrLut); mpe != nil; mpe = cmsStageNext(mpe) {
		if cmsStageType(mpe) == cmsSigNamedColorElemType {
			return false
		}
	}

	// Pre-optimize and check for identity transformations.
	AnySuccess = PreOptimize(*PtrLut)
	if (*PtrLut).Elements == nil {
		cmsPipelineSetOptimizationParameters(*PtrLut, FastIdentity16, unsafe.Pointer(*PtrLut), nil, nil)
		return true
	}

	// Skip optimization if explicitly disabled.
	if *dwFlags&cmsFLAGS_NOOPTIMIZE != 0 {
		return false
	}

	// Try plugin optimizations.
	for opts := ctx.OptimizationCollection; opts != nil; opts = opts.Next {
		if opts.OptimizePtr(PtrLut, Intent, InputFormat, OutputFormat, dwFlags) {
			return true
		}
	}

	// Try built-in optimizations.
	for opts := DefaultOptimization; opts != nil; opts = opts.Next {
		if opts.OptimizePtr(PtrLut, Intent, InputFormat, OutputFormat, dwFlags) {
			return true
		}
	}

	// Only simple optimizations succeeded.
	return AnySuccess
}
