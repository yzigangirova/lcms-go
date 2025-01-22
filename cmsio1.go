package golcms

import (
	"unsafe"
)

// LUT tags
var (
	Device2PCS16 = []cmsTagSignature{
		cmsSigAToB0Tag, // Perceptual
		cmsSigAToB1Tag, // Relative colorimetric
		cmsSigAToB2Tag, // Saturation
		cmsSigAToB1Tag, // Absolute colorimetric
	}

	Device2PCSFloat = []cmsTagSignature{
		cmsSigDToB0Tag, // Perceptual
		cmsSigDToB1Tag, // Relative colorimetric
		cmsSigDToB2Tag, // Saturation
		cmsSigDToB3Tag, // Absolute colorimetric
	}

	PCS2Device16 = []cmsTagSignature{
		cmsSigBToA0Tag, // Perceptual
		cmsSigBToA1Tag, // Relative colorimetric
		cmsSigBToA2Tag, // Saturation
		cmsSigBToA1Tag, // Absolute colorimetric
	}

	PCS2DeviceFloat = []cmsTagSignature{
		cmsSigBToD0Tag, // Perceptual
		cmsSigBToD1Tag, // Relative colorimetric
		cmsSigBToD2Tag, // Saturation
		cmsSigBToD3Tag, // Absolute colorimetric
	}
)

// Factors to convert from 1.15 fixed point to 0..1.0 range and vice-versa
const (
	InpAdj  = 1.0 / MAX_ENCODEABLE_XYZ // (65536.0 / (65535.0 * 2.0))
	OutpAdj = MAX_ENCODEABLE_XYZ       // ((2.0 * 65535.0) / 65536.0)
)

// Several resources for gray conversions
var (
	GrayInputMatrix = []float64{
		InpAdj * cmsD50X,
		InpAdj * cmsD50Y,
		InpAdj * cmsD50Z,
	}

	OneToThreeInputMatrix = []float64{
		1, 1, 1,
	}

	PickYMatrix = []float64{
		0,
		OutpAdj * cmsD50Y,
		0,
	}

	PickLstarMatrix = []float64{
		1, 0, 0,
	}
)

// cmsReadMediaWhitePoint retrieves the media white point and addresses issues in old profiles.
func cmsReadMediaWhitePoint(Dest *cmsCIEXYZ, hProfile cmsHPROFILE) bool {
	// Ensure Dest is not nil
	if Dest == nil {
		return false
	}

	// Read the media white point tag
	Tag := (*cmsCIEXYZ)(cmsReadTag(hProfile, cmsSigMediaWhitePointTag))

	// If no white point, use D50 as default
	if Tag == nil {
		*Dest = *cmsD50_XYZ()
		return true
	}

	// For V2 display profiles, return D50 as the white point
	if cmsGetEncodedICCversion(unsafe.Pointer(hProfile)) < 0x4000000 {
		if cmsGetDeviceClass(unsafe.Pointer(hProfile)) == cmsSigDisplayClass {
			*Dest = *cmsD50_XYZ()
			return true
		}
	}

	// Assign the retrieved tag to Dest
	*Dest = *Tag
	return true
}
func cmsReadCHAD(Dest *cmsMAT3, hProfile cmsHPROFILE) bool {
	if Dest == nil {
		panic("Destination matrix cannot be nil") // Replace cmsAssert
	}

	// Attempt to read the Chromatic Adaptation Tag
	Tag := (*cmsMAT3)(cmsReadTag(hProfile, cmsSigChromaticAdaptationTag))
	if Tag != nil {
		*Dest = *Tag
		return true
	}

	// No CHAD available, default it to identity
	cmsMAT3identity(Dest)

	// For V2 display profiles, ensure D50 as the white point
	if cmsGetEncodedICCversion(unsafe.Pointer(hProfile)) < 0x4000000 {
		if cmsGetDeviceClass(unsafe.Pointer(hProfile)) == cmsSigDisplayClass {
			White := (*cmsCIEXYZ)(cmsReadTag(hProfile, cmsSigMediaWhitePointTag))
			if White == nil {
				cmsMAT3identity(Dest)
				return true
			}

			return cmsAdaptationMatrix(Dest, nil, White, cmsD50_XYZ())
		}
	}

	return true
}
func cmsReadFloatDevicelinkTag(hProfile cmsHPROFILE, tagFloat cmsTagSignature) *cmsPipeline {
	// Get the profile's context ID
	ContextID := cmsGetProfileContextID(hProfile)

	// Duplicate the LUT pipeline from the specified tag
	Lut := cmsPipelineDup((*cmsPipeline)(cmsReadTag(hProfile, tagFloat)))
	if Lut == nil {
		return nil
	}

	// Get the profile's PCS and color space signatures
	PCS := cmsGetPCS(unsafe.Pointer(hProfile))
	spc := cmsGetColorSpace(unsafe.Pointer(hProfile))

	// Check if the source color space is Lab and adjust encoding
	if spc == cmsSigLabData {
		if !cmsPipelineInsertStage(Lut, cmsAT_BEGIN, cmsStageNormalizeToLabFloat(ContextID)) {
			goto Error
		}
	} else if spc == cmsSigXYZData {
		// Check if the source color space is XYZ and adjust encoding
		if !cmsPipelineInsertStage(Lut, cmsAT_BEGIN, cmsStageNormalizeToXyzFloat(ContextID)) {
			goto Error
		}
	}

	// Check if the PCS is Lab and adjust encoding
	if PCS == cmsSigLabData {
		if !cmsPipelineInsertStage(Lut, cmsAT_END, cmsStageNormalizeFromLabFloat(ContextID)) {
			goto Error
		}
	} else if PCS == cmsSigXYZData {
		// Check if the PCS is XYZ and adjust encoding
		if !cmsPipelineInsertStage(Lut, cmsAT_END, cmsStageNormalizeFromXyzFloat(ContextID)) {
			goto Error
		}
	}

	// Return the adjusted LUT
	return Lut

Error:
	// Free the LUT pipeline if an error occurs
	cmsPipelineFree(Lut)
	return nil
}

