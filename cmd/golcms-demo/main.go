package main

import (
	"fmt"
	"os"
	"reflect"

	gol "github.com/yzigangirova/lcms-go"
	"github.com/yzigangirova/lcms-go/mem"
)

// -----------------------
// Replace with real files
// -----------------------
const (
	SrcRGBProfilePath  = "" // e.g., sRGB or camera RGB
	DstRGBProfilePath  = "" // empty means use built-in sRGB if available
	DstCMYKProfilePath = ""
)

func main() {
	must(exampleRGB8toRGB8())
	must(exampleRGB8toCMYK8())
	must(exampleLab16toRGB8())
	must(exampleXYZ16toRGB8Stride())
	fmt.Println("\nAll examples finished.")
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// u16ToByte converts a 0..65535 channel to 0..255 with rounding.
func u16ToByte(v uint16) uint8 {
	// round: (v + 128) / 257  ≈ v / 257 with rounding
	// 65535 -> 255, 0 -> 0
	return uint8((uint32(v) + 128) / 257)
}

// byteToU16 expands 0..255 to 0..65535 (simple scale).
// Only 16-bit values that are exact multiples of 257 come back exactly
func byteToU16(b uint8) uint16 {
	// multiply by 257 to span full 16-bit (0..65535)
	return uint16(uint32(b) * 257)
}

func invalidXform(x gol.CmsHTRANSFORM) bool {
	if x == nil {
		return true
	}
	v := reflect.ValueOf(x)
	if v.Kind() == reflect.Ptr && v.IsNil() { // typed-nil inside interface
		return true
	}
	// If your concrete type exposes a handle/validity, check it too:
	type hasHandle interface{ Handle() uintptr }
	if h, ok := x.(hasHandle); ok {
		return h.Handle() == 0
	}
	return false
}

// --------------------------------------
// Example 1: RGB8 -> RGB8 (identity-ish)
// --------------------------------------
func exampleRGB8toRGB8() error {
	fmt.Println("\n[Example] RGB8 -> RGB8")

	// Open / create profiles
	var src gol.CmsHPROFILE
	var dst gol.CmsHPROFILE
	var err error

	if SrcRGBProfilePath != "" {
		src = gol.CmsOpenProfileFromFile(mem.Manager{}, SrcRGBProfilePath, "r")

	} else {
		src = gol.CmsCreate_sRGBProfile(mem.Manager{}) // TODO: adjust name; e.g., cmsCreate_sRGBProfile
	}

	if DstRGBProfilePath != "" {
		dst = gol.CmsOpenProfileFromFile(mem.Manager{}, DstRGBProfilePath, "r") // optional explicit dst
		if dst == nil {
			return fmt.Errorf("open dst profile: %w", err)
		}
	} else {
		dst = gol.CmsCreate_sRGBProfile(mem.Manager{})
	}

	// Build transform RGB_8 -> RGB_8
	xform := gol.CmsCreateTransform(mem.Manager{},
		src, gol.TYPE_RGB_8,
		dst, gol.TYPE_RGB_8,
		gol.INTENT_PERCEPTUAL, gol.CmsFLAGS_BLACKPOINTCOMPENSATION)

	if invalidXform(xform) {
		return fmt.Errorf("CreateTransform failed (nil/invalid handle)")
	}

	// Process a few pixels
	inInt16 := []uint16{
		65535, 0, 0, // red
		0, 65535, 0, // green
		3084, 8703, 14392, // ~ {12,34,56}<<8 with rounding, example
	}

	// Pack to bytes exactly like Xpdf does before cmsDoTransform
	inBytes := make([]uint8, len(inInt16))
	for i, v := range inInt16 {
		inBytes[i] = u16ToByte(v) // 16→8 with clamping + rounding
	}

	// Transform
	// Transform
	outBytes := make([]uint8, len(inBytes))
	nPix := uint32(len(inBytes) / 3)
	gol.CmsDoTransform(mem.Manager{}, xform, inBytes, outBytes, nPix)

	// Optionally expand back to 16-bit-ish integers
	outInt16 := make([]uint16, len(outBytes))
	for i, b := range outBytes {
		outInt16[i] = byteToU16(b) // 8→16 expansion (simple scale)
	}

	fmt.Printf("in  (u16): %v\n", inInt16)
	fmt.Printf("in  (u8) : %v\n", inBytes)
	fmt.Printf("out (u8) : %v\n", outBytes)
	fmt.Printf("out (u16): %v\n", outInt16)
	return nil
}

// --------------------------------------
// Example 2: RGB8 -> CMYK8 (printer profile)
// --------------------------------------
func exampleRGB8toCMYK8() error {
	fmt.Println("\n[Example] RGB8 -> CMYK8")

	// Open/create source RGB
	var src gol.CmsHPROFILE
	if SrcRGBProfilePath != "" {
		p := gol.CmsOpenProfileFromFile(mem.Manager{}, SrcRGBProfilePath, "r")

		src = p
	} else {
		src = gol.CmsCreate_sRGBProfile(mem.Manager{})
	}

	// Destination CMYK profile from file (or memory)
	if DstCMYKProfilePath == "" {
		fmt.Println("  (skipping: set DstCMYKProfilePath to a real printer ICC)")
		return nil
	}
	dst := gol.CmsOpenProfileFromFile(mem.Manager{}, DstCMYKProfilePath, "r") // or OpenProfileFromMem

	xform := gol.CmsCreateTransform(mem.Manager{},
		src, gol.TYPE_RGB_8,
		dst, gol.TYPE_CMYK_8,
		gol.INTENT_PERCEPTUAL, gol.CmsFLAGS_BLACKPOINTCOMPENSATION,
	)
	if invalidXform(xform) {
		return fmt.Errorf("CreateTransform failed (nil/invalid handle)")
	}

	rgbIn := []uint8{255, 0, 0, 0, 255, 0, 12, 34, 56}
	out := make([]uint8, (len(rgbIn)/3)*4)
	n := len(rgbIn) / 3
	gol.CmsDoTransform(mem.Manager{}, xform, rgbIn, out, uint32(n))
	fmt.Printf("RGB in : %v\n", rgbIn)
	fmt.Printf("CMYK out: %v\n", out)
	return nil
}

// --------------------------------------
// Example 3: Lab16 -> RGB8 (via D50 Lab)
// --------------------------------------
func exampleLab16toRGB8() error {
	fmt.Println("\n[Example] Lab16 -> RGB8")
	// D50 reference white for Lab (ICC standard)
	wp := gol.CmsCIExyY{
		X_small: 0.34567,
		Y_small: 0.35850,
		Y_large: 1.0,
	}
	src := gol.CmsCreateLab2Profile(mem.Manager{}, &wp)
	dst := gol.CmsCreate_sRGBProfile(mem.Manager{})
	if src == nil || dst == nil {
		return fmt.Errorf("failed to create Lab or sRGB profiles")
	}

	xform := gol.CmsCreateTransform(mem.Manager{},
		src, gol.TYPE_Lab_16,
		dst, gol.TYPE_RGB_8,
		gol.INTENT_PERCEPTUAL, gol.CmsFLAGS_BLACKPOINTCOMPENSATION)
	if invalidXform(xform) {
		return fmt.Errorf("CreateTransform failed (nil/invalid handle)")
	}

	// One pixel: L=100, a=0, b=0 in 16-bit ICC encoding (approx)
	labIn := []uint16{65535, 32768, 32768}
	rgbOut := make([]uint8, 3)
	gol.CmsDoTransform(mem.Manager{}, xform, labIn, rgbOut, 1)
	fmt.Printf("Lab16 in : %v\n", labIn)
	fmt.Printf("RGB8  out: %v\n", rgbOut)
	return nil
}

// ----------------------------------------------------
// Example 4: XYZ16 -> RGB8 using line stride API
// ----------------------------------------------------
func exampleXYZ16toRGB8Stride() error {
	fmt.Println("\n[Example] XYZ16 -> RGB8 (line stride)")

	src := gol.CmsCreateXYZProfile(mem.Manager{})
	dst := gol.CmsCreate_sRGBProfile(mem.Manager{})
	if src == nil || dst == nil {
		return fmt.Errorf("failed to create XYZ or sRGB profiles")
	}

	// Some workflows prefer disabling optimization from XYZ
	flags := gol.CmsFLAGS_BLACKPOINTCOMPENSATION | gol.CmsFLAGS_NOOPTIMIZE // TODO: adjust

	xform := gol.CmsCreateTransform(mem.Manager{},
		src, gol.TYPE_XYZ_16,
		dst, gol.TYPE_RGB_8,
		gol.INTENT_PERCEPTUAL, uint32(flags),
	)
	if invalidXform(xform) {
		return fmt.Errorf("CreateTransform failed (nil/invalid handle)")
	}

	// 2 pixels of XYZ16 (dummy data)
	xyzIn := []uint16{0, 0, 0, 65535, 65535, 65535}
	rgbOut := make([]uint8, 2*3)

	// Use the stride function when processing rows or planar data.
	// Signature varies; this matches a common pattern: DoTransformLineStride(xf, in, out, channelsIn, nPixels, inStrideBytes, outStrideBytes, inSkip, outSkip)
	//1 - packed, 2 - nPix, 3*2 - in stride: 3 channels * 2 bytes, 3 - out stride: 3 bytes
	gol.CmsDoTransformLineStride(mem.Manager{}, xform, xyzIn, rgbOut, 1, 2, 3*2, 3, 0, 0)

	fmt.Printf("XYZ16 in : %v\n", xyzIn)
	fmt.Printf("RGB8  out: %v\n", rgbOut)
	return nil
}
