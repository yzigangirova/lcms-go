package golcms

import (
	"errors"
	"math"
	"unsafe"
)

//FIRST HALF OF THE FILE SKIPPED YET
// Constants for formatter flags

// Change endianness of a 16-bit word
func CHANGE_ENDIAN(w cmsUInt16Number) cmsUInt16Number {
	return (w << 8) | (w >> 8)
}

// Reverse the flavor for 8-bit values
func REVERSE_FLAVOR_8(x cmsUInt8Number) cmsUInt8Number {
	return cmsUInt8Number(0xFF - x)
}

// Reverse the flavor for 16-bit values
func REVERSE_FLAVOR_16(x cmsUInt16Number) cmsUInt16Number {
	return 0xFFFF - x
}

// Convert LabV2 to LabV4
func FromLabV2ToLabV4(x cmsUInt16Number) cmsUInt16Number {
	a := (int(x)<<8 | int(x)) >> 8 // Multiply by 257 / 256
	if a > math.MaxUint16 {
		return math.MaxUint16
	}
	return cmsUInt16Number(a)
}

// Convert LabV4 to LabV2
func FromLabV4ToLabV2(x cmsUInt16Number) cmsUInt16Number {
	return cmsUInt16Number(((int(x) << 8) + 0x80) / 257) // Multiply by 256 / 257
}

// Formatter for 16-bit formats
type cmsFormatters16 struct {
	Type uint32         // Format type
	Mask uint32         // Mask to apply
	Frm  cmsFormatter16 // Formatter function
}

// Formatter for floating-point formats
type cmsFormattersFloat struct {
	Type uint32            // Format type
	Mask uint32            // Mask to apply
	Frm  cmsFormatterFloat // Formatter function
}

var (
	// Flags for pixel format
	ANYSPACE     = COLORSPACE_SH(31)
	ANYCHANNELS  = CHANNELS_SH(15)
	ANYEXTRA     = EXTRA_SH(7)
	ANYPLANAR    = PLANAR_SH(1)
	ANYENDIAN    = ENDIAN16_SH(1)
	ANYSWAP      = DOSWAP_SH(1)
	ANYSWAPFIRST = SWAPFIRST_SH(1)
	ANYFLAVOR    = FLAVOR_SH(1)
	ANYPREMUL    = PREMUL_SH(1)
)
// Unpacking routines (16 bits) ----------------------------------------------------------------------------------------
func UnrollChunkyBytes(info *cmsTRANSFORM, wIn []cmsUInt16Number, accum []cmsUInt8Number, stride cmsUInt32Number) []cmsUInt8Number {
    nChan := int(T_CHANNELS(uint32(uint32(info.InputFormat))))
    doSwap := T_DOSWAP(uint32(uint32(info.InputFormat)))
    reverse := T_FLAVOR(uint32(uint32(info.InputFormat)))
    swapFirst := T_SWAPFIRST(uint32(uint32(info.InputFormat)))
    extra := T_EXTRA(uint32(uint32(info.InputFormat)))
    premul := T_PREMUL(uint32(uint32(info.InputFormat)))

    extraFirst := doSwap ^ swapFirst
    alphaFactor := cmsUInt32Number(1)

    if extraFirst != 0 {
        if premul != 0 && extra != 0 {
            alphaFactor = cmsUInt32Number(cmsToFixedDomain(int(FROM_8_TO_16(accum[0]))))
        }
        accum = accum[extra:]
    } else {
        if premul != 0 && extra != 0 {
            alphaFactor = cmsUInt32Number(cmsToFixedDomain(int(FROM_8_TO_16(accum[nChan]))))
        }
    }

    for i := 0; i < nChan; i++ {
        var index int
        if doSwap != 0 {
            index = nChan - i - 1
        } else {
            index = i
        }

        v := FROM_8_TO_16(accum[0])
        if reverse != 0 {
            v = REVERSE_FLAVOR_16(v)
        }

        if premul != 0 && alphaFactor > 0 {
            v = cmsUInt16Number(((cmsUInt32Number(v) << 16) / alphaFactor))
            if v > 0xffff {
                v = 0xffff
            }
        }

        wIn[index] = cmsUInt16Number(v)
        accum = accum[1:]
    }

    if extraFirst == 0 {
        accum = accum[extra:]
    }

    if extra == 0 && swapFirst != 0 {
        tmp := wIn[0]
        copy(wIn[:nChan-1], wIn[1:nChan])
        wIn[nChan-1] = tmp
    }

    return accum
}
func UnrollPlanarBytes(info *cmsTRANSFORM, wIn []cmsUInt16Number, accum []cmsUInt8Number, stride cmsUInt32Number) []cmsUInt8Number {
    nChan := int(T_CHANNELS(uint32(uint32(info.InputFormat))))
    doSwap := T_DOSWAP(uint32(uint32(info.InputFormat)))
    swapFirst := T_SWAPFIRST(uint32(uint32(info.InputFormat)))
    reverse := T_FLAVOR(uint32(uint32(info.InputFormat)))
    extraFirst := doSwap ^ swapFirst
    extra := T_EXTRA(uint32(uint32(info.InputFormat)))
    premul := T_PREMUL(uint32(uint32(info.InputFormat)))
    init := accum
    alphaFactor := cmsUInt32Number(1)

    if extraFirst != 0 {
        if premul != 0 && extra != 0 {
            alphaFactor = cmsUInt32Number(cmsToFixedDomain(int(FROM_8_TO_16(accum[0]))))
        }
        accum = accum[int(cmsUInt32Number(extra)*stride):]
    } else {
        if premul != 0 && extra != 0 {
            alphaFactor = cmsUInt32Number(cmsToFixedDomain(int(FROM_8_TO_16(accum[nChan*int(stride)]))))
        }
    }

    for i := 0; i < nChan; i++ {
        var index int
        if doSwap != 0 {
            index = nChan - i - 1
        } else {
            index = i
        }

        v := FROM_8_TO_16(accum[0])

        if reverse != 0 {
            v = REVERSE_FLAVOR_16(v)
        }

        if premul != 0 && alphaFactor > 0 {
            v = (cmsUInt16Number)((cmsUInt32Number(v) << 16) / alphaFactor)
            if v > 0xffff {
                v = 0xffff
            }
        }

        wIn[index] = cmsUInt16Number(v)
        accum = accum[stride:]
    }

    return init[1:]
}

// Unroll4Bytes processes 4 bytes in sequence.
func Unroll4Bytes(info *cmsTRANSFORM, wIn []cmsUInt16Number, accum []cmsUInt8Number, stride cmsUInt32Number) []cmsUInt8Number {
    wIn[0] = cmsUInt16Number(FROM_8_TO_16(accum[0])) // C
    wIn[1] = cmsUInt16Number(FROM_8_TO_16(accum[1])) // M
    wIn[2] = cmsUInt16Number(FROM_8_TO_16(accum[2])) // Y
    wIn[3] = cmsUInt16Number(FROM_8_TO_16(accum[3])) // K
    return accum[4:]
}

// Unroll4BytesReverse processes 4 bytes with reverse flavor applied.
func Unroll4BytesReverse(info *cmsTRANSFORM, wIn []cmsUInt16Number, accum []cmsUInt8Number, stride cmsUInt32Number) []cmsUInt8Number {
    wIn[0] = cmsUInt16Number(FROM_8_TO_16(REVERSE_FLAVOR_8(accum[0]))) // C
    wIn[1] = cmsUInt16Number(FROM_8_TO_16(REVERSE_FLAVOR_8(accum[1]))) // M
    wIn[2] = cmsUInt16Number(FROM_8_TO_16(REVERSE_FLAVOR_8(accum[2]))) // Y
    wIn[3] = cmsUInt16Number(FROM_8_TO_16(REVERSE_FLAVOR_8(accum[3]))) // K
    return accum[4:]
}

// Unroll4BytesSwapFirst processes 4 bytes with the first byte swapped to the last position.
func Unroll4BytesSwapFirst(info *cmsTRANSFORM, wIn []cmsUInt16Number, accum []cmsUInt8Number, stride cmsUInt32Number) []cmsUInt8Number {
    wIn[3] = cmsUInt16Number(FROM_8_TO_16(accum[0])) // K
    wIn[0] = cmsUInt16Number(FROM_8_TO_16(accum[1])) // C
    wIn[1] = cmsUInt16Number(FROM_8_TO_16(accum[2])) // M
    wIn[2] = cmsUInt16Number(FROM_8_TO_16(accum[3])) // Y
    return accum[4:]
}

// Unroll4BytesSwap processes 4 bytes in KYMC order.
func Unroll4BytesSwap(info *cmsTRANSFORM, wIn []cmsUInt16Number, accum []cmsUInt8Number, stride cmsUInt32Number) []cmsUInt8Number {
    wIn[3] = cmsUInt16Number(FROM_8_TO_16(accum[0])) // K
    wIn[2] = cmsUInt16Number(FROM_8_TO_16(accum[1])) // Y
    wIn[1] = cmsUInt16Number(FROM_8_TO_16(accum[2])) // M
    wIn[0] = cmsUInt16Number(FROM_8_TO_16(accum[3])) // C
    return accum[4:]
}

// Unroll4BytesSwapSwapFirst processes 4 bytes in swapped order with the first byte swapped.
func Unroll4BytesSwapSwapFirst(info *cmsTRANSFORM, wIn []cmsUInt16Number, accum []cmsUInt8Number, stride cmsUInt32Number) []cmsUInt8Number {
    wIn[2] = cmsUInt16Number(FROM_8_TO_16(accum[0])) // K
    wIn[1] = cmsUInt16Number(FROM_8_TO_16(accum[1])) // Y
    wIn[0] = cmsUInt16Number(FROM_8_TO_16(accum[2])) // M
    wIn[3] = cmsUInt16Number(FROM_8_TO_16(accum[3])) // C
    return accum[4:]
}

// Unroll3Bytes processes 3 bytes in RGB order.
func Unroll3Bytes(info *cmsTRANSFORM, wIn []cmsUInt16Number, accum []cmsUInt8Number, stride cmsUInt32Number) []cmsUInt8Number {
    wIn[0] = cmsUInt16Number(FROM_8_TO_16(accum[0])) // R
    wIn[1] = cmsUInt16Number(FROM_8_TO_16(accum[1])) // G
    wIn[2] = cmsUInt16Number(FROM_8_TO_16(accum[2])) // B
    return accum[3:]
}

// Unroll3BytesSkip1Swap processes 3 bytes, skips 1 (A), and swaps to BRG order.
func Unroll3BytesSkip1Swap(info *cmsTRANSFORM, wIn []cmsUInt16Number, accum []cmsUInt8Number, stride cmsUInt32Number) []cmsUInt8Number {
    accum = accum[1:] // Skip A
    wIn[2] = cmsUInt16Number(FROM_8_TO_16(accum[0])) // B
    wIn[1] = cmsUInt16Number(FROM_8_TO_16(accum[1])) // G
    wIn[0] = cmsUInt16Number(FROM_8_TO_16(accum[2])) // R
    return accum[3:]
}

// Unroll3BytesSkip1SwapSwapFirst processes 3 bytes, skips 1 (A), and swaps to BRG order with the first byte swapped.
func Unroll3BytesSkip1SwapSwapFirst(info *cmsTRANSFORM, wIn []cmsUInt16Number, accum []cmsUInt8Number, stride cmsUInt32Number) []cmsUInt8Number {
    wIn[2] = cmsUInt16Number(FROM_8_TO_16(accum[0])) // B
    wIn[1] = cmsUInt16Number(FROM_8_TO_16(accum[1])) // G
    wIn[0] = cmsUInt16Number(FROM_8_TO_16(accum[2])) // R
    accum = accum[4:] // Skip 1 (A) and move forward
    return accum
}

// Unroll3BytesSkip1SwapFirst processes 3 bytes, skips 1 (A), and places R first.
func Unroll3BytesSkip1SwapFirst(info *cmsTRANSFORM, wIn []cmsUInt16Number, accum []cmsUInt8Number, stride cmsUInt32Number) []cmsUInt8Number {
    accum = accum[1:] // Skip A
    wIn[0] = cmsUInt16Number(FROM_8_TO_16(accum[0])) // R
    wIn[1] = cmsUInt16Number(FROM_8_TO_16(accum[1])) // G
    wIn[2] = cmsUInt16Number(FROM_8_TO_16(accum[2])) // B
    return accum[3:]
}

// Unroll3BytesSwap processes 3 bytes in BRG order.
func Unroll3BytesSwap(info *cmsTRANSFORM, wIn []cmsUInt16Number, accum []cmsUInt8Number, stride cmsUInt32Number) []cmsUInt8Number {
    wIn[2] = cmsUInt16Number(FROM_8_TO_16(accum[0])) // B
    wIn[1] = cmsUInt16Number(FROM_8_TO_16(accum[1])) // G
    wIn[0] = cmsUInt16Number(FROM_8_TO_16(accum[2])) // R
    return accum[3:]
}
// UnrollLabV2_8 processes Lab values from 8-bit input and converts them to 16-bit.
func UnrollLabV2_8(info *cmsTRANSFORM, wIn []cmsUInt16Number, accum []cmsUInt8Number, stride cmsUInt32Number) []cmsUInt8Number {
    wIn[0] = cmsUInt16Number(FromLabV2ToLabV4(FROM_8_TO_16(accum[0]))) // L
    wIn[1] = cmsUInt16Number(FromLabV2ToLabV4(FROM_8_TO_16(accum[1]))) // a
    wIn[2] = cmsUInt16Number(FromLabV2ToLabV4(FROM_8_TO_16(accum[2]))) // b
    return accum[3:]
}

