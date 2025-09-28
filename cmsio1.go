package golcms

import "github.com/yzigangirova/lcms-go/mem"

// LUT tags
var (
	Device2PCS16 = []cmsTagSignature{
		CmsSigAToB0Tag, // Perceptual
		CmsSigAToB1Tag, // Relative colorimetric
		CmsSigAToB2Tag, // Saturation
		CmsSigAToB1Tag, // Absolute colorimetric
	}

	Device2PCSFloat = []cmsTagSignature{
		CmsSigDToB0Tag, // Perceptual
		CmsSigDToB1Tag, // Relative colorimetric
		CmsSigDToB2Tag, // Saturation
		CmsSigDToB3Tag, // Absolute colorimetric
	}

	PCS2Device16 = []cmsTagSignature{
		CmsSigBToA0Tag, // Perceptual
		CmsSigBToA1Tag, // Relative colorimetric
		CmsSigBToA2Tag, // Saturation
		CmsSigBToA1Tag, // Absolute colorimetric
	}

	PCS2DeviceFloat = []cmsTagSignature{
		CmsSigBToD0Tag, // Perceptual
		CmsSigBToD1Tag, // Relative colorimetric
		CmsSigBToD2Tag, // Saturation
		CmsSigBToD3Tag, // Absolute colorimetric
	}
)

