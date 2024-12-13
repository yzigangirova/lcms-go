package golcms

import (
	"math"
	"unsafe"
)

// cmsIntentFn represents a function type for intents.

// cmsIntentsList represents the structure holding implementations for all supported intents.
type cmsIntentsList struct {
	Intent      uint32
	Description string
	Link        cmsIntentFn
	Next        *cmsIntentsList
}

var DefaultIntents []cmsIntentsList

func initDefaultIntents() {
	// Initialize the intents without setting the `Next` pointers initially
	DefaultIntents = []cmsIntentsList{
		{Intent: INTENT_PERCEPTUAL, Description: "Perceptual", Link: DefaultICCintents},
		{Intent: INTENT_RELATIVE_COLORIMETRIC, Description: "Relative colorimetric", Link: DefaultICCintents},
		{Intent: INTENT_SATURATION, Description: "Saturation", Link: DefaultICCintents},
		{Intent: INTENT_ABSOLUTE_COLORIMETRIC, Description: "Absolute colorimetric", Link: DefaultICCintents},
		{Intent: INTENT_PRESERVE_K_ONLY_PERCEPTUAL, Description: "Perceptual preserving black ink", Link: BlackPreservingKOnlyIntents},
		{Intent: INTENT_PRESERVE_K_ONLY_RELATIVE_COLORIMETRIC, Description: "Relative colorimetric preserving black ink", Link: BlackPreservingKOnlyIntents},
		{Intent: INTENT_PRESERVE_K_ONLY_SATURATION, Description: "Saturation preserving black ink", Link: BlackPreservingKOnlyIntents},
		{Intent: INTENT_PRESERVE_K_PLANE_PERCEPTUAL, Description: "Perceptual preserving black plane", Link: BlackPreservingKPlaneIntents},
		{Intent: INTENT_PRESERVE_K_PLANE_RELATIVE_COLORIMETRIC, Description: "Relative colorimetric preserving black plane", Link: BlackPreservingKPlaneIntents},
		{Intent: INTENT_PRESERVE_K_PLANE_SATURATION, Description: "Saturation preserving black plane", Link: BlackPreservingKPlaneIntents},
	}

	// Link the list
	for i := 0; i < len(DefaultIntents)-1; i++ {
		DefaultIntents[i].Next = &DefaultIntents[i+1]
	}
}

func init() {
	initDefaultIntents()
}

// Default handler for ICC-style intents