// UnrollALabV2_8 processes ALab values from 8-bit input, skipping the alpha channel.
func UnrollALabV2_8(info *cmsTRANSFORM, wIn []cmsUInt16Number, accum []cmsUInt8Number, stride cmsUInt32Number) []cmsUInt8Number {
    accum = accum[1:] // Skip alpha
    wIn[0] = cmsUInt16Number(FromLabV2ToLabV4(FROM_8_TO_16(accum[0]))) // L
    wIn[1] = cmsUInt16Number(FromLabV2ToLabV4(FROM_8_TO_16(accum[1]))) // a
    wIn[2] = cmsUInt16Number(FromLabV2ToLabV4(FROM_8_TO_16(accum[2]))) // b
    return accum[3:]
}

// UnrollLabV2_16 processes Lab values from 16-bit input.
func UnrollLabV2_16(info *cmsTRANSFORM, wIn []cmsUInt16Number, accum []cmsUInt8Number, stride cmsUInt32Number) []cmsUInt8Number {
    wIn[0] = cmsUInt16Number(FromLabV2ToLabV4(cmsUInt16Number(accum[0]) | cmsUInt16Number(accum[1])<<8)) // L
    accum = accum[2:]
    wIn[1] = cmsUInt16Number(FromLabV2ToLabV4(cmsUInt16Number(accum[0]) | cmsUInt16Number(accum[1])<<8)) // a
    accum = accum[2:]
    wIn[2] = cmsUInt16Number(FromLabV2ToLabV4(cmsUInt16Number(accum[0]) | cmsUInt16Number(accum[1])<<8)) // b
    return accum[2:]
}

// Unroll2Bytes processes duplex values from 8-bit input.
func Unroll2Bytes(info *cmsTRANSFORM, wIn []cmsUInt16Number, accum []cmsUInt8Number, stride cmsUInt32Number) []cmsUInt8Number {
    wIn[0] = cmsUInt16Number(FROM_8_TO_16(accum[0])) // ch1
    wIn[1] = cmsUInt16Number(FROM_8_TO_16(accum[1])) // ch2
    return accum[2:]
}

// Unroll1Byte duplicates L into RGB channels for monochrome data.
func Unroll1Byte(info *cmsTRANSFORM, wIn []cmsUInt16Number, accum []cmsUInt8Number, stride cmsUInt32Number) []cmsUInt8Number {
    l := cmsUInt16Number(FROM_8_TO_16(accum[0]))
    wIn[0], wIn[1], wIn[2] = l, l, l // L
    return accum[1:]
}

// Unroll1ByteSkip1 processes monochrome data, skipping one channel.
func Unroll1ByteSkip1(info *cmsTRANSFORM, wIn []cmsUInt16Number, accum []cmsUInt8Number, stride cmsUInt32Number) []cmsUInt8Number {
    l := cmsUInt16Number(FROM_8_TO_16(accum[0]))
    wIn[0], wIn[1], wIn[2] = l, l, l // L
    return accum[2:]
}

// Unroll1ByteSkip2 processes monochrome data, skipping two channels.
func Unroll1ByteSkip2(info *cmsTRANSFORM, wIn []cmsUInt16Number, accum []cmsUInt8Number, stride cmsUInt32Number) []cmsUInt8Number {
    l := cmsUInt16Number(FROM_8_TO_16(accum[0]))
    wIn[0], wIn[1], wIn[2] = l, l, l // L
    return accum[3:]
}

// Unroll1ByteReversed processes monochrome data with reversed flavor.
func Unroll1ByteReversed(info *cmsTRANSFORM, wIn []cmsUInt16Number, accum []cmsUInt8Number, stride cmsUInt32Number) []cmsUInt8Number {
    l := cmsUInt16Number(REVERSE_FLAVOR_16(FROM_8_TO_16(accum[0])))
    wIn[0], wIn[1], wIn[2] = l, l, l // L
    return accum[1:]
}

func UnrollAnyWords(info *cmsTRANSFORM, wIn []cmsUInt16Number, accum []cmsUInt8Number, stride cmsUInt32Number) []cmsUInt8Number {
    nChan := int(T_CHANNELS(uint32(info.InputFormat)))
    swapEndian := T_ENDIAN16(uint32(info.InputFormat))
    doSwap := T_DOSWAP(uint32(info.InputFormat))
    reverse := T_FLAVOR(uint32(info.InputFormat))
    swapFirst := T_SWAPFIRST(uint32(info.InputFormat))
    extra := T_EXTRA(uint32(info.InputFormat))
    extraFirst := doSwap ^ swapFirst

    if extraFirst != 0 {
        accum = accum[extra*2:]
    }

    for i := uint32(0); i < nChan; i++ {
        index := i
        if doSwap != 0 {
            index = nChan - i - 1
        }

        v := cmsUInt16Number(accum[0]) | (cmsUInt16Number(accum[1]) << 8)

        if swapEndian != 0 {
            v = CHANGE_ENDIAN(v)
        }

        if reverse != 0 {
            v = REVERSE_FLAVOR_16(v)
        }

        wIn[index] = v
        accum = accum[2:]
    }

    if extraFirst == 0 {
        accum = accum[extra*2:]
    }

    if extra == 0 && swapFirst != 0 {
        tmp := wIn[0]
        copy(wIn[:], wIn[1:])
        wIn[nChan-1] = tmp
    }

    return accum
}

func UnrollAnyWordsPremul(info *cmsTRANSFORM, wIn []cmsUInt16Number, accum []cmsUInt8Number, stride cmsUInt32Number) []cmsUInt8Number {
    nChan := int(T_CHANNELS(uint32(info.InputFormat)))
    swapEndian := T_ENDIAN16(uint32(info.InputFormat))
    doSwap := T_DOSWAP(uint32(info.InputFormat))
    reverse := T_FLAVOR(uint32(info.InputFormat))
    swapFirst := T_SWAPFIRST(uint32(info.InputFormat))
    extraFirst := doSwap ^ swapFirst

    var alpha cmsUInt16Number
    if extraFirst != 0 {
        alpha = cmsUInt16Number(accum[0]) | (cmsUInt16Number(accum[1]) << 8)
        accum = accum[2:]
    } else {
        alpha = cmsUInt16Number(accum[(nChan-1)*2]) | (cmsUInt16Number(accum[(nChan-1)*2+1]) << 8)
    }

    alphaFactor := cmsToFixedDomain(FROM_8_TO_16(alpha))

    for i := uint32(0); i < nChan; i++ {
        index := i
        if doSwap != 0 {
            index = nChan - i - 1
        }

        v := cmsUInt16Number(accum[0]) | (cmsUInt16Number(accum[1]) << 8)

        if swapEndian != 0 {
            v = CHANGE_ENDIAN(v)
        }

        if alphaFactor > 0 {
            v = cmsUInt16Number((uint32(v) << 16) / alphaFactor)
            if v > 0xffff {
                v = 0xffff
            }
        }

        if reverse != 0 {
            v = REVERSE_FLAVOR_16(v)
        }

        wIn[index] = v
        accum = accum[2:]
    }

    return accum
}

func UnrollPlanarWords(info *cmsTRANSFORM, wIn []cmsUInt16Number, accum []cmsUInt8Number, stride cmsUInt32Number) []cmsUInt8Number {
    nChan := int(T_CHANNELS(uint32(info.InputFormat)))
    doSwap := T_DOSWAP(uint32(info.InputFormat))
    reverse := T_FLAVOR(uint32(info.InputFormat))
    swapEndian := T_ENDIAN16(uint32(info.InputFormat))

    if doSwap != 0 {
        accum = accum[T_EXTRA(uint32(info.InputFormat))*uint32(stride):]
    }

    for i := uint32(0); i < nChan; i++ {
        index := i
        if doSwap != 0 {
            index = nChan - i - 1
        }

        v := cmsUInt16Number(accum[0]) | (cmsUInt16Number(accum[1]) << 8)

        if swapEndian != 0 {
            v = CHANGE_ENDIAN(v)
        }

        if reverse != 0 {
            v = REVERSE_FLAVOR_16(v)
        }

        wIn[index] = v
        accum = accum[stride:]
    }

    return accum[:]
}

func UnrollPlanarWordsPremul(info *cmsTRANSFORM, wIn []cmsUInt16Number, accum []cmsUInt8Number, stride cmsUInt32Number) []cmsUInt8Number {
    nChan := int(T_CHANNELS(uint32(info.InputFormat)))
    doSwap := T_DOSWAP(uint32(info.InputFormat))
    swapFirst := T_SWAPFIRST(uint32(info.InputFormat))
    reverse := T_FLAVOR(uint32(info.InputFormat))
    swapEndian := T_ENDIAN16(uint32(info.InputFormat))
    extraFirst := doSwap ^ swapFirst

    var alpha cmsUInt16Number
    if extraFirst != 0 {
        alpha = cmsUInt16Number(accum[0]) | (cmsUInt16Number(accum[1]) << 8)
        accum = accum[int(stride):]
    } else {
        alpha = cmsUInt16Number(accum[(nChan-1)*int(stride)]) | (cmsUInt16Number(accum[(nChan-1)*int(stride)+1]) << 8)
    }

    alphaFactor := cmsUInt32Number(cmsToFixedDomain(int(alpha)))

    for i := 0; i < nChan; i++ {
        index := i
        if doSwap != 0 {
            index = nChan - i - 1
        }

        v := cmsUInt16Number(accum[0]) | (cmsUInt16Number(accum[1]) << 8)

        if swapEndian != 0 {
            v = CHANGE_ENDIAN(v)
        }

        if alphaFactor > 0 {
            v = cmsUInt16Number((uint32(v) << 16) / uint32(alphaFactor))
            if v > 0xffff {
                v = 0xffff
            }
        }

        if reverse != 0 {
            v = REVERSE_FLAVOR_16(v)
        }

        wIn[index] = v
        accum = accum[stride:]
    }

    return accum[:]
}
func Unroll4Words(info *cmsTRANSFORM, wIn []cmsUInt16Number, accum []cmsUInt8Number, stride cmsUInt32Number) []cmsUInt8Number {
    wIn[0] = cmsUInt16Number(accum[0]) | (cmsUInt16Number(accum[1]) << 8) // C
    accum = accum[2:]
    wIn[1] = cmsUInt16Number(accum[0]) | (cmsUInt16Number(accum[1]) << 8) // M
    accum = accum[2:]
    wIn[2] = cmsUInt16Number(accum[0]) | (cmsUInt16Number(accum[1]) << 8) // Y
    accum = accum[2:]
    wIn[3] = cmsUInt16Number(accum[0]) | (cmsUInt16Number(accum[1]) << 8) // K
    accum = accum[2:]
    return accum
}

func Unroll4WordsReverse(info *cmsTRANSFORM, wIn []cmsUInt16Number, accum []cmsUInt8Number, stride cmsUInt32Number) []cmsUInt8Number {
    wIn[0] = REVERSE_FLAVOR_16(cmsUInt16Number(accum[0]) | (cmsUInt16Number(accum[1]) << 8)) // C
    accum = accum[2:]
    wIn[1] = REVERSE_FLAVOR_16(cmsUInt16Number(accum[0]) | (cmsUInt16Number(accum[1]) << 8)) // M
    accum = accum[2:]
    wIn[2] = REVERSE_FLAVOR_16(cmsUInt16Number(accum[0]) | (cmsUInt16Number(accum[1]) << 8)) // Y
    accum = accum[2:]
    wIn[3] = REVERSE_FLAVOR_16(cmsUInt16Number(accum[0]) | (cmsUInt16Number(accum[1]) << 8)) // K
    accum = accum[2:]
    return accum
}

func Unroll4WordsSwapFirst(info *cmsTRANSFORM, wIn []cmsUInt16Number, accum []cmsUInt8Number, stride cmsUInt32Number) []cmsUInt8Number {
    wIn[3] = cmsUInt16Number(accum[0]) | (cmsUInt16Number(accum[1]) << 8) // K
    accum = accum[2:]
    wIn[0] = cmsUInt16Number(accum[0]) | (cmsUInt16Number(accum[1]) << 8) // C
    accum = accum[2:]
    wIn[1] = cmsUInt16Number(accum[0]) | (cmsUInt16Number(accum[1]) << 8) // M
    accum = accum[2:]
    wIn[2] = cmsUInt16Number(accum[0]) | (cmsUInt16Number(accum[1]) << 8) // Y
    accum = accum[2:]
    return accum
}

func Unroll4WordsSwap(info *cmsTRANSFORM, wIn []cmsUInt16Number, accum []cmsUInt8Number, stride cmsUInt32Number) []cmsUInt8Number {
    wIn[3] = cmsUInt16Number(accum[0]) | (cmsUInt16Number(accum[1]) << 8) // K
    accum = accum[2:]
    wIn[2] = cmsUInt16Number(accum[0]) | (cmsUInt16Number(accum[1]) << 8) // Y
    accum = accum[2:]
    wIn[1] = cmsUInt16Number(accum[0]) | (cmsUInt16Number(accum[1]) << 8) // M
    accum = accum[2:]
    wIn[0] = cmsUInt16Number(accum[0]) | (cmsUInt16Number(accum[1]) << 8) // C
    accum = accum[2:]
    return accum
}

