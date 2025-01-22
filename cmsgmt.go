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

	SampledPoints := (*float32)(cmsCalloc(ContextID, nPoints, uint32(unsafe.Sizeof(float32(0)))))

	for i := uint32(0); i < nPoints; i++ {
		cmyk := [4]float32{0, 0, 0, float32((float64(i) * 100.0) / float64(nPoints-1))}
		var Lab cmsCIELab
		cmsDoTransform(xform, unsafe.Pointer(&cmyk[0]), unsafe.Pointer(&Lab), 1)

		// Calculate the offset for the current index and assign the value
		*(*float32)(unsafe.Add(unsafe.Pointer(SampledPoints), uintptr(i)*unsafe.Sizeof(float32(0)))) = float32(1.0 - Lab.L/100.0)
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
func EstimateTAC(in []uint16, out []uint16, cargo unsafe.Pointer) int32 {
	bp := (*cmsTACestimator)(cargo)
	var roundTrip [cmsMAXCHANNELS]float32
	var sum float32

	// Evaluate the transform
	cmsDoTransform(bp.hRoundTrip, unsafe.Pointer(&in[0]), unsafe.Pointer(&roundTrip[0]), 1)

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
	if cmsGetDeviceClass(unsafe.Pointer(hProfile)) != cmsSigOutputClass {
		return 0
	}

	// Create a fake formatter for result
	dwFormatter = cmsFormatterForColorspaceOfProfile(unsafe.Pointer(hProfile), 4, true)

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

	if !cmsSliceSpace16(3, gridPoints[:], EstimateTAC, unsafe.Pointer(&bp)) {
		bp.MaxTAC = 0
	}

	cmsDeleteTransform(bp.hRoundTrip)

	// Results in %
	return float64(bp.MaxTAC)
}

// Gamut LUT Creation -----------------------------------------------------------------------------------------

// Define the GAMUTCHAIN structure
type GAMUTCHAIN struct {
	hInput    cmsHTRANSFORM // From whatever input color space. 16 bits to DBL
	hForward  cmsHTRANSFORM // Transforms going from Lab to colorant
	hReverse  cmsHTRANSFORM // Transforms going from colorant back to Lab
	Threshold float64       // The threshold after which is considered out of gamut
}

const ERR_THRESHOLD = 5

// This sampler does compute gamut boundaries by comparing original
// values with a transform going back and forth. Values above ERR_THRESHOLD
// of maximum are considered out of gamut.

// GamutSampler computes gamut boundaries by comparing original values with a transform
// going back and forth. Values above ERR_THRESHOLD are considered out of gamut.
func GamutSampler(In []uint16, Out []uint16, Cargo unsafe.Pointer) int32 {
	t := (*GAMUTCHAIN)(Cargo)
	var LabIn1, LabOut1 cmsCIELab
	var LabIn2, LabOut2 cmsCIELab
	var Proof [cmsMAXCHANNELS]uint16
	var Proof2 [cmsMAXCHANNELS]uint16
	var dE1, dE2, ErrorRatio float64

	// Assume in-gamut by default.
	ErrorRatio = 1.0

	// Convert input to Lab
	cmsDoTransform(t.hInput, unsafe.Pointer(&In[0]), unsafe.Pointer(&LabIn1), 1)

	// Convert from PCS to colorant. This always returns in-gamut values.
	cmsDoTransform(t.hForward, unsafe.Pointer(&LabIn1), unsafe.Pointer(&Proof[0]), 1)

	// Convert from colorant to PCS.
	cmsDoTransform(t.hReverse, unsafe.Pointer(&Proof[0]), unsafe.Pointer(&LabOut1), 1)

	// Copy LabOut1 to LabIn2
	memmove(unsafe.Pointer(&LabIn2), unsafe.Pointer(&LabOut1), unsafe.Sizeof(cmsCIELab{}))

	// Forward and reverse transform again, using LabOut1 as input
	cmsDoTransform(t.hForward, unsafe.Pointer(&LabOut1), unsafe.Pointer(&Proof2[0]), 1)
	cmsDoTransform(t.hReverse, unsafe.Pointer(&Proof2[0]), unsafe.Pointer(&LabOut2), 1)

	// Compute differences
	dE1 = cmsDeltaE(&LabIn1, &LabOut1)
	dE2 = cmsDeltaE(&LabIn2, &LabOut2)

	// Determine gamut status based on differences
	if dE1 < t.Threshold && dE2 < t.Threshold {
		Out[0] = 0
	} else {
		if dE1 < t.Threshold && dE2 > t.Threshold {
			Out[0] = 0
		} else if dE1 > t.Threshold && dE2 < t.Threshold {
			Out[0] = uint16(cmsQuickFloor(dE1 - t.Threshold + 0.5))
		} else {
			if dE2 == 0.0 {
				ErrorRatio = dE1
			} else {
				ErrorRatio = dE1 / dE2
			}

			if ErrorRatio > t.Threshold {
				Out[0] = uint16(cmsQuickFloor(ErrorRatio - t.Threshold + 0.5))
			} else {
				Out[0] = 0
			}
		}
	}

	return 1 // TRUE
}

// Does compute a gamut LUT going back and forth across pcs -> relativ. colorimetric intent -> pcs
// the dE obtained is then annotated on the LUT. Values truly out of gamut are clipped to dE = 0xFFFE
// and values changed are supposed to be handled by any gamut remapping, so, are out of gamut as well.
//
// **WARNING: This algorithm does assume that gamut remapping algorithms does NOT move in-gamut colors,
// of course, many perceptual and saturation intents does not work in such way, but relativ. ones should.
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
	var hLab cmsHPROFILE
	var Gamut *cmsPipeline
	var CLUT *cmsStage
	var dwFormat uint32
	var Chain GAMUTCHAIN
	var nGridpoints uint32
	var nChannels int32
	var ColorSpace cmsColorSpaceSignature
	var i uint32
	var ProfileList [256]cmsHPROFILE
	var BPCList [256]bool
	var AdaptationList [256]float64
	var IntentList [256]uint32

	// Initialize Chain to zero
	memset(unsafe.Pointer(&Chain), 0, unsafe.Sizeof(Chain))

	// Validate PCS position
	if nGamutPCSposition <= 0 || nGamutPCSposition > 255 {
		cmsSignalError(unsafe.Pointer(ContextID), cmsERROR_RANGE, "Wrong position of PCS. 1..255 expected")
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

	ColorSpace = cmsGetColorSpace(unsafe.Pointer(hGamut))
	nChannels = cmsChannelsOfColorSpace(ColorSpace)
	nGridpoints = cmsReasonableGridpointsByColorspace(ColorSpace, cmsFLAGS_HIGHRESPRECALC)
	dwFormat = CHANNELS_SH(uint32(nChannels)) | BYTES_SH(2)

	// Create the input transform
	Chain.hInput = cmsHTRANSFORM(cmsCreateExtendedTransform(
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
	))

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
		ContextID   cmsContext
		hXYZ        cmsHPROFILE
		xform       cmsHTRANSFORM
		YCurve      *cmsToneCurve
		rgb         [256][3]uint16
		XYZ         [256]cmsCIEXYZ
		YNormalized [256]float32
		gamma       float64
		cls         cmsProfileClassSignature
	)

	// Ensure the profile is in RGB color space
	if cmsGetColorSpace(unsafe.Pointer(hProfile)) != cmsSigRgbData {
		return -1
	}

	// Check the profile class
	cls = cmsGetDeviceClass(unsafe.Pointer(hProfile))
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
	for i := uint8(0); i <= uint8(255); i++ {
		rgb[i][0] = FROM_8_TO_16(i)
		rgb[i][1] = FROM_8_TO_16(i)
		rgb[i][2] = FROM_8_TO_16(i)
	}

	// Perform the transform
	cmsDoTransform(xform, unsafe.Pointer(&rgb[0]), unsafe.Pointer(&XYZ[0]), 256)

	// Clean up the transform and XYZ profile
	cmsDeleteTransform(xform)
	cmsCloseProfile(hXYZ)

	// Normalize the Y component
	for i := 0; i < 256; i++ {
		YNormalized[i] = float32(XYZ[i].Y)
	}

	// Build a tone curve from the normalized Y values
	YCurve = cmsBuildTabulatedToneCurveFloat(ContextID, 256, &YNormalized[0])
	if YCurve == nil {
		return -1
	}

	// Estimate gamma
	gamma = cmsEstimateGamma(YCurve, threshold)

	// Free the tone curve and return the gamma value
	cmsFreeToneCurve(YCurve)
	return gamma
}