func DefaultICCintents(
	ContextID cmsContext,
	nProfiles uint32,
	TheIntents []uint32,
	hProfiles []cmsHPROFILE,
	BPC []bool,
	AdaptationStates []float64,
	dwFlags uint32,
) *cmsPipeline {
	var (
		Lut               *cmsPipeline
		Result            *cmsPipeline
		hProfile          cmsHPROFILE
		m                 cmsMAT3
		off               cmsVEC3
		ColorSpaceIn      cmsColorSpaceSignature
		ColorSpaceOut     cmsColorSpaceSignature = cmsSigLabData
		CurrentColorSpace cmsColorSpaceSignature
		ClassSig          cmsProfileClassSignature
		Intent            uint32
	)

	// For safety
	if nProfiles == 0 {
		return nil
	}

	// Allocate an empty LUT for holding the result. 0 as channel count means 'undefined'
	Result = cmsPipelineAlloc(ContextID, 0, 0)
	if Result == nil {
		return nil
	}

	CurrentColorSpace = cmsGetColorSpace(hProfiles[0])

	for i := uint32(0); i < nProfiles; i++ {
		var lIsDeviceLink, lIsInput bool

		hProfile = hProfiles[i]
		ClassSig = cmsGetDeviceClass(hProfile)
		lIsDeviceLink = (ClassSig == cmsSigLinkClass || ClassSig == cmsSigAbstractClass)

		// Determine if the profile is input
		if (i == 0) && !lIsDeviceLink {
			lIsInput = true
		} else {
			lIsInput = (CurrentColorSpace != cmsSigXYZData) &&
				(CurrentColorSpace != cmsSigLabData)
		}

		Intent = TheIntents[i]

		if lIsInput || lIsDeviceLink {
			ColorSpaceIn = cmsGetColorSpace(hProfile)
			ColorSpaceOut = cmsGetPCS(hProfile)
		} else {
			ColorSpaceIn = cmsGetPCS(hProfile)
			ColorSpaceOut = cmsGetColorSpace(hProfile)
		}

		if !ColorSpaceIsCompatible(ColorSpaceIn, CurrentColorSpace) {
			cmsSignalError(ContextID, cmsERROR_COLORSPACE_CHECK, "ColorSpace mismatch")
			goto Error
		}

		// If devicelink or named color class
		if lIsDeviceLink || (ClassSig == cmsSigNamedColorClass && nProfiles == 1) {
			Lut = cmsReadDevicelinkLUT(hProfile, Intent)
			if Lut == nil {
				goto Error
			}

			if ClassSig == cmsSigAbstractClass && i > 0 {
				if !ComputeConversion(i, hProfiles, Intent, BPC[i], AdaptationStates[i], &m, &off) {
					goto Error
				}
			} else {
				cmsMAT3identity(&m)
				cmsVEC3init(&off, 0, 0, 0)
			}

			if !AddConversion(Result, CurrentColorSpace, ColorSpaceIn, &m, &off) {
				goto Error
			}
		} else {
			if lIsInput {
				Lut = cmsReadInputLUT(hProfile, Intent)
				if Lut == nil {
					goto Error
				}
			} else {
				Lut = cmsReadOutputLUT(hProfile, Intent)
				if Lut == nil {
					goto Error
				}

				if !ComputeConversion(i, hProfiles, Intent, BPC[i], AdaptationStates[i], &m, &off) {
					goto Error
				}
				if !AddConversion(Result, CurrentColorSpace, ColorSpaceIn, &m, &off) {
					goto Error
				}
			}
		}

		// Concatenate LUT
		if !cmsPipelineCat(Result, Lut) {
			goto Error
		}

		cmsPipelineFree(Lut)
		Lut = nil
		CurrentColorSpace = ColorSpaceOut
	}

	// Handle non-negatives clip
	if dwFlags&cmsFLAGS_NONEGATIVES != 0 {
		if ColorSpaceOut == cmsSigGrayData || ColorSpaceOut == cmsSigRgbData || ColorSpaceOut == cmsSigCmykData {
			clip := cmsStageClipNegatives(Result.ContextID, cmsChannelsOfColorSpace(ColorSpaceOut))
			if clip == nil {
				goto Error
			}

			if !cmsPipelineInsertStage(Result, cmsAT_END, clip) {
				goto Error
			}
		}
	}

	return Result

Error:
	if Lut != nil {
		cmsPipelineFree(Lut)
	}
	if Result != nil {
		cmsPipelineFree(Result)
	}
	return nil
}

func cmsDefaultICCintents(
	ContextID cmsContext,
	nProfiles uint32,
	TheIntents []uint32,
	hProfiles []cmsHPROFILE,
	BPC []bool,
	AdaptationStates []float64,
	dwFlags uint32,
) *cmsPipeline {
	return DefaultICCintents(ContextID, nProfiles, TheIntents, hProfiles, BPC, AdaptationStates, dwFlags)
}

func TranslateNonICCIntents(Intent uint32) uint32 {
	switch Intent {
	case INTENT_PRESERVE_K_ONLY_PERCEPTUAL, INTENT_PRESERVE_K_PLANE_PERCEPTUAL:
		return INTENT_PERCEPTUAL
	case INTENT_PRESERVE_K_ONLY_RELATIVE_COLORIMETRIC, INTENT_PRESERVE_K_PLANE_RELATIVE_COLORIMETRIC:
		return INTENT_RELATIVE_COLORIMETRIC
	case INTENT_PRESERVE_K_ONLY_SATURATION, INTENT_PRESERVE_K_PLANE_SATURATION:
		return INTENT_SATURATION
	default:
		return Intent
	}
}

