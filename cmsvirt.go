package golcms



func cmsCreateInkLimitingDeviceLinkTHR(ContextID cmsContext, ColorSpace cmsColorSpaceSignature, Limit cmsFloat64Number) cmsHPROFILE {
	var hICC cmsHPROFILE
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

	nChannels = cmsChannelsOf(ColorSpace)

	CLUT = cmsStageAllocCLut16bit(ContextID, 17, int(nChannels), int(nChannels), nil)
	if CLUT == nil {
		goto Error
	}

	if !cmsStageSampleCLut16bit(CLUT, InkLimitingSampler, &Limit, 0) {
		goto Error
	}

	if !cmsPipelineInsertStage(LUT, cmsAT_BEGIN, cmsStageAllocIdentityCurves(ContextID, int(nChannels))) ||
		!cmsPipelineInsertStage(LUT, cmsAT_END, CLUT) ||
		!cmsPipelineInsertStage(LUT, cmsAT_END, cmsStageAllocIdentityCurves(ContextID, int(nChannels))) {
		goto Error
	}

	if !SetTextTags(hICC, "ink-limiting built-in") {
		goto Error
	}
	if !cmsWriteTag(hICC, cmsSigAToB0Tag, LUT) {
		goto Error
	}
	if !SetSeqDescTag(hICC, "ink-limiting built-in") {
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

func cmsCreateInkLimitingDeviceLink(ColorSpace cmsColorSpaceSignature, Limit cmsFloat64Number) cmsHPROFILE {
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
	cmsSetDeviceClass(hProfile, cmsSigAbstractClass)
	cmsSetColorSpace(hProfile, cmsSigLabData)
	cmsSetPCS(hProfile, cmsSigLabData)

	if !SetTextTags(hProfile, "Lab identity built-in") {
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
	cmsSetDeviceClass(hProfile, cmsSigAbstractClass)
	cmsSetColorSpace(hProfile, cmsSigLabData)
	cmsSetPCS(hProfile, cmsSigLabData)

	if !SetTextTags(hProfile, "Lab identity built-in") {
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
	cmsSetDeviceClass(hProfile, cmsSigAbstractClass)
	cmsSetColorSpace(hProfile, cmsSigXYZData)
	cmsSetPCS(hProfile, cmsSigXYZData)

	if !SetTextTags(hProfile, "XYZ identity built-in") {
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
		cmsCloseProfile(hProfile)
	}
	return nil
}

func cmsCreateXYZProfile() cmsHPROFILE {
	return cmsCreateXYZProfileTHR(nil)
}
