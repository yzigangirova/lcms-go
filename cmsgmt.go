package golcms

import (
	"unsafe"
)
// Append a Lab identity after the given sequence of profiles and return the transform.
// Lab profile is closed, rest of the profiles are kept open.
func cmsChain2Lab(ContextID cmsContext,
	nProfiles uint32,
	InputFormat uint32,
	OutputFormat uint32,
	Intents []uint32,
	hProfiles []cmsHPROFILE,
	BPC []bool,
	AdaptationStates []float64,
	dwFlags uint32) cmsHTRANSFORM {

	if nProfiles > 254 {
		return nil // Limit exceeded: 254 + 1 (Lab) = 255
	}

	// Create Lab profile
	hLab := cmsCreateLab4ProfileTHR(ContextID, nil)
	if hLab == nil {
		return nil
	}

	// Prepare arrays for the extended transform
	var ProfileList [256]cmsHPROFILE
	var BPCList [256]bool
	var AdaptationList [256]float64
	var IntentList [256]uint32

	// Copy input profiles and their parameters
	for i := uint32(0); i < nProfiles; i++ {
		ProfileList[i] = hProfiles[i]
		BPCList[i] = BPC[i]
		AdaptationList[i] = AdaptationStates[i]
		IntentList[i] = Intents[i]
	}

	// Append Lab profile at the end
	ProfileList[nProfiles] = hLab
	BPCList[nProfiles] = false
	AdaptationList[nProfiles] = 1.0
	IntentList[nProfiles] = INTENT_RELATIVE_COLORIMETRIC

	// Create the transform
	xform := cmsCreateExtendedTransform(ContextID, nProfiles+1, ProfileList[:],
		BPCList[:],
		IntentList[:],
		AdaptationList[:],
		nil, 0,
		InputFormat,
		OutputFormat,
		dwFlags)

	cmsCloseProfile(hLab)
	return cmsHTRANSFORM(xform)
}

// Compute K -> L* relationship. Flags may include black point compensation.
// In this case, the relationship is assumed from the profile with BPC to a black point zero.
func ComputeKToLstar(ContextID cmsContext,
	nPoints uint32,
	nProfiles uint32,
	Intents []uint32,
	hProfiles []cmsHPROFILE,
	BPC []bool,
	AdaptationStates []float64,
	dwFlags uint32) *cmsToneCurve {

	xform := cmsChain2Lab(ContextID, nProfiles, TYPE_CMYK_FLT, TYPE_Lab_DBL, Intents, hProfiles, BPC, AdaptationStates, dwFlags)
	if xform == nil {
		return nil
	}

	SampledPoints := make([]float32, nPoints)
	defer cmsFree(ContextID, SampledPoints)

	for i := uint32(0); i < nPoints; i++ {
		cmyk := [4]float32{0, 0, 0, float32((float64(i) * 100.0) / float64(nPoints-1))}
		var Lab cmsCIELab
		cmsDoTransform(xform, cmyk[:], &Lab, 1)
		SampledPoints[i] = float32(1.0 - Lab.L/100.0) // Negate K for easier operation
	}

	out := cmsBuildTabulatedToneCurveFloat(ContextID, nPoints, SampledPoints)
	cmsDeleteTransform(xform)
	return out
}

