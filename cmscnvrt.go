package golcms

import (

	//"fmt"
	"math"
	"unsafe"

	"github.com/yzigangirova/lcms-go/mem"
)

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

// A pointer to the beginning of the list
var cmsIntentsPluginChunk = cmsIntentsPluginChunkType{Intents: nil}

func init() {
	initDefaultIntents()
}

// SearchIntent translates the given function
func SearchIntent(ContextID CmsContext, Intent uint32) *cmsIntentsList {
	// Retrieve the plugin chunk for intents
	ctx, ok := CmsContextGetClientChunk(ContextID, IntentPlugin).(*cmsIntentsPluginChunkType)
	if !ok {
		cmsSignalError(ContextID, cmsERROR_UNDEFINED, "Error: Interface data assertion error, not cmsIntentsPluginChunkType\n")
		return nil
	}
	// Search in the plugin intents list
	for pt := ctx.Intents; pt != nil; pt = pt.Next {
		if pt.Intent == Intent {
			return pt
		}
	}

	// Search in the default intents list
	for pt := &DefaultIntents[0]; pt != nil; pt = pt.Next {
		if pt.Intent == Intent {
			return pt
		}
	}

	return nil
}

// ComputeBlackPointCompensation calculates the black point compensation matrix and offset.
// Black points should come relative to the white point. Fills a matrix `m` and an offset `off`,
// organized as a 4x4 matrix.
//
// Implementation details:
// The function computes a linear scaling in XYZ such that:
// - [m]*bpin + off = bpout
// - [m]*D50 + off = D50
//
// This linear scaling takes the form: ax+b, where:
// - a = (bpout - D50) / (bpin - D50)
// - b = -D50 * (bpout - bpin) / (bpin - D50)
func ComputeBlackPointCompensation(BlackPointIn *cmsCIEXYZ, BlackPointOut *cmsCIEXYZ, m *cmsMAT3, off *cmsVEC3) {
	//	fmt.Println("start ComputeBlackPointCompensation")
	var ax, ay, az, bx, by, bz, tx, ty, tz float64

	// Compute differences between black points and D50
	tx = BlackPointIn.X - cmsD50_XYZ().X
	ty = BlackPointIn.Y - cmsD50_XYZ().Y
	tz = BlackPointIn.Z - cmsD50_XYZ().Z

	// Calculate scaling factors (a) for each channel
	ax = (BlackPointOut.X - cmsD50_XYZ().X) / tx
	ay = (BlackPointOut.Y - cmsD50_XYZ().Y) / ty
	az = (BlackPointOut.Z - cmsD50_XYZ().Z) / tz

	// Calculate offset factors (b) for each channel
	bx = -cmsD50_XYZ().X * (BlackPointOut.X - BlackPointIn.X) / tx
	by = -cmsD50_XYZ().Y * (BlackPointOut.Y - BlackPointIn.Y) / ty
	bz = -cmsD50_XYZ().Z * (BlackPointOut.Z - BlackPointIn.Z) / tz

	// Initialize the transformation matrix with scaling factors
	cmsVEC3init(&m.V[0], ax, 0, 0)
	cmsVEC3init(&m.V[1], 0, ay, 0)
	cmsVEC3init(&m.V[2], 0, 0, az)

	// Initialize the offset vector
	cmsVEC3init(off, bx, by, bz)

	// fmt.Println("end ComputeBlackPointCompensation")
}

// Approximate a blackbody illuminant based on CHAD information
// Approximate a blackbody illuminant based on CHAD information
func CHAD2Temp(Chad *cmsMAT3) float64 {
	var d, s cmsVEC3
	var Dest cmsCIEXYZ
	var DestChromaticity CmsCIExyY
	var TempK float64
	var m1, m2 cmsMAT3

	// Copy CHAD into m1
	m1 = *Chad

	// Invert the CHAD matrix
	if !cmsMAT3inverse(&m1, &m2) {
		return -1.0
	}

	// Convert D50 across inverse CHAD to get the absolute white point
	s.N[VX] = cmsD50_XYZ().X
	s.N[VY] = cmsD50_XYZ().Y
	s.N[VZ] = cmsD50_XYZ().Z

	cmsMAT3eval(&d, &m2, &s)

	// Populate the destination XYZ values
	Dest.X = d.N[VX]
	Dest.Y = d.N[VY]
	Dest.Z = d.N[VZ]

	// Convert XYZ to chromaticity
	cmsXYZ2xyY(&DestChromaticity, &Dest)

	// Compute the temperature from the white point
	if !cmsTempFromWhitePoint(&TempK, &DestChromaticity) {
		return -1.0 // Return an invalid temperature if the conversion fails
	}

	return TempK
}

