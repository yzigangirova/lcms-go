package golcms

import (
	"unsafe"
	"utf16"
)

// StringToUTF16Slice converts a Go string to a slice of uint16 for wide characters.
func StringToUTF16Slice(s string) []uint16 {
	// Convert the string to a slice of runes (Unicode code points)
	runes := []rune(s)
	// Encode the runes into UTF-16
	return utf16.Encode(runes)
}
func SetTextTags(hProfile cmsHPROFILE, Description []uint16) bool {
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

	if !cmsWriteTag(hProfile, cmsSigProfileDescriptionTag, unsafe.Pointer(DescriptionMLU)) {
		goto Error
	}
	if !cmsWriteTag(hProfile, cmsSigCopyrightTag, unsafe.Pointer(CopyrightMLU)) {
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
func SetSeqDescTag(hProfile cmsHPROFILE, Model *byte) bool {
	var rc bool
	ContextID := cmsGetProfileContextID(hProfile)
	Seq := cmsAllocProfileSequenceDescription(ContextID, 1)

	if Seq == nil {
		return false
	}

	// Initialize fields in Seq
	(*Seq.seq).deviceMfg = 0
	(*Seq.seq).deviceModel = 0

	// Set attributes based on conditional compilation
	(*Seq.seq).attributes = 0

	(*Seq.seq).technology = 0

	// Set Manufacturer and Model text
	cmsMLUsetASCII((*Seq.seq).Manufacturer, cmsNoLanguage, cmsNoCountry, &([]byte("Little CMS"))[0])
	cmsMLUsetASCII((*Seq.seq).Model, cmsNoLanguage, cmsNoCountry, Model)

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

// cmsCreateRGBProfileTHR translates the function to Go
func cmsCreateRGBProfileTHR(ContextID cmsContext, WhitePoint *cmsCIExyY, Primaries *cmsCIExyYTRIPLE, TransferFunction []*cmsToneCurve) cmsHPROFILE {
	var (
		hICC          cmsHPROFILE
		MColorants    cmsMAT3
		Colorants     cmsCIEXYZTRIPLE
		MaxWhite      cmsCIExyY
		CHAD          cmsMAT3
		WhitePointXYZ cmsCIEXYZ
	)

	hICC = cmsCreateProfilePlaceholder(ContextID)
	if hICC == nil {
		return nil // can't allocate
	}

	cmsSetProfileVersion(hICC, 4.4)
	cmsSetDeviceClass(unsafe.Pointer(hICC), cmsSigDisplayClass)
	cmsSetColorSpace(unsafe.Pointer(hICC), cmsSigRgbData)
	cmsSetPCS(unsafe.Pointer(hICC), cmsSigXYZData)
	cmsSetHeaderRenderingIntent(unsafe.Pointer(hICC), INTENT_PERCEPTUAL)

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
		if !cmsWriteTag(hICC, cmsSigMediaWhitePointTag, unsafe.Pointer(cmsD50_XYZ())) {
			goto Error
		}

		cmsxyY2XYZ(&WhitePointXYZ, WhitePoint)
		cmsAdaptationMatrix(&CHAD, nil, &WhitePointXYZ, cmsD50_XYZ())

		if !cmsWriteTag(hICC, cmsSigChromaticAdaptationTag, unsafe.Pointer(&CHAD)) {
			goto Error
		}
	}

	if WhitePoint != nil && Primaries != nil {
		MaxWhite.x = WhitePoint.x
		MaxWhite.y = WhitePoint.y
		MaxWhite.Y = 1.0

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

		if !cmsWriteTag(hICC, cmsSigRedColorantTag, unsafe.Pointer(&Colorants.Red)) ||
			!cmsWriteTag(hICC, cmsSigGreenColorantTag, unsafe.Pointer(&Colorants.Green)) ||
			!cmsWriteTag(hICC, cmsSigBlueColorantTag, unsafe.Pointer(&Colorants.Blue)) {
			goto Error
		}
	}

	if TransferFunction != nil {
		if !cmsWriteTag(hICC, cmsSigRedTRCTag, unsafe.Pointer(TransferFunction[0])) {
			goto Error
		}

		if TransferFunction[1] == TransferFunction[0] {
			if !cmsLinkTag(hICC, cmsSigGreenTRCTag, cmsSigRedTRCTag) {
				goto Error
			}
		} else {
			if !cmsWriteTag(hICC, cmsSigGreenTRCTag, unsafe.Pointer(TransferFunction[1])) {
				goto Error
			}
		}

		if TransferFunction[2] == TransferFunction[0] {
			if !cmsLinkTag(hICC, cmsSigBlueTRCTag, cmsSigRedTRCTag) {
				goto Error
			}
		} else {
			if !cmsWriteTag(hICC, cmsSigBlueTRCTag, unsafe.Pointer(TransferFunction[2])) {
				goto Error
			}
		}
	}

	if Primaries != nil {
		if !cmsWriteTag(hICC, cmsSigChromaticityTag, unsafe.Pointer(Primaries)) {
			goto Error
		}
	}

	return hICC

Error:
	if hICC != nil {
		cmsCloseProfile(hICC)
	}
	return nil
}

// cmsCreateRGBProfile translates the function to Go
func cmsCreateRGBProfile(WhitePoint *cmsCIExyY, Primaries *cmsCIExyYTRIPLE, TransferFunction []*cmsToneCurve) cmsHPROFILE {
	return cmsCreateRGBProfileTHR(nil, WhitePoint, Primaries, TransferFunction)
}

// cmsCreateGrayProfileTHR translates the function to Go
func cmsCreateGrayProfileTHR(ContextID cmsContext, WhitePoint *cmsCIExyY, TransferFunction *cmsToneCurve) cmsHPROFILE {
	var tmp cmsCIEXYZ
	hICC := cmsCreateProfilePlaceholder(ContextID)
	if hICC == nil {
		return nil
	}

	cmsSetProfileVersion(hICC, 4.4)
	cmsSetDeviceClass(unsafe.Pointer(hICC), cmsSigDisplayClass)
	cmsSetColorSpace(unsafe.Pointer(hICC), cmsSigGrayData)
	cmsSetPCS(unsafe.Pointer(hICC), cmsSigXYZData)
	cmsSetHeaderRenderingIntent(unsafe.Pointer(hICC), INTENT_PERCEPTUAL)

	if !SetTextTags(hICC, StringToUTF16Slice("gray built-in")) {
		goto Error
	}

	if WhitePoint != nil {
		cmsxyY2XYZ(&tmp, WhitePoint)
		if !cmsWriteTag(hICC, cmsSigMediaWhitePointTag, unsafe.Pointer(&tmp)) {
			goto Error
		}
	}

	if TransferFunction != nil {
		if !cmsWriteTag(hICC, cmsSigGrayTRCTag, unsafe.Pointer(TransferFunction)) {
			goto Error
		}
	}

	return hICC

Error:
	if hICC != nil {
		cmsCloseProfile(hICC)
	}
	return nil
}

// cmsCreateGrayProfile translates the function to Go
func cmsCreateGrayProfile(WhitePoint *cmsCIExyY, TransferFunction *cmsToneCurve) cmsHPROFILE {
	return cmsCreateGrayProfileTHR(nil, WhitePoint, TransferFunction)
}

// cmsCreateLinearizationDeviceLinkTHR translates the function to Go
func cmsCreateLinearizationDeviceLinkTHR(ContextID cmsContext, ColorSpace cmsColorSpaceSignature, TransferFunctions []*cmsToneCurve) cmsHPROFILE {
	hICC := cmsCreateProfilePlaceholder(ContextID)
	if hICC == nil {
		return nil
	}

	cmsSetProfileVersion(hICC, 4.4)
	cmsSetDeviceClass(unsafe.Pointer(hICC), cmsSigLinkClass)
	cmsSetColorSpace(unsafe.Pointer(hICC), ColorSpace)
	cmsSetPCS(unsafe.Pointer(hICC), ColorSpace)
	cmsSetHeaderRenderingIntent(unsafe.Pointer(hICC), INTENT_PERCEPTUAL)

	nChannels := cmsChannelsOfColorSpace(ColorSpace)

	Pipeline := cmsPipelineAlloc(ContextID, uint32(nChannels), uint32(nChannels))
	if Pipeline == nil {
		goto Error
	}

	if !cmsPipelineInsertStage(Pipeline, cmsAT_BEGIN, cmsStageAllocToneCurves(ContextID, uint32(nChannels), &TransferFunctions[0])) {
		goto Error
	}

	if !SetTextTags(hICC, StringToUTF16Slice("Linearization built-in")) ||
		!cmsWriteTag(hICC, cmsSigAToB0Tag, unsafe.Pointer(Pipeline)) ||
		!SetSeqDescTag(hICC, &[]byte("Linearization built-in")[0]) {
		goto Error
	}

	cmsPipelineFree(Pipeline)
	return hICC

Error:
	cmsPipelineFree(Pipeline)
	if hICC != nil {
		cmsCloseProfile(hICC)
	}
	return nil
}

// cmsCreateLinearizationDeviceLink translates the function to Go
func cmsCreateLinearizationDeviceLink(ColorSpace cmsColorSpaceSignature, TransferFunctions []*cmsToneCurve) cmsHPROFILE {
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
func InkLimitingSampler(In []uint16, Out []uint16, Cargo unsafe.Pointer) int32 {
	InkLimit := *(*float64)(Cargo)
	var SumCMY, SumCMYK, Ratio float64

	// Convert InkLimit to 0-65535 range
	InkLimit *= 655.35

	// Compute CMY and CMYK sums
	SumCMY = float64(In[0]) + float64(In[1]) + float64(In[2])
	SumCMYK = SumCMY + float64(In[3])

	// Adjust CMY channels if the ink limit is exceeded
	if SumCMYK > InkLimit {
		Ratio = 1 - ((SumCMYK - InkLimit) / SumCMY)
		if Ratio < 0 {
			Ratio = 0
		}
	} else {
		Ratio = 1
	}

	// Apply the ratio to CMY channels
	Out[0] = cmsQuickSaturateWord(float64(In[0]) * Ratio) // C
	Out[1] = cmsQuickSaturateWord(float64(In[1]) * Ratio) // M
	Out[2] = cmsQuickSaturateWord(float64(In[2]) * Ratio) // Y

	// K channel is untouched
	Out[3] = In[3]

	return 1 // Equivalent to TRUE in C
}

func cmsCreateInkLimitingDeviceLinkTHR(ContextID cmsContext, ColorSpace cmsColorSpaceSignature, Limit float64) cmsHPROFILE {
	var hICC cmsHPROFILE
	var LUT *cmsPipeline
	var CLUT *cmsStage
	var nChannels int32

	if ColorSpace != cmsSigCmykData {
		cmsSignalError(unsafe.Pointer(ContextID), cmsERROR_COLORSPACE_CHECK, "InkLimiting: Only CMYK currently supported")
		return nil
	}

	if Limit < 0.0 || Limit > 400 {
		cmsSignalError(unsafe.Pointer(ContextID), cmsERROR_RANGE, "InkLimiting: Limit should be between 0..400")
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
	cmsSetDeviceClass(unsafe.Pointer(hICC), cmsSigLinkClass)
	cmsSetColorSpace(unsafe.Pointer(hICC), ColorSpace)
	cmsSetPCS(unsafe.Pointer(hICC), ColorSpace)
	cmsSetHeaderRenderingIntent(unsafe.Pointer(hICC), INTENT_PERCEPTUAL)

	LUT = cmsPipelineAlloc(ContextID, 4, 4)
	if LUT == nil {
		goto Error
	}

	nChannels = int32(cmsChannelsOf(ColorSpace))

	CLUT = cmsStageAllocCLut16bit(ContextID, 17, uint32(nChannels), uint32(nChannels), nil)
	if CLUT == nil {
		goto Error
	}

	if !cmsStageSampleCLut16bit(CLUT, InkLimitingSampler, unsafe.Pointer(&Limit), 0) {
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
	if !cmsWriteTag(hICC, cmsSigAToB0Tag, unsafe.Pointer(LUT)) {
		goto Error
	}
	if !SetSeqDescTag(hICC, &[]byte("ink-limiting built-in")[0]) {
		goto Error
	}

	cmsPipelineFree(LUT)
	return hICC

Error:
	if LUT != nil {
		cmsPipelineFree(LUT)
	}
	if hICC != nil {
		cmsCloseProfile(hICC)
	}
	return nil
}

func cmsCreateInkLimitingDeviceLink(ColorSpace cmsColorSpaceSignature, Limit float64) cmsHPROFILE {
	return cmsCreateInkLimitingDeviceLinkTHR(nil, ColorSpace, Limit)
}

func cmsCreateLab2ProfileTHR(ContextID cmsContext, WhitePoint *cmsCIExyY) cmsHPROFILE {
	var hProfile cmsHPROFILE
	var LUT *cmsPipeline

	hProfile = cmsCreateRGBProfileTHR(ContextID, cmsD50_xyY(), nil, nil)
	if hProfile == nil {
		return nil
	}

	cmsSetProfileVersion(hProfile, 2.1)
	cmsSetDeviceClass(unsafe.Pointer(hProfile), cmsSigAbstractClass)
	cmsSetColorSpace(unsafe.Pointer(hProfile), cmsSigLabData)
	cmsSetPCS(unsafe.Pointer(hProfile), cmsSigLabData)

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

	if !cmsWriteTag(hProfile, cmsSigAToB0Tag, unsafe.Pointer(LUT)) {
		goto Error
	}
	cmsPipelineFree(LUT)
	return hProfile

Error:
	if LUT != nil {
		cmsPipelineFree(LUT)
	}
	if hProfile != nil {
		cmsCloseProfile(hProfile)
	}
	return nil
}

func cmsCreateLab2Profile(WhitePoint *cmsCIExyY) cmsHPROFILE {
	return cmsCreateLab2ProfileTHR(nil, WhitePoint)
}

func cmsCreateLab4ProfileTHR(ContextID cmsContext, WhitePoint *cmsCIExyY) cmsHPROFILE {
	var hProfile cmsHPROFILE
	var LUT *cmsPipeline

	hProfile = cmsCreateRGBProfileTHR(ContextID, cmsD50_xyY(), nil, nil)
	if hProfile == nil {
		return nil
	}

	cmsSetProfileVersion(hProfile, 4.4)
	cmsSetDeviceClass(unsafe.Pointer(hProfile), cmsSigAbstractClass)
	cmsSetColorSpace(unsafe.Pointer(hProfile), cmsSigLabData)
	cmsSetPCS(unsafe.Pointer(hProfile), cmsSigLabData)

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

	if !cmsWriteTag(hProfile, cmsSigAToB0Tag, unsafe.Pointer(LUT)) {
		goto Error
	}
	cmsPipelineFree(LUT)
	return hProfile

Error:
	if LUT != nil {
		cmsPipelineFree(LUT)
	}
	if hProfile != nil {
		cmsCloseProfile(hProfile)
	}
	return nil
}

func cmsCreateLab4Profile(WhitePoint *cmsCIExyY) cmsHPROFILE {
	return cmsCreateLab4ProfileTHR(nil, WhitePoint)
}

func cmsCreateXYZProfileTHR(ContextID cmsContext) cmsHPROFILE {
	var hProfile cmsHPROFILE
	var LUT *cmsPipeline

	hProfile = cmsCreateRGBProfileTHR(ContextID, cmsD50_xyY(), nil, nil)
	if hProfile == nil {
		return nil
	}

	cmsSetProfileVersion(hProfile, 4.4)
	cmsSetDeviceClass(unsafe.Pointer(hProfile), cmsSigAbstractClass)
	cmsSetColorSpace(unsafe.Pointer(hProfile), cmsSigXYZData)
	cmsSetPCS(unsafe.Pointer(hProfile), cmsSigXYZData)

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

	if !cmsWriteTag(hProfile, cmsSigAToB0Tag, unsafe.Pointer(LUT)) {
		goto Error
	}
	cmsPipelineFree(LUT)
	return hProfile

Error:
	if LUT != nil {
		cmsPipelineFree(LUT)
	}
	if hProfile != nil {
		cmsCloseProfile(hProfile)
	}
	return nil
}

func cmsCreateXYZProfile() cmsHPROFILE {
	return cmsCreateXYZProfileTHR(nil)
}