// Compute Black tone curve on a CMYK -> CMYK transform. This is done by
// using the proof direction on both profiles to find K->L* relationship
// then joining both curves. dwFlags may include black point compensation.
func cmsBuildKToneCurve(ContextID cmsContext,
	nPoints uint32,
	nProfiles uint32,
	Intents []uint32,
	hProfiles []cmsHPROFILE,
	BPC []bool,
	AdaptationStates []float64,
	dwFlags uint32) *cmsToneCurve {

	// Ensure CMYK -> CMYK
	if cmsGetColorSpace(unsafe.Pointer(hProfiles[0])) != cmsSigCmykData || cmsGetColorSpace(unsafe.Pointer(hProfiles[nProfiles-1])) != cmsSigCmykData {
		return nil
	}

	// Ensure the last profile is an output profile
	if cmsGetDeviceClass(unsafe.Pointer(hProfiles[nProfiles-1])) != cmsSigOutputClass {
		return nil
	}

	// Compute K->L* relationships for the input and output
	in := ComputeKToLstar(ContextID, nPoints, nProfiles-1, Intents, hProfiles, BPC, AdaptationStates, dwFlags)
	if in == nil {
		return nil
	}

	out := ComputeKToLstar(ContextID, nPoints, 1,
		Intents[nProfiles-1:nProfiles],
		hProfiles[nProfiles-1:nProfiles],
		BPC[nProfiles-1:nProfiles],
		AdaptationStates[nProfiles-1:nProfiles],
		dwFlags)
	if out == nil {
		cmsFreeToneCurve(in)
		return nil
	}

	// Join the input and output curves
	KTone := cmsJoinToneCurve(ContextID, in, out, nPoints)
	cmsFreeToneCurve(in)
	cmsFreeToneCurve(out)

	if KTone == nil {
		return nil
	}

	// Ensure the resulting tone curve is monotonic
	if !cmsIsToneCurveMonotonic(KTone) {
		cmsFreeToneCurve(KTone)
		return nil
	}

	return KTone
}

type cmsTACestimator struct {
	nOutputChans uint32
	hRoundTrip   cmsHTRANSFORM
	MaxTAC       float32
	MaxInput     [cmsMAXCHANNELS]float32
}


// EstimateTAC is the callback function to calculate maximum TAC.
func EstimateTAC(in []uint16, out []uint16, cargo interface{}) int {
	bp := cargo.(*cmsTACestimator)
	var roundTrip [cmsMAXCHANNELS]float32
	var sum float32

	// Evaluate the transform
	cmsDoTransform(bp.hRoundTrip, in, &roundTrip, 1)

	// Sum all amounts of ink
	for i := 0; i < int(bp.nOutputChans); i++ {
		sum += roundTrip[i]
	}

	// If above maximum, keep track of input values
	if sum > bp.MaxTAC {
		bp.MaxTAC = sum
		for i := 0; i < int(bp.nOutputChans); i++ {
			bp.MaxInput[i] = float32(in[i])
		}
	}

	return 1 // Return TRUE
}

// cmsDetectTAC detects the total area coverage (TAC) of the profile.
func cmsDetectTAC(hProfile cmsHPROFILE) float64 {
	var bp cmsTACestimator
	var dwFormatter uint32
	var gridPoints [MAX_INPUT_DIMENSIONS]uint32
	var contextID cmsContext

	contextID = cmsGetProfileContextID(hProfile)

	// TAC only works on output profiles
	if cmsGetDeviceClass(hProfile) != cmsSigOutputClass {
		return 0
	}

	// Create a fake formatter for result
	dwFormatter = cmsFormatterForColorspaceOfProfile(hProfile, 4, true)

	// Unsupported color space?
	if dwFormatter == 0 {
		return 0
	}

	bp.nOutputChans = T_CHANNELS(dwFormatter)
	bp.MaxTAC = 0 // Initial TAC is 0

	// For safety
	if bp.nOutputChans >= cmsMAXCHANNELS {
		return 0
	}

	hLab := cmsCreateLab4ProfileTHR(contextID, nil)
	if hLab == nil {
		return 0
	}

	// Setup a roundtrip on perceptual intent in output profile for TAC estimation
	bp.hRoundTrip = cmsCreateTransformTHR(
		contextID,
		hLab,
		TYPE_Lab_16,
		hProfile,
		dwFormatter,
		INTENT_PERCEPTUAL,
		cmsFLAGS_NOOPTIMIZE|cmsFLAGS_NOCACHE,
	)
	cmsCloseProfile(hLab)

	if bp.hRoundTrip == nil {
		return 0
	}

	// For L* we only need black and white. For C* we need many points.
	gridPoints[0] = 6
	gridPoints[1] = 74
	gridPoints[2] = 74

	if !cmsSliceSpace16(3, gridPoints, EstimateTAC, &bp) {
		bp.MaxTAC = 0
	}

	cmsDeleteTransform(bp.hRoundTrip)

	// Results in %
	return float64(bp.MaxTAC)
}