func cmsReadDevicelinkLUT(hProfile cmsHPROFILE, Intent uint32) *cmsPipeline {
	ContextID := cmsGetProfileContextID(hProfile)

	if Intent > INTENT_ABSOLUTE_COLORIMETRIC {
		return nil
	}

	tag16 := Device2PCS16[Intent]
	tagFloat := Device2PCSFloat[Intent]

	// Handle named color profiles
	if cmsGetDeviceClass(unsafe.Pointer(hProfile)) == cmsSigNamedColorClass {
		nc := (*cmsNAMEDCOLORLIST)(cmsReadTag(hProfile, cmsSigNamedColor2Tag))
		if nc == nil {
			return nil
		}

		Lut := cmsPipelineAlloc(ContextID, 0, 0)
		if Lut == nil {
			goto Error
		}

		if !cmsPipelineInsertStage(Lut, cmsAT_BEGIN, cmsStageAllocNamedColor(nc, false)) {
			goto Error
		}

		if cmsGetColorSpace(unsafe.Pointer(hProfile)) == cmsSigLabData {
			if !cmsPipelineInsertStage(Lut, cmsAT_END, cmsStageAllocLabV2ToV4(ContextID)) {
				goto Error
			}
		}

		return Lut

	Error:
		cmsPipelineFree(Lut)
		return nil
	}

	// Handle floating point LUTs
	if cmsIsTag(hProfile, tagFloat) {
		return cmsReadFloatDevicelinkTag(hProfile, tagFloat)
	}

	tagFloat = Device2PCSFloat[0]
	if cmsIsTag(hProfile, tagFloat) {
		return cmsPipelineDup((*cmsPipeline)(cmsReadTag(hProfile, tagFloat)))
	}

	// Check for 16-bit LUTs
	if !cmsIsTag(hProfile, tag16) {
		tag16 = Device2PCS16[0]
		if !cmsIsTag(hProfile, tag16) {
			return nil
		}
	}

	// Read the tag
	Lut := (*cmsPipeline)(cmsReadTag(hProfile, tag16))
	if Lut == nil {
		return nil
	}

	// Duplicate the pipeline as the profile owns the original
	Lut = cmsPipelineDup(Lut)
	if Lut == nil {
		return nil
	}

	// Adjust interpolation for Lab PCS
	if cmsGetPCS(unsafe.Pointer(hProfile)) == cmsSigLabData {
		ChangeInterpolationToTrilinear(Lut)
	}

	// Check original tag type
	OriginalType := cmsGetTagTrueType(hProfile, tag16)

	// Adjust for Lab16 output
	if OriginalType != cmsSigLut16Type {
		return Lut
	}

	if cmsGetColorSpace(unsafe.Pointer(hProfile)) == cmsSigLabData {
		if !cmsPipelineInsertStage(Lut, cmsAT_BEGIN, cmsStageAllocLabV4ToV2(ContextID)) {
			goto Error2
		}
	}

	if cmsGetPCS(unsafe.Pointer(hProfile)) == cmsSigLabData {
		if !cmsPipelineInsertStage(Lut, cmsAT_END, cmsStageAllocLabV2ToV4(ContextID)) {
			goto Error2
		}
	}

	return Lut

Error2:
	cmsPipelineFree(Lut)
	return nil
}

