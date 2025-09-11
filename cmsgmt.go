package golcms

import (
	//"unsafe"
	"arena"
	//"fmt"
)

// Append a Lab identity after the given sequence of profiles and return the transform.
// Lab profile is closed, rest of the profiles are kept open.
func cmsChain2Lab(ar *arena.Arena, ContextID CmsContext,
	nProfiles uint32,
	InputFormat uint32,
	OutputFormat uint32,
	Intents []uint32,
	hProfiles []CmsHPROFILE,
	BPC []bool,
	AdaptationStates []float64,
	dwFlags uint32) CmsHTRANSFORM {

	if nProfiles > 254 {
		return nil // Limit exceeded: 254 + 1 (Lab) = 255
	}

	// Create Lab profile
	hLab := cmsCreateLab4ProfileTHR(ar,ContextID, nil)
	if hLab == nil {
		return nil
	}

	// Prepare arrays for the extended transform
	var ProfileList [256]CmsHPROFILE
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
	xform := cmsCreateExtendedTransform(ar,ContextID, nProfiles+1, ProfileList[:],
		BPCList[:],
		IntentList[:],
		AdaptationList[:],
		nil, 0,
		InputFormat,
		OutputFormat,
		dwFlags)

	CmsCloseProfile(ar, hLab)
	return CmsHTRANSFORM(xform)
}

// Compute K -> L* relationship. Flags may include black point compensation.
// In this case, the relationship is assumed from the profile with BPC to a black point zero.
func ComputeKToLstar(ar *arena.Arena, ContextID CmsContext,
	nPoints uint32,
	nProfiles uint32,
	Intents []uint32,
	hProfiles []CmsHPROFILE,
	BPC []bool,
	AdaptationStates []float64,
	dwFlags uint32) *CmsToneCurve {

	xform := cmsChain2Lab(ar, ContextID, nProfiles, TYPE_CMYK_FLT, TYPE_Lab_DBL, Intents, hProfiles, BPC, AdaptationStates, dwFlags)
	if xform == nil {
		return nil
	}

	SampledPoints := make([]float32, nPoints)

	for i := uint32(0); i < nPoints; i++ {
		cmyk := [4]float32{0, 0, 0, float32((float64(i) * 100.0) / float64(nPoints-1))}
		var Lab cmsCIELab
		CmsDoTransform(ar,xform, cmyk, Lab, 1)

		// Calculate the offset for the current index and assign the value
		SampledPoints[i] = float32(1.0 - Lab.L/100.0) // Negate K for easier operation
	}

	out := cmsBuildTabulatedToneCurveFloat(ar, ContextID, nPoints, SampledPoints)
	cmsDeleteTransform(ar,xform)
	return out
}

// Compute Black tone curve on a CMYK -> CMYK transform. This is done by
// using the proof direction on both profiles to find K->L* relationship
// then joining both curves. dwFlags may include black point compensation.
func cmsBuildKToneCurve(ar *arena.Arena, ContextID CmsContext,
	nPoints uint32,
	nProfiles uint32,
	Intents []uint32,
	hProfiles []CmsHPROFILE,
	BPC []bool,
	AdaptationStates []float64,
	dwFlags uint32) *CmsToneCurve {

	// Ensure CMYK -> CMYK
	if CmsGetColorSpace(hProfiles[0]) != cmsSigCmykData || CmsGetColorSpace(hProfiles[nProfiles-1]) != cmsSigCmykData {
		return nil
	}

	// Ensure the last profile is an output profile
	if cmsGetDeviceClass(hProfiles[nProfiles-1]) != cmsSigOutputClass {
		return nil
	}

	// Compute K->L* relationships for the input and output
	in := ComputeKToLstar(ar, ContextID, nPoints, nProfiles-1, Intents, hProfiles, BPC, AdaptationStates, dwFlags)
	if in == nil {
		return nil
	}

	out := ComputeKToLstar(ar, ContextID, nPoints, 1,
		Intents[nProfiles-1:nProfiles],
		hProfiles[nProfiles-1:nProfiles],
		BPC[nProfiles-1:nProfiles],
		AdaptationStates[nProfiles-1:nProfiles],
		dwFlags)
	if out == nil {
		CmsFreeToneCurve(in)
		return nil
	}

	// Join the input and output curves
	KTone := cmsJoinToneCurve(ar, ContextID, in, out, nPoints)
	CmsFreeToneCurve(in)
	CmsFreeToneCurve(out)

	if KTone == nil {
		return nil
	}

	// Ensure the resulting tone curve is monotonic
	if !cmsIsToneCurveMonotonic(KTone) {
		CmsFreeToneCurve(KTone)
		return nil
	}

	return KTone
}