func Unroll4WordsSwapSwapFirst(info *cmsTRANSFORM, wIn []cmsUInt16Number, accum []cmsUInt8Number, stride cmsUInt32Number) []cmsUInt8Number {
    wIn[2] = cmsUInt16Number(accum[0]) | (cmsUInt16Number(accum[1]) << 8) // K
    accum = accum[2:]
    wIn[1] = cmsUInt16Number(accum[0]) | (cmsUInt16Number(accum[1]) << 8) // Y
    accum = accum[2:]
    wIn[0] = cmsUInt16Number(accum[0]) | (cmsUInt16Number(accum[1]) << 8) // M
    accum = accum[2:]
    wIn[3] = cmsUInt16Number(accum[0]) | (cmsUInt16Number(accum[1]) << 8) // C
    accum = accum[2:]
    return accum
}
func Unroll3Words(info *cmsTRANSFORM, wIn []cmsUInt16Number, accum []cmsUInt8Number, stride cmsUInt32Number) []cmsUInt8Number {
    wIn[0] = cmsUInt16Number(accum[0]) | (cmsUInt16Number(accum[1]) << 8) // C R
    accum = accum[2:]
    wIn[1] = cmsUInt16Number(accum[0]) | (cmsUInt16Number(accum[1]) << 8) // M G
    accum = accum[2:]
    wIn[2] = cmsUInt16Number(accum[0]) | (cmsUInt16Number(accum[1]) << 8) // Y B
    accum = accum[2:]
    return accum
}

func Unroll3WordsSwap(info *cmsTRANSFORM, wIn []cmsUInt16Number, accum []cmsUInt8Number, stride cmsUInt32Number) []cmsUInt8Number {
    wIn[2] = cmsUInt16Number(accum[0]) | (cmsUInt16Number(accum[1]) << 8) // C R
    accum = accum[2:]
    wIn[1] = cmsUInt16Number(accum[0]) | (cmsUInt16Number(accum[1]) << 8) // M G
    accum = accum[2:]
    wIn[0] = cmsUInt16Number(accum[0]) | (cmsUInt16Number(accum[1]) << 8) // Y B
    accum = accum[2:]
    return accum
}

func Unroll3WordsSkip1Swap(info *cmsTRANSFORM, wIn []cmsUInt16Number, accum []cmsUInt8Number, stride cmsUInt32Number) []cmsUInt8Number {
    accum = accum[2:] // Skip A
    wIn[2] = cmsUInt16Number(accum[0]) | (cmsUInt16Number(accum[1]) << 8) // R
    accum = accum[2:]
    wIn[1] = cmsUInt16Number(accum[0]) | (cmsUInt16Number(accum[1]) << 8) // G
    accum = accum[2:]
    wIn[0] = cmsUInt16Number(accum[0]) | (cmsUInt16Number(accum[1]) << 8) // B
    accum = accum[2:]
    return accum
}

func Unroll3WordsSkip1SwapFirst(info *cmsTRANSFORM, wIn []cmsUInt16Number, accum []cmsUInt8Number, stride cmsUInt32Number) []cmsUInt8Number {
    accum = accum[2:] // Skip A
    wIn[0] = cmsUInt16Number(accum[0]) | (cmsUInt16Number(accum[1]) << 8) // R
    accum = accum[2:]
    wIn[1] = cmsUInt16Number(accum[0]) | (cmsUInt16Number(accum[1]) << 8) // G
    accum = accum[2:]
    wIn[2] = cmsUInt16Number(accum[0]) | (cmsUInt16Number(accum[1]) << 8) // B
    accum = accum[2:]
    return accum
}

func Unroll1Word(info *cmsTRANSFORM, wIn []cmsUInt16Number, accum []cmsUInt8Number, stride cmsUInt32Number) []cmsUInt8Number {
    word := cmsUInt16Number(accum[0]) | (cmsUInt16Number(accum[1]) << 8)
    wIn[0], wIn[1], wIn[2] = word, word, word // L duplicated to RGB
    accum = accum[2:]
    return accum
}

func Unroll1WordReversed(info *cmsTRANSFORM, wIn []cmsUInt16Number, accum []cmsUInt8Number, stride cmsUInt32Number) []cmsUInt8Number {
    word := REVERSE_FLAVOR_16(cmsUInt16Number(accum[0]) | (cmsUInt16Number(accum[1]) << 8))
    wIn[0], wIn[1], wIn[2] = word, word, word // L reversed and duplicated to RGB
    accum = accum[2:]
    return accum
}