// ReadICCMatrixRGB2XYZ translates the given function
func ReadICCMatrixRGB2XYZ(r *cmsMAT3, hProfile cmsHPROFILE) bool {
	if r == nil {
		panic("r cannot be nil") // Equivalent to `_cmsAssert`
	}

	PtrRed := (*cmsCIEXYZ)(cmsReadTag(hProfile, cmsSigRedColorantTag))
	PtrGreen := (*cmsCIEXYZ)(cmsReadTag(hProfile, cmsSigGreenColorantTag))
	PtrBlue := (*cmsCIEXYZ)(cmsReadTag(hProfile, cmsSigBlueColorantTag))

	if PtrRed == nil || PtrGreen == nil || PtrBlue == nil {
		return false
	}

	cmsVEC3init(&r.V[0], PtrRed.X, PtrGreen.X, PtrBlue.X)
	cmsVEC3init(&r.V[1], PtrRed.Y, PtrGreen.Y, PtrBlue.Y)
	cmsVEC3init(&r.V[2], PtrRed.Z, PtrGreen.Z, PtrBlue.Z)

	return true
}

// BuildGrayInputMatrixPipeline translates the first function
func BuildGrayInputMatrixPipeline(hProfile cmsHPROFILE) *cmsPipeline {
	ContextID := cmsGetProfileContextID(hProfile)
	GrayTRC := (*cmsToneCurve)(cmsReadTag(hProfile, cmsSigGrayTRCTag))
	if GrayTRC == nil {
		return nil
	}

	Lut := cmsPipelineAlloc(ContextID, 1, 3)
	if Lut == nil {
		goto Error
	}

	if cmsGetPCS(unsafe.Pointer(hProfile)) == cmsSigLabData {
		Zero := [2]uint16{0x8080, 0x8080}
		EmptyTab := cmsBuildTabulatedToneCurve16(ContextID, 2, &Zero[0])
		if EmptyTab == nil {
			goto Error
		}

		LabCurves := [3]*cmsToneCurve{GrayTRC, EmptyTab, EmptyTab}

		if !cmsPipelineInsertStage(Lut, cmsAT_END, cmsStageAllocMatrix(ContextID, 3, 1, OneToThreeInputMatrix, nil)) ||
			!cmsPipelineInsertStage(Lut, cmsAT_END, cmsStageAllocToneCurves(ContextID, 3, &LabCurves[0])) {
			cmsFreeToneCurve(EmptyTab)
			goto Error
		}

		cmsFreeToneCurve(EmptyTab)
	} else {
		if !cmsPipelineInsertStage(Lut, cmsAT_END, cmsStageAllocToneCurves(ContextID, 1, &GrayTRC)) ||
			!cmsPipelineInsertStage(Lut, cmsAT_END, cmsStageAllocMatrix(ContextID, 3, 1, GrayInputMatrix, nil)) {
			goto Error
		}
	}

	return Lut

Error:
	cmsPipelineFree(Lut)
	return nil
}

