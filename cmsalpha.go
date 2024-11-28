package golcms

import (
	"errors"
	"math"
	"unsafe"
)

// Utility to change endianness of a 16-bit number
func changeEndian(w cmsUInt16Number) cmsUInt16Number {
	return (w<<8 | w>>8)
}

// Saturates a float64 to a byte value
func cmsQuickSaturateByte(d cmsFloat64Number) cmsUInt8Number {
	d += 0.5
	if d <= 0 {
		return 0
	}
	if d >= 255.0 {
		return 255
	}
	return cmsUInt8Number(math.Floor(float64(d)))
}

// Computes the true size in bytes for a format
func trueBytesSize(format cmsUInt32Number) cmsUInt32Number {
	fmtBytes := T_BYTES(uint32(format)) // T_BYTES is assumed to extract byte information from `format`
	if fmtBytes == 0 {
		return cmsUInt32Number(unsafe.Sizeof(float64(0)))
	}
	return cmsUInt32Number(fmtBytes)
}

// Formatter function type
type FormatterAlphaFn func(dst, src unsafe.Pointer)

// Formatters from 8-bit
func copy8(dst, src unsafe.Pointer) {
	memmove(dst, src, 1)
}

func from8to16(dst, src unsafe.Pointer) {
	n := *(*cmsUInt8Number)(src)
	*(*cmsUInt16Number)(dst) = cmsUInt16Number(FROM_8_TO_16(n)) // FROM_8_TO_16(n)
}

func from8to16SE(dst, src unsafe.Pointer) {
	n := *(*cmsUInt8Number)(src)
	*(*cmsUInt16Number)(dst) = changeEndian(FROM_8_TO_16(n))
}

func from8toFLT(dst, src unsafe.Pointer) {
	*(*cmsFloat32Number)(dst) = cmsFloat32Number(*(*cmsUInt8Number)(src) / 255.0)
}

// Converts from 8-bit to double (64-bit float)
func from8toDBL(dst, src unsafe.Pointer) {
	*(*cmsFloat64Number)(dst) = cmsFloat64Number(*(*cmsUInt8Number)(src) / 255.0)
}

// Converts from 8-bit to half-precision float
func from8toHLF(dst, src unsafe.Pointer) {

	n := cmsFloat32Number(*(*cmsUInt8Number)(src) / 255.0)
	*(*cmsUInt16Number)(dst) = cmsFloat2Half(n) // Assumes FloatToHalf is implemented

}

// Converts from 16-bit to 8-bit
func from16to8(dst, src unsafe.Pointer) {
	n := *(*cmsUInt16Number)(src)
	*(*cmsUInt8Number)(dst) = FROM_16_TO_8(n) // Uses previously defined From16To8 function
}

// Converts from 16-bit (big-endian) to 8-bit
func from16SEto8(dst, src unsafe.Pointer) {
	n := *(*cmsUInt16Number)(src)
	*(*cmsUInt8Number)(dst) = FROM_16_TO_8(changeEndian(n)) // Uses changeEndian function
}

// Copies 2 bytes from src to dst
func copy16(dst, src unsafe.Pointer) {
	memmove(dst, src, 2) // Uses the previously defined memmove function
}

// Converts from 16-bit to 16-bit with endian swap
func from16to16(dst, src unsafe.Pointer) {
	n := *(*cmsUInt16Number)(src)
	*(*cmsUInt16Number)(dst) = changeEndian(n)
}

// Converts from 16-bit to 32-bit float
func from16toFLT(dst, src unsafe.Pointer) {
	*(*cmsFloat32Number)(dst) = cmsFloat32Number(*(*cmsUInt16Number)(src) / 65535.0)
}

func from16SEtoFLT(dst, src unsafe.Pointer) {
	*(*cmsFloat32Number)(dst) = cmsFloat32Number(changeEndian(*(*cmsUInt16Number)(src)) / 65535.0)
}