// Compute a CHAD based on a given temperature
func Temp2CHAD(Chad *cmsMAT3, Temp float64) {
	var White cmsCIEXYZ
	var ChromaticityOfWhite CmsCIExyY

	// Compute chromaticity from the given temperature
	cmsWhitePointFromTemp(&ChromaticityOfWhite, Temp)

	// Convert chromaticity to XYZ
	cmsxyY2XYZ(&White, &ChromaticityOfWhite)

	// Compute the chromatic adaptation matrix (CHAD) for the given white point
	cmsAdaptationMatrix(Chad, nil, &White, cmsD50_XYZ())
}

// Join scalings to obtain relative input to absolute and then to relative output.
// Result is stored in a 3x3 matrix
func ComputeAbsoluteIntent(
	AdaptationState float64,
	WhitePointIn *cmsCIEXYZ,
	ChromaticAdaptationMatrixIn *cmsMAT3,
	WhitePointOut *cmsCIEXYZ,
	ChromaticAdaptationMatrixOut *cmsMAT3,
	m *cmsMAT3,
) bool {
	var Scale, m1, m2, m3, m4 cmsMAT3

	// TODO: Follow Marc Mahy's recommendation to check if CHAD is same by using M1*M2 == M2*M1. If so, do nothing.
	// TODO: Add support for ArgyllArts tag

	// Adaptation state
	if AdaptationState == 1.0 {
		// Observer is fully adapted. Keep chromatic adaptation.
		// That is the standard V4 behaviour
		cmsVEC3init(&m.V[0], WhitePointIn.X/WhitePointOut.X, 0, 0)
		cmsVEC3init(&m.V[1], 0, WhitePointIn.Y/WhitePointOut.Y, 0)
		cmsVEC3init(&m.V[2], 0, 0, WhitePointIn.Z/WhitePointOut.Z)
	} else {
		// Incomplete adaptation. This is an advanced feature.
		cmsVEC3init(&Scale.V[0], WhitePointIn.X/WhitePointOut.X, 0, 0)
		cmsVEC3init(&Scale.V[1], 0, WhitePointIn.Y/WhitePointOut.Y, 0)
		cmsVEC3init(&Scale.V[2], 0, 0, WhitePointIn.Z/WhitePointOut.Z)

		if AdaptationState == 0.0 {
			m1 = *ChromaticAdaptationMatrixOut
			m2 = cmsMAT3per(&m1, &Scale)
			// m2 holds CHAD from output white to D50 times abs. col. scaling

			// Observer is not adapted, undo the chromatic adaptation
			*m = cmsMAT3per(&m2, ChromaticAdaptationMatrixOut)

			m3 = *ChromaticAdaptationMatrixIn
			if !cmsMAT3inverse(&m3, &m4) {
				return false
			}
			*m = cmsMAT3per(&m2, &m4)
		} else {
			var MixedCHAD cmsMAT3
			var TempSrc, TempDest, Temp float64

			m1 = *ChromaticAdaptationMatrixIn
			if !cmsMAT3inverse(&m1, &m2) {
				return false
			}
			m3 = cmsMAT3per(&m2, &Scale)
			// m3 holds CHAD from input white to D50 times abs. col. scaling

			TempSrc = CHAD2Temp(ChromaticAdaptationMatrixIn)
			TempDest = CHAD2Temp(ChromaticAdaptationMatrixOut)

			if TempSrc < 0.0 || TempDest < 0.0 {
				return false // Something went wrong
			}

			if cmsMAT3isIdentity(&Scale) && math.Abs(TempSrc-TempDest) < 0.01 {
				cmsMAT3identity(m)
				return true
			}

			Temp = (1.0-AdaptationState)*TempDest + AdaptationState*TempSrc

			// Get a CHAD from whatever output temperature to D50. This replaces output CHAD
			Temp2CHAD(&MixedCHAD, Temp)

			*m = cmsMAT3per(&m3, &MixedCHAD)
		}
	}
	return true
}

// cmsIntentFn represents a function type for intents.

// Default handler for ICC-style intents