// BuildRGBInputMatrixShaper translates the second function
func BuildRGBInputMatrixShaper(hProfile cmsHPROFILE) *cmsPipeline {
	ContextID := cmsGetProfileContextID(hProfile)
	var Mat cmsMAT3

	if !ReadICCMatrixRGB2XYZ(&Mat, hProfile) {
		return nil
	}

	// Adjust the matrix values
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			Mat.V[i].N[j] *= InpAdj
		}
	}

	Shapes := [3]*cmsToneCurve{
		(*cmsToneCurve)(cmsReadTag(hProfile, cmsSigRedTRCTag)),
		(*cmsToneCurve)(cmsReadTag(hProfile, cmsSigGreenTRCTag)),
		(*cmsToneCurve)(cmsReadTag(hProfile, cmsSigBlueTRCTag)),
	}

	if Shapes[0] == nil || Shapes[1] == nil || Shapes[2] == nil {
		return nil
	}

	Lut := cmsPipelineAlloc(ContextID, 3, 3)
	if Lut != nil {

		if !cmsPipelineInsertStage(Lut, cmsAT_END, cmsStageAllocToneCurves(ContextID, 3, &Shapes[0])) ||
			!cmsPipelineInsertStage(Lut, cmsAT_END, cmsStageAllocMatrix(ContextID, 3, 3, unsafe.Slice(((*float64)(&(Mat.V[0].N[0]))), 9), nil)) {
			goto Error
		}

		if cmsGetPCS(unsafe.Pointer(hProfile)) == cmsSigLabData {
			if !cmsPipelineInsertStage(Lut, cmsAT_END, cmsStageAllocXYZ2Lab(ContextID)) {
				goto Error
			}
		}
		return Lut
	}

Error:
	cmsPipelineFree(Lut)
	return nil
}

// cmsReadFloatInputTag translates the first function
func cmsReadFloatInputTag(hProfile cmsHPROFILE, tagFloat cmsTagSignature) *cmsPipeline {
	ContextID := cmsGetProfileContextID(hProfile)
	Lut := cmsPipelineDup((*cmsPipeline)(cmsReadTag(hProfile, tagFloat)))
	spc := cmsGetColorSpace(unsafe.Pointer(hProfile))
	PCS := cmsGetPCS(unsafe.Pointer(hProfile))

	if Lut == nil {
		return nil
	}

	if spc == cmsSigLabData {
		if !cmsPipelineInsertStage(Lut, cmsAT_BEGIN, cmsStageNormalizeToLabFloat(ContextID)) {
			goto Error
		}
	} else if spc == cmsSigXYZData {
		if !cmsPipelineInsertStage(Lut, cmsAT_BEGIN, cmsStageNormalizeToXyzFloat(ContextID)) {
			goto Error
		}
	}

	if PCS == cmsSigLabData {
		if !cmsPipelineInsertStage(Lut, cmsAT_END, cmsStageNormalizeFromLabFloat(ContextID)) {
			goto Error
		}
	} else if PCS == cmsSigXYZData {
		if !cmsPipelineInsertStage(Lut, cmsAT_END, cmsStageNormalizeFromXyzFloat(ContextID)) {
			goto Error
		}
	}

	return Lut

Error:
	cmsPipelineFree(Lut)
	return nil
}

// cmsReadInputLUT translates the second function
func cmsReadInputLUT(hProfile cmsHPROFILE, Intent uint32) *cmsPipeline {
	ContextID := cmsGetProfileContextID(hProfile)

	if cmsGetDeviceClass(unsafe.Pointer(hProfile)) == cmsSigNamedColorClass {
		nc := (*cmsNAMEDCOLORLIST)(cmsReadTag(hProfile, cmsSigNamedColor2Tag))
		if nc == nil {
			return nil
		}

		Lut := cmsPipelineAlloc(ContextID, 0, 0)
		if Lut == nil {
			return nil
		}

		if !cmsPipelineInsertStage(Lut, cmsAT_BEGIN, cmsStageAllocNamedColor(nc, true)) ||
			!cmsPipelineInsertStage(Lut, cmsAT_END, cmsStageAllocLabV2ToV4(ContextID)) {
			cmsPipelineFree(Lut)
			return nil
		}
		return Lut
	}

	if Intent <= INTENT_ABSOLUTE_COLORIMETRIC {
		tag16 := Device2PCS16[Intent]
		tagFloat := Device2PCSFloat[Intent]

		if cmsIsTag(hProfile, tagFloat) {
			return cmsReadFloatInputTag(hProfile, tagFloat)
		}

		if !cmsIsTag(hProfile, tag16) {
			tag16 = Device2PCS16[0]
		}

		if cmsIsTag(hProfile, tag16) {
			Lut := (*cmsPipeline)(cmsReadTag(hProfile, tag16))
			if Lut == nil {
				return nil
			}

			OriginalType := cmsGetTagTrueType(hProfile, tag16)
			Lut = cmsPipelineDup(Lut)

			if OriginalType != cmsSigLut16Type || cmsGetPCS(unsafe.Pointer(hProfile)) != cmsSigLabData {
				return Lut
			}

			if cmsGetColorSpace(unsafe.Pointer(hProfile)) == cmsSigLabData &&
				!cmsPipelineInsertStage(Lut, cmsAT_BEGIN, cmsStageAllocLabV4ToV2(ContextID)) {
				cmsPipelineFree(Lut)
				return nil
			}

			if !cmsPipelineInsertStage(Lut, cmsAT_END, cmsStageAllocLabV2ToV4(ContextID)) {
				cmsPipelineFree(Lut)
				return nil
			}
			return Lut
		}
	}

	if cmsGetColorSpace(unsafe.Pointer(hProfile)) == cmsSigGrayData {
		return BuildGrayInputMatrixPipeline(hProfile)
	}

	return BuildRGBInputMatrixShaper(hProfile)
}