type GrayOnlyParams struct {
	Cmyk2Cmyk *cmsPipeline  // The original transform
	KTone     *cmsToneCurve // Black-to-black tone curve
}

// BlackPreservingGrayOnlySampler preserves black-only CMYK transformations.
func BlackPreservingGrayOnlySampler(In []uint16, Out []uint16, Cargo unsafe.Pointer) bool {
	bp := (*GrayOnlyParams)(Cargo)

	// If going across black only, keep black only
	if In[0] == 0 && In[1] == 0 && In[2] == 0 {
		// TAC does not apply because it is black ink!
		Out[0], Out[1], Out[2] = 0, 0, 0
		Out[3] = cmsEvalToneCurve16(bp.KTone, In[3])
		return true
	}

	// Keep normal transform for other colors
	bp.Cmyk2Cmyk.Eval16Fn(In, Out, bp.Cmyk2Cmyk.Data)
	return true
}

// BlackPreservingKOnlyIntents handles black-preserving K-only intents.
func BlackPreservingKOnlyIntents(
	ContextID cmsContext,
	nProfiles uint32,
	TheIntents []uint32,
	hProfiles []cmsHPROFILE,
	BPC []bool,
	AdaptationStates []float64,
	dwFlags uint32,
) *cmsPipeline {
	var bp GrayOnlyParams
	var Result *cmsPipeline
	var CLUT *cmsStage
	var ICCIntents [256]uint32
	var lastProfilePos, preservationProfilesCount uint32
	var hLastProfile cmsHPROFILE

	// Sanity check
	if nProfiles < 1 || nProfiles > 255 {
		return nil
	}

	// Translate black-preserving intents to ICC ones
	for i := uint32(0); i < nProfiles; i++ {
		ICCIntents[i] = TranslateNonICCIntents(TheIntents[i])
	}

	// Trim all CMYK devicelinks at the end
	lastProfilePos = nProfiles - 1
	hLastProfile = hProfiles[lastProfilePos]

	for lastProfilePos > 1 {
		hLastProfile = hProfiles[lastProfilePos-1]
		lastProfilePos--

		if cmsGetColorSpace(hLastProfile) != cmsSigCmykData ||
			cmsGetDeviceClass(hLastProfile) != cmsSigLinkClass {
			break
		}
	}

	preservationProfilesCount = lastProfilePos + 1

	// Check for non-CMYK profiles
	if cmsGetColorSpace(hProfiles[0]) != cmsSigCmykData ||
		!(cmsGetColorSpace(hLastProfile) == cmsSigCmykData ||
			cmsGetDeviceClass(hLastProfile) == cmsSigOutputClass) {
		return DefaultICCintents(ContextID, nProfiles, ICCIntents[:], hProfiles, BPC, AdaptationStates, dwFlags)
	}

	// Allocate an empty LUT for holding the result
	Result = cmsPipelineAlloc(ContextID, 4, 4)
	if Result == nil {
		return nil
	}

	// Create a LUT holding normal ICC transform
	bp.Cmyk2Cmyk = DefaultICCintents(ContextID, preservationProfilesCount, ICCIntents[:], hProfiles, BPC, AdaptationStates, dwFlags)
	if bp.Cmyk2Cmyk == nil {
		goto Error
	}

	// Compute the tone curve
	bp.KTone = cmsBuildKToneCurve(ContextID, 4096, preservationProfilesCount, ICCIntents[:], hProfiles, BPC, AdaptationStates, dwFlags)
	if bp.KTone == nil {
		goto Error
	}

	// Determine the number of gridpoints
	nGridPoints := cmsReasonableGridpointsByColorspace(cmsSigCmykData, dwFlags)

	// Create the CLUT
	CLUT = cmsStageAllocCLut16bit(ContextID, nGridPoints, 4, 4, nil)
	if CLUT == nil {
		goto Error
	}

	// Insert CLUT into the pipeline
	if !cmsPipelineInsertStage(Result, cmsAT_BEGIN, CLUT) {
		goto Error
	}

	// Sample the CLUT
	if !cmsStageSampleCLut16bit(CLUT, BlackPreservingGrayOnlySampler, unsafe.Pointer(&bp), 0) {
		goto Error
	}

	// Insert possible devicelinks at the end
	for i := lastProfilePos + 1; i < nProfiles; i++ {
		devlink := cmsReadDevicelinkLUT(hProfiles[i], ICCIntents[i])
		if devlink == nil {
			goto Error
		}

		if !cmsPipelineCat(Result, devlink) {
			goto Error
		}
	}

	// Free resources
	cmsPipelineFree(bp.Cmyk2Cmyk)
	cmsFreeToneCurve(bp.KTone)
	return Result

Error:
	if bp.Cmyk2Cmyk != nil {
		cmsPipelineFree(bp.Cmyk2Cmyk)
	}
	if bp.KTone != nil {
		cmsFreeToneCurve(bp.KTone)
	}
	if Result != nil {
		cmsPipelineFree(Result)
	}
	return nil
}