type cmsTACestimator struct {
	nOutputChans uint32
	hRoundTrip   CmsHTRANSFORM
	MaxTAC       float32
	MaxInput     [cmsMAXCHANNELS]float32
}

// EstimateTAC is the callback function to calculate maximum TAC.
func EstimateTAC(ar *arena.Arena, in []uint16, out []uint16, cargo interface{}) int32 {
	bp, ok := cargo.(*cmsTACestimator)
	if !ok {
		cmsSignalError(nil, cmsERROR_UNDEFINED, "Interface data assertion error, not *cmsTACestimator\n")
		return 0
	}
	var roundTrip [cmsMAXCHANNELS]float32
	var sum float32

	// Evaluate the transform
	CmsDoTransform(ar,bp.hRoundTrip, in, roundTrip, 1)

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
func cmsDetectTAC(ar *arena.Arena, hProfile CmsHPROFILE) float64 {
	var bp cmsTACestimator
	var dwFormatter uint32
	var gridPoints [MAX_INPUT_DIMENSIONS]uint32

	contextID := cmsGetProfileContextID(hProfile)

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

	hLab := cmsCreateLab4ProfileTHR(ar,contextID, nil)
	if hLab == nil {
		return 0
	}

	// Setup a roundtrip on perceptual intent in output profile for TAC estimation
	bp.hRoundTrip = cmsCreateTransformTHR(ar,
		contextID,
		hLab,
		TYPE_Lab_16,
		hProfile,
		dwFormatter,
		INTENT_PERCEPTUAL,
		cmsFLAGS_NOOPTIMIZE|cmsFLAGS_NOCACHE,
	)
	CmsCloseProfile(ar, hLab)

	if bp.hRoundTrip == nil {
		return 0
	}

	// For L* we only need black and white. For C* we need many points.
	gridPoints[0] = 6
	gridPoints[1] = 74
	gridPoints[2] = 74

	if !cmsSliceSpace16(ar, 3, gridPoints[:], EstimateTAC, &bp) {
		bp.MaxTAC = 0
	}

	cmsDeleteTransform(ar,bp.hRoundTrip)

	// Results in %
	return float64(bp.MaxTAC)
}

// Gamut LUT Creation -----------------------------------------------------------------------------------------

// Define the GAMUTCHAIN structure
type GAMUTCHAIN struct {
	hInput    CmsHTRANSFORM // From whatever input color space. 16 bits to DBL
	hForward  CmsHTRANSFORM // Transforms going from Lab to colorant
	hReverse  CmsHTRANSFORM // Transforms going from colorant back to Lab
	Threshold float64       // The threshold after which is considered out of gamut
}

const ERR_THRESHOLD = 5

// This sampler does compute gamut boundaries by comparing original
// values with a transform going back and forth. Values above ERR_THRESHOLD
// of maximum are considered out of gamut.

// GamutSampler computes gamut boundaries by comparing original values with a transform
// going back and forth. Values above ERR_THRESHOLD are considered out of gamut.
func GamutSampler(ar *arena.Arena, In []uint16, Out []uint16, cargo interface{}) int32 {
	t, ok := cargo.(*GAMUTCHAIN)
	if !ok {
		cmsSignalError(nil, cmsERROR_UNDEFINED, "Interface data assertion error, not *GAMUTCHAIN\n")
		return 0
	}
	var LabIn1, LabOut1 cmsCIELab
	var LabIn2, LabOut2 cmsCIELab
	var Proof [cmsMAXCHANNELS]uint16
	var Proof2 [cmsMAXCHANNELS]uint16
	var dE1, dE2, ErrorRatio float64

	// Assume in-gamut by default.
	ErrorRatio = 1.0

	// Convert input to Lab
	//CmsDoTransform(t.hInput, In,LabIn1, 1)
	LabIn1Slice := LabToSlice(LabIn1)
	CmsDoTransform(ar,t.hInput, In, LabIn1Slice, 1)

	// Convert from PCS to colorant. This always returns in-gamut values.
	CmsDoTransform(ar,t.hForward, LabIn1Slice, Proof[:], 1)

	// Convert from colorant to PCS.
	LabOut1Slice := LabToSlice(LabOut1)
	CmsDoTransform(ar,t.hReverse, Proof[:], LabOut1Slice, 1)

	// Copy LabOut1 to LabIn2

	//memmove(unsafe.Pointer(&LabIn2), unsafe.Pointer(&LabOut1), unsafe.Sizeof(cmsCIELab{}))
	LabIn2Slice := LabToSlice(LabIn2)
	copy(LabIn2Slice, LabOut1Slice)
	LabIn1 = SliceToLab(LabIn1Slice)
	LabIn2 = SliceToLab(LabIn2Slice)
	// Forward and reverse transform again, using LabOut1 as input
	CmsDoTransform(ar,t.hForward, LabOut1Slice, Proof2[:], 1)
	LabOut2Slice := LabToSlice(LabOut2)
	CmsDoTransform(ar,t.hReverse, Proof2[:], LabOut2Slice, 1)
	LabOut2 = SliceToLab(LabOut2Slice)
	LabOut1 = SliceToLab(LabOut1Slice)

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
	ar *arena.Arena,
	ContextID CmsContext,
	hProfiles []CmsHPROFILE,
	BPC []bool,
	Intents []uint32,
	AdaptationStates []float64,
	nGamutPCSposition uint32,
	hGamut CmsHPROFILE,
) *cmsPipeline {
	var hLab CmsHPROFILE
	var Gamut *cmsPipeline
	var CLUT *cmsStage
	var dwFormat uint32
	var Chain GAMUTCHAIN
	var nGridpoints uint32
	var nChannels int32
	var ColorSpace cmsColorSpaceSignature
	var i uint32
	var ProfileList [256]CmsHPROFILE
	var BPCList [256]bool
	var AdaptationList [256]float64
	var IntentList [256]uint32

	// Validate PCS position
	if nGamutPCSposition <= 0 || nGamutPCSposition > 255 {
		cmsSignalError(ContextID, cmsERROR_RANGE, "Wrong position of PCS. 1..255 expected")
		return nil
	}

	hLab = cmsCreateLab4ProfileTHR(ar,ContextID, nil)
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

	ColorSpace = CmsGetColorSpace(hGamut)
	nChannels = cmsChannelsOfColorSpace(ColorSpace)
	nGridpoints = cmsReasonableGridpointsByColorspace(ColorSpace, cmsFLAGS_HIGHRESPRECALC)
	dwFormat = CHANNELS_SH(uint32(nChannels)) | BYTES_SH(2)

	// Create the input transform
	Chain.hInput = CmsHTRANSFORM(cmsCreateExtendedTransform(ar,
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
	Chain.hForward = cmsCreateTransformTHR(ar,
		ContextID,
		hLab, TYPE_Lab_DBL,
		hGamut, dwFormat,
		INTENT_RELATIVE_COLORIMETRIC,
		cmsFLAGS_NOCACHE,
	)

	// Create the backwards step
	Chain.hReverse = cmsCreateTransformTHR(ar,
		ContextID,
		hGamut, dwFormat,
		hLab, TYPE_Lab_DBL,
		INTENT_RELATIVE_COLORIMETRIC,
		cmsFLAGS_NOCACHE,
	)

	// Verify all steps are created successfully
	if Chain.hInput != nil && Chain.hForward != nil && Chain.hReverse != nil {
		// Compute gamut LUT
		Gamut = cmsPipelineAlloc(ar, ContextID, 3, 1)
		if Gamut != nil {
			CLUT = cmsStageAllocCLut16bit(ar,ContextID, nGridpoints, uint32(nChannels), 1, nil)
			if !cmsPipelineInsertStage(Gamut, cmsAT_BEGIN, CLUT) {
				cmsPipelineFree(ar,Gamut)
				Gamut = nil
			} else {
				cmsStageSampleCLut16bit(ar,CLUT, GamutSampler, &Chain, 0)
			}
		}
	} else {
		Gamut = nil // Failed to create transform
	}

	// Free resources
	if Chain.hInput != nil {
		cmsDeleteTransform(ar,Chain.hInput)
	}
	if Chain.hForward != nil {
		cmsDeleteTransform(ar,Chain.hForward)
	}
	if Chain.hReverse != nil {
		cmsDeleteTransform(ar,Chain.hReverse)
	}
	if hLab != nil {
		CmsCloseProfile(ar, hLab)
	}

	// Return the computed LUT
	return Gamut
}

// cmsDetectRGBProfileGamma detects whether a given ICC profile works in linear (gamma 1.0) space.
// It uses least squares fitting to estimate gamma for a synthetic gray (R=G=B).
// If gamma is close to 1.0, RGB is linear. On unsupported profiles, -1 is returned.
func cmsDetectRGBProfileGamma(ar *arena.Arena, hProfile CmsHPROFILE, threshold float64) float64 {
	var (
		ContextID   CmsContext
		hXYZ        CmsHPROFILE
		xform       CmsHTRANSFORM
		YCurve      *CmsToneCurve
		rgb         [256][3]uint16
		XYZ         [256]cmsCIEXYZ
		YNormalized [256]float32
		gamma       float64
		cls         cmsProfileClassSignature
	)

	// Ensure the profile is in RGB color space
	if CmsGetColorSpace(hProfile) != cmsSigRgbData {
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
	hXYZ = cmsCreateXYZProfileTHR(ar,ContextID)
	if hXYZ == nil {
		return -1
	}

	// Create a transform from RGB to XYZ
	xform = cmsCreateTransformTHR(ar,ContextID, hProfile, TYPE_RGB_16, hXYZ, TYPE_XYZ_DBL,
		INTENT_RELATIVE_COLORIMETRIC, cmsFLAGS_NOOPTIMIZE)

	if xform == nil {
		CmsCloseProfile(ar, hXYZ)
		return -1
	}

	// Generate a synthetic gray (R=G=B) ramp
	for i := uint8(0); i <= uint8(255); i++ {
		rgb[i][0] = FROM_8_TO_16(i)
		rgb[i][1] = FROM_8_TO_16(i)
		rgb[i][2] = FROM_8_TO_16(i)
	}

	// Perform the transform
	CmsDoTransform(ar,xform, rgb[:], XYZ[:], 256)

	// Clean up the transform and XYZ profile
	cmsDeleteTransform(ar,xform)
	CmsCloseProfile(ar, hXYZ)

	// Normalize the Y component
	for i := 0; i < 256; i++ {
		YNormalized[i] = float32(XYZ[i].Y)
	}

	// Build a tone curve from the normalized Y values
	YCurve = cmsBuildTabulatedToneCurveFloat(ar, ContextID, 256, YNormalized[:])
	if YCurve == nil {
		return -1
	}

	// Estimate gamma
	gamma = cmsEstimateGamma(YCurve, threshold)

	// Free the tone curve and return the gamma value
	CmsFreeToneCurve(YCurve)
	return gamma
}
