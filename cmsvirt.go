package golcms

//"unsafe"
import "arena"

func SetTextTags(ar *arena.Arena, hProfile CmsHPROFILE, Description []uint16) bool {
	var DescriptionMLU, CopyrightMLU *cmsMLU
	var rc bool
	ContextID := cmsGetProfileContextID(hProfile)

	DescriptionMLU = cmsMLUalloc(ar, ContextID, 1)
	CopyrightMLU = cmsMLUalloc(ar, ContextID, 1)

	if DescriptionMLU == nil || CopyrightMLU == nil {
		goto Error
	}

	if !cmsMLUsetWide(DescriptionMLU, "en", "US", Description) {
		goto Error
	}
	if !cmsMLUsetWide(CopyrightMLU, "en", "US", StringToUTF16Slice("No copyright, use freely")) {
		goto Error
	}

	if !cmsWriteTag(ar, hProfile, CmsSigProfileDescriptionTag, DescriptionMLU) {
		goto Error
	}
	if !cmsWriteTag(ar, hProfile, CmsSigCopyrightTag, CopyrightMLU) {
		goto Error
	}

	rc = true

Error:
	if DescriptionMLU != nil {
		cmsMLUfree(DescriptionMLU)
	}
	if CopyrightMLU != nil {
		cmsMLUfree(CopyrightMLU)
	}
	return rc
}
func SetSeqDescTag(ar *arena.Arena, hProfile CmsHPROFILE, Model []byte) bool {
	var rc bool
	ContextID := cmsGetProfileContextID(hProfile)
	Seq := cmsAllocProfileSequenceDescription(ar, ContextID, 1)

	if Seq == nil {
		return false
	}

	// Initialize fields in Seq
	Seq.seq[0].deviceMfg = 0
	Seq.seq[0].deviceModel = 0

	// Set attributes based on conditional compilation
	Seq.seq[0].attributes = 0

	Seq.seq[0].technology = 0

	// Set Manufacturer and Model text
	cmsMLUsetASCII(Seq.seq[0].Manufacturer, cmsNoLanguage, cmsNoCountry, "Little CMS")
	cmsMLUsetASCII(Seq.seq[0].Model, cmsNoLanguage, cmsNoCountry, string(Model))

	// Write the sequence description
	if !cmsWriteProfileSequence(ar, hProfile, Seq) {
		goto Error
	}

	rc = true

Error:
	cmsFreeProfileSequenceDescription(Seq)
	return rc
}