func Unroll1WordSkip3(info *cmsTRANSFORM, wIn []cmsUInt16Number, accum []cmsUInt8Number, stride cmsUInt32Number) []cmsUInt8Number {
    word := cmsUInt16Number(accum[0]) | (cmsUInt16Number(accum[1]) << 8)
    wIn[0], wIn[1], wIn[2] = word, word, word // L duplicated to RGB
    accum = accum[8:] // Skip 3 words
    return accum
}
func Unroll2Words(info *cmsTRANSFORM, wIn []cmsUInt16Number, accum []cmsUInt8Number, stride cmsUInt32Number) []cmsUInt8Number {
    wIn[0] = cmsUInt16Number(accum[0]) | (cmsUInt16Number(accum[1]) << 8) // ch1
    accum = accum[2:]
    wIn[1] = cmsUInt16Number(accum[0]) | (cmsUInt16Number(accum[1]) << 8) // ch2
    accum = accum[2:]
    return accum
}
func UnrollLabDoubleTo16(info *cmsTRANSFORM, wIn []cmsUInt16Number, accum []cmsUInt8Number, stride cmsUInt32Number) []cmsUInt8Number {
    if T_PLANAR(uint32(info.InputFormat)) != 0 {
        var Lab cmsCIELab

        posL := accum
        posa := accum[stride:]
        posb := accum[stride*2:]

        Lab.L = *(*cmsFloat64Number)(unsafe.Pointer(&posL[0]))
        Lab.a = *(*cmsFloat64Number)(unsafe.Pointer(&posa[0]))
        Lab.b = *(*cmsFloat64Number)(unsafe.Pointer(&posb[0]))

        cmsFloat2LabEncoded(wIn, &Lab)
        return accum[8:] // sizeof(cmsFloat64Number)
    } else {
        cmsFloat2LabEncoded(wIn, (*cmsCIELab)(unsafe.Pointer(&accum[0])))
        extra := T_EXTRA(uint32(info.InputFormat))
        accum = accum[(unsafe.Sizeof(cmsCIELab) + extra*8):] // sizeof(cmsFloat64Number)
        return accum
    }
}
func UnrollLabFloatTo16(info *cmsTRANSFORM, wIn []cmsUInt16Number, accum []cmsUInt8Number, stride cmsUInt32Number) []cmsUInt8Number {
    var Lab cmsCIELab

    if T_PLANAR(uint32(info.InputFormat)) != 0 {
        posL := accum
        posa := accum[stride:]
        posb := accum[stride*2:]

        Lab.L = *(*cmsFloat64Number)(unsafe.Pointer(&posL[0]))
        Lab.a = *(*cmsFloat64Number)(unsafe.Pointer(&posa[0]))
        Lab.b = *(*cmsFloat64Number)(unsafe.Pointer(&posb[0]))

        cmsFloat2LabEncoded(wIn, &Lab)
        return accum[4:] // sizeof(cmsFloat32Number)
    } else {
        Lab.L = *(*cmsFloat64Number)(unsafe.Pointer(&accum[0]))
        Lab.a = *(*cmsFloat64Number)(unsafe.Pointer(&accum[4]))
        Lab.b = *(*cmsFloat64Number)(unsafe.Pointer(&accum[8]))

        cmsFloat2LabEncoded(wIn, &Lab)
        extra := T_EXTRA(uint32(info.InputFormat))
        accum = accum[(3+extra)*4:] // 3 components + extra
        return accum
    }
}
func UnrollXYZDoubleTo16(info *cmsTRANSFORM, wIn []cmsUInt16Number, accum []cmsUInt8Number, stride cmsUInt32Number) []cmsUInt8Number {
    if T_PLANAR(uint32(info.InputFormat)) != 0 {
        var XYZ cmsCIEXYZ

        posX := accum
        posY := accum[stride:]
        posZ := accum[stride*2:]

        XYZ.X = *(*cmsFloat64Number)(unsafe.Pointer(&posX[0]))
        XYZ.Y = *(*cmsFloat64Number)(unsafe.Pointer(&posY[0]))
        XYZ.Z = *(*cmsFloat64Number)(unsafe.Pointer(&posZ[0]))

        cmsFloat2XYZEncoded(wIn, &XYZ)

        return accum[8:] // sizeof(cmsFloat64Number)
    } else {
        cmsFloat2XYZEncoded(wIn, (*cmsCIEXYZ)(unsafe.Pointer(&accum[0])))
        extra := T_EXTRA(uint32(info.InputFormat))
        accum = accum[(8*3 + extra*8):] // sizeof(cmsCIEXYZ) + T_EXTRA * sizeof(cmsFloat64Number)
        return accum
    }
}
func UnrollXYZFloatTo16(info *cmsTRANSFORM, wIn []cmsUInt16Number, accum []cmsUInt8Number, stride cmsUInt32Number) []cmsUInt8Number {
    if T_PLANAR(uint32(info.InputFormat)) != 0{
        var XYZ cmsCIEXYZ

        posX := accum
        posY := accum[stride:]
        posZ := accum[stride*2:]

        XYZ.X = *(*cmsFloat64Number)(unsafe.Pointer(&posX[0]))
        XYZ.Y = *(*cmsFloat64Number)(unsafe.Pointer(&posY[0]))
        XYZ.Z = *(*cmsFloat64Number)(unsafe.Pointer(&posZ[0]))

        cmsFloat2XYZEncoded(wIn, &XYZ)

        return accum[4:] // sizeof(cmsFloat32Number)
    } else {
        Pt := (*[3]cmsFloat32Number)(unsafe.Pointer(&accum[0]))
        var XYZ cmsCIEXYZ

        XYZ.X = cmsFloat64Number(Pt[0])
        XYZ.Y = cmsFloat64Number(Pt[1])
        XYZ.Z = cmsFloat64Number(Pt[2])

        cmsFloat2XYZEncoded(wIn, &XYZ)

        extra := T_EXTRA(uint32(info.InputFormat))
        accum = accum[(4*3 + extra*4):] // 3 * sizeof(cmsFloat32Number) + T_EXTRA * sizeof(cmsFloat32Number)
        return accum
    }
}
func IsInkSpace(Type cmsUInt32Number) bool {
    switch T_COLORSPACE(uint32(Type)) {
    case PT_CMY, PT_CMYK, PT_MCH5, PT_MCH6, PT_MCH7, PT_MCH8, PT_MCH9, PT_MCH10,
        PT_MCH11, PT_MCH12, PT_MCH13, PT_MCH14, PT_MCH15:
        return true
    default:
        return false
    }
}
func PixelSize(Format cmsUInt32Number) cmsUInt32Number {
    fmtBytes := T_BYTES(uint32(Format))

    // For double, the T_BYTES field is zero
    if fmtBytes == 0 {
        return cmsUInt32Number(unsafe.Sizeof(cmsUInt64Number(0)))
    }

    // Otherwise, it is already correct for all formats
    return cmsUInt32Number(fmtBytes)
}
func UnrollDoubleTo16(info *cmsTRANSFORM, wIn []cmsUInt16Number, accum []cmsUInt8Number, Stride cmsUInt32Number) []cmsUInt8Number {
    nChan := int(T_CHANNELS(uint32(info.InputFormat)))
    DoSwap := T_DOSWAP(uint32(info.InputFormat))
    Reverse := T_FLAVOR(uint32(info.InputFormat))
    SwapFirst := T_SWAPFIRST(uint32(info.InputFormat))
    Extra := T_EXTRA(uint32(info.InputFormat))
    ExtraFirst := DoSwap ^ SwapFirst
    Planar := T_PLANAR(uint32(info.InputFormat))
    var start cmsUInt32Number
    maximum := func() cmsFloat64Number {
        if IsInkSpace(info.InputFormat) {
            return 655.35
        }
        return 65535.0
    }()
    Stride /= PixelSize(info.InputFormat)

    if ExtraFirst != 0 {
        start = cmsUInt32Number(Extra)
    }

    for i := 0; i < nChan; i++ {
        index := i
        if DoSwap != 0{
            index = nChan - i - 1
        }

        var v cmsFloat64Number
        if Planar != 0 {
            v = cmsFloat64Number(accum[(i+int(start))*int(Stride)])
        } else {
            v = cmsFloat64Number(accum[i+int(start)])
        }

        vi := cmsQuickSaturateWord(v * maximum)
        if Reverse != 0{
            vi = REVERSE_FLAVOR_16(vi)
        }

        wIn[index] = vi
    }

    if Extra == 0 && SwapFirst != 0 {
        tmp := wIn[0]
        copy(wIn, wIn[1:nChan])
        wIn[nChan-1] = tmp
    }

    if Planar != 0{
        return accum[cmsUInt32Number(len(wIn)):]
    }

    return accum[(nChan+int(Extra))*8:]
}
func UnrollFloatTo16(info *cmsTRANSFORM, wIn []cmsUInt16Number, accum []cmsUInt8Number, Stride cmsUInt32Number) []cmsUInt8Number {
    nChan := int(T_CHANNELS(uint32(info.InputFormat)))
    DoSwap := T_DOSWAP(uint32(info.InputFormat))
    Reverse := T_FLAVOR(uint32(info.InputFormat))
    SwapFirst := T_SWAPFIRST(uint32(info.InputFormat))
    Extra := T_EXTRA(uint32(info.InputFormat))
    ExtraFirst := DoSwap ^ SwapFirst
    Planar := T_PLANAR(uint32(info.InputFormat))
    var start cmsUInt32Number
    maximum := func() cmsFloat64Number {
        if IsInkSpace(info.InputFormat) {
            return 655.35
        }
        return 65535.0
    }()
    Stride /= PixelSize(info.InputFormat)

    if ExtraFirst != 0 {
        start =  cmsUInt32Number(Extra)
    }

    for i := 0; i < nChan; i++ {
        index := i
        if DoSwap != 0{
            index = nChan - i - 1
        }

        var v cmsFloat32Number
        if Planar != 0{
            v = cmsFloat32Number(accum[(i+int(start))*int(Stride)])
        } else {
            v = cmsFloat32Number(accum[i+int(start)])
        }

        vi := cmsQuickSaturateWord(cmsFloat64Number(v) * maximum)
        if Reverse != 0 {
            vi = REVERSE_FLAVOR_16(vi)
        }

        wIn[index] = vi
    }

    if Extra == 0 && SwapFirst != 0{
        tmp := wIn[0]
        copy(wIn, wIn[1:nChan])
        wIn[nChan-1] = tmp
    }

    if Planar != 0 {
        return accum[cmsUInt32Number(len(wIn)):]
    }

    return accum[(nChan+int(Extra))*4:]
}
func UnrollDouble1Chan(info *cmsTRANSFORM, wIn []cmsUInt16Number, accum []cmsUInt8Number, Stride cmsUInt32Number) []cmsUInt8Number {
    Inks := (*[1]cmsFloat64Number)(unsafe.Pointer(&accum[0]))

    wIn[0] = cmsQuickSaturateWord(Inks[0] * 65535.0)
    wIn[1] = wIn[0]
    wIn[2] = wIn[0]

    return accum[8:]
}
func Unroll8ToFloat(info *cmsTRANSFORM, wIn []cmsFloat32Number, accum []cmsUInt8Number, Stride cmsUInt32Number) []cmsUInt8Number {
    nChan := int(T_CHANNELS(uint32(info.InputFormat)))
    DoSwap := T_DOSWAP(uint32(info.InputFormat))
    Reverse := T_FLAVOR(uint32(info.InputFormat))
    SwapFirst := T_SWAPFIRST(uint32(info.InputFormat))
    Extra := T_EXTRA(uint32(info.InputFormat))
    ExtraFirst := DoSwap ^ SwapFirst
    Planar := T_PLANAR(uint32(info.InputFormat))
    var start cmsUInt32Number

    Stride /= PixelSize(info.InputFormat)

    if ExtraFirst !=0 {
        start = cmsUInt32Number(Extra)
    }

    for i :=0; i < nChan; i++ {
        index := i
        if DoSwap !=0 {
            index = nChan - i - 1
        }

        var v cmsFloat32Number
        if Planar !=0 {
            v = cmsFloat32Number(accum[(i+start)*Stride])
        } else {
            v = cmsFloat32Number(accum[i+start])
        }

        v /= 255.0
        wIn[index] = ReverseFloat(v, Reverse)
    }

    if Extra == 0 && SwapFirst !=0 {
        SwapFirstFloat(wIn, nChan)
    }

    if Planar !=0 {
        return accum[1:]
    }

    return accum[(nChan+Extra):]
}
func Unroll16ToFloat(info *cmsTRANSFORM, wIn []cmsFloat32Number, accum []cmsUInt8Number, Stride cmsUInt32Number) []cmsUInt8Number {
    nChan := int(T_CHANNELS(uint32(info.InputFormat)))
    DoSwap := T_DOSWAP(uint32(info.InputFormat))
    Reverse := T_FLAVOR(uint32(info.InputFormat))
    SwapFirst := T_SWAPFIRST(uint32(info.InputFormat))
    Extra := T_EXTRA(uint32(info.InputFormat))
    ExtraFirst := DoSwap ^ SwapFirst
    Planar := T_PLANAR(uint32(info.InputFormat))
    var start cmsUInt32Number

    Stride /= PixelSize(info.InputFormat)

    if ExtraFirst != 0 {
        start = cmsUInt32Number(Extra)
    }

    for i :=0; i < nChan; i++ {
        index := i
        if DoSwap !=0 {
            index = nChan - i - 1
        }

        var v cmsFloat32Number
        if Planar !=0 {
            v = cmsFloat32Number(binary.LittleEndian.Uint16(accum[(i+start)*Stride:]))
        } else {
            v = cmsFloat32Number(binary.LittleEndian.Uint16(accum[(i+start)*2:]))
        }

        v /= 65535.0
        wIn[index] = ReverseFloat(v, Reverse)
    }

    if Extra == 0 && SwapFirst !=0 {
        SwapFirstFloat(wIn, nChan)
    }

    if Planar !=0 {
        return accum[2:]
    }

    return accum[(nChan+Extra)*2:]
}
func UnrollFloatsToFloat(info *cmsTRANSFORM, wIn []cmsFloat32Number, accum []cmsUInt8Number, Stride cmsUInt32Number) []cmsUInt8Number {
    nChan := int(T_CHANNELS(uint32(info.InputFormat)))
    DoSwap := T_DOSWAP(uint32(info.InputFormat))
    Reverse := T_FLAVOR(uint32(info.InputFormat))
    SwapFirst := T_SWAPFIRST(uint32(info.InputFormat))
    Extra := T_EXTRA(uint32(info.InputFormat))
    ExtraFirst := DoSwap ^ SwapFirst
    Planar := T_PLANAR(uint32(info.InputFormat))
    Premul := T_PREMUL(uint32(info.InputFormat))
    maximum := cmsFloat32Number(1.0)
    if IsInkSpace(info.InputFormat) {
        maximum = 100.0
    }
    var alphaFactor cmsFloat32Number = 1.0
    var start cmsUInt32Number

    Stride /= PixelSize(info.InputFormat)

    if Premul !=0 && Extra > 0 {
        if Planar !=0 {
            if ExtraFirst !=0 {
                alphaFactor = cmsFloat32Number(binary.LittleEndian.Uint32(accum[:4])) / maximum
            } else {
                alphaFactor = cmsFloat32Number(binary.LittleEndian.Uint32(accum[nChan*int(Stride):])) / maximum
            }
        } else {
            if ExtraFirst !=0 {
                alphaFactor = cmsFloat32Number(binary.LittleEndian.Uint32(accum[:4])) / maximum
            } else {
                alphaFactor = cmsFloat32Number(binary.LittleEndian.Uint32(accum[nChan*4:])) / maximum
            }
        }
    }

    if ExtraFirst !=0 {
        start = cmsUInt32Number(Extra)
    }

    for i :=0; i < nChan; i++ {
        index := i
        if DoSwap !=0 {
            index = nChan - i - 1
        }

        var v cmsFloat32Number
        if Planar !=0 {
            v = cmsFloat32Number(binary.LittleEndian.Uint32(accum[(i+start)*Stride:]))
        } else {
            v = cmsFloat32Number(binary.LittleEndian.Uint32(accum[(i+start)*4:]))
        }

        if Premul !=0 && alphaFactor > 0 {
            v /= alphaFactor
        }

        v /= maximum
        wIn[index] = ReverseFloat(v, Reverse)
    }

    if Extra == 0 && SwapFirst !=0 {
        SwapFirstFloat(wIn, nChan)
    }

    if Planar !=0 {
        return accum[4:]
    }

    return accum[(nChan+Extra)*4:]
}
func UnrollDoublesToFloat(info *cmsTRANSFORM, wIn []cmsFloat32Number, accum []cmsUInt8Number, Stride cmsUInt32Number) []cmsUInt8Number {
    nChan := int(T_CHANNELS(uint32(info.InputFormat)))
    DoSwap := T_DOSWAP(uint32(info.InputFormat))
    Reverse := T_FLAVOR(uint32(info.InputFormat))
    SwapFirst := T_SWAPFIRST(uint32(info.InputFormat))
    Extra := T_EXTRA(uint32(info.InputFormat))
    ExtraFirst := DoSwap ^ SwapFirst
    Planar := T_PLANAR(uint32(info.InputFormat))
    Premul := T_PREMUL(uint32(info.InputFormat))
    maximum := 1.0
    if IsInkSpace(info.InputFormat) {
        maximum = 100.0
    }
    alphaFactor := 1.0
    var start cmsUInt32Number

    Stride /= PixelSize(info.InputFormat)

    ptr := (*[1 << 30]cmsFloat64Number)(unsafe.Pointer(&accum[0]))[:len(accum)/8]

    if Premul !=0 && Extra > 0 {
        if Planar !=0 {
            if ExtraFirst !=0 {
                alphaFactor = ptr[0] / maximum
            } else {
                alphaFactor = ptr[nChan*Stride] / maximum
            }
        } else {
            if ExtraFirst !=0 {
                alphaFactor = ptr[0] / maximum
            } else {
                alphaFactor = ptr[nChan] / maximum
            }
        }
    }

    if ExtraFirst !=0 {
        start = cmsUInt32Number(Extra)
    }

    for i :=0; i < nChan; i++ {
        index := i
        if DoSwap !=0 {
            index = nChan - i - 1
        }

        var v cmsFloat64Number
        if Planar !=0 {
            v = ptr[(i+start)*Stride]
        } else {
            v = ptr[i+start]
        }

        if Premul !=0 && alphaFactor > 0 {
            v /= alphaFactor
        }

        v /= maximum
        wIn[index] = cmsFloat32Number(ReverseFloat(v, Reverse))
    }

    if Extra == 0 && SwapFirst !=0 {
        SwapFirstFloat(wIn, nChan)
    }

    if Planar !=0 {
        return accum[8:]
    }
    return accum[(nChan+Extra)*8:]
}
func UnrollLabDoubleToFloat(info *cmsTRANSFORM, wIn []cmsFloat32Number, accum []cmsUInt8Number, Stride cmsUInt32Number) []cmsUInt8Number {
    ptr := (*[1 << 30]cmsFloat64Number)(unsafe.Pointer(&accum[0]))[:len(accum)/8]

    if T_PLANAR(uint32(info.InputFormat)) {
        Stride /= PixelSize(info.InputFormat)
        wIn[0] = cmsFloat32Number(ptr[0] / 100.0)                 // L: 0..100 to 0..1
        wIn[1] = cmsFloat32Number((ptr[Stride] + 128.0) / 255.0)  // a: -128..127 to 0..1
        wIn[2] = cmsFloat32Number((ptr[Stride*2] + 128.0) / 255.0) // b: -128..127 to 0..1
        return accum[8:]
    }

    wIn[0] = cmsFloat32Number(ptr[0] / 100.0)                 // L: 0..100 to 0..1
    wIn[1] = cmsFloat32Number((ptr[1] + 128.0) / 255.0)       // a: -128..127 to 0..1
    wIn[2] = cmsFloat32Number((ptr[2] + 128.0) / 255.0)       // b: -128..127 to 0..1

    return accum[(3+T_EXTRA(uint32(info.InputFormat)))*8:]
}
func UnrollLabFloatToFloat(info *cmsTRANSFORM, wIn []cmsFloat32Number, accum []cmsUInt8Number, Stride cmsUInt32Number) []cmsUInt8Number {
    ptr := (*[1 << 30]cmsFloat32Number)(unsafe.Pointer(&accum[0]))[:len(accum)/4]

    if T_PLANAR(uint32(info.InputFormat)) {
        Stride /= PixelSize(info.InputFormat)
        wIn[0] = ptr[0] / 100.0                              // L: 0..100 to 0..1
        wIn[1] = (ptr[Stride] + 128.0) / 255.0              // a: -128..127 to 0..1
        wIn[2] = (ptr[Stride*2] + 128.0) / 255.0            // b: -128..127 to 0..1
        return accum[4:]
    }

    wIn[0] = ptr[0] / 100.0                               // L: 0..100 to 0..1
    wIn[1] = (ptr[1] + 128.0) / 255.0                    // a: -128..127 to 0..1
    wIn[2] = (ptr[2] + 128.0) / 255.0                    // b: -128..127 to 0..1

    return accum[(3+T_EXTRA(uint32(info.InputFormat)))*4:]
}
func UnrollXYZDoubleToFloat(info *cmsTRANSFORM, wIn []cmsFloat32Number, accum []cmsUInt8Number, Stride cmsUInt32Number) []cmsUInt8Number {
    ptr := (*[1 << 30]cmsFloat64Number)(unsafe.Pointer(&accum[0]))[:len(accum)/8]

    if T_PLANAR(uint32(info.InputFormat)) {
        Stride /= PixelSize(info.InputFormat)

        wIn[0] = cmsFloat32Number(ptr[0] / MAX_ENCODEABLE_XYZ)
        wIn[1] = cmsFloat32Number(ptr[Stride] / MAX_ENCODEABLE_XYZ)
        wIn[2] = cmsFloat32Number(ptr[Stride*2] / MAX_ENCODEABLE_XYZ)

        return accum[8:]
    }

    wIn[0] = cmsFloat32Number(ptr[0] / MAX_ENCODEABLE_XYZ)
    wIn[1] = cmsFloat32Number(ptr[1] / MAX_ENCODEABLE_XYZ)
    wIn[2] = cmsFloat32Number(ptr[2] / MAX_ENCODEABLE_XYZ)

    return accum[(3+T_EXTRA(uint32(info.InputFormat)))*8:]
}
func UnrollXYZFloatToFloat(info *cmsTRANSFORM, wIn []cmsFloat32Number, accum []cmsUInt8Number, Stride cmsUInt32Number) []cmsUInt8Number {
    ptr := (*[1 << 30]cmsFloat32Number)(unsafe.Pointer(&accum[0]))[:len(accum)/4]

    if T_PLANAR(uint32(info.InputFormat)) {
        Stride /= PixelSize(info.InputFormat)

        wIn[0] = cmsFloat32Number(ptr[0] / MAX_ENCODEABLE_XYZ)
        wIn[1] = cmsFloat32Number(ptr[Stride] / MAX_ENCODEABLE_XYZ)
        wIn[2] = cmsFloat32Number(ptr[Stride*2] / MAX_ENCODEABLE_XYZ)

        return accum[4:]
    }

    wIn[0] = cmsFloat32Number(ptr[0] / MAX_ENCODEABLE_XYZ)
    wIn[1] = cmsFloat32Number(ptr[1] / MAX_ENCODEABLE_XYZ)
    wIn[2] = cmsFloat32Number(ptr[2] / MAX_ENCODEABLE_XYZ)

    return accum[(3+T_EXTRA(uint32(info.InputFormat)))*4:]
}
func lab4toFloat(wIn []cmsFloat32Number, lab4 [3]cmsUInt16Number) {
    L := cmsFloat32Number(lab4[0]) / 655.35
    a := (cmsFloat32Number(lab4[1]) / 257.0) - 128.0
    b := (cmsFloat32Number(lab4[2]) / 257.0) - 128.0

    wIn[0] = L / 100.0            // from 0..100 to 0..1
    wIn[1] = (a + 128.0) / 255.0  // from -128..+127 to 0..1
    wIn[2] = (b + 128.0) / 255.0
}
func UnrollLabV2_8ToFloat(info *cmsTRANSFORM, wIn []cmsFloat32Number, accum []cmsUInt8Number, Stride cmsUInt32Number) []cmsUInt8Number {
    lab4 := [3]cmsUInt16Number{
        FomLabV2ToLabV4(FROM_8_TO_16(accum[0])),
        FomLabV2ToLabV4(FROM_8_TO_16(accum[1])),
        FomLabV2ToLabV4(FROM_8_TO_16(accum[2])),
    }

    lab4toFloat(wIn, lab4)

    return accum[3:]
}
func UnrollALabV2_8ToFloat(info *cmsTRANSFORM, wIn []cmsFloat32Number, accum []cmsUInt8Number, Stride cmsUInt32Number) []cmsUInt8Number {
    lab4 := [3]cmsUInt16Number{
        FomLabV2ToLabV4(FROM_8_TO_16(accum[1])),
        FomLabV2ToLabV4(FROM_8_TO_16(accum[2])),
        FomLabV2ToLabV4(FROM_8_TO_16(accum[3])),
    }

    lab4toFloat(wIn, lab4)

    return accum[4:]
}
func UnrollLabV2_16ToFloat(info *cmsTRANSFORM, wIn []cmsFloat32Number, accum []cmsUInt8Number, Stride cmsUInt32Number) []cmsUInt8Number {
    lab4 := [3]cmsUInt16Number{
        FomLabV2ToLabV4(*(*cmsUInt16Number)(unsafe.Pointer(&accum[0]))),
        FomLabV2ToLabV4(*(*cmsUInt16Number)(unsafe.Pointer(&accum[2]))),
        FomLabV2ToLabV4(*(*cmsUInt16Number)(unsafe.Pointer(&accum[4]))),
    }

    lab4toFloat(wIn, lab4)

    return accum[6:]
}
func PackChunkyBytes(info *_cmsTRANSFORM, wOut []cmsUInt16Number, output []cmsUInt8Number, Stride cmsUInt32Number) []cmsUInt8Number {
    nChan := T_CHANNELS(info.OutputFormat)
    DoSwap := T_DOSWAP(info.OutputFormat)
    Reverse := T_FLAVOR(info.OutputFormat)
    Extra := T_EXTRA(info.OutputFormat)
    SwapFirst := T_SWAPFIRST(info.OutputFormat)
    Premul := T_PREMUL(info.OutputFormat)
    ExtraFirst := DoSwap ^ SwapFirst
    swap1 := output
    var v cmsUInt16Number
    var alphaFactor cmsUInt32Number

    if ExtraFirst {
        if Premul && Extra != 0 {
            alphaFactor = _cmsToFixedDomain(FROM_8_TO_16(output[0]))
        }
        output = output[Extra:]
    } else {
        if Premul && Extra != 0 {
            alphaFactor = _cmsToFixedDomain(FROM_8_TO_16(output[nChan]))
        }
    }

    for i := uint32(0); i < nChan; i++ {
        index := i
        if DoSwap != 0 {
            index = nChan - i - 1
        }

        v = wOut[index]

        if Reverse != 0 {
            v = REVERSE_FLAVOR_16(v)
        }

        if Premul != 0 {
            v = cmsUInt16Number((cmsUInt32Number(v)*alphaFactor + 0x8000) >> 16)
        }

        output[0] = FROM_16_TO_8(v)
        output = output[1:]
    }

    if ExtraFirst == 0 {
        output = output[Extra:]
    }

    if Extra == 0 && SwapFirst != 0 {
        copy(swap1[1:], swap1[:nChan-1])
        swap1[0] = FROM_16_TO_8(v)
    }

    return output
}
func PackChunkyWords(info *_cmsTRANSFORM, wOut []cmsUInt16Number, output []cmsUInt8Number, Stride cmsUInt32Number) []cmsUInt8Number {
    nChan := T_CHANNELS(info.OutputFormat)
    SwapEndian := T_ENDIAN16(info.OutputFormat)
    DoSwap := T_DOSWAP(info.OutputFormat)
    Reverse := T_FLAVOR(info.OutputFormat)
    Extra := T_EXTRA(info.OutputFormat)
    SwapFirst := T_SWAPFIRST(info.OutputFormat)
    Premul := T_PREMUL(info.OutputFormat)
    ExtraFirst := DoSwap ^ SwapFirst
    swap1 := (*cmsUInt16Number)(unsafe.Pointer(&output[0]))
    var v cmsUInt16Number
    var alphaFactor cmsUInt32Number

    if ExtraFirst != 0 {
        if Premul != 0 && Extra != 0 {
            alphaFactor = _cmsToFixedDomain(*(*cmsUInt16Number)(unsafe.Pointer(&output[0])))
        }
        output = output[Extra*2:]
    } else {
        if Premul != 0 && Extra != 0 {
            alphaFactor = _cmsToFixedDomain((*(*cmsUInt16Number)(unsafe.Pointer(&output[0])))[nChan])
        }
    }

    for i := uint32(0); i < nChan; i++ {
        index := i
        if DoSwap != 0 {
            index = nChan - i - 1
        }

        v = wOut[index]

        if SwapEndian != 0 {
            v = CHANGE_ENDIAN(v)
        }

        if Reverse != 0 {
            v = REVERSE_FLAVOR_16(v)
        }

        if Premul != 0 {
            v = cmsUInt16Number((cmsUInt32Number(v)*alphaFactor + 0x8000) >> 16)
        }

        *(*cmsUInt16Number)(unsafe.Pointer(&output[0])) = v
        output = output[2:]
    }

    if ExtraFirst == 0 {
        output = output[Extra*2:]
    }

    if Extra == 0 && SwapFirst != 0 {
        copy(swap1[1:], swap1[:nChan-1])
        swap1[0] = v
    }

    return output
}
func PackPlanarBytes(info *_cmsTRANSFORM, wOut []cmsUInt16Number, output []cmsUInt8Number, Stride cmsUInt32Number) []cmsUInt8Number {
    nChan := T_CHANNELS(info.OutputFormat)
    DoSwap := T_DOSWAP(info.OutputFormat)
    SwapFirst := T_SWAPFIRST(info.OutputFormat)
    Reverse := T_FLAVOR(info.OutputFormat)
    Extra := T_EXTRA(info.OutputFormat)
    ExtraFirst := DoSwap ^ SwapFirst
    Premul := T_PREMUL(info.OutputFormat)
    Init := output
    var alphaFactor cmsUInt32Number

    if ExtraFirst != 0 {
        if Premul != 0 && Extra != 0 {
            alphaFactor = _cmsToFixedDomain(FROM_8_TO_16(output[0]))
        }
        output = output[Extra*Stride:]
    } else {
        if Premul != 0 && Extra != 0 {
            alphaFactor = _cmsToFixedDomain(FROM_8_TO_16(output[nChan*Stride]))
        }
    }

    for i := uint32(0); i < nChan; i++ {
        index := i
        if DoSwap != 0 {
            index = nChan - i - 1
        }

        v := wOut[index]

        if Reverse != 0 {
            v = REVERSE_FLAVOR_16(v)
        }

        if Premul != 0 {
            v = cmsUInt16Number((cmsUInt32Number(v)*alphaFactor + 0x8000) >> 16)
        }

        output[0] = FROM_16_TO_8(v)
        output = output[Stride:]
    }

    return Init[1:]
}
func PackPlanarWords(info *_cmsTRANSFORM, wOut []cmsUInt16Number, output []cmsUInt8Number, Stride cmsUInt32Number) []cmsUInt8Number {
    nChan := T_CHANNELS(info.OutputFormat)
    DoSwap := T_DOSWAP(info.OutputFormat)
    SwapFirst := T_SWAPFIRST(info.OutputFormat)
    Reverse := T_FLAVOR(info.OutputFormat)
    Extra := T_EXTRA(info.OutputFormat)
    ExtraFirst := DoSwap ^ SwapFirst
    Premul := T_PREMUL(info.OutputFormat)
    SwapEndian := T_ENDIAN16(info.OutputFormat)
    Init := output
    var v cmsUInt16Number
    var alphaFactor cmsUInt32Number

    if ExtraFirst != 0 {
        if Premul != 0 && Extra != 0 {
            alphaFactor = _cmsToFixedDomain(*(*cmsUInt16Number)(unsafe.Pointer(&output[0])))
        }
        output = output[Extra*Stride:]
    } else {
        if Premul != 0 && Extra != 0 {
            alphaFactor = _cmsToFixedDomain((*(*cmsUInt16Number)(unsafe.Pointer(&output[0])))[nChan*Stride])
        }
    }

    for i := uint32(0); i < nChan; i++ {
        index := i
        if DoSwap != 0 {
            index = nChan - i - 1
        }

        v = wOut[index]

        if SwapEndian != 0 {
            v = CHANGE_ENDIAN(v)
        }

        if Reverse != 0 {
            v = REVERSE_FLAVOR_16(v)
        }

        if Premul != 0 {
            v = cmsUInt16Number((cmsUInt32Number(v)*alphaFactor + 0x8000) >> 16)
        }

        *(*cmsUInt16Number)(unsafe.Pointer(&output[0])) = v
        output = output[Stride:]
    }

    return Init[2:]
}
func Pack6Bytes(info *_cmsTRANSFORM, wOut []cmsUInt16Number, output []cmsUInt8Number, Stride cmsUInt32Number) []cmsUInt8Number {
    output[0] = FROM_16_TO_8(wOut[0])
    output[1] = FROM_16_TO_8(wOut[1])
    output[2] = FROM_16_TO_8(wOut[2])
    output[3] = FROM_16_TO_8(wOut[3])
    output[4] = FROM_16_TO_8(wOut[4])
    output[5] = FROM_16_TO_8(wOut[5])

    return output[6:]
}
func Pack6BytesSwap(info *_cmsTRANSFORM, wOut []cmsUInt16Number, output []cmsUInt8Number, Stride cmsUInt32Number) []cmsUInt8Number {
    output[0] = FROM_16_TO_8(wOut[5])
    output[1] = FROM_16_TO_8(wOut[4])
    output[2] = FROM_16_TO_8(wOut[3])
    output[3] = FROM_16_TO_8(wOut[2])
    output[4] = FROM_16_TO_8(wOut[1])
    output[5] = FROM_16_TO_8(wOut[0])

    return output[6:]
}
func Pack6Words(info *_cmsTRANSFORM, wOut []cmsUInt16Number, output []cmsUInt8Number, Stride cmsUInt32Number) []cmsUInt8Number {
    for i := 0; i < 6; i++ {
        *(*cmsUInt16Number)(unsafe.Pointer(&output[i*2])) = wOut[i]
    }
    return output[12:]
}
func Pack6WordsSwap(info *_cmsTRANSFORM, wOut []cmsUInt16Number, output []cmsUInt8Number, Stride cmsUInt32Number) []cmsUInt8Number {
    for i := 0; i < 6; i++ {
        *(*cmsUInt16Number)(unsafe.Pointer(&output[i*2])) = wOut[5-i]
    }
    return output[12:]
}
func Pack4Bytes(info *_cmsTRANSFORM, wOut []cmsUInt16Number, output []cmsUInt8Number, Stride cmsUInt32Number) []cmsUInt8Number {
    for i := 0; i < 4; i++ {
        output[i] = FROM_16_TO_8(wOut[i])
    }
    return output[4:]
}
func Pack4BytesReverse(info *_cmsTRANSFORM, wOut []cmsUInt16Number, output []cmsUInt8Number, Stride cmsUInt32Number) []cmsUInt8Number {
    for i := 0; i < 4; i++ {
        output[i] = REVERSE_FLAVOR_8(FROM_16_TO_8(wOut[i]))
    }
    return output[4:]
}
func Pack4BytesSwapFirst(info *_cmsTRANSFORM, wOut []cmsUInt16Number, output []cmsUInt8Number, Stride cmsUInt32Number) []cmsUInt8Number {
    output[0] = FROM_16_TO_8(wOut[3])
    output[1] = FROM_16_TO_8(wOut[0])
    output[2] = FROM_16_TO_8(wOut[1])
    output[3] = FROM_16_TO_8(wOut[2])

    return output[4:]
}
func Pack4BytesSwap(info *_cmsTRANSFORM, wOut []cmsUInt16Number, output []cmsUInt8Number, Stride cmsUInt32Number) []cmsUInt8Number {
    for i := 0; i < 4; i++ {
        output[i] = FROM_16_TO_8(wOut[3-i])
    }
    return output[4:]
}
func Pack4BytesSwapSwapFirst(info *_cmsTRANSFORM, wOut []cmsUInt16Number, output []cmsUInt8Number, Stride cmsUInt32Number) []cmsUInt8Number {
    output[0] = FROM_16_TO_8(wOut[2])
    output[1] = FROM_16_TO_8(wOut[1])
    output[2] = FROM_16_TO_8(wOut[0])
    output[3] = FROM_16_TO_8(wOut[3])

    return output[4:]
}
func Pack4Words(info *_cmsTRANSFORM, wOut []cmsUInt16Number, output []cmsUInt8Number, Stride cmsUInt32Number) []cmsUInt8Number {
    for i := 0; i < 4; i++ {
        *(*cmsUInt16Number)(unsafe.Pointer(&output[i*2])) = wOut[i]
    }
    return output[8:]
}
func Pack4WordsReverse(info *_cmsTRANSFORM, wOut []cmsUInt16Number, output []cmsUInt8Number, Stride cmsUInt32Number) []cmsUInt8Number {
    for i := 0; i < 4; i++ {
        *(*cmsUInt16Number)(unsafe.Pointer(&output[i*2])) = REVERSE_FLAVOR_16(wOut[i])
    }
    return output[8:]
}
func Pack4WordsSwap(info *_cmsTRANSFORM, wOut []cmsUInt16Number, output []cmsUInt8Number, Stride cmsUInt32Number) []cmsUInt8Number {
    for i := 0; i < 4; i++ {
        *(*cmsUInt16Number)(unsafe.Pointer(&output[i*2])) = wOut[3-i]
    }
    return output[8:]
}
func Pack4WordsBigEndian(info *_cmsTRANSFORM, wOut []cmsUInt16Number, output []cmsUInt8Number, Stride cmsUInt32Number) []cmsUInt8Number {
    for i := 0; i < 4; i++ {
        *(*cmsUInt16Number)(unsafe.Pointer(&output[i*2])) = CHANGE_ENDIAN(wOut[i])
    }
    return output[8:]
}
func PackLabV2_8(info *_cmsTRANSFORM, wOut []cmsUInt16Number, output []cmsUInt8Number, Stride cmsUInt32Number) []cmsUInt8Number {
    output[0] = FROM_16_TO_8(FomLabV4ToLabV2(wOut[0]))
    output[1] = FROM_16_TO_8(FomLabV4ToLabV2(wOut[1]))
    output[2] = FROM_16_TO_8(FomLabV4ToLabV2(wOut[2]))

    return output[3:]
}
func PackALabV2_8(info *_cmsTRANSFORM, wOut []cmsUInt16Number, output []cmsUInt8Number, Stride cmsUInt32Number) []cmsUInt8Number {
    output[0] = 0 // Placeholder for alpha channel
    output[1] = FROM_16_TO_8(FomLabV4ToLabV2(wOut[0]))
    output[2] = FROM_16_TO_8(FomLabV4ToLabV2(wOut[1]))
    output[3] = FROM_16_TO_8(FomLabV4ToLabV2(wOut[2]))

    return output[4:]
}
func PackLabV2_16(info *_cmsTRANSFORM, wOut []cmsUInt16Number, output []cmsUInt8Number, Stride cmsUInt32Number) []cmsUInt8Number {
    *(*cmsUInt16Number)(unsafe.Pointer(&output[0])) = FomLabV4ToLabV2(wOut[0])
    *(*cmsUInt16Number)(unsafe.Pointer(&output[2])) = FomLabV4ToLabV2(wOut[1])
    *(*cmsUInt16Number)(unsafe.Pointer(&output[4])) = FomLabV4ToLabV2(wOut[2])

    return output[6:]
}
func Pack3Bytes(info *_cmsTRANSFORM, wOut []cmsUInt16Number, output []cmsUInt8Number, Stride cmsUInt32Number) []cmsUInt8Number {
    output[0] = FROM_16_TO_8(wOut[0])
    output[1] = FROM_16_TO_8(wOut[1])
    output[2] = FROM_16_TO_8(wOut[2])

    return output[3:]
}
func Pack3BytesOptimized(info *_cmsTRANSFORM, wOut []cmsUInt16Number, output []cmsUInt8Number, Stride cmsUInt32Number) []cmsUInt8Number {
    output[0] = cmsUInt8Number(wOut[0] & 0xFF)
    output[1] = cmsUInt8Number(wOut[1] & 0xFF)
    output[2] = cmsUInt8Number(wOut[2] & 0xFF)

    return output[3:]
}
func Pack3BytesSwap(info *_cmsTRANSFORM, wOut []cmsUInt16Number, output []cmsUInt8Number, Stride cmsUInt32Number) []cmsUInt8Number {
    output[0] = FROM_16_TO_8(wOut[2])
    output[1] = FROM_16_TO_8(wOut[1])
    output[2] = FROM_16_TO_8(wOut[0])

    return output[3:]
}
func Pack3BytesSwapOptimized(info *_cmsTRANSFORM, wOut []cmsUInt16Number, output []cmsUInt8Number, Stride cmsUInt32Number) []cmsUInt8Number {
    output[0] = cmsUInt8Number(wOut[2] & 0xFF)
    output[1] = cmsUInt8Number(wOut[1] & 0xFF)
    output[2] = cmsUInt8Number(wOut[0] & 0xFF)

    return output[3:]
}
func Pack3Words(info *_cmsTRANSFORM, wOut []cmsUInt16Number, output []cmsUInt8Number, Stride cmsUInt32Number) []cmsUInt8Number {
    *(*cmsUInt16Number)(unsafe.Pointer(&output[0])) = wOut[0]
    *(*cmsUInt16Number)(unsafe.Pointer(&output[2])) = wOut[1]
    *(*cmsUInt16Number)(unsafe.Pointer(&output[4])) = wOut[2]

    return output[6:]
}
func Pack3WordsSwap(info *_cmsTRANSFORM, wOut []cmsUInt16Number, output []cmsUInt8Number, Stride cmsUInt32Number) []cmsUInt8Number {
    *(*cmsUInt16Number)(unsafe.Pointer(&output[0])) = wOut[2]
    *(*cmsUInt16Number)(unsafe.Pointer(&output[2])) = wOut[1]
    *(*cmsUInt16Number)(unsafe.Pointer(&output[4])) = wOut[0]

    return output[6:]
}
func Pack3WordsBigEndian(info *_cmsTRANSFORM, wOut []cmsUInt16Number, output []cmsUInt8Number, Stride cmsUInt32Number) []cmsUInt8Number {
    *(*cmsUInt16Number)(unsafe.Pointer(&output[0])) = CHANGE_ENDIAN(wOut[0])
    *(*cmsUInt16Number)(unsafe.Pointer(&output[2])) = CHANGE_ENDIAN(wOut[1])
    *(*cmsUInt16Number)(unsafe.Pointer(&output[4])) = CHANGE_ENDIAN(wOut[2])

    return output[6:]
}
func PackDoubleFrom16(info *_cmsTRANSFORM, wOut []cmsUInt16Number, output []cmsUInt8Number, Stride cmsUInt32Number) []cmsUInt8Number {
    nChan := T_CHANNELS(info.OutputFormat)
    DoSwap := T_DOSWAP(info.OutputFormat)
    Reverse := T_FLAVOR(info.OutputFormat)
    Extra := T_EXTRA(info.OutputFormat)
    SwapFirst := T_SWAPFIRST(info.OutputFormat)
    Planar := T_PLANAR(info.OutputFormat)
    ExtraFirst := DoSwap ^ SwapFirst
    var maximum cmsFloat64Number = 65535.0
    if IsInkSpace(info.OutputFormat) {
        maximum = 655.35
    }
    var v cmsFloat64Number
    outputFloats := (*[1 << 30]cmsFloat64Number)(unsafe.Pointer(&output[0]))[:len(output)/8]
    var start cmsUInt32Number

    Stride /= PixelSize(info.OutputFormat)

    if ExtraFirst != 0 {
        start = Extra
    }

    for i := cmsUInt32Number(0); i < nChan; i++ {
        var index cmsUInt32Number
        if DoSwap != 0 {
            index = nChan - i - 1
        } else {
            index = i
        }

        v = cmsFloat64Number(wOut[index]) / maximum

        if Reverse != 0 {
            v = maximum - v
        }

        if Planar != 0 {
            outputFloats[(i+start)*Stride] = v
        } else {
            outputFloats[i+start] = v
        }
    }

    if Extra == 0 && SwapFirst != 0 {
        copy(outputFloats[1:], outputFloats[:nChan-1])
        outputFloats[0] = v
    }

    if Planar != 0 {
        return output[8:] // sizeof(cmsFloat64Number)
    } else {
        return output[(nChan+Extra)*8:]
    }
}
func PackFloatFrom16(info *_cmsTRANSFORM, wOut []cmsUInt16Number, output []cmsUInt8Number, Stride cmsUInt32Number) []cmsUInt8Number {
    nChan := T_CHANNELS(info.OutputFormat)
    DoSwap := T_DOSWAP(info.OutputFormat)
    Reverse := T_FLAVOR(info.OutputFormat)
    Extra := T_EXTRA(info.OutputFormat)
    SwapFirst := T_SWAPFIRST(info.OutputFormat)
    Planar := T_PLANAR(info.OutputFormat)
    ExtraFirst := DoSwap ^ SwapFirst
    var maximum cmsFloat64Number = 65535.0
    if IsInkSpace(info.OutputFormat) {
        maximum = 655.35
    }
    var v cmsFloat64Number
    outputFloats := (*[1 << 30]cmsFloat32Number)(unsafe.Pointer(&output[0]))[:len(output)/4]
    var start cmsUInt32Number

    Stride /= PixelSize(info.OutputFormat)

    if ExtraFirst != 0 {
        start = Extra
    }

    for i := cmsUInt32Number(0); i < nChan; i++ {
        var index cmsUInt32Number
        if DoSwap != 0 {
            index = nChan - i - 1
        } else {
            index = i
        }

        v = cmsFloat64Number(wOut[index]) / maximum

        if Reverse != 0 {
            v = maximum - v
        }

        if Planar != 0 {
            outputFloats[(i+start)*Stride] = cmsFloat32Number(v)
        } else {
            outputFloats[i+start] = cmsFloat32Number(v)
        }
    }

    if Extra == 0 && SwapFirst != 0 {
        copy(outputFloats[1:], outputFloats[:nChan-1])
        outputFloats[0] = cmsFloat32Number(v)
    }

    if Planar != 0 {
        return output[4:] // sizeof(cmsFloat32Number)
    } else {
        return output[(nChan+Extra)*4:]
    }
}
func PackFloatsFromFloat(info *_cmsTRANSFORM, wOut []cmsFloat32Number, output []cmsUInt8Number, Stride cmsUInt32Number) []cmsUInt8Number {
    nChan := T_CHANNELS(info.OutputFormat)
    DoSwap := T_DOSWAP(info.OutputFormat)
    Reverse := T_FLAVOR(info.OutputFormat)
    Extra := T_EXTRA(info.OutputFormat)
    SwapFirst := T_SWAPFIRST(info.OutputFormat)
    Planar := T_PLANAR(info.OutputFormat)
    ExtraFirst := DoSwap ^ SwapFirst
    maximum := cmsFloat64Number(1.0)
    if IsInkSpace(info.OutputFormat) {
        maximum = 100.0
    }
    outputFloats := (*[1 << 30]cmsFloat32Number)(unsafe.Pointer(&output[0]))[:len(output)/4]
    var v cmsFloat64Number
    var start cmsUInt32Number

    Stride /= PixelSize(info.OutputFormat)

    if ExtraFirst != 0 {
        start = Extra
    }

    for i := cmsUInt32Number(0); i < nChan; i++ {
        var index cmsUInt32Number
        if DoSwap != 0 {
            index = nChan - i - 1
        } else {
            index = i
        }

        v = cmsFloat64Number(wOut[index]) * maximum

        if Reverse != 0 {
            v = maximum - v
        }

        if Planar != 0 {
            outputFloats[(i+start)*Stride] = cmsFloat32Number(v)
        } else {
            outputFloats[i+start] = cmsFloat32Number(v)
        }
    }

    if Extra == 0 && SwapFirst != 0 {
        copy(outputFloats[1:], outputFloats[:nChan-1])
        outputFloats[0] = cmsFloat32Number(v)
    }

    if Planar != 0 {
        return output[4:]
    } else {
        return output[(nChan+Extra)*4:]
    }
}
func PackDoublesFromFloat(info *_cmsTRANSFORM, wOut []cmsFloat32Number, output []cmsUInt8Number, Stride cmsUInt32Number) []cmsUInt8Number {
    nChan := T_CHANNELS(info.OutputFormat)
    DoSwap := T_DOSWAP(info.OutputFormat)
    Reverse := T_FLAVOR(info.OutputFormat)
    Extra := T_EXTRA(info.OutputFormat)
    SwapFirst := T_SWAPFIRST(info.OutputFormat)
    Planar := T_PLANAR(info.OutputFormat)
    ExtraFirst := DoSwap ^ SwapFirst
    maximum := cmsFloat64Number(1.0)
    if IsInkSpace(info.OutputFormat) {
        maximum = 100.0
    }
    outputFloats := (*[1 << 30]cmsFloat64Number)(unsafe.Pointer(&output[0]))[:len(output)/8]
    var v cmsFloat64Number
    var start cmsUInt32Number

    Stride /= PixelSize(info.OutputFormat)

    if ExtraFirst != 0 {
        start = Extra
    }

    for i := cmsUInt32Number(0); i < nChan; i++ {
        var index cmsUInt32Number
        if DoSwap != 0 {
            index = nChan - i - 1
        } else {
            index = i
        }

        v = cmsFloat64Number(wOut[index]) * maximum

        if Reverse != 0 {
            v = maximum - v
        }

        if Planar != 0 {
            outputFloats[(i+start)*Stride] = v
        } else {
            outputFloats[i+start] = v
        }
    }

    if Extra == 0 && SwapFirst != 0 {
        copy(outputFloats[1:], outputFloats[:nChan-1])
        outputFloats[0] = v
    }

    if Planar != 0 {
        return output[8:]
    } else {
        return output[(nChan+Extra)*8:]
    }
}
func PackLabFloatFromFloat(info *_cmsTRANSFORM, wOut []cmsFloat32Number, output []cmsUInt8Number, Stride cmsUInt32Number) []cmsUInt8Number {
    if T_PLANAR(info.OutputFormat) {
        out := (*[1 << 30]cmsFloat32Number)(unsafe.Pointer(&output[0]))[:len(output)/4]
        Stride /= PixelSize(info.OutputFormat)

        out[0] = wOut[0] * 100.0
        out[Stride] = wOut[1]*255.0 - 128.0
        out[Stride*2] = wOut[2]*255.0 - 128.0

        return output[4:]
    } else {
        out := (*[3]cmsFloat32Number)(unsafe.Pointer(&output[0]))
        out[0] = wOut[0] * 100.0
        out[1] = wOut[1]*255.0 - 128.0
        out[2] = wOut[2]*255.0 - 128.0

        return output[12+(T_EXTRA(info.OutputFormat)*4):]
    }
}
func PackLabDoubleFromFloat(info *_cmsTRANSFORM, wOut []cmsFloat32Number, output []cmsUInt8Number, Stride cmsUInt32Number) []cmsUInt8Number {
    if T_PLANAR(info.OutputFormat) {
        out := (*[1 << 30]cmsFloat64Number)(unsafe.Pointer(&output[0]))[:len(output)/8]
        Stride /= PixelSize(info.OutputFormat)

        out[0] = cmsFloat64Number(wOut[0] * 100.0)
        out[Stride] = cmsFloat64Number(wOut[1]*255.0 - 128.0)
        out[Stride*2] = cmsFloat64Number(wOut[2]*255.0 - 128.0)

        return output[8:]
    } else {
        out := (*[3]cmsFloat64Number)(unsafe.Pointer(&output[0]))
        out[0] = cmsFloat64Number(wOut[0] * 100.0)
        out[1] = cmsFloat64Number(wOut[1]*255.0 - 128.0)
        out[2] = cmsFloat64Number(wOut[2]*255.0 - 128.0)

        return output[24+(T_EXTRA(info.OutputFormat)*8):]
    }
}
func PackXYZFloatFromFloat(info *_cmsTRANSFORM, wOut []cmsFloat32Number, output []cmsUInt8Number, Stride cmsUInt32Number) []cmsUInt8Number {
    if T_PLANAR(info.OutputFormat) {
        out := (*[1 << 30]cmsFloat32Number)(unsafe.Pointer(&output[0]))[:len(output)/4]
        Stride /= PixelSize(info.OutputFormat)

        out[0] = wOut[0] * MAX_ENCODEABLE_XYZ
        out[Stride] = wOut[1] * MAX_ENCODEABLE_XYZ
        out[Stride*2] = wOut[2] * MAX_ENCODEABLE_XYZ

        return output[4:]
    } else {
        out := (*[3]cmsFloat32Number)(unsafe.Pointer(&output[0]))
        out[0] = wOut[0] * MAX_ENCODEABLE_XYZ
        out[1] = wOut[1] * MAX_ENCODEABLE_XYZ
        out[2] = wOut[2] * MAX_ENCODEABLE_XYZ

        return output[12+(T_EXTRA(info.OutputFormat)*4):]
    }
}
func PackXYZDoubleFromFloat(info *_cmsTRANSFORM, wOut []cmsFloat32Number, output []cmsUInt8Number, Stride cmsUInt32Number) []cmsUInt8Number {
    if T_PLANAR(info.OutputFormat) {
        out := (*[1 << 30]cmsFloat64Number)(unsafe.Pointer(&output[0]))[:len(output)/8]
        Stride /= PixelSize(info.OutputFormat)

        out[0] = cmsFloat64Number(wOut[0] * MAX_ENCODEABLE_XYZ)
        out[Stride] = cmsFloat64Number(wOut[1] * MAX_ENCODEABLE_XYZ)
        out[Stride*2] = cmsFloat64Number(wOut[2] * MAX_ENCODEABLE_XYZ)

        return output[8:]
    } else {
        out := (*[3]cmsFloat64Number)(unsafe.Pointer(&output[0]))
        out[0] = cmsFloat64Number(wOut[0] * MAX_ENCODEABLE_XYZ)
        out[1] = cmsFloat64Number(wOut[1] * MAX_ENCODEABLE_XYZ)
        out[2] = cmsFloat64Number(wOut[2] * MAX_ENCODEABLE_XYZ)

        return output[24+(T_EXTRA(info.OutputFormat)*8):]
    }
}
func UnrollHalfTo16(info *_cmsTRANSFORM, wIn []cmsUInt16Number, accum []cmsUInt8Number, Stride cmsUInt32Number) []cmsUInt8Number {
    nChan := T_CHANNELS(info.InputFormat)
    DoSwap := T_DOSWAP(info.InputFormat)
    Reverse := T_FLAVOR(info.InputFormat)
    SwapFirst := T_SWAPFIRST(info.InputFormat)
    Extra := T_EXTRA(info.InputFormat)
    ExtraFirst := DoSwap ^ SwapFirst
    Planar := T_PLANAR(info.InputFormat)
    var start cmsUInt32Number
    var v cmsFloat32Number
    var maximum cmsFloat32Number = 65535.0
    if IsInkSpace(info.InputFormat) {
        maximum = 655.35
    }

    accumHalf := (*[1 << 30]cmsUInt16Number)(unsafe.Pointer(&accum[0]))[:len(accum)/2]

    Stride /= PixelSize(info.InputFormat)

    if ExtraFirst != 0 {
        start = Extra
    }

    for i := cmsUInt32Number(0); i < nChan; i++ {
        var index cmsUInt32Number
        if DoSwap != 0 {
            index = nChan - i - 1
        } else {
            index = i
        }

        if Planar != 0 {
            v = _cmsHalf2Float(accumHalf[(i+start)*Stride])
        } else {
            v = _cmsHalf2Float(accumHalf[i+start])
        }

        if Reverse != 0 {
            v = maximum - v
        }

        wIn[index] = _cmsQuickSaturateWord(cmsFloat64Number(v * maximum))
    }

    if Extra == 0 && SwapFirst != 0 {
        tmp := wIn[0]
        copy(wIn[0:], wIn[1:])
        wIn[nChan-1] = tmp
    }

    if Planar != 0 {
        return accum[2:] // sizeof(cmsUInt16Number)
    } else {
        return accum[(nChan+Extra)*2:]
    }
}
func UnrollHalfToFloat(info *_cmsTRANSFORM, wIn []cmsFloat32Number, accum []cmsUInt8Number, Stride cmsUInt32Number) []cmsUInt8Number {
    nChan := T_CHANNELS(info.InputFormat)
    DoSwap := T_DOSWAP(info.InputFormat)
    Reverse := T_FLAVOR(info.InputFormat)
    SwapFirst := T_SWAPFIRST(info.InputFormat)
    Extra := T_EXTRA(info.InputFormat)
    ExtraFirst := DoSwap ^ SwapFirst
    Planar := T_PLANAR(info.InputFormat)
    var start cmsUInt32Number
    var v cmsFloat32Number
    var maximum cmsFloat32Number = 1.0
    if IsInkSpace(info.InputFormat) {
        maximum = 100.0
    }

    accumHalf := (*[1 << 30]cmsUInt16Number)(unsafe.Pointer(&accum[0]))[:len(accum)/2]

    Stride /= PixelSize(info.InputFormat)

    if ExtraFirst != 0 {
        start = Extra
    }

    for i := cmsUInt32Number(0); i < nChan; i++ {
        var index cmsUInt32Number
        if DoSwap != 0 {
            index = nChan - i - 1
        } else {
            index = i
        }

        if Planar != 0 {
            v = _cmsHalf2Float(accumHalf[(i+start)*Stride])
        } else {
            v = _cmsHalf2Float(accumHalf[i+start])
        }

        v /= maximum

        if Reverse != 0 {
            v = 1 - v
        }

        wIn[index] = v
    }

    if Extra == 0 && SwapFirst != 0 {
        tmp := wIn[0]
        copy(wIn[0:], wIn[1:])
        wIn[nChan-1] = tmp
    }

    if Planar != 0 {
        return accum[2:] // sizeof(cmsUInt16Number)
    } else {
        return accum[(nChan+Extra)*2:]
    }
}
func PackHalfFrom16(info *_cmsTRANSFORM, wOut []cmsUInt16Number, output []cmsUInt8Number, Stride cmsUInt32Number) []cmsUInt8Number {
    nChan := T_CHANNELS(info.OutputFormat)
    DoSwap := T_DOSWAP(info.OutputFormat)
    Reverse := T_FLAVOR(info.OutputFormat)
    Extra := T_EXTRA(info.OutputFormat)
    SwapFirst := T_SWAPFIRST(info.OutputFormat)
    Planar := T_PLANAR(info.OutputFormat)
    ExtraFirst := DoSwap ^ SwapFirst
    var maximum cmsFloat32Number = 65535.0
    if IsInkSpace(info.OutputFormat) {
        maximum = 655.35
    }
    var v cmsFloat32Number
    outputHalf := (*[1 << 30]cmsUInt16Number)(unsafe.Pointer(&output[0]))[:len(output)/2]
    var start cmsUInt32Number

    Stride /= PixelSize(info.OutputFormat)

    if ExtraFirst != 0 {
        start = Extra
    }

    for i := cmsUInt32Number(0); i < nChan; i++ {
        var index cmsUInt32Number
        if DoSwap != 0 {
            index = nChan - i - 1
        } else {
            index = i
        }

        v = cmsFloat32Number(wOut[index]) / maximum

        if Reverse != 0 {
            v = maximum - v
        }

        if Planar != 0 {
            outputHalf[(i+start)*Stride] = _cmsFloat2Half(v)
        } else {
            outputHalf[i+start] = _cmsFloat2Half(v)
        }
    }

    if Extra == 0 && SwapFirst != 0 {
        copy(outputHalf[1:], outputHalf[:nChan-1])
        outputHalf[0] = _cmsFloat2Half(v)
    }

    if Planar != 0 {
        return output[2:] // sizeof(cmsUInt16Number)
    } else {
        return output[(nChan+Extra)*2:]
    }
}
func PackHalfFromFloat(info *_cmsTRANSFORM, wOut []cmsFloat32Number, output []cmsUInt8Number, Stride cmsUInt32Number) []cmsUInt8Number {
    nChan := T_CHANNELS(info.OutputFormat)
    DoSwap := T_DOSWAP(info.OutputFormat)
    Reverse := T_FLAVOR(info.OutputFormat)
    Extra := T_EXTRA(info.OutputFormat)
    SwapFirst := T_SWAPFIRST(info.OutputFormat)
    Planar := T_PLANAR(info.OutputFormat)
    ExtraFirst := DoSwap ^ SwapFirst
    var maximum cmsFloat32Number = 1.0
    if IsInkSpace(info.OutputFormat) {
        maximum = 100.0
    }
    outputHalf := (*[1 << 30]cmsUInt16Number)(unsafe.Pointer(&output[0]))[:len(output)/2]
    var v cmsFloat32Number
    var start cmsUInt32Number

    Stride /= PixelSize(info.OutputFormat)

    if ExtraFirst != 0 {
        start = Extra
    }

    for i := cmsUInt32Number(0); i < nChan; i++ {
        var index cmsUInt32Number
        if DoSwap != 0 {
            index = nChan - i - 1
        } else {
            index = i
        }

        v = wOut[index] * maximum

        if Reverse != 0 {
            v = maximum - v
        }

        if Planar != 0 {
            outputHalf[(i+start)*Stride] = _cmsFloat2Half(v)
        } else {
            outputHalf[i+start] = _cmsFloat2Half(v)
        }
    }

    if Extra == 0 && SwapFirst != 0 {
        copy(outputHalf[1:], outputHalf[:nChan-1])
        outputHalf[0] = _cmsFloat2Half(v)
    }

    if Planar != 0 {
        return output[2:] // sizeof(cmsUInt16Number)
    } else {
        return output[(nChan+Extra)*2:]
    }
}