// Gamut LUT Creation -----------------------------------------------------------------------------------------

// Used by gamut & softproofing
func cmsCreateGamutCheckPipeline(
	ContextID cmsContext,
	hProfiles []cmsHPROFILE,
	BPC []bool,
	Intents []uint32,
	AdaptationStates []float64,
	nGamutPCSposition uint32,
	hGamut cmsHPROFILE,
) *cmsPipeline {
	var hLab *cmsHPROFILE
	var Gamut *cmsPipeline
	var CLUT *cmsStage
	var dwFormat uint32
	var Chain GAMUTCHAIN
	var nGridpoints uint32
	var nChannels int32
	var ColorSpace cmsColorSpaceSignature
	var i uint32
	var ProfileList [256]*cmsHPROFILE
	var BPCList [256]bool
	var AdaptationList [256]float64
	var IntentList [256]uint32

	// Initialize Chain to zero
	memset(unsafe.Pointer(&Chain), 0, unsafe.Sizeof(Chain))

	// Validate PCS position
	if nGamutPCSposition <= 0 || nGamutPCSposition > 255 {
		cmsSignalError(ContextID, cmsERROR_RANGE, "Wrong position of PCS. 1..255 expected, %d found.", nGamutPCSposition)
		return nil
	}

	hLab = cmsCreateLab4ProfileTHR(ContextID, nil)
	if hLab == nil {
		return nil
	}

	// Determine the threshold
	if cmsIsMatrixShaper(hGamut) {
		Chain.Threshold = 1.0
	} else {
		Chain.Threshold = ERR_THRESHOLD
	}

	// Copy parameters
	for i = 0; i < nGamutPCSposition; i++ {
		ProfileList[i] = hProfiles[i]
		BPCList[i] = BPC[i]
		AdaptationList[i] = AdaptationStates[i]
		IntentList[i] = Intents[i]
	}

	// Fill Lab identity
	ProfileList[nGamutPCSposition] = hLab
	BPCList[nGamutPCSposition] = false
	AdaptationList[nGamutPCSposition] = 1.0
	IntentList[nGamutPCSposition] = INTENT_RELATIVE_COLORIMETRIC

	ColorSpace = cmsGetColorSpace(hGamut)
	nChannels = cmsChannelsOfColorSpace(ColorSpace)
	nGridpoints = cmsReasonableGridpointsByColorspace(ColorSpace, cmsFLAGS_HIGHRESPRECALC)
	dwFormat = CHANNELS_SH(uint32(nChannels)) | BYTES_SH(2)

	// Create the input transform
	Chain.hInput = cmsCreateExtendedTransform(
		ContextID,
		nGamutPCSposition+1,
		ProfileList[:],
		BPCList[:],
		IntentList[:],
		AdaptationList[:],
		nil,
		0,
		dwFormat,
		TYPE_Lab_DBL,
		cmsFLAGS_NOCACHE,
	)

	// Create the forward step
	Chain.hForward = cmsCreateTransformTHR(
		ContextID,
		hLab, TYPE_Lab_DBL,
		hGamut, dwFormat,
		INTENT_RELATIVE_COLORIMETRIC,
		cmsFLAGS_NOCACHE,
	)

	// Create the backwards step
	Chain.hReverse = cmsCreateTransformTHR(
		ContextID,
		hGamut, dwFormat,
		hLab, TYPE_Lab_DBL,
		INTENT_RELATIVE_COLORIMETRIC,
		cmsFLAGS_NOCACHE,
	)

	// Verify all steps are created successfully
	if Chain.hInput != nil && Chain.hForward != nil && Chain.hReverse != nil {
		// Compute gamut LUT
		Gamut = cmsPipelineAlloc(ContextID, 3, 1)
		if Gamut != nil {
			CLUT = cmsStageAllocCLut16bit(ContextID, nGridpoints, uint32(nChannels), 1, nil)
			if !cmsPipelineInsertStage(Gamut, cmsAT_BEGIN, CLUT) {
				cmsPipelineFree(Gamut)
				Gamut = nil
			} else {
				cmsStageSampleCLut16bit(CLUT, GamutSampler, unsafe.Pointer(&Chain), 0)
			}
		}
	} else {
		Gamut = nil // Failed to create transform
	}

	// Free resources
	if Chain.hInput != nil {
		cmsDeleteTransform(Chain.hInput)
	}
	if Chain.hForward != nil {
		cmsDeleteTransform(Chain.hForward)
	}
	if Chain.hReverse != nil {
		cmsDeleteTransform(Chain.hReverse)
	}
	if hLab != nil {
		cmsCloseProfile(hLab)
	}

	// Return the computed LUT
	return Gamut
}
// cmsDetectRGBProfileGamma detects whether a given ICC profile works in linear (gamma 1.0) space.
// It uses least squares fitting to estimate gamma for a synthetic gray (R=G=B).
// If gamma is close to 1.0, RGB is linear. On unsupported profiles, -1 is returned.
func cmsDetectRGBProfileGamma(hProfile cmsHPROFILE, threshold float64) float64 {
	var (
		ContextID    cmsContext
		hXYZ         cmsHPROFILE
		xform        cmsHTRANSFORM
		YCurve       *cmsToneCurve
		rgb          [256][3]uint16
		XYZ          [256]cmsCIEXYZ
		YNormalized  [256]float32
		gamma        float64
		cls          cmsProfileClassSignature
	)

	// Ensure the profile is in RGB color space
	if cmsGetColorSpace(unsafe.Pointer(hProfile)) != cmsSigRgbData {
		return -1
	}

	// Check the profile class
	cls = cmsGetDeviceClass(hProfile)
	if cls != cmsSigInputClass && cls != cmsSigDisplayClass &&
		cls != cmsSigOutputClass && cls != cmsSigColorSpaceClass {
		return -1
	}

	// Obtain the context ID and create an XYZ profile
	ContextID = cmsGetProfileContextID(hProfile)
	hXYZ = cmsCreateXYZProfileTHR(ContextID)
	if hXYZ == nil {
		return -1
	}

	// Create a transform from RGB to XYZ
	xform = cmsCreateTransformTHR(ContextID, hProfile, TYPE_RGB_16, hXYZ, TYPE_XYZ_DBL,
		INTENT_RELATIVE_COLORIMETRIC, cmsFLAGS_NOOPTIMIZE)

	if xform == nil {
		cmsCloseProfile(hXYZ)
		return -1
	}

	// Generate a synthetic gray (R=G=B) ramp
	for i := 0; i < 256; i++ {
		rgb[i][0] = FROM_8_TO_16(i)
		rgb[i][1] = FROM_8_TO_16(i)
		rgb[i][2] = FROM_8_TO_16(i)
	}

	// Perform the transform
	cmsDoTransform(xform, rgb[:], XYZ[:], 256)

	// Clean up the transform and XYZ profile
	cmsDeleteTransform(xform)
	cmsCloseProfile(hXYZ)

	// Normalize the Y component
	for i := 0; i < 256; i++ {
		YNormalized[i] = float32(XYZ[i].Y)
	}

	// Build a tone curve from the normalized Y values
	YCurve = cmsBuildTabulatedToneCurveFloat(ContextID, 256, YNormalized[:])
	if YCurve == nil {
		return -1
	}

	// Estimate gamma
	gamma = cmsEstimateGamma(YCurve, threshold)

	// Free the tone curve and return the gamma value
	cmsFreeToneCurve(YCurve)
	return gamma
}