// Factors to convert from 1.15 fixed point to 0..1.0 range and vice-versa
const (
	InpAdj  = float64(1.0 / MAX_ENCODEABLE_XYZ) // (65536.0 / (65535.0 * 2.0))
	OutpAdj = float64(MAX_ENCODEABLE_XYZ)       // ((2.0 * 65535.0) / 65536.0)
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
func cmsReadMediaWhitePoint(mm mem.Manager, Dest *cmsCIEXYZ, hProfile CmsHPROFILE) bool {
	// Ensure Dest is not nil
	if Dest == nil {
		return false
	}

	// Read the media white point tag
	Tag, ok := cmsReadTag(mm, hProfile, CmsSigMediaWhitePointTag).(*cmsCIEXYZ)
	// If no white point, use D50 as default
	if Tag == nil {
		*Dest = *cmsD50_XYZ()
		return true
	}
	//not nil and the wrong structure
	if !ok {
		panic("Tag is not of the type *cmsCIEXYZ\n")

	}

	// For V2 display profiles, return D50 as the white point
	if cmsGetEncodedICCversion(hProfile) < 0x4000000 {
		if cmsGetDeviceClass(hProfile) == CmsSigDisplayClass {
			*Dest = *cmsD50_XYZ()
			return true
		}
	}

	// Assign the retrieved tag to Dest
	*Dest = *Tag
	return true
}
func cmsReadCHAD(mm mem.Manager, Dest *cmsMAT3, hProfile CmsHPROFILE) bool {
	if Dest == nil {
		cmsAssert(Dest != nil, "Destination matrix cannot be nil")
	}

	// Attempt to read the Chromatic Adaptation Tag
	Tag, ok := cmsReadTag(mm, hProfile, CmsSigChromaticAdaptationTag).(*cmsMAT3)
	if Tag != nil {
		*Dest = *Tag
		return true
	}
	if !ok {
		panic("tag is not of the type *cmsMAT3\n")

	}

	// No CHAD available, default it to identity
	cmsMAT3identity(Dest)

	// For V2 display profiles, ensure D50 as the white point
	if cmsGetEncodedICCversion(hProfile) < 0x4000000 {
		if cmsGetDeviceClass(hProfile) == CmsSigDisplayClass {
			White, ok := cmsReadTag(mm, hProfile, CmsSigMediaWhitePointTag).(*cmsCIEXYZ)
			if White == nil {
				cmsMAT3identity(Dest)
				return true
			}
			if !ok {
				panic("tag is not of the type *cmsCIEXYZ\n")

			}
			return cmsAdaptationMatrix(Dest, nil, White, cmsD50_XYZ())
		}
	}

	return true
}
func cmsReadFloatDevicelinkTag(mm mem.Manager, hProfile CmsHPROFILE, tagFloat cmsTagSignature) *cmsPipeline {
	// Get the profile's context ID
	ContextID := cmsGetProfileContextID(hProfile)

	// Duplicate the LUT pipeline from the specified tag
	pl, ok := cmsReadTag(mm, hProfile, tagFloat).(*cmsPipeline)
	if pl == nil {
		return nil
	}
	if !ok {
		panic("tag is not of the type *cmsPipeline\n")
	}
	Lut := cmsPipelineDup(mm, pl)
	if Lut == nil {
		return nil
	}

	// Get the profile's PCS and color space signatures
	PCS := cmsGetPCS(hProfile)
	spc := CmsGetColorSpace(hProfile)

	// Check if the source color space is Lab and adjust encoding
	if spc == CmsSigLabData {
		if !cmsPipelineInsertStage(Lut, CmsAT_BEGIN, cmsStageNormalizeToLabFloat(mm, ContextID)) {
			goto Error
		}
	} else if spc == CmsSigXYZData {
		// Check if the source color space is XYZ and adjust encoding
		if !cmsPipelineInsertStage(Lut, CmsAT_BEGIN, cmsStageNormalizeToXyzFloat(mm, ContextID)) {
			goto Error
		}
	}

	// Check if the PCS is Lab and adjust encoding
	if PCS == CmsSigLabData {
		if !cmsPipelineInsertStage(Lut, CmsAT_END, cmsStageNormalizeFromLabFloat(mm, ContextID)) {
			goto Error
		}
	} else if PCS == CmsSigXYZData {
		// Check if the PCS is XYZ and adjust encoding
		if !cmsPipelineInsertStage(Lut, CmsAT_END, cmsStageNormalizeFromXyzFloat(mm, ContextID)) {
			goto Error
		}
	}

	// Return the adjusted LUT
	return Lut

Error:
	// Free the LUT pipeline if an error occurs
	cmsPipelineFree(mm, Lut)
	return nil
}

func cmsReadDevicelinkLUT(mm mem.Manager, hProfile CmsHPROFILE, Intent uint32) *cmsPipeline {
	ContextID := cmsGetProfileContextID(hProfile)

	if Intent > INTENT_ABSOLUTE_COLORIMETRIC {
		return nil
	}

	tag16 := Device2PCS16[Intent]
	tagFloat := Device2PCSFloat[Intent]

	// Handle named color profiles
	if cmsGetDeviceClass(hProfile) == CmsSigNamedColorClass {
		nc, ok := cmsReadTag(mm, hProfile, CmsSigNamedColor2Tag).(*cmsNAMEDCOLORLIST)
		if nc == nil {
			return nil
		}
		if !ok {
			panic("tag is not of the type *cmsNAMEDCOLORLIST\n")

		}

		Lut := cmsPipelineAlloc(mm, ContextID, 0, 0)
		if Lut == nil {
			goto Error
		}

		if !cmsPipelineInsertStage(Lut, CmsAT_BEGIN, cmsStageAllocNamedColor(mm, nc, false)) {
			goto Error
		}

		if CmsGetColorSpace(hProfile) == CmsSigLabData {
			if !cmsPipelineInsertStage(Lut, CmsAT_END, cmsStageAllocLabV2ToV4(mm, ContextID)) {
				goto Error
			}
		}

		return Lut

	Error:
		cmsPipelineFree(mm, Lut)
		return nil
	}

	// Handle floating point LUTs
	if cmsIsTag(hProfile, tagFloat) {
		return cmsReadFloatDevicelinkTag(mm, hProfile, tagFloat)
	}

	tagFloat = Device2PCSFloat[0]
	if cmsIsTag(hProfile, tagFloat) {
		pl, ok := cmsReadTag(mm, hProfile, tagFloat).(*cmsPipeline)
		if pl == nil {
			return nil
		}
		if !ok {
			panic("tag is not of the type *cmsPipeline\n")

		}
		return cmsPipelineDup(mm, pl)
	}

	// Check for 16-bit LUTs
	if !cmsIsTag(hProfile, tag16) {
		tag16 = Device2PCS16[0]
		if !cmsIsTag(hProfile, tag16) {
			return nil
		}
	}

	// Read the tag
	Lut, ok := cmsReadTag(mm, hProfile, tag16).(*cmsPipeline)
	if !ok {
		panic("tag is not of the type *cmsPipeline\n")

	}
	if Lut == nil {
		return nil
	}

	// Duplicate the pipeline as the profile owns the original
	Lut = cmsPipelineDup(mm, Lut)
	if Lut == nil {
		return nil
	}

	// Adjust interpolation for Lab PCS
	if cmsGetPCS(hProfile) == CmsSigLabData {
		ChangeInterpolationToTrilinear(Lut)
	}

	// Check original tag type
	OriginalType := cmsGetTagTrueType(hProfile, tag16)

	// Adjust for Lab16 output
	if OriginalType != CmsSigLut16Type {
		return Lut
	}

	if CmsGetColorSpace(hProfile) == CmsSigLabData {
		if !cmsPipelineInsertStage(Lut, CmsAT_BEGIN, cmsStageAllocLabV4ToV2(mm, ContextID)) {
			goto Error2
		}
	}

	if cmsGetPCS(hProfile) == CmsSigLabData {
		if !cmsPipelineInsertStage(Lut, CmsAT_END, cmsStageAllocLabV2ToV4(mm, ContextID)) {
			goto Error2
		}
	}

	return Lut

Error2:
	cmsPipelineFree(mm, Lut)
	return nil
}

// ReadICCMatrixRGB2XYZ translates the given function
func ReadICCMatrixRGB2XYZ(mm mem.Manager, r *cmsMAT3, hProfile CmsHPROFILE) bool {
	cmsAssert(r != nil, "r cannot be nil") // Equivalent to `_cmsAssert`

	PtrRed, ok := cmsReadTag(mm, hProfile, CmsSigRedColorantTag).(*cmsCIEXYZ)
	if PtrRed == nil {
		return false
	}
	if !ok {
		panic("tag is not of the type *cmsCIEXYZ\n")
	}
	PtrGreen, ok := cmsReadTag(mm, hProfile, CmsSigGreenColorantTag).(*cmsCIEXYZ)
	if PtrGreen == nil {
		return false
	}
	if !ok {
		panic("tag is not of the type *cmsCIEXYZ\n")

	}
	PtrBlue, ok := cmsReadTag(mm, hProfile, CmsSigBlueColorantTag).(*cmsCIEXYZ)
	if PtrBlue == nil {
		return false
	}
	if !ok {
		panic("tag is not of the type *cmsCIEXYZ\n")

	}

	cmsVEC3init(&r.V[0], PtrRed.X, PtrGreen.X, PtrBlue.X)
	cmsVEC3init(&r.V[1], PtrRed.Y, PtrGreen.Y, PtrBlue.Y)
	cmsVEC3init(&r.V[2], PtrRed.Z, PtrGreen.Z, PtrBlue.Z)

	return true
}

// BuildGrayInputMatrixPipeline translates the first function
func BuildGrayInputMatrixPipeline(mm mem.Manager, hProfile CmsHPROFILE) *cmsPipeline {
	ContextID := cmsGetProfileContextID(hProfile)
	GrayTRC, ok := cmsReadTag(mm, hProfile, CmsSigGrayTRCTag).(*CmsToneCurve)
	if GrayTRC == nil {
		return nil
	}
	if !ok {
		panic("tag is not of the type *cmsToneCurve\n")
	}
	Lut := cmsPipelineAlloc(mm, ContextID, 1, 3)
	if Lut == nil {
		goto Error
	}

	if cmsGetPCS(hProfile) == CmsSigLabData {
		Zero := [2]uint16{0x8080, 0x8080}
		EmptyTab := cmsBuildTabulatedToneCurve16(mm, ContextID, 2, Zero[:])
		if EmptyTab == nil {
			goto Error
		}

		LabCurves := [3]*CmsToneCurve{GrayTRC, EmptyTab, EmptyTab}

		if !cmsPipelineInsertStage(Lut, CmsAT_END, cmsStageAllocMatrix(mm, ContextID, 3, 1, OneToThreeInputMatrix, nil)) ||
			!cmsPipelineInsertStage(Lut, CmsAT_END, cmsStageAllocToneCurves(mm, ContextID, 3, LabCurves[:])) {
			CmsFreeToneCurve(EmptyTab)
			goto Error
		}

		CmsFreeToneCurve(EmptyTab)
	} else {
		if !cmsPipelineInsertStage(Lut, CmsAT_END, cmsStageAllocToneCurves(mm, ContextID, 1, []*CmsToneCurve{GrayTRC})) ||
			!cmsPipelineInsertStage(Lut, CmsAT_END, cmsStageAllocMatrix(mm, ContextID, 3, 1, GrayInputMatrix, nil)) {
			goto Error
		}
	}

	return Lut

Error:
	cmsPipelineFree(mm, Lut)
	return nil
}
func BuildRGBInputMatrixShaper(mm mem.Manager, hProfile CmsHPROFILE) *cmsPipeline {
	//fmt.Println("START BuildRGBInputMatrixShaper")

	ContextID := cmsGetProfileContextID(hProfile)
	var Mat cmsMAT3

	if !ReadICCMatrixRGB2XYZ(mm, &Mat, hProfile) {
		return nil
	}

	// Adjust the matrix values
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			Mat.V[i].N[j] *= InpAdj
		}
	}
	rtag, ok1 := cmsReadTag(mm, hProfile, CmsSigRedTRCTag).(*CmsToneCurve)
	grtag, ok2 := cmsReadTag(mm, hProfile, CmsSigGreenTRCTag).(*CmsToneCurve)
	bltag, ok3 := cmsReadTag(mm, hProfile, CmsSigBlueTRCTag).(*CmsToneCurve)

	// Load tone curves
	Shapes := [3]*CmsToneCurve{rtag, grtag, bltag}

	if Shapes[0] == nil || Shapes[1] == nil || Shapes[2] == nil {
		return nil
	}

	if !ok1 || !ok2 || !ok3 {
		panic("tag is not of the type *CmsToneCurve\n")

	}
	// Deep Debug: Print tone curve contents
	/*	for i, shape := range Shapes {
		if shape == nil {
			fmt.Printf("ToneCurve[%d]: nil\n", i)
			continue
		}
		fmt.Printf("ToneCurve[%d]:\n", i)
		fmt.Printf("  nSegments: %d\n", shape.nSegments)
		fmt.Printf("  nEntries:  %d\n", shape.nEntries)

		if len(shape.Table16) > 0 {
			fmt.Printf("  Table16 (len=%d): [", len(shape.Table16))
			limit := len(shape.Table16)
			if limit > 10 {
				limit = 10
			}
			for j := 0; j < limit; j++ {
				fmt.Printf("%d ", shape.Table16[j])
			}
			if len(shape.Table16) > 10 {
				fmt.Print("... ")
			}
			fmt.Println("]")
		}

		if len(shape.Segments) > 0 {
			fmt.Printf("  Segments (len=%d):\n", len(shape.Segments))
			for j, seg := range shape.Segments {
				fmt.Printf("    Segment[%d]: X0=%.6f X1=%.6f Type=%d NGridPoints=%d\n",
					j, seg.X0, seg.X1, seg.Type, seg.NGridPoints)
				if seg.NGridPoints > 0 && len(seg.SampledPoints) > 0 {
					limit := len(seg.SampledPoints)
					if limit > 10 {
						limit = 10
					}
					fmt.Printf("      SampledPoints (len=%d): [", len(seg.SampledPoints))
					for k := 0; k < limit; k++ {
						fmt.Printf("%.6f ", seg.SampledPoints[k])
					}
					if len(seg.SampledPoints) > 10 {
						fmt.Print("... ")
					}
					fmt.Println("]")
				}
			}
		}

		if shape.InterpParams != nil {
			p := shape.InterpParams
			fmt.Printf("  InterpParams:\n")
			fmt.Printf("    nInputs: %d, nOutputs: %d, dwFlags: %d\n", p.nInputs, p.nOutputs, p.dwFlags)
			fmt.Printf("    nSamples: %v\n", p.nSamples[:])
			fmt.Printf("    Domain:   %v\n", p.Domain[:])
		} else {
			fmt.Println("  InterpParams: nil")
		}
	}*/

	// Build pipeline
	Lut := cmsPipelineAlloc(mm, ContextID, 3, 3)
	if Lut != nil {
		if !cmsPipelineInsertStage(Lut, CmsAT_END, cmsStageAllocToneCurves(mm, ContextID, 3, Shapes[:])) ||
			!cmsPipelineInsertStage(Lut, CmsAT_END, cmsStageAllocMatrix(mm, ContextID, 3, 3, MatToSlice(Mat), nil)) {
			goto Error
		}

		if cmsGetPCS(hProfile) == CmsSigLabData {
			if !cmsPipelineInsertStage(Lut, CmsAT_END, cmsStageAllocXYZ2Lab(mm, ContextID)) {
				goto Error
			}
		}
	}
	//fmt.Println("END BuildRGBInputMatrixShaper")
	return Lut

