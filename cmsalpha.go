package golcms

import (
	"errors"
	"math"
	"unsafe"
)

// Utility to change endianness of a 16-bit number
func changeEndian(w uint16) uint16 {
	return (w<<8 | w>>8)
}

// Saturates a float64 to a byte value
func cmsQuickSaturateByte(d float64) uint8 {
	d += 0.5
	if d <= 0 {
		return 0
	}
	if d >= 255.0 {
		return 255
	}
	return uint8(math.Floor(d))
}

// Computes the true size in bytes for a format
func trueBytesSize(format uint32) uint32 {
	fmtBytes := T_BYTES(format) // T_BYTES is assumed to extract byte information from `format`
	if fmtBytes == 0 {
		return uint32(unsafe.Sizeof(float64(0)))
	}
	return uint32(fmtBytes)
}

// Formatter function type
type FormatterAlphaFn func(dst, src unsafe.Pointer)

// Formatters from 8-bit
func copy8(dst, src unsafe.Pointer) {
	memmove(dst, src, 1)
}

func from8to16(dst, src unsafe.Pointer) {
	n := *(*uint8)(src)
	*(*uint16)(dst) = uint16(FROM_8_TO_16(n)) // FROM_8_TO_16(n)
}

func from8to16SE(dst, src unsafe.Pointer) {
	n := *(*uint8)(src)
	*(*uint16)(dst) = changeEndian(FROM_8_TO_16(n))
}

func from8toFLT(dst, src unsafe.Pointer) {
	*(*float32)(dst) = float32(*(*uint8)(src) / 255.0)
}

// Converts from 8-bit to double (64-bit float)
func from8toDBL(dst, src unsafe.Pointer) {
	*(*float64)(dst) = float64(*(*uint8)(src) / 255.0)
}

// Converts from 8-bit to half-precision float
func from8toHLF(dst, src unsafe.Pointer) {

	n := float32(*(*uint8)(src) / 255.0)
	*(*uint16)(dst) = cmsFloat2Half(n) // Assumes FloatToHalf is implemented

}

// Converts from 16-bit to 8-bit
func from16to8(dst, src unsafe.Pointer) {
	n := *(*uint16)(src)
	*(*uint8)(dst) = FROM_16_TO_8(n) // Uses previously defined From16To8 function
}

// Converts from 16-bit (big-endian) to 8-bit
func from16SEto8(dst, src unsafe.Pointer) {
	n := *(*uint16)(src)
	*(*uint8)(dst) = FROM_16_TO_8(changeEndian(n)) // Uses changeEndian function
}

// Copies 2 bytes from src to dst
func copy16(dst, src unsafe.Pointer) {
	memmove(dst, src, 2) // Uses the previously defined memmove function
}

// Converts from 16-bit to 16-bit with endian swap
func from16to16(dst, src unsafe.Pointer) {
	n := *(*uint16)(src)
	*(*uint16)(dst) = changeEndian(n)
}

// Converts from 16-bit to 32-bit float
func from16toFLT(dst, src unsafe.Pointer) {
	*(*float32)(dst) = float32(*(*uint16)(src) / 65535.0)
}

func from16SEtoFLT(dst, src unsafe.Pointer) {
	*(*float32)(dst) = float32(changeEndian(*(*uint16)(src)) / 65535.0)
}

func from16toDBL(dst, src unsafe.Pointer) {
	*(*float64)(dst) = float64(*(*uint16)(src) / 65535.0)
}

func from16SEtoDBL(dst, src unsafe.Pointer) {
	*(*float64)(dst) = float64(changeEndian(*(*uint16)(src)) / 65535.0)
}

func from16toHLF(dst, src unsafe.Pointer) {
	n := float32((*(*uint16)(src) / 65535.0))
	*(*uint16)(dst) = cmsFloat2Half(n)
}

func from16SEtoHLF(dst, src unsafe.Pointer) {
	n := float32(changeEndian(*(*uint16)(src) / 65535.0))
	*(*uint16)(dst) = cmsFloat2Half(n)

}

// From Float

func fromFLTto8(dst, src unsafe.Pointer) {
	n := *(*float64)(src)
	*(*uint8)(dst) = cmsQuickSaturateByte(n * 255.0)
}

func fromFLTto16(dst, src unsafe.Pointer) {
	n := *(*float64)(src)
	*(*uint16)(dst) = cmsQuickSaturateWord(n * 65535.0)
}

func fromFLTto16SE(dst, src unsafe.Pointer) {
	n := *(*float64)(src)
	i := cmsQuickSaturateWord(n * 65535.0)

	*(*uint16)(dst) = changeEndian(i)
}