func from16toDBL(dst, src unsafe.Pointer) {
	*(*cmsFloat64Number)(dst) = cmsFloat64Number(*(*cmsUInt16Number)(src) / 65535.0)
}

func from16SEtoDBL(dst, src unsafe.Pointer) {
	*(*cmsFloat64Number)(dst) = cmsFloat64Number(changeEndian(*(*cmsUInt16Number)(src)) / 65535.0)
}

func from16toHLF(dst, src unsafe.Pointer) {
	n := cmsFloat32Number((*(*cmsUInt16Number)(src) / 65535.0))
	*(*cmsUInt16Number)(dst) = cmsFloat2Half(n)
}

func from16SEtoHLF(dst, src unsafe.Pointer) {
	n := cmsFloat32Number(changeEndian(*(*cmsUInt16Number)(src) / 65535.0))
	*(*cmsUInt16Number)(dst) = cmsFloat2Half(n)

}

// From Float

func fromFLTto8(dst, src unsafe.Pointer) {
	n := *(*cmsFloat64Number)(src)
	*(*cmsUInt8Number)(dst) = cmsQuickSaturateByte(n * 255.0)
}

func fromFLTto16(dst, src unsafe.Pointer) {
	n := *(*cmsFloat64Number)(src)
	*(*cmsUInt16Number)(dst) = cmsQuickSaturateWord(n * 65535.0)
}

func fromFLTto16SE(dst, src unsafe.Pointer) {
	n := *(*cmsFloat64Number)(src)
	i := cmsQuickSaturateWord(n * 65535.0)

	*(*cmsUInt16Number)(dst) = changeEndian(i)
}

func copy32(dst, src unsafe.Pointer) {
	memmove(dst, src, unsafe.Sizeof(cmsFloat32Number(0)))
}

func fromFLTtoDBL(dst, src unsafe.Pointer) {
	n := *(*cmsFloat32Number)(src)
	*(*cmsFloat64Number)(dst) = cmsFloat64Number(n)
}

func fromFLTtoHLF(dst, src unsafe.Pointer) {
	n := *(*cmsFloat32Number)(src)
	*(*cmsUInt16Number)(dst) = cmsFloat2Half(n)
}

// From HALF

func fromHLFto8(dst, src unsafe.Pointer) {
	n := cmsHalf2Float(*(*cmsUInt16Number)(src))
	*(*cmsUInt8Number)(dst) = cmsQuickSaturateByte(cmsFloat64Number(n) * 255.0)
}

func fromHLFto16(dst, src unsafe.Pointer) {
	n := cmsHalf2Float(*(*cmsUInt16Number)(src))
	*(*cmsUInt16Number)(dst) = cmsQuickSaturateWord(cmsFloat64Number(n) * 65535.0)
}

func fromHLFto16SE(dst, src unsafe.Pointer) {
	n := cmsHalf2Float(*(*cmsUInt16Number)(src))
	i := cmsQuickSaturateWord(cmsFloat64Number(n) * 65535.0)
	*(*cmsUInt16Number)(dst) = changeEndian(i)
}

func fromHLFtoFLT(dst, src unsafe.Pointer) {
	*(*cmsFloat32Number)(dst) = cmsHalf2Float(*(*cmsUInt16Number)(src))

}

func fromHLFtoDBL(dst, src unsafe.Pointer) {
	*(*cmsFloat64Number)(dst) = cmsFloat64Number(cmsHalf2Float(*(*cmsUInt16Number)(src)))

}

// From double
func fromDBLto8(dst, src unsafe.Pointer) {
	n := *(*cmsFloat64Number)(src)
	*(*cmsUInt8Number)(dst) = cmsQuickSaturateByte(n * 255.0)
}

func fromDBLto16(dst, src unsafe.Pointer) {
	n := *(*cmsFloat64Number)(src)
	*(*cmsUInt16Number)(dst) = cmsQuickSaturateWord(n * 65535.0)
}

