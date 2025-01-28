package golcms

import (
	"encoding/binary"
	//"errors"
	"math"
	"unsafe"
)

//FIRST HALF OF THE FILE SKIPPED YET
// Constants for formatter flags

// Change endianness of a 16-bit word
func CHANGE_ENDIAN(w uint16) uint16 {
	return (w << 8) | (w >> 8)
}

// Reverse the flavor for 8-bit values
func REVERSE_FLAVOR_8(x uint8) uint8 {
	return uint8(0xFF - x)
}

// Reverse the flavor for 16-bit values
func REVERSE_FLAVOR_16(x uint16) uint16 {
	return 0xFFFF - x
}

// Convert LabV2 to LabV4
func FromLabV2ToLabV4(x uint16) uint16 {
	a := (int(x)<<8 | int(x)) >> 8 // Multiply by 257 / 256
	if a > math.MaxUint16 {
		return math.MaxUint16
	}
	return uint16(a)
}

// Convert LabV4 to LabV2
func FromLabV4ToLabV2(x uint16) uint16 {
	return uint16(((int(x) << 8) + 0x80) / 257) // Multiply by 256 / 257
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
func UnrollChunkyBytes(info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	nChan := T_CHANNELS(info.InputFormat)
	doSwap := T_DOSWAP(info.InputFormat)
	reverse := T_FLAVOR(info.InputFormat)
	swapFirst := T_SWAPFIRST(info.InputFormat)
	extra := T_EXTRA(info.InputFormat)
	premul := T_PREMUL(info.InputFormat)

	extraFirst := doSwap ^ swapFirst
	alphaFactor := uint32(1)

	if extraFirst != 0 {
		if premul != 0 && extra != 0 {
			alphaFactor = uint32(cmsToFixedDomain(int(FROM_8_TO_16(accum[0]))))
		}
		accum = accum[extra:]
	} else {
		if premul != 0 && extra != 0 {
			alphaFactor = uint32(cmsToFixedDomain(int(FROM_8_TO_16(accum[nChan]))))
		}
	}

	for i := uint32(0); i < nChan; i++ {
		var index uint32
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
			v = uint16(((uint32(v) << 16) / alphaFactor))
			if v > 0xffff {
				v = 0xffff
			}
		}

		wIn[index] = uint16(v)
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
func UnrollPlanarBytes(info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	nChan := T_CHANNELS(info.InputFormat)
	doSwap := T_DOSWAP(info.InputFormat)
	swapFirst := T_SWAPFIRST(info.InputFormat)
	reverse := T_FLAVOR(info.InputFormat)
	extraFirst := doSwap ^ swapFirst
	extra := T_EXTRA(info.InputFormat)
	premul := T_PREMUL(info.InputFormat)
	init := accum
	alphaFactor := uint32(1)

	if extraFirst != 0 {
		if premul != 0 && extra != 0 {
			alphaFactor = uint32(cmsToFixedDomain(int(FROM_8_TO_16(accum[0]))))
		}
		accum = accum[int(uint32(extra)*stride):]
	} else {
		if premul != 0 && extra != 0 {
			alphaFactor = uint32(cmsToFixedDomain(int(FROM_8_TO_16(accum[nChan*stride]))))
		}
	}

	for i := uint32(0); i < nChan; i++ {
		var index uint32
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
			v = (uint16)((uint32(v) << 16) / alphaFactor)
			if v > 0xffff {
				v = 0xffff
			}
		}

		wIn[index] = uint16(v)
		accum = accum[stride:]
	}

	return init[1:]
}

// Unroll4Bytes processes 4 bytes in sequence.
func Unroll4Bytes(info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	wIn[0] = FROM_8_TO_16(accum[0]) // C
	wIn[1] = FROM_8_TO_16(accum[1]) // M
	wIn[2] = FROM_8_TO_16(accum[2]) // Y
	wIn[3] = FROM_8_TO_16(accum[3]) // K
	return accum[4:]
}

// Unroll4BytesReverse processes 4 bytes with reverse flavor applied.
func Unroll4BytesReverse(info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	wIn[0] = FROM_8_TO_16(REVERSE_FLAVOR_8(accum[0])) // C
	wIn[1] = FROM_8_TO_16(REVERSE_FLAVOR_8(accum[1])) // M
	wIn[2] = FROM_8_TO_16(REVERSE_FLAVOR_8(accum[2])) // Y
	wIn[3] = FROM_8_TO_16(REVERSE_FLAVOR_8(accum[3])) // K
	return accum[4:]
}

// Unroll4BytesSwapFirst processes 4 bytes with the first byte swapped to the last position.
func Unroll4BytesSwapFirst(info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	wIn[3] = FROM_8_TO_16(accum[0]) // K
	wIn[0] = FROM_8_TO_16(accum[1]) // C
	wIn[1] = FROM_8_TO_16(accum[2]) // M
	wIn[2] = FROM_8_TO_16(accum[3]) // Y
	return accum[4:]
}

// Unroll4BytesSwap processes 4 bytes in KYMC order.
func Unroll4BytesSwap(info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	wIn[3] = FROM_8_TO_16(accum[0]) // K
	wIn[2] = FROM_8_TO_16(accum[1]) // Y
	wIn[1] = FROM_8_TO_16(accum[2]) // M
	wIn[0] = FROM_8_TO_16(accum[3]) // C
	return accum[4:]
}

// Unroll4BytesSwapSwapFirst processes 4 bytes in swapped order with the first byte swapped.
func Unroll4BytesSwapSwapFirst(info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	wIn[2] = FROM_8_TO_16(accum[0]) // K
	wIn[1] = FROM_8_TO_16(accum[1]) // Y
	wIn[0] = FROM_8_TO_16(accum[2]) // M
	wIn[3] = FROM_8_TO_16(accum[3]) // C
	return accum[4:]
}

// Unroll3Bytes processes 3 bytes in RGB order.
func Unroll3Bytes(info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	wIn[0] = FROM_8_TO_16(accum[0]) // R
	wIn[1] = FROM_8_TO_16(accum[1]) // G
	wIn[2] = FROM_8_TO_16(accum[2]) // B
	return accum[3:]
}

// Unroll3BytesSkip1Swap processes 3 bytes, skips 1 (A), and swaps to BRG order.
func Unroll3BytesSkip1Swap(info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	accum = accum[1:]               // Skip A
	wIn[2] = FROM_8_TO_16(accum[0]) // B
	wIn[1] = FROM_8_TO_16(accum[1]) // G
	wIn[0] = FROM_8_TO_16(accum[2]) // R
	return accum[3:]
}

// Unroll3BytesSkip1SwapSwapFirst processes 3 bytes, skips 1 (A), and swaps to BRG order with the first byte swapped.
func Unroll3BytesSkip1SwapSwapFirst(info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	wIn[2] = FROM_8_TO_16(accum[0]) // B
	wIn[1] = FROM_8_TO_16(accum[1]) // G
	wIn[0] = FROM_8_TO_16(accum[2]) // R
	accum = accum[4:]               // Skip 1 (A) and move forward
	return accum
}

// Unroll3BytesSkip1SwapFirst processes 3 bytes, skips 1 (A), and places R first.
func Unroll3BytesSkip1SwapFirst(info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	accum = accum[1:]               // Skip A
	wIn[0] = FROM_8_TO_16(accum[0]) // R
	wIn[1] = FROM_8_TO_16(accum[1]) // G
	wIn[2] = FROM_8_TO_16(accum[2]) // B
	return accum[3:]
}

// Unroll3BytesSwap processes 3 bytes in BRG order.
func Unroll3BytesSwap(info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	wIn[2] = FROM_8_TO_16(accum[0]) // B
	wIn[1] = FROM_8_TO_16(accum[1]) // G
	wIn[0] = FROM_8_TO_16(accum[2]) // R
	return accum[3:]
}

// UnrollLabV2_8 processes Lab values from 8-bit input and converts them to 16-bit.
func UnrollLabV2_8(info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	wIn[0] = FromLabV2ToLabV4(FROM_8_TO_16(accum[0])) // L
	wIn[1] = FromLabV2ToLabV4(FROM_8_TO_16(accum[1])) // a
	wIn[2] = FromLabV2ToLabV4(FROM_8_TO_16(accum[2])) // b
	return accum[3:]
}

// UnrollALabV2_8 processes ALab values from 8-bit input, skipping the alpha channel.
func UnrollALabV2_8(info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	accum = accum[1:]                                 // Skip alpha
	wIn[0] = FromLabV2ToLabV4(FROM_8_TO_16(accum[0])) // L
	wIn[1] = FromLabV2ToLabV4(FROM_8_TO_16(accum[1])) // a
	wIn[2] = FromLabV2ToLabV4(FROM_8_TO_16(accum[2])) // b
	return accum[3:]
}

// UnrollLabV2_16 processes Lab values from 16-bit input.
func UnrollLabV2_16(info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	wIn[0] = FromLabV2ToLabV4(uint16(accum[0]) | uint16(accum[1])<<8) // L
	accum = accum[2:]
	wIn[1] = FromLabV2ToLabV4(uint16(accum[0]) | uint16(accum[1])<<8) // a
	accum = accum[2:]
	wIn[2] = FromLabV2ToLabV4(uint16(accum[0]) | uint16(accum[1])<<8) // b
	return accum[2:]
}

// Unroll2Bytes processes duplex values from 8-bit input.
func Unroll2Bytes(info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	wIn[0] = FROM_8_TO_16(accum[0]) // ch1
	wIn[1] = FROM_8_TO_16(accum[1]) // ch2
	return accum[2:]
}

// Unroll1Byte duplicates L into RGB channels for monochrome data.
func Unroll1Byte(info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	l := FROM_8_TO_16(accum[0])
	wIn[0], wIn[1], wIn[2] = l, l, l // L
	return accum[1:]
}

// Unroll1ByteSkip1 processes monochrome data, skipping one channel.
func Unroll1ByteSkip1(info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	l := FROM_8_TO_16(accum[0])
	wIn[0], wIn[1], wIn[2] = l, l, l // L
	return accum[2:]
}

// Unroll1ByteSkip2 processes monochrome data, skipping two channels.
func Unroll1ByteSkip2(info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	l := FROM_8_TO_16(accum[0])
	wIn[0], wIn[1], wIn[2] = l, l, l // L
	return accum[3:]
}

// Unroll1ByteReversed processes monochrome data with reversed flavor.
func Unroll1ByteReversed(info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	l := REVERSE_FLAVOR_16(FROM_8_TO_16(accum[0]))
	wIn[0], wIn[1], wIn[2] = l, l, l // L
	return accum[1:]
}

func UnrollAnyWords(info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	nChan := T_CHANNELS(info.InputFormat)
	swapEndian := T_ENDIAN16(info.InputFormat)
	doSwap := T_DOSWAP(info.InputFormat)
	reverse := T_FLAVOR(info.InputFormat)
	swapFirst := T_SWAPFIRST(info.InputFormat)
	extra := T_EXTRA(info.InputFormat)
	extraFirst := doSwap ^ swapFirst

	if extraFirst != 0 {
		accum = accum[extra*2:]
	}

	for i := uint32(0); i < nChan; i++ {
		index := i
		if doSwap != 0 {
			index = nChan - i - 1
		}

		v := uint16(accum[0]) | (uint16(accum[1]) << 8)

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

func UnrollAnyWordsPremul(info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	nChan := T_CHANNELS(info.InputFormat)
	swapEndian := T_ENDIAN16(info.InputFormat)
	doSwap := T_DOSWAP(info.InputFormat)
	reverse := T_FLAVOR(info.InputFormat)
	swapFirst := T_SWAPFIRST(info.InputFormat)
	extraFirst := doSwap ^ swapFirst

	var alpha uint16
	if extraFirst != 0 {
		alpha = uint16(accum[0]) | (uint16(accum[1]) << 8)
		accum = accum[2:]
	} else {
		alpha = uint16(accum[(nChan-1)*2]) | (uint16(accum[(nChan-1)*2+1]) << 8)
	}

	alphaFactor := cmsToFixedDomain(int(alpha))

	for i := uint32(0); i < nChan; i++ {
		index := i
		if doSwap != 0 {
			index = nChan - i - 1
		}

		v := uint16(accum[0]) | (uint16(accum[1]) << 8)

		if swapEndian != 0 {
			v = CHANGE_ENDIAN(v)
		}

		if alphaFactor > 0 {
			v = uint16((uint32(v) << 16) / (uint32(alphaFactor)))
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

func UnrollPlanarWords(info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	nChan := T_CHANNELS(info.InputFormat)
	doSwap := T_DOSWAP(info.InputFormat)
	reverse := T_FLAVOR(info.InputFormat)
	swapEndian := T_ENDIAN16(info.InputFormat)

	if doSwap != 0 {
		accum = accum[T_EXTRA(info.InputFormat)*stride:]
	}

	for i := uint32(0); i < nChan; i++ {
		index := i
		if doSwap != 0 {
			index = nChan - i - 1
		}

		v := uint16(accum[0]) | (uint16(accum[1]) << 8)

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

func UnrollPlanarWordsPremul(info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	nChan := T_CHANNELS(info.InputFormat)
	doSwap := T_DOSWAP(info.InputFormat)
	swapFirst := T_SWAPFIRST(info.InputFormat)
	reverse := T_FLAVOR(info.InputFormat)
	swapEndian := T_ENDIAN16(info.InputFormat)
	extraFirst := doSwap ^ swapFirst

	var alpha uint16
	if extraFirst != 0 {
		alpha = uint16(accum[0]) | (uint16(accum[1]) << 8)
		accum = accum[int(stride):]
	} else {
		alpha = uint16(accum[(nChan-1)*stride]) | (uint16(accum[(nChan-1)*stride+1]) << 8)
	}

	alphaFactor := uint32(cmsToFixedDomain(int(alpha)))

	for i := uint32(0); i < nChan; i++ {
		index := i
		if doSwap != 0 {
			index = nChan - i - 1
		}

		v := uint16(accum[0]) | (uint16(accum[1]) << 8)

		if swapEndian != 0 {
			v = CHANGE_ENDIAN(v)
		}

		if alphaFactor > 0 {
			v = uint16((uint32(v) << 16) / uint32(alphaFactor))
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
func Unroll4Words(info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	wIn[0] = uint16(accum[0]) | (uint16(accum[1]) << 8) // C
	accum = accum[2:]
	wIn[1] = uint16(accum[0]) | (uint16(accum[1]) << 8) // M
	accum = accum[2:]
	wIn[2] = uint16(accum[0]) | (uint16(accum[1]) << 8) // Y
	accum = accum[2:]
	wIn[3] = uint16(accum[0]) | (uint16(accum[1]) << 8) // K
	accum = accum[2:]
	return accum
}

func Unroll4WordsReverse(info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	wIn[0] = REVERSE_FLAVOR_16(uint16(accum[0]) | (uint16(accum[1]) << 8)) // C
	accum = accum[2:]
	wIn[1] = REVERSE_FLAVOR_16(uint16(accum[0]) | (uint16(accum[1]) << 8)) // M
	accum = accum[2:]
	wIn[2] = REVERSE_FLAVOR_16(uint16(accum[0]) | (uint16(accum[1]) << 8)) // Y
	accum = accum[2:]
	wIn[3] = REVERSE_FLAVOR_16(uint16(accum[0]) | (uint16(accum[1]) << 8)) // K
	accum = accum[2:]
	return accum
}

func Unroll4WordsSwapFirst(info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	wIn[3] = uint16(accum[0]) | (uint16(accum[1]) << 8) // K
	accum = accum[2:]
	wIn[0] = uint16(accum[0]) | (uint16(accum[1]) << 8) // C
	accum = accum[2:]
	wIn[1] = uint16(accum[0]) | (uint16(accum[1]) << 8) // M
	accum = accum[2:]
	wIn[2] = uint16(accum[0]) | (uint16(accum[1]) << 8) // Y
	accum = accum[2:]
	return accum
}

func Unroll4WordsSwap(info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	wIn[3] = uint16(accum[0]) | (uint16(accum[1]) << 8) // K
	accum = accum[2:]
	wIn[2] = uint16(accum[0]) | (uint16(accum[1]) << 8) // Y
	accum = accum[2:]
	wIn[1] = uint16(accum[0]) | (uint16(accum[1]) << 8) // M
	accum = accum[2:]
	wIn[0] = uint16(accum[0]) | (uint16(accum[1]) << 8) // C
	accum = accum[2:]
	return accum
}

func Unroll4WordsSwapSwapFirst(info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	wIn[2] = uint16(accum[0]) | (uint16(accum[1]) << 8) // K
	accum = accum[2:]
	wIn[1] = uint16(accum[0]) | (uint16(accum[1]) << 8) // Y
	accum = accum[2:]
	wIn[0] = uint16(accum[0]) | (uint16(accum[1]) << 8) // M
	accum = accum[2:]
	wIn[3] = uint16(accum[0]) | (uint16(accum[1]) << 8) // C
	accum = accum[2:]
	return accum
}
func Unroll3Words(info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	wIn[0] = uint16(accum[0]) | (uint16(accum[1]) << 8) // C R
	accum = accum[2:]
	wIn[1] = uint16(accum[0]) | (uint16(accum[1]) << 8) // M G
	accum = accum[2:]
	wIn[2] = uint16(accum[0]) | (uint16(accum[1]) << 8) // Y B
	accum = accum[2:]
	return accum
}

func Unroll3WordsSwap(info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	wIn[2] = uint16(accum[0]) | (uint16(accum[1]) << 8) // C R
	accum = accum[2:]
	wIn[1] = uint16(accum[0]) | (uint16(accum[1]) << 8) // M G
	accum = accum[2:]
	wIn[0] = uint16(accum[0]) | (uint16(accum[1]) << 8) // Y B
	accum = accum[2:]
	return accum
}

func Unroll3WordsSkip1Swap(info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	accum = accum[2:]                                   // Skip A
	wIn[2] = uint16(accum[0]) | (uint16(accum[1]) << 8) // R
	accum = accum[2:]
	wIn[1] = uint16(accum[0]) | (uint16(accum[1]) << 8) // G
	accum = accum[2:]
	wIn[0] = uint16(accum[0]) | (uint16(accum[1]) << 8) // B
	accum = accum[2:]
	return accum
}

func Unroll3WordsSkip1SwapFirst(info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	accum = accum[2:]                                   // Skip A
	wIn[0] = uint16(accum[0]) | (uint16(accum[1]) << 8) // R
	accum = accum[2:]
	wIn[1] = uint16(accum[0]) | (uint16(accum[1]) << 8) // G
	accum = accum[2:]
	wIn[2] = uint16(accum[0]) | (uint16(accum[1]) << 8) // B
	accum = accum[2:]
	return accum
}

func Unroll1Word(info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	word := uint16(accum[0]) | (uint16(accum[1]) << 8)
	wIn[0], wIn[1], wIn[2] = word, word, word // L duplicated to RGB
	accum = accum[2:]
	return accum
}

func Unroll1WordReversed(info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	word := REVERSE_FLAVOR_16(uint16(accum[0]) | (uint16(accum[1]) << 8))
	wIn[0], wIn[1], wIn[2] = word, word, word // L reversed and duplicated to RGB
	accum = accum[2:]
	return accum
}

func Unroll1WordSkip3(info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	word := uint16(accum[0]) | (uint16(accum[1]) << 8)
	wIn[0], wIn[1], wIn[2] = word, word, word // L duplicated to RGB
	accum = accum[8:]                         // Skip 3 words
	return accum
}
func Unroll2Words(info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	wIn[0] = uint16(accum[0]) | (uint16(accum[1]) << 8) // ch1
	accum = accum[2:]
	wIn[1] = uint16(accum[0]) | (uint16(accum[1]) << 8) // ch2
	accum = accum[2:]
	return accum
}
func UnrollLabDoubleTo16(info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	if T_PLANAR(info.InputFormat) != 0 {
		var Lab cmsCIELab

		posL := accum
		posa := accum[stride:]
		posb := accum[stride*2:]

		Lab.L = *(*float64)(unsafe.Pointer(&posL[0]))
		Lab.a = *(*float64)(unsafe.Pointer(&posa[0]))
		Lab.b = *(*float64)(unsafe.Pointer(&posb[0]))

		cmsFloat2LabEncoded(*(*[3]uint16)(wIn), &Lab)
		return accum[8:] // sizeof(float64)
	} else {
		cmsFloat2LabEncoded(*(*[3]uint16)(wIn), (*cmsCIELab)(unsafe.Pointer(&accum[0])))
		extra := T_EXTRA(info.InputFormat)
		accum = accum[(uint32(unsafe.Sizeof(cmsCIELab{})) + extra*8):] // sizeof(float64)
		return accum
	}
}
func UnrollLabFloatTo16(info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	var Lab cmsCIELab

	if T_PLANAR(info.InputFormat) != 0 {
		posL := accum
		posa := accum[stride:]
		posb := accum[stride*2:]

		Lab.L = *(*float64)(unsafe.Pointer(&posL[0]))
		Lab.a = *(*float64)(unsafe.Pointer(&posa[0]))
		Lab.b = *(*float64)(unsafe.Pointer(&posb[0]))

		cmsFloat2LabEncoded(*(*[3]uint16)(wIn), &Lab)
		return accum[4:] // sizeof(float32)
	} else {
		Lab.L = *(*float64)(unsafe.Pointer(&accum[0]))
		Lab.a = *(*float64)(unsafe.Pointer(&accum[4]))
		Lab.b = *(*float64)(unsafe.Pointer(&accum[8]))

		cmsFloat2LabEncoded(*(*[3]uint16)(wIn), &Lab)
		extra := T_EXTRA(info.InputFormat)
		accum = accum[(3+extra)*4:] // 3 components + extra
		return accum
	}
}
func UnrollXYZDoubleTo16(info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	if T_PLANAR(info.InputFormat) != 0 {
		var XYZ cmsCIEXYZ

		posX := accum
		posY := accum[stride:]
		posZ := accum[stride*2:]

		XYZ.X = *(*float64)(unsafe.Pointer(&posX[0]))
		XYZ.Y = *(*float64)(unsafe.Pointer(&posY[0]))
		XYZ.Z = *(*float64)(unsafe.Pointer(&posZ[0]))

		cmsFloat2XYZEncoded(*(*[3]uint16)(wIn), &XYZ)

		return accum[8:] // sizeof(float64)
	} else {
		cmsFloat2XYZEncoded(*(*[3]uint16)(wIn), (*cmsCIEXYZ)(unsafe.Pointer(&accum[0])))
		extra := T_EXTRA(info.InputFormat)
		accum = accum[(8*3 + extra*8):] // sizeof(cmsCIEXYZ) + T_EXTRA * sizeof(float64)
		return accum
	}
}
func UnrollXYZFloatTo16(info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	if T_PLANAR(info.InputFormat) != 0 {
		var XYZ cmsCIEXYZ

		posX := accum
		posY := accum[stride:]
		posZ := accum[stride*2:]

		XYZ.X = *(*float64)(unsafe.Pointer(&posX[0]))
		XYZ.Y = *(*float64)(unsafe.Pointer(&posY[0]))
		XYZ.Z = *(*float64)(unsafe.Pointer(&posZ[0]))

		cmsFloat2XYZEncoded(*(*[3]uint16)(wIn), &XYZ)

		return accum[4:] // sizeof(float32)
	} else {
		Pt := (*[3]float32)(unsafe.Pointer(&accum[0]))
		var XYZ cmsCIEXYZ

		XYZ.X = float64(Pt[0])
		XYZ.Y = float64(Pt[1])
		XYZ.Z = float64(Pt[2])

		cmsFloat2XYZEncoded(*(*[3]uint16)(wIn), &XYZ)

		extra := T_EXTRA(info.InputFormat)
		accum = accum[(4*3 + extra*4):] // 3 * sizeof(float32) + T_EXTRA * sizeof(float32)
		return accum
	}
}
func IsInkSpace(Type uint32) bool {
	switch T_COLORSPACE(Type) {
	case PT_CMY, PT_CMYK, PT_MCH5, PT_MCH6, PT_MCH7, PT_MCH8, PT_MCH9, PT_MCH10,
		PT_MCH11, PT_MCH12, PT_MCH13, PT_MCH14, PT_MCH15:
		return true
	default:
		return false
	}
}

func UnrollDoubleTo16(info *cmsTRANSFORM, wIn []uint16, accum []uint8, Stride uint32) []uint8 {
	nChan := T_CHANNELS(info.InputFormat)
	DoSwap := T_DOSWAP(info.InputFormat)
	Reverse := T_FLAVOR(info.InputFormat)
	SwapFirst := T_SWAPFIRST(info.InputFormat)
	Extra := T_EXTRA(info.InputFormat)
	ExtraFirst := DoSwap ^ SwapFirst
	Planar := T_PLANAR(info.InputFormat)
	var start uint32
	maximum := func() float64 {
		if IsInkSpace(info.InputFormat) {
			return 655.35
		}
		return 65535.0
	}()
	Stride /= PixelSize(info.InputFormat)

	if ExtraFirst != 0 {
		start = uint32(Extra)
	}

	for i := uint32(0); i < nChan; i++ {
		index := i
		if DoSwap != 0 {
			index = nChan - i - 1
		}

		var v float64
		if Planar != 0 {
			v = float64(accum[(i+start)*Stride])
		} else {
			v = float64(accum[i+start])
		}

		vi := cmsQuickSaturateWord(v * maximum)
		if Reverse != 0 {
			vi = REVERSE_FLAVOR_16(vi)
		}

		wIn[index] = vi
	}

	if Extra == 0 && SwapFirst != 0 {
		tmp := wIn[0]
		copy(wIn, wIn[1:nChan])
		wIn[nChan-1] = tmp
	}

	if Planar != 0 {
		return accum[uint32(len(wIn)):]
	}

	return accum[(nChan+Extra)*8:]
}
func UnrollFloatTo16(info *cmsTRANSFORM, wIn []uint16, accum []uint8, Stride uint32) []uint8 {
	nChan := T_CHANNELS(info.InputFormat)
	DoSwap := T_DOSWAP(info.InputFormat)
	Reverse := T_FLAVOR(info.InputFormat)
	SwapFirst := T_SWAPFIRST(info.InputFormat)
	Extra := T_EXTRA(info.InputFormat)
	ExtraFirst := DoSwap ^ SwapFirst
	Planar := T_PLANAR(info.InputFormat)
	var start uint32
	maximum := func() float64 {
		if IsInkSpace(info.InputFormat) {
			return 655.35
		}
		return 65535.0
	}()
	Stride /= PixelSize(info.InputFormat)

	if ExtraFirst != 0 {
		start = uint32(Extra)
	}

	for i := uint32(0); i < nChan; i++ {
		index := i
		if DoSwap != 0 {
			index = nChan - i - 1
		}

		var v float32
		if Planar != 0 {
			v = float32(accum[(i+start)*Stride])
		} else {
			v = float32(accum[i+start])
		}

		vi := cmsQuickSaturateWord(float64(v) * maximum)
		if Reverse != 0 {
			vi = REVERSE_FLAVOR_16(vi)
		}

		wIn[index] = vi
	}

	if Extra == 0 && SwapFirst != 0 {
		tmp := wIn[0]
		copy(wIn, wIn[1:nChan])
		wIn[nChan-1] = tmp
	}

	if Planar != 0 {
		return accum[uint32(len(wIn)):]
	}

	return accum[(nChan+Extra)*4:]
}
func UnrollDouble1Chan(info *cmsTRANSFORM, wIn []uint16, accum []uint8, Stride uint32) []uint8 {
	Inks := (*[1]float64)(unsafe.Pointer(&accum[0]))

	wIn[0] = cmsQuickSaturateWord(Inks[0] * 65535.0)
	wIn[1] = wIn[0]
	wIn[2] = wIn[0]

	return accum[8:]
}
func ReverseFloat(value float32, reverse bool) float32 {
	if reverse {
		return 1 - value
	}
	return value
}

func SwapFirstFloat(wIn []float32, nChan uint32) {
	tmp := wIn[0]
	copy(wIn[:nChan-1], wIn[1:nChan])
	wIn[nChan-1] = tmp
}

func Unroll8ToFloat(info *cmsTRANSFORM, wIn []float32, accum []uint8, Stride uint32) []uint8 {
	nChan := T_CHANNELS(info.InputFormat)
	DoSwap := T_DOSWAP(info.InputFormat)
	Reverse := T_FLAVOR(info.InputFormat)
	SwapFirst := T_SWAPFIRST(info.InputFormat)
	Extra := T_EXTRA(info.InputFormat)
	ExtraFirst := DoSwap ^ SwapFirst
	Planar := T_PLANAR(info.InputFormat)
	var start uint32

	Stride /= PixelSize(info.InputFormat)

	if ExtraFirst != 0 {
		start = uint32(Extra)
	}

	for i := uint32(0); i < nChan; i++ {
		index := i
		if DoSwap != 0 {
			index = nChan - i - 1
		}

		var v float32
		if Planar != 0 {
			v = float32(accum[(i+start)*Stride])
		} else {
			v = float32(accum[i+start])
		}

		v /= 255.0
		wIn[index] = ReverseFloat(v, Reverse != 0)
	}

	if Extra == 0 && SwapFirst != 0 {
		SwapFirstFloat(wIn, nChan)
	}

	if Planar != 0 {
		return accum[1:]
	}

	return accum[(nChan + Extra):]
}
func Unroll16ToFloat(info *cmsTRANSFORM, wIn []float32, accum []uint8, Stride uint32) []uint8 {
	nChan := T_CHANNELS(info.InputFormat)
	DoSwap := T_DOSWAP(info.InputFormat)
	Reverse := T_FLAVOR(info.InputFormat)
	SwapFirst := T_SWAPFIRST(info.InputFormat)
	Extra := T_EXTRA(info.InputFormat)
	ExtraFirst := DoSwap ^ SwapFirst
	Planar := T_PLANAR(info.InputFormat)
	var start uint32

	Stride /= PixelSize(info.InputFormat)

	if ExtraFirst != 0 {
		start = uint32(Extra)
	}

	for i := uint32(0); i < nChan; i++ {
		index := i
		if DoSwap != 0 {
			index = nChan - i - 1
		}

		var v float32
		if Planar != 0 {
			v = float32(binary.LittleEndian.Uint16(accum[(i+start)*Stride:]))
		} else {
			v = float32(binary.LittleEndian.Uint16(accum[(i+start)*2:]))
		}

		v /= 65535.0
		wIn[index] = ReverseFloat(v, Reverse != 0)
	}

	if Extra == 0 && SwapFirst != 0 {
		SwapFirstFloat(wIn, nChan)
	}

	if Planar != 0 {
		return accum[2:]
	}

	return accum[(nChan+Extra)*2:]
}
func UnrollFloatsToFloat(info *cmsTRANSFORM, wIn []float32, accum []uint8, Stride uint32) []uint8 {
	nChan := T_CHANNELS(info.InputFormat)
	DoSwap := T_DOSWAP(info.InputFormat)
	Reverse := T_FLAVOR(info.InputFormat)
	SwapFirst := T_SWAPFIRST(info.InputFormat)
	Extra := T_EXTRA(info.InputFormat)
	ExtraFirst := DoSwap ^ SwapFirst
	Planar := T_PLANAR(info.InputFormat)
	Premul := T_PREMUL(uint32(info.InputFormat))
	maximum := float32(1.0)
	if IsInkSpace(info.InputFormat) {
		maximum = 100.0
	}
	var alphaFactor float32 = 1.0
	var start uint32

	Stride /= PixelSize(info.InputFormat)

	if Premul != 0 && Extra > 0 {
		if Planar != 0 {
			if ExtraFirst != 0 {
				alphaFactor = float32(binary.LittleEndian.Uint32(accum[:4])) / maximum
			} else {
				alphaFactor = float32(binary.LittleEndian.Uint32(accum[nChan*Stride:])) / maximum
			}
		} else {
			if ExtraFirst != 0 {
				alphaFactor = float32(binary.LittleEndian.Uint32(accum[:4])) / maximum
			} else {
				alphaFactor = float32(binary.LittleEndian.Uint32(accum[nChan*4:])) / maximum
			}
		}
	}

	if ExtraFirst != 0 {
		start = uint32(Extra)
	}

	for i := uint32(0); i < nChan; i++ {
		index := i
		if DoSwap != 0 {
			index = nChan - i - 1
		}

		var v float32
		if Planar != 0 {
			v = float32(binary.LittleEndian.Uint32(accum[(i+start)*Stride:]))
		} else {
			v = float32(binary.LittleEndian.Uint32(accum[(i+start)*4:]))
		}

		if Premul != 0 && alphaFactor > 0 {
			v /= alphaFactor
		}

		v /= maximum
		wIn[index] = ReverseFloat(v, Reverse != 0)
	}

	if Extra == 0 && SwapFirst != 0 {
		SwapFirstFloat(wIn, nChan)
	}

	if Planar != 0 {
		return accum[4:]
	}

	return accum[(nChan+Extra)*4:]
}
func UnrollDoublesToFloat(info *cmsTRANSFORM, wIn []float32, accum []uint8, Stride uint32) []uint8 {
	nChan := T_CHANNELS(info.InputFormat)
	DoSwap := T_DOSWAP(info.InputFormat)
	Reverse := T_FLAVOR(info.InputFormat)
	SwapFirst := T_SWAPFIRST(info.InputFormat)
	Extra := T_EXTRA(info.InputFormat)
	ExtraFirst := DoSwap ^ SwapFirst
	Planar := T_PLANAR(info.InputFormat)
	Premul := T_PREMUL(info.InputFormat)
	maximum := 1.0
	if IsInkSpace(info.InputFormat) {
		maximum = 100.0
	}
	alphaFactor := 1.0
	var start uint32

	Stride /= PixelSize(info.InputFormat)

	ptr := (*[1 << 30]float64)(unsafe.Pointer(&accum[0]))[:len(accum)/8]

	if Premul != 0 && Extra > 0 {
		if Planar != 0 {
			if ExtraFirst != 0 {
				alphaFactor = ptr[0] / maximum
			} else {
				alphaFactor = ptr[nChan*Stride] / maximum
			}
		} else {
			if ExtraFirst != 0 {
				alphaFactor = ptr[0] / maximum
			} else {
				alphaFactor = ptr[nChan] / maximum
			}
		}
	}

	if ExtraFirst != 0 {
		start = uint32(Extra)
	}

	for i := uint32(0); i < nChan; i++ {
		index := i
		if DoSwap != 0 {
			index = nChan - i - 1
		}

		var v float64
		if Planar != 0 {
			v = ptr[(i+start)*Stride]
		} else {
			v = ptr[i+start]
		}

		if Premul != 0 && alphaFactor > 0 {
			v /= alphaFactor
		}

		v /= maximum
		wIn[index] = float32(ReverseFloat(float32(v), Reverse != 0))
	}

	if Extra == 0 && SwapFirst != 0 {
		SwapFirstFloat(wIn, nChan)
	}

	if Planar != 0 {
		return accum[8:]
	}
	return accum[(nChan+Extra)*8:]
}
func UnrollLabDoubleToFloat(info *cmsTRANSFORM, wIn []float32, accum []uint8, Stride uint32) []uint8 {
	ptr := (*[1 << 30]float64)(unsafe.Pointer(&accum[0]))[:len(accum)/8]

	if T_PLANAR(info.InputFormat) != 0 {
		Stride /= PixelSize(info.InputFormat)
		wIn[0] = float32(ptr[0] / 100.0)                  // L: 0..100 to 0..1
		wIn[1] = float32((ptr[Stride] + 128.0) / 255.0)   // a: -128..127 to 0..1
		wIn[2] = float32((ptr[Stride*2] + 128.0) / 255.0) // b: -128..127 to 0..1
		return accum[8:]
	}

	wIn[0] = float32(ptr[0] / 100.0)           // L: 0..100 to 0..1
	wIn[1] = float32((ptr[1] + 128.0) / 255.0) // a: -128..127 to 0..1
	wIn[2] = float32((ptr[2] + 128.0) / 255.0) // b: -128..127 to 0..1

	return accum[(3+T_EXTRA(info.InputFormat))*8:]
}
func UnrollLabFloatToFloat(info *cmsTRANSFORM, wIn []float32, accum []uint8, Stride uint32) []uint8 {
	ptr := (*[1 << 30]float32)(unsafe.Pointer(&accum[0]))[:len(accum)/4]

	if T_PLANAR(info.InputFormat) != 0 {
		Stride /= PixelSize(info.InputFormat)
		wIn[0] = ptr[0] / 100.0                  // L: 0..100 to 0..1
		wIn[1] = (ptr[Stride] + 128.0) / 255.0   // a: -128..127 to 0..1
		wIn[2] = (ptr[Stride*2] + 128.0) / 255.0 // b: -128..127 to 0..1
		return accum[4:]
	}

	wIn[0] = ptr[0] / 100.0           // L: 0..100 to 0..1
	wIn[1] = (ptr[1] + 128.0) / 255.0 // a: -128..127 to 0..1
	wIn[2] = (ptr[2] + 128.0) / 255.0 // b: -128..127 to 0..1

	return accum[(3+T_EXTRA(info.InputFormat))*4:]
}
func UnrollXYZDoubleToFloat(info *cmsTRANSFORM, wIn []float32, accum []uint8, Stride uint32) []uint8 {
	ptr := (*[1 << 30]float64)(unsafe.Pointer(&accum[0]))[:len(accum)/8]

	if T_PLANAR(info.InputFormat) != 0 {
		Stride /= PixelSize(info.InputFormat)

		wIn[0] = float32(ptr[0] / MAX_ENCODEABLE_XYZ)
		wIn[1] = float32(ptr[Stride] / MAX_ENCODEABLE_XYZ)
		wIn[2] = float32(ptr[Stride*2] / MAX_ENCODEABLE_XYZ)

		return accum[8:]
	}

	wIn[0] = float32(ptr[0] / MAX_ENCODEABLE_XYZ)
	wIn[1] = float32(ptr[1] / MAX_ENCODEABLE_XYZ)
	wIn[2] = float32(ptr[2] / MAX_ENCODEABLE_XYZ)

	return accum[(3+T_EXTRA(info.InputFormat))*8:]
}
func UnrollXYZFloatToFloat(info *cmsTRANSFORM, wIn []float32, accum []uint8, Stride uint32) []uint8 {
	ptr := (*[1 << 30]float32)(unsafe.Pointer(&accum[0]))[:len(accum)/4]

	if T_PLANAR(info.InputFormat) != 0 {
		Stride /= PixelSize(info.InputFormat)

		wIn[0] = float32(ptr[0] / MAX_ENCODEABLE_XYZ)
		wIn[1] = float32(ptr[Stride] / MAX_ENCODEABLE_XYZ)
		wIn[2] = float32(ptr[Stride*2] / MAX_ENCODEABLE_XYZ)

		return accum[4:]
	}

	wIn[0] = float32(ptr[0] / MAX_ENCODEABLE_XYZ)
	wIn[1] = float32(ptr[1] / MAX_ENCODEABLE_XYZ)
	wIn[2] = float32(ptr[2] / MAX_ENCODEABLE_XYZ)

	return accum[(3+T_EXTRA(info.InputFormat))*4:]
}
func lab4toFloat(wIn []float32, lab4 [3]uint16) {
	L := float32(lab4[0]) / 655.35
	a := (float32(lab4[1]) / 257.0) - 128.0
	b := (float32(lab4[2]) / 257.0) - 128.0

	wIn[0] = L / 100.0           // from 0..100 to 0..1
	wIn[1] = (a + 128.0) / 255.0 // from -128..+127 to 0..1
	wIn[2] = (b + 128.0) / 255.0
}
func UnrollLabV2_8ToFloat(info *cmsTRANSFORM, wIn []float32, accum []uint8, Stride uint32) []uint8 {
	lab4 := [3]uint16{
		FromLabV2ToLabV4(FROM_8_TO_16(accum[0])),
		FromLabV2ToLabV4(FROM_8_TO_16(accum[1])),
		FromLabV2ToLabV4(FROM_8_TO_16(accum[2])),
	}

	lab4toFloat(wIn, lab4)

	return accum[3:]
}
func UnrollALabV2_8ToFloat(info *cmsTRANSFORM, wIn []float32, accum []uint8, Stride uint32) []uint8 {
	lab4 := [3]uint16{
		FromLabV2ToLabV4(FROM_8_TO_16(accum[1])),
		FromLabV2ToLabV4(FROM_8_TO_16(accum[2])),
		FromLabV2ToLabV4(FROM_8_TO_16(accum[3])),
	}

	lab4toFloat(wIn, lab4)

	return accum[4:]
}
func UnrollLabV2_16ToFloat(info *cmsTRANSFORM, wIn []float32, accum []uint8, Stride uint32) []uint8 {
	lab4 := [3]uint16{
		FromLabV2ToLabV4(*(*uint16)(unsafe.Pointer(&accum[0]))),
		FromLabV2ToLabV4(*(*uint16)(unsafe.Pointer(&accum[2]))),
		FromLabV2ToLabV4(*(*uint16)(unsafe.Pointer(&accum[4]))),
	}

	lab4toFloat(wIn, lab4)

	return accum[6:]
}
func PackChunkyBytes(info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	nChan := T_CHANNELS(info.OutputFormat)
	DoSwap := T_DOSWAP(info.OutputFormat)
	Reverse := T_FLAVOR(info.OutputFormat)
	Extra := T_EXTRA(info.OutputFormat)
	SwapFirst := T_SWAPFIRST(info.OutputFormat)
	Premul := T_PREMUL(info.OutputFormat)
	ExtraFirst := DoSwap ^ SwapFirst
	swap1 := output
	var v uint16
	var alphaFactor uint32

	if ExtraFirst != 0 {
		if Premul != 0 && Extra != 0 {
			alphaFactor = uint32(cmsToFixedDomain(int(FROM_8_TO_16(output[0]))))
		}
		output = output[Extra:]
	} else {
		if Premul != 0 && Extra != 0 {
			alphaFactor = uint32(cmsToFixedDomain(int(FROM_8_TO_16(output[nChan]))))
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
			v = uint16((uint32(v)*alphaFactor + 0x8000) >> 16)
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
func PackChunkyWords(info *cmsTRANSFORM, wOut []uint16, output []byte, stride uint32) []byte {
	nChan := T_CHANNELS(info.OutputFormat)
	swapEndian := T_ENDIAN16(info.OutputFormat)
	doSwap := T_DOSWAP(info.OutputFormat)
	reverse := T_FLAVOR(info.OutputFormat)
	extra := T_EXTRA(info.OutputFormat)
	swapFirst := T_SWAPFIRST(info.OutputFormat)
	premul := T_PREMUL(info.OutputFormat)
	extraFirst := doSwap ^ swapFirst

	alphaFactor := uint32(0)
	swap1 := output[:2*nChan] // Create a slice equivalent to the first `nChan` words

	// Pointer to the current position in the output buffer
	outputPtr := unsafe.Pointer(&output[0])

	if extraFirst != 0 {
		if premul != 0 && extra != 0 {
			alphaFactor = uint32(cmsToFixedDomain(int(*(*uint16)(outputPtr))))
		}
		outputPtr = unsafe.Add(outputPtr, uintptr(extra*2))
	} else {
		if premul != 0 && extra != 0 {
			alphaFactor = uint32(cmsToFixedDomain(int(*(*uint16)(unsafe.Add(outputPtr, uintptr(nChan*2))))))
		}
	}

	for i := uint32(0); i < nChan; i++ {
		index := i
		if doSwap != 0 {
			index = nChan - i - 1
		}

		v := wOut[index]

		if swapEndian != 0 {
			v = CHANGE_ENDIAN(v)
		}

		if reverse != 0 {
			v = REVERSE_FLAVOR_16(v)
		}

		if premul != 0 {
			v = uint16((uint32(v)*alphaFactor + 0x8000) >> 16)
		}

		// Write v to the output buffer
		*(*uint16)(outputPtr) = v
		outputPtr = unsafe.Add(outputPtr, 2)
	}

	if extraFirst == 0 {
		outputPtr = unsafe.Add(outputPtr, uintptr(extra*2))
	}

	if extra == 0 && swapFirst != 0 {
		// Use memmove to shift the memory in the slice
		memmove(
			unsafe.Pointer(&swap1[2]), // Destination: second element in swap1
			unsafe.Pointer(&swap1[0]), // Source: first element in swap1
			uintptr((nChan-1)*2),      // Number of bytes to move
		)
		*(*uint16)(unsafe.Pointer(&swap1[0])) = wOut[nChan-1]
	}

	// Return the slice advanced by the number of bytes written
	bytesWritten := uintptr(outputPtr) - uintptr(unsafe.Pointer(&output[0]))
	return output[:bytesWritten]
}

func PackPlanarBytes(info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	nChan := T_CHANNELS(info.OutputFormat)
	DoSwap := T_DOSWAP(info.OutputFormat)
	SwapFirst := T_SWAPFIRST(info.OutputFormat)
	Reverse := T_FLAVOR(info.OutputFormat)
	Extra := T_EXTRA(info.OutputFormat)
	ExtraFirst := DoSwap ^ SwapFirst
	Premul := T_PREMUL(info.OutputFormat)
	Init := output
	var alphaFactor uint32

	if ExtraFirst != 0 {
		if Premul != 0 && Extra != 0 {
			alphaFactor = uint32(cmsToFixedDomain(int(FROM_8_TO_16(output[0]))))
		}
		output = output[Extra*Stride:]
	} else {
		if Premul != 0 && Extra != 0 {
			alphaFactor = uint32(cmsToFixedDomain(int(FROM_8_TO_16(output[nChan*Stride]))))
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
			v = uint16((uint32(v)*alphaFactor + 0x8000) >> 16)
		}

		output[0] = FROM_16_TO_8(v)
		output = output[Stride:]
	}

	return Init[1:]
}
func PackPlanarWords(info *cmsTRANSFORM, wOut []uint16, output []byte, stride uint32) []byte {
	nChan := T_CHANNELS(info.OutputFormat)
	doSwap := T_DOSWAP(info.OutputFormat)
	swapFirst := T_SWAPFIRST(info.OutputFormat)
	reverse := T_FLAVOR(info.OutputFormat)
	extra := T_EXTRA(info.OutputFormat)
	extraFirst := doSwap ^ swapFirst
	premul := T_PREMUL(info.OutputFormat)
	swapEndian := T_ENDIAN16(info.OutputFormat)

	init := output // Keep a reference to the start of the output slice
	alphaFactor := uint32(0)

	// Handle extra channels
	if extraFirst != 0 {
		if premul != 0 && extra != 0 {
			alphaFactor = uint32(cmsToFixedDomain(int(uint16(output[0]) | uint16(output[1])<<8)))
		}
		output = output[extra*stride:]
	} else {
		if premul != 0 && extra != 0 {
			offset := int(nChan * stride)
			alphaFactor = uint32(cmsToFixedDomain(int(uint16(output[offset]) | uint16(output[offset+1])<<8)))
		}
	}

	for i := uint32(0); i < nChan; i++ {
		index := i
		if doSwap != 0 {
			index = nChan - i - 1
		}

		v := wOut[index]

		if swapEndian != 0 {
			v = CHANGE_ENDIAN(v)
		}

		if reverse != 0 {
			v = REVERSE_FLAVOR_16(v)
		}

		if premul != 0 {
			v = uint16((uint32(v)*alphaFactor + 0x8000) >> 16)
		}

		// Write the value to the output slice
		output[0] = byte(v & 0xFF)
		output[1] = byte((v >> 8) & 0xFF)

		// Move to the next stride position
		output = output[stride:]
	}

	// Return the updated slice
	bytesWritten := len(init) - len(output) + 2
	return init[:bytesWritten]
}

func Pack6Bytes(info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	output[0] = FROM_16_TO_8(wOut[0])
	output[1] = FROM_16_TO_8(wOut[1])
	output[2] = FROM_16_TO_8(wOut[2])
	output[3] = FROM_16_TO_8(wOut[3])
	output[4] = FROM_16_TO_8(wOut[4])
	output[5] = FROM_16_TO_8(wOut[5])

	return output[6:]
}
func Pack6BytesSwap(info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	output[0] = FROM_16_TO_8(wOut[5])
	output[1] = FROM_16_TO_8(wOut[4])
	output[2] = FROM_16_TO_8(wOut[3])
	output[3] = FROM_16_TO_8(wOut[2])
	output[4] = FROM_16_TO_8(wOut[1])
	output[5] = FROM_16_TO_8(wOut[0])

	return output[6:]
}
func Pack6Words(info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	for i := 0; i < 6; i++ {
		*(*uint16)(unsafe.Pointer(&output[i*2])) = wOut[i]
	}
	return output[12:]
}
func Pack6WordsSwap(info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	for i := 0; i < 6; i++ {
		*(*uint16)(unsafe.Pointer(&output[i*2])) = wOut[5-i]
	}
	return output[12:]
}
func Pack4Bytes(info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	for i := 0; i < 4; i++ {
		output[i] = FROM_16_TO_8(wOut[i])
	}
	return output[4:]
}
func Pack4BytesReverse(info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	for i := 0; i < 4; i++ {
		output[i] = REVERSE_FLAVOR_8(FROM_16_TO_8(wOut[i]))
	}
	return output[4:]
}
func Pack4BytesSwapFirst(info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	output[0] = FROM_16_TO_8(wOut[3])
	output[1] = FROM_16_TO_8(wOut[0])
	output[2] = FROM_16_TO_8(wOut[1])
	output[3] = FROM_16_TO_8(wOut[2])

	return output[4:]
}
func Pack4BytesSwap(info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	for i := 0; i < 4; i++ {
		output[i] = FROM_16_TO_8(wOut[3-i])
	}
	return output[4:]
}
func Pack4BytesSwapSwapFirst(info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	output[0] = FROM_16_TO_8(wOut[2])
	output[1] = FROM_16_TO_8(wOut[1])
	output[2] = FROM_16_TO_8(wOut[0])
	output[3] = FROM_16_TO_8(wOut[3])

	return output[4:]
}
func Pack4Words(info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	for i := 0; i < 4; i++ {
		*(*uint16)(unsafe.Pointer(&output[i*2])) = wOut[i]
	}
	return output[8:]
}
func Pack4WordsReverse(info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	for i := 0; i < 4; i++ {
		*(*uint16)(unsafe.Pointer(&output[i*2])) = REVERSE_FLAVOR_16(wOut[i])
	}
	return output[8:]
}
func Pack4WordsSwap(info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	for i := 0; i < 4; i++ {
		*(*uint16)(unsafe.Pointer(&output[i*2])) = wOut[3-i]
	}
	return output[8:]
}
func Pack4WordsBigEndian(info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	for i := 0; i < 4; i++ {
		*(*uint16)(unsafe.Pointer(&output[i*2])) = CHANGE_ENDIAN(wOut[i])
	}
	return output[8:]
}
func PackLabV2_8(info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	output[0] = FROM_16_TO_8(FromLabV4ToLabV2(wOut[0]))
	output[1] = FROM_16_TO_8(FromLabV4ToLabV2(wOut[1]))
	output[2] = FROM_16_TO_8(FromLabV4ToLabV2(wOut[2]))

	return output[3:]
}
func PackALabV2_8(info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	output[0] = 0 // Placeholder for alpha channel
	output[1] = FROM_16_TO_8(FromLabV4ToLabV2(wOut[0]))
	output[2] = FROM_16_TO_8(FromLabV4ToLabV2(wOut[1]))
	output[3] = FROM_16_TO_8(FromLabV4ToLabV2(wOut[2]))

	return output[4:]
}
func PackLabV2_16(info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	*(*uint16)(unsafe.Pointer(&output[0])) = FromLabV4ToLabV2(wOut[0])
	*(*uint16)(unsafe.Pointer(&output[2])) = FromLabV4ToLabV2(wOut[1])
	*(*uint16)(unsafe.Pointer(&output[4])) = FromLabV4ToLabV2(wOut[2])

	return output[6:]
}
func Pack3Bytes(info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	output[0] = FROM_16_TO_8(wOut[0])
	output[1] = FROM_16_TO_8(wOut[1])
	output[2] = FROM_16_TO_8(wOut[2])

	return output[3:]
}
func Pack3BytesOptimized(info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	output[0] = uint8(wOut[0] & 0xFF)
	output[1] = uint8(wOut[1] & 0xFF)
	output[2] = uint8(wOut[2] & 0xFF)

	return output[3:]
}
func Pack3BytesSwap(info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	output[0] = FROM_16_TO_8(wOut[2])
	output[1] = FROM_16_TO_8(wOut[1])
	output[2] = FROM_16_TO_8(wOut[0])

	return output[3:]
}
func Pack3BytesSwapOptimized(info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	output[0] = uint8(wOut[2] & 0xFF)
	output[1] = uint8(wOut[1] & 0xFF)
	output[2] = uint8(wOut[0] & 0xFF)

	return output[3:]
}
func Pack3Words(info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	*(*uint16)(unsafe.Pointer(&output[0])) = wOut[0]
	*(*uint16)(unsafe.Pointer(&output[2])) = wOut[1]
	*(*uint16)(unsafe.Pointer(&output[4])) = wOut[2]

	return output[6:]
}
func Pack3WordsSwap(info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	*(*uint16)(unsafe.Pointer(&output[0])) = wOut[2]
	*(*uint16)(unsafe.Pointer(&output[2])) = wOut[1]
	*(*uint16)(unsafe.Pointer(&output[4])) = wOut[0]

	return output[6:]
}
func Pack3WordsBigEndian(info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	*(*uint16)(unsafe.Pointer(&output[0])) = CHANGE_ENDIAN(wOut[0])
	*(*uint16)(unsafe.Pointer(&output[2])) = CHANGE_ENDIAN(wOut[1])
	*(*uint16)(unsafe.Pointer(&output[4])) = CHANGE_ENDIAN(wOut[2])

	return output[6:]
}
func Pack3BytesAndSkip1(info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	output[0] = FROM_16_TO_8(wOut[0])
	output[1] = FROM_16_TO_8(wOut[1])
	output[2] = FROM_16_TO_8(wOut[2])
	output[3] = 0 // Skip 1 byte

	return output[4:]
}
func Pack3BytesAndSkip1Optimized(info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	output[0] = uint8(wOut[0] & 0xFF)
	output[1] = uint8(wOut[1] & 0xFF)
	output[2] = uint8(wOut[2] & 0xFF)
	output[3] = 0 // Skip 1 byte

	return output[4:]
}
func Pack3BytesAndSkip1SwapFirst(info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	output[0] = 0 // Skip first byte
	output[1] = FROM_16_TO_8(wOut[0])
	output[2] = FROM_16_TO_8(wOut[1])
	output[3] = FROM_16_TO_8(wOut[2])

	return output[4:]
}
func Pack3BytesAndSkip1SwapFirstOptimized(info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	output[0] = 0 // Skip first byte
	output[1] = uint8(wOut[0] & 0xFF)
	output[2] = uint8(wOut[1] & 0xFF)
	output[3] = uint8(wOut[2] & 0xFF)

	return output[4:]
}
func Pack3BytesAndSkip1Swap(info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	output[0] = 0 // Skip first byte
	output[1] = FROM_16_TO_8(wOut[2])
	output[2] = FROM_16_TO_8(wOut[1])
	output[3] = FROM_16_TO_8(wOut[0])

	return output[4:]
}
func Pack3BytesAndSkip1SwapOptimized(info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	output[0] = 0 // Skip first byte
	output[1] = uint8(wOut[2] & 0xFF)
	output[2] = uint8(wOut[1] & 0xFF)
	output[3] = uint8(wOut[0] & 0xFF)

	return output[4:]
}
func Pack3WordsAndSkip1(info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	*(*uint16)(unsafe.Pointer(&output[0])) = wOut[0]
	*(*uint16)(unsafe.Pointer(&output[2])) = wOut[1]
	*(*uint16)(unsafe.Pointer(&output[4])) = wOut[2]
	output = output[8:] // Skip 1 word (2 bytes)

	return output
}
func Pack3WordsAndSkip1Swap(info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	output = output[2:] // Skip 1 word (2 bytes)
	*(*uint16)(unsafe.Pointer(&output[0])) = wOut[2]
	*(*uint16)(unsafe.Pointer(&output[2])) = wOut[1]
	*(*uint16)(unsafe.Pointer(&output[4])) = wOut[0]

	return output[6:]
}
func Pack3WordsAndSkip1SwapFirst(info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	output = output[2:] // Skip first word
	*(*uint16)(unsafe.Pointer(&output[0])) = wOut[0]
	*(*uint16)(unsafe.Pointer(&output[2])) = wOut[1]
	*(*uint16)(unsafe.Pointer(&output[4])) = wOut[2]

	return output[6:]
}

func Pack3WordsAndSkip1SwapSwapFirst(info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	*(*uint16)(unsafe.Pointer(&output[0])) = wOut[2]
	*(*uint16)(unsafe.Pointer(&output[2])) = wOut[1]
	*(*uint16)(unsafe.Pointer(&output[4])) = wOut[0]
	output = output[8:] // Skip 1 word after swapping

	return output
}
func Pack3BytesAndSkip1SwapSwapFirst(info *cmsTRANSFORM, wOut []uint16, output []uint8, stride uint32) []uint8 {
	output[0] = uint8(wOut[2] >> 8) // FROM_16_TO_8 in the C code.
	output[1] = uint8(wOut[1] >> 8) // FROM_16_TO_8 in the C code.
	output[2] = uint8(wOut[0] >> 8) // FROM_16_TO_8 in the C code.
	output[3] = 0                   // Skip 1 byte (set it to 0 or leave as is).
	return output[4:]               // Advance the pointer.

	// `info` and `stride` are unused, as in the C code.
}

func Pack3BytesAndSkip1SwapSwapFirstOptimized(info *cmsTRANSFORM, wOut []uint16, output []uint8, stride uint32) []uint8 {
	output[0] = uint8(wOut[2] & 0xFF) // Extract the least significant byte.
	output[1] = uint8(wOut[1] & 0xFF) // Extract the least significant byte.
	output[2] = uint8(wOut[0] & 0xFF) // Extract the least significant byte.
	output[3] = 0                     // Skip 1 byte (set it to 0 or leave as is).
	return output[4:]                 // Advance the pointer.

	// `info` and `stride` are unused, as in the C code.
}

func Pack1Byte(info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	output[0] = FROM_16_TO_8(wOut[0])
	return output[1:]
}
func Pack1ByteReversed(info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	output[0] = FROM_16_TO_8(REVERSE_FLAVOR_16(wOut[0]))
	return output[1:]
}
func Pack1ByteSkip1(info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	output[0] = FROM_16_TO_8(wOut[0])
	return output[2:] // Skip 1 byte
}
func Pack1ByteSkip1SwapFirst(info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	output[1] = FROM_16_TO_8(wOut[0]) // Skip the first byte
	return output[2:]
}
func Pack1Word(info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	*(*uint16)(unsafe.Pointer(&output[0])) = wOut[0]
	return output[2:]
}
func Pack1WordReversed(info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	*(*uint16)(unsafe.Pointer(&output[0])) = REVERSE_FLAVOR_16(wOut[0])
	return output[2:]
}
func Pack1WordBigEndian(info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	*(*uint16)(unsafe.Pointer(&output[0])) = CHANGE_ENDIAN(wOut[0])
	return output[2:]
}
func Pack1WordSkip1(info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	*(*uint16)(unsafe.Pointer(&output[0])) = wOut[0]
	return output[4:] // Skip 2 bytes (1 word)
}
func Pack1WordSkip1SwapFirst(info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	output = output[2:] // Skip 2 bytes (1 word)
	*(*uint16)(unsafe.Pointer(&output[0])) = wOut[0]
	return output[2:]
}
func PackLabDoubleFrom16(info *cmsTRANSFORM, wOut []uint16, output []uint8, stride uint32) []uint8 {
	if T_PLANAR(info.OutputFormat) != 0 {
		// Planar format
		var lab cmsCIELab
		cmsLabEncoded2Float(&lab, *(*[3]uint16)(wOut))

		// Convert output to a []float64 equivalent using unsafe
		outPtr := (*[3]float64)(unsafe.Pointer(&output[0])) // Assuming sufficient space in output
		outPtr[0] = lab.L
		outPtr[stride] = lab.a
		outPtr[2*stride] = lab.b

		// Return the updated slice as uint8
		return output[8:] // sizeof(float64) = 8, move forward by one float64
	} else {
		// Interleaved format
		var lab cmsCIELab
		cmsLabEncoded2Float(&lab, *(*[3]uint16)(wOut))

		// Write directly into the output slice
		outPtr := (*[3]float64)(unsafe.Pointer(&output[0])) // Assuming sufficient space in output
		outPtr[0] = lab.L
		outPtr[1] = lab.a
		outPtr[2] = lab.b

		// Adjust for extra bytes (stride)
		extraStride := T_EXTRA(info.OutputFormat) * 8 // Extra space in bytes
		return output[24+extraStride:]                // 3 * sizeof(float64) = 24, add extraStride
	}
}

func PackLabFloatFrom16(info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	var Lab cmsCIELab
	cmsLabEncoded2Float(&Lab, *(*[3]uint16)(wOut))

	if T_PLANAR(info.OutputFormat) != 0 {
		//out := (*float32)(unsafe.Pointer(&output[0]))
		Stride /= PixelSize(info.OutputFormat)

		// Convert output to a []float64 equivalent using unsafe
		outPtr := (*[3]float32)(unsafe.Pointer(&output[0])) // Assuming sufficient space in output
		outPtr[0] = float32(Lab.L)
		outPtr[Stride] = float32(Lab.a)
		outPtr[Stride*2] = float32(Lab.b)

		return output[4:] // sizeof(float32)
	} else {
		outPtr := (*[3]float32)(unsafe.Pointer(&output[0])) // Assuming sufficient space in output
		outPtr[0] = float32(Lab.L)
		outPtr[1] = float32(Lab.a)
		outPtr[2] = float32(Lab.b)

		return output[12+(T_EXTRA(info.OutputFormat)*4):] // 3 * sizeof(float32) + Extra
	}
}
func PackXYZDoubleFrom16(info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	if T_PLANAR(info.OutputFormat) != 0 {
		var XYZ cmsCIEXYZ
		cmsXYZEncoded2Float(&XYZ, *(*[3]uint16)(wOut))

		out := (*[3]float64)(unsafe.Pointer(&output[0]))
		Stride /= PixelSize(info.OutputFormat)

		out[0] = XYZ.X
		out[Stride] = XYZ.Y
		out[Stride*2] = XYZ.Z

		return output[8:] // sizeof(float64)
	} else {
		cmsXYZEncoded2Float((*cmsCIEXYZ)(unsafe.Pointer(&output[0])), *(*[3]uint16)(wOut))
		return output[8+(T_EXTRA(info.OutputFormat)*8):] // sizeof(cmsCIEXYZ) + Extra
	}
}
func PackXYZFloatFrom16(info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	if T_PLANAR(info.OutputFormat) != 0 {
		var XYZ cmsCIEXYZ
		cmsXYZEncoded2Float(&XYZ, *(*[3]uint16)(wOut))

		out := (*[3]float32)(unsafe.Pointer(&output[0]))
		Stride /= PixelSize(info.OutputFormat)

		out[0] = float32(XYZ.X)
		out[Stride] = float32(XYZ.Y)
		out[Stride*2] = float32(XYZ.Z)

		return output[4:] // sizeof(float32)
	} else {
		var XYZ cmsCIEXYZ
		out := (*[3]float32)(unsafe.Pointer(&output[0]))
		cmsXYZEncoded2Float(&XYZ, *(*[3]uint16)(wOut))

		out[0] = float32(XYZ.X)
		out[1] = float32(XYZ.Y)
		out[2] = float32(XYZ.Z)

		return output[12+(T_EXTRA(info.OutputFormat)*4):] // 3 * sizeof(float32) + Extra
	}
}

func PackDoubleFrom16(info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	nChan := T_CHANNELS(info.OutputFormat)
	DoSwap := T_DOSWAP(info.OutputFormat)
	Reverse := T_FLAVOR(info.OutputFormat)
	Extra := T_EXTRA(info.OutputFormat)
	SwapFirst := T_SWAPFIRST(info.OutputFormat)
	Planar := T_PLANAR(info.OutputFormat)
	ExtraFirst := DoSwap ^ SwapFirst
	var maximum float64 = 65535.0
	if IsInkSpace(info.OutputFormat) {
		maximum = 655.35
	}
	var v float64
	outputFloats := (*[1 << 30]float64)(unsafe.Pointer(&output[0]))[:len(output)/8]
	var start uint32

	Stride /= PixelSize(info.OutputFormat)

	if ExtraFirst != 0 {
		start = Extra
	}

	for i := uint32(0); i < nChan; i++ {
		var index uint32
		if DoSwap != 0 {
			index = nChan - i - 1
		} else {
			index = i
		}

		v = float64(wOut[index]) / maximum

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
		return output[8:] // sizeof(float64)
	} else {
		return output[(nChan+Extra)*8:]
	}
}
func PackFloatFrom16(info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	nChan := T_CHANNELS(info.OutputFormat)
	DoSwap := T_DOSWAP(info.OutputFormat)
	Reverse := T_FLAVOR(info.OutputFormat)
	Extra := T_EXTRA(info.OutputFormat)
	SwapFirst := T_SWAPFIRST(info.OutputFormat)
	Planar := T_PLANAR(info.OutputFormat)
	ExtraFirst := DoSwap ^ SwapFirst
	var maximum float64 = 65535.0
	if IsInkSpace(info.OutputFormat) {
		maximum = 655.35
	}
	var v float64
	outputFloats := (*[1 << 30]float32)(unsafe.Pointer(&output[0]))[:len(output)/4]
	var start uint32

	Stride /= PixelSize(info.OutputFormat)

	if ExtraFirst != 0 {
		start = Extra
	}

	for i := uint32(0); i < nChan; i++ {
		var index uint32
		if DoSwap != 0 {
			index = nChan - i - 1
		} else {
			index = i
		}

		v = float64(wOut[index]) / maximum

		if Reverse != 0 {
			v = maximum - v
		}

		if Planar != 0 {
			outputFloats[(i+start)*Stride] = float32(v)
		} else {
			outputFloats[i+start] = float32(v)
		}
	}

	if Extra == 0 && SwapFirst != 0 {
		copy(outputFloats[1:], outputFloats[:nChan-1])
		outputFloats[0] = float32(v)
	}

	if Planar != 0 {
		return output[4:] // sizeof(float32)
	} else {
		return output[(nChan+Extra)*4:]
	}
}
func PackFloatsFromFloat(info *cmsTRANSFORM, wOut []float32, output []uint8, Stride uint32) []uint8 {
	nChan := T_CHANNELS(info.OutputFormat)
	DoSwap := T_DOSWAP(info.OutputFormat)
	Reverse := T_FLAVOR(info.OutputFormat)
	Extra := T_EXTRA(info.OutputFormat)
	SwapFirst := T_SWAPFIRST(info.OutputFormat)
	Planar := T_PLANAR(info.OutputFormat)
	ExtraFirst := DoSwap ^ SwapFirst
	maximum := float64(1.0)
	if IsInkSpace(info.OutputFormat) {
		maximum = 100.0
	}
	outputFloats := (*[1 << 30]float32)(unsafe.Pointer(&output[0]))[:len(output)/4]
	var v float64
	var start uint32

	Stride /= PixelSize(info.OutputFormat)

	if ExtraFirst != 0 {
		start = Extra
	}

	for i := uint32(0); i < nChan; i++ {
		var index uint32
		if DoSwap != 0 {
			index = nChan - i - 1
		} else {
			index = i
		}

		v = float64(wOut[index]) * maximum

		if Reverse != 0 {
			v = maximum - v
		}

		if Planar != 0 {
			outputFloats[(i+start)*Stride] = float32(v)
		} else {
			outputFloats[i+start] = float32(v)
		}
	}

	if Extra == 0 && SwapFirst != 0 {
		copy(outputFloats[1:], outputFloats[:nChan-1])
		outputFloats[0] = float32(v)
	}

	if Planar != 0 {
		return output[4:]
	} else {
		return output[(nChan+Extra)*4:]
	}
}
func PackDoublesFromFloat(info *cmsTRANSFORM, wOut []float32, output []uint8, Stride uint32) []uint8 {
	nChan := T_CHANNELS(info.OutputFormat)
	DoSwap := T_DOSWAP(info.OutputFormat)
	Reverse := T_FLAVOR(info.OutputFormat)
	Extra := T_EXTRA(info.OutputFormat)
	SwapFirst := T_SWAPFIRST(info.OutputFormat)
	Planar := T_PLANAR(info.OutputFormat)
	ExtraFirst := DoSwap ^ SwapFirst
	maximum := float64(1.0)
	if IsInkSpace(info.OutputFormat) {
		maximum = 100.0
	}
	outputFloats := (*[1 << 30]float64)(unsafe.Pointer(&output[0]))[:len(output)/8]
	var v float64
	var start uint32

	Stride /= PixelSize(info.OutputFormat)

	if ExtraFirst != 0 {
		start = Extra
	}

	for i := uint32(0); i < nChan; i++ {
		var index uint32
		if DoSwap != 0 {
			index = nChan - i - 1
		} else {
			index = i
		}

		v = float64(wOut[index]) * maximum

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
func PackLabFloatFromFloat(info *cmsTRANSFORM, wOut []float32, output []uint8, Stride uint32) []uint8 {
	if T_PLANAR(info.OutputFormat) != 0 {
		out := (*[1 << 30]float32)(unsafe.Pointer(&output[0]))[:len(output)/4]
		Stride /= PixelSize(info.OutputFormat)

		out[0] = wOut[0] * 100.0
		out[Stride] = wOut[1]*255.0 - 128.0
		out[Stride*2] = wOut[2]*255.0 - 128.0

		return output[4:]
	} else {
		out := (*[3]float32)(unsafe.Pointer(&output[0]))
		out[0] = wOut[0] * 100.0
		out[1] = wOut[1]*255.0 - 128.0
		out[2] = wOut[2]*255.0 - 128.0

		return output[12+(T_EXTRA(info.OutputFormat)*4):]
	}
}
func PackLabDoubleFromFloat(info *cmsTRANSFORM, wOut []float32, output []uint8, Stride uint32) []uint8 {
	if T_PLANAR(info.OutputFormat) != 0 {
		out := (*[1 << 30]float64)(unsafe.Pointer(&output[0]))[:len(output)/8]
		Stride /= PixelSize(info.OutputFormat)

		out[0] = float64(wOut[0] * 100.0)
		out[Stride] = float64(wOut[1]*255.0 - 128.0)
		out[Stride*2] = float64(wOut[2]*255.0 - 128.0)

		return output[8:]
	} else {
		out := (*[3]float64)(unsafe.Pointer(&output[0]))
		out[0] = float64(wOut[0] * 100.0)
		out[1] = float64(wOut[1]*255.0 - 128.0)
		out[2] = float64(wOut[2]*255.0 - 128.0)

		return output[24+(T_EXTRA(info.OutputFormat)*8):]
	}
}
func PackXYZFloatFromFloat(info *cmsTRANSFORM, wOut []float32, output []uint8, Stride uint32) []uint8 {
	if T_PLANAR(info.OutputFormat) != 0 {
		out := (*[1 << 30]float32)(unsafe.Pointer(&output[0]))[:len(output)/4]
		Stride /= PixelSize(info.OutputFormat)

		out[0] = wOut[0] * MAX_ENCODEABLE_XYZ
		out[Stride] = wOut[1] * MAX_ENCODEABLE_XYZ
		out[Stride*2] = wOut[2] * MAX_ENCODEABLE_XYZ

		return output[4:]
	} else {
		out := (*[3]float32)(unsafe.Pointer(&output[0]))
		out[0] = wOut[0] * MAX_ENCODEABLE_XYZ
		out[1] = wOut[1] * MAX_ENCODEABLE_XYZ
		out[2] = wOut[2] * MAX_ENCODEABLE_XYZ

		return output[12+(T_EXTRA(info.OutputFormat)*4):]
	}
}
func PackXYZDoubleFromFloat(info *cmsTRANSFORM, wOut []float32, output []uint8, Stride uint32) []uint8 {
	if T_PLANAR(info.OutputFormat) != 0 {
		out := (*[1 << 30]float64)(unsafe.Pointer(&output[0]))[:len(output)/8]
		Stride /= PixelSize(info.OutputFormat)

		out[0] = float64(wOut[0] * MAX_ENCODEABLE_XYZ)
		out[Stride] = float64(wOut[1] * MAX_ENCODEABLE_XYZ)
		out[Stride*2] = float64(wOut[2] * MAX_ENCODEABLE_XYZ)

		return output[8:]
	} else {
		out := (*[3]float64)(unsafe.Pointer(&output[0]))
		out[0] = float64(wOut[0] * MAX_ENCODEABLE_XYZ)
		out[1] = float64(wOut[1] * MAX_ENCODEABLE_XYZ)
		out[2] = float64(wOut[2] * MAX_ENCODEABLE_XYZ)

		return output[24+(T_EXTRA(info.OutputFormat)*8):]
	}
}
func UnrollHalfTo16(info *cmsTRANSFORM, wIn []uint16, accum []uint8, Stride uint32) []uint8 {
	nChan := T_CHANNELS(info.InputFormat)
	DoSwap := T_DOSWAP(info.InputFormat)
	Reverse := T_FLAVOR(info.InputFormat)
	SwapFirst := T_SWAPFIRST(info.InputFormat)
	Extra := T_EXTRA(info.InputFormat)
	ExtraFirst := DoSwap ^ SwapFirst
	Planar := T_PLANAR(info.InputFormat)
	var start uint32
	var v float32
	var maximum float32 = 65535.0
	if IsInkSpace(info.InputFormat) {
		maximum = 655.35
	}

	accumHalf := (*[1 << 30]uint16)(unsafe.Pointer(&accum[0]))[:len(accum)/2]

	Stride /= PixelSize(info.InputFormat)

	if ExtraFirst != 0 {
		start = Extra
	}

	for i := uint32(0); i < nChan; i++ {
		var index uint32
		if DoSwap != 0 {
			index = nChan - i - 1
		} else {
			index = i
		}

		if Planar != 0 {
			v = cmsHalf2Float(accumHalf[(i+start)*Stride])
		} else {
			v = cmsHalf2Float(accumHalf[i+start])
		}

		if Reverse != 0 {
			v = maximum - v
		}

		wIn[index] = cmsQuickSaturateWord(float64(v * maximum))
	}

	if Extra == 0 && SwapFirst != 0 {
		tmp := wIn[0]
		copy(wIn[0:], wIn[1:])
		wIn[nChan-1] = tmp
	}

	if Planar != 0 {
		return accum[2:] // sizeof(uint16)
	} else {
		return accum[(nChan+Extra)*2:]
	}
}
func UnrollHalfToFloat(info *cmsTRANSFORM, wIn []float32, accum []uint8, Stride uint32) []uint8 {
	nChan := T_CHANNELS(info.InputFormat)
	DoSwap := T_DOSWAP(info.InputFormat)
	Reverse := T_FLAVOR(info.InputFormat)
	SwapFirst := T_SWAPFIRST(info.InputFormat)
	Extra := T_EXTRA(info.InputFormat)
	ExtraFirst := DoSwap ^ SwapFirst
	Planar := T_PLANAR(info.InputFormat)
	var start uint32
	var v float32
	var maximum float32 = 1.0
	if IsInkSpace(info.InputFormat) {
		maximum = 100.0
	}

	accumHalf := (*[1 << 30]uint16)(unsafe.Pointer(&accum[0]))[:len(accum)/2]

	Stride /= PixelSize(info.InputFormat)

	if ExtraFirst != 0 {
		start = Extra
	}

	for i := uint32(0); i < nChan; i++ {
		var index uint32
		if DoSwap != 0 {
			index = nChan - i - 1
		} else {
			index = i
		}

		if Planar != 0 {
			v = cmsHalf2Float(accumHalf[(i+start)*Stride])
		} else {
			v = cmsHalf2Float(accumHalf[i+start])
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
		return accum[2:] // sizeof(uint16)
	} else {
		return accum[(nChan+Extra)*2:]
	}
}
func PackHalfFrom16(info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	nChan := T_CHANNELS(info.OutputFormat)
	DoSwap := T_DOSWAP(info.OutputFormat)
	Reverse := T_FLAVOR(info.OutputFormat)
	Extra := T_EXTRA(info.OutputFormat)
	SwapFirst := T_SWAPFIRST(info.OutputFormat)
	Planar := T_PLANAR(info.OutputFormat)
	ExtraFirst := DoSwap ^ SwapFirst
	var maximum float32 = 65535.0
	if IsInkSpace(info.OutputFormat) {
		maximum = 655.35
	}
	var v float32
	outputHalf := (*[1 << 30]uint16)(unsafe.Pointer(&output[0]))[:len(output)/2]
	var start uint32

	Stride /= PixelSize(info.OutputFormat)

	if ExtraFirst != 0 {
		start = Extra
	}

	for i := uint32(0); i < nChan; i++ {
		var index uint32
		if DoSwap != 0 {
			index = nChan - i - 1
		} else {
			index = i
		}

		v = float32(wOut[index]) / maximum

		if Reverse != 0 {
			v = maximum - v
		}

		if Planar != 0 {
			outputHalf[(i+start)*Stride] = cmsFloat2Half(v)
		} else {
			outputHalf[i+start] = cmsFloat2Half(v)
		}
	}

	if Extra == 0 && SwapFirst != 0 {
		copy(outputHalf[1:], outputHalf[:nChan-1])
		outputHalf[0] = cmsFloat2Half(v)
	}

	if Planar != 0 {
		return output[2:] // sizeof(uint16)
	} else {
		return output[(nChan+Extra)*2:]
	}
}
func PackHalfFromFloat(info *cmsTRANSFORM, wOut []float32, output []uint8, Stride uint32) []uint8 {
	nChan := T_CHANNELS(info.OutputFormat)
	DoSwap := T_DOSWAP(info.OutputFormat)
	Reverse := T_FLAVOR(info.OutputFormat)
	Extra := T_EXTRA(info.OutputFormat)
	SwapFirst := T_SWAPFIRST(info.OutputFormat)
	Planar := T_PLANAR(info.OutputFormat)
	ExtraFirst := DoSwap ^ SwapFirst
	var maximum float32 = 1.0
	if IsInkSpace(info.OutputFormat) {
		maximum = 100.0
	}
	outputHalf := (*[1 << 30]uint16)(unsafe.Pointer(&output[0]))[:len(output)/2]
	var v float32
	var start uint32

	Stride /= PixelSize(info.OutputFormat)

	if ExtraFirst != 0 {
		start = Extra
	}

	for i := uint32(0); i < nChan; i++ {
		var index uint32
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
			outputHalf[(i+start)*Stride] = cmsFloat2Half(v)
		} else {
			outputHalf[i+start] = cmsFloat2Half(v)
		}
	}

	if Extra == 0 && SwapFirst != 0 {
		copy(outputHalf[1:], outputHalf[:nChan-1])
		outputHalf[0] = cmsFloat2Half(v)
	}

	if Planar != 0 {
		return output[2:] // sizeof(uint16)
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

// Define the `InputFormattersFloat` array.
var InputFormattersFloat = []cmsFormattersFloat{
	{Type: TYPE_Lab_DBL, Mask: ANYPLANAR | ANYEXTRA, Frm: UnrollLabDoubleToFloat},
	{Type: TYPE_Lab_FLT, Mask: ANYPLANAR | ANYEXTRA, Frm: UnrollLabFloatToFloat},
	{Type: TYPE_XYZ_DBL, Mask: ANYPLANAR | ANYEXTRA, Frm: UnrollXYZDoubleToFloat},
	{Type: TYPE_XYZ_FLT, Mask: ANYPLANAR | ANYEXTRA, Frm: UnrollXYZFloatToFloat},
	{
		Type: FLOAT_SH(1) | BYTES_SH(4),
		Mask: ANYPLANAR | ANYSWAPFIRST | ANYSWAP | ANYEXTRA | ANYPREMUL | ANYCHANNELS | ANYSPACE,
		Frm:  UnrollFloatsToFloat,
	},
	{
		Type: FLOAT_SH(1) | BYTES_SH(0),
		Mask: ANYPLANAR | ANYSWAPFIRST | ANYSWAP | ANYEXTRA | ANYCHANNELS | ANYSPACE | ANYPREMUL,
		Frm:  UnrollDoublesToFloat,
	},
	{Type: TYPE_LabV2_8, Mask: 0, Frm: UnrollLabV2_8ToFloat},
	{Type: TYPE_ALabV2_8, Mask: 0, Frm: UnrollALabV2_8ToFloat},
	{Type: TYPE_LabV2_16, Mask: 0, Frm: UnrollLabV2_16ToFloat},
	{
		Type: BYTES_SH(1),
		Mask: ANYPLANAR | ANYSWAPFIRST | ANYSWAP | ANYEXTRA | ANYCHANNELS | ANYSPACE,
		Frm:  Unroll8ToFloat,
	},
	{
		Type: BYTES_SH(2),
		Mask: ANYPLANAR | ANYSWAPFIRST | ANYSWAP | ANYEXTRA | ANYCHANNELS | ANYSPACE,
		Frm:  Unroll16ToFloat,
	},
	// Conditional inclusion based on `CMS_NO_HALF_SUPPORT`
	// Use build tags or runtime checks in Go as needed.
	{
		Type: FLOAT_SH(1) | BYTES_SH(2),
		Mask: ANYPLANAR | ANYSWAPFIRST | ANYSWAP | ANYEXTRA | ANYCHANNELS | ANYSPACE,
		Frm:  UnrollHalfToFloat,
	},
}

// _cmsGetStockInputFormatter returns the appropriate formatter based on the input and flags.
func cmsGetStockInputFormatter(dwInput, dwFlags uint32) cmsFormatter {
	var fr cmsFormatter

	switch dwFlags {
	case CMS_PACK_FLAGS_16BITS:
		for _, f := range InputFormatters16 {
			if (dwInput & ^f.Mask) == f.Type {
				fr.Fmt16 = f.Frm
				return fr
			}
		}

	case CMS_PACK_FLAGS_FLOAT:
		for _, f := range InputFormattersFloat {
			if (dwInput & ^f.Mask) == f.Type {
				fr.FmtFloat = f.Frm
				return fr
			}
		}

	default:
		// No matching case
	}

	// If no formatter was found, return a zero-value cmsFormatter.
	return fr
}

// OutputFormatters16 is the Go equivalent of the C++ array.
var OutputFormatters16 = []cmsFormatters16{
	{Type: TYPE_Lab_DBL, Mask: ANYPLANAR | ANYEXTRA, Frm: PackLabDoubleFrom16},
	{Type: TYPE_XYZ_DBL, Mask: ANYPLANAR | ANYEXTRA, Frm: PackXYZDoubleFrom16},
	{Type: TYPE_Lab_FLT, Mask: ANYPLANAR | ANYEXTRA, Frm: PackLabFloatFrom16},
	{Type: TYPE_XYZ_FLT, Mask: ANYPLANAR | ANYEXTRA, Frm: PackXYZFloatFrom16},
	{Type: FLOAT_SH(1) | BYTES_SH(0), Mask: ANYFLAVOR | ANYSWAPFIRST | ANYSWAP |
		ANYCHANNELS | ANYPLANAR | ANYEXTRA | ANYSPACE, Frm: PackDoubleFrom16},
	{Type: FLOAT_SH(1) | BYTES_SH(4), Mask: ANYFLAVOR | ANYSWAPFIRST | ANYSWAP |
		ANYCHANNELS | ANYPLANAR | ANYEXTRA | ANYSPACE, Frm: PackFloatFrom16},
	{Type: FLOAT_SH(1) | BYTES_SH(2), Mask: ANYFLAVOR | ANYSWAPFIRST | ANYSWAP |
		ANYCHANNELS | ANYPLANAR | ANYEXTRA | ANYSPACE, Frm: PackHalfFrom16},
	{Type: CHANNELS_SH(1) | BYTES_SH(1), Mask: ANYSPACE, Frm: Pack1Byte},
	{Type: CHANNELS_SH(1) | BYTES_SH(1) | EXTRA_SH(1), Mask: ANYSPACE, Frm: Pack1ByteSkip1},
	{Type: CHANNELS_SH(1) | BYTES_SH(1) | EXTRA_SH(1) | SWAPFIRST_SH(1), Mask: ANYSPACE, Frm: Pack1ByteSkip1SwapFirst},
	{Type: CHANNELS_SH(1) | BYTES_SH(1) | FLAVOR_SH(1), Mask: ANYSPACE, Frm: Pack1ByteReversed},
	{Type: TYPE_LabV2_8, Mask: 0, Frm: PackLabV2_8},
	{Type: TYPE_ALabV2_8, Mask: 0, Frm: PackALabV2_8},
	{Type: TYPE_LabV2_16, Mask: 0, Frm: PackLabV2_16},
	{Type: CHANNELS_SH(3) | BYTES_SH(1) | OPTIMIZED_SH(1), Mask: ANYSPACE, Frm: Pack3BytesOptimized},
	{Type: CHANNELS_SH(3) | BYTES_SH(1) | EXTRA_SH(1) | OPTIMIZED_SH(1), Mask: ANYSPACE, Frm: Pack3BytesAndSkip1Optimized},
	{Type: CHANNELS_SH(3) | BYTES_SH(1) | EXTRA_SH(1) | SWAPFIRST_SH(1) | OPTIMIZED_SH(1),
		Mask: ANYSPACE, Frm: Pack3BytesAndSkip1SwapFirstOptimized},
	{Type: CHANNELS_SH(3) | BYTES_SH(1) | EXTRA_SH(1) | DOSWAP_SH(1) | SWAPFIRST_SH(1) | OPTIMIZED_SH(1),
		Mask: ANYSPACE, Frm: Pack3BytesAndSkip1SwapSwapFirstOptimized},
	{Type: CHANNELS_SH(3) | BYTES_SH(1) | DOSWAP_SH(1) | EXTRA_SH(1) | OPTIMIZED_SH(1),
		Mask: ANYSPACE, Frm: Pack3BytesAndSkip1SwapOptimized},
	{Type: CHANNELS_SH(3) | BYTES_SH(1) | DOSWAP_SH(1) | OPTIMIZED_SH(1), Mask: ANYSPACE, Frm: Pack3BytesSwapOptimized},
	{Type: CHANNELS_SH(3) | BYTES_SH(1), Mask: ANYSPACE, Frm: Pack3Bytes},
	{Type: CHANNELS_SH(3) | BYTES_SH(1) | EXTRA_SH(1), Mask: ANYSPACE, Frm: Pack3BytesAndSkip1},
	{Type: CHANNELS_SH(3) | BYTES_SH(1) | EXTRA_SH(1) | SWAPFIRST_SH(1), Mask: ANYSPACE, Frm: Pack3BytesAndSkip1SwapFirst},
	{Type: CHANNELS_SH(3) | BYTES_SH(1) | EXTRA_SH(1) | DOSWAP_SH(1) | SWAPFIRST_SH(1),
		Mask: ANYSPACE, Frm: Pack3BytesAndSkip1SwapSwapFirst},
	{Type: CHANNELS_SH(3) | BYTES_SH(1) | DOSWAP_SH(1) | EXTRA_SH(1), Mask: ANYSPACE, Frm: Pack3BytesAndSkip1Swap},
	{Type: CHANNELS_SH(3) | BYTES_SH(1) | DOSWAP_SH(1), Mask: ANYSPACE, Frm: Pack3BytesSwap},
	{Type: CHANNELS_SH(4) | BYTES_SH(1), Mask: ANYSPACE, Frm: Pack4Bytes},
	{Type: CHANNELS_SH(4) | BYTES_SH(1) | FLAVOR_SH(1), Mask: ANYSPACE, Frm: Pack4BytesReverse},
	{Type: CHANNELS_SH(4) | BYTES_SH(1) | SWAPFIRST_SH(1), Mask: ANYSPACE, Frm: Pack4BytesSwapFirst},
	{Type: CHANNELS_SH(4) | BYTES_SH(1) | DOSWAP_SH(1), Mask: ANYSPACE, Frm: Pack4BytesSwap},
	{Type: CHANNELS_SH(4) | BYTES_SH(1) | DOSWAP_SH(1) | SWAPFIRST_SH(1), Mask: ANYSPACE, Frm: Pack4BytesSwapSwapFirst},
	{Type: CHANNELS_SH(6) | BYTES_SH(1), Mask: ANYSPACE, Frm: Pack6Bytes},
	{Type: CHANNELS_SH(6) | BYTES_SH(1) | DOSWAP_SH(1), Mask: ANYSPACE, Frm: Pack6BytesSwap},
	{Type: BYTES_SH(1), Mask: ANYFLAVOR | ANYSWAPFIRST | ANYSWAP | ANYEXTRA | ANYCHANNELS | ANYSPACE | ANYPREMUL,
		Frm: PackChunkyBytes},
	{Type: BYTES_SH(1) | PLANAR_SH(1), Mask: ANYFLAVOR | ANYSWAPFIRST | ANYSWAP | ANYEXTRA | ANYCHANNELS | ANYSPACE | ANYPREMUL,
		Frm: PackPlanarBytes},
	{Type: CHANNELS_SH(1) | BYTES_SH(2), Mask: ANYSPACE, Frm: Pack1Word},
	{Type: CHANNELS_SH(1) | BYTES_SH(2) | EXTRA_SH(1), Mask: ANYSPACE, Frm: Pack1WordSkip1},
	{Type: CHANNELS_SH(1) | BYTES_SH(2) | EXTRA_SH(1) | SWAPFIRST_SH(1), Mask: ANYSPACE, Frm: Pack1WordSkip1SwapFirst},
	{Type: CHANNELS_SH(1) | BYTES_SH(2) | FLAVOR_SH(1), Mask: ANYSPACE, Frm: Pack1WordReversed},
	{Type: CHANNELS_SH(1) | BYTES_SH(2) | ENDIAN16_SH(1), Mask: ANYSPACE, Frm: Pack1WordBigEndian},
	{Type: CHANNELS_SH(3) | BYTES_SH(2), Mask: ANYSPACE, Frm: Pack3Words},
	{Type: CHANNELS_SH(3) | BYTES_SH(2) | DOSWAP_SH(1), Mask: ANYSPACE, Frm: Pack3WordsSwap},
	{Type: CHANNELS_SH(3) | BYTES_SH(2) | ENDIAN16_SH(1), Mask: ANYSPACE, Frm: Pack3WordsBigEndian},
	{Type: CHANNELS_SH(3) | BYTES_SH(2) | EXTRA_SH(1), Mask: ANYSPACE, Frm: Pack3WordsAndSkip1},
	{Type: CHANNELS_SH(3) | BYTES_SH(2) | EXTRA_SH(1) | DOSWAP_SH(1), Mask: ANYSPACE, Frm: Pack3WordsAndSkip1Swap},
	{Type: CHANNELS_SH(3) | BYTES_SH(2) | EXTRA_SH(1) | SWAPFIRST_SH(1), Mask: ANYSPACE, Frm: Pack3WordsAndSkip1SwapFirst},
	{Type: CHANNELS_SH(3) | BYTES_SH(2) | EXTRA_SH(1) | DOSWAP_SH(1) | SWAPFIRST_SH(1), Mask: ANYSPACE, Frm: Pack3WordsAndSkip1SwapSwapFirst},
	{Type: CHANNELS_SH(4) | BYTES_SH(2), Mask: ANYSPACE, Frm: Pack4Words},
	{Type: CHANNELS_SH(4) | BYTES_SH(2) | FLAVOR_SH(1), Mask: ANYSPACE, Frm: Pack4WordsReverse},
	{Type: CHANNELS_SH(4) | BYTES_SH(2) | DOSWAP_SH(1), Mask: ANYSPACE, Frm: Pack4WordsSwap},
	{Type: CHANNELS_SH(4) | BYTES_SH(2) | ENDIAN16_SH(1), Mask: ANYSPACE, Frm: Pack4WordsBigEndian},
	{Type: CHANNELS_SH(6) | BYTES_SH(2), Mask: ANYSPACE, Frm: Pack6Words},
	{Type: CHANNELS_SH(6) | BYTES_SH(2) | DOSWAP_SH(1), Mask: ANYSPACE, Frm: Pack6WordsSwap},
	{Type: BYTES_SH(2), Mask: ANYFLAVOR | ANYSWAPFIRST | ANYSWAP | ANYENDIAN | ANYEXTRA | ANYCHANNELS | ANYSPACE | ANYPREMUL, Frm: PackChunkyWords},
	{Type: BYTES_SH(2) | PLANAR_SH(1), Mask: ANYFLAVOR | ANYENDIAN | ANYSWAP | ANYEXTRA | ANYCHANNELS | ANYSPACE | ANYPREMUL, Frm: PackPlanarWords},
}

// OutputFormattersFloat is the Go equivalent of the C++ array.
var OutputFormattersFloat = []cmsFormattersFloat{
	{Type: TYPE_Lab_FLT, Mask: ANYPLANAR | ANYEXTRA, Frm: PackLabFloatFromFloat},
	{Type: TYPE_XYZ_FLT, Mask: ANYPLANAR | ANYEXTRA, Frm: PackXYZFloatFromFloat},
	{Type: TYPE_Lab_DBL, Mask: ANYPLANAR | ANYEXTRA, Frm: PackLabDoubleFromFloat},
	{Type: TYPE_XYZ_DBL, Mask: ANYPLANAR | ANYEXTRA, Frm: PackXYZDoubleFromFloat},
	{Type: FLOAT_SH(1) | BYTES_SH(4), Mask: ANYPLANAR | ANYFLAVOR | ANYSWAPFIRST | ANYSWAP | ANYEXTRA | ANYCHANNELS | ANYSPACE,
		Frm: PackFloatsFromFloat},
	{Type: FLOAT_SH(1) | BYTES_SH(0), Mask: ANYPLANAR | ANYFLAVOR | ANYSWAPFIRST | ANYSWAP | ANYEXTRA | ANYCHANNELS | ANYSPACE,
		Frm: PackDoublesFromFloat},
	{Type: FLOAT_SH(1) | BYTES_SH(2), Mask: ANYFLAVOR | ANYSWAPFIRST | ANYSWAP | ANYEXTRA | ANYCHANNELS | ANYSPACE,
		Frm: PackHalfFromFloat},
}

func cmsGetStockOutputFormatter(dwInput, dwFlags uint32) cmsFormatter {
	var fr cmsFormatter

	// Optimization is only a hint
	dwInput &= ^OPTIMIZED_SH(1)

	switch dwFlags {
	case CMS_PACK_FLAGS_16BITS:
		for _, f := range OutputFormatters16 {
			if (dwInput & ^f.Mask) == f.Type {
				fr.Fmt16 = f.Frm
				return fr
			}
		}

	case CMS_PACK_FLAGS_FLOAT:
		for _, f := range OutputFormattersFloat {
			if (dwInput & ^f.Mask) == f.Type {
				fr.FmtFloat = f.Frm
				return fr
			}
		}

	default:
		// Do nothing
	}

	fr.Fmt16 = nil // Set to nil if no match is found
	return fr
}

// Structure for formatters factory list
type cmsFormattersFactoryList struct {
	Factory cmsFormatterFactory
	Next    *cmsFormattersFactoryList
}

// Duplicate the zone of memory used by the plugin in the new context
func DupFormatterFactoryList(ctx CmsContext, src CmsContext) {
	var newHead cmsFormattersPluginChunkType
	var previousEntry *cmsFormattersFactoryList
	head := (*cmsFormattersPluginChunkType)((CmsContextStruct)(*src).chunks[FormattersPlugin])

	if head == nil {
		panic("Source context does not contain FormattersPlugin chunk")
	}

	// Walk the list and copy all nodes
	for entry := head.FactoryList; entry != nil; entry = entry.Next {
		newEntry := (*cmsFormattersFactoryList)(cmsSubAllocDup((CmsContextStruct)(*ctx).MemPool, unsafe.Pointer(entry), uint32(unsafe.Sizeof((*cmsFormattersFactoryList)(entry)))))
		if newEntry == nil {
			return
		}

		newEntry.Next = nil
		if previousEntry != nil {
			previousEntry.Next = newEntry
		}

		previousEntry = newEntry

		if newHead.FactoryList == nil {
			newHead.FactoryList = newEntry
		}
	}

	(*ctx).chunks[FormattersPlugin] = cmsSubAllocDup((CmsContextStruct)(*ctx).MemPool, unsafe.Pointer(&newHead), uint32(unsafe.Sizeof(newHead)))
}

// Allocate and initialize the Formatters plugin chunk
func cmsAllocFormattersPluginChunk(ctx CmsContext, src CmsContext) {
	if ctx == nil {
		panic("Context is nil")
	}

	if src != nil {
		// Duplicate the list
		DupFormatterFactoryList(ctx, src)
	} else {
		staticChunk := cmsFormattersPluginChunkType{}
		(*ctx).chunks[FormattersPlugin] = cmsSubAllocDup((CmsContextStruct)(*ctx).MemPool, unsafe.Pointer(&staticChunk), uint32(unsafe.Sizeof(staticChunk)))
	}
}

// Register formatters plugin
func cmsRegisterFormattersPlugin(contextID CmsContext, data *cmsPluginBase) bool {
	ctx := (*cmsFormattersPluginChunkType)((CmsContextStruct)(*contextID).chunks[FormattersPlugin])
	plugin := (*cmsPluginFormatters)(unsafe.Pointer(data))

	if plugin == nil {
		// Reset to built-in defaults
		ctx.FactoryList = nil
		return true
	}
	var list cmsFormattersFactoryList
	newEntry := (*cmsFormattersFactoryList)(cmsPluginMalloc(contextID, uint32(unsafe.Sizeof(list))))

	newEntry.Factory = plugin.FormattersFactory
	newEntry.Next = ctx.FactoryList
	ctx.FactoryList = newEntry

	return true
}

// Get a formatter
func cmsGetFormatter(contextID CmsContext, typeID uint32, direction cmsFormatterDirection, dwFlags uint32) cmsFormatter {
	ctx := (*cmsFormattersPluginChunkType)((CmsContextStruct)(*contextID).chunks[FormattersPlugin])

	if T_CHANNELS(typeID) == 0 {
		return cmsFormatter{} // Return a null formatter
	}

	for entry := ctx.FactoryList; entry != nil; entry = entry.Next {
		formatter := entry.Factory(typeID, direction, dwFlags)
		if formatter.Fmt16 != nil {
			return formatter
		}
	}

	// Revert to default
	if direction == cmsFormatterInput {
		return cmsGetStockInputFormatter(typeID, dwFlags)
	}

	return cmsGetStockOutputFormatter(typeID, dwFlags)
}

// Additional utility functions
func cmsFormatterIsFloat(formatType uint32) bool {
	return T_FLOAT(formatType) != 0
}

func cmsFormatterIs8bit(formatType uint32) bool {
	return T_BYTES(formatType) == 1
}

func cmsFormatterForColorspaceOfProfile(hProfile CmsHPROFILE, nBytes uint32, isFloat bool) uint32 {
	colorSpace := CmsGetColorSpace(hProfile)
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

func cmsFormatterForPCSOfProfile(hProfile CmsHPROFILE, nBytes uint32, isFloat bool) uint32 {
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