Error:
	cmsPipelineFree(mm, Lut)
	return nil
}

// cmsReadFloatInputTag translates the first function
func cmsReadFloatInputTag(mm mem.Manager, hProfile CmsHPROFILE, tagFloat cmsTagSignature) *cmsPipeline {
	ContextID := cmsGetProfileContextID(hProfile)
	pl, ok := cmsReadTag(mm, hProfile, tagFloat).(*cmsPipeline)
	if pl == nil {
		return nil
	}
	if !ok {
		panic("tag is not of the type *cmsPipeline\n")

	}
	Lut := cmsPipelineDup(mm, pl)
	spc := CmsGetColorSpace(hProfile)
	PCS := cmsGetPCS(hProfile)

	if Lut == nil {
		return nil
	}

	if spc == CmsSigLabData {
		if !cmsPipelineInsertStage(Lut, CmsAT_BEGIN, cmsStageNormalizeToLabFloat(mm, ContextID)) {
			goto Error
		}
	} else if spc == CmsSigXYZData {
		if !cmsPipelineInsertStage(Lut, CmsAT_BEGIN, cmsStageNormalizeToXyzFloat(mm, ContextID)) {
			goto Error
		}
	}

	if PCS == CmsSigLabData {
		if !cmsPipelineInsertStage(Lut, CmsAT_END, cmsStageNormalizeFromLabFloat(mm, ContextID)) {
			goto Error
		}
	} else if PCS == CmsSigXYZData {
		if !cmsPipelineInsertStage(Lut, CmsAT_END, cmsStageNormalizeFromXyzFloat(mm, ContextID)) {
			goto Error
		}
	}

	return Lut

Error:
	cmsPipelineFree(mm, Lut)
	return nil
}