// InputFormatters16 is the table of 16-bit input formatters
var InputFormatters16 = []cmsFormatters16{
	{Type: TYPE_Lab_DBL, Mask: ANYPLANAR | ANYEXTRA, Frm: UnrollLabDoubleTo16},
	{Type: TYPE_XYZ_DBL, Mask: ANYPLANAR | ANYEXTRA, Frm: UnrollXYZDoubleTo16},
	{Type: TYPE_Lab_FLT, Mask: ANYPLANAR | ANYEXTRA, Frm: UnrollLabFloatTo16},
	{Type: TYPE_XYZ_FLT, Mask: ANYPLANAR | ANYEXTRA, Frm: UnrollXYZFloatTo16},
	{Type: TYPE_GRAY_DBL, Mask: 0, Frm: UnrollDouble1Chan},
	{Type: FLOAT_SH(1) | BYTES_SH(0), Mask: ANYCHANNELS | ANYPLANAR | ANYSWAPFIRST | ANYFLAVOR | ANYSWAP | ANYEXTRA | ANYSPACE, Frm: UnrollDoubleTo16},
	{Type: FLOAT_SH(1) | BYTES_SH(4), Mask: ANYCHANNELS | ANYPLANAR | ANYSWAPFIRST | ANYFLAVOR | ANYSWAP | ANYEXTRA | ANYSPACE, Frm: UnrollFloatTo16},
	// Uncomment if half support is enabled
	{Type: FLOAT_SH(1) | BYTES_SH(2), Mask: ANYCHANNELS | ANYPLANAR | ANYSWAPFIRST | ANYFLAVOR | ANYEXTRA | ANYSWAP | ANYSPACE, Frm: UnrollHalfTo16},
	{Type: CHANNELS_SH(1) | BYTES_SH(1), Mask: ANYSPACE, Frm: Unroll1Byte},
	{Type: CHANNELS_SH(1) | BYTES_SH(1) | EXTRA_SH(1), Mask: ANYSPACE, Frm: Unroll1ByteSkip1},
	{Type: CHANNELS_SH(1) | BYTES_SH(1) | EXTRA_SH(2), Mask: ANYSPACE, Frm: Unroll1ByteSkip2},
	{Type: CHANNELS_SH(1) | BYTES_SH(1) | FLAVOR_SH(1), Mask: ANYSPACE, Frm: Unroll1ByteReversed},
	{Type: COLORSPACE_SH(PT_MCH2) | CHANNELS_SH(2) | BYTES_SH(1), Mask: 0, Frm: Unroll2Bytes},
	{Type: TYPE_LabV2_8, Mask: 0, Frm: UnrollLabV2_8},
	{Type: TYPE_ALabV2_8, Mask: 0, Frm: UnrollALabV2_8},
	{Type: TYPE_LabV2_16, Mask: 0, Frm: UnrollLabV2_16},
	{Type: CHANNELS_SH(3) | BYTES_SH(1), Mask: ANYSPACE, Frm: Unroll3Bytes},
	{Type: CHANNELS_SH(3) | BYTES_SH(1) | DOSWAP_SH(1), Mask: ANYSPACE, Frm: Unroll3BytesSwap},
	{Type: CHANNELS_SH(3) | EXTRA_SH(1) | BYTES_SH(1) | DOSWAP_SH(1), Mask: ANYSPACE, Frm: Unroll3BytesSkip1Swap},
	{Type: CHANNELS_SH(3) | EXTRA_SH(1) | BYTES_SH(1) | SWAPFIRST_SH(1), Mask: ANYSPACE, Frm: Unroll3BytesSkip1SwapFirst},
	{Type: CHANNELS_SH(3) | EXTRA_SH(1) | BYTES_SH(1) | DOSWAP_SH(1) | SWAPFIRST_SH(1), Mask: ANYSPACE, Frm: Unroll3BytesSkip1SwapSwapFirst},
	{Type: CHANNELS_SH(4) | BYTES_SH(1), Mask: ANYSPACE, Frm: Unroll4Bytes},
	{Type: CHANNELS_SH(4) | BYTES_SH(1) | FLAVOR_SH(1), Mask: ANYSPACE, Frm: Unroll4BytesReverse},
	{Type: CHANNELS_SH(4) | BYTES_SH(1) | SWAPFIRST_SH(1), Mask: ANYSPACE, Frm: Unroll4BytesSwapFirst},
	{Type: CHANNELS_SH(4) | BYTES_SH(1) | DOSWAP_SH(1), Mask: ANYSPACE, Frm: Unroll4BytesSwap},
	{Type: CHANNELS_SH(4) | BYTES_SH(1) | DOSWAP_SH(1) | SWAPFIRST_SH(1), Mask: ANYSPACE, Frm: Unroll4BytesSwapSwapFirst},
	{Type: BYTES_SH(1) | PLANAR_SH(1), Mask: ANYFLAVOR | ANYSWAPFIRST | ANYPREMUL | ANYSWAP | ANYEXTRA | ANYCHANNELS | ANYSPACE, Frm: UnrollPlanarBytes},
	{Type: BYTES_SH(1), Mask: ANYFLAVOR | ANYSWAPFIRST | ANYSWAP | ANYPREMUL | ANYEXTRA | ANYCHANNELS | ANYSPACE, Frm: UnrollChunkyBytes},
	{Type: CHANNELS_SH(1) | BYTES_SH(2), Mask: ANYSPACE, Frm: Unroll1Word},
	{Type: CHANNELS_SH(1) | BYTES_SH(2) | FLAVOR_SH(1), Mask: ANYSPACE, Frm: Unroll1WordReversed},
	{Type: CHANNELS_SH(1) | BYTES_SH(2) | EXTRA_SH(3), Mask: ANYSPACE, Frm: Unroll1WordSkip3},
	{Type: CHANNELS_SH(2) | BYTES_SH(2), Mask: ANYSPACE, Frm: Unroll2Words},
	{Type: CHANNELS_SH(3) | BYTES_SH(2), Mask: ANYSPACE, Frm: Unroll3Words},
	{Type: CHANNELS_SH(4) | BYTES_SH(2), Mask: ANYSPACE, Frm: Unroll4Words},
	{Type: CHANNELS_SH(3) | BYTES_SH(2) | DOSWAP_SH(1), Mask: ANYSPACE, Frm: Unroll3WordsSwap},
	{Type: CHANNELS_SH(3) | BYTES_SH(2) | EXTRA_SH(1) | SWAPFIRST_SH(1), Mask: ANYSPACE, Frm: Unroll3WordsSkip1SwapFirst},
	{Type: CHANNELS_SH(3) | BYTES_SH(2) | EXTRA_SH(1) | DOSWAP_SH(1), Mask: ANYSPACE, Frm: Unroll3WordsSkip1Swap},
	{Type: CHANNELS_SH(4) | BYTES_SH(2) | FLAVOR_SH(1), Mask: ANYSPACE, Frm: Unroll4WordsReverse},
	{Type: CHANNELS_SH(4) | BYTES_SH(2) | SWAPFIRST_SH(1), Mask: ANYSPACE, Frm: Unroll4WordsSwapFirst},
	{Type: CHANNELS_SH(4) | BYTES_SH(2) | DOSWAP_SH(1), Mask: ANYSPACE, Frm: Unroll4WordsSwap},
	{Type: CHANNELS_SH(4) | BYTES_SH(2) | DOSWAP_SH(1) | SWAPFIRST_SH(1), Mask: ANYSPACE, Frm: Unroll4WordsSwapSwapFirst},
	{Type: BYTES_SH(2) | PLANAR_SH(1), Mask: ANYFLAVOR | ANYSWAP | ANYENDIAN | ANYEXTRA | ANYCHANNELS | ANYSPACE, Frm: UnrollPlanarWords},
	{Type: BYTES_SH(2), Mask: ANYFLAVOR | ANYSWAPFIRST | ANYSWAP | ANYENDIAN | ANYEXTRA | ANYCHANNELS | ANYSPACE, Frm: UnrollAnyWords},
	{Type: BYTES_SH(2) | PLANAR_SH(1), Mask: ANYFLAVOR | ANYSWAP | ANYENDIAN | ANYEXTRA | ANYCHANNELS | ANYSPACE | PREMUL_SH(1), Frm: UnrollPlanarWordsPremul},
	{Type: BYTES_SH(2), Mask: ANYFLAVOR | ANYSWAPFIRST | ANYSWAP | ANYENDIAN | ANYEXTRA | ANYCHANNELS | ANYSPACE | PREMUL_SH(1), Frm: UnrollAnyWordsPremul},
}

