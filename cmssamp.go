package golcms

// This file contains routines for resampling and LUT optimization, black point detection
// and black preservation.

import (
	"fmt"
	"math"

	"github.com/yzigangirova/lcms-go/mem"
	//"unsafe"
)

func debugPrintCmsICCPROFILE(prefix string, p *cmsICCPROFILE) {
	if p == nil {
		fmt.Printf("[%s] cmsICCPROFILE is nil\n", prefix)
		return
	}
	fmt.Printf("[%s] cmsICCPROFILE @ %p\n", prefix, p)
	fmt.Printf("[%s] ContextID: %v\n", prefix, p.ContextID)
	fmt.Printf("[%s] Created: %v\n", prefix, p.Created)
	fmt.Printf("[%s] Version: %d\n", prefix, p.Version)
	fmt.Printf("[%s] DeviceClass: 0x%X\n", prefix, p.DeviceClass)
	fmt.Printf("[%s] ColorSpace: 0x%X\n", prefix, p.ColorSpace)
	fmt.Printf("[%s] PCS: 0x%X\n", prefix, p.PCS)
	fmt.Printf("[%s] RenderingIntent: %d\n", prefix, p.RenderingIntent)
	fmt.Printf("[%s] Flags: 0x%X\n", prefix, p.Flags)
	fmt.Printf("[%s] Manufacturer: 0x%X\n", prefix, p.Manufacturer)
	fmt.Printf("[%s] Model: 0x%X\n", prefix, p.Model)
	fmt.Printf("[%s] Attributes: 0x%X\n", prefix, p.Attributes)
	fmt.Printf("[%s] Creator: 0x%X\n", prefix, p.Creator)
	fmt.Printf("[%s] ProfileID: %v\n", prefix, p.ProfileID)
	fmt.Printf("[%s] TagCount: %d\n", prefix, p.TagCount)
	fmt.Printf("[%s] IsWrite: %v\n", prefix, p.IsWrite)
}
func debugPrintCmsTRANSFORM(prefix string, t *cmsTRANSFORM) {
	if t == nil {
		fmt.Printf("[%s] cmsTRANSFORM is nil\n", prefix)
		return
	}
	fmt.Printf("[%s] cmsTRANSFORM @ %p\n", prefix, t)
	fmt.Printf("[%s] InputFormat: 0x%X, OutputFormat: 0x%X\n", prefix, t.InputFormat, t.OutputFormat)
	fmt.Printf("[%s] EntryColorSpace: 0x%X, ExitColorSpace: 0x%X\n", prefix, t.EntryColorSpace, t.ExitColorSpace)
	fmt.Printf("[%s] EntryWhitePoint: %+v\n", prefix, t.EntryWhitePoint)
	fmt.Printf("[%s] ExitWhitePoint: %+v\n", prefix, t.ExitWhitePoint)
	fmt.Printf("[%s] DwOriginalFlags: 0x%X\n", prefix, t.DwOriginalFlags)
	fmt.Printf("[%s] AdaptationState: %.6f\n", prefix, t.AdaptationState)
	fmt.Printf("[%s] RenderingIntent: %d\n", prefix, t.RenderingIntent)
	fmt.Printf("[%s] ContextID: %v\n", prefix, t.ContextID)
	fmt.Printf("[%s] MaxWorkers: %d, WorkerFlags: 0x%X\n", prefix, t.MaxWorkers, t.WorkerFlags)

	// LUT Pipeline
	if t.Lut != nil {
		fmt.Printf("[%s] Lut Pipeline @ %p\n", prefix, t.Lut)
		fmt.Printf("[%s] Lut InputChannels: %d, OutputChannels: %d\n", prefix, t.Lut.InputChannels, t.Lut.OutputChannels)
		fmt.Printf("[%s] Lut SaveAs8Bits: %v\n", prefix, t.Lut.SaveAs8Bits)
		fmt.Printf("[%s] Lut ContextID: %v\n", prefix, t.Lut.ContextID)
		if t.Lut.Elements != nil {
			debugPrintCmsStage(prefix+"->Lut.Elements", t.Lut.Elements)
		} else {
			fmt.Printf("[%s] Lut.Elements: nil\n", prefix)
		}

	} else {
		fmt.Printf("[%s] Lut Pipeline: nil\n", prefix)
	}

	// GamutCheck Pipeline
	if t.GamutCheck != nil {
		fmt.Printf("[%s] GamutCheck Pipeline @ %p\n", prefix, t.GamutCheck)
		fmt.Printf("[%s] GamutCheck InputChannels: %d, OutputChannels: %d\n", prefix, t.GamutCheck.InputChannels, t.GamutCheck.OutputChannels)
	} else {
		fmt.Printf("[%s] GamutCheck Pipeline: nil\n", prefix)
	}

	// InputColorant
	if t.InputColorant != nil {
		fmt.Printf("[%s] InputColorant @ %p\n", prefix, t.InputColorant)
		fmt.Printf("[%s] InputColorant nColors: %d, Allocated: %d, ColorantCount: %d\n",
			prefix, t.InputColorant.nColors, t.InputColorant.Allocated, t.InputColorant.ColorantCount)
		fmt.Printf("[%s] InputColorant Prefix: %s\n", prefix, string(t.InputColorant.Prefix[:]))
		fmt.Printf("[%s] InputColorant Suffix: %s\n", prefix, string(t.InputColorant.Suffix[:]))
		fmt.Printf("[%s] InputColorant ContextID: %v\n", prefix, t.InputColorant.ContextID)
	} else {
		fmt.Printf("[%s] InputColorant: nil\n", prefix)
	}

	// OutputColorant
	if t.OutputColorant != nil {
		fmt.Printf("[%s] OutputColorant @ %p\n", prefix, t.OutputColorant)
		fmt.Printf("[%s] OutputColorant nColors: %d, Allocated: %d, ColorantCount: %d\n",
			prefix, t.OutputColorant.nColors, t.OutputColorant.Allocated, t.OutputColorant.ColorantCount)
		fmt.Printf("[%s] OutputColorant Prefix: %s\n", prefix, string(t.OutputColorant.Prefix[:]))
		fmt.Printf("[%s] OutputColorant Suffix: %s\n", prefix, string(t.OutputColorant.Suffix[:]))
		fmt.Printf("[%s] OutputColorant ContextID: %v\n", prefix, t.OutputColorant.ContextID)
	} else {
		fmt.Printf("[%s] OutputColorant: nil\n", prefix)
	}

	// Sequence
	if t.Sequence != nil {
		fmt.Printf("[%s] Sequence @ %p (details omitted)\n", prefix, t.Sequence)
	} else {
		fmt.Printf("[%s] Sequence: nil\n", prefix)
	}

	// Worker and Xform function pointers
	fmt.Printf("[%s] Worker: %v, Xform: %v, OldXform: %v\n",
		prefix, t.Worker != nil, t.Xform != nil, t.OldXform != nil)

	// UserData
	if t.UserData != nil {
		fmt.Printf("[%s] UserData: present (type: %T)\n", prefix, t.UserData)
	} else {
		fmt.Printf("[%s] UserData: nil\n", prefix)
	}
}