// cmsReadInputLUT translates the second function
func cmsReadInputLUT(mm mem.Manager, hProfile CmsHPROFILE, Intent uint32) *cmsPipeline {
	ContextID := cmsGetProfileContextID(hProfile)

	if cmsGetDeviceClass(hProfile) == CmsSigNamedColorClass {
		nc, ok := cmsReadTag(mm, hProfile, CmsSigNamedColor2Tag).(*cmsNAMEDCOLORLIST)
		if nc == nil {
			return nil
		}
		if !ok {
			cmsSignalError(nil, cmsERROR_UNDEFINED, "Interface data assertion error, not *cmsNAMEDCOLORLIST\n")
			return nil
		}
		Lut := cmsPipelineAlloc(mm, ContextID, 0, 0)
		if Lut == nil {
			return nil
		}

		if !cmsPipelineInsertStage(Lut, CmsAT_BEGIN, cmsStageAllocNamedColor(mm, nc, true)) ||
			!cmsPipelineInsertStage(Lut, CmsAT_END, cmsStageAllocLabV2ToV4(mm, ContextID)) {
			cmsPipelineFree(mm, Lut)
			return nil
		}
		return Lut
	}

	// This is an attempt to reuse this function to retrieve the matrix-shaper as pipeline no
	// matter other LUT are present and have precedence. Intent = 0xffffffff can be used for that.
	if Intent <= INTENT_ABSOLUTE_COLORIMETRIC {
		tag16 := Device2PCS16[Intent]
		tagFloat := Device2PCSFloat[Intent]

		// Floating point LUT are always V4, but the encoding range is no
		// longer 0..1.0, so we need to add an stage depending on the color space
		if cmsIsTag(hProfile, tagFloat) {
			return cmsReadFloatInputTag(mm, hProfile, tagFloat)
		}

		if !cmsIsTag(hProfile, tag16) {
			tag16 = Device2PCS16[0]
		}

		if cmsIsTag(hProfile, tag16) {
			Lut, ok := cmsReadTag(mm, hProfile, tag16).(*cmsPipeline)
			if Lut == nil {
				return nil
			}
			if !ok {
				cmsSignalError(nil, cmsERROR_UNDEFINED, "Interface data assertion error, not *cmsPipeline\n")
				return nil
			}

			OriginalType := cmsGetTagTrueType(hProfile, tag16)
			Lut = cmsPipelineDup(mm, Lut)

			if OriginalType != CmsSigLut16Type || cmsGetPCS(hProfile) != CmsSigLabData {
				return Lut
			}

			if CmsGetColorSpace(hProfile) == CmsSigLabData &&
				!cmsPipelineInsertStage(Lut, CmsAT_BEGIN, cmsStageAllocLabV4ToV2(mm, ContextID)) {
				cmsPipelineFree(mm, Lut)
				return nil
			}

			if !cmsPipelineInsertStage(Lut, CmsAT_END, cmsStageAllocLabV2ToV4(mm, ContextID)) {
				cmsPipelineFree(mm, Lut)
				return nil
			}
			return Lut
		}
	}

	if CmsGetColorSpace(hProfile) == CmsSigGrayData {
		return BuildGrayInputMatrixPipeline(mm, hProfile)
	}
	return BuildRGBInputMatrixShaper(mm, hProfile)
}