func fromDBLto16SE(dst, src unsafe.Pointer) {
	n := *(*cmsFloat64Number)(src)
	i := cmsQuickSaturateWord(n * 65535.0)
	*(*cmsUInt16Number)(dst) = changeEndian(i)
}

func fromDBLtoFLT(dst, src unsafe.Pointer) {
	n := *(*cmsFloat64Number)(src)
	*(*cmsFloat32Number)(dst) = cmsFloat32Number(n)
}

func fromDBLtoHLF(dst, src unsafe.Pointer) {
	n := cmsFloat32Number(*(*cmsFloat64Number)(src))
	*(*cmsUInt16Number)(dst) = cmsFloat2Half(n)
}

func copy64(dst, src unsafe.Pointer) {
	memmove(dst, src, unsafe.Sizeof(cmsFloat64Number(0)))
}

// Returns the position (x or y) of the formatter in the table of functions
func FormatterPos(frm cmsUInt32Number) int32 {
	b := cmsUInt32Number(T_BYTES(uint32(frm)))

	if b == 0 && T_FLOAT(uint32(frm)) != 0 {
		return 5 // DBL
	}
	if b == 2 && T_FLOAT(uint32(frm)) != 0 {
		return 3 // HLF
	}
	if b == 4 && T_FLOAT(uint32(frm)) != 0 {
		return 4 // FLT
	}
	if b == 2 && T_FLOAT(uint32(frm)) == 0 {
		if T_ENDIAN16(uint32(frm)) != 0 {
			return 2 // 16SE
		} else {
			return 1 // 16
		}
	}
	if b == 1 && T_FLOAT(uint32(frm)) == 0 {
		return 0 // 8
	}
	return -1 // not recognized
}

// Define function types for the formatters
type cmsFormatterAlphaFn func(dst, src unsafe.Pointer)

// FormatterAlpha is a static array of functions
var FormatterAlpha = [6][6]cmsFormatterAlphaFn{
	/* from 8 */ {copy8, from8to16, from8to16SE, from8toHLF, from8toFLT, from8toDBL},
	/* from 16 */ {from16to8, copy16, from16to16, from16toHLF, from16toFLT, from16toDBL},
	/* from 16SE */ {from16SEto8, from16to16, copy16, from16SEtoHLF, from16SEtoFLT, from16SEtoDBL},
	/* from HLF */ {fromHLFto8, fromHLFto16, fromHLFto16SE, copy16, fromHLFtoFLT, fromHLFtoDBL},
	/* from FLT */ {fromFLTto8, fromFLTto16, fromFLTto16SE, fromFLTtoHLF, copy32, fromFLTtoDBL},
	/* from DBL */ {fromDBLto8, fromDBLto16, fromDBLto16SE, fromDBLtoHLF, fromDBLtoFLT, copy64},
}

// cmsGetFormatterAlpha implements the logic
func cmsGetFormatterAlpha(id interface{}, in, out cmsUInt32Number) (cmsFormatterAlphaFn, error) {
	inN := FormatterPos(in)
	outN := FormatterPos(out)

	if inN < 0 || outN < 0 || inN > 5 || outN > 5 {
		cmsSignalError(id, 1, "Unrecognized alpha channel width")
		return nil, errors.New("unrecognized alpha channel width")
	}

	return FormatterAlpha[inN][outN], nil
}