func DefaultICCintents(mm mem.Manager,
	ContextID CmsContext,
	nProfiles uint32,
	TheIntents []uint32,
	hProfiles []CmsHPROFILE,
	BPC []bool,
	AdaptationStates []float64,
	dwFlags uint32,
) *cmsPipeline {
	//	fmt.Println("START DefaultICCintents")
	var (
		Lut               *cmsPipeline
		Result            *cmsPipeline
		hProfile          CmsHPROFILE
		m                 cmsMAT3
		off               cmsVEC3
		ColorSpaceIn      cmsColorSpaceSignature
		ColorSpaceOut     cmsColorSpaceSignature = CmsSigLabData
		CurrentColorSpace cmsColorSpaceSignature
		ClassSig          cmsProfileClassSignature
		Intent            uint32
	)
	// For safety
	if nProfiles == 0 {
		return nil
	}

	// Allocate an empty LUT for holding the result. 0 as channel count means 'undefined'
	Result = cmsPipelineAlloc(mm, ContextID, 0, 0)
	if Result == nil {
		return nil
	}

	CurrentColorSpace = CmsGetColorSpace(hProfiles[0])

	for i := uint32(0); i < nProfiles; i++ {
		var lIsDeviceLink, lIsInput bool

		hProfile = hProfiles[i]
		ClassSig = cmsGetDeviceClass(hProfile)
		lIsDeviceLink = (ClassSig == CmsSigLinkClass || ClassSig == CmsSigAbstractClass)

		// Determine if the profile is input
		if (i == 0) && !lIsDeviceLink {
			lIsInput = true
		} else {
			lIsInput = (CurrentColorSpace != CmsSigXYZData) &&
				(CurrentColorSpace != CmsSigLabData)
		}

		Intent = TheIntents[i]

		if lIsInput || lIsDeviceLink {
			ColorSpaceIn = CmsGetColorSpace(hProfile)
			ColorSpaceOut = cmsGetPCS(hProfile)
		} else {
			ColorSpaceIn = cmsGetPCS(hProfile)
			ColorSpaceOut = CmsGetColorSpace(hProfile)
		}

		if !ColorSpaceIsCompatible(ColorSpaceIn, CurrentColorSpace) {
			cmsSignalError(ContextID, cmsERROR_COLORSPACE_CHECK, "ColorSpace mismatch")
			goto Error
		}

		// If devicelink or named color class
		if lIsDeviceLink || (ClassSig == CmsSigNamedColorClass && nProfiles == 1) {
			Lut = cmsReadDevicelinkLUT(mm, hProfile, Intent)
			if Lut == nil {
				goto Error
			}

			if ClassSig == CmsSigAbstractClass && i > 0 {
				if !ComputeConversion(mm, i, hProfiles, Intent, BPC[i], AdaptationStates[i], &m, &off) {
					goto Error
				}
			} else {
				cmsMAT3identity(&m)
				cmsVEC3init(&off, 0, 0, 0)
			}

			if !AddConversion(mm, Result, CurrentColorSpace, ColorSpaceIn, &m, &off) {
				goto Error
			}
		} else {
			if lIsInput {
				Lut = cmsReadInputLUT(mm, hProfile, Intent)
				if Lut == nil {
					goto Error
				}
			} else {
				Lut = cmsReadOutputLUT(mm, hProfile, Intent)
				if Lut == nil {
					goto Error
				}

				if !ComputeConversion(mm, i, hProfiles, Intent, BPC[i], AdaptationStates[i], &m, &off) {
					goto Error
				}
				if !AddConversion(mm, Result, CurrentColorSpace, ColorSpaceIn, &m, &off) {
					goto Error
				}
			}
		}

		// Concatenate LUT //trying a hack with steal to improve speed
		if !cmsPipelineCatSteal(mm, Result, Lut) {
			goto Error
		}

		cmsPipelineFree(mm, Lut)
		Lut = nil
		// Update current space
		CurrentColorSpace = ColorSpaceOut
	}

	// Handle non-negatives clip
	if dwFlags&CmsFLAGS_NONEGATIVES != 0 {
		if ColorSpaceOut == CmsSigGrayData || ColorSpaceOut == CmsSigRgbData || ColorSpaceOut == CmsSigCmykData {
			clip := cmsStageClipNegatives(mm, Result.ContextID, uint32(cmsChannelsOfColorSpace(ColorSpaceOut)))
			if clip == nil {
				goto Error
			}

			if !cmsPipelineInsertStage(Result, CmsAT_END, clip) {
				goto Error
			}
		}
	}
	//	fmt.Println("END DefaultICCintents")
	//здесь появляется input channel 4 output 3 после cmsDoTransform перед третьим возвращением формы
	return Result

Error:

	if Lut != nil {
		cmsPipelineFree(mm, Lut)
	}
	cmsPipelineFree(mm, Result)
	return nil
}

// IsEmptyLayer checks if the given matrix `m` and offset `off` represent an empty layer.
// Returns true if the layer is effectively empty, false otherwise.
//
// An empty layer is defined as:
// - Both `m` and `off` are NULL (allowed as an empty layer).
// - `m` is NULL but `off` is not (this indicates an internal error).
// - `m` is the identity matrix and `off` is a zero vector.
//
// Tolerance for differences is set to 0.002.
func IsEmptyLayer(m *cmsMAT3, off *cmsVEC3) bool {
	var diff float64 = 0
	var Ident cmsMAT3
	var i int

	// If both m and off are NULL, it's an empty layer.
	if m == nil && off == nil {
		return true
	}

	// If m is NULL but off is not, it's an internal error.
	if m == nil && off != nil {
		return false
	}

	// Initialize the identity matrix for comparison.
	cmsMAT3identity(&Ident)

	// Compare the matrix `m` with the identity matrix.

	for row := 0; row < 3; row++ {
		for col := 0; col < 3; col++ {
			diff += math.Abs(m.V[row].N[col] - Ident.V[row].N[col])
		}
	}

	// Compare the offset `off` with a zero vector.
	for i = 0; i < 3; i++ {
		diff += math.Abs(off.N[i])
	}

	// If the total difference is below the threshold, consider the layer empty.
	return diff < 0.002
}