// ---------------------------------------------------------------------------------------------------------------

// Gray output pipeline.
// XYZ. Gray or Lab. Gray. Since we only know the GrayTRC, we need to do some assumptions. Gray component will be
// given by Y on XYZ PCS and by L* on Lab PCS, Both across inverse TRC curve.
// The complete pipeline on XYZ is Matrix[3:1]. Tone curve and in Lab Matrix[3:1]. Tone Curve as well.

func BuildGrayOutputPipeline(mm mem.Manager, hProfile CmsHPROFILE) *cmsPipeline {
	ContextID := cmsGetProfileContextID(hProfile)
	GrayTRC, ok := cmsReadTag(mm, hProfile, CmsSigGrayTRCTag).(*CmsToneCurve)
	if GrayTRC == nil {
		return nil
	}

	if !ok {
		panic("tag is not of the type *CmsToneCurve\n")
	}

	RevGrayTRC := cmsReverseToneCurve(mm, GrayTRC)
	if RevGrayTRC == nil {
		return nil
	}

	Lut := cmsPipelineAlloc(mm, ContextID, 3, 1)
	if Lut == nil {
		CmsFreeToneCurve(RevGrayTRC)
		return nil
	}

	if cmsGetPCS(hProfile) == CmsSigLabData {
		if !cmsPipelineInsertStage(Lut, CmsAT_END, cmsStageAllocMatrix(mm, ContextID, 1, 3, PickLstarMatrix, nil)) {
			CmsFreeToneCurve(RevGrayTRC)
			cmsPipelineFree(mm, Lut)
			return nil
		}
	} else {
		if !cmsPipelineInsertStage(Lut, CmsAT_END, cmsStageAllocMatrix(mm, ContextID, 1, 3, PickYMatrix, nil)) {
			CmsFreeToneCurve(RevGrayTRC)
			cmsPipelineFree(mm, Lut)
			return nil
		}
	}

	if !cmsPipelineInsertStage(Lut, CmsAT_END, cmsStageAllocToneCurves(mm, ContextID, 1, []*CmsToneCurve{RevGrayTRC})) {
		CmsFreeToneCurve(RevGrayTRC)
		cmsPipelineFree(mm, Lut)
		return nil
	}

	CmsFreeToneCurve(RevGrayTRC)
	return Lut
}