func copy32(dst, src unsafe.Pointer) {
	memmove(dst, src, unsafe.Sizeof(float32(0)))
}

func fromFLTtoDBL(dst, src unsafe.Pointer) {
	n := *(*float32)(src)
	*(*float64)(dst) = float64(n)
}

func fromFLTtoHLF(dst, src unsafe.Pointer) {
	n := *(*float32)(src)
	*(*uint16)(dst) = cmsFloat2Half(n)
}

// From HALF

func fromHLFto8(dst, src unsafe.Pointer) {
	n := cmsHalf2Float(*(*uint16)(src))
	*(*uint8)(dst) = cmsQuickSaturateByte(float64(n) * 255.0)
}

func fromHLFto16(dst, src unsafe.Pointer) {
	n := cmsHalf2Float(*(*uint16)(src))
	*(*uint16)(dst) = cmsQuickSaturateWord(float64(n) * 65535.0)
}

func fromHLFto16SE(dst, src unsafe.Pointer) {
	n := cmsHalf2Float(*(*uint16)(src))
	i := cmsQuickSaturateWord(float64(n) * 65535.0)
	*(*uint16)(dst) = changeEndian(i)
}

func fromHLFtoFLT(dst, src unsafe.Pointer) {
	*(*float32)(dst) = cmsHalf2Float(*(*uint16)(src))

}

func fromHLFtoDBL(dst, src unsafe.Pointer) {
	*(*float64)(dst) = float64(cmsHalf2Float(*(*uint16)(src)))

}

// From double
func fromDBLto8(dst, src unsafe.Pointer) {
	n := *(*float64)(src)
	*(*uint8)(dst) = cmsQuickSaturateByte(n * 255.0)
}

func fromDBLto16(dst, src unsafe.Pointer) {
	n := *(*float64)(src)
	*(*uint16)(dst) = cmsQuickSaturateWord(n * 65535.0)
}

func fromDBLto16SE(dst, src unsafe.Pointer) {
	n := *(*float64)(src)
	i := cmsQuickSaturateWord(n * 65535.0)
	*(*uint16)(dst) = changeEndian(i)
}

func fromDBLtoFLT(dst, src unsafe.Pointer) {
	n := *(*float64)(src)
	*(*float32)(dst) = float32(n)
}

func fromDBLtoHLF(dst, src unsafe.Pointer) {
	n := float32(*(*float64)(src))
	*(*uint16)(dst) = cmsFloat2Half(n)
}

func copy64(dst, src unsafe.Pointer) {
	memmove(dst, src, unsafe.Sizeof(float64(0)))
}