// K Plane-preserving CMYK to CMYK ------------------------------------------------------------------------------------
type PreserveKPlaneParams struct {
	Cmyk2Cmyk    *cmsPipeline  // The original transform
	HProofOutput cmsHTRANSFORM // Output CMYK to Lab (last profile)
	Cmyk2Lab     cmsHTRANSFORM // The input chain
	KTone        *cmsToneCurve // Black-to-black tone curve
	LabK2Cmyk    *cmsPipeline  // The output profile
	MaxError     float64       // Maximum error
	HRoundTrip   cmsHTRANSFORM // Round-trip transform
	MaxTAC       float64       // Maximum total area coverage
}

// BlackPreservingSampler performs sampling for K-plane preservation.
func BlackPreservingSampler(In, Out []uint16, Cargo unsafe.Pointer) bool {
	bp := (*PreserveKPlaneParams)(Cargo)
	var Inf, Outf, LabK [4]float32
	var ColorimetricLab, BlackPreservingLab cmsCIELab
	var SumCMY, SumCMYK, Error, Ratio float64

	// Convert from 16 bits to floating point
	for i := 0; i < 4; i++ {
		Inf[i] = float32(In[i]) / 65535.0
	}

	// Get the K across Tone curve
	LabK[3] = cmsEvalToneCurveFloat(bp.KTone, Inf[3])

	// If going across black only, keep black only
	if In[0] == 0 && In[1] == 0 && In[2] == 0 {
		Out[0], Out[1], Out[2] = 0, 0, 0
		Out[3] = cmsQuickSaturateWord(LabK[3] * 65535.0)
		return true
	}

	// Try the original transform
	cmsPipelineEvalFloat(bp.Cmyk2Cmyk, Inf[:], Outf[:])

	// Store a copy of the floating-point result into 16-bit
	for i := 0; i < 4; i++ {
		Out[i] = cmsQuickSaturateWord(Outf[i] * 65535.0)
	}

	// Check if K is already OK
	if math.Abs(float64(Outf[3]-LabK[3])) < (3.0 / 65535.0) {
		return true
	}

	// Measure and keep Lab measurement for further usage
	cmsDoTransform(bp.HProofOutput, Out, &ColorimetricLab, 1)

	// Transform to Lab
	cmsDoTransform(bp.Cmyk2Lab, Outf[:], LabK[:], 1)

	// Reverse interpolation to obtain CMY with fixed K
	if !cmsPipelineEvalReverseFloat(bp.LabK2Cmyk, LabK[:], Outf[:], Outf[:]) {
		// Use colorimetric transform if reverse interpolation fails
		return true
	}

	// Fix K
	Outf[3] = LabK[3]

	// Apply TAC if needed
	SumCMY = float64(Outf[0]) + float64(Outf[1]) + float64(Outf[2])
	SumCMYK = SumCMY + float64(Outf[3])

	if SumCMYK > bp.MaxTAC {
		Ratio = 1 - (SumCMYK-bp.MaxTAC)/SumCMY
		if Ratio < 0 {
			Ratio = 0
		}
	} else {
		Ratio = 1.0
	}

	Out[0] = cmsQuickSaturateWord(Outf[0] * float32(Ratio) * 65535.0)
	Out[1] = cmsQuickSaturateWord(Outf[1] * float32(Ratio) * 65535.0)
	Out[2] = cmsQuickSaturateWord(Outf[2] * float32(Ratio) * 65535.0)
	Out[3] = cmsQuickSaturateWord(Outf[3] * 65535.0)

	// Estimate the error
	cmsDoTransform(bp.HProofOutput, Out, &BlackPreservingLab, 1)
	Error = cmsDeltaE(&ColorimetricLab, &BlackPreservingLab)
	if Error > bp.MaxError {
		bp.MaxError = Error
	}

	return true
}