func CmsCreateRGBProfileTHR(ar *arena.Arena, ContextID CmsContext, WhitePoint *CmsCIExyY, Primaries *CmsCIExyYTRIPLE, TransferFunction []*CmsToneCurve) CmsHPROFILE {
	var (
		hICC          CmsHPROFILE
		MColorants    cmsMAT3
		Colorants     cmsCIEXYZTRIPLE
		MaxWhite      CmsCIExyY
		CHAD          cmsMAT3
		WhitePointXYZ cmsCIEXYZ
	)

	hICC = cmsCreateProfilePlaceholder(ar, ContextID)
	if hICC == nil {
		return nil // can't allocate
	}

	cmsSetProfileVersion(hICC, 4.4)
	cmsSetDeviceClass(hICC, CmsSigDisplayClass)
	cmsSetColorSpace(hICC, CmsSigRgbData)
	cmsSetPCS(hICC, CmsSigXYZData)
	cmsSetHeaderRenderingIntent(hICC, INTENT_PERCEPTUAL)

	// Implement profile using following tags:
	//
	//  1 CmsSigProfileDescriptionTag
	//  2 CmsSigMediaWhitePointTag
	//  3 CmsSigRedColorantTag
	//  4 CmsSigGreenColorantTag
	//  5 CmsSigBlueColorantTag
	//  6 CmsSigRedTRCTag
	//  7 CmsSigGreenTRCTag
	//  8 CmsSigBlueTRCTag
	//  9 Chromatic adaptation Tag
	// This conforms a standard RGB DisplayProfile as says ICC, and then I add (As per addendum II)
	// 10 CmsSigChromaticityTag

	if !SetTextTags(ar, hICC, StringToUTF16Slice("RGB built-in")) {
		goto Error
	}

	if WhitePoint != nil {
		if !cmsWriteTag(ar, hICC, CmsSigMediaWhitePointTag, cmsD50_XYZ()) {
			goto Error
		}

		cmsxyY2XYZ(&WhitePointXYZ, WhitePoint)
		cmsAdaptationMatrix(&CHAD, nil, &WhitePointXYZ, cmsD50_XYZ())

		if !cmsWriteTag(ar, hICC, CmsSigChromaticAdaptationTag, &CHAD) {
			goto Error
		}
	}

	if WhitePoint != nil && Primaries != nil {
		MaxWhite.X_small = WhitePoint.X_small
		MaxWhite.Y_small = WhitePoint.Y_small
		MaxWhite.Y_large = 1.0

		if !cmsBuildRGB2XYZtransferMatrix(&MColorants, &MaxWhite, Primaries) {
			goto Error
		}

		Colorants.Red.X = MColorants.V[0].N[0]
		Colorants.Red.Y = MColorants.V[1].N[0]
		Colorants.Red.Z = MColorants.V[2].N[0]

		Colorants.Green.X = MColorants.V[0].N[1]
		Colorants.Green.Y = MColorants.V[1].N[1]
		Colorants.Green.Z = MColorants.V[2].N[1]

		Colorants.Blue.X = MColorants.V[0].N[2]
		Colorants.Blue.Y = MColorants.V[1].N[2]
		Colorants.Blue.Z = MColorants.V[2].N[2]

		if !cmsWriteTag(ar, hICC, CmsSigRedColorantTag, &Colorants.Red) ||
			!cmsWriteTag(ar, hICC, CmsSigGreenColorantTag, &Colorants.Green) ||
			!cmsWriteTag(ar, hICC, CmsSigBlueColorantTag, &Colorants.Blue) {
			goto Error
		}
	}

	if TransferFunction != nil {
		if !cmsWriteTag(ar, hICC, CmsSigRedTRCTag, TransferFunction[0]) {
			goto Error
		}

		if TransferFunction[1] == TransferFunction[0] {
			if !cmsLinkTag(ar, hICC, CmsSigGreenTRCTag, CmsSigRedTRCTag) {
				goto Error
			}
		} else {
			if !cmsWriteTag(ar, hICC, CmsSigGreenTRCTag, TransferFunction[1]) {
				goto Error
			}
		}

		if TransferFunction[2] == TransferFunction[0] {
			if !cmsLinkTag(ar, hICC, CmsSigBlueTRCTag, CmsSigRedTRCTag) {
				goto Error
			}
		} else {
			if !cmsWriteTag(ar, hICC, CmsSigBlueTRCTag, TransferFunction[2]) {
				goto Error
			}
		}
	}

	if Primaries != nil {
		if !cmsWriteTag(ar, hICC, CmsSigChromaticityTag, Primaries) {
			goto Error
		}
	}

	return hICC

Error:
	CmsCloseProfile(ar, hICC)
	return nil
}

func CmsCreateRGBProfile(ar *arena.Arena, WhitePoint *CmsCIExyY, Primaries *CmsCIExyYTRIPLE, TransferFunction []*CmsToneCurve) CmsHPROFILE {
	return CmsCreateRGBProfileTHR(ar, nil, WhitePoint, Primaries, TransferFunction)
}