func ComputeConversion(mm mem.Manager, i uint32, hProfiles []CmsHPROFILE, Intent uint32, BPC bool, AdaptationState float64, m *cmsMAT3, off *cmsVEC3) bool {
	//	fmt.Println("START ComputeConversion")
	// Initialize m and off to identity
	cmsMAT3identity(m)
	cmsVEC3init(off, 0, 0, 0)
	// Handle absolute colorimetric intent
	if Intent == INTENT_ABSOLUTE_COLORIMETRIC {
		var (
			WhitePointIn, WhitePointOut                               cmsCIEXYZ
			ChromaticAdaptationMatrixIn, ChromaticAdaptationMatrixOut cmsMAT3
		)

		if !cmsReadMediaWhitePoint(mm, &WhitePointIn, hProfiles[i-1]) || !cmsReadCHAD(mm, &ChromaticAdaptationMatrixIn, hProfiles[i-1]) {
			return false
		}
		if !cmsReadMediaWhitePoint(mm, &WhitePointOut, hProfiles[i]) || !cmsReadCHAD(mm, &ChromaticAdaptationMatrixOut, hProfiles[i]) {
			return false
		}
		if !ComputeAbsoluteIntent(AdaptationState, &WhitePointIn, &ChromaticAdaptationMatrixIn, &WhitePointOut, &ChromaticAdaptationMatrixOut, m) {
			return false
		}
	} else {
		if BPC {
			// Handle black point compensation
			var BlackPointIn, BlackPointOut cmsCIEXYZ

			cmsDetectBlackPoint(mm, &BlackPointIn, hProfiles[i-1], Intent, 0)
			cmsDetectDestinationBlackPoint(mm, &BlackPointOut, hProfiles[i], Intent, 0)

			// Skip if black points are equal

			if BlackPointIn.X != BlackPointOut.X || BlackPointIn.Y != BlackPointOut.Y || BlackPointIn.Z != BlackPointOut.Z {
				ComputeBlackPointCompensation(&BlackPointIn, &BlackPointOut, m, off)
			}
		}
	}

	// Adjust offset for encoding
	// Offset should be adjusted because the encoding. We encode XYZ normalized to 0..1.0,
	// to do that, we divide by MAX_ENCODEABLE_XZY. The conversion stage goes XYZ -> XYZ so
	// we have first to convert from encoded to XYZ and then convert back to encoded.
	// y = Mx + Off
	// x = x'c
	// y = M x'c + Off
	// y = y'c; y' = y / c
	// y' = (Mx'c + Off) /c = Mx' + (Off / c)
	for k := 0; k < 3; k++ {
		off.N[k] /= MAX_ENCODEABLE_XYZ
	}

	//	fmt.Println("END ComputeConversion")

	return true
}
func AddConversion(mm mem.Manager, Result *cmsPipeline, InPCS cmsColorSpaceSignature, OutPCS cmsColorSpaceSignature, m *cmsMAT3, off *cmsVEC3) bool {
	//	fmt.Println("start AddConversion")
	mAsDbl := MatToSlice(*m)
	offAsDbl := VecToSlice(*off)

	// Handle PCS mismatches
	switch InPCS {
	case CmsSigXYZData: // Input profile operates in XYZ
		switch OutPCS {
		case CmsSigXYZData: // XYZ -> XYZ
			if !IsEmptyLayer(m, off) {
				if !cmsPipelineInsertStage(Result, CmsAT_END, cmsStageAllocMatrix(mm, Result.ContextID, 3, 3, mAsDbl, offAsDbl)) {
					return false
				}
			}
		case CmsSigLabData: // XYZ -> Lab
			if !IsEmptyLayer(m, off) {
				if !cmsPipelineInsertStage(Result, CmsAT_END, cmsStageAllocMatrix(mm, Result.ContextID, 3, 3, mAsDbl, offAsDbl)) {
					return false
				}
			}
			if !cmsPipelineInsertStage(Result, CmsAT_END, cmsStageAllocXYZ2Lab(mm, Result.ContextID)) {
				return false
			}
		default:
			return false // Colorspace mismatch
		}

	case CmsSigLabData: // Input profile operates in Lab
		switch OutPCS {
		case CmsSigXYZData: // Lab -> XYZ
			if !cmsPipelineInsertStage(Result, CmsAT_END, cmsStageAllocLab2XYZ(mm, Result.ContextID)) {
				return false
			}
			if !IsEmptyLayer(m, off) {
				if !cmsPipelineInsertStage(Result, CmsAT_END, cmsStageAllocMatrix(mm, Result.ContextID, 3, 3, mAsDbl, offAsDbl)) {
					return false
				}
			}
		case CmsSigLabData: // Lab -> Lab
			if !IsEmptyLayer(m, off) {
				if !cmsPipelineInsertStage(Result, CmsAT_END, cmsStageAllocLab2XYZ(mm, Result.ContextID)) ||
					!cmsPipelineInsertStage(Result, CmsAT_END, cmsStageAllocMatrix(mm, Result.ContextID, 3, 3, mAsDbl, offAsDbl)) ||
					!cmsPipelineInsertStage(Result, CmsAT_END, cmsStageAllocXYZ2Lab(mm, Result.ContextID)) {
					return false
				}
			}
		default:
			return false // Mismatch
		}

	default:
		// Non-PCS colorspaces must match
		if InPCS != OutPCS {
			return false
		}
	}

	//	fmt.Println("end AddConversion")
	return true
}