//lint:ignore U1000 kept for parity with lcms; used in future ports
func debugPrintCmsStage(prefix string, s *cmsStage) {
	if s == nil {
		fmt.Printf("[%s] cmsStage: nil\n", prefix)
		return
	}
	fmt.Printf("[%s] cmsStage @ %p\n", prefix, s)
	fmt.Printf("[%s] ContextID: %v\n", prefix, s.ContextID)
	fmt.Printf("[%s] Type: 0x%X (%s)\n", prefix, s.Type, decodeStageSignature(s.Type))
	fmt.Printf("[%s] Implements: 0x%X (%s)\n", prefix, s.Implements, decodeStageSignature(s.Implements))
	fmt.Printf("[%s] InputChannels: %d, OutputChannels: %d\n", prefix, s.InputChannels, s.OutputChannels)
	fmt.Printf("[%s] EvalPtr: present: %v\n", prefix, s.EvalPtr != nil)
	fmt.Printf("[%s] DupElemPtr: present: %v\n", prefix, s.DupElemPtr != nil)
	fmt.Printf("[%s] FreePtr: present: %v\n", prefix, s.FreePtr != nil)

	// Print Data type
	if s.Data != nil {
		fmt.Printf("[%s] Data present (type: %T)\n", prefix, s.Data)

		// If Data is *cmsStageMatrixData, print contents
		if m, ok := s.Data.(*cmsStageMatrixData); ok {
			fmt.Printf("[%s] cmsStageMatrixData @ %p\n", prefix, m)
			fmt.Printf("[%s] Matrix (Double, len=%d): ", prefix, len(m.Double))
			for i, v := range m.Double {
				fmt.Printf("%.6f ", v)
				if i >= 15 {
					fmt.Print("... ")
					break
				}
			}
			fmt.Println()
			if m.Offset != nil {
				fmt.Printf("[%s] Offset (len=%d): ", prefix, len(m.Offset))
				for i, v := range m.Offset {
					fmt.Printf("%.6f ", v)
					if i >= 15 {
						fmt.Print("... ")
						break
					}
				}
				fmt.Println()
			} else {
				fmt.Printf("[%s] Offset: nil\n", prefix)
			}
		}
	} else {
		fmt.Printf("[%s] Data: nil\n", prefix)
	}

	if s.Next != nil {
		fmt.Printf("[%s] Next: %p (following next)\n", prefix, s.Next)
		debugPrintCmsStage(prefix+"->Next", s.Next)
	} else {
		fmt.Printf("[%s] Next: nil\n", prefix)
	}
}

