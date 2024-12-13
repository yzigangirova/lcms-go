package golcms

import (
	"unsafe"
)

// Main plug-in entry
func cmsRegisterInterpPlugin(ContextID cmsContext, Data *cmsPluginBase) bool {
    Plugin := (*cmsPluginInterpolation)(unsafe.Pointer(Data))
    ptr := (*cmsInterpPluginChunkType)(cmsContextGetClientChunk(ContextID, InterpPlugin))

    if Data == nil {
        ptr.Interpolators = nil
        return true
    }

    // Set replacement functions
    ptr.Interpolators = Plugin.InterpolatorsFactory
    return true
}