// ColorSpaceIsCompatible checks if two color spaces are compatible.
func ColorSpaceIsCompatible(a, b cmsColorSpaceSignature) bool {
	// If they are the same, they are compatible.
	if a == b {
		return true
	}

	// Check for MCH4 substitution of CMYK.
	if (a == CmsSig4colorData && b == CmsSigCmykData) || (a == CmsSigCmykData && b == CmsSig4colorData) {
		return true
	}

	// Check for XYZ/Lab compatibility.
	if (a == CmsSigXYZData && b == CmsSigLabData) || (a == CmsSigLabData && b == CmsSigXYZData) {
		return true
	}

	// If none of the above conditions are met, they are not compatible.
	return false
}
func cmsDefaultICCintents(mm mem.Manager,
	ContextID CmsContext,
	nProfiles uint32,
	TheIntents []uint32,
	hProfiles []CmsHPROFILE,
	BPC []bool,
	AdaptationStates []float64,
	dwFlags uint32,
) *cmsPipeline {
	return DefaultICCintents(mm, ContextID, nProfiles, TheIntents, hProfiles, BPC, AdaptationStates, dwFlags)
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
	KTone     *CmsToneCurve // Black-to-black tone curve
}

// BlackPreservingGrayOnlySampler preserves black-only CMYK transformations.
func BlackPreservingGrayOnlySampler(mm mem.Manager, In []uint16, Out []uint16, cargo any) int32 {
	bp, ok := cargo.(*GrayOnlyParams)
	if !ok {
		cmsSignalError(nil, cmsERROR_UNDEFINED, "Interface data assertion error, not *GrayOnlyParams \n")
		return 0
	}
	// If going across black only, keep black only
	if In[0] == 0 && In[1] == 0 && In[2] == 0 {
		// TAC does not apply because it is black ink!
		Out[0], Out[1], Out[2] = 0, 0, 0
		Out[3] = cmsEvalToneCurve16(mm, bp.KTone, In[3])
		return int32(1)
	}

	// Keep normal transform for other colors
	bp.Cmyk2Cmyk.Eval16Fn(mm, In, Out, bp.Cmyk2Cmyk.Data)
	return int32(1)
}