func decodeStageSignature(sig cmsStageSignature) string {
	// Replace with your actual signature constants if needed
	switch sig {
	case CmsSigMatrixElemType:
		return "Matrix"
	case CmsSigCurveSetElemType:
		return "CurveSet"
	case CmsSigCLutElemType:
		return "CLUT"
	case CmsSigLab2XYZElemType:
		return "Lab2XYZ"
	case CmsSigXYZ2LabElemType:
		return "XYZ2Lab"
	default:
		return "Unknown"
	}
}

// CreateRoundtripXForm creates a PCS -> PCS round trip transform, always using relative intent on the device -> PCS.
func CreateRoundtripXForm(mm mem.Manager, hProfile CmsHPROFILE, nIntent uint32) CmsHTRANSFORM {
	ContextID := cmsGetProfileContextID(hProfile)
	hLab := cmsCreateLab4ProfileTHR(mm, ContextID, nil)
	var xform CmsHTRANSFORM
	BPC := [4]bool{false, false, false, false}
	States := [4]float64{1.0, 1.0, 1.0, 1.0}
	hProfiles := [4]CmsHPROFILE{hLab, hProfile, hProfile, hLab}
	Intents := [4]uint32{INTENT_RELATIVE_COLORIMETRIC, nIntent, INTENT_RELATIVE_COLORIMETRIC, INTENT_RELATIVE_COLORIMETRIC}
	xform = CmsHTRANSFORM(cmsCreateExtendedTransform(mm,
		ContextID, 4, hProfiles[:], BPC[:], Intents[:],
		States[:], nil, 0, TYPE_Lab_DBL, TYPE_Lab_DBL, CmsFLAGS_NOCACHE|CmsFLAGS_NOOPTIMIZE,
	))

	//hlabProfile := hLab.(*cmsICCPROFILE)
	//xformTransform := xform.(*cmsTRANSFORM)

	// Debug output
	//debugPrintCmsICCPROFILE("CreateRoundtripXForm", hlabProfile)
	//debugPrintCmsTRANSFORM("CreateRoundtripXForm", xformTransform)
	CmsCloseProfile(mm, hLab)
	return xform
}