// ---------------------------------------------------------------------------------------------------------------

// Gray output pipeline.
// XYZ -> Gray or Lab -> Gray. Since we only know the GrayTRC, we need to do some assumptions. Gray component will be
// given by Y on XYZ PCS and by L* on Lab PCS, Both across inverse TRC curve.
// The complete pipeline on XYZ is Matrix[3:1] -> Tone curve and in Lab Matrix[3:1] -> Tone Curve as well.

func BuildGrayOutputPipeline(hProfile cmsHPROFILE) *cmsPipeline {
	ContextID := cmsGetProfileContextID(hProfile)
	GrayTRC := (*cmsToneCurve)(cmsReadTag(hProfile, cmsSigGrayTRCTag))
	if GrayTRC == nil {
		return nil
	}

	RevGrayTRC := cmsReverseToneCurve(GrayTRC)
	if RevGrayTRC == nil {
		return nil
	}

	Lut := cmsPipelineAlloc(ContextID, 3, 1)
	if Lut == nil {
		cmsFreeToneCurve(RevGrayTRC)
		return nil
	}

	if cmsGetPCS(unsafe.Pointer(hProfile)) == cmsSigLabData {
		if !cmsPipelineInsertStage(Lut, cmsAT_END, cmsStageAllocMatrix(ContextID, 1, 3, PickLstarMatrix, nil)) {
			cmsFreeToneCurve(RevGrayTRC)
			cmsPipelineFree(Lut)
			return nil
		}
	} else {
		if !cmsPipelineInsertStage(Lut, cmsAT_END, cmsStageAllocMatrix(ContextID, 1, 3, PickYMatrix, nil)) {
			cmsFreeToneCurve(RevGrayTRC)
			cmsPipelineFree(Lut)
			return nil
		}
	}

	if !cmsPipelineInsertStage(Lut, cmsAT_END, cmsStageAllocToneCurves(ContextID, 1, &RevGrayTRC)) {
		cmsFreeToneCurve(RevGrayTRC)
		cmsPipelineFree(Lut)
		return nil
	}

	cmsFreeToneCurve(RevGrayTRC)
	return Lut
}

// BuildRGBOutputMatrixShaper translates the given function
func BuildRGBOutputMatrixShaper(hProfile cmsHPROFILE) *cmsPipeline {
	ContextID := cmsGetProfileContextID(hProfile)
	var Mat, Inv cmsMAT3
	var Shapes, InvShapes [3]*cmsToneCurve

	if !ReadICCMatrixRGB2XYZ(&Mat, hProfile) {
		return nil
	}

	if !cmsMAT3inverse(&Mat, &Inv) {
		return nil
	}

	// Adjust the matrix for output encoding
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			Inv.V[i].N[j] *= OutpAdj
		}
	}

	Shapes[0] = (*cmsToneCurve)(cmsReadTag(hProfile, cmsSigRedTRCTag))
	Shapes[1] = (*cmsToneCurve)(cmsReadTag(hProfile, cmsSigGreenTRCTag))
	Shapes[2] = (*cmsToneCurve)(cmsReadTag(hProfile, cmsSigBlueTRCTag))

	if Shapes[0] == nil || Shapes[1] == nil || Shapes[2] == nil {
		return nil
	}

	InvShapes[0] = cmsReverseToneCurve(Shapes[0])
	InvShapes[1] = cmsReverseToneCurve(Shapes[1])
	InvShapes[2] = cmsReverseToneCurve(Shapes[2])

	if InvShapes[0] == nil || InvShapes[1] == nil || InvShapes[2] == nil {
		return nil
	}

	Lut := cmsPipelineAlloc(ContextID, 3, 3)
	if Lut != nil {
		// Handle profiles with Lab PCS
		if cmsGetPCS(unsafe.Pointer(hProfile)) == cmsSigLabData {
			if !cmsPipelineInsertStage(Lut, cmsAT_END, cmsStageAllocLab2XYZ(ContextID)) {
				goto Error
			}
		}

		if !cmsPipelineInsertStage(Lut, cmsAT_END, cmsStageAllocMatrix(ContextID, 3, 3, unsafe.Slice(((*float64)(&(Inv.V[0].N[0]))), 9), nil)) ||
			!cmsPipelineInsertStage(Lut, cmsAT_END, cmsStageAllocToneCurves(ContextID, 3, &InvShapes[0])) {
			goto Error
		}
	}

	cmsFreeToneCurveTriple(InvShapes)
	return Lut