// Compute increments for chunky formats
func ComputeIncrementsForChunky(format cmsUInt32Number, componentStartingOrder, componentPointerIncrements []cmsUInt32Number) {
	var channels [cmsMAXCHANNELS]cmsUInt32Number
	extra := T_EXTRA(uint32(format))
	nchannels := T_CHANNELS(uint32(format))
	totalChans := nchannels + extra
	channelSize := trueBytesSize(format)
	pixelSize := uint32(channelSize) * totalChans

	// Sanity check
	if totalChans <= 0 || totalChans >= cmsMAXCHANNELS {
		return
	}

	// Initialize increments
	for i := uint32(0); i < extra; i++ {
		componentPointerIncrements[i] = cmsUInt32Number(pixelSize)
	}

	// Handle swap logic
	for i := cmsUInt32Number(0); i < cmsUInt32Number(totalChans); i++ {
		if T_DOSWAP(uint32(format)) != 0 {
			channels[i] = cmsUInt32Number(totalChans) - i - 1
		} else {
			channels[i] = i
		}
	}

	// Handle swap first
	if T_SWAPFIRST(uint32(format)) != 0 && totalChans > 1 {
		tmp := channels[0]
		for i := cmsUInt32Number(0); i < cmsUInt32Number(totalChans)-1; i++ {
			channels[i] = channels[i+1]
		}
		channels[totalChans-1] = tmp
	}

	// Apply channel size
	for i := cmsUInt32Number(0); i < cmsUInt32Number(totalChans); i++ {
		channels[i] *= channelSize
	}

	// Set component starting order
	for i := cmsUInt32Number(0); i < cmsUInt32Number(extra); i++ {
		componentStartingOrder[i] = channels[i+cmsUInt32Number(nchannels)]
	}
}

// Compute increments for planar formats
func ComputeIncrementsForPlanar(format cmsUInt32Number, bytesPerPlane cmsUInt32Number, componentStartingOrder, componentPointerIncrements []cmsUInt32Number) {
	var channels [cmsMAXCHANNELS]cmsUInt32Number
	extra := T_EXTRA(uint32(format))
	nchannels := T_CHANNELS(uint32(format))
	totalChans := nchannels + extra
	channelSize := trueBytesSize(format)

	// Sanity check
	if totalChans <= 0 || totalChans >= cmsMAXCHANNELS {
		return
	}

	// Initialize increments
	for i := cmsUInt32Number(0); i < cmsUInt32Number(extra); i++ {
		componentPointerIncrements[i] = channelSize
	}

	// Handle swap logic
	for i := cmsUInt32Number(0); i < cmsUInt32Number(totalChans); i++ {
		if T_DOSWAP(uint32(format)) != 0 {
			channels[i] = cmsUInt32Number(totalChans) - i - 1
		} else {
			channels[i] = i
		}
	}

	// Handle swap first
	if T_SWAPFIRST(uint32(format)) != 0 && totalChans > 0 {
		tmp := channels[0]
		for i := cmsUInt32Number(0); i < cmsUInt32Number(totalChans)-1; i++ {
			channels[i] = channels[i+1]
		}
		channels[totalChans-1] = tmp
	}

	// Apply channel size
	for i := cmsUInt32Number(0); i < cmsUInt32Number(totalChans); i++ {
		channels[i] *= bytesPerPlane
	}

	// Set component starting order
	for i := cmsUInt32Number(0); i < cmsUInt32Number(extra); i++ {
		componentStartingOrder[i] = channels[i+cmsUInt32Number(nchannels)]
	}
}

// Dispatcher for chunky and planar formats
func ComputeComponentIncrements(format, bytesPerPlane cmsUInt32Number, componentStartingOrder, componentPointerIncrements []cmsUInt32Number) {
	if T_PLANAR(uint32(format)) != 0 {
		ComputeIncrementsForPlanar(format, bytesPerPlane, componentStartingOrder, componentPointerIncrements)
	} else {
		ComputeIncrementsForChunky(format, componentStartingOrder, componentPointerIncrements)
	}
}