// Returns the position (x or y) of the formatter in the table of functions
func FormatterPos(frm uint32) int32 {
	b := T_BYTES(frm)

	if b == 0 && T_FLOAT(frm) != 0 {
		return 5 // DBL
	}
	if b == 2 && T_FLOAT(frm) != 0 {
		return 3 // HLF
	}
	if b == 4 && T_FLOAT(frm) != 0 {
		return 4 // FLT
	}
	if b == 2 && T_FLOAT(frm) == 0 {
		if T_ENDIAN16(frm) != 0 {
			return 2 // 16SE
		} else {
			return 1 // 16
		}
	}
	if b == 1 && T_FLOAT(frm) == 0 {
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
func cmsGetFormatterAlpha(id unsafe.Pointer, in, out uint32) (cmsFormatterAlphaFn, error) {
	inN := FormatterPos(in)
	outN := FormatterPos(out)

	if inN < 0 || outN < 0 || inN > 5 || outN > 5 {
		cmsSignalError(id, 1, "Unrecognized alpha channel width")
		return nil, errors.New("unrecognized alpha channel width")
	}

	return FormatterAlpha[inN][outN], nil
}

// Compute increments for chunky formats
func ComputeIncrementsForChunky(format uint32, componentStartingOrder, componentPointerIncrements []uint32) {
	var channels [cmsMAXCHANNELS]uint32
	extra := T_EXTRA(format)
	nchannels := T_CHANNELS(format)
	totalChans := nchannels + extra
	channelSize := trueBytesSize(format)
	pixelSize := channelSize * totalChans

	// Sanity check
	if totalChans <= 0 || totalChans >= cmsMAXCHANNELS {
		return
	}

	// Initialize increments
	for i := uint32(0); i < extra; i++ {
		componentPointerIncrements[i] = pixelSize
	}

	// Handle swap logic
	for i := uint32(0); i < totalChans; i++ {
		if T_DOSWAP(format) != 0 {
			channels[i] = totalChans - i - 1
		} else {
			channels[i] = i
		}
	}

	// Handle swap first
	if T_SWAPFIRST(format) != 0 && totalChans > 1 {
		tmp := channels[0]
		for i := uint32(0); i < totalChans-1; i++ {
			channels[i] = channels[i+1]
		}
		channels[totalChans-1] = tmp
	}

	// Apply channel size
	for i := uint32(0); i < totalChans; i++ {
		channels[i] *= channelSize
	}

	// Set component starting order
	for i := uint32(0); i < uint32(extra); i++ {
		componentStartingOrder[i] = channels[i+nchannels]
	}
}

// Compute increments for planar formats
func ComputeIncrementsForPlanar(format uint32, bytesPerPlane uint32, componentStartingOrder, componentPointerIncrements []uint32) {
	var channels [cmsMAXCHANNELS]uint32
	extra := T_EXTRA(format)
	nchannels := T_CHANNELS(format)
	totalChans := nchannels + extra
	channelSize := trueBytesSize(format)

	// Sanity check
	if totalChans <= 0 || totalChans >= cmsMAXCHANNELS {
		return
	}

	// Initialize increments
	for i := uint32(0); i < uint32(extra); i++ {
		componentPointerIncrements[i] = channelSize
	}

	// Handle swap logic
	for i := uint32(0); i < uint32(totalChans); i++ {
		if T_DOSWAP(format) != 0 {
			channels[i] = totalChans - i - 1
		} else {
			channels[i] = i
		}
	}

	// Handle swap first
	if T_SWAPFIRST(format) != 0 && totalChans > 0 {
		tmp := channels[0]
		for i := uint32(0); i < uint32(totalChans)-1; i++ {
			channels[i] = channels[i+1]
		}
		channels[totalChans-1] = tmp
	}

	// Apply channel size
	for i := uint32(0); i < uint32(totalChans); i++ {
		channels[i] *= bytesPerPlane
	}

	// Set component starting order
	for i := uint32(0); i < uint32(extra); i++ {
		componentStartingOrder[i] = channels[i+nchannels]
	}
}

// Dispatcher for chunky and planar formats
func ComputeComponentIncrements(format, bytesPerPlane uint32, componentStartingOrder, componentPointerIncrements []uint32) {
	if T_PLANAR(format) != 0 {
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
	PixelsPerLine uint32,
	LineCount uint32,
	Stride *cmsStride,
) {
	var (
		SourceStartingOrder [cmsMAXCHANNELS]uint32
		SourceIncrements    [cmsMAXCHANNELS]uint32
		DestStartingOrder   [cmsMAXCHANNELS]uint32
		DestIncrements      [cmsMAXCHANNELS]uint32
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
	nExtra := T_EXTRA(p.InputFormat)
	if nExtra != T_EXTRA(p.OutputFormat) {
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
		var SourceStrideIncrement, DestStrideIncrement uint32

		for i := uint32(0); i < LineCount; i++ {
			// Prepare pointers
			SourcePtr := uintptr(in) + uintptr(SourceStartingOrder[0]+SourceStrideIncrement)
			DestPtr := uintptr(out) + uintptr(DestStartingOrder[0]+DestStrideIncrement)

			for j := uint32(0); j < PixelsPerLine; j++ {
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
			SourceStrideIncrements [cmsMAXCHANNELS]uint32
			DestStrideIncrements   [cmsMAXCHANNELS]uint32
		)

		for i := uint32(0); i < LineCount; i++ {
			// Prepare pointers
			for j := uint32(0); j < uint32(nExtra); j++ {
				SourcePtr[j] = uintptr(in) + uintptr(SourceStartingOrder[j]+SourceStrideIncrements[j])
				DestPtr[j] = uintptr(out) + uintptr(DestStartingOrder[j]+DestStrideIncrements[j])
			}

			for j := uint32(0); j < PixelsPerLine; j++ {
				for k := uint32(0); k < uint32(nExtra); k++ {
					copyValueFn(unsafe.Pointer(DestPtr[k]), unsafe.Pointer(SourcePtr[k]))

					SourcePtr[k] += uintptr(SourceIncrements[k])
					DestPtr[k] += uintptr(DestIncrements[k])
				}
			}

			for j := uint32(0); j < uint32(nExtra); j++ {
				SourceStrideIncrements[j] += Stride.BytesPerLineIn
				DestStrideIncrements[j] += Stride.BytesPerLineOut
			}
		}
	}
}