// BlackPreservingKPlaneIntents handles black-plane preserving intents.
func BlackPreservingKPlaneIntents(
	ContextID cmsContext,
	nProfiles uint32,
	TheIntents []uint32,
	hProfiles []cmsHPROFILE,
	BPC []bool,
	AdaptationStates []float64,
	dwFlags uint32,
) *cmsPipeline {
	var bp PreserveKPlaneParams
	var Result *cmsPipeline
	var CLUT *cmsStage
	var ICCIntents [256]uint32
	var lastProfilePos, preservationProfilesCount uint32
	var hLastProfile, hLab cmsHPROFILE

	// Sanity check
	if nProfiles < 1 || nProfiles > 255 {
		return nil
	}

	// Translate intents
	for i := uint32(0); i < nProfiles; i++ {
		ICCIntents[i] = TranslateNonICCIntents(TheIntents[i])
	}

	// Trim CMYK devicelinks at the end
	lastProfilePos = nProfiles - 1
	hLastProfile = hProfiles[lastProfilePos]

	for lastProfilePos > 1 {
		hLastProfile = hProfiles[lastProfilePos-1]
		lastProfilePos--

		if cmsGetColorSpace(hLastProfile) != cmsSigCmykData || cmsGetDeviceClass(hLastProfile) != cmsSigLinkClass {
			break
		}
	}

	preservationProfilesCount = lastProfilePos + 1

	// Check for non-CMYK profiles
	if cmsGetColorSpace(hProfiles[0]) != cmsSigCmykData ||
		!(cmsGetColorSpace(hLastProfile) == cmsSigCmykData || cmsGetDeviceClass(hLastProfile) == cmsSigOutputClass) {
		return DefaultICCintents(ContextID, nProfiles, ICCIntents[:], hProfiles, BPC, AdaptationStates, dwFlags)
	}

	// Allocate LUT
	Result = cmsPipelineAlloc(ContextID, 4, 4)
	if Result == nil {
		return nil
	}

	// Read input LUT
	bp.LabK2Cmyk = cmsReadInputLUT(hLastProfile, INTENT_RELATIVE_COLORIMETRIC)
	if bp.LabK2Cmyk == nil {
		goto Cleanup
	}

	// Get TAC
	bp.MaxTAC = cmsDetectTAC(hLastProfile) / 100.0
	if bp.MaxTAC <= 0 {
		goto Cleanup
	}

	// Create ICC transform
	bp.Cmyk2Cmyk = DefaultICCintents(ContextID, preservationProfilesCount, ICCIntents[:], hProfiles, BPC, AdaptationStates, dwFlags)
	if bp.Cmyk2Cmyk == nil {
		goto Cleanup
	}

	// Compute tone curve
	bp.KTone = cmsBuildKToneCurve(ContextID, 4096, preservationProfilesCount, ICCIntents[:], hProfiles, BPC, AdaptationStates, dwFlags)
	if bp.KTone == nil {
		goto Cleanup
	}

	// Prepare proof output
	hLab = cmsCreateLab4ProfileTHR(ContextID, nil)
	bp.HProofOutput = cmsCreateTransformTHR(ContextID, hLastProfile, CHANNELS_SH(4)|BYTES_SH(2), hLab, TYPE_Lab_DBL, INTENT_RELATIVE_COLORIMETRIC, cmsFLAGS_NOCACHE|cmsFLAGS_NOOPTIMIZE)
	if bp.HProofOutput == nil {
		goto Cleanup
	}

	// Prepare CMYK to Lab
	bp.Cmyk2Lab = cmsCreateTransformTHR(ContextID, hLastProfile, FLOAT_SH(1)|CHANNELS_SH(4)|BYTES_SH(4), hLab, FLOAT_SH(1)|CHANNELS_SH(3)|BYTES_SH(4), INTENT_RELATIVE_COLORIMETRIC, cmsFLAGS_NOCACHE|cmsFLAGS_NOOPTIMIZE)
	if bp.Cmyk2Lab == nil {
		goto Cleanup
	}
	cmsCloseProfile(hLab)

	// Create CLUT
	nGridPoints := cmsReasonableGridpointsByColorspace(cmsSigCmykData, dwFlags)
	CLUT = cmsStageAllocCLut16bit(ContextID, nGridPoints, 4, 4, nil)
	if CLUT == nil {
		goto Cleanup
	}

	// Insert and sample CLUT
	if !cmsPipelineInsertStage(Result, cmsAT_BEGIN, CLUT) || !cmsStageSampleCLut16bit(CLUT, BlackPreservingSampler, unsafe.Pointer(&bp), 0) {
		goto Cleanup
	}

	// Insert devicelinks
	for i := lastProfilePos + 1; i < nProfiles; i++ {
		devlink := cmsReadDevicelinkLUT(hProfiles[i], ICCIntents[i])
		if devlink == nil || !cmsPipelineCat(Result, devlink) {
			goto Cleanup
		}
	}

Cleanup:
	if bp.Cmyk2Cmyk != nil {
		cmsPipelineFree(bp.Cmyk2Cmyk)
	}
	if bp.Cmyk2Lab != nil {
		cmsDeleteTransform(bp.Cmyk2Lab)
	}
	if bp.HProofOutput != nil {
		cmsDeleteTransform(bp.HProofOutput)
	}
	if bp.KTone != nil {
		cmsFreeToneCurve(bp.KTone)
	}
	if bp.LabK2Cmyk != nil {
		cmsPipelineFree(bp.LabK2Cmyk)
	}

	return Result
}

// _cmsRegisterRenderingIntentPlugin registers a rendering intent plugin.
func cmsRegisterRenderingIntentPlugin(id cmsContext, Data *cmsPluginBase) bool {
	ctx := (*cmsIntentsPluginChunkType)(cmsContextGetClientChunk(id, IntentPlugin))
	Plugin := (*cmsPluginRenderingIntent)(unsafe.Pointer(Data))

	// Reset custom intents if Data is nil.
	if Data == nil {
		ctx.Intents = nil
		return true
	}

	// Allocate memory for the new intent node.
	fl := (*cmsIntentsList)(cmsPluginMalloc(id, uint32(unsafe.Sizeof(cmsIntentsList{}))))
	if fl == nil {
		return false
	}

	// Populate the new node's fields.
	fl.Intent = Plugin.Intent
	copy(fl.Description[:], Plugin.Description[:len(Plugin.Description)])
	fl.Description[len(fl.Description)-1] = 0 // Ensure null termination.
	fl.Link = Plugin.Link

	// Update the linked list.
	fl.Next = ctx.Intents
	ctx.Intents = fl

	return true
}