// BlackPreservingKOnlyIntents handles black-preserving K-only intents.
func BlackPreservingKOnlyIntents(mm mem.Manager,
	ContextID CmsContext,
	nProfiles uint32,
	TheIntents []uint32,
	hProfiles []CmsHPROFILE,
	BPC []bool,
	AdaptationStates []float64,
	dwFlags uint32,
) *cmsPipeline {
	//fmt.Println("BlackPreservingKOnlyIntents")
	var bp GrayOnlyParams
	var Result *cmsPipeline
	var CLUT *cmsStage
	var ICCIntents [256]uint32
	var lastProfilePos, preservationProfilesCount uint32
	var hLastProfile CmsHPROFILE

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

		if CmsGetColorSpace(hLastProfile) != CmsSigCmykData ||
			cmsGetDeviceClass(hLastProfile) != CmsSigLinkClass {
			break
		}
	}

	preservationProfilesCount = lastProfilePos + 1

	// Check for non-CMYK profiles
	if CmsGetColorSpace(hProfiles[0]) != CmsSigCmykData ||
		!(CmsGetColorSpace(hLastProfile) == CmsSigCmykData ||
			cmsGetDeviceClass(hLastProfile) == CmsSigOutputClass) {
		return DefaultICCintents(mm, ContextID, nProfiles, ICCIntents[:], hProfiles, BPC, AdaptationStates, dwFlags)
	}

	// Allocate an empty LUT for holding the result
	Result = cmsPipelineAlloc(mm, ContextID, 4, 4)
	if Result == nil {
		return nil
	}
	var nGridPoints uint32
	// Create a LUT holding normal ICC transform
	bp.Cmyk2Cmyk = DefaultICCintents(mm, ContextID, preservationProfilesCount, ICCIntents[:], hProfiles, BPC, AdaptationStates, dwFlags)
	if bp.Cmyk2Cmyk == nil {
		goto Error
	}

	// Compute the tone curve
	bp.KTone = cmsBuildKToneCurve(mm, ContextID, 4096, preservationProfilesCount, ICCIntents[:], hProfiles, BPC, AdaptationStates, dwFlags)
	if bp.KTone == nil {
		goto Error
	}

	// Determine the number of gridpoints
	nGridPoints = cmsReasonableGridpointsByColorspace(CmsSigCmykData, dwFlags)

	// Create the CLUT
	CLUT = cmsStageAllocCLut16bit(mm, ContextID, nGridPoints, 4, 4, nil)
	if CLUT == nil {
		goto Error
	}

	// Insert CLUT into the pipeline
	if !cmsPipelineInsertStage(Result, CmsAT_BEGIN, CLUT) {
		goto Error
	}

	// Insert possible devicelinks at the end
	for i := lastProfilePos + 1; i < nProfiles; i++ {
		devlink := cmsReadDevicelinkLUT(mm, hProfiles[i], ICCIntents[i])
		if devlink == nil {
			goto Error
		}

		// Steal stages instead of duplicating them
		if !cmsPipelineCatSteal(mm, Result, devlink) {
			cmsPipelineFree(mm, devlink) // free empty header
			goto Error
		}

		// devlink.Elements is now nil, so this only frees the header
		cmsPipelineFree(mm, devlink)
	}

	// Free resources
	cmsPipelineFree(mm, bp.Cmyk2Cmyk)
	CmsFreeToneCurve(bp.KTone)
	return Result

Error:
	if bp.Cmyk2Cmyk != nil {
		cmsPipelineFree(mm, bp.Cmyk2Cmyk)
	}
	if bp.KTone != nil {
		CmsFreeToneCurve(bp.KTone)
	}

	return nil
}

// K Plane-preserving CMYK to CMYK ------------------------------------------------------------------------------------
type PreserveKPlaneParams struct {
	Cmyk2Cmyk    *cmsPipeline  // The original transform
	HProofOutput CmsHTRANSFORM // Output CMYK to Lab (last profile)
	Cmyk2Lab     CmsHTRANSFORM // The input chain
	KTone        *CmsToneCurve // Black-to-black tone curve
	LabK2Cmyk    *cmsPipeline  // The output profile
	MaxError     float64       // Maximum error
	HRoundTrip   CmsHTRANSFORM // Round-trip transform
	MaxTAC       float64       // Maximum total area coverage
}

