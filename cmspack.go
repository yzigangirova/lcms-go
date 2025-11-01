package golcms

import (
	"encoding/binary"
	//"errors"

	"bytes"

	//"fmt"
	"math"
	"unsafe"

	"github.com/yzigangirova/lcms-go/mem"
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

func getF32(accum []uint8, offset int) float32 {
	if offset+4 > len(accum) {
		return 0.0
	}
	return math.Float32frombits(binary.LittleEndian.Uint32(accum[offset : offset+4]))
}

func getF64(accum []uint8, offset int) float64 {
	if offset+8 > len(accum) {
		return 0
	}
	return math.Float64frombits(binary.LittleEndian.Uint64(accum[offset : offset+8]))
}

// Unpacking routines (16 bits) ----------------------------------------------------------------------------------------
func UnrollChunkyBytes(mm mem.Manager, info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	//fmt.Println("UnrollChunkyBytes")
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

		v := uint32(FROM_8_TO_16(accum[0]))
		if reverse != 0 {
			v = uint32(REVERSE_FLAVOR_16(uint16(v)))
		}

		if premul != 0 && alphaFactor > 0 {
			v = (v << 16) / alphaFactor
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
func UnrollPlanarBytes(mm mem.Manager,info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	//fmt.Println("UnrollPlanarBytes")
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

		v := uint32(FROM_8_TO_16(accum[0]))

		if reverse != 0 {
			v = uint32(REVERSE_FLAVOR_16(uint16(v)))
		}

		if premul != 0 && alphaFactor > 0 {
			v = (v << 16) / alphaFactor
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
func Unroll4Bytes(mm mem.Manager,info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	wIn[0] = FROM_8_TO_16(accum[0]) // C
	wIn[1] = FROM_8_TO_16(accum[1]) // M
	wIn[2] = FROM_8_TO_16(accum[2]) // Y
	wIn[3] = FROM_8_TO_16(accum[3]) // K
	return accum[4:]
}

// Unroll4BytesReverse processes 4 bytes with reverse flavor applied.
func Unroll4BytesReverse(mm mem.Manager,info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	wIn[0] = FROM_8_TO_16(REVERSE_FLAVOR_8(accum[0])) // C
	wIn[1] = FROM_8_TO_16(REVERSE_FLAVOR_8(accum[1])) // M
	wIn[2] = FROM_8_TO_16(REVERSE_FLAVOR_8(accum[2])) // Y
	wIn[3] = FROM_8_TO_16(REVERSE_FLAVOR_8(accum[3])) // K
	return accum[4:]
}

// Unroll4BytesSwapFirst processes 4 bytes with the first byte swapped to the last position.
func Unroll4BytesSwapFirst(mm mem.Manager,info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	wIn[3] = FROM_8_TO_16(accum[0]) // K
	wIn[0] = FROM_8_TO_16(accum[1]) // C
	wIn[1] = FROM_8_TO_16(accum[2]) // M
	wIn[2] = FROM_8_TO_16(accum[3]) // Y
	return accum[4:]
}

// Unroll4BytesSwap processes 4 bytes in KYMC order.
func Unroll4BytesSwap(mm mem.Manager,info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	wIn[3] = FROM_8_TO_16(accum[0]) // K
	wIn[2] = FROM_8_TO_16(accum[1]) // Y
	wIn[1] = FROM_8_TO_16(accum[2]) // M
	wIn[0] = FROM_8_TO_16(accum[3]) // C
	return accum[4:]
}

// Unroll4BytesSwapSwapFirst processes 4 bytes in swapped order with the first byte swapped.
func Unroll4BytesSwapSwapFirst(mm mem.Manager,info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	wIn[2] = FROM_8_TO_16(accum[0]) // K
	wIn[1] = FROM_8_TO_16(accum[1]) // Y
	wIn[0] = FROM_8_TO_16(accum[2]) // M
	wIn[3] = FROM_8_TO_16(accum[3]) // C
	return accum[4:]
}

// Unroll3Bytes processes 3 bytes in RGB order.
func Unroll3Bytes(mm mem.Manager,info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	wIn[0] = FROM_8_TO_16(accum[0]) // R
	wIn[1] = FROM_8_TO_16(accum[1]) // G
	wIn[2] = FROM_8_TO_16(accum[2]) // B
	return accum[3:]
}

// Unroll3BytesSkip1Swap processes 3 bytes, skips 1 (A), and swaps to BRG order.
func Unroll3BytesSkip1Swap(mm mem.Manager,info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	accum = accum[1:]               // Skip A
	wIn[2] = FROM_8_TO_16(accum[0]) // B
	wIn[1] = FROM_8_TO_16(accum[1]) // G
	wIn[0] = FROM_8_TO_16(accum[2]) // R
	return accum[3:]
}

// Unroll3BytesSkip1SwapSwapFirst processes 3 bytes, skips 1 (A), and swaps to BRG order with the first byte swapped.
func Unroll3BytesSkip1SwapSwapFirst(mm mem.Manager,info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	wIn[2] = FROM_8_TO_16(accum[0]) // B
	wIn[1] = FROM_8_TO_16(accum[1]) // G
	wIn[0] = FROM_8_TO_16(accum[2]) // R
	accum = accum[4:]               // Skip 1 (A) and move forward
	return accum
}

// Unroll3BytesSkip1SwapFirst processes 3 bytes, skips 1 (A), and places R first.
func Unroll3BytesSkip1SwapFirst(mm mem.Manager,info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	accum = accum[1:]               // Skip A
	wIn[0] = FROM_8_TO_16(accum[0]) // R
	wIn[1] = FROM_8_TO_16(accum[1]) // G
	wIn[2] = FROM_8_TO_16(accum[2]) // B
	return accum[3:]
}

// Unroll3BytesSwap processes 3 bytes in BRG order.
func Unroll3BytesSwap(mm mem.Manager,info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	wIn[2] = FROM_8_TO_16(accum[0]) // B
	wIn[1] = FROM_8_TO_16(accum[1]) // G
	wIn[0] = FROM_8_TO_16(accum[2]) // R
	return accum[3:]
}

// UnrollLabV2_8 processes Lab values from 8-bit input and converts them to 16-bit.
func UnrollLabV2_8(mm mem.Manager,info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	//fmt.Println("UnrollLabV2_8")
	wIn[0] = FromLabV2ToLabV4(FROM_8_TO_16(accum[0])) // L
	wIn[1] = FromLabV2ToLabV4(FROM_8_TO_16(accum[1])) // a
	wIn[2] = FromLabV2ToLabV4(FROM_8_TO_16(accum[2])) // b
	return accum[3:]
}

// UnrollALabV2_8 processes ALab values from 8-bit input, skipping the alpha channel.
func UnrollALabV2_8(mm mem.Manager,info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	//fmt.Println("UnrollALabV2_8")
	accum = accum[1:]                                 // Skip alpha
	wIn[0] = FromLabV2ToLabV4(FROM_8_TO_16(accum[0])) // L
	wIn[1] = FromLabV2ToLabV4(FROM_8_TO_16(accum[1])) // a
	wIn[2] = FromLabV2ToLabV4(FROM_8_TO_16(accum[2])) // b
	return accum[3:]
}

// UnrollLabV2_16 processes Lab values from 16-bit input.
func UnrollLabV2_16(mm mem.Manager,info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	//fmt.Println("UnrollLabV2_16")
	wIn[0] = FromLabV2ToLabV4(uint16(accum[0]) | uint16(accum[1])<<8) // L
	accum = accum[2:]
	wIn[1] = FromLabV2ToLabV4(uint16(accum[0]) | uint16(accum[1])<<8) // a
	accum = accum[2:]
	wIn[2] = FromLabV2ToLabV4(uint16(accum[0]) | uint16(accum[1])<<8) // b
	return accum[2:]
}

// Unroll2Bytes processes duplex values from 8-bit input.
func Unroll2Bytes(mm mem.Manager,info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	//fmt.Println("Unroll2Bytes")
	wIn[0] = FROM_8_TO_16(accum[0]) // ch1
	wIn[1] = FROM_8_TO_16(accum[1]) // ch2
	return accum[2:]
}

// Unroll1Byte duplicates L into RGB channels for monochrome data.
func Unroll1Byte(mm mem.Manager,info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	//fmt.Println("Unroll1Byte")
	l := FROM_8_TO_16(accum[0])
	wIn[0], wIn[1], wIn[2] = l, l, l // L
	return accum[1:]
}

// Unroll1ByteSkip1 processes monochrome data, skipping one channel.
func Unroll1ByteSkip1(mm mem.Manager,info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	//fmt.Println("Unroll1ByteSkip1")
	l := FROM_8_TO_16(accum[0])
	wIn[0], wIn[1], wIn[2] = l, l, l // L
	return accum[2:]
}

// Unroll1ByteSkip2 processes monochrome data, skipping two channels.
func Unroll1ByteSkip2(mm mem.Manager,info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	//fmt.Println("Unroll1ByteSkip2")
	l := FROM_8_TO_16(accum[0])
	wIn[0], wIn[1], wIn[2] = l, l, l // L
	return accum[3:]
}

// Unroll1ByteReversed processes monochrome data with reversed flavor.
func Unroll1ByteReversed(mm mem.Manager,info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	//fmt.Println("Unroll1ByteReversed")
	l := REVERSE_FLAVOR_16(FROM_8_TO_16(accum[0]))
	wIn[0], wIn[1], wIn[2] = l, l, l // L
	return accum[1:]
}

func UnrollAnyWords(mm mem.Manager,info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	//fmt.Println("UnrollAnyWords")
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

func UnrollAnyWordsPremul(mm mem.Manager,info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	//fmt.Println("UnrollAnyWordsPremul")
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

		v := uint32(uint16(accum[0]) | (uint16(accum[1]) << 8))

		if swapEndian != 0 {
			v = uint32(CHANGE_ENDIAN(uint16(v)))
		}

		if alphaFactor > 0 {
			v = (v << 16) / uint32(alphaFactor)
			if v > 0xffff {
				v = 0xffff
			}
		}

		if reverse != 0 {
			v = uint32(REVERSE_FLAVOR_16(uint16(v)))
		}

		wIn[index] = uint16(v)
		accum = accum[2:]
	}

	return accum
}

func UnrollPlanarWords(mm mem.Manager,info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	//fmt.Println("UnrollPlanarWords")
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

func UnrollPlanarWordsPremul(mm mem.Manager,info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	//fmt.Println("UnrollPlanarWordsPremul")
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

		v := uint32(accum[0]) | (uint32(accum[1]) << 8)

		if swapEndian != 0 {
			v = uint32(CHANGE_ENDIAN(uint16(v)))
		}

		if alphaFactor > 0 {
			v = (v << 16) / alphaFactor
			if v > 0xffff {
				v = 0xffff
			}
		}

		if reverse != 0 {
			v = uint32(REVERSE_FLAVOR_16(uint16(v)))
		}

		wIn[index] = uint16(v)
		accum = accum[stride:]
	}

	return accum[:]
}
func Unroll4Words(mm mem.Manager,info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	//fmt.Println("Unroll4Words")
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

func Unroll4WordsReverse(mm mem.Manager,info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	//fmt.Println("Unroll4WordsReverse")
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

func Unroll4WordsSwapFirst(mm mem.Manager,info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	//fmt.Println("Unroll4WordsSwapFirst")
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

func Unroll4WordsSwap(mm mem.Manager,info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	//fmt.Println("Unroll4WordsSwap")
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

func Unroll4WordsSwapSwapFirst(mm mem.Manager,info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	//fmt.Println("Unroll4WordsSwapSwapFirst")
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
func Unroll3Words(mm mem.Manager,info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	//fmt.Println("Unroll3Words")
	wIn[0] = uint16(accum[0]) | (uint16(accum[1]) << 8) // C R
	accum = accum[2:]
	wIn[1] = uint16(accum[0]) | (uint16(accum[1]) << 8) // M G
	accum = accum[2:]
	wIn[2] = uint16(accum[0]) | (uint16(accum[1]) << 8) // Y B
	accum = accum[2:]
	return accum
}

func Unroll3WordsSwap(mm mem.Manager,info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	//fmt.Println("Unroll3WordsSwap")
	wIn[2] = uint16(accum[0]) | (uint16(accum[1]) << 8) // C R
	accum = accum[2:]
	wIn[1] = uint16(accum[0]) | (uint16(accum[1]) << 8) // M G
	accum = accum[2:]
	wIn[0] = uint16(accum[0]) | (uint16(accum[1]) << 8) // Y B
	accum = accum[2:]
	return accum
}

func Unroll3WordsSkip1Swap(mm mem.Manager,info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	//fmt.Println("Unroll3WordsSkip1Swap")
	accum = accum[2:]                                   // Skip A
	wIn[2] = uint16(accum[0]) | (uint16(accum[1]) << 8) // R
	accum = accum[2:]
	wIn[1] = uint16(accum[0]) | (uint16(accum[1]) << 8) // G
	accum = accum[2:]
	wIn[0] = uint16(accum[0]) | (uint16(accum[1]) << 8) // B
	accum = accum[2:]
	return accum
}

func Unroll3WordsSkip1SwapFirst(mm mem.Manager,info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	//fmt.Println("Unroll3WordsSkip1SwapFirst")
	accum = accum[2:]                                   // Skip A
	wIn[0] = uint16(accum[0]) | (uint16(accum[1]) << 8) // R
	accum = accum[2:]
	wIn[1] = uint16(accum[0]) | (uint16(accum[1]) << 8) // G
	accum = accum[2:]
	wIn[2] = uint16(accum[0]) | (uint16(accum[1]) << 8) // B
	accum = accum[2:]
	return accum
}

func Unroll1Word(mm mem.Manager,info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	//fmt.Println("Unroll1Words")
	word := uint16(accum[0]) | (uint16(accum[1]) << 8)
	wIn[0], wIn[1], wIn[2] = word, word, word // L duplicated to RGB
	accum = accum[2:]
	return accum
}

func Unroll1WordReversed(mm mem.Manager,info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	//fmt.Println("Unroll1WordsReversed")
	word := REVERSE_FLAVOR_16(uint16(accum[0]) | (uint16(accum[1]) << 8))
	wIn[0], wIn[1], wIn[2] = word, word, word // L reversed and duplicated to RGB
	accum = accum[2:]
	return accum
}

func Unroll1WordSkip3(mm mem.Manager,info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	//fmt.Println("Unroll1WordSkip3")
	word := uint16(accum[0]) | (uint16(accum[1]) << 8)
	wIn[0], wIn[1], wIn[2] = word, word, word // L duplicated to RGB
	accum = accum[8:]                         // Skip 3 words
	return accum
}
func Unroll2Words(mm mem.Manager,info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	//fmt.Println("Unroll2Words")
	wIn[0] = uint16(accum[0]) | (uint16(accum[1]) << 8) // ch1
	accum = accum[2:]
	wIn[1] = uint16(accum[0]) | (uint16(accum[1]) << 8) // ch2
	accum = accum[2:]
	return accum
}

func UnrollLabDoubleTo16(mm mem.Manager, info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	//fmt.Println("UnrollLabDoubleTo16")
	if T_PLANAR(info.InputFormat) != 0 {
		if len(accum) < int(stride*2+1) {
			return accum // not enough data
		}

		Lab := cmsCIELab{
			L: float64(accum[0]),
			a: float64(accum[stride]),
			b: float64(accum[stride*2]),
		}
		cmsFloat2LabEncoded(wIn, &Lab)
		return accum[8:]
	} else {
		// interpret accum[0:24] as cmsCIELab in float64 form
		if len(accum) < int(unsafe.Sizeof(cmsCIELab{})) {
			return accum // not enough data
		}

		var Lab cmsCIELab
		buf := bytes.NewReader(accum[:24])
		_ = binary.Read(buf, binary.LittleEndian, &Lab)
		if len(wIn) < 3 {
			cmsSignalError(nil, cmsERROR_UNDEFINED, "wIn lenght is less than 3")
			return accum
		}
		cmsFloat2LabEncoded(wIn, &Lab)

		extra := T_EXTRA(info.InputFormat)
		return accum[24+int(extra)*8:]
	}
}

func UnrollLabFloatTo16(mm mem.Manager,info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	//fmt.Println("UnrollLabFloatTo16")
	var Lab cmsCIELab

	if T_PLANAR(info.InputFormat) != 0 {
		posL := accum
		posa := accum[stride:]
		posb := accum[stride*2:]

		Lab.L = (float64)(posL[0])
		Lab.a = (float64)(posa[0])
		Lab.b = (float64)(posb[0])

		if len(wIn) < 3 {
			cmsSignalError(nil, cmsERROR_UNDEFINED, "wIn lenght is less than 3")
			return accum
		}
		cmsFloat2LabEncoded(wIn, &Lab)
		return accum[4:] // sizeof(float32)
	} else {
		Lab.L = (float64)(accum[0])
		Lab.a = (float64)(accum[4])
		Lab.b = (float64)(accum[8])

		if len(wIn) < 3 {
			cmsSignalError(nil, cmsERROR_UNDEFINED, "wIn lenght is less than 3")
			return accum
		}
		cmsFloat2LabEncoded(wIn, &Lab)
		extra := T_EXTRA(info.InputFormat)
		accum = accum[(3+extra)*4:] // 3 components + extra
		return accum
	}
}

func UnrollXYZDoubleTo16(mm mem.Manager, info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	//fmt.Println("UnrollXYZDoubleTo16")
	var XYZ cmsCIEXYZ

	if T_PLANAR(info.InputFormat) != 0 {
		readFloat64 := func(b []uint8) float64 {
			bits := binary.LittleEndian.Uint64(b)
			return math.Float64frombits(bits)
		}

		XYZ.X = readFloat64(accum[0:])
		XYZ.Y = readFloat64(accum[stride:])
		XYZ.Z = readFloat64(accum[stride*2:])

		cmsFloat2XYZEncoded((*[3]uint16)(wIn), &XYZ)
		return accum[8:]
	} else {
		if len(accum) < 24 {
			// insufficient data
			return accum
		}
		XYZ.X = math.Float64frombits(binary.LittleEndian.Uint64(accum[0:8]))
		XYZ.Y = math.Float64frombits(binary.LittleEndian.Uint64(accum[8:16]))
		XYZ.Z = math.Float64frombits(binary.LittleEndian.Uint64(accum[16:24]))

		cmsFloat2XYZEncoded((*[3]uint16)(wIn), &XYZ)

		extra := T_EXTRA(info.InputFormat)
		return accum[24+extra*8:]
	}
}

func UnrollXYZFloatTo16(mm mem.Manager, info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	//fmt.Println("UnrollXYZFloatTo16")
	var XYZ cmsCIEXYZ

	if T_PLANAR(info.InputFormat) != 0 {
		readFloat32 := func(b []uint8) float64 {
			if len(b) < 4 {
				return 0
			}
			bits := binary.LittleEndian.Uint32(b)
			return float64(math.Float32frombits(bits))
		}

		XYZ.X = readFloat32(accum[0:])
		XYZ.Y = readFloat32(accum[stride:])
		XYZ.Z = readFloat32(accum[stride*2:])

		cmsFloat2XYZEncoded((*[3]uint16)(wIn), &XYZ)

		return accum[4:] // sizeof(float32)
	} else {
		if len(accum) < 12 {
			return accum // not enough data
		}

		XYZ.X = float64(math.Float32frombits(binary.LittleEndian.Uint32(accum[0:4])))
		XYZ.Y = float64(math.Float32frombits(binary.LittleEndian.Uint32(accum[4:8])))
		XYZ.Z = float64(math.Float32frombits(binary.LittleEndian.Uint32(accum[8:12])))

		cmsFloat2XYZEncoded((*[3]uint16)(wIn), &XYZ)

		extra := T_EXTRA(info.InputFormat)
		return accum[12+extra*4:]
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

func UnrollDoubleTo16(mm mem.Manager, info *cmsTRANSFORM, wIn []uint16, accum []uint8, Stride uint32) []uint8 {
	//fmt.Println("UnrollDoubleTo16")
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
func UnrollFloatTo16(mm mem.Manager, info *cmsTRANSFORM, wIn []uint16, accum []uint8, Stride uint32) []uint8 {
	//fmt.Println("UnrollFloatTo16")
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
func UnrollDouble1Chan(mm mem.Manager, info *cmsTRANSFORM, wIn []uint16, accum []uint8, Stride uint32) []uint8 {
	//fmt.Println("UnrollDouble1Chan")
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

func Unroll8ToFloat(mm mem.Manager, info *cmsTRANSFORM, wIn []float32, accum []uint8, Stride uint32) []uint8 {
	//fmt.Println("Unroll8ToFloat")
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
func Unroll16ToFloat(mm mem.Manager, info *cmsTRANSFORM, wIn []float32, accum []uint8, Stride uint32) []uint8 {
	//fmt.Println("Unroll16ToFloat")
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
func UnrollFloatsToFloat(mm mem.Manager, info *cmsTRANSFORM, wIn []float32, accum []uint8, Stride uint32) []uint8 {
	//fmt.Println("UnrollFloatsToFloat")
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

func UnrollDoublesToFloat(mm mem.Manager, info *cmsTRANSFORM, wIn []float32, accum []uint8, Stride uint32) []uint8 {
	//fmt.Println("UnrollDoublesToFloat")
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

	if Premul != 0 && Extra > 0 {
		if Planar != 0 {
			if ExtraFirst != 0 {
				alphaFactor = getF64(accum, 0) / maximum
			} else {
				alphaFactor = getF64(accum, int(nChan*Stride*8)) / maximum
			}
		} else {
			if ExtraFirst != 0 {
				alphaFactor = getF64(accum, 0) / maximum
			} else {
				alphaFactor = getF64(accum, int(nChan*8)) / maximum
			}
		}
	}

	if ExtraFirst != 0 {
		start = Extra
	}

	for i := uint32(0); i < nChan; i++ {
		index := i
		if DoSwap != 0 {
			index = nChan - i - 1
		}

		var val float64
		if Planar != 0 {
			val = getF64(accum, int((i+start)*Stride*8))
		} else {
			val = getF64(accum, int((i+start)*8))
		}

		if Premul != 0 && alphaFactor > 0 {
			val /= alphaFactor
		}

		val /= maximum
		wIn[index] = float32(ReverseFloat(float32(val), Reverse != 0))
	}

	if Extra == 0 && SwapFirst != 0 {
		SwapFirstFloat(wIn, nChan)
	}

	if Planar != 0 {
		return accum[8:]
	}
	return accum[int((nChan+Extra)*8):]
}

func UnrollLabDoubleToFloat(mm mem.Manager, info *cmsTRANSFORM, wIn []float32, accum []uint8, stride uint32) []uint8 {
	//fmt.Println("UnrollLabDoubleToFloat")

	if T_PLANAR(info.InputFormat) != 0 {
		stride /= PixelSize(info.InputFormat)

		wIn[0] = float32(getF64(accum, 0) / 100.0)
		wIn[1] = float32((getF64(accum, int(stride*8)) + 128.0) / 255.0)
		wIn[2] = float32((getF64(accum, int(stride*2*8)) + 128.0) / 255.0)

		return accum[8:]
	}

	wIn[0] = float32(getF64(accum, 0) / 100.0)
	wIn[1] = float32((getF64(accum, 8) + 128.0) / 255.0)
	wIn[2] = float32((getF64(accum, 16) + 128.0) / 255.0)
	/*fmt.Printf("wIn[0] %.7f\n", wIn[0])
	fmt.Printf("wIn[1] %.7f\n", wIn[1])
	fmt.Printf("wIn[2] %.7f\n", wIn[2])*/

	extra := T_EXTRA(info.InputFormat)
	return accum[int((3+extra)*8):]
}

func UnrollLabFloatToFloat(mm mem.Manager, info *cmsTRANSFORM, wIn []float32, accum []uint8, stride uint32) []uint8 {
	//fmt.Println("UnrollLabFloatToFloat")

	if T_PLANAR(info.InputFormat) != 0 {
		stride /= PixelSize(info.InputFormat)

		wIn[0] = getF32(accum, 0) / 100.0
		wIn[1] = (getF32(accum, int(stride*4)) + 128.0) / 255.0
		wIn[2] = (getF32(accum, int(stride*2*4)) + 128.0) / 255.0

		return accum[4:]
	}

	wIn[0] = getF32(accum, 0) / 100.0
	wIn[1] = (getF32(accum, 4) + 128.0) / 255.0
	wIn[2] = (getF32(accum, 8) + 128.0) / 255.0

	extra := T_EXTRA(info.InputFormat)
	return accum[int((3+extra)*4):]
}

func UnrollXYZDoubleToFloat(mm mem.Manager, info *cmsTRANSFORM, wIn []float32, accum []uint8, stride uint32) []uint8 {
	//fmt.Println("UnrollXYZDoubleToFloat")

	if T_PLANAR(info.InputFormat) != 0 {
		stride /= PixelSize(info.InputFormat)

		wIn[0] = float32(getF64(accum, 0) / MAX_ENCODEABLE_XYZ)
		wIn[1] = float32(getF64(accum, int(stride*8)) / MAX_ENCODEABLE_XYZ)
		wIn[2] = float32(getF64(accum, int(stride*2*8)) / MAX_ENCODEABLE_XYZ)

		return accum[8:]
	}

	wIn[0] = float32(getF64(accum, 0) / MAX_ENCODEABLE_XYZ)
	wIn[1] = float32(getF64(accum, 8) / MAX_ENCODEABLE_XYZ)
	wIn[2] = float32(getF64(accum, 16) / MAX_ENCODEABLE_XYZ)

	extra := T_EXTRA(info.InputFormat)
	return accum[int((3+extra)*8):]
}

func UnrollXYZFloatToFloat(mm mem.Manager, info *cmsTRANSFORM, wIn []float32, accum []uint8, stride uint32) []uint8 {
	//fmt.Println("UnrollXYZFloatToFloat")

	if T_PLANAR(info.InputFormat) != 0 {
		stride /= PixelSize(info.InputFormat)

		wIn[0] = float32(float64(getF32(accum, 0)) / MAX_ENCODEABLE_XYZ)
		wIn[1] = float32(float64(getF32(accum, int(stride)*4)) / MAX_ENCODEABLE_XYZ)
		wIn[2] = float32(float64(getF32(accum, int(stride)*2*4)) / MAX_ENCODEABLE_XYZ)

		return accum[4:]
	}

	wIn[0] = float32(float64(getF32(accum, 0)) / MAX_ENCODEABLE_XYZ)
	wIn[1] = float32(float64(getF32(accum, 4)) / MAX_ENCODEABLE_XYZ)
	wIn[2] = float32(float64(getF32(accum, 8)) / MAX_ENCODEABLE_XYZ)

	extra := T_EXTRA(info.InputFormat)
	return accum[int((3+extra)*4):]
}

func lab4toFloat(wIn []float32, lab4 [3]uint16) {
	L := float32(lab4[0]) / 655.35
	a := (float32(lab4[1]) / 257.0) - 128.0
	b := (float32(lab4[2]) / 257.0) - 128.0

	wIn[0] = L / 100.0           // from 0..100 to 0..1
	wIn[1] = (a + 128.0) / 255.0 // from -128..+127 to 0..1
	wIn[2] = (b + 128.0) / 255.0
}
func UnrollLabV2_8ToFloat(mm mem.Manager, info *cmsTRANSFORM, wIn []float32, accum []uint8, Stride uint32) []uint8 {
	//fmt.Println("UnrollLabV2_8ToFloat")
	lab4 := [3]uint16{
		FromLabV2ToLabV4(FROM_8_TO_16(accum[0])),
		FromLabV2ToLabV4(FROM_8_TO_16(accum[1])),
		FromLabV2ToLabV4(FROM_8_TO_16(accum[2])),
	}

	lab4toFloat(wIn, lab4)

	return accum[3:]
}
func UnrollALabV2_8ToFloat(mm mem.Manager, info *cmsTRANSFORM, wIn []float32, accum []uint8, Stride uint32) []uint8 {
	//fmt.Println("UnrollALabV2_8ToFloat")
	lab4 := [3]uint16{
		FromLabV2ToLabV4(FROM_8_TO_16(accum[1])),
		FromLabV2ToLabV4(FROM_8_TO_16(accum[2])),
		FromLabV2ToLabV4(FROM_8_TO_16(accum[3])),
	}

	lab4toFloat(wIn, lab4)

	return accum[4:]
}

func UnrollLabV2_16ToFloat(mm mem.Manager, info *cmsTRANSFORM, wIn []float32, accum []uint8, stride uint32) []uint8 {
	//fmt.Println("UnrollLabV2_16ToFloat")
	if len(accum) < 6 {
		return accum // Not enough data
	}

	// Interpret input bytes as big-endian uint16 values
	lab4 := [3]uint16{
		FromLabV2ToLabV4(binary.LittleEndian.Uint16(accum[0:2])),
		FromLabV2ToLabV4(binary.LittleEndian.Uint16(accum[2:4])),
		FromLabV2ToLabV4(binary.LittleEndian.Uint16(accum[4:6])),
	}

	lab4toFloat(wIn, lab4)

	return accum[6:]
}

func PackChunkyBytes(mm mem.Manager, info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	//fmt.Println("PackChunkyBytes")
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
func PackChunkyWords(mm mem.Manager, info *cmsTRANSFORM, wOut []uint16, output []byte, stride uint32) []byte {
	//fmt.Println("PackChunkyWords")
	nChan := T_CHANNELS(info.OutputFormat)
	swapEndian := T_ENDIAN16(info.OutputFormat)
	doSwap := T_DOSWAP(info.OutputFormat)
	reverse := T_FLAVOR(info.OutputFormat)
	extra := T_EXTRA(info.OutputFormat)
	swapFirst := T_SWAPFIRST(info.OutputFormat)
	premul := T_PREMUL(info.OutputFormat)
	extraFirst := doSwap ^ swapFirst

	alphaFactor := uint32(0)

	expectedWords := int(nChan + extra)
	if len(output) < expectedWords*2 {
		return output // Not enough space
	}

	// Create a temporary buffer to hold packed words before writing to byte slice
	packed := make([]uint16, 0, expectedWords)

	// Handle premultiplied alpha
	if extraFirst != 0 {
		if premul != 0 && extra != 0 {
			alphaFactor = uint32(cmsToFixedDomain(int(wOut[0])))
		}
		wOut = wOut[extra:]
	} else {
		if premul != 0 && extra != 0 {
			alphaFactor = uint32(cmsToFixedDomain(int(wOut[nChan])))
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

		packed = append(packed, v)
	}

	// Add extra channels (alpha or others)
	if extraFirst != 0 {
		packed = append(packed, wOut[:extra]...)
	} else {
		packed = append(packed, wOut[nChan:nChan+extra]...)
	}

	// Swap first if needed and no extra channels
	if extra == 0 && swapFirst != 0 {
		// Rotate left, moving last to front
		last := packed[nChan-1]
		copy(packed[1:], packed[:nChan-1])
		packed[0] = last
	}

	// Write packed uint16s to output []byte
	for i := 0; i < len(packed); i++ {
		if i*2+1 >= len(output) {
			break
		}
		binary.LittleEndian.PutUint16(output[i*2:], packed[i])
	}

	return output[:len(packed)*2]
}

func PackPlanarBytes(mm mem.Manager, info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	//fmt.Println("PackPlanarBytes")
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
func PackPlanarWords(mm mem.Manager, info *cmsTRANSFORM, wOut []uint16, output []byte, stride uint32) []byte {
	//fmt.Println("PackPlanarWords")
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

func Pack6Bytes(mm mem.Manager, info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	output[0] = FROM_16_TO_8(wOut[0])
	output[1] = FROM_16_TO_8(wOut[1])
	output[2] = FROM_16_TO_8(wOut[2])
	output[3] = FROM_16_TO_8(wOut[3])
	output[4] = FROM_16_TO_8(wOut[4])
	output[5] = FROM_16_TO_8(wOut[5])

	return output[6:]
}
func Pack6BytesSwap(mm mem.Manager, info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	output[0] = FROM_16_TO_8(wOut[5])
	output[1] = FROM_16_TO_8(wOut[4])
	output[2] = FROM_16_TO_8(wOut[3])
	output[3] = FROM_16_TO_8(wOut[2])
	output[4] = FROM_16_TO_8(wOut[1])
	output[5] = FROM_16_TO_8(wOut[0])

	return output[6:]
}

func Pack6Words(mm mem.Manager, info *cmsTRANSFORM, wOut []uint16, output []uint8, stride uint32) []uint8 {
	//fmt.Println("Pack6Words")

	if len(output) < 12 || len(wOut) < 6 {
		return output
	}

	for i := 0; i < 6; i++ {
		binary.LittleEndian.PutUint16(output[i*2:], wOut[i])
	}

	return output[12:]
}
func Pack6WordsSwap(mm mem.Manager, info *cmsTRANSFORM, wOut []uint16, output []uint8, stride uint32) []uint8 {
	////fmt.Println("Pack6WordsSwap")
	if len(output) < 12 || len(wOut) < 6 {
		return output
	}

	for i := 0; i < 6; i++ {
		binary.LittleEndian.PutUint16(output[i*2:], wOut[5-i])
	}

	return output[12:]
}

func Pack4Bytes(mm mem.Manager, info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	////fmt.Println("Pack4Bytes")
	for i := 0; i < 4; i++ {
		//fmt.Printf("wOut[i] %d\n", wOut[i])
		output[i] = FROM_16_TO_8(wOut[i])
		//fmt.Printf("output[i] %d\n",output[i])
	}
	return output[4:]
}
func Pack4BytesReverse(mm mem.Manager, info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	////fmt.Println("Pack4BytesReverse")
	for i := 0; i < 4; i++ {
		output[i] = REVERSE_FLAVOR_8(FROM_16_TO_8(wOut[i]))
	}
	return output[4:]
}
func Pack4BytesSwapFirst(mm mem.Manager, info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	output[0] = FROM_16_TO_8(wOut[3])
	output[1] = FROM_16_TO_8(wOut[0])
	output[2] = FROM_16_TO_8(wOut[1])
	output[3] = FROM_16_TO_8(wOut[2])

	return output[4:]
}
func Pack4BytesSwap(mm mem.Manager, info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	for i := 0; i < 4; i++ {
		output[i] = FROM_16_TO_8(wOut[3-i])
	}
	return output[4:]
}
func Pack4BytesSwapSwapFirst(mm mem.Manager, info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	output[0] = FROM_16_TO_8(wOut[2])
	output[1] = FROM_16_TO_8(wOut[1])
	output[2] = FROM_16_TO_8(wOut[0])
	output[3] = FROM_16_TO_8(wOut[3])

	return output[4:]
}

func Pack4Words(mm mem.Manager, info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	//fmt.Println("Pack4Words")
	if len(output) < 8 || len(wOut) < 4 {
		return output
	}
	for i := 0; i < 4; i++ {
		binary.LittleEndian.PutUint16(output[i*2:], wOut[i])
	}
	return output[8:]
}
func Pack4WordsReverse(mm mem.Manager, info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	//fmt.Println("Pack4WordsReverse")
	if len(output) < 8 || len(wOut) < 4 {
		return output
	}
	for i := 0; i < 4; i++ {
		reversed := REVERSE_FLAVOR_16(wOut[i])
		binary.LittleEndian.PutUint16(output[i*2:], reversed)
	}
	return output[8:]
}
func Pack4WordsSwap(mm mem.Manager, info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	//fmt.Println("Pack4WordsSwap")
	if len(output) < 8 || len(wOut) < 4 {
		return output
	}
	for i := 0; i < 4; i++ {
		binary.LittleEndian.PutUint16(output[i*2:], wOut[3-i])
	}
	return output[8:]
}
func Pack4WordsBigEndian(mm mem.Manager, info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	//fmt.Println("Pack4WordsBigEndian")
	if len(output) < 8 || len(wOut) < 4 {
		return output
	}
	for i := 0; i < 4; i++ {
		binary.BigEndian.PutUint16(output[i*2:], wOut[i])
	}
	return output[8:]
}

func PackLabV2_8(mm mem.Manager, info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	output[0] = FROM_16_TO_8(FromLabV4ToLabV2(wOut[0]))
	output[1] = FROM_16_TO_8(FromLabV4ToLabV2(wOut[1]))
	output[2] = FROM_16_TO_8(FromLabV4ToLabV2(wOut[2]))

	return output[3:]
}
func PackALabV2_8(mm mem.Manager, info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	output[0] = 0 // Placeholder for alpha channel
	output[1] = FROM_16_TO_8(FromLabV4ToLabV2(wOut[0]))
	output[2] = FROM_16_TO_8(FromLabV4ToLabV2(wOut[1]))
	output[3] = FROM_16_TO_8(FromLabV4ToLabV2(wOut[2]))

	return output[4:]
}

func PackLabV2_16(mm mem.Manager, info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	//fmt.Println("PackLabV2_16")
	binary.LittleEndian.PutUint16(output[0:2], FromLabV4ToLabV2(wOut[0]))
	binary.LittleEndian.PutUint16(output[2:4], FromLabV4ToLabV2(wOut[1]))
	binary.LittleEndian.PutUint16(output[4:6], FromLabV4ToLabV2(wOut[2]))
	return output[6:]
}
func Pack3Bytes(mm mem.Manager, info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	//fmt.Println("Pack3Bytes")
	/*	fmt.Println("wOut[0]", wOut[0])
		fmt.Println("wOut[1]", wOut[1])
		fmt.Println("wOut[2]", wOut[2])*/
	output[0] = FROM_16_TO_8(wOut[0])
	output[1] = FROM_16_TO_8(wOut[1])
	output[2] = FROM_16_TO_8(wOut[2])
	/*	fmt.Println("output[0]", output[0])
		fmt.Println("output[1]", output[1])
		fmt.Println("output[2]", output[2])*/

	return output[3:]
}
func Pack3BytesOptimized(mm mem.Manager, info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	//fmt.Println("Pack3BytesOptimized")
	output[0] = uint8(wOut[0] & 0xFF)
	output[1] = uint8(wOut[1] & 0xFF)
	output[2] = uint8(wOut[2] & 0xFF)

	return output[3:]
}
func Pack3BytesSwap(mm mem.Manager, info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	//fmt.Println("Pack3BytesSwap")
	output[0] = FROM_16_TO_8(wOut[2])
	output[1] = FROM_16_TO_8(wOut[1])
	output[2] = FROM_16_TO_8(wOut[0])

	return output[3:]
}
func Pack3BytesSwapOptimized(mm mem.Manager, info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	//fmt.Println("Pack3BytesSwapOptimized")
	output[0] = uint8(wOut[2] & 0xFF)
	output[1] = uint8(wOut[1] & 0xFF)
	output[2] = uint8(wOut[0] & 0xFF)

	return output[3:]
}

func Pack3Words(mm mem.Manager, info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	//fmt.Println("Pack3Words")
	if len(output) < 6 || len(wOut) < 3 {
		return output
	}
	binary.LittleEndian.PutUint16(output[0:], wOut[0])
	binary.LittleEndian.PutUint16(output[2:], wOut[1])
	binary.LittleEndian.PutUint16(output[4:], wOut[2])
	return output[6:]
}
func Pack3WordsSwap(mm mem.Manager, info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	//fmt.Println("Pack3WordsSwap")
	if len(output) < 6 || len(wOut) < 3 {
		return output
	}
	binary.LittleEndian.PutUint16(output[0:], wOut[2])
	binary.LittleEndian.PutUint16(output[2:], wOut[1])
	binary.LittleEndian.PutUint16(output[4:], wOut[0])
	return output[6:]
}
func Pack3WordsBigEndian(mm mem.Manager, info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	//fmt.Println("Pack3WordsBigEndian")
	if len(output) < 6 || len(wOut) < 3 {
		return output
	}
	binary.BigEndian.PutUint16(output[0:], wOut[0])
	binary.BigEndian.PutUint16(output[2:], wOut[1])
	binary.BigEndian.PutUint16(output[4:], wOut[2])
	return output[6:]
}

func Pack3BytesAndSkip1(mm mem.Manager, info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	output[0] = FROM_16_TO_8(wOut[0])
	output[1] = FROM_16_TO_8(wOut[1])
	output[2] = FROM_16_TO_8(wOut[2])
	output[3] = 0 // Skip 1 byte

	return output[4:]
}
func Pack3BytesAndSkip1Optimized(mm mem.Manager, info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	output[0] = uint8(wOut[0] & 0xFF)
	output[1] = uint8(wOut[1] & 0xFF)
	output[2] = uint8(wOut[2] & 0xFF)
	output[3] = 0 // Skip 1 byte

	return output[4:]
}
func Pack3BytesAndSkip1SwapFirst(mm mem.Manager, info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	output[0] = 0 // Skip first byte
	output[1] = FROM_16_TO_8(wOut[0])
	output[2] = FROM_16_TO_8(wOut[1])
	output[3] = FROM_16_TO_8(wOut[2])

	return output[4:]
}
func Pack3BytesAndSkip1SwapFirstOptimized(mm mem.Manager, info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	output[0] = 0 // Skip first byte
	output[1] = uint8(wOut[0] & 0xFF)
	output[2] = uint8(wOut[1] & 0xFF)
	output[3] = uint8(wOut[2] & 0xFF)

	return output[4:]
}
func Pack3BytesAndSkip1Swap(mm mem.Manager, info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	output[0] = 0 // Skip first byte
	output[1] = FROM_16_TO_8(wOut[2])
	output[2] = FROM_16_TO_8(wOut[1])
	output[3] = FROM_16_TO_8(wOut[0])

	return output[4:]
}
func Pack3BytesAndSkip1SwapOptimized(mm mem.Manager, info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	output[0] = 0 // Skip first byte
	output[1] = uint8(wOut[2] & 0xFF)
	output[2] = uint8(wOut[1] & 0xFF)
	output[3] = uint8(wOut[0] & 0xFF)

	return output[4:]
}

func Pack3WordsAndSkip1(mm mem.Manager, info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	//fmt.Println("Pack3WordsAndSkip1")
	if len(output) < 8 || len(wOut) < 3 {
		return output
	}
	binary.LittleEndian.PutUint16(output[0:], wOut[0])
	binary.LittleEndian.PutUint16(output[2:], wOut[1])
	binary.LittleEndian.PutUint16(output[4:], wOut[2])
	return output[8:] // skip 1 word
}
func Pack3WordsAndSkip1Swap(mm mem.Manager, info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	//fmt.Println("Pack3WordsAndSkip1Swap")
	if len(output) < 8 || len(wOut) < 3 {
		return output
	}
	output = output[2:] // skip 1 word

	binary.LittleEndian.PutUint16(output[0:], wOut[2])
	binary.LittleEndian.PutUint16(output[2:], wOut[1])
	binary.LittleEndian.PutUint16(output[4:], wOut[0])
	return output[6:]
}
func Pack3WordsAndSkip1SwapFirst(mm mem.Manager, info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	//fmt.Println("Pack3WordsAndSkip1SwapFirst")
	if len(output) < 8 || len(wOut) < 3 {
		return output
	}
	output = output[2:] // skip 1 word

	binary.LittleEndian.PutUint16(output[0:], wOut[0])
	binary.LittleEndian.PutUint16(output[2:], wOut[1])
	binary.LittleEndian.PutUint16(output[4:], wOut[2])
	return output[6:]
}
func Pack3WordsAndSkip1SwapSwapFirst(mm mem.Manager, info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	//fmt.Println("Pack3WordsAndSkip1SwapSwapFirst")
	if len(output) < 8 || len(wOut) < 3 {
		return output
	}
	binary.LittleEndian.PutUint16(output[0:], wOut[2])
	binary.LittleEndian.PutUint16(output[2:], wOut[1])
	binary.LittleEndian.PutUint16(output[4:], wOut[0])
	return output[8:] // skip 1 word after writing
}

func Pack3BytesAndSkip1SwapSwapFirst(mm mem.Manager, info *cmsTRANSFORM, wOut []uint16, output []uint8, stride uint32) []uint8 {
	output[0] = uint8(wOut[2] >> 8) // FROM_16_TO_8 in the C code.
	output[1] = uint8(wOut[1] >> 8) // FROM_16_TO_8 in the C code.
	output[2] = uint8(wOut[0] >> 8) // FROM_16_TO_8 in the C code.
	output[3] = 0                   // Skip 1 byte (set it to 0 or leave as is).
	return output[4:]               // Advance the pointer.

	// `info` and `stride` are unused, as in the C code.
}

func Pack3BytesAndSkip1SwapSwapFirstOptimized(mm mem.Manager, info *cmsTRANSFORM, wOut []uint16, output []uint8, stride uint32) []uint8 {
	output[0] = uint8(wOut[2] & 0xFF) // Extract the least significant byte.
	output[1] = uint8(wOut[1] & 0xFF) // Extract the least significant byte.
	output[2] = uint8(wOut[0] & 0xFF) // Extract the least significant byte.
	output[3] = 0                     // Skip 1 byte (set it to 0 or leave as is).
	return output[4:]                 // Advance the pointer.

	// `info` and `stride` are unused, as in the C code.
}

func Pack1Byte(mm mem.Manager, info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	output[0] = FROM_16_TO_8(wOut[0])
	return output[1:]
}
func Pack1ByteReversed(mm mem.Manager, info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	output[0] = FROM_16_TO_8(REVERSE_FLAVOR_16(wOut[0]))
	return output[1:]
}
func Pack1ByteSkip1(mm mem.Manager, info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	output[0] = FROM_16_TO_8(wOut[0])
	return output[2:] // Skip 1 byte
}
func Pack1ByteSkip1SwapFirst(mm mem.Manager, info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	output[1] = FROM_16_TO_8(wOut[0]) // Skip the first byte
	return output[2:]
}

func Pack1Word(mm mem.Manager, info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	if len(output) < 2 || len(wOut) < 1 {
		return output
	}
	binary.LittleEndian.PutUint16(output, wOut[0])
	return output[2:]
}
func Pack1WordReversed(mm mem.Manager, info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	//fmt.Println("Pack1WordReversed")
	if len(output) < 2 || len(wOut) < 1 {
		return output
	}
	v := REVERSE_FLAVOR_16(wOut[0])
	binary.LittleEndian.PutUint16(output, v)
	return output[2:]
}
func Pack1WordBigEndian(mm mem.Manager, info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	//fmt.Println("Pack1WordBigEndian")
	if len(output) < 2 || len(wOut) < 1 {
		return output
	}
	v := CHANGE_ENDIAN(wOut[0])
	binary.LittleEndian.PutUint16(output, v)
	return output[2:]
}
func Pack1WordSkip1(mm mem.Manager, info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	//fmt.Println("Pack1WordsSkip")
	if len(output) < 4 || len(wOut) < 1 {
		return output
	}
	binary.LittleEndian.PutUint16(output, wOut[0])
	return output[4:]
}
func Pack1WordSkip1SwapFirst(mm mem.Manager, info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	//fmt.Println("Pack1WordSkip1SwapFirst")
	if len(output) < 4 || len(wOut) < 1 {
		return output
	}
	output = output[2:]
	binary.LittleEndian.PutUint16(output, wOut[0])
	return output[2:]
}

func PackLabDoubleFrom16(mm mem.Manager, info *cmsTRANSFORM, wOut []uint16, output []uint8, stride uint32) []uint8 {
	//fmt.Println("PackLabDoubleFrom16")
	var lab cmsCIELab
	cmsLabEncoded2Float(&lab, &[3]uint16{wOut[0], wOut[1], wOut[2]})

	if T_PLANAR(info.OutputFormat) != 0 {
		stride /= PixelSize(info.OutputFormat)

		binary.LittleEndian.PutUint64(output[0:], math.Float64bits(lab.L))
		binary.LittleEndian.PutUint64(output[stride*8:], math.Float64bits(lab.a))
		binary.LittleEndian.PutUint64(output[stride*16:], math.Float64bits(lab.b))

		return output[8:]
	}

	binary.LittleEndian.PutUint64(output[0:], math.Float64bits(lab.L))
	binary.LittleEndian.PutUint64(output[8:], math.Float64bits(lab.a))
	binary.LittleEndian.PutUint64(output[16:], math.Float64bits(lab.b))

	return output[24+(T_EXTRA(info.OutputFormat)*8):]
}
func PackLabFloatFrom16(mm mem.Manager, info *cmsTRANSFORM, wOut []uint16, output []uint8, stride uint32) []uint8 {
	//fmt.Println("PackLabFloatFrom16")
	var lab cmsCIELab
	cmsLabEncoded2Float(&lab, &[3]uint16{wOut[0], wOut[1], wOut[2]})

	if T_PLANAR(info.OutputFormat) != 0 {
		stride /= PixelSize(info.OutputFormat)

		binary.LittleEndian.PutUint32(output[0:], math.Float32bits(float32(lab.L)))
		binary.LittleEndian.PutUint32(output[stride*4:], math.Float32bits(float32(lab.a)))
		binary.LittleEndian.PutUint32(output[stride*8:], math.Float32bits(float32(lab.b)))

		return output[4:]
	}

	binary.LittleEndian.PutUint32(output[0:], math.Float32bits(float32(lab.L)))
	binary.LittleEndian.PutUint32(output[4:], math.Float32bits(float32(lab.a)))
	binary.LittleEndian.PutUint32(output[8:], math.Float32bits(float32(lab.b)))

	return output[12+(T_EXTRA(info.OutputFormat)*4):]
}
func PackXYZDoubleFrom16(mm mem.Manager, info *cmsTRANSFORM, wOut []uint16, output []uint8, stride uint32) []uint8 {
	//fmt.Println("PackXYZDoubleFrom16")
	var xyz cmsCIEXYZ
	cmsXYZEncoded2Float(&xyz, &[3]uint16{wOut[0], wOut[1], wOut[2]})

	if T_PLANAR(info.OutputFormat) != 0 {
		stride /= PixelSize(info.OutputFormat)

		binary.LittleEndian.PutUint64(output[0:], math.Float64bits(xyz.X))
		binary.LittleEndian.PutUint64(output[stride*8:], math.Float64bits(xyz.Y))
		binary.LittleEndian.PutUint64(output[stride*16:], math.Float64bits(xyz.Z))

		return output[8:]
	}

	binary.LittleEndian.PutUint64(output[0:], math.Float64bits(xyz.X))
	binary.LittleEndian.PutUint64(output[8:], math.Float64bits(xyz.Y))
	binary.LittleEndian.PutUint64(output[16:], math.Float64bits(xyz.Z))

	return output[24+(T_EXTRA(info.OutputFormat)*8):]
}
func PackXYZFloatFrom16(mm mem.Manager, info *cmsTRANSFORM, wOut []uint16, output []uint8, stride uint32) []uint8 {
	//fmt.Println("PackXYZFloatFrom16")
	var xyz cmsCIEXYZ
	cmsXYZEncoded2Float(&xyz, &[3]uint16{wOut[0], wOut[1], wOut[2]})

	if T_PLANAR(info.OutputFormat) != 0 {
		stride /= PixelSize(info.OutputFormat)

		binary.LittleEndian.PutUint32(output[0:], math.Float32bits(float32(xyz.X)))
		binary.LittleEndian.PutUint32(output[stride*4:], math.Float32bits(float32(xyz.Y)))
		binary.LittleEndian.PutUint32(output[stride*8:], math.Float32bits(float32(xyz.Z)))

		return output[4:]
	}

	binary.LittleEndian.PutUint32(output[0:], math.Float32bits(float32(xyz.X)))
	binary.LittleEndian.PutUint32(output[4:], math.Float32bits(float32(xyz.Y)))
	binary.LittleEndian.PutUint32(output[8:], math.Float32bits(float32(xyz.Z)))

	return output[12+(T_EXTRA(info.OutputFormat)*4):]
}

func PackDoubleFrom16(mm mem.Manager, info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	//fmt.Println("PackDoubleFrom16")
	nChan := T_CHANNELS(info.OutputFormat)
	DoSwap := T_DOSWAP(info.OutputFormat)
	Reverse := T_FLAVOR(info.OutputFormat)
	Extra := T_EXTRA(info.OutputFormat)
	SwapFirst := T_SWAPFIRST(info.OutputFormat)
	Planar := T_PLANAR(info.OutputFormat)
	ExtraFirst := DoSwap ^ SwapFirst

	maximum := 65535.0
	if IsInkSpace(info.OutputFormat) {
		maximum = 655.35
	}

	start := uint32(0)
	if ExtraFirst != 0 {
		start = Extra
	}

	Stride /= PixelSize(info.OutputFormat)
	//offset := 0
	var v float64
	size := 8 // float64 size in bytes

	buf := mem.MakeSlice[float64](mm, (int(nChan)+int(Extra))*int(Stride)+1)

	for i := uint32(0); i < nChan; i++ {
		index := i
		if DoSwap != 0 {
			index = nChan - i - 1
		}

		v = float64(wOut[index]) / maximum
		if Reverse != 0 {
			v = maximum - v
		}

		if Planar != 0 {
			buf[(i+start)*Stride] = v
		} else {
			buf[i+start] = v
		}
	}

	if Extra == 0 && SwapFirst != 0 {
		copy(buf[1:], buf[:nChan-1])
		buf[0] = v
	}

	b := bytes.NewBuffer(output[:0])
	for i := 0; i < (int(nChan) + int(Extra)); i++ {
		_ = binary.Write(b, binary.LittleEndian, buf[i])
	}

	if Planar != 0 {
		return output[size:]
	}
	return output[(nChan+Extra)*uint32(size):]
}
func PackFloatFrom16(mm mem.Manager,info *cmsTRANSFORM, wOut []uint16, output []uint8, Stride uint32) []uint8 {
	//fmt.Println("PackFloatFrom16")
	nChan := T_CHANNELS(info.OutputFormat)
	DoSwap := T_DOSWAP(info.OutputFormat)
	Reverse := T_FLAVOR(info.OutputFormat)
	Extra := T_EXTRA(info.OutputFormat)
	SwapFirst := T_SWAPFIRST(info.OutputFormat)
	Planar := T_PLANAR(info.OutputFormat)
	ExtraFirst := DoSwap ^ SwapFirst

	maximum := 65535.0
	if IsInkSpace(info.OutputFormat) {
		maximum = 655.35
	}

	start := uint32(0)
	if ExtraFirst != 0 {
		start = Extra
	}

	Stride /= PixelSize(info.OutputFormat)
	//offset := 0
	var v float64
	size := 4 // float32 size in bytes

	buf := mem.MakeSlice[float32](mm, (int(nChan)+int(Extra))*int(Stride)+1)

	for i := uint32(0); i < nChan; i++ {
		index := i
		if DoSwap != 0 {
			index = nChan - i - 1
		}

		v = float64(wOut[index]) / maximum
		if Reverse != 0 {
			v = maximum - v
		}

		if Planar != 0 {
			buf[(i+start)*Stride] = float32(v)
		} else {
			buf[i+start] = float32(v)
		}
	}

	if Extra == 0 && SwapFirst != 0 {
		copy(buf[1:], buf[:nChan-1])
		buf[0] = float32(v)
	}

	b := bytes.NewBuffer(output[:0])
	for i := 0; i < (int(nChan) + int(Extra)); i++ {
		_ = binary.Write(b, binary.LittleEndian, buf[i])
	}

	if Planar != 0 {
		return output[size:]
	}
	return output[(nChan+Extra)*uint32(size):]
}
func PackFloatsFromFloat(mm mem.Manager,info *cmsTRANSFORM, wOut []float32, output []uint8, Stride uint32) []uint8 {
	//fmt.Println("PackFloatsFromFloat")
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

	start := uint32(0)
	if ExtraFirst != 0 {
		start = Extra
	}

	Stride /= PixelSize(info.OutputFormat)
	size := 4 // float32

	buf := mem.MakeSlice[float32](mm, (int(nChan)+int(Extra))*int(Stride)+1)

	for i := uint32(0); i < nChan; i++ {
		index := i
		if DoSwap != 0 {
			index = nChan - i - 1
		}

		v := float64(wOut[index]) * maximum
		if Reverse != 0 {
			v = maximum - v
		}

		if Planar != 0 {
			buf[(i+start)*Stride] = float32(v)
		} else {
			buf[i+start] = float32(v)
		}
	}

	if Extra == 0 && SwapFirst != 0 {
		copy(buf[1:], buf[:nChan-1])
		buf[0] = float32(buf[nChan-1])
	}

	b := bytes.NewBuffer(output[:0])
	for i := 0; i < (int(nChan) + int(Extra)); i++ {
		_ = binary.Write(b, binary.LittleEndian, buf[i])
	}

	if Planar != 0 {
		return output[size:]
	}
	return output[(nChan+Extra)*uint32(size):]
}
func PackDoublesFromFloat(mm mem.Manager,info *cmsTRANSFORM, wOut []float32, output []uint8, Stride uint32) []uint8 {
	//fmt.Println("PackDoublesFromFloat")
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

	start := uint32(0)
	if ExtraFirst != 0 {
		start = Extra
	}

	Stride /= PixelSize(info.OutputFormat)
	size := 8 // float64

	buf := mem.MakeSlice[float64](mm, (int(nChan)+int(Extra))*int(Stride)+1)

	for i := uint32(0); i < nChan; i++ {
		index := i
		if DoSwap != 0 {
			index = nChan - i - 1
		}

		v := float64(wOut[index]) * maximum
		if Reverse != 0 {
			v = maximum - v
		}

		if Planar != 0 {
			buf[(i+start)*Stride] = v
		} else {
			buf[i+start] = v
		}
	}

	if Extra == 0 && SwapFirst != 0 {
		copy(buf[1:], buf[:nChan-1])
		buf[0] = buf[nChan-1]
	}

	b := bytes.NewBuffer(output[:0])
	for i := 0; i < (int(nChan) + int(Extra)); i++ {
		_ = binary.Write(b, binary.LittleEndian, buf[i])
	}

	if Planar != 0 {
		return output[size:]
	}
	return output[(nChan+Extra)*uint32(size):]
}
func PackLabFloatFromFloat(mm mem.Manager,info *cmsTRANSFORM, wOut []float32, output []uint8, Stride uint32) []uint8 {
	//fmt.Println("PackLabFloatFromFloat")
	L := wOut[0] * 100.0
	a := wOut[1]*255.0 - 128.0
	b := wOut[2]*255.0 - 128.0

	buf := bytes.NewBuffer(output[:0])
	if T_PLANAR(info.OutputFormat) != 0 {
		Stride /= PixelSize(info.OutputFormat)

		labBuf := mem.MakeSlice[float32](mm, int(3*Stride))
		labBuf[0] = L
		labBuf[Stride] = a
		labBuf[Stride*2] = b

		_ = binary.Write(buf, binary.LittleEndian, labBuf[0])
		return output[4:]
	} else {
		_ = binary.Write(buf, binary.LittleEndian, float32(L))
		_ = binary.Write(buf, binary.LittleEndian, float32(a))
		_ = binary.Write(buf, binary.LittleEndian, float32(b))

		return output[12+(T_EXTRA(info.OutputFormat)*4):]
	}
}
func PackLabDoubleFromFloat(mm mem.Manager,info *cmsTRANSFORM, wOut []float32, output []uint8, Stride uint32) []uint8 {
	//fmt.Println("PackLabDoubleFromFloat")
	L := float64(wOut[0] * 100.0)
	a := float64(wOut[1]*255.0 - 128.0)
	b := float64(wOut[2]*255.0 - 128.0)
	/*fmt.Printf("wOut[0]  %.7f\n", L)
	fmt.Printf("wOut[1]  %.7f\n", a)
	fmt.Printf("wOut[2]  %.7f\n", b)*/

	buf := bytes.NewBuffer(output[:0])
	if T_PLANAR(info.OutputFormat) != 0 {
		Stride /= PixelSize(info.OutputFormat)

		labBuf := mem.MakeSlice[float64](mm, int(3*Stride))
		labBuf[0] = L
		labBuf[Stride] = a
		labBuf[Stride*2] = b

		_ = binary.Write(buf, binary.LittleEndian, labBuf[0])
		return output[8:]
	} else {
		_ = binary.Write(buf, binary.LittleEndian, L)
		_ = binary.Write(buf, binary.LittleEndian, a)
		_ = binary.Write(buf, binary.LittleEndian, b)

		return output[24+(T_EXTRA(info.OutputFormat)*8):]
	}
}
func PackXYZFloatFromFloat(mm mem.Manager,info *cmsTRANSFORM, wOut []float32, output []uint8, Stride uint32) []uint8 {
	//fmt.Println("PackXZYFloatFromFloat")
	X := float64(wOut[0]) * MAX_ENCODEABLE_XYZ
	Y := float64(wOut[1]) * MAX_ENCODEABLE_XYZ
	Z := float64(wOut[2]) * MAX_ENCODEABLE_XYZ

	buf := bytes.NewBuffer(output[:0])
	if T_PLANAR(info.OutputFormat) != 0 {
		Stride /= PixelSize(info.OutputFormat)

		xyzBuf := mem.MakeSlice[float32](mm, int(3*Stride))
		xyzBuf[0] = float32(X)
		xyzBuf[Stride] = float32(Y)
		xyzBuf[Stride*2] = float32(Z)

		_ = binary.Write(buf, binary.LittleEndian, xyzBuf[0])
		return output[4:]
	} else {
		_ = binary.Write(buf, binary.LittleEndian, float32(X))
		_ = binary.Write(buf, binary.LittleEndian, float32(Y))
		_ = binary.Write(buf, binary.LittleEndian, float32(Z))

		return output[12+(T_EXTRA(info.OutputFormat)*4):]
	}
}
func PackXYZDoubleFromFloat(mm mem.Manager,info *cmsTRANSFORM, wOut []float32, output []uint8, Stride uint32) []uint8 {
	//fmt.Println("PackXYZDoubleFromFloat")
	X := float64(wOut[0]) * MAX_ENCODEABLE_XYZ
	Y := float64(wOut[1]) * MAX_ENCODEABLE_XYZ
	Z := float64(wOut[2]) * MAX_ENCODEABLE_XYZ

	buf := bytes.NewBuffer(output[:0])
	if T_PLANAR(info.OutputFormat) != 0 {
		Stride /= PixelSize(info.OutputFormat)

		xyzBuf := mem.MakeSlice[float64](mm, int(3*Stride))
		xyzBuf[0] = X
		xyzBuf[Stride] = Y
		xyzBuf[Stride*2] = Z

		_ = binary.Write(buf, binary.LittleEndian, xyzBuf[0])
		return output[8:]
	} else {
		_ = binary.Write(buf, binary.LittleEndian, X)
		_ = binary.Write(buf, binary.LittleEndian, Y)
		_ = binary.Write(buf, binary.LittleEndian, Z)

		return output[24+(T_EXTRA(info.OutputFormat)*8):]
	}
}

func UnrollHalfTo16(mm mem.Manager,info *cmsTRANSFORM, wIn []uint16, accum []uint8, stride uint32) []uint8 {
	//fmt.Println("UnrollHalfTo16")
	nChan := T_CHANNELS(info.InputFormat)
	doSwap := T_DOSWAP(info.InputFormat)
	reverse := T_FLAVOR(info.InputFormat)
	swapFirst := T_SWAPFIRST(info.InputFormat)
	extra := T_EXTRA(info.InputFormat)
	extraFirst := doSwap ^ swapFirst
	planar := T_PLANAR(info.InputFormat)
	var start uint32
	var maximum float32 = 65535.0
	if IsInkSpace(info.InputFormat) {
		maximum = 655.35
	}

	stride /= PixelSize(info.InputFormat)
	buf := bytes.NewReader(accum)
	accumWords := mem.MakeSlice[uint16](mm, len(accum)/2)
	binary.Read(buf, binary.LittleEndian, &accumWords)

	if extraFirst != 0 {
		start = extra
	}

	for i := uint32(0); i < nChan; i++ {
		index := i
		if doSwap != 0 {
			index = nChan - i - 1
		}
		var v float32
		if planar != 0 {
			v = cmsHalf2Float(accumWords[(i+start)*stride])
		} else {
			v = cmsHalf2Float(accumWords[i+start])
		}
		if reverse != 0 {
			v = maximum - v
		}
		wIn[index] = cmsQuickSaturateWord(float64(v * maximum))
	}

	if extra == 0 && swapFirst != 0 {
		tmp := wIn[0]
		copy(wIn[0:], wIn[1:])
		wIn[nChan-1] = tmp
	}

	if planar != 0 {
		return accum[2:]
	} else {
		return accum[(nChan+extra)*2:]
	}
}
func UnrollHalfToFloat(mm mem.Manager,info *cmsTRANSFORM, wIn []float32, accum []uint8, stride uint32) []uint8 {
	//fmt.Println("UnrollHalfToFloat")
	nChan := T_CHANNELS(info.InputFormat)
	doSwap := T_DOSWAP(info.InputFormat)
	reverse := T_FLAVOR(info.InputFormat)
	swapFirst := T_SWAPFIRST(info.InputFormat)
	extra := T_EXTRA(info.InputFormat)
	extraFirst := doSwap ^ swapFirst
	planar := T_PLANAR(info.InputFormat)
	var start uint32
	var maximum float32 = 1.0
	if IsInkSpace(info.InputFormat) {
		maximum = 100.0
	}

	stride /= PixelSize(info.InputFormat)
	buf := bytes.NewReader(accum)
	accumWords := mem.MakeSlice[uint16](mm, len(accum)/2)
	binary.Read(buf, binary.LittleEndian, &accumWords)

	if extraFirst != 0 {
		start = extra
	}

	for i := uint32(0); i < nChan; i++ {
		index := i
		if doSwap != 0 {
			index = nChan - i - 1
		}
		var v float32
		if planar != 0 {
			v = cmsHalf2Float(accumWords[(i+start)*stride])
		} else {
			v = cmsHalf2Float(accumWords[i+start])
		}
		v /= maximum
		if reverse != 0 {
			v = 1.0 - v
		}
		wIn[index] = v
	}

	if extra == 0 && swapFirst != 0 {
		tmp := wIn[0]
		copy(wIn[0:], wIn[1:])
		wIn[nChan-1] = tmp
	}

	if planar != 0 {
		return accum[2:]
	} else {
		return accum[(nChan+extra)*2:]
	}
}
func PackHalfFrom16(mm mem.Manager,info *cmsTRANSFORM, wOut []uint16, output []uint8, stride uint32) []uint8 {
	//fmt.Println("PackHalfFrom16")
	nChan := T_CHANNELS(info.OutputFormat)
	doSwap := T_DOSWAP(info.OutputFormat)
	reverse := T_FLAVOR(info.OutputFormat)
	extra := T_EXTRA(info.OutputFormat)
	swapFirst := T_SWAPFIRST(info.OutputFormat)
	planar := T_PLANAR(info.OutputFormat)
	extraFirst := doSwap ^ swapFirst
	var maximum float32 = 65535.0
	if IsInkSpace(info.OutputFormat) {
		maximum = 655.35
	}
	stride /= PixelSize(info.OutputFormat)

	outputWords := mem.MakeSlice[uint16](mm, int((nChan+extra)*uint32(stride)+1))
	var start uint32
	if extraFirst != 0 {
		start = extra
	}

	var v float32
	for i := uint32(0); i < nChan; i++ {
		index := i
		if doSwap != 0 {
			index = nChan - i - 1
		}
		v = float32(wOut[index]) / maximum
		if reverse != 0 {
			v = maximum - v
		}
		if planar != 0 {
			outputWords[(i+start)*stride] = cmsFloat2Half(v)
		} else {
			outputWords[i+start] = cmsFloat2Half(v)
		}
	}

	if extra == 0 && swapFirst != 0 {
		copy(outputWords[1:], outputWords[:nChan-1])
		outputWords[0] = cmsFloat2Half(v)
	}

	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, outputWords)
	return buf.Bytes()
}
func PackHalfFromFloat(mm mem.Manager,info *cmsTRANSFORM, wOut []float32, output []uint8, stride uint32) []uint8 {
	//fmt.Println("PackHalfFromFloat")
	nChan := T_CHANNELS(info.OutputFormat)
	doSwap := T_DOSWAP(info.OutputFormat)
	reverse := T_FLAVOR(info.OutputFormat)
	extra := T_EXTRA(info.OutputFormat)
	swapFirst := T_SWAPFIRST(info.OutputFormat)
	planar := T_PLANAR(info.OutputFormat)
	extraFirst := doSwap ^ swapFirst
	var maximum float32 = 1.0
	if IsInkSpace(info.OutputFormat) {
		maximum = 100.0
	}
	stride /= PixelSize(info.OutputFormat)

	outputWords := mem.MakeSlice[uint16](mm, int((nChan+extra)*uint32(stride)+1))
	var start uint32
	if extraFirst != 0 {
		start = extra
	}

	var v float32
	for i := uint32(0); i < nChan; i++ {
		index := i
		if doSwap != 0 {
			index = nChan - i - 1
		}
		v = wOut[index] * maximum
		if reverse != 0 {
			v = maximum - v
		}
		if planar != 0 {
			outputWords[(i+start)*stride] = cmsFloat2Half(v)
		} else {
			outputWords[i+start] = cmsFloat2Half(v)
		}
	}

	if extra == 0 && swapFirst != 0 {
		copy(outputWords[1:], outputWords[:nChan-1])
		outputWords[0] = cmsFloat2Half(v)
	}

	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, outputWords)
	return buf.Bytes()
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

var cmsFormattersPluginChunk = cmsFormattersPluginChunkType{FactoryList: nil}

// Duplicate the zone of memory used by the plugin in the new context
func DupFormatterFactoryList(mm mem.Manager, ctx CmsContext, src CmsContext) {
	var newHead cmsFormattersPluginChunkType
	var previousEntry *cmsFormattersFactoryList
	head := (CmsContextStruct)(*src).chunks[FormattersPlugin].(*cmsFormattersPluginChunkType)

	if head == nil {
		panic("Source context does not contain FormattersPlugin chunk")
	}

	// Walk the list and copy all nodes
	for entry := head.FactoryList; entry != nil; entry = entry.Next {
		newEntry := mem.New[cmsFormattersFactoryList](mm)
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

	(*ctx).chunks[FormattersPlugin] = cmsSubAllocDup(mm, (CmsContextStruct)(*ctx).MemPool, &newHead, uint32(unsafe.Sizeof(newHead)))
}

// Allocate and initialize the Formatters plugin chunk
func cmsAllocFormattersPluginChunk(mm mem.Manager, ctx CmsContext, src CmsContext) {
	cmsAssert(ctx != nil, "Context is nil")

	if src != nil {
		// Duplicate the list
		DupFormatterFactoryList(mm, ctx, src)
	} else {
		staticChunk := cmsFormattersPluginChunkType{}
		(*ctx).chunks[FormattersPlugin] = cmsSubAllocDup(mm, (CmsContextStruct)(*ctx).MemPool, &staticChunk, uint32(unsafe.Sizeof(staticChunk)))
	}
}

// Register formatters plugin
func cmsRegisterFormattersPlugin(mm mem.Manager, contextID CmsContext, Data PluginIntrfc) bool {
	//ctx := (*cmsFormattersPluginChunkType)((CmsContextStruct)(*contextID).chunks[FormattersPlugin])
	ctx := CmsContextGetClientChunk(contextID, FormattersPlugin).(*cmsFormattersPluginChunkType)
	plugin, ok := Data.(*cmsPluginFormatters)
	if !ok {
		panic("Plugin is not of the type cmsPluginFormatters\n")

	}
	if Data == nil {
		// Reset to built-in defaults
		ctx.FactoryList = nil
		return true
	}
	//newEntry := (*cmsFormattersFactoryList)(cmsPluginMalloc(contextID, uint32(unsafe.Sizeof(list))))
	newEntry := mem.New[cmsFormattersFactoryList](mm)

	newEntry.Factory = plugin.FormattersFactory
	newEntry.Next = ctx.FactoryList
	ctx.FactoryList = newEntry

	return true
}

// Get a formatter
func cmsGetFormatter(contextID CmsContext, typeID uint32, direction cmsFormatterDirection, dwFlags uint32) cmsFormatter {
	//ctx := (*cmsFormattersPluginChunkType)((CmsContextStruct)(*contextID).chunks[FormattersPlugin])
	ctx := CmsContextGetClientChunk(contextID, FormattersPlugin).(*cmsFormattersPluginChunkType)
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
	// cmsChannelsOf always returns a non-zero unsigned count; LCMS falls back to 3 on error.
	// (The original C had `if (nOutputChans < 0) return 0;`, which can’t happen here.)

	floatFlag := uint32(0)
	if isFloat {
		floatFlag = 1
	}
	return FLOAT_SH(floatFlag) | COLORSPACE_SH(uint32(colorSpaceBits)) | BYTES_SH(nBytes) | CHANNELS_SH(uint32(nOutputChans))
}