// BlackPointAsDarkerColorant uses darker colorants to obtain the black point.
// This works in the relative colorimetric intent and assumes more ink results in darker colors. No ink limit is assumed.
func BlackPointAsDarkerColorant(mm mem.Manager, hInput CmsHPROFILE, Intent uint32, BlackPoint *cmsCIEXYZ, dwFlags uint32) bool {
	var Black []uint16
	var xform CmsHTRANSFORM
	var Lab cmsCIELab
	var BlackXYZ cmsCIEXYZ
	var dwFormat uint32
	var nChannels uint32
	var Space cmsColorSpaceSignature
	ContextID := cmsGetProfileContextID(hInput)
	//fmt.Println("START BlackPointAsDarkerColorant")

	// If the profile does not support input direction, assume Black point 0.
	if !cmsIsIntentSupported(hInput, Intent, LCMS_USED_AS_INPUT) {
		if BlackPoint != nil {
			BlackPoint.X, BlackPoint.Y, BlackPoint.Z = 0.0, 0.0, 0.0
		}
		return false
	}

	// Create a formatter with n channels and no floating point.
	dwFormat = cmsFormatterForColorspaceOfProfile(hInput, 2, false)

	// Try to get black by using black colorant.
	Space = CmsGetColorSpace(hInput)
	Black = make([]uint16, 4)

	// This function returns darker colorant in 16 bits for several spaces.
	if !cmsEndPointsBySpace(Space, nil, &Black, &nChannels) {
		if BlackPoint != nil {
			BlackPoint.X, BlackPoint.Y, BlackPoint.Z = 0.0, 0.0, 0.0
		}
		return false
	}

	if nChannels != T_CHANNELS(dwFormat) {
		if BlackPoint != nil {
			BlackPoint.X, BlackPoint.Y, BlackPoint.Z = 0.0, 0.0, 0.0
		}
		return false
	}

	// Use Lab as the output space, avoiding recursion with Lab2.
	hLab := cmsCreateLab2ProfileTHR(mm, ContextID, nil)
	if hLab == nil {
		if BlackPoint != nil {
			BlackPoint.X, BlackPoint.Y, BlackPoint.Z = 0.0, 0.0, 0.0
		}
		return false
	}

	// Create the transform.
	xform = cmsCreateTransformTHR(mm,
		ContextID, hInput, dwFormat, hLab, TYPE_Lab_DBL,
		Intent, CmsFLAGS_NOOPTIMIZE|CmsFLAGS_NOCACHE,
	)

	CmsCloseProfile(mm, hLab)

	if xform == nil {
		if BlackPoint != nil {
			BlackPoint.X, BlackPoint.Y, BlackPoint.Z = 0.0, 0.0, 0.0
		}
		return false
	}

	// Convert black to Lab.
	LabSlice := LabToSlice(Lab)
	CmsDoTransform(mm, xform, Black, LabSlice, 1)
	Lab = SliceToLab(LabSlice)

	// Force it to be neutral; check for inconsistencies.
	Lab.a = 0
	Lab.b = 0
	if Lab.L > 50 || Lab.L < 0 {
		Lab.L = 0
	}

	// Free the resources.
	cmsDeleteTransform(mm, xform)

	// Convert from Lab (now clipped) to XYZ.
	cmsLab2XYZ(nil, &BlackXYZ, &Lab)

	if BlackPoint != nil {
		*BlackPoint = BlackXYZ
	}
	//fmt.Printf("END BlackPointAsDarkerColorant BlackPoint.X %.7f, BlackPoint.Y %.7f, BlackPoint.Z %.7f\n", (*BlackPoint).X, (*BlackPoint).Y, (*BlackPoint).Z)

	return true
}

// BlackPointUsingPerceptualBlack calculates the black point of an output CMYK profile,
// discounting any ink-limiting embedded in the profile.
// The process involves a roundtrip transformation using perceptual intent:
// Lab (0, 0, 0) -> [Perceptual] Profile -> CMYK -> [Rel. Colorimetric] Profile -> Lab.
func BlackPointUsingPerceptualBlack(mm mem.Manager, BlackPoint *cmsCIEXYZ, hProfile CmsHPROFILE) bool {
	//fmt.Println("START BlackPointUsingPerceptualBlack BlackPoint.X %.7f, BlackPoint.Y %.7f, BlackPoint.Z %.7f\n ", (*BlackPoint).X, (*BlackPoint).Y, (*BlackPoint).Z)
	var LabIn, LabOut cmsCIELab
	var BlackXYZ cmsCIEXYZ

	// Check if the profile supports perceptual intent in input direction
	if !cmsIsIntentSupported(hProfile, INTENT_PERCEPTUAL, LCMS_USED_AS_INPUT) {
		if BlackPoint != nil {
			BlackPoint.X, BlackPoint.Y, BlackPoint.Z = 0.0, 0.0, 0.0
		}
		return true
	}

	// Create a roundtrip transformation using perceptual intent
	hRoundTrip := CreateRoundtripXForm(mm, hProfile, INTENT_PERCEPTUAL)
	if hRoundTrip == nil {
		if BlackPoint != nil {
			BlackPoint.X, BlackPoint.Y, BlackPoint.Z = 0.0, 0.0, 0.0
		}
		return false
	}

	// Perform the roundtrip transformation
	LabOutSlice := LabToSlice(LabOut)
	CmsDoTransform(mm, hRoundTrip, []float64{LabIn.L, LabIn.a, LabIn.b}, LabOutSlice, 1)
	LabOut = SliceToLab(LabOutSlice)
	// Clip Lab values to reasonable limits
	if LabOut.L > 50 {
		LabOut.L = 50
	}
	LabOut.a, LabOut.b = 0, 0

	// Free the transformation resource
	cmsDeleteTransform(mm, hRoundTrip)

	// Convert the output Lab to XYZ
	cmsLab2XYZ(nil, &BlackXYZ, &LabOut)

	// Store the result in the provided BlackPoint pointer
	if BlackPoint != nil {
		*BlackPoint = BlackXYZ
	}
	//fmt.Printf("END BlackPointUsingPerceptualBlack  BlackPoint.X %.7f, BlackPoint.Y %.7f, BlackPoint.Z %.7f\n ", (*BlackPoint).X, (*BlackPoint).Y, (*BlackPoint).Z)

	return true
}