func cmsCreateGrayProfileTHR(ar *arena.Arena, ContextID CmsContext, WhitePoint *CmsCIExyY, TransferFunction *CmsToneCurve) CmsHPROFILE {
	var tmp cmsCIEXYZ
	hICC := cmsCreateProfilePlaceholder(ar, ContextID)
	if hICC == nil {
		return nil
	}

	cmsSetProfileVersion(hICC, 4.4)
	cmsSetDeviceClass(hICC, CmsSigDisplayClass)
	cmsSetColorSpace(hICC, CmsSigGrayData)
	cmsSetPCS(hICC, CmsSigXYZData)
	cmsSetHeaderRenderingIntent(hICC, INTENT_PERCEPTUAL)

	if !SetTextTags(ar, hICC, StringToUTF16Slice("gray built-in")) {
		goto Error
	}

	if WhitePoint != nil {
		cmsxyY2XYZ(&tmp, WhitePoint)
		if !cmsWriteTag(ar, hICC, CmsSigMediaWhitePointTag, &tmp) {
			goto Error
		}
	}

	if TransferFunction != nil {
		if !cmsWriteTag(ar, hICC, CmsSigGrayTRCTag, TransferFunction) {
			goto Error
		}
	}

	return hICC

Error:
		CmsCloseProfile(ar, hICC)
	return nil
}

func CmsCreateGrayProfile(ar *arena.Arena, WhitePoint *CmsCIExyY, TransferFunction *CmsToneCurve) CmsHPROFILE {
	return cmsCreateGrayProfileTHR(ar, nil, WhitePoint, TransferFunction)
}

//lint:ignore U1000 kept for parity with lcms; used in future ports
func cmsCreateLinearizationDeviceLinkTHR(ar *arena.Arena, ContextID CmsContext, ColorSpace cmsColorSpaceSignature, TransferFunctions []*CmsToneCurve) CmsHPROFILE {
	hICC := cmsCreateProfilePlaceholder(ar, ContextID)
	if hICC == nil {
		return nil
	}

	cmsSetProfileVersion(hICC, 4.4)
	cmsSetDeviceClass(hICC, CmsSigLinkClass)
	cmsSetColorSpace(hICC, ColorSpace)
	cmsSetPCS(hICC, ColorSpace)
	cmsSetHeaderRenderingIntent(hICC, INTENT_PERCEPTUAL)

	nChannels := cmsChannelsOfColorSpace(ColorSpace)

	Pipeline := cmsPipelineAlloc(ar, ContextID, uint32(nChannels), uint32(nChannels))
	if Pipeline == nil {
		goto Error
	}

	if !cmsPipelineInsertStage(Pipeline, CmsAT_BEGIN, cmsStageAllocToneCurves(ar, ContextID, uint32(nChannels), TransferFunctions)) {
		goto Error
	}

	if !SetTextTags(ar, hICC, StringToUTF16Slice("Linearization built-in")) ||
		!cmsWriteTag(ar, hICC, CmsSigAToB0Tag, Pipeline) ||
		!SetSeqDescTag(ar, hICC, []byte("Linearization built-in")) {
		goto Error
	}

	cmsPipelineFree(ar, Pipeline)
	return hICC

Error:
	cmsPipelineFree(ar, Pipeline)
	CmsCloseProfile(ar, hICC)
	return nil
}

func cmsCreateLinearizationDeviceLink(ar *arena.Arena, ColorSpace cmsColorSpaceSignature, TransferFunctions []*CmsToneCurve) CmsHPROFILE {
	return cmsCreateLinearizationDeviceLinkTHR(ar, nil, ColorSpace, TransferFunctions)
}

// Ink-limiting algorithm
//
//  Sum = C + M + Y + K
//  If Sum > InkLimit
//        Ratio= 1 - (Sum - InkLimit) / (C + M + Y)
//        if Ratio <0
//              Ratio=0
//        endif
//     Else
//         Ratio=1
//     endif
//
//     C = Ratio * C
//     M = Ratio * M
//     Y = Ratio * Y
//     K: Does not change

