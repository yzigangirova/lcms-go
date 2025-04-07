package golcms

// This file contains routines for resampling and LUT optimization, black point detection
// and black preservation.

import (
	"fmt"
	"math"
	"unsafe"
)

// CreateRoundtripXForm creates a PCS -> PCS round trip transform, always using relative intent on the device -> PCS.
func CreateRoundtripXForm(hProfile CmsHPROFILE, nIntent uint32) CmsHTRANSFORM {
	ContextID := cmsGetProfileContextID(hProfile)
	hLab := cmsCreateLab4ProfileTHR(ContextID, nil)
	var xform CmsHTRANSFORM
	BPC := [4]bool{false, false, false, false}
	States := [4]float64{1.0, 1.0, 1.0, 1.0}
	hProfiles := [4]CmsHPROFILE{hLab, hProfile, hProfile, hLab}
	Intents := [4]uint32{INTENT_RELATIVE_COLORIMETRIC, nIntent, INTENT_RELATIVE_COLORIMETRIC, INTENT_RELATIVE_COLORIMETRIC}
	xform = CmsHTRANSFORM(cmsCreateExtendedTransform(
		ContextID, 4, hProfiles[:], BPC[:], Intents[:],
		States[:], nil, 0, TYPE_Lab_DBL, TYPE_Lab_DBL, cmsFLAGS_NOCACHE|cmsFLAGS_NOOPTIMIZE,
	))

	CmsCloseProfile(hLab)
	return xform
}

// BlackPointAsDarkerColorant uses darker colorants to obtain the black point.
// This works in the relative colorimetric intent and assumes more ink results in darker colors. No ink limit is assumed.
func BlackPointAsDarkerColorant(hInput CmsHPROFILE, Intent uint32, BlackPoint *cmsCIEXYZ, dwFlags uint32) bool {
	var Black []uint16
	var xform CmsHTRANSFORM
	var Lab cmsCIELab
	var BlackXYZ cmsCIEXYZ
	var dwFormat uint32
	var nChannels uint32
	var Space cmsColorSpaceSignature
	ContextID := cmsGetProfileContextID(hInput)
	fmt.Println("START BlackPointAsDarkerColorant")

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
	hLab := cmsCreateLab2ProfileTHR(ContextID, nil)
	if hLab == nil {
		if BlackPoint != nil {
			BlackPoint.X, BlackPoint.Y, BlackPoint.Z = 0.0, 0.0, 0.0
		}
		return false
	}

	// Create the transform.

	fmt.Println("start: xform = cmsCreateTransformTHR")
	xform = cmsCreateTransformTHR(
		ContextID, hInput, dwFormat, hLab, TYPE_Lab_DBL,
		Intent, cmsFLAGS_NOOPTIMIZE|cmsFLAGS_NOCACHE,
	)
	fmt.Println("end: xform = cmsCreateTransformTHR")

	CmsCloseProfile(hLab)

	if xform == nil {
		if BlackPoint != nil {
			BlackPoint.X, BlackPoint.Y, BlackPoint.Z = 0.0, 0.0, 0.0
		}
		return false
	}

	// Convert black to Lab.
	LabSlice := LabToSlice(Lab)
	CmsDoTransform(xform, Black, LabSlice, 1)
	Lab = SliceToLab(LabSlice)

	// Force it to be neutral; check for inconsistencies.
	Lab.a = 0
	Lab.b = 0
	if Lab.L > 50 || Lab.L < 0 {
		Lab.L = 0
	}

	// Free the resources.
	cmsDeleteTransform(xform)

	// Convert from Lab (now clipped) to XYZ.
	cmsLab2XYZ(nil, &BlackXYZ, &Lab)

	if BlackPoint != nil {
		*BlackPoint = BlackXYZ
	}
	fmt.Println("END BlackPointAsDarkerColorant BlackPoint.X, BlackPoint.Y, BlackPoint.Z ", (*BlackPoint).X, (*BlackPoint).Y, (*BlackPoint).Z)

	return true
}

// BlackPointUsingPerceptualBlack calculates the black point of an output CMYK profile,
// discounting any ink-limiting embedded in the profile.
// The process involves a roundtrip transformation using perceptual intent:
// Lab (0, 0, 0) -> [Perceptual] Profile -> CMYK -> [Rel. Colorimetric] Profile -> Lab.
func BlackPointUsingPerceptualBlack(BlackPoint *cmsCIEXYZ, hProfile CmsHPROFILE) bool {
	fmt.Println("START BlackPointUsingPerceptualBlack")
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
	hRoundTrip := CreateRoundtripXForm(hProfile, INTENT_PERCEPTUAL)
	if hRoundTrip == nil {
		if BlackPoint != nil {
			BlackPoint.X, BlackPoint.Y, BlackPoint.Z = 0.0, 0.0, 0.0
		}
		return false
	}

	// Perform the roundtrip transformation
	LabOutSlice := LabToSlice(LabOut)
	CmsDoTransform(hRoundTrip, []float64{LabIn.L, LabIn.a, LabIn.b}, LabOutSlice, 1)
	LabOut = SliceToLab(LabOutSlice)
	// Clip Lab values to reasonable limits
	if LabOut.L > 50 {
		LabOut.L = 50
	}
	LabOut.a, LabOut.b = 0, 0

	// Free the transformation resource
	cmsDeleteTransform(hRoundTrip)

	// Convert the output Lab to XYZ
	cmsLab2XYZ(nil, &BlackXYZ, &LabOut)

	// Store the result in the provided BlackPoint pointer
	if BlackPoint != nil {
		*BlackPoint = BlackXYZ
	}
	fmt.Println("END BlackPointUsingPerceptualBlack  BlackPoint.X, BlackPoint.Y, BlackPoint.Z ", (*BlackPoint).X, (*BlackPoint).Y, (*BlackPoint).Z)

	return true
}