// cmsDetectBlackPoint detects the black point for a given profile and intent.
// This function attempts to address the issues with broken black point tags in profiles.
// It ensures the chromaticity of the black point is neutral to avoid tints during compensation.
func cmsDetectBlackPoint(mm mem.Manager, BlackPoint *cmsCIEXYZ, hProfile CmsHPROFILE, Intent, dwFlags uint32) bool {
	//	fmt.Println("START cmsDetectBlackPoint")

	// Ensure the device class is adequate
	devClass := cmsGetDeviceClass(hProfile)
	if devClass == CmsSigLinkClass ||
		devClass == CmsSigAbstractClass ||
		devClass == CmsSigNamedColorClass {
		if BlackPoint != nil {
			BlackPoint.X, BlackPoint.Y, BlackPoint.Z = 0.0, 0.0, 0.0
		}
		return false
	}

	// Ensure the intent is adequate
	if Intent != INTENT_PERCEPTUAL &&
		Intent != INTENT_RELATIVE_COLORIMETRIC &&
		Intent != INTENT_SATURATION {
		if BlackPoint != nil {
			BlackPoint.X, BlackPoint.Y, BlackPoint.Z = 0.0, 0.0, 0.0
		}
		return false
	}

	// v4 profiles with perceptual and saturation intents have well-specified black points.
	// The black point tag is deprecated in v4.
	if cmsGetEncodedICCversion(hProfile) >= 0x4000000 &&
		(Intent == INTENT_PERCEPTUAL || Intent == INTENT_SATURATION) {

		// Use matrix shaper for relative colorimetric intent if applicable
		if cmsIsMatrixShaper(hProfile) {
			return BlackPointAsDarkerColorant(mm, hProfile, INTENT_RELATIVE_COLORIMETRIC, BlackPoint, 0)
		}

		// Use the fixed perceptual black for v4 profiles
		if BlackPoint != nil {
			BlackPoint.X = CmsPERCEPTUAL_BLACK_X
			BlackPoint.Y = CmsPERCEPTUAL_BLACK_Y
			BlackPoint.Z = CmsPERCEPTUAL_BLACK_Z
		}
		return true
	}

	// Handle v2 profiles and compute the black point based on the profile class
	if Intent == INTENT_RELATIVE_COLORIMETRIC &&
		cmsGetDeviceClass(hProfile) == CmsSigOutputClass &&
		CmsGetColorSpace(hProfile) == CmsSigCmykData {
		return BlackPointUsingPerceptualBlack(mm, BlackPoint, hProfile)
	}

	// Compute black point using the current intent
	return BlackPointAsDarkerColorant(mm, hProfile, Intent, BlackPoint, dwFlags)
}

