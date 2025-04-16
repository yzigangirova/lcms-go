package golcms

import (
	//"errors"
	"unsafe"
	//"sync"

	"reflect"
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
func cmsAllocAdaptationStateChunk(ctx CmsContext, src CmsContext) {
	// Default adaptation state chunk used when no source is provided.
	defaultAdaptationStateChunk := cmsAdaptationStateChunkType{
		AdaptationState: DEFAULT_OBSERVER_ADAPTATION_STATE,
	}

	var from unsafe.Pointer
	if src != nil {
		from = src.chunks[AdaptationStateContext]
	} else {
		from = unsafe.Pointer(&defaultAdaptationStateChunk)
	}

	ctx.chunks[AdaptationStateContext] = cmsSubAllocDup(ctx.MemPool, from, uint32(unsafe.Sizeof(cmsAdaptationStateChunkType{})))
}

// Sets adaptation state for absolute colorimetric intent in the given context.  Adaptation state applies on all
// but cmsCreateExtendedTransformTHR().  Little CMS can handle incomplete adaptation states.
func cmsSetAdaptationStateTHR(ContextID CmsContext, d float64) float64 {

	ptr := (*cmsAdaptationStateChunkType)(CmsContextGetClientChunk(ContextID, AdaptationStateContext))

	// Get previous value for return
	prev := ptr.AdaptationState

	// Set the value if d is positive or zero
	if d >= 0.0 {

		ptr.AdaptationState = d
	}

	// Always return previous value
	return prev
}

// The adaptation state may be defaulted by this function. If you don't like it, use the extended transform routine
func cmsSetAdaptationState(d float64) float64 {
	return cmsSetAdaptationStateTHR(nil, d)
}

// Default alarm codes

// -----------------------------------------------------------------------

// Alarm codes for 16-bit transformations, because the fixed range of containers there are
// no values left to mark out of gamut.

var DEFAULT_ALARM_CODES_VALUE = [cmsMAXCHANNELS]uint16{0x7F00, 0x7F00, 0x7F00, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

// Global default alarm codes chunk
var cmsAlarmCodesChunk = cmsAlarmCodesChunkType{DEFAULT_ALARM_CODES_VALUE}

// Mutex for thread-safe access
//var alarmCodeMutex sync.Mutex

// cmsSetAlarmCodesTHR sets the alarm codes for a specific context.
func cmsSetAlarmCodesTHR(ContextID CmsContext, AlarmCodesP [cmsMAXCHANNELS]uint16) {
	//alarmCodeMutex.Lock()
	//defer alarmCodeMutex.Unlock()

	ContextAlarmCodes := (*cmsAlarmCodesChunkType)(CmsContextGetClientChunk(ContextID, AlarmCodesContext))
	if ContextAlarmCodes == nil {
		panic("ContextAlarmCodes is nil")
	}

	MemcpySlice(ContextAlarmCodes.AlarmCodes[:], AlarmCodesP[:], 16)
}

// cmsGetAlarmCodesTHR gets the alarm codes for a specific context.
func cmsGetAlarmCodesTHR(ContextID CmsContext, AlarmCodesP [cmsMAXCHANNELS]uint16) {
	//alarmCodeMutex.Lock()
	//defer alarmCodeMutex.Unlock()

	ContextAlarmCodes := (*cmsAlarmCodesChunkType)(CmsContextGetClientChunk(ContextID, AlarmCodesContext))
	if ContextAlarmCodes == nil {
		panic("ContextAlarmCodes is nil")
	}

	MemcpySlice(AlarmCodesP[:], ContextAlarmCodes.AlarmCodes[:], 16)
}

// cmsSetAlarmCodes sets the global alarm codes.
func cmsSetAlarmCodes(NewAlarm [cmsMAXCHANNELS]uint16) {
	if &NewAlarm[0] == nil {
		panic("NewAlarm is nil")
	}

	cmsSetAlarmCodesTHR(nil, NewAlarm)
}

// cmsGetAlarmCodes gets the global alarm codes.
func cmsGetAlarmCodes(OldAlarm [cmsMAXCHANNELS]uint16) {
	if &OldAlarm[0] == nil {
		panic("OldAlarm is nil")
	}

	cmsGetAlarmCodesTHR(nil, OldAlarm)
}

// cmsAllocAlarmCodesChunk initializes and duplicates alarm codes.
func cmsAllocAlarmCodesChunk(ctx CmsContext, src CmsContext) {
	// Define the static default alarm codes chunk
	AlarmCodesChunk := &cmsAlarmCodesChunkType{
		AlarmCodes: DEFAULT_ALARM_CODES_VALUE,
	}

	var from unsafe.Pointer

	// Check if src is not nil
	if src != nil {
		// Access the chunk from the source context
		from = src.chunks[AlarmCodesContext]
	} else {
		// Use the static default chunk
		from = unsafe.Pointer(AlarmCodesChunk)
	}

	// Allocate and duplicate the chunk in the context's memory pool
	ctx.chunks[AlarmCodesContext] = cmsSubAllocDup(ctx.MemPool, from, uint32(unsafe.Sizeof(cmsAlarmCodesChunkType{})))
}

// -----------------------------------------------------------------------

// cmsDeleteTransform releases the resources associated with a transform.
func cmsDeleteTransform(hTransform CmsHTRANSFORM) {
	p := (*cmsTRANSFORM)(hTransform)

	if p == nil {
		return
	}

	// Free GamutCheck pipeline if it exists
	if p.GamutCheck != nil {
		cmsPipelineFree(p.GamutCheck)
	}

	// Free the LUT pipeline if it exists
	if p.Lut != nil {
		cmsPipelineFree(p.Lut)
	}

	// Free input named color list if it exists
	if p.InputColorant != nil {
		cmsFreeNamedColorList(p.InputColorant)
	}

	// Free output named color list if it exists
	if p.OutputColorant != nil {
		cmsFreeNamedColorList(p.OutputColorant)
	}

	// Free profile sequence description if it exists
	if p.Sequence != nil {
		cmsFreeProfileSequenceDescription(p.Sequence)
	}

	// Free user data if it exists, using the user-defined deallocator
	if p.UserData != nil && p.FreeUserData != nil {
		p.FreeUserData(p.ContextID, p.UserData)
	}

	// Finally, free the transform object itself
	cmsFree(p.ContextID, unsafe.Pointer(p))
}

// PixelSize calculates the size of a pixel in bytes based on its format.
// If the format specifies double-precision, it returns the size of a double (float64 in Go).
func PixelSize(Format uint32) uint32 {
	fmtBytes := T_BYTES(Format)

	// For double-precision, the T_BYTES field is zero
	if fmtBytes == 0 {
		return uint32(unsafe.Sizeof(float64(0))) // Size of a float64
	}

	// Otherwise, it is already correct for all formats
	return fmtBytes
}

// cmsDoTransform applies a transformation to the input buffer and writes the result to the output buffer.
func CmsDoTransform(Transform CmsHTRANSFORM, InputBuffer, OutputBuffer any, Size uint32) {
	p := (*cmsTRANSFORM)(Transform) // Cast the generic Transform to the specific type cmsTRANSFORM
	var stride cmsStride

	// Initialize stride parameters
	stride.BytesPerLineIn = 0 // Not used
	stride.BytesPerLineOut = 0
	stride.BytesPerPlaneIn = Size * PixelSize(p.InputFormat)
	stride.BytesPerPlaneOut = Size * PixelSize(p.OutputFormat)

	// Perform the transformation
	p.Xform(p, InputBuffer, OutputBuffer, Size, 1, &stride)
}

func CmsDoTransformStride(
	Transform CmsHTRANSFORM,
	InputBuffer, OutputBuffer any,
	Size uint32,
	Stride uint32) {

	p := (*cmsTRANSFORM)(Transform)
	var stride cmsStride

	stride.BytesPerLineIn = 0
	stride.BytesPerLineOut = 0
	stride.BytesPerPlaneIn = Stride
	stride.BytesPerPlaneOut = Stride

	p.Xform(p, InputBuffer, OutputBuffer, Size, 1, &stride)
}

func CmsDoTransformLineStride(
	Transform CmsHTRANSFORM,
	InputBuffer,
	OutputBuffer any,
	PixelsPerLine uint32,
	LineCount uint32,
	BytesPerLineIn uint32,
	BytesPerLineOut uint32,
	BytesPerPlaneIn uint32,
	BytesPerPlaneOut uint32) {

	p := (*cmsTRANSFORM)(Transform)
	var stride cmsStride

	stride.BytesPerLineIn = BytesPerLineIn
	stride.BytesPerLineOut = BytesPerLineOut
	stride.BytesPerPlaneIn = BytesPerPlaneIn
	stride.BytesPerPlaneOut = BytesPerPlaneOut

	p.Xform(p, InputBuffer, OutputBuffer, PixelsPerLine, LineCount, &stride)
}

// Transform routines ----------------------------------------------------------------------------------------------------------

// Float xform converts floats. Since there are no performance issues, one routine does all job, including gamut check.
// Note that because extended range, we can use a -1.0 value for out of gamut in this case.

func FloatXFORM(
	p *cmsTRANSFORM,
	in, out any,
	PixelsPerLine, LineCount uint32,
	Stride *cmsStride,
) {
	var fIn, fOut [cmsMAXCHANNELS]float32
	var OutOfGamut float32
	var strideIn, strideOut uint32
	var inBytes, outBytes []byte
	// Type assertion for input and output
	var accum, output []byte

	// Type assertion and conversion for input
	switch v := in.(type) {
	case []byte:
		inBytes = v
	case []float32:
		inBytes = float32SliceToBytes(v)
	case []float64:
		inBytes = float64SliceToBytes(v)
	case []uint16:
		inBytes = uint16SliceToBytes(v)
	default:
		panic("Error: 'in' must be of type []byte, []float32, []float64, or []uint16")
	}

	// Type assertion and conversion for output
	switch v := out.(type) {
	case []byte:
		outBytes = v
	case []float32:
		outBytes = float32SliceToBytes(v)
	case []float64:
		outBytes = float64SliceToBytes(v)
	case []uint16:
		outBytes = uint16SliceToBytes(v)
	default:
		panic("Error: 'out' must be of type []byte, []float32, []float64, or []uint16")
	}

	cmsHandleExtraChannels(p, in, out, PixelsPerLine, LineCount, Stride)

	strideIn, strideOut = 0, 0

	for i := uint32(0); i < LineCount; i++ {
		//  Use slices with offsets instead of unsafe
		accum = inBytes[strideIn:]
		output = outBytes[strideOut:]

		for j := uint32(0); j < PixelsPerLine; j++ {
			//  Process input correctly using slice indexing
			accum = p.FromInputFloat(p, fIn[:], accum, Stride.BytesPerPlaneIn)

			//  Replace unsafe pointer arithmetic for `OutOfGamut`
			outOfGamutSlice := []float32{OutOfGamut}

			if p.GamutCheck != nil {
				//  Use slice indexing instead of pointer casting
				cmsPipelineEvalFloat(fIn[:], outOfGamutSlice, p.GamutCheck)

				if outOfGamutSlice[0] > 0.0 {
					//  Mark all output channels as out of gamut efficiently
					for c := range fOut {
						fOut[c] = -1.0
					}
				} else {
					//  Evaluate the pipeline normally
					cmsPipelineEvalFloat(fIn[:], fOut[:], p.Lut)
				}
			} else {
				//  No gamut check; evaluate pipeline directly
				cmsPipelineEvalFloat(fIn[:], fOut[:], p.Lut)
			}

			//  Process output correctly
			output = p.ToOutputFloat(p, fOut[:], output, Stride.BytesPerPlaneOut)
		}

		//  Update strides correctly
		strideIn += Stride.BytesPerLineIn
		strideOut += Stride.BytesPerLineOut
	}
}

func NullFloatXFORM(
	p *cmsTRANSFORM,
	in, out any,
	PixelsPerLine, LineCount uint32,
	Stride *cmsStride,
) {
	var fIn [cmsMAXCHANNELS]float32
	var strideIn, strideOut uint32
	var accum, output []byte
	// Type assertion for input and output
	inBytes, okIn := in.([]byte)
	outBytes, okOut := out.([]byte)

	if !okIn || !okOut {
		panic(" in and out must be of type []byte")
	}

	cmsHandleExtraChannels(p, in, out, PixelsPerLine, LineCount, Stride)

	strideIn, strideOut = 0, 0

	for i := uint32(0); i < LineCount; i++ {
		//  Use slices with offsets instead of unsafe
		accum = inBytes[strideIn:]
		output = outBytes[strideOut:]

		for j := uint32(0); j < PixelsPerLine; j++ {
			//  Process input correctly using slice indexing
			accum = p.FromInputFloat(p, fIn[:], accum, Stride.BytesPerPlaneIn)
			output = p.ToOutputFloat(p, fIn[:], output, Stride.BytesPerPlaneOut)
		}

		//  Update strides correctly
		strideIn += Stride.BytesPerLineIn
		strideOut += Stride.BytesPerLineOut
	}
}

func NullXFORM(
	p *cmsTRANSFORM,
	in, out any,
	PixelsPerLine, LineCount uint32,
	Stride *cmsStride,
) {
	var wIn [cmsMAXCHANNELS]uint16
	var strideIn, strideOut uint32
	var accum, output []byte
	// Type assertion for input and output
	inBytes, okIn := in.([]byte)
	outBytes, okOut := out.([]byte)

	if !okIn || !okOut {
		panic(" in and out must be of type []byte")
	}

	cmsHandleExtraChannels(p, in, out, PixelsPerLine, LineCount, Stride)

	strideIn, strideOut = 0, 0

	for i := uint32(0); i < LineCount; i++ {
		//  Use slices with offsets instead of unsafe
		accum = inBytes[strideIn:]
		output = outBytes[strideOut:]

		for j := uint32(0); j < PixelsPerLine; j++ {
			//  Process input correctly using slice indexing
			accum = p.FromInput(p, wIn[:], accum, Stride.BytesPerPlaneIn)
			output = p.ToOutput(p, wIn[:], output, Stride.BytesPerPlaneOut)
		}

		//  Update strides correctly
		strideIn += Stride.BytesPerLineIn
		strideOut += Stride.BytesPerLineOut
	}
}

func PrecalculatedXFORM(
	p *cmsTRANSFORM,
	in, out any,
	PixelsPerLine, LineCount uint32,
	Stride *cmsStride,
) {
	var wIn, wOut [cmsMAXCHANNELS]uint16
	var strideIn, strideOut uint32
	var accum, output []byte
	// Type assertion for input and output
	inBytes, okIn := in.([]byte)
	outBytes, okOut := out.([]byte)

	if !okIn || !okOut {
		panic(" in and out must be of type []byte")
	}

	cmsHandleExtraChannels(p, in, out, PixelsPerLine, LineCount, Stride)

	strideIn, strideOut = 0, 0

	for i := uint32(0); i < LineCount; i++ {
		// Accumulator slices for this line
		accum = inBytes[strideIn:]
		output = outBytes[strideOut:]

		for j := uint32(0); j < PixelsPerLine; j++ {
			// Process input
			accum = p.FromInput(p, wIn[:], accum, Stride.BytesPerPlaneIn)

			// Evaluate LUT
			p.Lut.Eval16Fn(wIn[:], wOut[:], p.Lut.Data)

			// Process output
			output = p.ToOutput(p, wOut[:], output, Stride.BytesPerPlaneOut)
		}

		// Update strides
		strideIn += Stride.BytesPerLineIn
		strideOut += Stride.BytesPerLineOut
	}
}

// Auxiliary: Handle precalculated gamut check. The retrieval of context may be alittle bit slow, but this function is not critical.
func TransformOnePixelWithGamutCheck(p *cmsTRANSFORM, wIn, wOut []uint16) {
	var wOutOfGamut uint16

	woutOfGamutSlice := []uint16{wOutOfGamut}
	// Evaluate the gamut check function
	p.GamutCheck.Eval16Fn(wIn, woutOfGamutSlice, p.GamutCheck.Data)

	if woutOfGamutSlice[0] >= 1 {
		// If out of gamut, use alarm codes
		contextAlarmCodes := (*cmsAlarmCodesChunkType)(CmsContextGetClientChunk(p.ContextID, AlarmCodesContext))
		for i := uint32(0); i < p.Lut.OutputChannels; i++ {
			wOut[i] = contextAlarmCodes.AlarmCodes[i]
		}
	} else {
		// Otherwise, evaluate the LUT
		p.Lut.Eval16Fn(wIn, wOut, p.Lut.Data)
	}
}

func PrecalculatedXFORMGamutCheck(
	p *cmsTRANSFORM,
	in, out any,
	PixelsPerLine, LineCount uint32,
	Stride *cmsStride,
) {
	var wIn, wOut [cmsMAXCHANNELS]uint16
	var strideIn, strideOut uint32
	var accum, output []byte
	// Type assertion for input and output
	inBytes, okIn := in.([]byte)
	outBytes, okOut := out.([]byte)

	if !okIn || !okOut {
		panic(" in and out must be of type []byte")
	}

	cmsHandleExtraChannels(p, in, out, PixelsPerLine, LineCount, Stride)

	strideIn, strideOut = 0, 0

	for i := uint32(0); i < LineCount; i++ {
		// Use slices with offsets instead of large allocation
		accum = inBytes[strideIn:]
		output = outBytes[strideOut:]

		for j := uint32(0); j < PixelsPerLine; j++ {
			// Correctly advance accum and output slices
			accum = p.FromInput(p, wIn[:], accum, Stride.BytesPerPlaneIn)
			TransformOnePixelWithGamutCheck(p, wIn[:], wOut[:])
			output = p.ToOutput(p, wOut[:], output, Stride.BytesPerPlaneOut)
		}

		// Update strides correctly
		strideIn += Stride.BytesPerLineIn
		strideOut += Stride.BytesPerLineOut
	}
}

func CachedXFORM(
	p *cmsTRANSFORM,
	in, out any,
	PixelsPerLine, LineCount uint32,
	Stride *cmsStride,
) {
	var wIn, wOut [cmsMAXCHANNELS]uint16
	var strideIn, strideOut uint32
	var cache cmsCACHE
	var accum, output []byte
	// Type assertion for input and output
	inBytes, okIn := in.([]byte)
	outBytes, okOut := out.([]byte)
	//	fmt.Println("CachedXFORM")
	if !okIn || !okOut {
		panic(" in and out must be of type []byte")
	}

	cmsHandleExtraChannels(p, in, out, PixelsPerLine, LineCount, Stride)

	// Copy cache
	cache = p.Cache

	strideIn, strideOut = 0, 0

	for i := uint32(0); i < LineCount; i++ {
		// Use slices with offsets instead of pointer arithmetic
		accum = inBytes[strideIn:]
		output = outBytes[strideOut:]

		for j := uint32(0); j < PixelsPerLine; j++ {
			// Correctly advance accum and output using slices
			accum = p.FromInput(p, wIn[:], accum, Stride.BytesPerPlaneIn)

			// Use cache to avoid redundant calculations
			if reflect.DeepEqual(wIn, cache.CacheIn) {
				copy(wOut[:], cache.CacheOut[:])
			} else {
				p.Lut.Eval16Fn(wIn[:], wOut[:], p.Lut.Data)
				copy(cache.CacheIn[:], wIn[:])
				copy(cache.CacheOut[:], wOut[:])
			}

			// Advance output using slice indexing
			output = p.ToOutput(p, wOut[:], output, Stride.BytesPerPlaneOut)
		}

		// Update strides correctly
		strideIn += Stride.BytesPerLineIn
		strideOut += Stride.BytesPerLineOut
	}
}

func CachedXFORMGamutCheck(
	p *cmsTRANSFORM,
	in, out any,
	PixelsPerLine, LineCount uint32,
	Stride *cmsStride,
) {
	var wIn, wOut [cmsMAXCHANNELS]uint16
	var strideIn, strideOut uint32
	var cache cmsCACHE
	var accum, output []byte
	// Type assertion for input and output
	inBytes, okIn := in.([]byte)
	outBytes, okOut := out.([]byte)

	if !okIn || !okOut {
		panic(" in and out must be of type []byte")
	}

	cmsHandleExtraChannels(p, in, out, PixelsPerLine, LineCount, Stride)

	// Copy cache
	cache = p.Cache

	strideIn, strideOut = 0, 0

	for i := uint32(0); i < LineCount; i++ {
		//  Use slices with offsets instead of unsafe
		accum = inBytes[strideIn:]
		output = outBytes[strideOut:]

		for j := uint32(0); j < PixelsPerLine; j++ {
			//  Correctly advance accum using slices
			accum = p.FromInput(p, wIn[:], accum, Stride.BytesPerPlaneIn)

			//  Use cache for performance optimization
			if reflect.DeepEqual(wIn, cache.CacheIn) {
				copy(wOut[:], cache.CacheOut[:])
			} else {
				TransformOnePixelWithGamutCheck(p, wIn[:], wOut[:])
				copy(cache.CacheIn[:], wIn[:])
				copy(cache.CacheOut[:], wOut[:])
			}

			//  Correctly advance output using slices
			output = p.ToOutput(p, wOut[:], output, Stride.BytesPerPlaneOut)
		}

		//  Update strides correctly
		strideIn += Stride.BytesPerLineIn
		strideOut += Stride.BytesPerLineOut
	}
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
func DupPluginTransformList(ctx CmsContext, src CmsContext) {
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
func cmsAllocTransformPluginChunk(ctx CmsContext, src CmsContext) {
	if src != nil {
		DupPluginTransformList(ctx, src)
	} else {
		var defaultChunk cmsTransformPluginChunkType
		ctx.chunks[TransformPlugin] = cmsSubAllocDup(ctx.MemPool, unsafe.Pointer(&defaultChunk), uint32(unsafe.Sizeof(defaultChunk)))
	}
}

// cmsTransform2toTransformAdaptor adapts new-style transforms to the old-style interface.
func cmsTransform2toTransformAdaptor(cmmcargo *cmsTRANSFORM, in, out any, pixelsPerLine, lineCount uint32, stride *cmsStride) {
	var strideIn, strideOut uint32
	var accum, output []byte

	cmsHandleExtraChannels(cmmcargo, in, out, pixelsPerLine, lineCount, stride)

	// Type assertion for input and output
	inBytes, okIn := in.([]byte)
	outBytes, okOut := out.([]byte)

	if !okIn || !okOut {
		panic(" in and out must be of type []byte")
	}

	for i := uint32(0); i < lineCount; i++ {
		accum = inBytes[strideIn:]
		output = outBytes[strideOut:]

		cmmcargo.OldXform(cmmcargo, accum, output, pixelsPerLine, stride.BytesPerPlaneIn)

		strideIn += stride.BytesPerLineIn
		strideOut += stride.BytesPerLineOut
	}
}

func cmsTransform2toTransformConverter(
	p *cmsTRANSFORM,
	InputBuffer,
	OutputBuffer any,
	Size uint32,
	Stride uint32,
) {
	// Calculate PixelsPerLine and LineCount from Size and Stride
	PixelsPerLine := Size / Stride
	LineCount := uint32(1) // Assuming 1 line for simplicity, adapt as needed

	// Call the cmsTransform2Fn
	if p.Xform != nil {
		p.Xform(p, InputBuffer, OutputBuffer, PixelsPerLine, LineCount, nil)
	}
}

// cmsRegisterTransformPlugin registers a new transform plugin.
func cmsRegisterTransformPlugin(ContextID CmsContext, Data *cmsPluginBase) bool {
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

func ParallelizeIfSuitable(p *cmsTRANSFORM) {
	if p == nil {
		panic("cmsTRANSFORM pointer is nil")
	}

	ctx := (*cmsParallelizationPluginChunkType)(CmsContextGetClientChunk(p.ContextID, ParallelizationPlugin))

	if ctx != nil && ctx.SchedulerFn != nil {
		p.Worker = p.Xform
		p.Xform = ctx.SchedulerFn
		p.MaxWorkers = ctx.MaxWorkers
		p.WorkerFlags = uint32(ctx.WorkerFlags)
	}
}
func UnrollNothing(
	info *cmsTRANSFORM,
	wIn []uint16,
	accum []uint8,
	Stride uint32,
) []uint8 {
	// No operation, return the input slice unchanged
	return accum
}
func PackNothing(
	info *cmsTRANSFORM,
	wOut []uint16,
	output []uint8,
	Stride uint32,
) []uint8 {
	// No operation, return the output slice unchanged
	return output
}

func AllocEmptyTransform(
	ContextID CmsContext,
	lut *cmsPipeline,
	Intent uint32,
	InputFormat, OutputFormat, dwFlags *uint32,
) *cmsTRANSFORM {
	// Get the transform plugin chunk
	ctx := (*cmsTransformPluginChunkType)(CmsContextGetClientChunk(ContextID, TransformPlugin))
	var plugin *cmsTransformCollection

	// Allocate memory for the transform structure
	p := allocateStruct[cmsTRANSFORM]()
	if p == nil {
		cmsPipelineFree(lut)
		return nil
	}

	// Store the proposed pipeline
	p.Lut = lut

	// Check if any plugin wants to handle the transform
	if p.Lut != nil {
		if (*dwFlags & cmsFLAGS_NOOPTIMIZE) == 0 {
			for plugin = ctx.TransformCollection; plugin != nil; plugin = plugin.Next {
				if plugin.Factory(&p.Xform, &p.UserData, &p.FreeUserData, &p.Lut, InputFormat, OutputFormat, dwFlags) {
					// Set plugin-controlled parameters
					p.ContextID = ContextID
					p.InputFormat = *InputFormat
					p.OutputFormat = *OutputFormat
					p.DwOriginalFlags = *dwFlags

					// Fill formatters
					p.FromInput = cmsGetFormatter(ContextID, *InputFormat, cmsFormatterInput, CMS_PACK_FLAGS_16BITS).Fmt16
					p.ToOutput = cmsGetFormatter(ContextID, *OutputFormat, cmsFormatterOutput, CMS_PACK_FLAGS_16BITS).Fmt16
					p.FromInputFloat = cmsGetFormatter(ContextID, *InputFormat, cmsFormatterInput, CMS_PACK_FLAGS_FLOAT).FmtFloat
					p.ToOutputFloat = cmsGetFormatter(ContextID, *OutputFormat, cmsFormatterOutput, CMS_PACK_FLAGS_FLOAT).FmtFloat

					// Handle old transform plugins
					if plugin.OldXform {
						// Wrap the current Xform with an adapter
						p.OldXform = func(CMMcargo *cmsTRANSFORM, InputBuffer any, OutputBuffer any, Size uint32, Stride uint32) {
							if p.Xform != nil {
								cmsTransform2toTransformConverter(p, InputBuffer, OutputBuffer, Size, Stride)
							}
						}

						p.Xform = cmsTransform2toTransformAdaptor
					}

					// Parallelize if suitable
					ParallelizeIfSuitable(p)
					return p
				}
			}
		}

		// Optimize the pipeline if no plugin handled the transform
		cmsOptimizePipeline(ContextID, &p.Lut, Intent, InputFormat, OutputFormat, dwFlags)
	}

	// Check for floating-point transform
	if cmsFormatterIsFloat(*OutputFormat) {
		p.FromInputFloat = cmsGetFormatter(ContextID, *InputFormat, cmsFormatterInput, CMS_PACK_FLAGS_FLOAT).FmtFloat
		p.ToOutputFloat = cmsGetFormatter(ContextID, *OutputFormat, cmsFormatterOutput, CMS_PACK_FLAGS_FLOAT).FmtFloat
		*dwFlags |= cmsFLAGS_CAN_CHANGE_FORMATTER

		if p.FromInputFloat == nil || p.ToOutputFloat == nil {
			cmsSignalError(unsafe.Pointer(ContextID), cmsERROR_UNKNOWN_EXTENSION, "Unsupported raster format")
			cmsDeleteTransform(CmsHTRANSFORM(p))
			return nil
		}

		if (*dwFlags & cmsFLAGS_NULLTRANSFORM) != 0 {
			p.Xform = NullFloatXFORM
		} else {
			p.Xform = FloatXFORM
		}
	} else {
		// Handle non-floating point formats
		if *InputFormat == 0 && *OutputFormat == 0 {
			p.FromInput = UnrollNothing
			p.ToOutput = PackNothing
			*dwFlags |= cmsFLAGS_CAN_CHANGE_FORMATTER
		} else {
			p.FromInput = cmsGetFormatter(ContextID, *InputFormat, cmsFormatterInput, CMS_PACK_FLAGS_16BITS).Fmt16
			p.ToOutput = cmsGetFormatter(ContextID, *OutputFormat, cmsFormatterOutput, CMS_PACK_FLAGS_16BITS).Fmt16

			if p.FromInput == nil || p.ToOutput == nil {
				cmsSignalError(unsafe.Pointer(ContextID), cmsERROR_UNKNOWN_EXTENSION, "Unsupported raster format")
				cmsDeleteTransform(CmsHTRANSFORM(p))
				return nil
			}

			if T_BYTES(*InputFormat) >= 2 {
				*dwFlags |= cmsFLAGS_CAN_CHANGE_FORMATTER
			}
		}

		if (*dwFlags & cmsFLAGS_NULLTRANSFORM) != 0 {
			p.Xform = NullXFORM
		} else if (*dwFlags & cmsFLAGS_NOCACHE) != 0 {
			if (*dwFlags & cmsFLAGS_GAMUTCHECK) != 0 {
				p.Xform = PrecalculatedXFORMGamutCheck
			} else {
				p.Xform = PrecalculatedXFORM
			}
		} else {
			if (*dwFlags & cmsFLAGS_GAMUTCHECK) != 0 {
				p.Xform = CachedXFORMGamutCheck
			} else {
				p.Xform = CachedXFORM
			}
		}
	}

	// Finalize the transform structure
	p.InputFormat = *InputFormat
	p.OutputFormat = *OutputFormat
	p.DwOriginalFlags = *dwFlags
	p.ContextID = ContextID
	p.UserData = nil

	ParallelizeIfSuitable(p)
	return p
}
func GetXFormColorSpaces(
	nProfiles uint32,
	hProfiles []CmsHPROFILE,
	Input, Output *cmsColorSpaceSignature,
) bool {
	if nProfiles == 0 || hProfiles[0] == nil {
		return false
	}

	*Input = CmsGetColorSpace(hProfiles[0])
	PostColorSpace := *Input

	for i := uint32(0); i < nProfiles; i++ {
		hProfile := hProfiles[i]
		if hProfile == nil {
			return false
		}

		cls := cmsGetDeviceClass(hProfile)
		var ColorSpaceIn, ColorSpaceOut cmsColorSpaceSignature

		lIsInput := PostColorSpace != cmsSigXYZData && PostColorSpace != cmsSigLabData

		switch {
		case cls == cmsSigNamedColorClass:
			ColorSpaceIn = cmsSig1colorData
			if nProfiles > 1 {
				ColorSpaceOut = cmsGetPCS(hProfile)
			} else {
				ColorSpaceOut = CmsGetColorSpace(hProfile)
			}

		case lIsInput || cls == cmsSigLinkClass:
			ColorSpaceIn = CmsGetColorSpace(hProfile)
			ColorSpaceOut = cmsGetPCS(hProfile)

		default:
			ColorSpaceIn = cmsGetPCS(hProfile)
			ColorSpaceOut = CmsGetColorSpace(hProfile)
		}
		if i == 0 {
			*Input = ColorSpaceIn
		}

		PostColorSpace = ColorSpaceOut
	}

	*Output = PostColorSpace

	return true
}

func IsProperColorSpace(Check cmsColorSpaceSignature, dwFormat uint32) bool {
	Space1 := int(T_COLORSPACE(dwFormat))
	Space2 := cmsLCMScolorSpace(Check)

	if Space1 == PT_ANY {
		return true
	}
	if Space1 == Space2 {
		return true
	}
	if (Space1 == PT_LabV2 && Space2 == PT_Lab) || (Space1 == PT_Lab && Space2 == PT_LabV2) {
		return true
	}

	return false
}

// ----------------------------------------------------------------------------------------------------------------

// Jun-21-2000: Some profiles (those that comes with W2K) comes
// with the media white (media black?) x 100. Add a sanity check

func NormalizeXYZ(Dest *cmsCIEXYZ) {
	for Dest.X > 2. &&
		Dest.Y > 2. &&
		Dest.Z > 2. {

		Dest.X /= 10.
		Dest.Y /= 10.
		Dest.Z /= 10.
	}
}

func SetWhitePoint(wtPt *cmsCIEXYZ, src *cmsCIEXYZ) {
	if src == nil {
		wtPt.X = cmsD50X
		wtPt.Y = cmsD50Y
		wtPt.Z = cmsD50Z
	} else {
		wtPt.X = src.X
		wtPt.Y = src.Y
		wtPt.Z = src.Z

		NormalizeXYZ(wtPt)
	}

}
func cmsCreateExtendedTransform(
	ContextID CmsContext,
	nProfiles uint32,
	hProfiles []CmsHPROFILE,
	BPC []bool,
	Intents []uint32,
	AdaptationStates []float64,
	hGamutProfile CmsHPROFILE,
	nGamutPCSposition uint32,
	InputFormat uint32,
	OutputFormat uint32,
	dwFlags uint32,
) *cmsTRANSFORM {
	//fmt.Println("cmsCreateExtendedTransform")
	// Check if it's a fake transform
	if dwFlags&cmsFLAGS_NULLTRANSFORM != 0 {
		return AllocEmptyTransform(ContextID, nil, INTENT_PERCEPTUAL, &InputFormat, &OutputFormat, &dwFlags)
	}

	// Gamut check validation
	if dwFlags&cmsFLAGS_GAMUTCHECK != 0 && hGamutProfile == nil {
		dwFlags &^= cmsFLAGS_GAMUTCHECK
	}

	// Disable cache for floating-point formats
	if cmsFormatterIsFloat(InputFormat) || cmsFormatterIsFloat(OutputFormat) {
		dwFlags |= cmsFLAGS_NOCACHE
	}

	// Retrieve entry and exit color spaces
	var EntryColorSpace, ExitColorSpace cmsColorSpaceSignature
	if !GetXFormColorSpaces(nProfiles, hProfiles, &EntryColorSpace, &ExitColorSpace) {
		cmsSignalError(unsafe.Pointer(ContextID), cmsERROR_NULL, "NULL input profiles on transform")
		return nil
	}

	// Validate color spaces
	if !IsProperColorSpace(EntryColorSpace, InputFormat) {
		cmsSignalError(unsafe.Pointer(ContextID), cmsERROR_COLORSPACE_CHECK, "Wrong input color space on transform")
		return nil
	}
	if !IsProperColorSpace(ExitColorSpace, OutputFormat) {
		cmsSignalError(unsafe.Pointer(ContextID), cmsERROR_COLORSPACE_CHECK, "Wrong output color space on transform")
		return nil
	}
	// Check whatever the transform is 16 bits and involves linear RGB in first profile. If so, disable optimizations
	if EntryColorSpace == cmsSigRgbData && T_BYTES(InputFormat) == 2 && (dwFlags&cmsFLAGS_NOOPTIMIZE) == 0 {
		gamma := cmsDetectRGBProfileGamma(hProfiles[0], 0.1)

		if gamma > 0 && gamma < 1.6 {
			dwFlags |= cmsFLAGS_NOOPTIMIZE
		}
	}

	// Build transformation pipeline
	Lut := cmsLinkProfiles(ContextID, nProfiles, Intents, hProfiles, BPC, AdaptationStates, dwFlags)
	if Lut == nil {
		cmsSignalError(unsafe.Pointer(ContextID), cmsERROR_NOT_SUITABLE, "Couldn't link the profiles")
		return nil
	}

	// Validate channel counts
	// Check channel count
	if (cmsChannelsOfColorSpace(EntryColorSpace) != int32(cmsPipelineInputChannels(Lut))) ||
		(cmsChannelsOfColorSpace(ExitColorSpace) != int32(cmsPipelineOutputChannels(Lut))) {
		cmsPipelineFree(Lut)
		cmsSignalError(unsafe.Pointer(ContextID), cmsERROR_NOT_SUITABLE, "Channel count doesn't match. Profile is corrupted")
		return nil
	}

	// Allocate transform
	xform := AllocEmptyTransform(ContextID, Lut, Intents[nProfiles-1], &InputFormat, &OutputFormat, &dwFlags)
	if xform == nil {
		return nil
	}

	// Configure transform
	xform.EntryColorSpace = EntryColorSpace
	xform.ExitColorSpace = ExitColorSpace
	xform.RenderingIntent = Intents[nProfiles-1]
	// Take white points
	SetWhitePoint(&xform.EntryWhitePoint, (*cmsCIEXYZ)(cmsReadTag(hProfiles[0], cmsSigMediaWhitePointTag)))
	SetWhitePoint(&xform.ExitWhitePoint, (*cmsCIEXYZ)(cmsReadTag(hProfiles[nProfiles-1], cmsSigMediaWhitePointTag)))

	// Add optional gamut check
	if hGamutProfile != nil && (dwFlags&cmsFLAGS_GAMUTCHECK != 0) {
		xform.GamutCheck = cmsCreateGamutCheckPipeline(ContextID, hProfiles, BPC, Intents, AdaptationStates, nGamutPCSposition, hGamutProfile)
	}
	// Try to read input and output colorant table
	if cmsIsTag(hProfiles[0], cmsSigColorantTableTag) {

		// Input table can only come in this way.
		xform.InputColorant = cmsDupNamedColorList((*cmsNAMEDCOLORLIST)(cmsReadTag(hProfiles[0], cmsSigColorantTableTag)))
	}

	// Output is a little bit more complex.
	if cmsGetDeviceClass(hProfiles[nProfiles-1]) == cmsSigLinkClass {

		// This tag may exist only on devicelink profiles.
		if cmsIsTag(hProfiles[nProfiles-1], cmsSigColorantTableOutTag) {

			// It may be NULL if error
			xform.OutputColorant = cmsDupNamedColorList((*cmsNAMEDCOLORLIST)(cmsReadTag(hProfiles[nProfiles-1], cmsSigColorantTableOutTag)))
		}

	} else {

		if cmsIsTag(hProfiles[nProfiles-1], cmsSigColorantTableTag) {

			xform.OutputColorant = cmsDupNamedColorList((*cmsNAMEDCOLORLIST)(cmsReadTag(hProfiles[nProfiles-1], cmsSigColorantTableTag)))
		}
	}

	// Store the sequence of profiles
	if dwFlags&cmsFLAGS_KEEP_SEQUENCE != 0 {
		xform.Sequence = cmsCompileProfileSequence(ContextID, nProfiles, hProfiles)
	} else {
		xform.Sequence = nil
	}
	// If this is a cached transform, init first value, which is zero (16 bits only)
	if dwFlags&cmsFLAGS_NOCACHE == 0 {

		if xform.GamutCheck != nil {
			TransformOnePixelWithGamutCheck(xform, xform.Cache.CacheIn[:], xform.Cache.CacheOut[:])
		} else {
			xform.Lut.Eval16Fn(xform.Cache.CacheIn[:], xform.Cache.CacheOut[:], xform.Lut.Data)
		}

	}

	return xform
}

// cmsCreateMultiprofileTransformTHR creates a multiprofile transform with a specified context.
func cmsCreateMultiprofileTransformTHR(
	ContextID CmsContext,
	hProfiles []CmsHPROFILE,
	nProfiles uint32,
	InputFormat uint32,
	OutputFormat uint32,
	Intent uint32,
	dwFlags uint32,
) CmsHTRANSFORM {
	//fmt.Println("cmsCreateMultiprofileTransformTHR")
	var BPC [256]bool
	var Intents [256]uint32
	var AdaptationStates [256]float64

	// Check the number of profiles
	if nProfiles <= 0 || nProfiles > 255 {
		cmsSignalError(unsafe.Pointer(ContextID), cmsERROR_RANGE, "Wrong number of profiles. 1..255 expected")
		return nil
	}

	// Initialize BPC, Intents, and AdaptationStates
	for i := uint32(0); i < nProfiles; i++ {
		if dwFlags&cmsFLAGS_BLACKPOINTCOMPENSATION != 0 {
			BPC[i] = true
		} else {
			BPC[i] = false
		}
		Intents[i] = Intent
		AdaptationStates[i] = cmsSetAdaptationStateTHR(ContextID, -1)
	}

	// Create the extended transform
	return CmsHTRANSFORM(cmsCreateExtendedTransform(ContextID, nProfiles, hProfiles, BPC[:], Intents[:], AdaptationStates[:], nil, 0, InputFormat, OutputFormat, dwFlags))
}

// cmsCreateMultiprofileTransform creates a multiprofile transform with a default context.
func cmsCreateMultiprofileTransform(
	hProfiles []CmsHPROFILE,
	nProfiles uint32,
	InputFormat uint32,
	OutputFormat uint32,
	Intent uint32,
	dwFlags uint32,
) CmsHTRANSFORM {
	// Check the number of profiles
	if nProfiles <= 0 || nProfiles > 255 {
		cmsSignalError(nil, cmsERROR_RANGE, "Wrong number of profiles")
		return nil
	}

	// Get the context ID from the first profile and call the THR version
	return cmsCreateMultiprofileTransformTHR(
		cmsGetProfileContextID(hProfiles[0]),
		hProfiles,
		nProfiles,
		InputFormat,
		OutputFormat,
		Intent,
		dwFlags,
	)
}

func cmsCreateTransformTHR(
	ContextID CmsContext,
	Input CmsHPROFILE,
	InputFormat uint32,
	Output CmsHPROFILE,
	OutputFormat uint32,
	Intent uint32,
	dwFlags uint32,
) CmsHTRANSFORM {
	//fmt.Println("CmsCreateTransformTHR")

	hProfiles := []CmsHPROFILE{Input, Output}
	nProfiles := uint32(1)
	if Output != nil {
		nProfiles = 2
	}

	return cmsCreateMultiprofileTransformTHR(ContextID, hProfiles, nProfiles, InputFormat, OutputFormat, Intent, dwFlags)
}

func CmsCreateTransform(
	Input CmsHPROFILE,
	InputFormat uint32,
	Output CmsHPROFILE,
	OutputFormat uint32,
	Intent uint32,
	dwFlags uint32,
) CmsHTRANSFORM {
	//fmt.Println("CmsCreateTransform")
	return cmsCreateTransformTHR(cmsGetProfileContextID(Input), Input, InputFormat, Output, OutputFormat, Intent, dwFlags)
}

func cmsCreateProofingTransformTHR(
	ContextID CmsContext,
	InputProfile CmsHPROFILE,
	InputFormat uint32,
	OutputProfile CmsHPROFILE,
	OutputFormat uint32,
	ProofingProfile CmsHPROFILE,
	nIntent uint32,
	ProofingIntent uint32,
	dwFlags uint32,
) CmsHTRANSFORM {
	//fmt.Println("cmsCreateProofingTransformTHR")

	hArray := []CmsHPROFILE{InputProfile, ProofingProfile, ProofingProfile, OutputProfile}
	Intents := []uint32{nIntent, nIntent, INTENT_RELATIVE_COLORIMETRIC, ProofingIntent}
	BPC := []bool{
		dwFlags&cmsFLAGS_BLACKPOINTCOMPENSATION != 0,
		dwFlags&cmsFLAGS_BLACKPOINTCOMPENSATION != 0,
		false,
		false,
	}
	Adaptation := []float64{
		cmsSetAdaptationStateTHR(ContextID, -1),
		cmsSetAdaptationStateTHR(ContextID, -1),
		cmsSetAdaptationStateTHR(ContextID, -1),
		cmsSetAdaptationStateTHR(ContextID, -1),
	}

	if dwFlags&(cmsFLAGS_SOFTPROOFING|cmsFLAGS_GAMUTCHECK) == 0 {
		return cmsCreateTransformTHR(ContextID, InputProfile, InputFormat, OutputProfile, OutputFormat, nIntent, dwFlags)
	}

	return CmsHTRANSFORM(cmsCreateExtendedTransform(ContextID, 4, hArray, BPC, Intents, Adaptation, ProofingProfile, 1, InputFormat, OutputFormat, dwFlags))
}

func cmsCreateProofingTransform(
	InputProfile CmsHPROFILE,
	InputFormat uint32,
	OutputProfile CmsHPROFILE,
	OutputFormat uint32,
	ProofingProfile CmsHPROFILE,
	nIntent uint32,
	ProofingIntent uint32,
	dwFlags uint32,
) CmsHTRANSFORM {
	return cmsCreateProofingTransformTHR(
		cmsGetProfileContextID(InputProfile),
		InputProfile,
		InputFormat,
		OutputProfile,
		OutputFormat,
		ProofingProfile,
		nIntent,
		ProofingIntent,
		dwFlags,
	)
}

func cmsGetTransformContextID(hTransform CmsHTRANSFORM) CmsContext {
	xform := (*cmsTRANSFORM)(hTransform)
	if xform == nil {
		return nil
	}
	return xform.ContextID
}

func cmsGetTransformInputFormat(hTransform CmsHTRANSFORM) uint32 {
	xform := (*cmsTRANSFORM)(hTransform)
	if xform == nil {
		return 0
	}
	return xform.InputFormat
}

func cmsGetTransformOutputFormat(hTransform CmsHTRANSFORM) uint32 {
	xform := (*cmsTRANSFORM)(hTransform)
	if xform == nil {
		return 0
	}
	return xform.OutputFormat
}

func cmsChangeBuffersFormat(
	hTransform CmsHTRANSFORM,
	InputFormat uint32,
	OutputFormat uint32,
) bool {
	xform := (*cmsTRANSFORM)(hTransform)

	// Ensure the transform supports format change
	if xform.DwOriginalFlags&cmsFLAGS_CAN_CHANGE_FORMATTER == 0 {
		cmsSignalError(unsafe.Pointer(xform.ContextID), cmsERROR_NOT_SUITABLE, "cmsChangeBuffersFormat works only on transforms created originally with at least 16 bits of precision")
		return false
	}

	FromInput := cmsGetFormatter(xform.ContextID, InputFormat, cmsFormatterInput, CMS_PACK_FLAGS_16BITS).Fmt16
	ToOutput := cmsGetFormatter(xform.ContextID, OutputFormat, cmsFormatterOutput, CMS_PACK_FLAGS_16BITS).Fmt16

	if FromInput == nil || ToOutput == nil {
		cmsSignalError(unsafe.Pointer(xform.ContextID), cmsERROR_UNKNOWN_EXTENSION, "Unsupported raster format")
		return false
	}

	xform.InputFormat = InputFormat
	xform.OutputFormat = OutputFormat
	xform.FromInput = FromInput
	xform.ToOutput = ToOutput
	return true
}