// InkLimitingSampler translates the given function
func InkLimitingSampler(ar *arena.Arena, In []uint16, Out []uint16, cargo interface{}) int32 {
	inkLimit, ok := cargo.(float64)
	if !ok {
		cmsSignalError(nil, cmsERROR_RANGE, "Expected cargo to be float64")
		return 0
	}

	var sumCMY, sumCMYK, ratio float64

	// Convert InkLimit to 0-65535 scale
	inkLimit *= 655.35

	sumCMY = float64(In[0]) + float64(In[1]) + float64(In[2])
	sumCMYK = sumCMY + float64(In[3])

	if sumCMYK > inkLimit {
		ratio = 1 - ((sumCMYK - inkLimit) / sumCMY)
		if ratio < 0 {
			ratio = 0
		}
	} else {
		ratio = 1
	}

	Out[0] = cmsQuickSaturateWord(float64(In[0]) * ratio) // C
	Out[1] = cmsQuickSaturateWord(float64(In[1]) * ratio) // M
	Out[2] = cmsQuickSaturateWord(float64(In[2]) * ratio) // Y
	Out[3] = In[3]                                        // K unchanged

	return 1
}
//lint:ignore U1000 kept for parity with lcms; used in future ports
func cmsCreateInkLimitingDeviceLinkTHR(ar *arena.Arena, ContextID CmsContext, ColorSpace cmsColorSpaceSignature, Limit float64) CmsHPROFILE {
	var hICC CmsHPROFILE
	var LUT *cmsPipeline
	var CLUT *cmsStage
	var nChannels int32

	if ColorSpace != CmsSigCmykData {
		cmsSignalError(ContextID, cmsERROR_COLORSPACE_CHECK, "InkLimiting: Only CMYK currently supported")
		return nil
	}

	if Limit < 0.0 || Limit > 400 {
		cmsSignalError(ContextID, cmsERROR_RANGE, "InkLimiting: Limit should be between 0..400")
		if Limit < 0 {
			Limit = 0
		}
		if Limit > 400 {
			Limit = 400
		}
	}

	hICC = cmsCreateProfilePlaceholder(ar, ContextID)
	if hICC == nil {
		return nil
	}

	cmsSetProfileVersion(hICC, 4.4)
	cmsSetDeviceClass(hICC, CmsSigLinkClass)
	cmsSetColorSpace(hICC, ColorSpace)
	cmsSetPCS(hICC, ColorSpace)
	cmsSetHeaderRenderingIntent(hICC, INTENT_PERCEPTUAL)

	LUT = cmsPipelineAlloc(ar, ContextID, 4, 4)
	if LUT == nil {
		goto Error
	}

	nChannels = int32(cmsChannelsOf(ColorSpace))

	CLUT = cmsStageAllocCLut16bit(ar, ContextID, 17, uint32(nChannels), uint32(nChannels), nil)
	if CLUT == nil {
		goto Error
	}

	if !cmsStageSampleCLut16bit(ar, CLUT, InkLimitingSampler, &Limit, 0) {
		goto Error
	}

	if !cmsPipelineInsertStage(LUT, CmsAT_BEGIN, cmsStageAllocIdentityCurves(ar, ContextID, uint32(nChannels))) ||
		!cmsPipelineInsertStage(LUT, CmsAT_END, CLUT) ||
		!cmsPipelineInsertStage(LUT, CmsAT_END, cmsStageAllocIdentityCurves(ar, ContextID, uint32(nChannels))) {
		goto Error
	}

	if !SetTextTags(ar, hICC, StringToUTF16Slice("ink-limiting built-in")) {
		goto Error
	}
	if !cmsWriteTag(ar, hICC, CmsSigAToB0Tag, LUT) {
		goto Error
	}
	if !SetSeqDescTag(ar, hICC, []byte("ink-limiting built-in")) {
		goto Error
	}

	cmsPipelineFree(ar, LUT)
	return hICC

Error:
	if LUT != nil {
		cmsPipelineFree(ar, LUT)
	}
		CmsCloseProfile(ar, hICC)
	return nil
}

func cmsCreateInkLimitingDeviceLink(ar *arena.Arena, ColorSpace cmsColorSpaceSignature, Limit float64) CmsHPROFILE {
	return cmsCreateInkLimitingDeviceLinkTHR(ar, nil, ColorSpace, Limit)
}