// BuildRGBOutputMatrixShaper translates the given function
func BuildRGBOutputMatrixShaper(mm mem.Manager, hProfile CmsHPROFILE) *cmsPipeline {
	//fmt.Println("BuildRGBOutputMatrixShaper")
	ContextID := cmsGetProfileContextID(hProfile)
	var Mat, Inv cmsMAT3
	var Shapes, InvShapes [3]*CmsToneCurve

	if !ReadICCMatrixRGB2XYZ(mm, &Mat, hProfile) {
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
	rtag, ok1 := cmsReadTag(mm, hProfile, CmsSigRedTRCTag).(*CmsToneCurve)
	grtag, ok2 := cmsReadTag(mm, hProfile, CmsSigGreenTRCTag).(*CmsToneCurve)
	bltag, ok3 := cmsReadTag(mm, hProfile, CmsSigBlueTRCTag).(*CmsToneCurve)

	// Load tone curves
	Shapes = [3]*CmsToneCurve{rtag, grtag, bltag}

	if Shapes[0] == nil || Shapes[1] == nil || Shapes[2] == nil {
		return nil
	}

	if !ok1 || !ok2 || !ok3 {
		panic("tag is not of the type *CmsToneCurve\n")
	}

	InvShapes[0] = cmsReverseToneCurve(mm, Shapes[0])
	InvShapes[1] = cmsReverseToneCurve(mm, Shapes[1])
	InvShapes[2] = cmsReverseToneCurve(mm, Shapes[2])

	if InvShapes[0] == nil || InvShapes[1] == nil || InvShapes[2] == nil {
		return nil
	}

	Lut := cmsPipelineAlloc(mm, ContextID, 3, 3)
	if Lut != nil {
		// Handle profiles with Lab PCS
		if cmsGetPCS(hProfile) == CmsSigLabData {
			if !cmsPipelineInsertStage(Lut, CmsAT_END, cmsStageAllocLab2XYZ(mm, ContextID)) {
				goto Error
			}
		}

		if !cmsPipelineInsertStage(Lut, CmsAT_END, cmsStageAllocMatrix(mm, ContextID, 3, 3, MatToSlice(Inv), nil)) ||
			!cmsPipelineInsertStage(Lut, CmsAT_END, cmsStageAllocToneCurves(mm, ContextID, 3, InvShapes[:])) {
			goto Error
		}
	}

	cmsFreeToneCurveTriple(InvShapes)
	return Lut

Error:
	cmsFreeToneCurveTriple(InvShapes)
	cmsPipelineFree(mm, Lut)
	return nil
}

func ChangeInterpolationToTrilinear(Lut *cmsPipeline) {
	//	fmt.Println("ChangeInterpolationToTrilinear")
	for Stage := cmsPipelineGetPtrToFirstStage(Lut); Stage != nil; Stage = cmsStageNext(Stage) {
		if cmsStageType(Stage) == CmsSigCLutElemType {
			CLUT := Stage.Data.(*cmsStageCLutData)
			CLUT.Params.dwFlags |= CMS_LERP_FLAGS_TRILINEAR
			cmsSetInterpolationRoutine(Lut.ContextID, CLUT.Params)
		}
	}
}

// _cmsReadFloatOutputTag translates the given function
func cmsReadFloatOutputTag(mm mem.Manager, hProfile CmsHPROFILE, tagFloat cmsTagSignature) *cmsPipeline {
	ContextID := cmsGetProfileContextID(hProfile)
	pl, ok := cmsReadTag(mm, hProfile, tagFloat).(*cmsPipeline)

	if pl == nil {
		return nil
	}
	if !ok {
		panic("tag is not of the type *CmsToneCurve\n")
	}
	Lut := cmsPipelineDup(mm, pl)

	if Lut == nil {
		return nil
	}
	PCS := cmsGetPCS(hProfile)
	dataSpace := CmsGetColorSpace(hProfile)

	// If PCS is Lab or XYZ, adjust normalization at the beginning of the pipeline
	if PCS == CmsSigLabData {
		if !cmsPipelineInsertStage(Lut, CmsAT_BEGIN, cmsStageNormalizeToLabFloat(mm, ContextID)) {
			goto Error
		}
	} else if PCS == CmsSigXYZData {
		if !cmsPipelineInsertStage(Lut, CmsAT_BEGIN, cmsStageNormalizeToXyzFloat(mm, ContextID)) {
			goto Error
		}
	}

	// If the output is Lab or XYZ, normalization is needed at the end of the pipeline
	if dataSpace == CmsSigLabData {
		if !cmsPipelineInsertStage(Lut, CmsAT_END, cmsStageNormalizeFromLabFloat(mm, ContextID)) {
			goto Error
		}
	} else if dataSpace == CmsSigXYZData {
		if !cmsPipelineInsertStage(Lut, CmsAT_END, cmsStageNormalizeFromXyzFloat(mm, ContextID)) {
			goto Error
		}
	}

	return Lut

Error:
	cmsPipelineFree(mm, Lut)
	return nil
}

func cmsReadOutputLUT(mm mem.Manager, hProfile CmsHPROFILE, Intent uint32) *cmsPipeline {
	//	fmt.Println("cmsReadOutputLUT")
	ContextID := cmsGetProfileContextID(hProfile)

	if Intent <= INTENT_ABSOLUTE_COLORIMETRIC {
		tag16 := PCS2Device16[Intent]
		tagFloat := PCS2DeviceFloat[Intent]

		if cmsIsTag(hProfile, tagFloat) {
			return cmsReadFloatOutputTag(mm, hProfile, tagFloat)
		}

		if !cmsIsTag(hProfile, tag16) {
			tag16 = PCS2Device16[0]
		}

		if cmsIsTag(hProfile, tag16) {
			Lut, ok := cmsReadTag(mm, hProfile, tag16).(*cmsPipeline)
			if Lut == nil {
				return nil
			}
			if !ok {
				panic("tag is not of the type *CmsToneCurve\n")
			}

			OriginalType := cmsGetTagTrueType(hProfile, tag16)
			Lut = cmsPipelineDup(mm, Lut)

			if cmsGetPCS(hProfile) == CmsSigLabData {
				ChangeInterpolationToTrilinear(Lut)
			}

			if OriginalType != CmsSigLut16Type || cmsGetPCS(hProfile) != CmsSigLabData {
				return Lut
			}

			if !cmsPipelineInsertStage(Lut, CmsAT_BEGIN, cmsStageAllocLabV4ToV2(mm, ContextID)) {
				cmsPipelineFree(mm, Lut)
				return nil
			}

			if CmsGetColorSpace(hProfile) == CmsSigLabData &&
				!cmsPipelineInsertStage(Lut, CmsAT_END, cmsStageAllocLabV2ToV4(mm, ContextID)) {
				cmsPipelineFree(mm, Lut)
				return nil
			}

			return Lut
		}
	}

	if CmsGetColorSpace(hProfile) == CmsSigGrayData {
		return BuildGrayOutputPipeline(mm, hProfile)
	}

	return BuildRGBOutputMatrixShaper(mm, hProfile)
}
func cmsIsMatrixShaper(hProfile CmsHPROFILE) bool {
	switch CmsGetColorSpace(hProfile) {

	case CmsSigGrayData:
		return cmsIsTag(hProfile, CmsSigGrayTRCTag)

	case CmsSigRgbData:
		return cmsIsTag(hProfile, CmsSigRedColorantTag) &&
			cmsIsTag(hProfile, CmsSigGreenColorantTag) &&
			cmsIsTag(hProfile, CmsSigBlueColorantTag) &&
			cmsIsTag(hProfile, CmsSigRedTRCTag) &&
			cmsIsTag(hProfile, CmsSigGreenTRCTag) &&
			cmsIsTag(hProfile, CmsSigBlueTRCTag)

	default:
		return false
	}
}
func cmsIsCLUT(hProfile CmsHPROFILE, Intent uint32, UsedDirection uint32) bool {
	var TagTable []cmsTagSignature

	// For devicelinks, the supported intent is the one stated in the header
	if cmsGetDeviceClass(hProfile) == CmsSigLinkClass {
		return cmsGetHeaderRenderingIntent(hProfile) == Intent
	}

	switch UsedDirection {

	case LCMS_USED_AS_INPUT:
		TagTable = Device2PCS16

	case LCMS_USED_AS_OUTPUT:
		TagTable = PCS2Device16

	case LCMS_USED_AS_PROOF:
		return cmsIsIntentSupported(hProfile, Intent, LCMS_USED_AS_INPUT) &&
			cmsIsIntentSupported(hProfile, INTENT_RELATIVE_COLORIMETRIC, LCMS_USED_AS_OUTPUT)

	default:
		cmsSignalError(cmsGetProfileContextID(hProfile), cmsERROR_RANGE, "Unexpected direction ")
		return false
	}

	// Extended intents are not strictly CLUT-based
	if Intent > INTENT_ABSOLUTE_COLORIMETRIC {
		return false
	}
	// Use unsafe to index into TagTable
	return cmsIsTag(hProfile, TagTable[Intent])

}

func cmsIsIntentSupported(hProfile CmsHPROFILE, Intent uint32, UsedDirection uint32) bool {
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
func cmsReadProfileSequence(mm mem.Manager, hProfile CmsHPROFILE) *cmsSEQ {
	var ProfileSeq, ProfileId, NewSeq *cmsSEQ

	// Take profile sequence description first
	ProfileSeq, ok1 := cmsReadTag(mm, hProfile, CmsSigProfileSequenceDescTag).(*cmsSEQ)

	// Take profile sequence ID
	ProfileId, ok2 := cmsReadTag(mm, hProfile, CmsSigProfileSequenceIdTag).(*cmsSEQ)

	// Handle cases where either or both are NULL
	if ProfileSeq == nil && ProfileId == nil {
		return nil
	}
	if ProfileSeq == nil {
		return cmsDupProfileSequenceDescription(mm, ProfileId)
	}
	if ProfileId == nil {
		return cmsDupProfileSequenceDescription(mm, ProfileSeq)
	}
	if !ok1 || !ok2 {
		panic("tag is not of the type *cmsSEQ\n")

	}

	// Check if sequence lengths match; otherwise, duplicate the description sequence
	if ProfileSeq.n != ProfileId.n {
		return cmsDupProfileSequenceDescription(mm, ProfileSeq)
	}

	// Duplicate the profile sequence description
	NewSeq = cmsDupProfileSequenceDescription(mm, ProfileSeq)

	// Mix profile sequence ID into the new sequence
	// Ok, proceed to the mixing
	if NewSeq != nil {
		for i := uint32(0); i < ProfileSeq.n; i++ {
			copy(NewSeq.seq[i].ProfileID[:], ProfileId.seq[i].ProfileID[:])
			NewSeq.seq[i].Description = cmsMLUdup(mm, ProfileId.seq[i].Description)
		}
	}

	return NewSeq
}

// cmsWriteProfileSequence dumps the contents of the profile sequence in both tags (if v4 is available).
func cmsWriteProfileSequence(mm mem.Manager, hProfile CmsHPROFILE, seq *cmsSEQ) bool {
	// Write the profile sequence description tag
	if !cmsWriteTag(mm, hProfile, CmsSigProfileSequenceDescTag, seq) {
		return false
	}

	// If the profile is version 4 or later, write the profile sequence ID tag
	if cmsGetEncodedICCversion(hProfile) >= 0x4000000 {
		if !cmsWriteTag(mm, hProfile, CmsSigProfileSequenceIdTag, seq) {
			return false
		}
	}

	return true
}

// GetMLUFromProfile reads and duplicates an MLU tag from the profile if found.
func GetMLUFromProfile(mm mem.Manager, h CmsHPROFILE, sig cmsTagSignature) *cmsMLU {
	mlu, ok := cmsReadTag(mm, h, sig).(*cmsMLU)
	if mlu == nil {
		return nil
	}
	if !ok {
		cmsSignalError(nil, cmsERROR_UNDEFINED, "Interface data assertion error, not *cmsMLU\n")
		return nil
	}

	return cmsMLUdup(mm, mlu)
}

func cmsCompileProfileSequence(mm mem.Manager, ContextID CmsContext, nProfiles uint32, hProfiles []CmsHPROFILE) *cmsSEQ {
	// Allocate a profile sequence description
	seq := cmsAllocProfileSequenceDescription(mm, ContextID, nProfiles)
	if seq == nil {
		return nil
	}

	// Iterate through profiles and populate the sequence
	for i := uint32(0); i < nProfiles; i++ {
		ps := &seq.seq[i] // Reference to the current profile sequence descriptor
		h := hProfiles[i] // Current profile

		// Extract header attributes
		cmsGetHeaderAttributes(h, &ps.attributes)
		//	cmsGetHeaderProfileID(h, &ps.ProfileID.ID8[0])
		cmsGetHeaderProfileID(h, ps.ProfileID[:]) //instead of union in C
		ps.deviceMfg = cmsSignature(cmsGetHeaderManufacturer(h))
		ps.deviceModel = cmsSignature(cmsGetHeaderModel(h))

		// Retrieve technology tag
		techpt, ok := cmsReadTag(mm, h, CmsSigTechnologyTag).(*cmsTechnologySignature)
		if techpt == nil {
			ps.technology = cmsTechnologySignature(0)
		} else if !ok {
			cmsSignalError(nil, cmsERROR_UNDEFINED, "Interface data assertion error, not *cmsTechnologySignature\n")
			return nil
		} else {
			ps.technology = *techpt
		}

		// Retrieve MLU tags
		ps.Manufacturer = GetMLUFromProfile(mm, h, CmsSigDeviceMfgDescTag)
		ps.Model = GetMLUFromProfile(mm, h, CmsSigDeviceModelDescTag)
		ps.Description = GetMLUFromProfile(mm, h, CmsSigProfileDescriptionTag)
	}

	return seq
}
func GetInfo(mm mem.Manager, hProfile CmsHPROFILE, Info CmsInfoType) *cmsMLU {
	//	fmt.Println("GetInfo for info ", Info)
	var sig cmsTagSignature

	switch Info {
	case cmsInfoDescription:
		sig = CmsSigProfileDescriptionTag
	case cmsInfoManufacturer:
		sig = CmsSigDeviceMfgDescTag
	case cmsInfoModel:
		sig = CmsSigDeviceModelDescTag
	case cmsInfoCopyright:
		sig = CmsSigCopyrightTag
	default:
		return nil
	}
	mlu, ok := cmsReadTag(mm, hProfile, sig).(*cmsMLU)
	if mlu == nil {
		return nil
	}
	if !ok {
		cmsSignalError(nil, cmsERROR_UNDEFINED, "Interface data assertion error, not *cmsMLU\n")
		return nil
	}
	return mlu
}
func cmsGetProfileInfo(mm mem.Manager, hProfile CmsHPROFILE, Info CmsInfoType,
	LanguageCode string, CountryCode string,
	Buffer []uint16, BufferSize uint32) uint32 {

	mlu := GetInfo(mm, hProfile, Info)
	if mlu == nil {
		return 0
	}

	return cmsMLUgetWide(mlu, LanguageCode, CountryCode, Buffer, BufferSize)
}

func CmsGetProfileInfoASCII(mm mem.Manager, hProfile CmsHPROFILE, Info CmsInfoType,
	LanguageCode string, CountryCode string,
	Buffer []byte, BufferSize uint32) uint32 {
	//	fmt.Println("start CmsGetProfileInfoASCII info type ", Info)
	mlu := GetInfo(mm, hProfile, Info)
	if mlu == nil {
		return 0
	}
	//	fmt.Println("got mlu and mlu.Entries[0].Len ", mlu, mlu.Entries[0].Len)
	// Call the corresponding function to get ASCII info
	return cmsMLUgetASCII(mlu, LanguageCode, CountryCode, Buffer, BufferSize)
}