// BlackPreservingSampler performs sampling for K-plane preservation.
func BlackPreservingSampler(mm mem.Manager, In, Out []uint16, cargo any) int32 {
	bp, ok := cargo.(*PreserveKPlaneParams)
	if !ok {
		cmsSignalError(nil, cmsERROR_UNDEFINED, "Interface data assertion error,not PreserveKPlaneParams\n")
		return 0
	}
	var Inf, Outf, LabK [4]float32
	var ColorimetricLab, BlackPreservingLab cmsCIELab
	var SumCMY, SumCMYK, Error, Ratio float64

	// Convert from 16 bits to floating point
	for i := 0; i < 4; i++ {
		Inf[i] = float32(In[i]) / 65535.0
	}

	// Get the K across Tone curve
	LabK[3] = cmsEvalToneCurveFloat(mm, bp.KTone, Inf[3])

	// If going across black only, keep black only
	if In[0] == 0 && In[1] == 0 && In[2] == 0 {
		Out[0], Out[1], Out[2] = 0, 0, 0
		Out[3] = cmsQuickSaturateWord(float64(LabK[3] * 65535.0))
		return 1
	}

	// Try the original transform
	cmsPipelineEvalFloat(mm, Inf[:], Outf[:], bp.Cmyk2Cmyk)

	// Store a copy of the floating-point result into 16-bit
	for i := 0; i < 4; i++ {
		Out[i] = cmsQuickSaturateWord(float64(Outf[i] * 65535.0))
	}

	// Check if K is already OK
	if math.Abs(float64(Outf[3]-LabK[3])) < (3.0 / 65535.0) {
		return 1
	}

	// Measure and keep Lab measurement for further usage
	CmsDoTransform(mm, bp.HProofOutput, Out, ColorimetricLab, 1)

	// Transform to Lab
	CmsDoTransform(mm, bp.Cmyk2Lab, Outf, LabK, 1)

	// Reverse interpolation to obtain CMY with fixed K
	if !cmsPipelineEvalReverseFloat(mm, LabK[:], Outf[:], Outf[:], bp.LabK2Cmyk) {
		// Use colorimetric transform if reverse interpolation fails
		return 1
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

	Out[0] = cmsQuickSaturateWord(float64(Outf[0] * float32(Ratio) * 65535.0))
	Out[1] = cmsQuickSaturateWord(float64(Outf[1] * float32(Ratio) * 65535.0))
	Out[2] = cmsQuickSaturateWord(float64(Outf[2] * float32(Ratio) * 65535.0))
	Out[3] = cmsQuickSaturateWord(float64(Outf[3] * 65535.0))

	// Estimate the error
	CmsDoTransform(mm, bp.HProofOutput, Out, BlackPreservingLab, 1)
	Error = cmsDeltaE(&ColorimetricLab, &BlackPreservingLab)
	if Error > bp.MaxError {
		bp.MaxError = Error
	}

	return 1
}

// BlackPreservingKPlaneIntents handles black-plane preserving intents.
func BlackPreservingKPlaneIntents(mm mem.Manager,
	ContextID CmsContext,
	nProfiles uint32,
	TheIntents []uint32,
	hProfiles []CmsHPROFILE,
	BPC []bool,
	AdaptationStates []float64,
	dwFlags uint32,
) *cmsPipeline {
	//fmt.Println("BlackPreservingKPlaneIntents")
	var bp PreserveKPlaneParams
	var Result *cmsPipeline
	var CLUT *cmsStage
	var ICCIntents [256]uint32
	var lastProfilePos, preservationProfilesCount uint32
	var hLastProfile, hLab CmsHPROFILE

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

		if CmsGetColorSpace(hLastProfile) != CmsSigCmykData || cmsGetDeviceClass(hLastProfile) != CmsSigLinkClass {
			break
		}
	}

	preservationProfilesCount = lastProfilePos + 1

	// Check for non-CMYK profiles
	if CmsGetColorSpace(hProfiles[0]) != CmsSigCmykData ||
		!(CmsGetColorSpace(hLastProfile) == CmsSigCmykData || cmsGetDeviceClass(hLastProfile) == CmsSigOutputClass) {
		return DefaultICCintents(mm, ContextID, nProfiles, ICCIntents[:], hProfiles, BPC, AdaptationStates, dwFlags)
	}

	// Allocate LUT
	Result = cmsPipelineAlloc(mm, ContextID, 4, 4)
	if Result == nil {
		return nil
	}
	var nGridPoints uint32
	// Read input LUT
	bp.LabK2Cmyk = cmsReadInputLUT(mm, hLastProfile, INTENT_RELATIVE_COLORIMETRIC)
	if bp.LabK2Cmyk == nil {
		goto Cleanup
	}

	// Get TAC
	bp.MaxTAC = cmsDetectTAC(mm, hLastProfile) / 100.0
	if bp.MaxTAC <= 0 {
		goto Cleanup
	}

	// Create ICC transform
	bp.Cmyk2Cmyk = DefaultICCintents(mm, ContextID, preservationProfilesCount, ICCIntents[:], hProfiles, BPC, AdaptationStates, dwFlags)
	if bp.Cmyk2Cmyk == nil {
		goto Cleanup
	}

	// Compute tone curve
	bp.KTone = cmsBuildKToneCurve(mm, ContextID, 4096, preservationProfilesCount, ICCIntents[:], hProfiles, BPC, AdaptationStates, dwFlags)
	if bp.KTone == nil {
		goto Cleanup
	}

	// Prepare proof output
	hLab = cmsCreateLab4ProfileTHR(mm, ContextID, nil)
	bp.HProofOutput = cmsCreateTransformTHR(mm, ContextID, hLastProfile, CHANNELS_SH(4)|BYTES_SH(2), hLab, TYPE_Lab_DBL, INTENT_RELATIVE_COLORIMETRIC, CmsFLAGS_NOCACHE|CmsFLAGS_NOOPTIMIZE)
	if bp.HProofOutput == nil {
		goto Cleanup
	}

	// Prepare CMYK to Lab
	bp.Cmyk2Lab = cmsCreateTransformTHR(mm, ContextID, hLastProfile, FLOAT_SH(1)|CHANNELS_SH(4)|BYTES_SH(4), hLab, FLOAT_SH(1)|CHANNELS_SH(3)|BYTES_SH(4), INTENT_RELATIVE_COLORIMETRIC, CmsFLAGS_NOCACHE|CmsFLAGS_NOOPTIMIZE)
	if bp.Cmyk2Lab == nil {
		goto Cleanup
	}
	CmsCloseProfile(mm, hLab)

	// Create CLUT
	nGridPoints = cmsReasonableGridpointsByColorspace(CmsSigCmykData, dwFlags)
	CLUT = cmsStageAllocCLut16bit(mm, ContextID, nGridPoints, 4, 4, nil)
	if CLUT == nil {
		goto Cleanup
	}

	// Insert and sample CLUT
	if !cmsPipelineInsertStage(Result, CmsAT_BEGIN, CLUT) || !cmsStageSampleCLut16bit(mm, CLUT, BlackPreservingSampler, &bp, 0) {
		goto Cleanup
	}
	// Insert devicelinks
	for i := lastProfilePos + 1; i < nProfiles; i++ {
		devlink := cmsReadDevicelinkLUT(mm, hProfiles[i], ICCIntents[i])
		if devlink == nil {
			goto Cleanup
		}

		if !cmsPipelineCatSteal(mm, Result, devlink) {
			cmsPipelineFree(mm, devlink) // free empty header
			goto Cleanup
		}

		// devlink.Elements is now nil, so freeing just cleans up the header
		cmsPipelineFree(mm, devlink)
	}

Cleanup:
	if bp.Cmyk2Cmyk != nil {
		cmsPipelineFree(mm, bp.Cmyk2Cmyk)
	}
	if bp.Cmyk2Lab != nil {
		CmsDeleteTransform(bp.Cmyk2Lab)
	}
	if bp.HProofOutput != nil {
		CmsDeleteTransform(bp.HProofOutput)
	}
	if bp.KTone != nil {
		CmsFreeToneCurve(bp.KTone)
	}
	if bp.LabK2Cmyk != nil {
		cmsPipelineFree(mm, bp.LabK2Cmyk)
	}

	return Result
}