func cmsCreateLab2ProfileTHR(ar *arena.Arena, ContextID CmsContext, WhitePoint *CmsCIExyY) CmsHPROFILE {
	var hProfile CmsHPROFILE
	var LUT *cmsPipeline
	if WhitePoint == nil {
		hProfile = CmsCreateRGBProfileTHR(ar, ContextID, cmsD50_xyY(), nil, nil)
	} else {
		hProfile = CmsCreateRGBProfileTHR(ar, ContextID, WhitePoint, nil, nil)
	}
	if hProfile == nil {
		return nil
	}

	cmsSetProfileVersion(hProfile, 2.1)
	cmsSetDeviceClass(hProfile, CmsSigAbstractClass)
	cmsSetColorSpace(hProfile, CmsSigLabData)
	cmsSetPCS(hProfile, CmsSigLabData)

	if !SetTextTags(ar, hProfile, StringToUTF16Slice("Lab identity built-in")) {
		return nil
	}

	LUT = cmsPipelineAlloc(ar, ContextID, 3, 3)
	if LUT == nil {
		goto Error
	}

	if !cmsPipelineInsertStage(LUT, CmsAT_BEGIN, cmsStageAllocIdentityCLut(ar, ContextID, 3)) {
		goto Error
	}

	if !cmsWriteTag(ar, hProfile, CmsSigAToB0Tag, LUT) {
		goto Error
	}
	cmsPipelineFree(ar, LUT)
	return hProfile

Error:
	if LUT != nil {
		cmsPipelineFree(ar, LUT)
	}
		CmsCloseProfile(ar, hProfile)
	
	return nil
}

func CmsCreateLab2Profile(ar *arena.Arena, WhitePoint *CmsCIExyY) CmsHPROFILE {
	return cmsCreateLab2ProfileTHR(ar, nil, WhitePoint)
}

func cmsCreateLab4ProfileTHR(ar *arena.Arena, ContextID CmsContext, WhitePoint *CmsCIExyY) CmsHPROFILE {
	var hProfile CmsHPROFILE
	var LUT *cmsPipeline

	if WhitePoint == nil {
		hProfile = CmsCreateRGBProfileTHR(ar, ContextID, cmsD50_xyY(), nil, nil)
	} else {
		hProfile = CmsCreateRGBProfileTHR(ar, ContextID, WhitePoint, nil, nil)
	}
	if hProfile == nil {
		return nil
	}

	cmsSetProfileVersion(hProfile, 4.4)
	cmsSetDeviceClass(hProfile, CmsSigAbstractClass)
	cmsSetColorSpace(hProfile, CmsSigLabData)
	cmsSetPCS(hProfile, CmsSigLabData)

	if !SetTextTags(ar, hProfile, StringToUTF16Slice("Lab identity built-in")) {
		goto Error
	}

	LUT = cmsPipelineAlloc(ar, ContextID, 3, 3)
	if LUT == nil {
		goto Error
	}

	if !cmsPipelineInsertStage(LUT, CmsAT_BEGIN, cmsStageAllocIdentityCurves(ar, ContextID, 3)) {
		goto Error
	}

	if !cmsWriteTag(ar, hProfile, CmsSigAToB0Tag, LUT) {
		goto Error
	}
	cmsPipelineFree(ar, LUT)
	return hProfile

Error:
	if LUT != nil {
		cmsPipelineFree(ar, LUT)
	}
		CmsCloseProfile(ar, hProfile)
	
	return nil
}

func cmsCreateLab4Profile(ar *arena.Arena, WhitePoint *CmsCIExyY) CmsHPROFILE {
	return cmsCreateLab4ProfileTHR(ar, nil, WhitePoint)
}