// Function to handle extra channels copying alpha
func cmsHandleExtraChannels(
	p *cmsTRANSFORM,
	in unsafe.Pointer,
	out unsafe.Pointer,
	PixelsPerLine cmsUInt32Number,
	LineCount cmsUInt32Number,
	Stride *cmsStride,
) {
	var (
		SourceStartingOrder [cmsMAXCHANNELS]cmsUInt32Number
		SourceIncrements    [cmsMAXCHANNELS]cmsUInt32Number
		DestStartingOrder   [cmsMAXCHANNELS]cmsUInt32Number
		DestIncrements      [cmsMAXCHANNELS]cmsUInt32Number
	)

	// Check if alpha copying is needed
	if p.DwOriginalFlags&cmsFLAGS_COPY_ALPHA == 0 {
		return
	}

	// Exit early for in-place color management
	if p.InputFormat == p.OutputFormat && in == out {
		return
	}

	// Ensure the same number of alpha channels
	nExtra := T_EXTRA(uint32(p.InputFormat))
	if nExtra != T_EXTRA(uint32(p.OutputFormat)) {
		return
	}

	// Nothing to do if no extra channels
	if nExtra == 0 {
		return
	}

	// Compute the increments
	ComputeComponentIncrements(p.InputFormat, Stride.BytesPerPlaneIn, SourceStartingOrder[:], SourceIncrements[:])
	ComputeComponentIncrements(p.OutputFormat, Stride.BytesPerPlaneOut, DestStartingOrder[:], DestIncrements[:])

	// Get formatter function
	copyValueFn, _ := cmsGetFormatterAlpha(p.ContextID, p.InputFormat, p.OutputFormat)
	if copyValueFn == nil {
		return
	}

	if nExtra == 1 { // Optimized routine for single extra channel
		var SourceStrideIncrement, DestStrideIncrement cmsUInt32Number

		for i := cmsUInt32Number(0); i < LineCount; i++ {
			// Prepare pointers
			SourcePtr := uintptr(in) + uintptr(SourceStartingOrder[0]+SourceStrideIncrement)
			DestPtr := uintptr(out) + uintptr(DestStartingOrder[0]+DestStrideIncrement)

			for j := cmsUInt32Number(0); j < PixelsPerLine; j++ {
				copyValueFn(unsafe.Pointer(DestPtr), unsafe.Pointer(SourcePtr))

				SourcePtr += uintptr(SourceIncrements[0])
				DestPtr += uintptr(DestIncrements[0])
			}

			SourceStrideIncrement += Stride.BytesPerLineIn
			DestStrideIncrement += Stride.BytesPerLineOut
		}
	} else { // General case for multiple extra channels
		var (
			SourcePtr              [cmsMAXCHANNELS]uintptr
			DestPtr                [cmsMAXCHANNELS]uintptr
			SourceStrideIncrements [cmsMAXCHANNELS]cmsUInt32Number
			DestStrideIncrements   [cmsMAXCHANNELS]cmsUInt32Number
		)

		for i := cmsUInt32Number(0); i < LineCount; i++ {
			// Prepare pointers
			for j := cmsUInt32Number(0); j < cmsUInt32Number(nExtra); j++ {
				SourcePtr[j] = uintptr(in) + uintptr(SourceStartingOrder[j]+SourceStrideIncrements[j])
				DestPtr[j] = uintptr(out) + uintptr(DestStartingOrder[j]+DestStrideIncrements[j])
			}

			for j := cmsUInt32Number(0); j < PixelsPerLine; j++ {
				for k := cmsUInt32Number(0); k < cmsUInt32Number(nExtra); k++ {
					copyValueFn(unsafe.Pointer(DestPtr[k]), unsafe.Pointer(SourcePtr[k]))

					SourcePtr[k] += uintptr(SourceIncrements[k])
					DestPtr[k] += uintptr(DestIncrements[k])
				}
			}

			for j := cmsUInt32Number(0); j < cmsUInt32Number(nExtra); j++ {
				SourceStrideIncrements[j] += Stride.BytesPerLineIn
				DestStrideIncrements[j] += Stride.BytesPerLineOut
			}
		}
	}
}