Error:
	cmsFreeToneCurveTriple(InvShapes)
	cmsPipelineFree(Lut)
	return nil
}

func ChangeInterpolationToTrilinear(Lut *cmsPipeline) {
	for Stage := cmsPipelineGetPtrToFirstStage(Lut); Stage != nil; Stage = cmsStageNext(Stage) {
		if cmsStageType(Stage) == cmsSigCLutElemType {
			CLUT := (*cmsStageCLutData)(Stage.Data)
			CLUT.Params.dwFlags |= CMS_LERP_FLAGS_TRILINEAR
			cmsSetInterpolationRoutine(Lut.ContextID, CLUT.Params)
		}
	}
}

// _cmsReadFloatOutputTag translates the given function
func cmsReadFloatOutputTag(hProfile cmsHPROFILE, tagFloat cmsTagSignature) *cmsPipeline {
	ContextID := cmsGetProfileContextID(hProfile)
	Lut := cmsPipelineDup((*cmsPipeline)(cmsReadTag(hProfile, tagFloat)))
	PCS := cmsGetPCS(unsafe.Pointer(hProfile))
	dataSpace := cmsGetColorSpace(unsafe.Pointer(hProfile))

	if Lut == nil {
		return nil
	}

	// If PCS is Lab or XYZ, adjust normalization at the beginning of the pipeline
	if PCS == cmsSigLabData {
		if !cmsPipelineInsertStage(Lut, cmsAT_BEGIN, cmsStageNormalizeToLabFloat(ContextID)) {
			goto Error
		}
	} else if PCS == cmsSigXYZData {
		if !cmsPipelineInsertStage(Lut, cmsAT_BEGIN, cmsStageNormalizeToXyzFloat(ContextID)) {
			goto Error
		}
	}

	// If the output is Lab or XYZ, normalization is needed at the end of the pipeline
	if dataSpace == cmsSigLabData {
		if !cmsPipelineInsertStage(Lut, cmsAT_END, cmsStageNormalizeFromLabFloat(ContextID)) {
			goto Error
		}
	} else if dataSpace == cmsSigXYZData {
		if !cmsPipelineInsertStage(Lut, cmsAT_END, cmsStageNormalizeFromXyzFloat(ContextID)) {
			goto Error
		}
	}

	return Lut

Error:
	cmsPipelineFree(Lut)
	return nil
}