// Link routines ------------------------------------------------------------------------------------------------------

// Chain several profiles into a single LUT. It just checks the parameters and then calls the handler
// for the first intent in chain. The handler may be user-defined. Is up to the handler to deal with the
// rest of intents in chain. A maximum of 255 profiles at time are supported, which is pretty reasonable.
func cmsLinkProfiles(mm mem.Manager,
	ContextID CmsContext,
	nProfiles uint32,
	TheIntents []uint32,
	hProfiles []CmsHPROFILE,
	BPC []bool,
	AdaptationStates []float64,
	dwFlags uint32,
) *cmsPipeline {
	// Ensure a reasonable number of profiles is provided
	if nProfiles == 0 || nProfiles > 255 {
		cmsSignalError(ContextID, cmsERROR_RANGE, "Couldn't link profiles")
		return nil
	}

	// Loop through profiles to adjust BPC (Black Point Compensation) as needed
	for i := uint32(0); i < nProfiles; i++ {
		// Ensure BPC rules are enforced based on intent
		if TheIntents[i] == INTENT_ABSOLUTE_COLORIMETRIC {
			BPC[i] = false
		}

		if TheIntents[i] == INTENT_PERCEPTUAL || TheIntents[i] == INTENT_SATURATION {
			// Force BPC for V4 profiles in perceptual and saturation
			if cmsGetEncodedICCversion(hProfiles[i]) >= 0x4000000 {
				BPC[i] = true
			}
		}
	}

	// Search for an appropriate intent handler
	intent := SearchIntent(ContextID, TheIntents[0])
	if intent == nil {
		cmsSignalError(ContextID, cmsERROR_UNKNOWN_EXTENSION, "Unsupported intent")
		return nil
	}

	// Call the intent's handler to link profiles
	return intent.Link(mm, ContextID, nProfiles, TheIntents, hProfiles, BPC, AdaptationStates, dwFlags)
}

// cmsRegisterRenderingIntentPlugin registers a rendering intent plugin.
func cmsRegisterRenderingIntentPlugin(mm mem.Manager, id CmsContext, Data PluginIntrfc) bool {
	ctx := CmsContextGetClientChunk(id, IntentPlugin).(*cmsIntentsPluginChunkType)
	// Reset custom intents if Data is nil.
	if Data == nil {
		ctx.Intents = nil
		return true
	}

	plugin, ok := Data.(*cmsPluginRenderingIntent)
	if !ok {
		panic("Plugin is not of the type cmsPluginRenderingIntent\n")
	}

	// Allocate memory for the new intent node.
	//fl := (*cmsIntentsList)(cmsPluginMalloc(id, uint32(unsafe.Sizeof(cmsIntentsList{}))))
	fl := mem.New[cmsIntentsList](mm)

	/*	if fl == nil {
		return false
	}*/

	// Populate the new node's fields.
	fl.Intent = plugin.Intent
	fl.Description = strncpy(plugin.Description, int(unsafe.Sizeof(fl.Description)-1))
	fl.Link = plugin.Link

	// Update the linked list.
	fl.Next = ctx.Intents
	ctx.Intents = fl

	return true
}