// Input Formatters (Float)
var InputFormattersFloat = []cmsFormattersFloat{
	{Type: TYPE_Lab_DBL, Mask: ANYPLANAR | ANYEXTRA, Frm: UnrollLabDoubleToFloat},
	{Type: TYPE_Lab_FLT, Mask: ANYPLANAR | ANYEXTRA, Frm: UnrollLabFloatToFloat},
	{Type: TYPE_XYZ_DBL, Mask: ANYPLANAR | ANYEXTRA, Frm: UnrollXYZDoubleToFloat},
	{Type: TYPE_XYZ_FLT, Mask: ANYPLANAR | ANYEXTRA, Frm: UnrollXYZFloatToFloat},
	// More entries omitted for brevity
}

// Output Formatters (16-bit)
var OutputFormatters16 = []cmsFormatters16{
	{Type: TYPE_Lab_DBL, Mask: ANYPLANAR | ANYEXTRA, Frm: PackLabDoubleFrom16},
	{Type: TYPE_XYZ_DBL, Mask: ANYPLANAR | ANYEXTRA, Frm: PackXYZDoubleFrom16},
	// More entries omitted for brevity
}

// Output Formatters (Float)
var OutputFormattersFloat = []cmsFormattersFloat{
	{Type: TYPE_Lab_FLT, Mask: ANYPLANAR | ANYEXTRA, Frm: PackLabFloatFromFloat},
	{Type: TYPE_XYZ_FLT, Mask: ANYPLANAR | ANYEXTRA, Frm: PackXYZFloatFromFloat},
	// More entries omitted for brevity
}

