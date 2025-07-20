package golcms

//"unsafe"

func SetTextTags(hProfile CmsHPROFILE, Description []uint16) bool {
	var DescriptionMLU, CopyrightMLU *cmsMLU
	var rc bool
	ContextID := cmsGetProfileContextID(hProfile)

	DescriptionMLU = cmsMLUalloc(ContextID, 1)
	CopyrightMLU = cmsMLUalloc(ContextID, 1)

	if DescriptionMLU == nil || CopyrightMLU == nil {
		goto Error
	}

	if !cmsMLUsetWide(DescriptionMLU, "en", "US", Description) {
		goto Error
	}
	if !cmsMLUsetWide(CopyrightMLU, "en", "US", StringToUTF16Slice("No copyright, use freely")) {
		goto Error
	}

	if !cmsWriteTag(hProfile, cmsSigProfileDescriptionTag, DescriptionMLU) {
		goto Error
	}
	if !cmsWriteTag(hProfile, cmsSigCopyrightTag, CopyrightMLU) {
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
func SetSeqDescTag(hProfile CmsHPROFILE, Model []byte) bool {
	var rc bool
	ContextID := cmsGetProfileContextID(hProfile)
	Seq := cmsAllocProfileSequenceDescription(ContextID, 1)

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
	if !cmsWriteProfileSequence(hProfile, Seq) {
		goto Error
	}

	rc = true

Error:
	if Seq != nil {
		cmsFreeProfileSequenceDescription(Seq)
	}
	return rc
}

// CmsCreateRGBProfileTHR translates the function to Go
func CmsCreateRGBProfileTHR(ContextID CmsContext, WhitePoint *CmsCIExyY, Primaries *CmsCIExyYTRIPLE, TransferFunction []*CmsToneCurve) CmsHPROFILE {
	var (
		hICC          CmsHPROFILE
		MColorants    cmsMAT3
		Colorants     cmsCIEXYZTRIPLE
		MaxWhite      CmsCIExyY
		CHAD          cmsMAT3
		WhitePointXYZ cmsCIEXYZ
	)

	hICC = cmsCreateProfilePlaceholder(ContextID)
	if hICC == nil {
		return nil // can't allocate
	}

	cmsSetProfileVersion(hICC, 4.4)
	cmsSetDeviceClass(hICC, cmsSigDisplayClass)
	cmsSetColorSpace(hICC, cmsSigRgbData)
	cmsSetPCS(hICC, cmsSigXYZData)
	cmsSetHeaderRenderingIntent(hICC, INTENT_PERCEPTUAL)

	// Implement profile using following tags:
	//
	//  1 cmsSigProfileDescriptionTag
	//  2 cmsSigMediaWhitePointTag
	//  3 cmsSigRedColorantTag
	//  4 cmsSigGreenColorantTag
	//  5 cmsSigBlueColorantTag
	//  6 cmsSigRedTRCTag
	//  7 cmsSigGreenTRCTag
	//  8 cmsSigBlueTRCTag
	//  9 Chromatic adaptation Tag
	// This conforms a standard RGB DisplayProfile as says ICC, and then I add (As per addendum II)
	// 10 cmsSigChromaticityTag

	if !SetTextTags(hICC, StringToUTF16Slice("RGB built-in")) {
		goto Error
	}

	if WhitePoint != nil {
		if !cmsWriteTag(hICC, cmsSigMediaWhitePointTag, cmsD50_XYZ()) {
			goto Error
		}

		cmsxyY2XYZ(&WhitePointXYZ, WhitePoint)
		cmsAdaptationMatrix(&CHAD, nil, &WhitePointXYZ, cmsD50_XYZ())

		if !cmsWriteTag(hICC, cmsSigChromaticAdaptationTag, &CHAD) {
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

		if !cmsWriteTag(hICC, cmsSigRedColorantTag, &Colorants.Red) ||
			!cmsWriteTag(hICC, cmsSigGreenColorantTag, &Colorants.Green) ||
			!cmsWriteTag(hICC, cmsSigBlueColorantTag, &Colorants.Blue) {
			goto Error
		}
	}

	if TransferFunction != nil {
		if !cmsWriteTag(hICC, cmsSigRedTRCTag, TransferFunction[0]) {
			goto Error
		}

		if TransferFunction[1] == TransferFunction[0] {
			if !cmsLinkTag(hICC, cmsSigGreenTRCTag, cmsSigRedTRCTag) {
				goto Error
			}
		} else {
			if !cmsWriteTag(hICC, cmsSigGreenTRCTag, TransferFunction[1]) {
				goto Error
			}
		}

		if TransferFunction[2] == TransferFunction[0] {
			if !cmsLinkTag(hICC, cmsSigBlueTRCTag, cmsSigRedTRCTag) {
				goto Error
			}
		} else {
			if !cmsWriteTag(hICC, cmsSigBlueTRCTag, TransferFunction[2]) {
				goto Error
			}
		}
	}

	if Primaries != nil {
		if !cmsWriteTag(hICC, cmsSigChromaticityTag, Primaries) {
			goto Error
		}
	}

	return hICC

Error:
	if hICC != nil {
		CmsCloseProfile(hICC)
	}
	return nil
}

// CmsCreateRGBProfile translates the function to Go
func CmsCreateRGBProfile(WhitePoint *CmsCIExyY, Primaries *CmsCIExyYTRIPLE, TransferFunction []*CmsToneCurve) CmsHPROFILE {
	return CmsCreateRGBProfileTHR(nil, WhitePoint, Primaries, TransferFunction)
}

// cmsCreateGrayProfileTHR translates the function to Go
func cmsCreateGrayProfileTHR(ContextID CmsContext, WhitePoint *CmsCIExyY, TransferFunction *CmsToneCurve) CmsHPROFILE {
	var tmp cmsCIEXYZ
	hICC := cmsCreateProfilePlaceholder(ContextID)
	if hICC == nil {
		return nil
	}

	cmsSetProfileVersion(hICC, 4.4)
	cmsSetDeviceClass(hICC, cmsSigDisplayClass)
	cmsSetColorSpace(hICC, cmsSigGrayData)
	cmsSetPCS(hICC, cmsSigXYZData)
	cmsSetHeaderRenderingIntent(hICC, INTENT_PERCEPTUAL)

	if !SetTextTags(hICC, StringToUTF16Slice("gray built-in")) {
		goto Error
	}

	if WhitePoint != nil {
		cmsxyY2XYZ(&tmp, WhitePoint)
		if !cmsWriteTag(hICC, cmsSigMediaWhitePointTag, &tmp) {
			goto Error
		}
	}

	if TransferFunction != nil {
		if !cmsWriteTag(hICC, cmsSigGrayTRCTag, TransferFunction) {
			goto Error
		}
	}

	return hICC

Error:
	if hICC != nil {
		CmsCloseProfile(hICC)
	}
	return nil
}

// cmsCreateGrayProfile translates the function to Go
func CmsCreateGrayProfile(WhitePoint *CmsCIExyY, TransferFunction *CmsToneCurve) CmsHPROFILE {
	return cmsCreateGrayProfileTHR(nil, WhitePoint, TransferFunction)
}

// cmsCreateLinearizationDeviceLinkTHR translates the function to Go
func cmsCreateLinearizationDeviceLinkTHR(ContextID CmsContext, ColorSpace cmsColorSpaceSignature, TransferFunctions []*CmsToneCurve) CmsHPROFILE {
	hICC := cmsCreateProfilePlaceholder(ContextID)
	if hICC == nil {
		return nil
	}

	cmsSetProfileVersion(hICC, 4.4)
	cmsSetDeviceClass(hICC, cmsSigLinkClass)
	cmsSetColorSpace(hICC, ColorSpace)
	cmsSetPCS(hICC, ColorSpace)
	cmsSetHeaderRenderingIntent(hICC, INTENT_PERCEPTUAL)

	nChannels := cmsChannelsOfColorSpace(ColorSpace)

	Pipeline := cmsPipelineAlloc(ContextID, uint32(nChannels), uint32(nChannels))
	if Pipeline == nil {
		goto Error
	}

	if !cmsPipelineInsertStage(Pipeline, cmsAT_BEGIN, cmsStageAllocToneCurves(ContextID, uint32(nChannels), TransferFunctions)) {
		goto Error
	}

	if !SetTextTags(hICC, StringToUTF16Slice("Linearization built-in")) ||
		!cmsWriteTag(hICC, cmsSigAToB0Tag, Pipeline) ||
		!SetSeqDescTag(hICC, []byte("Linearization built-in")) {
		goto Error
	}

	cmsPipelineFree(Pipeline)
	return hICC

Error:
	cmsPipelineFree(Pipeline)
	if hICC != nil {
		CmsCloseProfile(hICC)
	}
	return nil
}

// cmsCreateLinearizationDeviceLink translates the function to Go
func cmsCreateLinearizationDeviceLink(ColorSpace cmsColorSpaceSignature, TransferFunctions []*CmsToneCurve) CmsHPROFILE {
	return cmsCreateLinearizationDeviceLinkTHR(nil, ColorSpace, TransferFunctions)
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
func InkLimitingSampler(In []uint16, Out []uint16, cargo interface{}) int32 {
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

func cmsCreateInkLimitingDeviceLinkTHR(ContextID CmsContext, ColorSpace cmsColorSpaceSignature, Limit float64) CmsHPROFILE {
	var hICC CmsHPROFILE
	var LUT *cmsPipeline
	var CLUT *cmsStage
	var nChannels int32

	if ColorSpace != cmsSigCmykData {
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

	hICC = cmsCreateProfilePlaceholder(ContextID)
	if hICC == nil {
		return nil
	}

	cmsSetProfileVersion(hICC, 4.4)
	cmsSetDeviceClass(hICC, cmsSigLinkClass)
	cmsSetColorSpace(hICC, ColorSpace)
	cmsSetPCS(hICC, ColorSpace)
	cmsSetHeaderRenderingIntent(hICC, INTENT_PERCEPTUAL)

	LUT = cmsPipelineAlloc(ContextID, 4, 4)
	if LUT == nil {
		goto Error
	}

	nChannels = int32(cmsChannelsOf(ColorSpace))

	CLUT = cmsStageAllocCLut16bit(ContextID, 17, uint32(nChannels), uint32(nChannels), nil)
	if CLUT == nil {
		goto Error
	}

	if !cmsStageSampleCLut16bit(CLUT, InkLimitingSampler, &Limit, 0) {
		goto Error
	}

	if !cmsPipelineInsertStage(LUT, cmsAT_BEGIN, cmsStageAllocIdentityCurves(ContextID, uint32(nChannels))) ||
		!cmsPipelineInsertStage(LUT, cmsAT_END, CLUT) ||
		!cmsPipelineInsertStage(LUT, cmsAT_END, cmsStageAllocIdentityCurves(ContextID, uint32(nChannels))) {
		goto Error
	}

	if !SetTextTags(hICC, StringToUTF16Slice("ink-limiting built-in")) {
		goto Error
	}
	if !cmsWriteTag(hICC, cmsSigAToB0Tag, LUT) {
		goto Error
	}
	if !SetSeqDescTag(hICC, []byte("ink-limiting built-in")) {
		goto Error
	}

	cmsPipelineFree(LUT)
	return hICC

Error:
	if LUT != nil {
		cmsPipelineFree(LUT)
	}
	if hICC != nil {
		CmsCloseProfile(hICC)
	}
	return nil
}

func cmsCreateInkLimitingDeviceLink(ColorSpace cmsColorSpaceSignature, Limit float64) CmsHPROFILE {
	return cmsCreateInkLimitingDeviceLinkTHR(nil, ColorSpace, Limit)
}

func cmsCreateLab2ProfileTHR(ContextID CmsContext, WhitePoint *CmsCIExyY) CmsHPROFILE {
	var hProfile CmsHPROFILE
	var LUT *cmsPipeline
	if WhitePoint == nil {
		hProfile = CmsCreateRGBProfileTHR(ContextID, cmsD50_xyY(), nil, nil)
	} else {
		hProfile = CmsCreateRGBProfileTHR(ContextID, WhitePoint, nil, nil)
	}
	if hProfile == nil {
		return nil
	}

	cmsSetProfileVersion(hProfile, 2.1)
	cmsSetDeviceClass(hProfile, cmsSigAbstractClass)
	cmsSetColorSpace(hProfile, cmsSigLabData)
	cmsSetPCS(hProfile, cmsSigLabData)

	if !SetTextTags(hProfile, StringToUTF16Slice("Lab identity built-in")) {
		return nil
	}

	LUT = cmsPipelineAlloc(ContextID, 3, 3)
	if LUT == nil {
		goto Error
	}

	if !cmsPipelineInsertStage(LUT, cmsAT_BEGIN, cmsStageAllocIdentityCLut(ContextID, 3)) {
		goto Error
	}

	if !cmsWriteTag(hProfile, cmsSigAToB0Tag, LUT) {
		goto Error
	}
	cmsPipelineFree(LUT)
	return hProfile

Error:
	if LUT != nil {
		cmsPipelineFree(LUT)
	}
	if hProfile != nil {
		CmsCloseProfile(hProfile)
	}
	return nil
}

func CmsCreateLab2Profile(WhitePoint *CmsCIExyY) CmsHPROFILE {
	return cmsCreateLab2ProfileTHR(nil, WhitePoint)
}

func cmsCreateLab4ProfileTHR(ContextID CmsContext, WhitePoint *CmsCIExyY) CmsHPROFILE {
	var hProfile CmsHPROFILE
	var LUT *cmsPipeline

	if WhitePoint == nil {
		hProfile = CmsCreateRGBProfileTHR(ContextID, cmsD50_xyY(), nil, nil)
	} else {
		hProfile = CmsCreateRGBProfileTHR(ContextID, WhitePoint, nil, nil)
	}
	if hProfile == nil {
		return nil
	}

	cmsSetProfileVersion(hProfile, 4.4)
	cmsSetDeviceClass(hProfile, cmsSigAbstractClass)
	cmsSetColorSpace(hProfile, cmsSigLabData)
	cmsSetPCS(hProfile, cmsSigLabData)

	if !SetTextTags(hProfile, StringToUTF16Slice("Lab identity built-in")) {
		goto Error
	}

	LUT = cmsPipelineAlloc(ContextID, 3, 3)
	if LUT == nil {
		goto Error
	}

	if !cmsPipelineInsertStage(LUT, cmsAT_BEGIN, cmsStageAllocIdentityCurves(ContextID, 3)) {
		goto Error
	}

	if !cmsWriteTag(hProfile, cmsSigAToB0Tag, LUT) {
		goto Error
	}
	cmsPipelineFree(LUT)
	return hProfile

Error:
	if LUT != nil {
		cmsPipelineFree(LUT)
	}
	if hProfile != nil {
		CmsCloseProfile(hProfile)
	}
	return nil
}

func cmsCreateLab4Profile(WhitePoint *CmsCIExyY) CmsHPROFILE {
	return cmsCreateLab4ProfileTHR(nil, WhitePoint)
}

func cmsCreateXYZProfileTHR(ContextID CmsContext) CmsHPROFILE {
	var hProfile CmsHPROFILE
	var LUT *cmsPipeline

	hProfile = CmsCreateRGBProfileTHR(ContextID, cmsD50_xyY(), nil, nil)
	if hProfile == nil {
		return nil
	}

	cmsSetProfileVersion(hProfile, 4.4)
	cmsSetDeviceClass(hProfile, cmsSigAbstractClass)
	cmsSetColorSpace(hProfile, cmsSigXYZData)
	cmsSetPCS(hProfile, cmsSigXYZData)

	if !SetTextTags(hProfile, StringToUTF16Slice("XYZ identity built-in")) {
		goto Error
	}

	LUT = cmsPipelineAlloc(ContextID, 3, 3)
	if LUT == nil {
		goto Error
	}

	if !cmsPipelineInsertStage(LUT, cmsAT_BEGIN, cmsStageAllocIdentityCurves(ContextID, 3)) {
		goto Error
	}

	if !cmsWriteTag(hProfile, cmsSigAToB0Tag, LUT) {
		goto Error
	}
	cmsPipelineFree(LUT)
	return hProfile

Error:
	if LUT != nil {
		cmsPipelineFree(LUT)
	}
	if hProfile != nil {
		CmsCloseProfile(hProfile)
	}
	return nil
}

func CmsCreateXYZProfile() CmsHPROFILE {
	return cmsCreateXYZProfileTHR(nil)
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

func Build_sRGBGamma(ContextID CmsContext) *CmsToneCurve {
	var Parameters [5]float64

	Parameters[0] = 2.4
	Parameters[1] = 1. / 1.055
	Parameters[2] = 0.055 / 1.055
	Parameters[3] = 1. / 12.92
	Parameters[4] = 0.04045

	return cmsBuildParametricToneCurve(ContextID, 4, Parameters[:])
}

func CmsCreate_sRGBProfileTHR(ContextID CmsContext) CmsHPROFILE {
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
	Gamma22[0] = Build_sRGBGamma(ContextID)
	Gamma22[1] = Gamma22[0]
	Gamma22[2] = Gamma22[0]

	if Gamma22[0] == nil {
		return nil
	}

	// Create the RGB profile
	hsRGB := CmsCreateRGBProfileTHR(ContextID, &D65, &Rec709Primaries, Gamma22[:])
	CmsFreeToneCurve(Gamma22[0]) // Free the tone curve memory

	if hsRGB == nil {
		return nil
	}

	// Set the text tags
	if !SetTextTags(hsRGB, StringToUTF16Slice("sRGB built-in")) {
		CmsCloseProfile(hsRGB)
		return nil
	}

	return hsRGB
}

func CmsCreate_sRGBProfile() CmsHPROFILE {
	return CmsCreate_sRGBProfileTHR(nil)
}
