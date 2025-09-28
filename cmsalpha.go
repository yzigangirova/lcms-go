package golcms

import (
	//"errors"
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
type FormatterAlphaFn func(dst, src any)

// Formatters from 8-bit
func copy8(dst, src any) {
	// Type assertion for input and output
	inBytes, okIn := src.([]byte)
	outBytes, okOut := dst.([]byte)
	if !okIn || !okOut {
		panic("in and out must be of type []byte")
	}
	outBytes[0] = inBytes[0]

}
func from8to16(dst, src any) {
	// Type assertion to ensure src is []uint8 and dst is []uint16
	srcSlice, okSrc := src.([]uint8)
	dstSlice, okDst := dst.([]uint16)

	if !okSrc || !okDst {
		panic("from8to16: expected src to be []uint8 and dst to be []uint16")
	}

	// Ensure the slices have at least one element
	if len(srcSlice) == 0 || len(dstSlice) == 0 {
		panic("from8to16: source or destination slice is empty")
	}

	// Perform the conversion
	n := srcSlice[0]                      // Read first byte
	dstSlice[0] = uint16(FROM_8_TO_16(n)) // Convert and store in first uint16
}

// Converts from 8-bit to 16-bit with endian swap
func from8to16SE(dst, src any) {
	srcBytes, okSrc := src.([]uint8)
	dstBytes, okDst := dst.([]uint16)

	if !okSrc || !okDst {
		panic("from8to16SE: src must be []uint8 and dst must be []uint16")
	}

	if len(srcBytes) == 0 || len(dstBytes) == 0 {
		panic("from8to16SE: empty source or destination slice")
	}

	n := srcBytes[0]
	dstBytes[0] = changeEndian(FROM_8_TO_16(n))
}

// Converts from 8-bit to float32
func from8toFLT(dst, src any) {
	srcBytes, okSrc := src.([]uint8)
	dstBytes, okDst := dst.([]float32)

	if !okSrc || !okDst {
		panic("from8toFLT: src must be []uint8 and dst must be []float32")
	}

	if len(srcBytes) == 0 || len(dstBytes) == 0 {
		panic("from8toFLT: empty source or destination slice")
	}

	dstBytes[0] = float32(srcBytes[0]) / 255.0
}

// Converts from 8-bit to double (64-bit float)
func from8toDBL(dst, src any) {
	srcBytes, okSrc := src.([]uint8)
	dstBytes, okDst := dst.([]float64)

	if !okSrc || !okDst {
		panic("from8toDBL: src must be []uint8 and dst must be []float64")
	}

	if len(srcBytes) == 0 || len(dstBytes) == 0 {
		panic("from8toDBL: empty source or destination slice")
	}

	dstBytes[0] = float64(srcBytes[0]) / 255.0
}

// Converts from 8-bit to half-precision float
func from8toHLF(dst, src any) {
	srcBytes, okSrc := src.([]uint8)
	dstBytes, okDst := dst.([]uint16)

	if !okSrc || !okDst {
		panic("from8toHLF: src must be []uint8 and dst must be []uint16")
	}

	if len(srcBytes) == 0 || len(dstBytes) == 0 {
		panic("from8toHLF: empty source or destination slice")
	}

	n := float32(srcBytes[0]) / 255.0
	dstBytes[0] = cmsFloat2Half(n)
}

// Converts from 16-bit to 8-bit
func from16to8(dst, src any) {
	srcBytes, okSrc := src.([]uint16)
	dstBytes, okDst := dst.([]uint8)

	if !okSrc || !okDst {
		panic("from16to8: src must be []uint16 and dst must be []uint8")
	}

	if len(srcBytes) == 0 || len(dstBytes) == 0 {
		panic("from16to8: empty source or destination slice")
	}

	dstBytes[0] = FROM_16_TO_8(srcBytes[0])
}

// Converts from 16-bit (big-endian) to 8-bit
func from16SEto8(dst, src any) {
	srcBytes, okSrc := src.([]uint16)
	dstBytes, okDst := dst.([]uint8)

	if !okSrc || !okDst {
		panic("from16SEto8: src must be []uint16 and dst must be []uint8")
	}

	if len(srcBytes) == 0 || len(dstBytes) == 0 {
		panic("from16SEto8: empty source or destination slice")
	}

	dstBytes[0] = FROM_16_TO_8(changeEndian(srcBytes[0]))
}

// Copies 2 bytes from src to dst
func copy16(dst, src any) {
	srcBytes, okSrc := src.([]byte)
	dstBytes, okDst := dst.([]byte)

	if !okSrc || !okDst {
		panic("copy16: src and dst must be []byte")
	}

	if len(srcBytes) < 2 || len(dstBytes) < 2 {
		panic("copy16: insufficient slice length")
	}

	MemmoveSlice(dstBytes, srcBytes, 2)
}

// Converts from 16-bit to 16-bit with endian swap
func from16to16(dst, src any) {
	srcBytes, okSrc := src.([]uint16)
	dstBytes, okDst := dst.([]uint16)

	if !okSrc || !okDst {
		panic("from16to16: src and dst must be []uint16")
	}

	if len(srcBytes) == 0 || len(dstBytes) == 0 {
		panic("from16to16: empty source or destination slice")
	}

	dstBytes[0] = changeEndian(srcBytes[0])
}

// Converts from 16-bit to 32-bit float
func from16toFLT(dst, src any) {
	srcUint16, okSrc := src.([]uint16)
	dstFloat32, okDst := dst.([]float32)

	if !okSrc || !okDst {
		panic("from16toFLT: src must be []uint16 and dst must be []float32")
	}

	if len(srcUint16) == 0 || len(dstFloat32) == 0 {
		panic("from16toFLT: empty source or destination slice")
	}

	dstFloat32[0] = float32(srcUint16[0]) / 65535.0
}

// Converts from 16-bit big-endian to 32-bit float
func from16SEtoFLT(dst, src any) {
	srcUint16, okSrc := src.([]uint16)
	dstFloat32, okDst := dst.([]float32)

	if !okSrc || !okDst {
		panic("from16SEtoFLT: src must be []uint16 and dst must be []float32")
	}

	if len(srcUint16) == 0 || len(dstFloat32) == 0 {
		panic("from16SEtoFLT: empty source or destination slice")
	}

	dstFloat32[0] = float32(changeEndian(srcUint16[0])) / 65535.0
}

// Converts from 16-bit to 64-bit float (double)
func from16toDBL(dst, src any) {
	srcUint16, okSrc := src.([]uint16)
	dstFloat64, okDst := dst.([]float64)

	if !okSrc || !okDst {
		panic("from16toDBL: src must be []uint16 and dst must be []float64")
	}

	if len(srcUint16) == 0 || len(dstFloat64) == 0 {
		panic("from16toDBL: empty source or destination slice")
	}

	dstFloat64[0] = float64(srcUint16[0]) / 65535.0
}

// Converts from 16-bit big-endian to 64-bit float (double)
func from16SEtoDBL(dst, src any) {
	srcUint16, okSrc := src.([]uint16)
	dstFloat64, okDst := dst.([]float64)

	if !okSrc || !okDst {
		panic("from16SEtoDBL: src must be []uint16 and dst must be []float64")
	}

	if len(srcUint16) == 0 || len(dstFloat64) == 0 {
		panic("from16SEtoDBL: empty source or destination slice")
	}

	dstFloat64[0] = float64(changeEndian(srcUint16[0])) / 65535.0
}

// Converts from 16-bit to 16-bit half-precision float
func from16toHLF(dst, src any) {
	srcUint16, okSrc := src.([]uint16)
	dstUint16, okDst := dst.([]uint16)

	if !okSrc || !okDst {
		panic("from16toHLF: src and dst must be []uint16")
	}

	if len(srcUint16) == 0 || len(dstUint16) == 0 {
		panic("from16toHLF: empty source or destination slice")
	}

	n := float32(srcUint16[0]) / 65535.0
	dstUint16[0] = cmsFloat2Half(n)
}

// Converts from 16-bit big-endian to 16-bit half-precision float
func from16SEtoHLF(dst, src any) {
	srcUint16, okSrc := src.([]uint16)
	dstUint16, okDst := dst.([]uint16)

	if !okSrc || !okDst {
		panic("from16SEtoHLF: src and dst must be []uint16")
	}

	if len(srcUint16) == 0 || len(dstUint16) == 0 {
		panic("from16SEtoHLF: empty source or destination slice")
	}

	n := float32(changeEndian(srcUint16[0])) / 65535.0
	dstUint16[0] = cmsFloat2Half(n)
}

// From Float to 8-bit
func fromFLTto8(dst, src any) {
	srcFloat64, okSrc := src.([]float64)
	dstUint8, okDst := dst.([]uint8)

	if !okSrc || !okDst {
		panic("fromFLTto8: src must be []float64 and dst must be []uint8")
	}

	if len(srcFloat64) == 0 || len(dstUint8) == 0 {
		panic("fromFLTto8: empty source or destination slice")
	}

	dstUint8[0] = cmsQuickSaturateByte(srcFloat64[0] * 255.0)
}

// From Float to 16-bit
func fromFLTto16(dst, src any) {
	srcFloat64, okSrc := src.([]float64)
	dstUint16, okDst := dst.([]uint16)

	if !okSrc || !okDst {
		panic("fromFLTto16: src must be []float64 and dst must be []uint16")
	}

	if len(srcFloat64) == 0 || len(dstUint16) == 0 {
		panic("fromFLTto16: empty source or destination slice")
	}

	dstUint16[0] = cmsQuickSaturateWord(srcFloat64[0] * 65535.0)
}

// From Float to 16-bit with Endian Swap
func fromFLTto16SE(dst, src any) {
	srcFloat64, okSrc := src.([]float64)
	dstUint16, okDst := dst.([]uint16)

	if !okSrc || !okDst {
		panic("fromFLTto16SE: src must be []float64 and dst must be []uint16")
	}

	if len(srcFloat64) == 0 || len(dstUint16) == 0 {
		panic("fromFLTto16SE: empty source or destination slice")
	}

	i := cmsQuickSaturateWord(srcFloat64[0] * 65535.0)
	dstUint16[0] = changeEndian(i)
}

// Copy 32-bit float (equivalent to memmove)
func copy32(dst, src any) {
	srcFloat32, okSrc := src.([]float32)
	dstFloat32, okDst := dst.([]float32)

	if !okSrc || !okDst {
		panic("copy32: src and dst must be []float32")
	}

	if len(srcFloat32) == 0 || len(dstFloat32) == 0 {
		panic("copy32: empty source or destination slice")
	}

	dstFloat32[0] = srcFloat32[0]
}

// From Float to Double
func fromFLTtoDBL(dst, src any) {
	srcFloat32, okSrc := src.([]float32)
	dstFloat64, okDst := dst.([]float64)

	if !okSrc || !okDst {
		panic("fromFLTtoDBL: src must be []float32 and dst must be []float64")
	}

	if len(srcFloat32) == 0 || len(dstFloat64) == 0 {
		panic("fromFLTtoDBL: empty source or destination slice")
	}

	dstFloat64[0] = float64(srcFloat32[0])
}

// From Float to Half Precision Float
func fromFLTtoHLF(dst, src any) {
	srcFloat32, okSrc := src.([]float32)
	dstUint16, okDst := dst.([]uint16)

	if !okSrc || !okDst {
		panic("fromFLTtoHLF: src must be []float32 and dst must be []uint16")
	}

	if len(srcFloat32) == 0 || len(dstUint16) == 0 {
		panic("fromFLTtoHLF: empty source or destination slice")
	}

	dstUint16[0] = cmsFloat2Half(srcFloat32[0])
}

// From Half-Precision Float (uint16) to 8-bit
func fromHLFto8(dst, src any) {
	srcUint16, okSrc := src.([]uint16)
	dstUint8, okDst := dst.([]uint8)

	if !okSrc || !okDst {
		panic("fromHLFto8: src must be []uint16 and dst must be []uint8")
	}
	if len(srcUint16) == 0 || len(dstUint8) == 0 {
		panic("fromHLFto8: empty source or destination slice")
	}

	n := cmsHalf2Float(srcUint16[0])
	dstUint8[0] = cmsQuickSaturateByte(float64(n) * 255.0)
}

// From Half-Precision Float (uint16) to 16-bit
func fromHLFto16(dst, src any) {
	srcUint16, okSrc := src.([]uint16)
	dstUint16, okDst := dst.([]uint16)

	if !okSrc || !okDst {
		panic("fromHLFto16: src must be []uint16 and dst must be []uint16")
	}
	if len(srcUint16) == 0 || len(dstUint16) == 0 {
		panic("fromHLFto16: empty source or destination slice")
	}

	n := cmsHalf2Float(srcUint16[0])
	dstUint16[0] = cmsQuickSaturateWord(float64(n) * 65535.0)
}

// From Half-Precision Float (uint16) to 16-bit with Endian Swap
func fromHLFto16SE(dst, src any) {
	srcUint16, okSrc := src.([]uint16)
	dstUint16, okDst := dst.([]uint16)

	if !okSrc || !okDst {
		panic("fromHLFto16SE: src must be []uint16 and dst must be []uint16")
	}
	if len(srcUint16) == 0 || len(dstUint16) == 0 {
		panic("fromHLFto16SE: empty source or destination slice")
	}

	n := cmsHalf2Float(srcUint16[0])
	i := cmsQuickSaturateWord(float64(n) * 65535.0)
	dstUint16[0] = changeEndian(i)
}

// From Half-Precision Float (uint16) to Float32
func fromHLFtoFLT(dst, src any) {
	srcUint16, okSrc := src.([]uint16)
	dstFloat32, okDst := dst.([]float32)

	if !okSrc || !okDst {
		panic("fromHLFtoFLT: src must be []uint16 and dst must be []float32")
	}
	if len(srcUint16) == 0 || len(dstFloat32) == 0 {
		panic("fromHLFtoFLT: empty source or destination slice")
	}

	dstFloat32[0] = cmsHalf2Float(srcUint16[0])
}

// From Half-Precision Float (uint16) to Float64 (Double)
func fromHLFtoDBL(dst, src any) {
	srcUint16, okSrc := src.([]uint16)
	dstFloat64, okDst := dst.([]float64)

	if !okSrc || !okDst {
		panic("fromHLFtoDBL: src must be []uint16 and dst must be []float64")
	}
	if len(srcUint16) == 0 || len(dstFloat64) == 0 {
		panic("fromHLFtoDBL: empty source or destination slice")
	}

	dstFloat64[0] = float64(cmsHalf2Float(srcUint16[0]))
}

// Converts from double (64-bit float) to 8-bit
func fromDBLto8(dst, src any) {
	srcFloat64, okSrc := src.([]float64)
	dstUint8, okDst := dst.([]uint8)

	if !okSrc || !okDst {
		panic("fromDBLto8: src must be []float64 and dst must be []uint8")
	}

	if len(srcFloat64) == 0 || len(dstUint8) == 0 {
		panic("fromDBLto8: empty source or destination slice")
	}

	n := srcFloat64[0]
	dstUint8[0] = cmsQuickSaturateByte(n * 255.0)
}

// Converts from double (64-bit float) to 16-bit
func fromDBLto16(dst, src any) {
	srcFloat64, okSrc := src.([]float64)
	dstUint16, okDst := dst.([]uint16)

	if !okSrc || !okDst {
		panic("fromDBLto16: src must be []float64 and dst must be []uint16")
	}

	if len(srcFloat64) == 0 || len(dstUint16) == 0 {
		panic("fromDBLto16: empty source or destination slice")
	}

	n := srcFloat64[0]
	dstUint16[0] = cmsQuickSaturateWord(n * 65535.0)
}

// Converts from double (64-bit float) to 16-bit with endian swap
func fromDBLto16SE(dst, src any) {
	srcFloat64, okSrc := src.([]float64)
	dstUint16, okDst := dst.([]uint16)

	if !okSrc || !okDst {
		panic("fromDBLto16SE: src must be []float64 and dst must be []uint16")
	}

	if len(srcFloat64) == 0 || len(dstUint16) == 0 {
		panic("fromDBLto16SE: empty source or destination slice")
	}

	n := srcFloat64[0]
	i := cmsQuickSaturateWord(n * 65535.0)
	dstUint16[0] = changeEndian(i)
}

// Converts from double (64-bit float) to 32-bit float
func fromDBLtoFLT(dst, src any) {
	srcFloat64, okSrc := src.([]float64)
	dstFloat32, okDst := dst.([]float32)

	if !okSrc || !okDst {
		panic("fromDBLtoFLT: src must be []float64 and dst must be []float32")
	}

	if len(srcFloat64) == 0 || len(dstFloat32) == 0 {
		panic("fromDBLtoFLT: empty source or destination slice")
	}

	dstFloat32[0] = float32(srcFloat64[0])
}

// Converts from double (64-bit float) to half-precision float
func fromDBLtoHLF(dst, src any) {
	srcFloat64, okSrc := src.([]float64)
	dstUint16, okDst := dst.([]uint16)

	if !okSrc || !okDst {
		panic("fromDBLtoHLF: src must be []float64 and dst must be []uint16")
	}

	if len(srcFloat64) == 0 || len(dstUint16) == 0 {
		panic("fromDBLtoHLF: empty source or destination slice")
	}

	n := float32(srcFloat64[0])
	dstUint16[0] = cmsFloat2Half(n)
}

// Copies 64-bit double from src to dst
func copy64(dst, src any) {
	srcFloat64, okSrc := src.([]float64)
	dstFloat64, okDst := dst.([]float64)

	if !okSrc || !okDst {
		panic("copy64: src and dst must be []float64")
	}

	if len(srcFloat64) == 0 || len(dstFloat64) == 0 {
		panic("copy64: empty source or destination slice")
	}

	dstFloat64[0] = srcFloat64[0]
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
type cmsFormatterAlphaFn func(dst, src any)

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
func cmsGetFormatterAlpha(id any, in, out uint32) cmsFormatterAlphaFn {
	inN := FormatterPos(in)
	outN := FormatterPos(out)

	if inN < 0 || outN < 0 || inN > 5 || outN > 5 {
		cmsSignalError(id, 1, "Unrecognized alpha channel width")
		return nil
	}

	return FormatterAlpha[inN][outN]
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
	in, out any,
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
	if p.DwOriginalFlags&CmsFLAGS_COPY_ALPHA == 0 {
		return
	}
	// Type assertion for input and output
	inBytes, okIn := in.([]byte)
	outBytes, okOut := out.([]byte)

	if !okIn || !okOut {
		panic("cmsHandleExtraChannels: in and out must be of type []byte")
	}
	// Exit early for in-place color management
	//In C pointers are compared here!  Do I have to do deep comparing?
	if p.InputFormat == p.OutputFormat && unsafe.SliceData(inBytes) == unsafe.SliceData(outBytes) {
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
	copyValueFn := cmsGetFormatterAlpha(p.ContextID, p.InputFormat, p.OutputFormat)
	if copyValueFn == nil {
		return
	}

	if nExtra == 1 { // Optimized routine for single extra channel
		var SourceStrideIncrement, DestStrideIncrement uint32

		for i := uint32(0); i < LineCount; i++ {
			// Prepare pointers
			SourcePtr := inBytes[SourceStartingOrder[0]+SourceStrideIncrement:]
			DestPtr := outBytes[DestStartingOrder[0]+DestStrideIncrement:]

			for j := uint32(0); j < PixelsPerLine; j++ {
				//COPYVALUEFN needs rethinking and rewriting into interfaces!
				copyValueFn(DestPtr, SourcePtr)

				SourcePtr = SourcePtr[SourceIncrements[0]:]
				DestPtr = DestPtr[DestIncrements[0]:]
			}

			SourceStrideIncrement += Stride.BytesPerLineIn
			DestStrideIncrement += Stride.BytesPerLineOut
		}
	} else { // General case for multiple extra channels
		var (
			SourcePtr              [cmsMAXCHANNELS][]byte
			DestPtr                [cmsMAXCHANNELS][]byte
			SourceStrideIncrements [cmsMAXCHANNELS]uint32
			DestStrideIncrements   [cmsMAXCHANNELS]uint32
		)

		for i := uint32(0); i < LineCount; i++ {
			// Prepare pointers
			for j := uint32(0); j < uint32(nExtra); j++ {
				SourcePtr[j] = inBytes[(SourceStartingOrder[j] + SourceStrideIncrements[j]):]
				DestPtr[j] = outBytes[DestStartingOrder[j]+DestStrideIncrements[j]:]
			}

			for j := uint32(0); j < PixelsPerLine; j++ {
				for k := uint32(0); k < uint32(nExtra); k++ {
					copyValueFn(DestPtr[k], SourcePtr[k])

					SourcePtr[k] = SourcePtr[k][SourceIncrements[k]:]
					DestPtr[k] = DestPtr[k][DestIncrements[k]:]
				}
			}

			for j := uint32(0); j < uint32(nExtra); j++ {
				SourceStrideIncrements[j] += Stride.BytesPerLineIn
				DestStrideIncrements[j] += Stride.BytesPerLineOut
			}
		}
	}
}