// Structure for formatters factory list
type cmsFormattersFactoryList struct {
	Factory cmsFormatterFactory
	Next    *cmsFormattersFactoryList
}


// Function to duplicate the formatter factory list
func DupFormatterFactoryList(ctx *cmsContext, src *cmsContext) error {
	newHead := &cmsFormattersPluginChunkType{FactoryList: nil}
	head := src.chunks[FormattersPlugin].(*cmsFormattersPluginChunkType)
	if head == nil {
		return errors.New("source chunk is nil")
	}

	var prev *cmsFormattersFactoryList
	for entry := head.FactoryList; entry != nil; entry = entry.Next {
		newEntry := &cmsFormattersFactoryList{
			Factory: entry.Factory,
			Next:    nil,
		}
		if prev != nil {
			prev.Next = newEntry
		}
		prev = newEntry
		if newHead.FactoryList == nil {
			newHead.FactoryList = newEntry
		}
	}

	ctx.chunks[FormattersPlugin] = newHead
	return nil
}

// Allocate formatters plugin chunk
func cmsAllocFormattersPluginChunk(ctx *cmsContext, src *cmsContext) {
	if src != nil {
		_ = DupFormatterFactoryList(ctx, src)
	} else {
		ctx.chunks[FormattersPlugin] = &cmsFormattersPluginChunkType{FactoryList: nil}
	}
}