// RootOfLeastSquaresFitQuadraticCurve calculates the root of a least squares fit quadratic curve to data.
// Reference: http://www.personal.psu.edu/jhm/f90/lectures/lsq2.html
func RootOfLeastSquaresFitQuadraticCurve(n int, x []float64, y []float64) float64 {
	var (
		sumX, sumX2, sumX3, sumX4 float64
		sumY, sumYX, sumYX2       float64
		d, a, b, c                float64
		m                         cmsMAT3
		v, res                    cmsVEC3
	)

	// A minimum of 4 data points is required for fitting
	if n < 4 {
		return 0
	}

	// Compute summations for the least squares calculation
	for i := 0; i < n; i++ {
		xn := x[i]
		yn := y[i]

		sumX += xn
		sumX2 += xn * xn
		sumX3 += xn * xn * xn
		sumX4 += xn * xn * xn * xn

		sumY += yn
		sumYX += yn * xn
		sumYX2 += yn * xn * xn
	}

	// Construct the matrix and vector for solving the quadratic coefficients
	cmsVEC3init(&m.V[0], float64(n), sumX, sumX2)
	cmsVEC3init(&m.V[1], sumX, sumX2, sumX3)
	cmsVEC3init(&m.V[2], sumX2, sumX3, sumX4)

	cmsVEC3init(&v, sumY, sumYX, sumYX2)

	// Solve the system of equations
	if !cmsMAT3solve(&res, &m, &v) {
		return 0
	}

	// Extract quadratic coefficients
	a = res.N[2]
	b = res.N[1]
	c = res.N[0]

	// Handle cases based on the value of 'a'
	if math.Abs(a) < 1.0e-10 {
		if math.Abs(b) < 1.0e-10 {
			return 0
		}
		// Linear solution
		return math.Min(50, math.Max(0, -c/b))
	} else {
		// Quadratic solution
		d = b*b - 4.0*a*c
		if d <= 0 {
			return 0
		} else {
			// Calculate the positive root of the quadratic equation
			rt := (-b + math.Sqrt(d)) / (2.0 * a)
			return math.Max(0, math.Min(50, rt))
		}
	}
}