// cmsDetectBlackPoint detects the black point for a given profile and intent.
// This function attempts to address the issues with broken black point tags in profiles.
// It ensures the chromaticity of the black point is neutral to avoid tints during compensation.
func cmsDetectBlackPoint(BlackPoint *cmsCIEXYZ, hProfile CmsHPROFILE, Intent, dwFlags uint32) bool {
	fmt.Println("START cmsDetectBlackPoint")

	// Ensure the device class is adequate
	devClass := cmsGetDeviceClass(hProfile)
	if devClass == cmsSigLinkClass ||
		devClass == cmsSigAbstractClass ||
		devClass == cmsSigNamedColorClass {
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
			return BlackPointAsDarkerColorant(hProfile, INTENT_RELATIVE_COLORIMETRIC, BlackPoint, 0)
		}

		// Use the fixed perceptual black for v4 profiles
		if BlackPoint != nil {
			BlackPoint.X = cmsPERCEPTUAL_BLACK_X
			BlackPoint.Y = cmsPERCEPTUAL_BLACK_Y
			BlackPoint.Z = cmsPERCEPTUAL_BLACK_Z
		}
		return true
	}

	// Handle v2 profiles and compute the black point based on the profile class
	if Intent == INTENT_RELATIVE_COLORIMETRIC &&
		cmsGetDeviceClass(hProfile) == cmsSigOutputClass &&
		CmsGetColorSpace(hProfile) == cmsSigCmykData {
		return BlackPointUsingPerceptualBlack(BlackPoint, hProfile)
	}

	// Compute black point using the current intent
	return BlackPointAsDarkerColorant(hProfile, Intent, BlackPoint, dwFlags)
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
func cmsDetectDestinationBlackPoint(BlackPoint *cmsCIEXYZ, hProfile CmsHPROFILE, Intent, dwFlags uint32) bool {
	var ColorSpace cmsColorSpaceSignature
	var hRoundTrip CmsHTRANSFORM
	var InitialLab, destLab, Lab cmsCIELab
	var inRamp, outRamp, yRamp, x, y [256]float64
	var MinL, MaxL, lo, hi float64
	var NearlyStraightMidrange bool
	var n, l int

	// Ensure the device class is adequate
	devClass := cmsGetDeviceClass(hProfile)
	if devClass == cmsSigLinkClass ||
		devClass == cmsSigAbstractClass ||
		devClass == cmsSigNamedColorClass {
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
			return BlackPointAsDarkerColorant(hProfile, INTENT_RELATIVE_COLORIMETRIC, BlackPoint, 0)
		}

		if BlackPoint != nil {
			BlackPoint.X = cmsPERCEPTUAL_BLACK_X
			BlackPoint.Y = cmsPERCEPTUAL_BLACK_Y
			BlackPoint.Z = cmsPERCEPTUAL_BLACK_Z
		}
		return true
	}

	// Check if the profile is LUT-based and its color space
	ColorSpace = CmsGetColorSpace(hProfile)
	if !cmsIsCLUT(hProfile, Intent, LCMS_USED_AS_OUTPUT) ||
		(ColorSpace != cmsSigGrayData &&
			ColorSpace != cmsSigRgbData &&
			ColorSpace != cmsSigCmykData) {
		return cmsDetectBlackPoint(BlackPoint, hProfile, Intent, dwFlags)
	}

	// Set an initial guess
	if Intent == INTENT_RELATIVE_COLORIMETRIC {
		var IniXYZ cmsCIEXYZ
		if !cmsDetectBlackPoint(&IniXYZ, hProfile, Intent, dwFlags) {
			return false
		}
		cmsXYZ2Lab(nil, &InitialLab, &IniXYZ)
	} else {
		InitialLab.L, InitialLab.a, InitialLab.b = 0, 0, 0
	}

	// Create a roundtrip transform
	hRoundTrip = CreateRoundtripXForm(hProfile, Intent)
	if hRoundTrip == nil {
		return false
	}

	// Compute ramps
	for l = 0; l < 256; l++ {
		Lab.L = float64(l) * 100.0 / 255.0
		Lab.a = math.Min(50, math.Max(-50, InitialLab.a))
		Lab.b = math.Min(50, math.Max(-50, InitialLab.b))

		CmsDoTransform(hRoundTrip, unsafe.Pointer(&Lab), unsafe.Pointer(&destLab), 1)

		inRamp[l] = Lab.L
		outRamp[l] = destLab.L
	}

	// Make monotonic
	for l = 254; l > 0; l-- {
		outRamp[l] = math.Min(outRamp[l], outRamp[l+1])
	}

	// Validate monotonicity
	if !(outRamp[0] < outRamp[255]) {
		cmsDeleteTransform(hRoundTrip)
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
			cmsDeleteTransform(hRoundTrip)
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
		cmsDeleteTransform(hRoundTrip)
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

	cmsDeleteTransform(hRoundTrip)
	return true
}