// Register formatters plugin
func cmsRegisterFormattersPlugin(ctx *cmsContext, data *cmsPluginBase) bool {
	pluginChunk := ctx.chunks[FormattersPlugin].(*cmsFormattersPluginChunkType)
	plugin := (*cmsPluginFormatters)(unsafe.Pointer(data))

	if data == nil {
		pluginChunk.FactoryList = nil
		return true
	}

	newEntry := &cmsFormattersFactoryList{
		Factory: plugin.FormattersFactory,
		Next:    pluginChunk.FactoryList,
	}
	pluginChunk.FactoryList = newEntry
	return true
}

// Get formatter
func cmsGetFormatter(ctx *cmsContext, formatType uint32, dir cmsFormatterDirection, dwFlags uint32) cmsFormatter {
	pluginChunk := ctx.chunks[FormattersPlugin].(*cmsFormattersPluginChunkType)
	for f := pluginChunk.FactoryList; f != nil; f = f.Next {
		formatter := f.Factory(formatType, dir, dwFlags)
		if formatter.Fmt16 != nil || formatter.FmtFloat != nil {
			return formatter
		}
	}
	if dir == cmsFormatterInput {
		return cmsGetStockInputFormatter(formatType, dwFlags)
	}
	return cmsGetStockOutputFormatter(formatType, dwFlags)
}

// Additional utility functions
func cmsFormatterIsFloat(formatType uint32) bool {
	return T_FLOAT(formatType) != 0
}

func cmsFormatterIs8bit(formatType uint32) bool {
	return T_BYTES(formatType) == 1
}

func cmsFormatterForColorspaceOfProfile(hProfile cmsHPROFILE, nBytes uint32, isFloat bool) uint32 {
	colorSpace := cmsGetColorSpace(hProfile)
	colorSpaceBits := cmsLCMScolorSpace(colorSpace)
	nOutputChans := cmsChannelsOfColorSpace(colorSpace)
	if nOutputChans < 0 {
		return 0
	}
	floatFlag := uint32(0)
	if isFloat {
		floatFlag = 1
	}
	return FLOAT_SH(floatFlag) | COLORSPACE_SH(uint32(colorSpaceBits)) | BYTES_SH(nBytes) | CHANNELS_SH(uint32(nOutputChans))
}

func cmsFormatterForPCSOfProfile(hProfile cmsHPROFILE, nBytes uint32, isFloat bool) uint32 {
	colorSpace := cmsGetPCS(hProfile)
	colorSpaceBits := cmsLCMScolorSpace(colorSpace)
	nOutputChans := cmsChannelsOf(colorSpace)
	if nOutputChans < 0 {
		return 0
	}
	floatFlag := uint32(0)
	if isFloat {
		floatFlag = 1
	}
	return FLOAT_SH(floatFlag) | COLORSPACE_SH(uint32(colorSpaceBits)) | BYTES_SH(nBytes) | CHANNELS_SH(uint32(nOutputChans))
}
