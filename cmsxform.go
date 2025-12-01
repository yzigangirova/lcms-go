package golcms

import (
	//"errors"
	"runtime"
	"sync"
	"unsafe"

	//"bytes"
	//"encoding/binary"

	"github.com/yzigangirova/lcms-go/mem"
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
func cmsAllocAdaptationStateChunk(mm mem.Manager, ctx CmsContext, src CmsContext) {
	// Default adaptation state chunk used when no source is provided.
	defaultAdaptationStateChunk := cmsAdaptationStateChunkType{
		AdaptationState: DEFAULT_OBSERVER_ADAPTATION_STATE,
	}

	var from any
	if src != nil {
		from = src.chunks[AdaptationStateContext]
	} else {
		from = &defaultAdaptationStateChunk
	}

	ctx.chunks[AdaptationStateContext] = cmsSubAllocDup(mm, ctx.MemPool, from, uint32(unsafe.Sizeof(cmsAdaptationStateChunkType{})))
}

// Sets adaptation state for absolute colorimetric intent in the given context.  Adaptation state applies on all
// but cmsCreateExtendedTransformTHR().  Little CMS can handle incomplete adaptation states.
func cmsSetAdaptationStateTHR(ContextID CmsContext, d float64) float64 {

	ptr := CmsContextGetClientChunk(ContextID, AdaptationStateContext).(*cmsAdaptationStateChunkType)

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
//
//lint:ignore U1000 kept for parity with lcms; used in future ports
func cmsSetAlarmCodesTHR(ContextID CmsContext, AlarmCodesP []uint16) {
	//alarmCodeMutex.Lock()
	//defer alarmCodeMutex.Unlock()

	ContextAlarmCodes := CmsContextGetClientChunk(ContextID, AlarmCodesContext).(*cmsAlarmCodesChunkType)
	cmsAssert(ContextAlarmCodes != nil, "ContextAlarmCodes is nil")

	MemcpySlice(ContextAlarmCodes.AlarmCodes[:], AlarmCodesP[:], 16)
}

// cmsGetAlarmCodesTHR gets the alarm codes for a specific context.
//
//lint:ignore U1000 kept for parity with lcms; used in future ports
func cmsGetAlarmCodesTHR(ContextID CmsContext, AlarmCodesP []uint16) {
	//alarmCodeMutex.Lock()
	//defer alarmCodeMutex.Unlock()

	ContextAlarmCodes := CmsContextGetClientChunk(ContextID, AlarmCodesContext).(*cmsAlarmCodesChunkType)
	cmsAssert(ContextAlarmCodes != nil, "ContextAlarmCodes is nil")

	MemcpySlice(AlarmCodesP[:], ContextAlarmCodes.AlarmCodes[:], 16)
}

// cmsSetAlarmCodes sets the global alarm codes.
//
//lint:ignore U1000 kept for parity with lcms; used in future ports
func cmsSetAlarmCodes(newAlarm []uint16) {
	if len(newAlarm) < cmsMAXCHANNELS {
		panic("oldAlarm must have length >= cmsMAXCHANNELS")
	}
	cmsSetAlarmCodesTHR(nil, newAlarm)
}

// cmsGetAlarmCodes gets the global alarm codes.
//
//lint:ignore U1000 kept for parity with lcms; used in future ports
func cmsGetAlarmCodes(oldAlarm []uint16) {
	if len(oldAlarm) < cmsMAXCHANNELS {
		panic("oldAlarm must have length >= cmsMAXCHANNELS")
	}
	cmsGetAlarmCodesTHR(nil, oldAlarm) // THR should also accept []uint16
}

// cmsAllocAlarmCodesChunk initializes and duplicates alarm codes.
func cmsAllocAlarmCodesChunk(mm mem.Manager, ctx CmsContext, src CmsContext) {
	// Define the static default alarm codes chunk
	AlarmCodesChunk := &cmsAlarmCodesChunkType{
		AlarmCodes: DEFAULT_ALARM_CODES_VALUE,
	}

	var from any

	// Check if src is not nil
	if src != nil {
		// Access the chunk from the source context
		from = src.chunks[AlarmCodesContext]
	} else {
		// Use the static default chunk
		from = AlarmCodesChunk
	}

	// Allocate and duplicate the chunk in the context's memory pool
	ctx.chunks[AlarmCodesContext] = cmsSubAllocDup(mm, ctx.MemPool, from, uint32(unsafe.Sizeof(cmsAlarmCodesChunkType{})))
}

// -----------------------------------------------------------------------

// cmsDeleteTransform releases the resources associated with a transform.
func CmsDeleteTransform(hTransform CmsHTRANSFORM) {
	//fmt.Println("cmsDeleteTransform")
	p := hTransform.(*cmsTRANSFORM)

	if p == nil {
		return
	}

	// Free GamutCheck pipeline if it exists
	if p.GamutCheck != nil {
		cmsPipelineFree(p.mem_manager, p.GamutCheck)
	}

	// Free the LUT pipeline if it exists
	if p.Lut != nil {
		//	fmt.Printf(" cmsPipelineFree pipeline ptr = %p\n", p.Lut)

		cmsPipelineFree(p.mem_manager, p.Lut)
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
	cmsFree(p.ContextID, p)

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

// BytesPerPixel returns how many bytes a single *pixel* occupies
// in memory for a given LCMS packed format. This is different from
// PixelSize, which is bytes per *sample* (channel).
func BytesPerPixel(fmt uint32) int {
    // Number of color channels (RGB=3, CMYK=4, Gray=1, etc.)
    nChan := int(T_CHANNELS(fmt))
    if nChan == 0 {
        nChan = 1
    }

    // Extra channels (alpha, spot, etc.)
    extra := int(T_EXTRA(fmt))
    total := nChan + extra
    if total <= 0 {
        total = 1
    }

    // Bytes per sample (per channel)
    b := int(T_BYTES(fmt))
    switch b {
    case 0:
        // LCMS uses 0 for "double" formats – 8 bytes per sample.
        // (You can refine this later if you support floats explicitly.)
        b = 8
    }

    // For now, only support chunky formats in the parallel path.
    if T_PLANAR(fmt) != 0 {
        panic("BytesPerPixel: planar formats not supported in parallel path yet")
    }

    return total * b
}

func CmsDoTransformParallel(
	_ mem.Manager,
	xform CmsHTRANSFORM,
	in, out []byte,
	pixels int,
	workers int,
) {
	if pixels <= 0 {
		return
	}
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	if workers <= 1 || pixels == 1 {
		// Fallback: single-threaded batch
		// Use a local Manager for scratch:
		m := mem.NewManager()
		defer m.FreeAll()
		CmsDoTransform(m, xform, in, out, uint32(pixels))
		return
	}
	if workers > pixels {
		workers = pixels
	}

	p := xform.(*cmsTRANSFORM)
	inPixSize := int(BytesPerPixel(p.InputFormat))
	outPixSize := int(BytesPerPixel(p.OutputFormat))

	if len(in) < pixels*inPixSize || len(out) < pixels*outPixSize {
		panic("CmsDoTransformParallel: buffers too small for given pixel count")
	}

	chunk := (pixels + workers - 1) / workers

	var wg sync.WaitGroup
	wg.Add(workers)

	for w := 0; w < workers; w++ {
		startPx := w * chunk
		if startPx >= pixels {
			wg.Done()
			continue
		}
		endPx := startPx + chunk
		if endPx > pixels {
			endPx = pixels
		}
		nPix := endPx - startPx

		inOff := startPx * inPixSize
		outOff := startPx * outPixSize

		inSlice := in[inOff : inOff+nPix*inPixSize]
		outSlice := out[outOff : outOff+nPix*outPixSize]

		wgPtr := &wg
		go func(inBuf, outBuf []byte, n uint32) {
			defer wgPtr.Done()

			// Each goroutine gets its *own* Manager with its own Scratch.
			m := mem.NewManager()
			defer m.FreeAll()

			CmsDoTransform(m, xform, inBuf, outBuf, n)
		}(inSlice, outSlice, uint32(nPix))
	}

	wg.Wait()
}

// cmsDoTransform applies a transformation to the input buffer and writes the result to the output buffer.
func CmsDoTransform(mm mem.Manager, Transform CmsHTRANSFORM, InputBuffer, OutputBuffer any, Size uint32) {

	//fmt.Printf("start CmsDoTransform\n")

	p, ok := Transform.(*cmsTRANSFORM) // Cast the generic Transform to the specific type cmsTRANSFORM
	if !ok {
		panic("p is not of the type cmsTransform")
	}
	//the main memory manager is stored inside cmsTransform.  The separate memory manager for each
	//cmsDoTransform may be provided for concurrent transforming
	if mm.IsZero() {
		mm = p.mem_manager
	}
	var stride cmsStride

	// Initialize stride parameters
	stride.BytesPerLineIn = 0 // Not used
	stride.BytesPerLineOut = 0
	stride.BytesPerPlaneIn = Size * PixelSize(p.InputFormat)
	stride.BytesPerPlaneOut = Size * PixelSize(p.OutputFormat)

	// Perform the transformation
	p.Xform(mm, p, InputBuffer, OutputBuffer, Size, 1, &stride)

	//fmt.Println("end CmsDoTransform")

}

func CmsDoTransformStride(mm mem.Manager,
	Transform CmsHTRANSFORM,
	InputBuffer, OutputBuffer any,
	Size uint32,
	Stride uint32) {

	p := Transform.(*cmsTRANSFORM)
	//the main memory manager is stored inside cmsTransform.  The separate memory manager for each
	//cmsDoTransform may be provided for concurrent transforming
	if mm.IsZero() {
		mm = p.mem_manager
	}
	var stride cmsStride

	stride.BytesPerLineIn = 0
	stride.BytesPerLineOut = 0
	stride.BytesPerPlaneIn = Stride
	stride.BytesPerPlaneOut = Stride

	p.Xform(mm, p, InputBuffer, OutputBuffer, Size, 1, &stride)
}

func CmsDoTransformLineStride(mm mem.Manager,
	Transform CmsHTRANSFORM,
	InputBuffer,
	OutputBuffer any,
	PixelsPerLine uint32,
	LineCount uint32,
	BytesPerLineIn uint32,
	BytesPerLineOut uint32,
	BytesPerPlaneIn uint32,
	BytesPerPlaneOut uint32) {

	p := Transform.(*cmsTRANSFORM)
	//the main memory manager is stored inside cmsTransform.  The separate memory manager for each
	//cmsDoTransform may be provided for concurrent transforming
	if mm.IsZero() {
		mm = p.mem_manager
	}
	var stride cmsStride

	stride.BytesPerLineIn = BytesPerLineIn
	stride.BytesPerLineOut = BytesPerLineOut
	stride.BytesPerPlaneIn = BytesPerPlaneIn
	stride.BytesPerPlaneOut = BytesPerPlaneOut

	p.Xform(mm, p, InputBuffer, OutputBuffer, PixelsPerLine, LineCount, &stride)
}

// Transform routines ----------------------------------------------------------------------------------------------------------

// Float xform converts floats. Since there are no performance issues, one routine does all job, including gamut check.
// Note that because extended range, we can use a -1.0 value for out of gamut in this case.

func FloatXFORM(mm mem.Manager,
	p *cmsTRANSFORM,
	in, out any,
	PixelsPerLine, LineCount uint32,
	Stride *cmsStride,
) {

	//fmt.Println("FloatXFORM")

	sc := mm.Scratch()
	fIn := sc.WInF32
	fOut := sc.WOutF32
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
		inBytes = Float32sToBytesLE(v)
	case []float64:
		/*	fmt.Printf("v[0] %.7f\n", v[0])
			fmt.Printf("v[1] %.7f\n", v[1])
			fmt.Printf("v[2] %.7f\n", v[2])*/
		inBytes = Float64sToBytesLE(v)
	case []uint16:
		inBytes = Uint16sToBytesLE(v)
	case *cmsCIELab:
		inBytes = Float64sToBytesLE(LabToSlice(*v))
	default:
		panic("Error: 'in' must be of type []byte, []float32, []float64, or []uint16 , or *cmsCIELab")
	}

	// Type assertion and conversion for output
	switch v := out.(type) {
	case []byte:
		outBytes = v
	case []float32:
		outBytes = Float32sToBytesLE(v)
	case []float64:
		outBytes = Float64sToBytesLE(v)
	case []uint16:
		outBytes = Uint16sToBytesLE(v)
	case *cmsCIELab:
		outBytes = Float64sToBytesLE(LabToSlice(*v))
	default:
		panic("Error: 'out' must be of type []byte, []float32, []float64, or []uint16, or *cmsCIELab")
	}

	/*fmt.Printf("inBytes[0] %df\n", inBytes[0])
	fmt.Printf("inBytes[1] %d\n", inBytes[1])
	fmt.Printf("inBytes[2] %d\n", inBytes[2])*/

	cmsHandleExtraChannels(p, in, out, PixelsPerLine, LineCount, Stride)

	strideIn, strideOut = 0, 0

	for i := uint32(0); i < LineCount; i++ {
		//  Use slices with offsets instead of unsafe
		accum = inBytes[strideIn:]
		output = outBytes[strideOut:]

		for j := uint32(0); j < PixelsPerLine; j++ {
			//  Process input correctly using slice indexing
			accum = p.FromInputFloat(mm, p, fIn[:], accum, Stride.BytesPerPlaneIn)

			//  Replace unsafe pointer arithmetic for `OutOfGamut`
			outOfGamutSlice := []float32{OutOfGamut}

			if p.GamutCheck != nil {
				//  Use slice indexing instead of pointer casting
				cmsPipelineEvalFloat(mm, fIn[:], outOfGamutSlice, p.GamutCheck)

				if outOfGamutSlice[0] > 0.0 {
					//  Mark all output channels as out of gamut efficiently
					for c := range fOut {
						fOut[c] = -1.0
					}
				} else {
					//  Evaluate the pipeline normally
					cmsPipelineEvalFloat(mm, fIn[:], fOut[:], p.Lut)
				}
			} else {
				//  No gamut check; evaluate pipeline directly
				/*fmt.Printf("fIn[0] %.7f\n", fIn[0])
				fmt.Printf("fIn[1] %.7f\n", fIn[1])
				fmt.Printf("fIn[2] %.7f\n", fIn[2])*/

				cmsPipelineEvalFloat(mm, fIn[:], fOut[:], p.Lut)
			}

			//  Process output correctly
			//	fmt.Println("fOut[:] ", fOut[:])

			output = p.ToOutputFloat(mm, p, fOut[:], output, Stride.BytesPerPlaneOut)
		}

		//  Update strides correctly
		strideIn += Stride.BytesPerLineIn
		strideOut += Stride.BytesPerLineOut

		//fmt.Println("outBytes ", outBytes)
		/*fmt.Printf("outBytes[0] %d\n", outBytes[0])
		fmt.Printf("outBytes[1] %d\n", outBytes[1])
		fmt.Printf("outBytes[2] %d\n", outBytes[2])*/
	}
	switch v := out.(type) {
	case []byte:
		copy(v, outBytes)

	case []float32:
		writeIntoFloat32Slice(v, outBytes)

	case []float64:
		writeIntoFloat64Slice(v, outBytes)

	case []uint16:
		writeIntoUint16Slice(v, outBytes)

	case *cmsCIELab:
		lab := bytesToLab(outBytes)
		v.L = lab.L
		v.a = lab.a
		v.b = lab.b

	default:
		panic("Unsupported type in FloatXFORM output finalization")
	}

}

func NullFloatXFORM(mm mem.Manager,
	p *cmsTRANSFORM,
	in, out any,
	PixelsPerLine, LineCount uint32,
	Stride *cmsStride,
) {
	//fmt.Println("NullXFORM ")

	sc := mm.Scratch()
	fIn := sc.WInF32
	var strideIn, strideOut uint32
	var accum, output []byte
	var inBytes, outBytes []byte
	// Type assertion for input and output
	// Type assertion and conversion for input
	switch v := in.(type) {
	case []byte:
		inBytes = v
	case []float32:
		inBytes = Float32sToBytesLE(v)
	case []float64:
		/*	fmt.Printf("v[0] %.7f\n", v[0])
			fmt.Printf("v[1] %.7f\n", v[1])
			fmt.Printf("v[2] %.7f\n", v[2])*/
		inBytes = Float64sToBytesLE(v)
	case []uint16:
		inBytes = Uint16sToBytesLE(v)
	case *cmsCIELab:
		inBytes = Float64sToBytesLE(LabToSlice(*v))
	default:
		panic("Error: 'in' must be of type []byte, []float32, []float64, or []uint16 , or *cmsCIELab")
	}

	// Type assertion and conversion for output
	switch v := out.(type) {
	case []byte:
		outBytes = v
	case []float32:
		outBytes = Float32sToBytesLE(v)
	case []float64:
		outBytes = Float64sToBytesLE(v)
	case []uint16:
		outBytes = Uint16sToBytesLE(v)
	case *cmsCIELab:
		outBytes = Float64sToBytesLE(LabToSlice(*v))
	default:
		panic("Error: 'out' must be of type []byte, []float32, []float64, or []uint16, or *cmsCIELab")
	}

	cmsHandleExtraChannels(p, in, out, PixelsPerLine, LineCount, Stride)

	strideIn, strideOut = 0, 0

	for i := uint32(0); i < LineCount; i++ {
		//  Use slices with offsets instead of unsafe
		accum = inBytes[strideIn:]
		output = outBytes[strideOut:]

		for j := uint32(0); j < PixelsPerLine; j++ {
			//  Process input correctly using slice indexing
			accum = p.FromInputFloat(mm, p, fIn[:], accum, Stride.BytesPerPlaneIn)
			output = p.ToOutputFloat(mm, p, fIn[:], output, Stride.BytesPerPlaneOut)
		}

		//  Update strides correctly
		strideIn += Stride.BytesPerLineIn
		strideOut += Stride.BytesPerLineOut
	}
	switch v := out.(type) {
	case []byte:
		copy(v, outBytes)

	case []float32:
		writeIntoFloat32Slice(v, outBytes)

	case []float64:
		writeIntoFloat64Slice(v, outBytes)

	case []uint16:
		writeIntoUint16Slice(v, outBytes)

	case *cmsCIELab:
		lab := bytesToLab(outBytes)
		v.L = lab.L
		v.a = lab.a
		v.b = lab.b

	default:
		panic("Unsupported type in NullFloatXFORM output finalization")
	}
}

func NullXFORM(mm mem.Manager,
	p *cmsTRANSFORM,
	in, out any,
	PixelsPerLine, LineCount uint32,
	Stride *cmsStride,
) {
	sc := mm.Scratch()
	wIn := sc.WInU16
	var strideIn, strideOut uint32
	var accum, output []byte
	var inBytes, outBytes []byte
	// Type assertion for input and output
	// Type assertion and conversion for input
	switch v := in.(type) {
	case []byte:
		inBytes = v
	case []float32:
		inBytes = Float32sToBytesLE(v)
	case []float64:
		/*	fmt.Printf("v[0] %.7f\n", v[0])
			fmt.Printf("v[1] %.7f\n", v[1])
			fmt.Printf("v[2] %.7f\n", v[2])*/
		inBytes = Float64sToBytesLE(v)
	case []uint16:
		inBytes = Uint16sToBytesLE(v)
	case *cmsCIELab:
		inBytes = Float64sToBytesLE(LabToSlice(*v))
	default:
		panic("Error: 'in' must be of type []byte, []float32, []float64, or []uint16 , or *cmsCIELab")
	}

	// Type assertion and conversion for output
	switch v := out.(type) {
	case []byte:
		outBytes = v
	case []float32:
		outBytes = Float32sToBytesLE(v)
	case []float64:
		outBytes = Float64sToBytesLE(v)
	case []uint16:
		outBytes = Uint16sToBytesLE(v)
	case *cmsCIELab:
		outBytes = Float64sToBytesLE(LabToSlice(*v))
	default:
		panic("Error: 'out' must be of type []byte, []float32, []float64, or []uint16, or *cmsCIELab")
	}

	cmsHandleExtraChannels(p, in, out, PixelsPerLine, LineCount, Stride)

	strideIn, strideOut = 0, 0

	for i := uint32(0); i < LineCount; i++ {
		//  Use slices with offsets instead of unsafe
		accum = inBytes[strideIn:]
		output = outBytes[strideOut:]

		for j := uint32(0); j < PixelsPerLine; j++ {
			//  Process input correctly using slice indexing
			accum = p.FromInput(mm, p, wIn[:], accum, Stride.BytesPerPlaneIn)
			output = p.ToOutput(mm, p, wIn[:], output, Stride.BytesPerPlaneOut)
		}

		//  Update strides correctly
		strideIn += Stride.BytesPerLineIn
		strideOut += Stride.BytesPerLineOut
	}
	switch v := out.(type) {
	case []byte:
		copy(v, outBytes)

	case []float32:
		writeIntoFloat32Slice(v, outBytes)

	case []float64:
		writeIntoFloat64Slice(v, outBytes)

	case []uint16:
		writeIntoUint16Slice(v, outBytes)

	case *cmsCIELab:
		lab := bytesToLab(outBytes)
		v.L = lab.L
		v.a = lab.a
		v.b = lab.b

	default:
		panic("Unsupported type in NullXFORM output finalization")
	}
}

// eval16 dispatches to the zero-closure fast path when available.
func eval16(mm mem.Manager, lut *cmsPipeline, in, out []uint16) {
	if lut.fastEval16 != nil {
		lut.fastEval16(mm, in, out, lut.fastParams)
	} else {
		lut.Eval16Fn(mm, in, out, lut.Data)
	}
}

func PrecalculatedXFORM(mm mem.Manager,
	p *cmsTRANSFORM,
	in, out any,
	PixelsPerLine, LineCount uint32,
	Stride *cmsStride,
) {
	//("PrecalculatedXFORM ")

	sc := mm.Scratch()
	wIn := sc.WInU16
	wOut := sc.WOutU16
	var strideIn, strideOut uint32
	var accum, output []byte
	var inBytes, outBytes []byte
	// Type assertion and conversion for input
	switch v := in.(type) {
	case []byte:
		inBytes = v
	case []float32:
		inBytes = Float32sToBytesLE(v)
	case []float64:
		/*	fmt.Printf("v[0] %.7f\n", v[0])
			fmt.Printf("v[1] %.7f\n", v[1])
			fmt.Printf("v[2] %.7f\n", v[2])*/
		inBytes = Float64sToBytesLE(v)
	case []uint16:
		inBytes = Uint16sToBytesLE(v)
	case *cmsCIELab:
		inBytes = Float64sToBytesLE(LabToSlice(*v))
	default:
		panic("Error: 'in' must be of type []byte, []float32, []float64, or []uint16 , or *cmsCIELab")
	}

	// Type assertion and conversion for output
	switch v := out.(type) {
	case []byte:
		outBytes = v
	case []float32:
		outBytes = Float32sToBytesLE(v)
	case []float64:
		outBytes = Float64sToBytesLE(v)
	case []uint16:
		outBytes = Uint16sToBytesLE(v)
	case *cmsCIELab:
		outBytes = Float64sToBytesLE(LabToSlice(*v))
	default:
		panic("Error: 'out' must be of type []byte, []float32, []float64, or []uint16, or *cmsCIELab")
	}

	cmsHandleExtraChannels(p, in, out, PixelsPerLine, LineCount, Stride)

	strideIn, strideOut = 0, 0

	for i := uint32(0); i < LineCount; i++ {
		// Accumulator slices for this line
		accum = inBytes[strideIn:]
		output = outBytes[strideOut:]

		for j := uint32(0); j < PixelsPerLine; j++ {
			// Process input
			accum = p.FromInput(mm, p, wIn[:], accum, Stride.BytesPerPlaneIn)
			// Evaluate LUT
			eval16(mm, p.Lut, wIn[:], wOut[:])

			// Process output
			output = p.ToOutput(mm, p, wOut[:], output, Stride.BytesPerPlaneOut)
		}

		// Update strides
		strideIn += Stride.BytesPerLineIn
		strideOut += Stride.BytesPerLineOut
	}
	switch v := out.(type) {
	case []byte:
		copy(v, outBytes)

	case []float32:
		writeIntoFloat32Slice(v, outBytes)

	case []float64:
		writeIntoFloat64Slice(v, outBytes)

	case []uint16:
		writeIntoUint16Slice(v, outBytes)

	case *cmsCIELab:
		lab := bytesToLab(outBytes)
		v.L = lab.L
		v.a = lab.a
		v.b = lab.b

	default:
		panic("Unsupported type in PrecalculatedXFORMoutput finalization")
	}
}

// Auxiliary: Handle precalculated gamut check. The retrieval of context may be alittle bit slow, but this function is not critical.
func TransformOnePixelWithGamutCheck(mm mem.Manager, p *cmsTRANSFORM, wIn, wOut []uint16) {
	var wOutOfGamut uint16

	woutOfGamutSlice := []uint16{wOutOfGamut}
	// Evaluate the gamut check function
	p.GamutCheck.Eval16Fn(mm, wIn, woutOfGamutSlice, p.GamutCheck.Data)

	if woutOfGamutSlice[0] >= 1 {
		// If out of gamut, use alarm codes
		contextAlarmCodes := CmsContextGetClientChunk(p.ContextID, AlarmCodesContext).(*cmsAlarmCodesChunkType)
		for i := uint32(0); i < p.Lut.OutputChannels; i++ {
			wOut[i] = contextAlarmCodes.AlarmCodes[i]
		}
	} else {
		// Otherwise, evaluate the LUT
		eval16(mm, p.Lut, wIn, wOut)
	}
}

func PrecalculatedXFORMGamutCheck(mm mem.Manager,
	p *cmsTRANSFORM,
	in, out any,
	PixelsPerLine, LineCount uint32,
	Stride *cmsStride,
) {
	//fmt.Println("PrecalculatedXFORMGamutCheck ")

	sc := mm.Scratch()
	wIn := sc.WInU16
	wOut := sc.WOutU16
	var strideIn, strideOut uint32
	var accum, output []byte
	var inBytes, outBytes []byte
	// Type assertion for input and output
	// Type assertion and conversion for input
	switch v := in.(type) {
	case []byte:
		inBytes = v
	case []float32:
		inBytes = Float32sToBytesLE(v)
	case []float64:
		/*	fmt.Printf("v[0] %.7f\n", v[0])
			fmt.Printf("v[1] %.7f\n", v[1])
			fmt.Printf("v[2] %.7f\n", v[2])*/
		inBytes = Float64sToBytesLE(v)
	case []uint16:
		inBytes = Uint16sToBytesLE(v)
	case *cmsCIELab:
		inBytes = Float64sToBytesLE(LabToSlice(*v))
	default:
		panic("Error: 'in' must be of type []byte, []float32, []float64, or []uint16 , or *cmsCIELab")
	}

	// Type assertion and conversion for output
	switch v := out.(type) {
	case []byte:
		outBytes = v
	case []float32:
		outBytes = Float32sToBytesLE(v)
	case []float64:
		outBytes = Float64sToBytesLE(v)
	case []uint16:
		outBytes = Uint16sToBytesLE(v)
	case *cmsCIELab:
		outBytes = Float64sToBytesLE(LabToSlice(*v))
	default:
		panic("Error: 'out' must be of type []byte, []float32, []float64, or []uint16, or *cmsCIELab")
	}

	cmsHandleExtraChannels(p, in, out, PixelsPerLine, LineCount, Stride)

	strideIn, strideOut = 0, 0

	for i := uint32(0); i < LineCount; i++ {
		// Use slices with offsets instead of large allocation
		accum = inBytes[strideIn:]
		output = outBytes[strideOut:]

		for j := uint32(0); j < PixelsPerLine; j++ {
			// Correctly advance accum and output slices
			accum = p.FromInput(mm, p, wIn[:], accum, Stride.BytesPerPlaneIn)
			TransformOnePixelWithGamutCheck(mm, p, wIn[:], wOut[:])
			output = p.ToOutput(mm, p, wOut[:], output, Stride.BytesPerPlaneOut)
		}

		// Update strides correctly
		strideIn += Stride.BytesPerLineIn
		strideOut += Stride.BytesPerLineOut
	}
	switch v := out.(type) {
	case []byte:
		copy(v, outBytes)

	case []float32:
		writeIntoFloat32Slice(v, outBytes)

	case []float64:
		writeIntoFloat64Slice(v, outBytes)

	case []uint16:
		writeIntoUint16Slice(v, outBytes)

	case *cmsCIELab:
		lab := bytesToLab(outBytes)
		v.L = lab.L
		v.a = lab.a
		v.b = lab.b

	default:
		panic("Unsupported type in PrecalculatedXFORMGamutCheck output finalization")
	}
}
func CachedXFORM(
	mm mem.Manager,
	p *cmsTRANSFORM,
	in, out any,
	PixelsPerLine, LineCount uint32,
	Stride *cmsStride,
) {
	// --- Scratch once
	sc := mm.Scratch()
	wIn := sc.WInU16 // len >= cmsMAXCHANNELS (16)
	wOut := sc.WOutU16

	// --- Derive channel counts (input side is what the cache compares)
	nIn := channelsOf(p.EntryColorSpace) // Gray=1, RGB/Lab/XYZ=3, CMYK=4, else clamp [1..16]
	if nIn < 1 {
		nIn = 1
	} else if nIn > 16 {
		nIn = 16
	}

	// --- Normalize input/output into byte slices exactly once (no per-pixel switches)
	var inBytes, outBytes []byte
	switch v := in.(type) {
	case []byte:
		inBytes = v
	case []float32:
		inBytes = Float32sToBytesLE(v)
	case []float64:
		inBytes = Float64sToBytesLE(v)
	case []uint16:
		inBytes = Uint16sToBytesLE(v) // allocates once; safe and simple
	case *cmsCIELab:
		inBytes = Float64sToBytesLE(LabToSlice(*v))
	default:
		panic("CachedXFORM: unsupported input type")
	}

	switch v := out.(type) {
	case []byte:
		outBytes = v
	case []float32:
		outBytes = Float32sToBytesLE(v)
	case []float64:
		outBytes = Float64sToBytesLE(v)
	case []uint16:
		outBytes = Uint16sToBytesLE(v)
	case *cmsCIELab:
		outBytes = Float64sToBytesLE(LabToSlice(*v))
	default:
		panic("CachedXFORM: unsupported output type")
	}

	// --- Handle extra channels once
	cmsHandleExtraChannels(p, in, out, PixelsPerLine, LineCount, Stride)

	// --- Local copies / aliases to avoid repeated indirections
	cache := p.Cache
	fromIn := p.FromInput
	toOut := p.ToOutput
	eval := eval16

	// Fast local stride vars (bytes)
	var strideIn, strideOut uint32
	if Stride != nil {
		strideIn = Stride.BytesPerLineIn
		strideOut = Stride.BytesPerLineOut
	} else {
		// Fallback: planes are contiguous if Stride is nil (rare path)
		strideIn, strideOut = 0, 0
	}

	// --- Inner loops: tight, branch-light
	for i := uint32(0); i < LineCount; i++ {

		// Slice windows for this scanline
		accum := inBytes[strideIn:]
		output := outBytes[strideOut:]

		for j := uint32(0); j < PixelsPerLine; j++ {
			// Decode one pixel to wIn; accum advanced by BytesPerPlaneIn
			accum = fromIn(mm, p, wIn[:], accum, Stride.BytesPerPlaneIn)

			// Check cache on the *actual* channels only
			hit := true
			// manual unroll gives a tiny edge for RGB/CMYK common cases
			switch nIn {
			case 1:
				hit = (wIn[0] == cache.CacheIn[0])
			case 3:
				hit = (wIn[0] == cache.CacheIn[0] &&
					wIn[1] == cache.CacheIn[1] &&
					wIn[2] == cache.CacheIn[2])
			case 4:
				hit = (wIn[0] == cache.CacheIn[0] &&
					wIn[1] == cache.CacheIn[1] &&
					wIn[2] == cache.CacheIn[2] &&
					wIn[3] == cache.CacheIn[3])
			default:
				for k := 0; k < nIn; k++ {
					if wIn[k] != cache.CacheIn[k] {
						hit = false
						break
					}
				}
			}

			if hit {
				// Copy cached output (only relevant lanes; copying 16 is cheap and branchless)
				copy(wOut[:], cache.CacheOut[:])
			} else {
				// Evaluate LUT
				eval(mm, p.Lut, wIn[:], wOut[:])
				// Update cache for next pixel
				copy(cache.CacheIn[:nIn], wIn[:nIn])
				copy(cache.CacheOut[:], wOut[:])
			}

			// Encode one pixel from wOut; output advanced by BytesPerPlaneOut
			output = toOut(mm, p, wOut[:], output, Stride.BytesPerPlaneOut)
		}

		// Advance to next line
		strideIn += Stride.BytesPerLineIn
		strideOut += Stride.BytesPerLineOut
	}

	// --- Finalize (write back only when needed)
	switch v := out.(type) {
	case []byte:
		// outBytes already aliases v; nothing to do
	case []float32:
		writeIntoFloat32Slice(v, outBytes)
	case []float64:
		writeIntoFloat64Slice(v, outBytes)
	case []uint16:
		writeIntoUint16Slice(v, outBytes)
	case *cmsCIELab:
		lab := bytesToLab(outBytes)
		v.L, v.a, v.b = lab.L, lab.a, lab.b
	}
}

// channelsOf returns the canonical number of components for a color space signature.
func channelsOf(sig cmsColorSpaceSignature) int {
	switch sig {
	case CmsSigGrayData:
		return 1
	case CmsSigRgbData, CmsSigLabData, CmsSigXYZData:
		return 3
	case CmsSigCmykData:
		return 4
	// Add other spaces as you wire them (e.g.,  n-color deviceN if ever needed)
	default:
		return 4 // conservative default; prevents zero, caps to 16 above
	}
}

func CachedXFORMGamutCheck(mm mem.Manager,
	p *cmsTRANSFORM,
	in, out any,
	PixelsPerLine, LineCount uint32,
	Stride *cmsStride,
) {
	//fmt.Println("CachedXFORMGamutCheck ")

	sc := mm.Scratch()
	wIn := sc.WInU16
	wOut := sc.WOutU16
	var strideIn, strideOut uint32
	var cache cmsCACHE
	var accum, output []byte
	var inBytes, outBytes []byte
	// Type assertion and conversion for input
	switch v := in.(type) {
	case []byte:
		inBytes = v
	case []float32:
		inBytes = Float32sToBytesLE(v)
	case []float64:
		/*	fmt.Printf("v[0] %.7f\n", v[0])
			fmt.Printf("v[1] %.7f\n", v[1])
			fmt.Printf("v[2] %.7f\n", v[2])*/
		inBytes = Float64sToBytesLE(v)
	case []uint16:
		inBytes = Uint16sToBytesLE(v)
	case *cmsCIELab:
		inBytes = Float64sToBytesLE(LabToSlice(*v))
	default:
		panic("Error: 'in' must be of type []byte, []float32, []float64, or []uint16 , or *cmsCIELab")
	}

	// Type assertion and conversion for output
	switch v := out.(type) {
	case []byte:
		outBytes = v
	case []float32:
		outBytes = Float32sToBytesLE(v)
	case []float64:
		outBytes = Float64sToBytesLE(v)
	case []uint16:
		outBytes = Uint16sToBytesLE(v)
	case *cmsCIELab:
		outBytes = Float64sToBytesLE(LabToSlice(*v))
	default:
		panic("Error: 'out' must be of type []byte, []float32, []float64, or []uint16, or *cmsCIELab")
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
			accum = p.FromInput(mm, p, wIn[:], accum, Stride.BytesPerPlaneIn)

			//  Use cache for performance optimization
			// Use cache to avoid redundant calculations
			equal := true
			for i := 0; i < 16; i++ {
				if wIn[i] != cache.CacheIn[i] {
					equal = false
					break
				}
			}
			if equal {
				copy(wOut[:], cache.CacheOut[:])
			} else {
				TransformOnePixelWithGamutCheck(mm, p, wIn[:], wOut[:])
				copy(cache.CacheIn[:], wIn[:])
				copy(cache.CacheOut[:], wOut[:])
			}

			//  Correctly advance output using slices
			output = p.ToOutput(mm, p, wOut[:], output, Stride.BytesPerPlaneOut)
		}

		//  Update strides correctly
		strideIn += Stride.BytesPerLineIn
		strideOut += Stride.BytesPerLineOut
	}
	switch v := out.(type) {
	case []byte:
		copy(v, outBytes)

	case []float32:
		writeIntoFloat32Slice(v, outBytes)

	case []float64:
		writeIntoFloat64Slice(v, outBytes)

	case []uint16:
		writeIntoUint16Slice(v, outBytes)

	case *cmsCIELab:
		lab := bytesToLab(outBytes)
		v.L = lab.L
		v.a = lab.a
		v.b = lab.b

	default:
		panic("Unsupported type in CachedXFORMGamutCheck output finalization")
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
func DupPluginTransformList(mm mem.Manager, ctx CmsContext, src CmsContext) {
	var newHead cmsTransformPluginChunkType
	var entry, prev *cmsTransformCollection
	head := src.chunks[TransformPlugin].(*cmsTransformPluginChunkType)

	if head == nil {
		return
	}

	// Walk the list and copy each node.
	for entry = head.TransformCollection; entry != nil; entry = entry.Next {
		newEntry := new(cmsTransformCollection) // Heap allocation
		*newEntry = *entry
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

	ctx.chunks[TransformPlugin] = cmsSubAllocDup(mm, ctx.MemPool, &newHead, uint32(unsafe.Sizeof(newHead)))
}

// cmsAllocTransformPluginChunk allocates the transform plugin chunk.
func cmsAllocTransformPluginChunk(mm mem.Manager, ctx CmsContext, src CmsContext) {
	if src != nil {
		DupPluginTransformList(mm, ctx, src)
	} else {
		var defaultChunk cmsTransformPluginChunkType
		ctx.chunks[TransformPlugin] = cmsSubAllocDup(mm, ctx.MemPool, &defaultChunk, uint32(unsafe.Sizeof(defaultChunk)))
	}
}

// cmsTransform2toTransformAdaptor adapts new-style transforms to the old-style interface.
func cmsTransform2toTransformAdaptor(mm mem.Manager, cmmcargo *cmsTRANSFORM, in, out any, pixelsPerLine, lineCount uint32, stride *cmsStride) {
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

func cmsTransform2toTransformConverter(mm mem.Manager,
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
		p.Xform(mm, p, InputBuffer, OutputBuffer, PixelsPerLine, LineCount, nil)
	}
}

// cmsRegisterTransformPlugin registers a new transform plugin.
func cmsRegisterTransformPlugin(mm mem.Manager, ContextID CmsContext, Data PluginIntrfc) bool {
	ctx := ContextID.chunks[TransformPlugin].(*cmsTransformPluginChunkType)

	if Data == nil {
		// Free the chain. Memory is safely freed at exit.
		ctx.TransformCollection = nil
		return true
	}
	plugin, ok := Data.(*cmsPluginTransform)
	if !ok {
		panic("Plugin is not of the type cmsPluginTransform\n")
		return false
	}
	// Ensure the factory callback is present.
	if plugin.Factories.Xform == nil {
		return false
	}

	// Allocate memory for the transform collection.
	//fl := (*cmsTransformCollection)(cmsPluginMalloc(ContextID, uint32(unsafe.Sizeof(cmsTransformCollection{}))))
	fl := mem.New[cmsTransformCollection](mm)
	if fl == nil {
		return false
	}

	// Check for old-style transform plugins (pre-version 2.8).
	if plugin.GetBase().ExpectedVersion < 2080 {
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
	cmsAssert(cmmCargo != nil, "CMMcargo cannot be nil")

	cmmCargo.UserData = ptr
	cmmCargo.FreeUserData = freePrivateDataFn
}

// GetTransformUserData retrieves the user-defined data.
func GetTransformUserData(cmmCargo *cmsTRANSFORM) any {
	cmsAssert(cmmCargo != nil, "CMMcargo cannot be nil")

	return cmmCargo.UserData
}

// GetTransformFormatters16 retrieves the current 16-bit formatters.
func GetTransformFormatters16(cmmCargo *cmsTRANSFORM) (fromInput, toOutput cmsFormatter16) {
	cmsAssert(cmmCargo != nil, "CMMcargo cannot be nil")

	return cmmCargo.FromInput, cmmCargo.ToOutput
}

// GetTransformFormattersFloat retrieves the current float formatters.
func GetTransformFormattersFloat(cmmCargo *cmsTRANSFORM) (fromInputFloat, toOutputFloat cmsFormatterFloat) {
	cmsAssert(cmmCargo != nil, "CMMcargo cannot be nil")

	return cmmCargo.FromInputFloat, cmmCargo.ToOutputFloat
}

// GetTransformFlags retrieves the original flags.
func GetTransformFlags(cmmCargo *cmsTRANSFORM) uint32 {
	cmsAssert(cmmCargo != nil, "CMMcargo cannot be nil")

	return cmmCargo.DwOriginalFlags
}

// GetTransformWorker retrieves the worker callback for parallelization plugins.
func GetTransformWorker(cmmCargo *cmsTRANSFORM) cmsTransform2Fn {
	cmsAssert(cmmCargo != nil, "CMMcargo cannot be nil")

	return cmmCargo.Worker
}

// GetTransformMaxWorkers retrieves the maximum number of workers or -1 for auto.
func GetTransformMaxWorkers(cmmCargo *cmsTRANSFORM) int32 {
	cmsAssert(cmmCargo != nil, "CMMcargo cannot be nil")

	return cmmCargo.MaxWorkers
}

// GetTransformWorkerFlags retrieves the worker flags.
func GetTransformWorkerFlags(cmmCargo *cmsTRANSFORM) uint32 {
	cmsAssert(cmmCargo != nil, "CMMcargo cannot be nil")

	return cmmCargo.WorkerFlags
}

func ParallelizeIfSuitable(p *cmsTRANSFORM) {
	ctx := CmsContextGetClientChunk(p.ContextID, ParallelizationPlugin).(*cmsParallelizationPluginChunkType)

	if ctx != nil && ctx.SchedulerFn != nil {
		p.Worker = p.Xform
		p.Xform = ctx.SchedulerFn
		p.MaxWorkers = ctx.MaxWorkers
		p.WorkerFlags = uint32(ctx.WorkerFlags)
	}
}
func UnrollNothing(mm mem.Manager,
	info *cmsTRANSFORM,
	wIn []uint16,
	accum []uint8,
	Stride uint32,
) []uint8 {
	// No operation, return the input slice unchanged
	return accum
}
func PackNothing(mm mem.Manager,
	info *cmsTRANSFORM,
	wOut []uint16,
	output []uint8,
	Stride uint32,
) []uint8 {
	// No operation, return the output slice unchanged
	return output
}

func AllocEmptyTransform(mm mem.Manager,
	ContextID CmsContext,
	lut *cmsPipeline,
	Intent uint32,
	InputFormat, OutputFormat, dwFlags *uint32,
) *cmsTRANSFORM {
	//	fmt.Println("AllocEmptyTransform")
	// Get the transform plugin chunk
	ctx := CmsContextGetClientChunk(ContextID, TransformPlugin).(*cmsTransformPluginChunkType)
	var plugin *cmsTransformCollection

	// Allocate memory for the transform structure
	p := mem.New[cmsTRANSFORM](mm)
	if p == nil {
		cmsPipelineFree(mm, lut)
		return nil
	}
	p.mem_manager = mm

	// Store the proposed pipeline
	p.Lut = lut

	// Check if any plugin wants to handle the transform
	if p.Lut != nil {
		if (*dwFlags & CmsFLAGS_NOOPTIMIZE) == 0 {
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
								cmsTransform2toTransformConverter(mm, p, InputBuffer, OutputBuffer, Size, Stride)
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

		cmsOptimizePipeline(mm, ContextID, &p.Lut, Intent, InputFormat, OutputFormat, dwFlags)

	}

	// Check for floating-point transform
	if cmsFormatterIsFloat(*OutputFormat) {
		p.FromInputFloat = cmsGetFormatter(ContextID, *InputFormat, cmsFormatterInput, CMS_PACK_FLAGS_FLOAT).FmtFloat
		p.ToOutputFloat = cmsGetFormatter(ContextID, *OutputFormat, cmsFormatterOutput, CMS_PACK_FLAGS_FLOAT).FmtFloat
		*dwFlags |= cmsFLAGS_CAN_CHANGE_FORMATTER

		if p.FromInputFloat == nil || p.ToOutputFloat == nil {
			cmsSignalError(ContextID, cmsERROR_UNKNOWN_EXTENSION, "Unsupported raster format")
			CmsDeleteTransform(CmsHTRANSFORM(p))
			return nil
		}

		if (*dwFlags & CmsFLAGS_NULLTRANSFORM) != 0 {
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
				cmsSignalError(ContextID, cmsERROR_UNKNOWN_EXTENSION, "Unsupported raster format")
				CmsDeleteTransform(CmsHTRANSFORM(p))
				return nil
			}

			if T_BYTES(*InputFormat) >= 2 {
				*dwFlags |= cmsFLAGS_CAN_CHANGE_FORMATTER
			}
		}

		if (*dwFlags & CmsFLAGS_NULLTRANSFORM) != 0 {
			p.Xform = NullXFORM
		} else if (*dwFlags & CmsFLAGS_NOCACHE) != 0 {
			if (*dwFlags & CmsFLAGS_GAMUTCHECK) != 0 {
				p.Xform = PrecalculatedXFORMGamutCheck
			} else {
				p.Xform = PrecalculatedXFORM
			}
		} else {
			if (*dwFlags & CmsFLAGS_GAMUTCHECK) != 0 {
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

	//	fmt.Println("END AllocEmptyTransform")
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

		lIsInput := PostColorSpace != CmsSigXYZData && PostColorSpace != CmsSigLabData

		switch {
		case cls == CmsSigNamedColorClass:
			ColorSpaceIn = CmsSig1colorData
			if nProfiles > 1 {
				ColorSpaceOut = cmsGetPCS(hProfile)
			} else {
				ColorSpaceOut = CmsGetColorSpace(hProfile)
			}

		case lIsInput || cls == CmsSigLinkClass:
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

func cmsCreateExtendedTransform(mm mem.Manager,
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

	if dwFlags&CmsFLAGS_NULLTRANSFORM != 0 {
		return AllocEmptyTransform(mm, ContextID, nil, INTENT_PERCEPTUAL, &InputFormat, &OutputFormat, &dwFlags)
	}

	// Gamut check validation
	if dwFlags&CmsFLAGS_GAMUTCHECK != 0 && hGamutProfile == nil {
		dwFlags &^= CmsFLAGS_GAMUTCHECK
	}

	// Disable cache for floating-point formats
	if cmsFormatterIsFloat(InputFormat) || cmsFormatterIsFloat(OutputFormat) {
		dwFlags |= CmsFLAGS_NOCACHE
	}

	// Retrieve entry and exit color spaces
	var EntryColorSpace, ExitColorSpace cmsColorSpaceSignature
	if !GetXFormColorSpaces(nProfiles, hProfiles, &EntryColorSpace, &ExitColorSpace) {
		cmsSignalError(ContextID, cmsERROR_NULL, "NULL input profiles on transform")
		return nil
	}

	// Validate color spaces
	if !IsProperColorSpace(EntryColorSpace, InputFormat) {
		cmsSignalError(ContextID, cmsERROR_COLORSPACE_CHECK, "Wrong input color space on transform")
		return nil
	}
	if !IsProperColorSpace(ExitColorSpace, OutputFormat) {
		cmsSignalError(ContextID, cmsERROR_COLORSPACE_CHECK, "Wrong output color space on transform")
		return nil
	}
	// Check whatever the transform is 16 bits and involves linear RGB in first profile. If so, disable optimizations
	if EntryColorSpace == CmsSigRgbData && T_BYTES(InputFormat) == 2 && (dwFlags&CmsFLAGS_NOOPTIMIZE) == 0 {
		gamma := cmsDetectRGBProfileGamma(mm, hProfiles[0], 0.1)

		if gamma > 0 && gamma < 1.6 {
			dwFlags |= CmsFLAGS_NOOPTIMIZE
		}
	}

	// Build transformation pipeline
	Lut := cmsLinkProfiles(mm, ContextID, nProfiles, Intents, hProfiles, BPC, AdaptationStates, dwFlags)
	if Lut == nil {
		cmsSignalError(ContextID, cmsERROR_NOT_SUITABLE, "Couldn't link the profiles")
		return nil
	}
	/*if _, ok := Lut.Data.(*cmsInterpParams); ok {
		fmt.Println("11ok := Lut.Data.(*cmsInterpParams)")
	} else {
		fmt.Println("11not ok := Lut.Data.(*cmsInterpParams)")

	}*/
	// Validate channel counts
	// Check channel count
	if (cmsChannelsOfColorSpace(EntryColorSpace) != int32(cmsPipelineInputChannels(Lut))) ||
		(cmsChannelsOfColorSpace(ExitColorSpace) != int32(cmsPipelineOutputChannels(Lut))) {
		cmsPipelineFree(mm, Lut)
		cmsSignalError(ContextID, cmsERROR_NOT_SUITABLE, "Channel count doesn't match. Profile is corrupted")
		return nil
	}

	// Allocate transform
	xform := AllocEmptyTransform(mm, ContextID, Lut, Intents[nProfiles-1], &InputFormat, &OutputFormat, &dwFlags)
	if xform == nil {
		return nil
	}

	// Configure transform
	xform.EntryColorSpace = EntryColorSpace
	xform.ExitColorSpace = ExitColorSpace
	xform.RenderingIntent = Intents[nProfiles-1]
	// Take white points
	SetWhitePoint(&xform.EntryWhitePoint, (cmsReadTag(mm, hProfiles[0], CmsSigMediaWhitePointTag).(*cmsCIEXYZ)))
	SetWhitePoint(&xform.ExitWhitePoint, (cmsReadTag(mm, hProfiles[nProfiles-1], CmsSigMediaWhitePointTag).(*cmsCIEXYZ)))

	// Add optional gamut check
	if hGamutProfile != nil && (dwFlags&CmsFLAGS_GAMUTCHECK != 0) {
		xform.GamutCheck = cmsCreateGamutCheckPipeline(mm, ContextID, hProfiles, BPC, Intents, AdaptationStates, nGamutPCSposition, hGamutProfile)
	}
	// Try to read input and output colorant table
	if cmsIsTag(hProfiles[0], CmsSigColorantTableTag) {

		// Input table can only come in this way.
		xform.InputColorant = cmsDupNamedColorList(mm, (cmsReadTag(mm, hProfiles[0], CmsSigColorantTableTag).(*cmsNAMEDCOLORLIST)))
	}

	// Output is a little bit more complex.
	if cmsGetDeviceClass(hProfiles[nProfiles-1]) == CmsSigLinkClass {

		// This tag may exist only on devicelink profiles.
		if cmsIsTag(hProfiles[nProfiles-1], CmsSigColorantTableOutTag) {

			// It may be NULL if error
			xform.OutputColorant = cmsDupNamedColorList(mm, (cmsReadTag(mm, hProfiles[nProfiles-1], CmsSigColorantTableOutTag).(*cmsNAMEDCOLORLIST)))
		}

	} else {

		if cmsIsTag(hProfiles[nProfiles-1], CmsSigColorantTableTag) {

			xform.OutputColorant = cmsDupNamedColorList(mm, (cmsReadTag(mm, hProfiles[nProfiles-1], CmsSigColorantTableTag)).(*cmsNAMEDCOLORLIST))
		}
	}

	// Store the sequence of profiles
	if dwFlags&CmsFLAGS_KEEP_SEQUENCE != 0 {
		xform.Sequence = cmsCompileProfileSequence(mm, ContextID, nProfiles, hProfiles)
	} else {
		xform.Sequence = nil
	}
	// If this is a cached transform, init first value, which is zero (16 bits only)
	if dwFlags&CmsFLAGS_NOCACHE == 0 {
		//fmt.Println("cached transform")
		if xform.GamutCheck != nil {
			TransformOnePixelWithGamutCheck(mm, xform, xform.Cache.CacheIn[:], xform.Cache.CacheOut[:])
		} else {
			eval16(mm, xform.Lut, xform.Cache.CacheIn[:], xform.Cache.CacheOut[:])
		}

	}
	//fmt.Println("end cmsCreateExtendedTransform before returning form")
	return xform
}

// cmsCreateMultiprofileTransformTHR creates a multiprofile transform with a specified context.
func cmsCreateMultiprofileTransformTHR(mm mem.Manager,
	ContextID CmsContext,
	hProfiles []CmsHPROFILE,
	nProfiles uint32,
	InputFormat uint32,
	OutputFormat uint32,
	Intent uint32,
	dwFlags uint32,
) CmsHTRANSFORM {
	//	fmt.Println("start cmsCreateMultiprofileTransformTHR")
	var BPC [256]bool
	var Intents [256]uint32
	var AdaptationStates [256]float64

	// Check the number of profiles
	if nProfiles <= 0 || nProfiles > 255 {
		cmsSignalError(ContextID, cmsERROR_RANGE, "Wrong number of profiles. 1..255 expected")
		return nil
	}

	// Initialize BPC, Intents, and AdaptationStates
	for i := uint32(0); i < nProfiles; i++ {
		if dwFlags&CmsFLAGS_BLACKPOINTCOMPENSATION != 0 {
			BPC[i] = true
		} else {
			BPC[i] = false
		}
		Intents[i] = Intent
		AdaptationStates[i] = cmsSetAdaptationStateTHR(ContextID, -1)
	}

	// Create the extended transform
	//	fmt.Println("end cmsCreateMultiprofileTransformTHR")
	return CmsHTRANSFORM(cmsCreateExtendedTransform(mm, ContextID, nProfiles, hProfiles, BPC[:], Intents[:], AdaptationStates[:], nil, 0, InputFormat, OutputFormat, dwFlags))
}

// cmsCreateMultiprofileTransform creates a multiprofile transform with a default context.
func cmsCreateMultiprofileTransform(mm mem.Manager,
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
		mm,
		cmsGetProfileContextID(hProfiles[0]),
		hProfiles,
		nProfiles,
		InputFormat,
		OutputFormat,
		Intent,
		dwFlags,
	)
}

func cmsCreateTransformTHR(mm mem.Manager,
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

	return cmsCreateMultiprofileTransformTHR(mm, ContextID, hProfiles, nProfiles, InputFormat, OutputFormat, Intent, dwFlags)
}

func CmsCreateTransform(mm mem.Manager,
	Input CmsHPROFILE,
	InputFormat uint32,
	Output CmsHPROFILE,
	OutputFormat uint32,
	Intent uint32,
	dwFlags uint32,
) CmsHTRANSFORM {
	if mm.IsZero() {
		panic("CmsCreateTransform: zero mem.Manager (call mem.NewManager() or mem.NewArena())")
	}

	return cmsCreateTransformTHR(mm, cmsGetProfileContextID(Input), Input, InputFormat, Output, OutputFormat, Intent, dwFlags)
	//fmt.Println("end CmsCreateTransform")

}

//lint:ignore U1000 kept for parity with lcms; used in future ports
func cmsCreateProofingTransformTHR(mm mem.Manager,
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
		dwFlags&CmsFLAGS_BLACKPOINTCOMPENSATION != 0,
		dwFlags&CmsFLAGS_BLACKPOINTCOMPENSATION != 0,
		false,
		false,
	}
	Adaptation := []float64{
		cmsSetAdaptationStateTHR(ContextID, -1),
		cmsSetAdaptationStateTHR(ContextID, -1),
		cmsSetAdaptationStateTHR(ContextID, -1),
		cmsSetAdaptationStateTHR(ContextID, -1),
	}

	if dwFlags&(CmsFLAGS_SOFTPROOFING|CmsFLAGS_GAMUTCHECK) == 0 {
		return cmsCreateTransformTHR(mm, ContextID, InputProfile, InputFormat, OutputProfile, OutputFormat, nIntent, dwFlags)
	}

	return CmsHTRANSFORM(cmsCreateExtendedTransform(mm, ContextID, 4, hArray, BPC, Intents, Adaptation, ProofingProfile, 1, InputFormat, OutputFormat, dwFlags))
}

func cmsCreateProofingTransform(mm mem.Manager,
	InputProfile CmsHPROFILE,
	InputFormat uint32,
	OutputProfile CmsHPROFILE,
	OutputFormat uint32,
	ProofingProfile CmsHPROFILE,
	nIntent uint32,
	ProofingIntent uint32,
	dwFlags uint32,
) CmsHTRANSFORM {
	return cmsCreateProofingTransformTHR(mm,
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
	xform := hTransform.(*cmsTRANSFORM)
	if xform == nil {
		return nil
	}
	return xform.ContextID
}

func cmsGetTransformInputFormat(hTransform CmsHTRANSFORM) uint32 {
	xform := hTransform.(*cmsTRANSFORM)
	if xform == nil {
		return 0
	}
	return xform.InputFormat
}

func cmsGetTransformOutputFormat(hTransform CmsHTRANSFORM) uint32 {
	xform := hTransform.(*cmsTRANSFORM)
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
	xform := hTransform.(*cmsTRANSFORM)

	// Ensure the transform supports format change
	if xform.DwOriginalFlags&cmsFLAGS_CAN_CHANGE_FORMATTER == 0 {
		cmsSignalError(xform.ContextID, cmsERROR_NOT_SUITABLE, "cmsChangeBuffersFormat works only on transforms created originally with at least 16 bits of precision")
		return false
	}

	FromInput := cmsGetFormatter(xform.ContextID, InputFormat, cmsFormatterInput, CMS_PACK_FLAGS_16BITS).Fmt16
	ToOutput := cmsGetFormatter(xform.ContextID, OutputFormat, cmsFormatterOutput, CMS_PACK_FLAGS_16BITS).Fmt16

	if FromInput == nil || ToOutput == nil {
		cmsSignalError(xform.ContextID, cmsERROR_UNKNOWN_EXTENSION, "Unsupported raster format")
		return false
	}

	xform.InputFormat = InputFormat
	xform.OutputFormat = OutputFormat
	xform.FromInput = FromInput
	xform.ToOutput = ToOutput
	return true
}