// cmsDetectDestinationBlackPoint calculates the black point of a destination profile.
// This algorithm comes from the Adobe paper disclosing its black point compensation method.
func cmsDetectDestinationBlackPoint(mm mem.Manager, BlackPoint *cmsCIEXYZ, hProfile CmsHPROFILE, Intent, dwFlags uint32) bool {
	//fmt.Printf("start cmsDetectDestinationBlackPoint\n")
	var ColorSpace cmsColorSpaceSignature
	var hRoundTrip CmsHTRANSFORM
	var InitialLab, destLab, Lab cmsCIELab
	var inRamp, outRamp, yRamp, x, y [256]float64
	var MinL, MaxL, lo, hi float64
	var NearlyStraightMidrange bool
	var n, l int

	// Ensure the device class is adequate
	devClass := cmsGetDeviceClass(hProfile)
	if devClass == CmsSigLinkClass ||
		devClass == CmsSigAbstractClass ||
		devClass == CmsSigNamedColorClass {
		if BlackPoint != nil {
			BlackPoint.X, BlackPoint.Y, BlackPoint.Z = 0.0, 0.0, 0.0
		}
		return false
	}

	// Ensure the intent is adequate
	if Intent != INTENT_PERCEPTUAL &&
		Intent != INTENT_RELATIVE_COLORIMETRIC &&
		Intent != INTENT_SATURATION {
		if BlackPoint != nil {
			BlackPoint.X, BlackPoint.Y, BlackPoint.Z = 0.0, 0.0, 0.0
		}
		return false
	}

	// Handle v4 profiles with perceptual and saturation intents
	if cmsGetEncodedICCversion(hProfile) >= 0x4000000 &&
		(Intent == INTENT_PERCEPTUAL || Intent == INTENT_SATURATION) {

		if cmsIsMatrixShaper(hProfile) {
			return BlackPointAsDarkerColorant(mm, hProfile, INTENT_RELATIVE_COLORIMETRIC, BlackPoint, 0)
		}

		if BlackPoint != nil {
			BlackPoint.X = CmsPERCEPTUAL_BLACK_X
			BlackPoint.Y = CmsPERCEPTUAL_BLACK_Y
			BlackPoint.Z = CmsPERCEPTUAL_BLACK_Z
		}
		return true
	}

	// Check if the profile is LUT-based and its color space
	ColorSpace = CmsGetColorSpace(hProfile)
	if !cmsIsCLUT(hProfile, Intent, LCMS_USED_AS_OUTPUT) ||
		(ColorSpace != CmsSigGrayData &&
			ColorSpace != CmsSigRgbData &&
			ColorSpace != CmsSigCmykData) {
		return cmsDetectBlackPoint(mm, BlackPoint, hProfile, Intent, dwFlags)
	}

	// Set an initial guess
	if Intent == INTENT_RELATIVE_COLORIMETRIC {
		var IniXYZ cmsCIEXYZ
		if !cmsDetectBlackPoint(mm, &IniXYZ, hProfile, Intent, dwFlags) {
			return false
		}
		cmsXYZ2Lab(nil, &InitialLab, &IniXYZ)
	} else {
		InitialLab.L, InitialLab.a, InitialLab.b = 0, 0, 0
	}

	// Create a roundtrip transform
	hRoundTrip = CreateRoundtripXForm(mm, hProfile, Intent)
	if hRoundTrip == nil {
		return false
	}

	// Compute ramps
	for l = 0; l < 256; l++ {
		Lab.L = float64(l) * 100.0 / 255.0
		Lab.a = math.Min(50, math.Max(-50, InitialLab.a))
		Lab.b = math.Min(50, math.Max(-50, InitialLab.b))
		/*  fmt.Printf("Lab.L %.7f\n", Lab.L)
		    fmt.Printf("Lab.a %.7f\n", Lab.a)
		    fmt.Printf("Lab.b %.7f\n", Lab.b)*/
		CmsDoTransform(mm, hRoundTrip, &Lab, &destLab, 1)

		inRamp[l] = Lab.L
		outRamp[l] = destLab.L
	}

	// Make monotonic
	for l = 254; l > 0; l-- {
		outRamp[l] = math.Min(outRamp[l], outRamp[l+1])
	}

	// Validate monotonicity
	if !(outRamp[0] < outRamp[255]) {
		cmsDeleteTransform(mm, hRoundTrip)
		if BlackPoint != nil {
			BlackPoint.X, BlackPoint.Y, BlackPoint.Z = 0.0, 0.0, 0.0
		}
		return false
	}

	// Check midrange straightness for relative colorimetric intent
	NearlyStraightMidrange = true
	MinL, MaxL = outRamp[0], outRamp[255]
	if Intent == INTENT_RELATIVE_COLORIMETRIC {
		for l = 0; l < 256; l++ {
			if !(inRamp[l] <= MinL+0.2*(MaxL-MinL) ||
				math.Abs(inRamp[l]-outRamp[l]) < 4.0) {
				NearlyStraightMidrange = false
			}
		}

		if NearlyStraightMidrange {
			cmsLab2XYZ(nil, BlackPoint, &InitialLab)
			cmsDeleteTransform(mm, hRoundTrip)
			return true
		}
	}

	// Perform curve fitting
	for l = 0; l < 256; l++ {
		yRamp[l] = (outRamp[l] - MinL) / (MaxL - MinL)
	}

	// Set thresholds
	if Intent == INTENT_RELATIVE_COLORIMETRIC {
		lo, hi = 0.1, 0.5
	} else {
		lo, hi = 0.03, 0.25
	}

	// Capture shadow points
	n = 0
	for l = 0; l < 256; l++ {
		ff := yRamp[l]
		if ff >= lo && ff < hi {
			x[n], y[n] = inRamp[l], yRamp[l]
			n++
		}
	}

	// Validate points
	if n < 3 {
		cmsDeleteTransform(mm, hRoundTrip)
		if BlackPoint != nil {
			BlackPoint.X, BlackPoint.Y, BlackPoint.Z = 0.0, 0.0, 0.0
		}
		return false
	}

	// Fit the curve and get the vertex
	Lab.L = RootOfLeastSquaresFitQuadraticCurve(n, x[:n], y[:n])
	if Lab.L < 0.0 {
		Lab.L = 0
	}

	Lab.a = InitialLab.a
	Lab.b = InitialLab.b
	cmsLab2XYZ(nil, BlackPoint, &Lab)

	cmsDeleteTransform(mm, hRoundTrip)
	//fmt("end cmsDetectDestinationBlackPoint\n")

	return true
}