func cmsReadOutputLUT(hProfile cmsHPROFILE, Intent uint32) *cmsPipeline {
	ContextID := cmsGetProfileContextID(hProfile)

	if Intent <= INTENT_ABSOLUTE_COLORIMETRIC {
		tag16 := PCS2Device16[Intent]
		tagFloat := PCS2DeviceFloat[Intent]

		if cmsIsTag(hProfile, tagFloat) {
			return cmsReadFloatOutputTag(hProfile, tagFloat)
		}

		if !cmsIsTag(hProfile, tag16) {
			tag16 = PCS2Device16[0]
		}

		if cmsIsTag(hProfile, tag16) {
			Lut := (*cmsPipeline)(cmsReadTag(hProfile, tag16))
			if Lut == nil {
				return nil
			}

			OriginalType := cmsGetTagTrueType(hProfile, tag16)
			Lut = cmsPipelineDup(Lut)

			if cmsGetPCS(unsafe.Pointer(hProfile)) == cmsSigLabData {
				ChangeInterpolationToTrilinear(Lut)
			}

			if OriginalType != cmsSigLut16Type || cmsGetPCS(unsafe.Pointer(hProfile)) != cmsSigLabData {
				return Lut
			}

			if !cmsPipelineInsertStage(Lut, cmsAT_BEGIN, cmsStageAllocLabV4ToV2(ContextID)) {
				cmsPipelineFree(Lut)
				return nil
			}

			if cmsGetColorSpace(unsafe.Pointer(hProfile)) == cmsSigLabData &&
				!cmsPipelineInsertStage(Lut, cmsAT_END, cmsStageAllocLabV2ToV4(ContextID)) {
				cmsPipelineFree(Lut)
				return nil
			}

			return Lut
		}
	}

	if cmsGetColorSpace(unsafe.Pointer(hProfile)) == cmsSigGrayData {
		return BuildGrayOutputPipeline(hProfile)
	}

	return BuildRGBOutputMatrixShaper(hProfile)
}
func cmsIsMatrixShaper(hProfile cmsHPROFILE) bool {
	switch cmsGetColorSpace(unsafe.Pointer(hProfile)) {

	case cmsSigGrayData:
		return cmsIsTag(hProfile, cmsSigGrayTRCTag)

	case cmsSigRgbData:
		return cmsIsTag(hProfile, cmsSigRedColorantTag) &&
			cmsIsTag(hProfile, cmsSigGreenColorantTag) &&
			cmsIsTag(hProfile, cmsSigBlueColorantTag) &&
			cmsIsTag(hProfile, cmsSigRedTRCTag) &&
			cmsIsTag(hProfile, cmsSigGreenTRCTag) &&
			cmsIsTag(hProfile, cmsSigBlueTRCTag)

	default:
		return false
	}
}
func cmsIsCLUT(hProfile cmsHPROFILE, Intent uint32, UsedDirection uint32) bool {
	var TagTable *cmsTagSignature

	// For devicelinks, the supported intent is the one stated in the header
	if cmsGetDeviceClass(unsafe.Pointer(hProfile)) == cmsSigLinkClass {
		return cmsGetHeaderRenderingIntent(unsafe.Pointer(hProfile)) == Intent
	}

	switch UsedDirection {

	case LCMS_USED_AS_INPUT:
		TagTable = &Device2PCS16[0]

	case LCMS_USED_AS_OUTPUT:
		TagTable = &PCS2Device16[0]

	case LCMS_USED_AS_PROOF:
		return cmsIsIntentSupported(hProfile, Intent, LCMS_USED_AS_INPUT) &&
			cmsIsIntentSupported(hProfile, INTENT_RELATIVE_COLORIMETRIC, LCMS_USED_AS_OUTPUT)

	default:
		cmsSignalError(unsafe.Pointer(cmsGetProfileContextID(hProfile)), cmsERROR_RANGE, "Unexpected direction ")
		return false
	}

	// Extended intents are not strictly CLUT-based
	if Intent > INTENT_ABSOLUTE_COLORIMETRIC {
		return false
	}
	// Use unsafe to index into TagTable
	tag := *(*cmsTagSignature)(unsafe.Add(unsafe.Pointer(TagTable), uintptr(Intent)*unsafe.Sizeof(cmsTagSignature(0))))

	return cmsIsTag(hProfile, tag)

	
}

func cmsIsIntentSupported(hProfile cmsHPROFILE, Intent uint32, UsedDirection uint32) bool {
	// Check if the intent is implemented as CLUT
	if cmsIsCLUT(hProfile, Intent, UsedDirection) {
		return true
	}

	// Check for matrix-shaper support
	return cmsIsMatrixShaper(hProfile)
}

// cmsReadProfileSequence reads both profile sequence description and profile sequence ID if present,
// then combines them into a unique structure holding both.