func cmsCreateXYZProfileTHR(ar *arena.Arena, ContextID CmsContext) CmsHPROFILE {
	var hProfile CmsHPROFILE
	var LUT *cmsPipeline

	hProfile = CmsCreateRGBProfileTHR(ar, ContextID, cmsD50_xyY(), nil, nil)
	if hProfile == nil {
		return nil
	}

	cmsSetProfileVersion(hProfile, 4.4)
	cmsSetDeviceClass(hProfile, CmsSigAbstractClass)
	cmsSetColorSpace(hProfile, CmsSigXYZData)
	cmsSetPCS(hProfile, CmsSigXYZData)

	if !SetTextTags(ar, hProfile, StringToUTF16Slice("XYZ identity built-in")) {
		goto Error
	}

	LUT = cmsPipelineAlloc(ar, ContextID, 3, 3)
	if LUT == nil {
		goto Error
	}

	if !cmsPipelineInsertStage(LUT, CmsAT_BEGIN, cmsStageAllocIdentityCurves(ar, ContextID, 3)) {
		goto Error
	}

	if !cmsWriteTag(ar, hProfile, CmsSigAToB0Tag, LUT) {
		goto Error
	}
	cmsPipelineFree(ar, LUT)
	return hProfile

Error:
	if LUT != nil {
		cmsPipelineFree(ar, LUT)
	}
		CmsCloseProfile(ar, hProfile)
	
	return nil
}

func CmsCreateXYZProfile(ar *arena.Arena) CmsHPROFILE {
	return cmsCreateXYZProfileTHR(ar, nil)
}

//sRGB Curves are defined by:
//
//If  R'sRGB,G'sRGB, B'sRGB < 0.04045
//
//    R =  R'sRGB / 12.92
//    G =  G'sRGB / 12.92
//    B =  B'sRGB / 12.92
//
//
//else if  R'sRGB,G'sRGB, B'sRGB >= 0.04045
//
//    R = ((R'sRGB + 0.055) / 1.055)^2.4
//    G = ((G'sRGB + 0.055) / 1.055)^2.4
//    B = ((B'sRGB + 0.055) / 1.055)^2.4

func Build_sRGBGamma(ar *arena.Arena, ContextID CmsContext) *CmsToneCurve {
	var Parameters [5]float64

	Parameters[0] = 2.4
	Parameters[1] = 1. / 1.055
	Parameters[2] = 0.055 / 1.055
	Parameters[3] = 1. / 12.92
	Parameters[4] = 0.04045

	return cmsBuildParametricToneCurve(ar, ContextID, 4, Parameters[:])
}

func CmsCreate_sRGBProfileTHR(ar *arena.Arena, ContextID CmsContext) CmsHPROFILE {
	// Define the D65 white point
	var D65 CmsCIExyY
	D65.X_small = 0.3127
	D65.Y_small = 0.3290
	D65.Y_large = 1.0

	// Define Rec709 primaries
	var Rec709Primaries CmsCIExyYTRIPLE
	Rec709Primaries.Red.X_small = 0.6400
	Rec709Primaries.Red.Y_small = 0.3300
	Rec709Primaries.Red.Y_large = 1.0
	Rec709Primaries.Green.X_small = 0.3000
	Rec709Primaries.Green.Y_small = 0.6000
	Rec709Primaries.Green.Y_large = 1.0
	Rec709Primaries.Blue.X_small = 0.1500
	Rec709Primaries.Blue.Y_small = 0.0600
	Rec709Primaries.Blue.Y_large = 1.0

	// Allocate Gamma22 tone curves
	var Gamma22 [3]*CmsToneCurve
	Gamma22[0] = Build_sRGBGamma(ar, ContextID)
	Gamma22[1] = Gamma22[0]
	Gamma22[2] = Gamma22[0]

	if Gamma22[0] == nil {
		return nil
	}

	// Create the RGB profile
	hsRGB := CmsCreateRGBProfileTHR(ar, ContextID, &D65, &Rec709Primaries, Gamma22[:])
	CmsFreeToneCurve(Gamma22[0]) // Free the tone curve memory

	if hsRGB == nil {
		return nil
	}

	// Set the text tags
	if !SetTextTags(ar, hsRGB, StringToUTF16Slice("sRGB built-in")) {
		CmsCloseProfile(ar, hsRGB)
		return nil
	}

	return hsRGB
}

func CmsCreate_sRGBProfile(ar *arena.Arena) CmsHPROFILE {
	return CmsCreate_sRGBProfileTHR(ar, nil)
}