// cmsReadProfileSequence translates the provided function
func cmsReadProfileSequence(hProfile cmsHPROFILE) *cmsSEQ {
	var ProfileSeq, ProfileId, NewSeq *cmsSEQ

	// Take profile sequence description first
	ProfileSeq = (*cmsSEQ)(cmsReadTag(hProfile, cmsSigProfileSequenceDescTag))

	// Take profile sequence ID
	ProfileId = (*cmsSEQ)(cmsReadTag(hProfile, cmsSigProfileSequenceIdTag))

	// Handle cases where either or both are NULL
	if ProfileSeq == nil && ProfileId == nil {
		return nil
	}
	if ProfileSeq == nil {
		return cmsDupProfileSequenceDescription(ProfileId)
	}
	if ProfileId == nil {
		return cmsDupProfileSequenceDescription(ProfileSeq)
	}

	// Check if sequence lengths match; otherwise, duplicate the description sequence
	if ProfileSeq.n != ProfileId.n {
		return cmsDupProfileSequenceDescription(ProfileSeq)
	}

	// Duplicate the profile sequence description
	NewSeq = cmsDupProfileSequenceDescription(ProfileSeq)

	// Mix profile sequence ID into the new sequence
	if NewSeq != nil {
		for i := uint32(0); i < ProfileSeq.n; i++ {
			// Compute the address of the ith element in the seq pointer
			NewSeqSeqPtr := (*cmsPSEQDESC)(unsafe.Add(unsafe.Pointer(NewSeq.seq), uintptr(i)*unsafe.Sizeof(cmsPSEQDESC{})))
			ProfileIdSeqPtr := (*cmsPSEQDESC)(unsafe.Add(unsafe.Pointer(ProfileId.seq), uintptr(i)*unsafe.Sizeof(cmsPSEQDESC{})))

			// Copy the ProfileID
			memmove(unsafe.Pointer(&NewSeqSeqPtr.ProfileID), unsafe.Pointer(&ProfileIdSeqPtr.ProfileID), unsafe.Sizeof(cmsProfileID{}))

			// Duplicate the Description
			NewSeqSeqPtr.Description = cmsMLUdup(ProfileIdSeqPtr.Description)
		}
	}

	return NewSeq
}

// cmsWriteProfileSequence dumps the contents of the profile sequence in both tags (if v4 is available).
func cmsWriteProfileSequence(hProfile cmsHPROFILE, seq *cmsSEQ) bool {
	// Write the profile sequence description tag
	if !cmsWriteTag(hProfile, cmsSigProfileSequenceDescTag, unsafe.Pointer(seq)) {
		return false
	}

	// If the profile is version 4 or later, write the profile sequence ID tag
	if cmsGetEncodedICCversion(unsafe.Pointer(hProfile)) >= 0x4000000 {
		if !cmsWriteTag(hProfile, cmsSigProfileSequenceIdTag, unsafe.Pointer(seq)) {
			return false
		}
	}

	return true
}

// GetMLUFromProfile reads and duplicates an MLU tag from the profile if found.
func GetMLUFromProfile(h cmsHPROFILE, sig cmsTagSignature) *cmsMLU {
	mlu := (*cmsMLU)(cmsReadTag(h, sig))
	if mlu == nil {
		return nil
	}

	return cmsMLUdup(mlu)
}

func cmsCompileProfileSequence(ContextID cmsContext, nProfiles uint32, hProfiles []cmsHPROFILE) *cmsSEQ {
	// Allocate a profile sequence description
	seq := cmsAllocProfileSequenceDescription(ContextID, nProfiles)
	if seq == nil {
		return nil
	}

	// Iterate through profiles and populate the sequence
	for i := uint32(0); i < nProfiles; i++ {
		NewSeqSeqPtr := (*cmsPSEQDESC)(unsafe.Add(unsafe.Pointer(seq.seq), uintptr(i)*unsafe.Sizeof(cmsPSEQDESC{})))
		ps := NewSeqSeqPtr // Reference to the current profile sequence descriptor
		h := hProfiles[i]  // Current profile

		// Extract header attributes
		cmsGetHeaderAttributes(unsafe.Pointer(h), &ps.attributes)
		cmsGetHeaderProfileID(unsafe.Pointer(h), &ps.ProfileID.ID8[0])
		ps.deviceMfg = cmsSignature(cmsGetHeaderManufacturer(unsafe.Pointer(h)))
		ps.deviceModel = cmsSignature(cmsGetHeaderModel(unsafe.Pointer(h)))

		// Retrieve technology tag
		techpt := (*cmsTechnologySignature)(cmsReadTag(h, cmsSigTechnologyTag))
		if techpt == nil {
			ps.technology = cmsTechnologySignature(0)
		} else {
			ps.technology = *techpt
		}

		// Retrieve MLU tags
		ps.Manufacturer = GetMLUFromProfile(h, cmsSigDeviceMfgDescTag)
		ps.Model = GetMLUFromProfile(h, cmsSigDeviceModelDescTag)
		ps.Description = GetMLUFromProfile(h, cmsSigProfileDescriptionTag)
	}

	return seq
}
